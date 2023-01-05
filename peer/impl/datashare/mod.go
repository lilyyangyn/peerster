package datashare

import (
	"crypto"
	"encoding/hex"
	"io"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/peer/impl/paxos"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type DataSharingModule struct {
	*message.MessageModule
	conf *peer.Configuration

	catalog        SafeCatalog
	replyChannels  SafeChannTable
	messageRecords SafeMsgRecord

	*paxos.PaxosInstance
}

func NewDataSharingModule(conf *peer.Configuration, messageModule *message.MessageModule, paxosModule *paxos.PaxosModule) *DataSharingModule {
	m := DataSharingModule{
		MessageModule:  messageModule,
		conf:           conf,
		catalog:        *NewSafeCatalog(),
		replyChannels:  *NewSafeChannTable(),
		messageRecords: *NewSafeMsgRecord(),
	}
	instance, err := paxosModule.CreateNewPaxos(
		types.PaxosTypeTag,
		storage.TagLastBlockKey,
		m.tagThreshold,
		m.tagCallback,
	)
	if err != nil {
		panic(err)
	}
	m.PaxosInstance = instance

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.DataRequestMessage{}, m.ProcessDataRequestMessage)
	m.conf.MessageRegistry.RegisterMessageCallback(types.DataReplyMessage{}, m.ProcessDataReplyMessage)
	m.conf.MessageRegistry.RegisterMessageCallback(types.SearchRequestMessage{}, m.ProcessSearchRequestMessage)
	m.conf.MessageRegistry.RegisterMessageCallback(types.SearchReplyMessage{}, m.ProcessSearchReplyMessage)

	return &m
}

/** Feature Functions **/

// Upload implements peer.Upload
func (m *DataSharingModule) Upload(data io.Reader) (metahash string, err error) {
	metadata := make([]byte, 0)
	metakey := make([]byte, 0)
	blobStorage := m.conf.Storage.GetDataBlobStore()
	metadataSepBytes := []byte(peer.MetafileSep)
	for {
		// read chunk
		chunk := make([]byte, m.conf.ChunkSize)
		size, readErr := data.Read(chunk)
		if readErr == io.EOF || size == 0 {
			break
		} else if readErr != nil {
			err = readErr
			return metahash, err
		}

		// compute CID
		chunkHash, chunkCID := computeCID(chunk[:size])
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
	_, metahash = computeCID(metakey)
	blobStorage.Set(metahash, metadata)

	return metahash, err
}

// Download implements peer.Download
func (m *DataSharingModule) Download(metahash string) (data []byte, err error) {
	// get metadata
	metadata, readErr := m.getData(metahash)
	if readErr != nil {
		err = readErr
		return data, err
	}
	// get chunks
	chunkCIDs := strings.Split(string(metadata), peer.MetafileSep)
	data = make([]byte, 0)
	for _, chunkCID := range chunkCIDs {
		chunkData, readErr := m.getData(chunkCID)
		if readErr != nil {
			err = readErr
			return data, err
		}
		data = append(data, chunkData...)
	}

	return data, err
}

// Tag implements peer.Tag
func (m *DataSharingModule) Tag(name string, mh string) error {
	if m.conf.TotalPeers == 1 {
		m.conf.Storage.GetNamingStore().Set(name, []byte(mh))
		return nil
	}

	return m.initTagConcensus(name, mh)
}

// Resolve implements peer.Resolve
func (m *DataSharingModule) Resolve(name string) string {
	return string(m.conf.Storage.GetNamingStore().Get(name))
}

// GetCatalog implements peer.GetCatalog
func (m *DataSharingModule) GetCatalog() peer.Catalog {
	return m.catalog.getAll()
}

// UpdateCatalog implements peer.UpdateCatalog
func (m *DataSharingModule) UpdateCatalog(key string, peer string) {
	m.catalog.add(key, peer)
}

// SearchAll implements peer.SearchAll
func (m *DataSharingModule) SearchAll(reg regexp.Regexp, budget uint,
	timeout time.Duration) (names []string, err error) {
	// search in remote naming store
	rid := xid.New().String()
	err = m.requestRemoteNames(reg, m.conf.Socket.GetAddress(), budget, rid, timeout)
	if err != nil {
		return names, err
	}

	// search in local naming store
	names = make([]string, 0)
	regMatch := func(key string, val []byte) bool {
		if reg.MatchString(key) {
			names = append(names, key)
		}
		return true
	}
	m.conf.Storage.GetNamingStore().ForEach(regMatch)

	return names, err
}

// SearchFirst implements peer.SearchFirst
func (m *DataSharingModule) SearchFirst(pattern regexp.Regexp, conf peer.ExpandingRing) (name string, err error) {
	// check local
	success := false
	regMatch := func(key string, val []byte) bool {
		if pattern.MatchString(key) {
			if m.isFullyKnown(string(val)) {
				name = key
				success = true
				return false
			}
		}
		return true
	}
	m.conf.Storage.GetNamingStore().ForEach(regMatch)
	if success {
		return name, err
	}

	// check remote
	name, err = m.requestRemoteFullyKnownFile(pattern, conf)

	return name, err
}

/** Private Helpfer Functions **/

// computeCID computes the encoded hash of the given byte array
func computeCID(data []byte) (hash []byte, hashHex string) {
	h := crypto.SHA256.New()
	h.Write(data)
	hash = h.Sum(nil)
	hashHex = hex.EncodeToString(hash)

	return hash, hashHex
}

// fairBudget distributes budget as fairly as possible to pieces
func fairBudget(budget uint, pieces uint) (base uint, extra int) {
	base = budget / pieces
	extra = int(budget % pieces)

	return base, extra
}

// getData returns an byte araay of the given encoded hash
func (m *DataSharingModule) getData(cid string) (data []byte, err error) {
	var backoffMult uint = 1
	backoffInfo := m.conf.BackoffDataRequest
	provider, ok := m.getRandomProvider(cid)
	for i := 0; i < int(backoffInfo.Retry); i++ {
		// check local storage
		if content := m.conf.Storage.GetDataBlobStore().Get(cid); content != nil {
			data = content
			return data, err
		}

		if !ok {
			err = xerrors.Errorf("No available provider for %s has been found.", cid)
			return data, err
		}

		// sleep for exponential backoff
		if i > 0 {
			time.Sleep(backoffInfo.Initial * time.Duration(backoffMult-1))
			backoffMult *= backoffInfo.Factor
		}

		// send request to another peer
		data, err = m.requestRemoteData(cid, provider, backoffInfo.Initial)
		if err != nil || data != nil {
			return data, err
		}
	}
	err = xerrors.Errorf("Data request timed out for %s", cid)

	return data, err
}

// getLocalFileInfo constructs a list of fileinfo from local storage
func (m *DataSharingModule) getLocalFileInfo(name string, metahash string) (fileinfo types.FileInfo, ok bool) {
	metadata := m.conf.Storage.GetDataBlobStore().Get(metahash)
	if metadata == nil {
		ok = false
		return fileinfo, ok
	}

	// get chunks
	chunkCIDs := strings.Split(string(metadata), peer.MetafileSep)
	chunks := make([][]byte, 0)
	for _, chunkCID := range chunkCIDs {
		chunkData := m.conf.Storage.GetDataBlobStore().Get(chunkCID)
		if chunkData != nil {
			chunks = append(chunks, []byte(chunkCID))
		} else {
			chunks = append(chunks, nil)
		}
	}

	fileinfo = types.FileInfo{Name: name, Metahash: metahash, Chunks: chunks}
	ok = true

	return fileinfo, ok
}

// IsFullKnown checks if the peer has all chunks of the file locally
func (m *DataSharingModule) isFullyKnown(metahash string) (ok bool) {
	metadata := m.conf.Storage.GetDataBlobStore().Get(metahash)
	if metadata == nil {
		ok = false
		return ok
	}

	// get chunks
	chunkCIDs := strings.Split(string(metadata), peer.MetafileSep)
	for _, chunkCID := range chunkCIDs {
		chunkData := m.conf.Storage.GetDataBlobStore().Get(chunkCID)
		if chunkData == nil {
			ok = false
			return ok
		}
	}
	ok = true

	return ok
}

// GetRandomNeighbor randomly returns a neighbor
func (m *DataSharingModule) getRandomProvider(cid string) (provider string, ok bool) {
	m.catalog.RLock()
	providers := []string{}
	for key := range m.catalog.catalog[cid] {
		providers = append(providers, key)
	}
	m.catalog.RUnlock()
	if len(providers) == 0 {
		ok = false
		return provider, ok
	}
	provider, ok = providers[rand.Intn(len(providers))], true

	return provider, ok
}

// requestRemoteData sends a data request to a random provider and wait until timeout
func (m *DataSharingModule) requestRemoteData(cid string, provider string,
	timeout time.Duration) (data []byte, err error) {
	// wait for response
	rid := xid.New().String()
	err = m.sendDataRequestMessage(provider, rid, cid)
	if err != nil {
		return data, err
	}
	channel := make(chan bool)
	m.replyChannels.add(rid, &channel)
	select {
	case <-channel:
		// get reply
		m.replyChannels.remove(rid)
		data = m.conf.Storage.GetDataBlobStore().Get(cid)
		if data == nil {
			err = xerrors.Errorf("Empty value in response for %s", cid)
		}
	case <-time.After(timeout):
		// no reply.
		data = nil
	}

	return data, err
}

// requestRemoteNames requests remote peers for matched names
func (m *DataSharingModule) requestRemoteNames(reg regexp.Regexp, origin string,
	budget uint, rid string, timeout time.Duration) (err error) {
	neighbors := m.GetNeighbors(map[string]struct{}{origin: {}})
	rand.Shuffle(len(neighbors), func(i, j int) { neighbors[i], neighbors[j] = neighbors[j], neighbors[i] })
	if len(neighbors) == 0 {
		return nil
	}

	base, extra := fairBudget(budget, uint(len(neighbors)))
	var channel chan bool
	if timeout > 0 {
		channel = make(chan bool)
		m.replyChannels.add(rid, &channel)
	}

	for i, neighbor := range neighbors {
		myBudget := base
		if i < extra {
			myBudget = base + 1
		} else {
			if myBudget == 0 {
				break
			}
		}
		err = m.sendSearchRequestMessage(neighbor, rid, origin, reg, myBudget)
		if err != nil {
			return err
		}
	}
	if timeout > 0 {
		var replyNum uint
		for {
			select {
			case <-channel:
				// get reply
				replyNum++
				if replyNum == budget {
					m.replyChannels.remove(rid)
					return nil
				}
			case <-time.After(timeout):
				// no reply.
				m.replyChannels.remove(rid)
				return nil
			}
		}
	}

	return err
}

func (m *DataSharingModule) requestRemoteFullyKnownFile(reg regexp.Regexp,
	conf peer.ExpandingRing) (name string, err error) {
	// expanding-ring search
	var backoffMult uint = 1
	for i := 0; i < int(conf.Retry); i++ {
		// sleep for exponential backoff
		if i > 0 {
			time.Sleep(conf.Timeout * time.Duration(backoffMult-1))
			backoffMult *= conf.Factor
		}

		// expanding-ring search
		rid := xid.New().String()
		err = m.requestRemoteNames(reg, m.conf.Socket.GetAddress(), conf.Initial*backoffMult, rid, conf.Timeout)
		if err != nil {
			return name, err
		}

		// check local catalog
		success := false
		regMatch := func(key string, val map[string]struct{}) bool {
			if reg.MatchString(key) {
				if len(val) > 0 {
					name = key
					success = true
					return false
				}
			}
			return true
		}
		m.catalog.forEachFullyKnown(regMatch)
		if success {
			return name, err
		}
	}

	return name, err
}

// SendStatusMessage sends a data request packet to the given dst
func (m *DataSharingModule) sendDataRequestMessage(dst string, rid string, cid string) error {
	payload := types.DataRequestMessage{RequestID: rid, Key: cid}
	msg, err := m.CreateMsg(payload)
	if err != nil {
		return err
	}
	err = m.Unicast(dst, msg)
	return err
}

// sendDataReplyMessage sends a data reply packet to the given dst
func (m *DataSharingModule) sendDataReplyMessage(dst string, rid string, cid string, data []byte) error {
	payload := types.DataReplyMessage{RequestID: rid, Key: cid, Value: data}
	msg, err := m.CreateMsg(payload)
	if err != nil {
		return err
	}
	err = m.Unicast(dst, msg)
	return err
}

// sendSearchRequestMessage sends a search request packet to the given dst
func (m *DataSharingModule) sendSearchRequestMessage(dst string, rid string, origin string,
	reg regexp.Regexp, budget uint) error {
	payload := types.SearchRequestMessage{
		RequestID: rid, Origin: origin,
		Pattern: reg.String(), Budget: budget}
	msg, err := m.CreateMsg(payload)
	if err != nil {
		return err
	}
	err = m.SendToNeighbor(dst, msg)
	if err == nil {
		m.messageRecords.add(rid)
	}
	return err
}

// sendSearchReplyMessage sends a search reply packet to the given dst
func (m *DataSharingModule) sendSearchReplyMessage(dst string, nextHop string, rid string,
	fileinfos []types.FileInfo) error {
	payload := types.SearchReplyMessage{
		RequestID: rid, Responses: fileinfos}
	msg, err := m.CreateMsg(payload)
	if err != nil {
		return err
	}
	header := transport.NewHeader(
		m.conf.Socket.GetAddress(),
		m.conf.Socket.GetAddress(),
		dst,
		0)
	pkt := transport.Packet{Header: &header, Msg: &msg}
	err = m.conf.Socket.Send(nextHop, pkt, message.WriteTimeout)
	return err
}
