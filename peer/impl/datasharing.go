package impl

import (
	"crypto"
	"encoding/hex"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// SafeCatalog implements a thread-safe catalog table
type SafeCatalog struct {
	*sync.RWMutex
	catalog peer.Catalog
}

func (c *SafeCatalog) add(key string, val string) {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.catalog[key]; ok {
		c.catalog[key][val] = struct{}{}
	} else {
		c.catalog[key] = map[string]struct{}{val: {}}
	}
}
func (c *SafeCatalog) getAll() peer.Catalog {
	catalog := peer.Catalog{}
	c.RLock()
	for key, value := range c.catalog {
		innerMap := make(map[string]struct{}, len(value))
		for innerKey, _ := range value {
			innerMap[innerKey] = struct{}{}
		}
		catalog[key] = innerMap
	}
	c.RUnlock()
	return catalog
}
func NewSafeCatalog() *SafeCatalog {
	catalog := SafeCatalog{&sync.RWMutex{}, peer.Catalog{}}
	return &catalog
}

// SafeChannTable implements a thread-safe channel table
type SafeChannTable struct {
	*sync.RWMutex
	channels map[string]*chan []byte
}

func (t SafeChannTable) add(key string, val *chan []byte) {
	t.Lock()
	defer t.Unlock()
	t.channels[key] = val
}
func (t SafeChannTable) remove(key string) {
	t.Lock()
	defer t.Unlock()
	delete(t.channels, key)
}
func (t *SafeChannTable) get(key string) (*chan []byte, bool) {
	t.RLock()
	val, ok := t.channels[key]
	t.RUnlock()
	return val, ok
}
func NewSafeChannTable() *SafeChannTable {
	channels := SafeChannTable{&sync.RWMutex{}, map[string]*chan []byte{}}
	return &channels
}

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

	rid := replyMsg.RequestID
	if channel, ok := n.dataChannels.get(rid); ok {
		*channel <- replyMsg.Value
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
		if provider, ok := n.GetRandomProvider(cid); ok {
			// TODO: wait for response
			channel := make(chan []byte)
			rid := xid.New().String()
			err = n.SendDataRequestMessage(provider, rid, cid, &channel)
			if err != nil {
				return
			}
			select {
			case data = <-channel:
				// get reply
				n.dataChannels.remove(rid)
				if data == nil {
					err = xerrors.Errorf("Empty value in response for %s", cid)
				}
				return
			case <-time.After(backoffInfo.Initial):
				// no reply. Exponential backoff
			}
		} else {
			err = xerrors.Errorf("No available provider for %s has been found.", cid)
			return
		}
	}
	err = xerrors.Errorf("Data request timed out for %s", cid)

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

// SendStatusMessage sends a data request packet to the given dst
func (n *node) SendDataRequestMessage(dst string, rid string, cid string, channel *chan []byte) error {
	payload := types.DataRequestMessage{RequestID: rid, Key: cid}
	msg, err := n.CreateMsg(payload)
	if err != nil {
		return err
	}
	n.dataChannels.add(rid, channel)
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
