package impl

import (
	"crypto"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

/** Feature Functions **/

// Upload implements peer.Upload
func (n *node) Upload(data io.Reader) (metahash string, err error) {
	metadata := make([]byte, 0)
	metakey := make([]byte, 0)
	blobStorage := n.conf.Storage.GetDataBlobStore()
	metadataSepBytes := []byte(peer.MetafileSep)
	for {
		// read chunk
		chunk := make([]byte, n.conf.ChunkSize)
		size, readErr := data.Read(chunk)
		if readErr == io.EOF || size == 0 {
			break
		} else if readErr != nil {
			err = readErr
			return
		}

		// compute CID
		chunkHash, chunkCID := ComputeCID(chunk[:size])
		// store chunk
		blobStorage.Set(chunkCID, chunk[:size])
		// add to metafile
		metakey = append(metakey, chunkHash...)
		if len(metadata) > 0 {
			metadata = append(metadata, metadataSepBytes...)
		}
		metadata = append(metadata, []byte(chunkCID)...)
	}

	// compute CID for metadata and save
	_, metahash = ComputeCID(metakey)
	blobStorage.Set(metahash, metadata)

	return
}

// Download implements peer.Download
func (n *node) Download(metahash string) (data []byte, err error) {
	// get metadata
	metadata, readErr := n.GetData(metahash)
	if readErr != nil {
		err = readErr
		return
	}
	// get chunks
	chunkCIDs := strings.Split(string(metadata), peer.MetafileSep)
	data = make([]byte, 0)
	for _, chunkCID := range chunkCIDs {
		chunkData, readErr := n.GetData(chunkCID)
		if readErr != nil {
			err = readErr
			return
		}
		data = append(data, chunkData...)
	}

	return
}

// Tag implements peer.Tag
func (n *node) Tag(name string, mh string) error {
	n.conf.Storage.GetNamingStore().Set(name, []byte(mh))
	return nil
}

// Resolve implements peer.Resolve
func (n *node) Resolve(name string) (metahash string) {
	metahash = string(n.conf.Storage.GetNamingStore().Get(name))
	return
}

// GetCatalog implements peer.GetCatalog
func (n *node) GetCatalog() peer.Catalog {
	return n.catalog.getAll()
}

// UpdateCatalog implements peer.UpdateCatalog
func (n *node) UpdateCatalog(key string, peer string) {
	n.catalog.add(key, peer)
}

// SearchAll implements peer.SearchAll
func (n *node) SearchAll(reg regexp.Regexp, budget uint, timeout time.Duration) (names []string, err error) {
	// search in remote naming store
	err = n.RequestRemoteNames(reg, n.conf.Socket.GetAddress(), budget, timeout)
	if err != nil {
		return
	}

	// search in local naming store
	names = make([]string, 0)
	regMatch := func(key string, val []byte) bool {
		if reg.MatchString(key) {
			names = append(names, key)
		}
		return true
	}
	n.conf.Storage.GetNamingStore().ForEach(regMatch)

	return
}

/** Message Handler **/

// ProcessDataRequestMessage is a callback function to handle received data request message
func (n *node) ProcessDataRequestMessage(msg types.Message, pkt transport.Packet) error {
	requestMsg, ok := msg.(*types.DataRequestMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	cid := requestMsg.Key
	data := n.conf.Storage.GetDataBlobStore().Get(cid)
	err := n.SendDataReplyMessage(pkt.Header.Source, requestMsg.RequestID, cid, data)

	return err
}

// ProcessDataReplyMessage is a callback function to handle received data reply message
func (n *node) ProcessDataReplyMessage(msg types.Message, pkt transport.Packet) error {
	replyMsg, ok := msg.(*types.DataReplyMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	if len(replyMsg.Value) > 0 {
		// update local storage
		n.conf.Storage.GetDataBlobStore().Set(replyMsg.Key, replyMsg.Value)
	}
	if channel, ok := n.replyChannels.get(replyMsg.RequestID); ok {
		*channel <- true
	}

	return nil
}

// ProcessSearchRequestMessage is a callback function to handle received search request message
func (n *node) ProcessSearchRequestMessage(msg types.Message, pkt transport.Packet) error {
	requestMsg, ok := msg.(*types.SearchRequestMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	// not process the request message from self
	if requestMsg.Origin == n.conf.Socket.GetAddress() {
		return nil
	}

	// forward search
	reg, err := regexp.Compile(requestMsg.Pattern)
	if err != nil {
		return err
	}
	// TODO check timeout
	budget := requestMsg.Budget - 1
	if budget > 0 {
		fmt.Println(requestMsg.Origin, budget)
		n.RequestRemoteNames(*reg, requestMsg.Origin, budget, 100*time.Millisecond)
	}

	// construct file info
	fileinfos := make([]types.FileInfo, 0)
	regMatch := func(key string, val []byte) bool {
		if reg.MatchString(key) {
			metahash := string(val)
			if fileinfo, ok := n.GetLocalFileInfo(key, metahash); ok {
				fileinfos = append(fileinfos, fileinfo)
			}
		}
		return true
	}
	n.conf.Storage.GetNamingStore().ForEach(regMatch)

	// send reply
	err = n.SendSearchReplyMessage(requestMsg.Origin, requestMsg.RequestID, fileinfos)
	if err != nil {
		return err
	}

	return err
}

// ProcessSearchReplyMessage is a callback function to handle received search reply message
func (n *node) ProcessSearchReplyMessage(msg types.Message, pkt transport.Packet) error {
	replyMsg, ok := msg.(*types.SearchReplyMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	for _, fileinfo := range replyMsg.Responses {
		// update naming store
		n.conf.Storage.GetNamingStore().Set(fileinfo.Name, []byte(fileinfo.Metahash))
		// update catalog
		n.catalog.add(fileinfo.Metahash, pkt.Header.Source)
		for _, chunkCID := range fileinfo.Chunks {
			if chunkCID == nil {
				continue
			}
			n.catalog.add(string(chunkCID), pkt.Header.Source)
		}
	}
	if channel, ok := n.replyChannels.get(replyMsg.RequestID); ok {
		*channel <- true
	}

	return nil
}

/** Private Helpfer Functions **/

// ComputeCID computes the encoded hash of the given byte array
func ComputeCID(data []byte) (hash []byte, hashHex string) {
	h := crypto.SHA256.New()
	h.Write(data)
	hash = h.Sum(nil)
	hashHex = hex.EncodeToString(hash)

	return
}

// FairBudget distributes budget as fairly as possible to pieces
func FairBudget(budget uint, pieces uint) (base uint, extra int) {
	base = budget / pieces
	extra = int(budget % pieces)

	return
}

// GetData returns an byte araay of the given encoded hash
func (n *node) GetData(cid string) (data []byte, err error) {
	var backoffMult uint = 1
	backoffInfo := n.conf.BackoffDataRequest
	for i := 0; i < int(backoffInfo.Retry); i++ {
		// check local storage
		if content := n.conf.Storage.GetDataBlobStore().Get(cid); content != nil {
			data = content
			return
		}

		// sleep for exponential backoff
		if i > 0 {
			time.Sleep(backoffInfo.Initial * time.Duration(backoffMult-1))
			backoffMult *= backoffInfo.Factor
		}

		// send request to another peer
		data, err = n.RequestRemoteData(cid, backoffInfo.Initial)
		if err != nil || data != nil {
			return
		}
	}
	err = xerrors.Errorf("Data request timed out for %s", cid)

	return
}

// GetLocalFileInfo constructs a list of fileinfo from local storage
func (n *node) GetLocalFileInfo(name string, metahash string) (fileinfo types.FileInfo, ok bool) {
	metadata := n.conf.Storage.GetDataBlobStore().Get(metahash)
	if metadata == nil {
		ok = false
		return
	}

	// get chunks
	chunkCIDs := strings.Split(string(metadata), peer.MetafileSep)
	chunks := make([][]byte, 0)
	for _, chunkCID := range chunkCIDs {
		chunkData := n.conf.Storage.GetDataBlobStore().Get(chunkCID)
		if chunkData != nil {
			chunks = append(chunks, []byte(chunkCID))
		} else {
			chunks = append(chunks, nil)
		}
	}

	fileinfo = types.FileInfo{Name: name, Metahash: metahash, Chunks: chunks}
	ok = true

	return
}

// GetRandomNeighbor randomly returns a neighbor
func (n *node) GetRandomProvider(cid string) (provider string, ok bool) {
	n.catalog.RLock()
	providers := []string{}
	for key, _ := range n.catalog.catalog[cid] {
		providers = append(providers, key)
	}
	n.catalog.RUnlock()
	if len(providers) == 0 {
		ok = false
		return
	}
	provider, ok = providers[rand.Intn(len(providers))], true

	return
}

// RequestRemoteData sends a data request to a random provider and wait until timeout
func (n *node) RequestRemoteData(cid string, timeout time.Duration) (data []byte, err error) {
	if provider, ok := n.GetRandomProvider(cid); ok {
		// wait for response
		rid := xid.New().String()
		err = n.SendDataRequestMessage(provider, rid, cid)
		if err != nil {
			return
		}
		channel := make(chan bool)
		n.replyChannels.add(rid, &channel)
		select {
		case <-channel:
			// get reply
			n.replyChannels.remove(rid)
			data = n.conf.Storage.GetDataBlobStore().Get(cid)
			if data == nil {
				err = xerrors.Errorf("Empty value in response for %s", cid)
			}
		case <-time.After(timeout):
			// no reply.
			data = nil
		}
	} else {
		err = xerrors.Errorf("No available provider for %s has been found.", cid)
	}

	return
}

// RequestRemoteNames requests remote peers for matched names
func (n *node) RequestRemoteNames(reg regexp.Regexp, origin string, budget uint, timeout time.Duration) (err error) {
	neighbors := n.GetNeighbors(origin)
	if len(neighbors) == 0 {
		return
	}

	base, extra := FairBudget(budget, uint(len(neighbors)))
	for i, neighbor := range neighbors {
		rid := xid.New().String()
		if i < extra {
			err = n.SendSearchRequestMessage(neighbor, rid, origin, reg, base+1)
		} else {
			if base == 0 {
				break
			}
			err = n.SendSearchRequestMessage(neighbor, rid, origin, reg, base)
		}
		if err != nil {
			return
		}
		channel := make(chan bool)
		n.replyChannels.add(rid, &channel)
		select {
		case <-channel:
			// get reply
			n.replyChannels.remove(rid)
		case <-time.After(timeout):
			// no reply.
		}
	}
	return
}

// SendStatusMessage sends a data request packet to the given dst
func (n *node) SendDataRequestMessage(dst string, rid string, cid string) error {
	payload := types.DataRequestMessage{RequestID: rid, Key: cid}
	msg, err := n.CreateMsg(payload)
	if err != nil {
		return err
	}
	err = n.Unicast(dst, msg)
	return err
}

// SendDataReplyMessage sends a data reply packet to the given dst
func (n *node) SendDataReplyMessage(dst string, rid string, cid string, data []byte) error {
	payload := types.DataReplyMessage{RequestID: rid, Key: cid, Value: data}
	msg, err := n.CreateMsg(payload)
	if err != nil {
		return err
	}
	err = n.Unicast(dst, msg)
	return err
}

// SendSearchRequestMessage sends a search request packet to the given dst
func (n *node) SendSearchRequestMessage(dst string, rid string, origin string, reg regexp.Regexp, budget uint) error {
	payload := types.SearchRequestMessage{
		RequestID: rid, Origin: origin,
		Pattern: reg.String(), Budget: budget}
	msg, err := n.CreateMsg(payload)
	if err != nil {
		return err
	}
	err = n.SendToNeighbor(dst, msg)
	return err
}

// SendSearchReplyMessage sends a search reply packet to the given dst
func (n *node) SendSearchReplyMessage(dst string, rid string, fileinfos []types.FileInfo) error {
	payload := types.SearchReplyMessage{
		RequestID: rid, Responses: fileinfos}
	msg, err := n.CreateMsg(payload)
	if err != nil {
		return err
	}
	err = n.SendToNeighbor(dst, msg)
	return err
}
