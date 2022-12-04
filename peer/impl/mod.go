package impl

import (
	"context"
	"io"
	"math/rand"
	"regexp"
	"time"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
)

const ReadTimeout = time.Millisecond * 100
const WriteTimeout = time.Millisecond * 100

// NewPeer creates a new peer. You can change the content and location of this
// function but you MUST NOT change its signature and package location.
func NewPeer(conf peer.Configuration) peer.Peer {
	// here you must return a struct that implements the peer.Peer functions.
	// Therefore, you are free to rename and change it as you want.
	n := node{
		conf:    conf,
		stopSig: nil,
	}
	n.routingTable = *NewSafeRoutingTable(n.conf.Socket.GetAddress())

	n.ChatModule = NewChatModule(&n)
	n.GossipModule = NewGossipModule(&n)
	n.DataSharingModule = NewDataSharingModule(&n)

	return &n
}

// node implements a peer to build a Peerster system
//
// - implements peer.Peer
type node struct {
	peer.Peer
	conf peer.Configuration

	*ChatModule
	*GossipModule
	*DataSharingModule

	stopSig      context.CancelFunc
	routingTable SafeRoutingTable
}

/** Feature Functions **/

// Start implements peer.Service
func (n *node) Start() error {
	//start a new loop to listen to the message (non-blocking)
	rand.Seed(time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	n.stopSig = cancel

	err := n.MessagingDaemon(ctx)
	if err != nil {
		return err
	}

	err = n.HeartBeatDaemon(ctx, n.conf.HeartbeatInterval)
	if err != nil {
		return err
	}

	err = n.AntiEntropyDaemon(ctx, n.conf.AntiEntropyInterval)
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
	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		dest,
		0)
	pkt := transport.Packet{Header: &header, Msg: &msg}
	// Send the msg even if the dst is self
	nextPeer, err := n.GetRoutingInfo(dest)
	if err != nil {
		return err
	}
	err = n.conf.Socket.Send(nextPeer, pkt, WriteTimeout)
	return err
}

// Broadcast implements peer.Messaging
func (n *node) Broadcast(msg transport.Message) error {
	return n.GossipModule.Broadcast(msg)
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
	return n.routingTable.getAll()
}

// SetRoutingEntry implements peer.Service
func (n *node) SetRoutingEntry(origin, relayAddr string) {
	// Delete the record if no relayAddr
	if relayAddr == "" {
		n.routingTable.remove(origin)
		return
	}
	// Otherwise, update the table
	n.routingTable.add(origin, relayAddr)
}

// Upload implements peer.Upload
func (n *node) Upload(data io.Reader) (metahash string, err error) {
	return n.DataSharingModule.Upload(data)
}

// Download implements peer.Download
func (n *node) Download(metahash string) (data []byte, err error) {
	return n.DataSharingModule.Download(metahash)
}

// Tag implements peer.Tag
func (n *node) Tag(name string, mh string) error {
	return n.DataSharingModule.Tag(name, mh)
}

// Resolve implements peer.Resolve
func (n *node) Resolve(name string) string {
	return n.DataSharingModule.Resolve(name)
}

// GetCatalog implements peer.GetCatalog
func (n *node) GetCatalog() peer.Catalog {
	return n.DataSharingModule.GetCatalog()
}

// UpdateCatalog implements peer.UpdateCatalog
func (n *node) UpdateCatalog(key string, peer string) {
	n.DataSharingModule.UpdateCatalog(key, peer)
}

// SearchAll implements peer.SearchAll
func (n *node) SearchAll(reg regexp.Regexp, budget uint, timeout time.Duration) (names []string, err error) {
	return n.DataSharingModule.SearchAll(reg, budget, timeout)
}

// SearchFirst implements peer.SearchFirst
func (n *node) SearchFirst(pattern regexp.Regexp, conf peer.ExpandingRing) (name string, err error) {
	return n.DataSharingModule.SearchFirst(pattern, conf)
}
