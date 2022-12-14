package impl

import (
	"context"
	"io"
	"math/rand"
	"regexp"
	"time"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/datashare"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/transport"
)

// NewPeer creates a new peer. You can change the content and location of this
// function but you MUST NOT change its signature and package location.
func NewPeer(conf peer.Configuration) peer.Peer {
	// here you must return a struct that implements the peer.Peer functions.
	// Therefore, you are free to rename and change it as you want.
	n := node{
		conf:    conf,
		stopSig: nil,
	}

	n.message = message.NewMessageModule(&conf)
	n.datasharing = datashare.NewDataSharingModule(&conf, n.message)

	return &n
}

// node implements a peer to build a Peerster system
//
// - implements peer.Peer
type node struct {
	peer.Peer
	conf peer.Configuration

	message     *message.MessageModule
	datasharing *datashare.DataSharingModule

	stopSig context.CancelFunc
}

/** Feature Functions **/

// Start implements peer.Service
func (n *node) Start() error {
	//start a new loop to listen to the message (non-blocking)
	rand.Seed(time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	n.stopSig = cancel

	err := n.message.MessagingDaemon(ctx)
	if err != nil {
		return err
	}

	err = n.message.HeartBeatDaemon(ctx, n.conf.HeartbeatInterval)
	if err != nil {
		return err
	}

	err = n.message.AntiEntropyDaemon(ctx, n.conf.AntiEntropyInterval)
	if err != nil {
		return err
	}

	// return once ready to use
	return nil
}

// Stop implements peer.Service
func (n *node) Stop() error {
	if n.stopSig != nil {
		n.stopSig()
	}

	return nil
}

// Unicast implements peer.Messaging
func (n *node) Unicast(dest string, msg transport.Message) error {
	return n.message.Unicast(dest, msg)
}

// Broadcast implements peer.Messaging
func (n *node) Broadcast(msg transport.Message) error {
	return n.message.Broadcast(msg)
}

// AddPeer implements peer.Service
func (n *node) AddPeer(addr ...string) {
	for _, peerAddr := range addr {
		// add self should have no effct
		if peerAddr == n.conf.Socket.GetAddress() {
			continue
		}
		// otherwise, update the routing table
		n.SetRoutingEntry(peerAddr, peerAddr)
	}
}

// GetRoutingTable implements peer.Service
func (n *node) GetRoutingTable() peer.RoutingTable {
	return n.message.GetRoutingTable()
}

// SetRoutingEntry implements peer.Service
func (n *node) SetRoutingEntry(origin, relayAddr string) {
	n.message.SetRoutingEntry(origin, relayAddr)
}

// Upload implements peer.Upload
func (n *node) Upload(data io.Reader) (metahash string, err error) {
	return n.datasharing.Upload(data)
}

// Download implements peer.Download
func (n *node) Download(metahash string) (data []byte, err error) {
	return n.datasharing.Download(metahash)
}

// Tag implements peer.Tag
func (n *node) Tag(name string, mh string) error {
	return n.datasharing.Tag(name, mh)
}

// Resolve implements peer.Resolve
func (n *node) Resolve(name string) string {
	return n.datasharing.Resolve(name)
}

// GetCatalog implements peer.GetCatalog
func (n *node) GetCatalog() peer.Catalog {
	return n.datasharing.GetCatalog()
}

// UpdateCatalog implements peer.UpdateCatalog
func (n *node) UpdateCatalog(key string, peer string) {
	n.datasharing.UpdateCatalog(key, peer)
}

// SearchAll implements peer.SearchAll
func (n *node) SearchAll(reg regexp.Regexp, budget uint, timeout time.Duration) (names []string, err error) {
	return n.datasharing.SearchAll(reg, budget, timeout)
}

// SearchFirst implements peer.SearchFirst
func (n *node) SearchFirst(pattern regexp.Regexp, conf peer.ExpandingRing) (name string, err error) {
	return n.datasharing.SearchFirst(pattern, conf)
}
