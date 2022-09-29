package impl

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// NewPeer creates a new peer. You can change the content and location of this
// function but you MUST NOT change its signature and package location.
func NewPeer(conf peer.Configuration) peer.Peer {
	// here you must return a struct that implements the peer.Peer functions.
	// Therefore, you are free to rename and change it as you want.
	n := node{}
	n.conf = conf
	n.stopSig = nil
	n.routingTable = *NewSafeRoutingTable(n.conf.Socket.GetAddress())

	// register handler
	n.conf.MessageRegistry.RegisterMessageCallback(types.ChatMessage{}, n.ExecChatMessage)
	return &n
}

// node implements a peer to build a Peerster system
//
// - implements peer.Peer
type node struct {
	peer.Peer
	conf peer.Configuration

	stopSig      context.CancelFunc
	routingTable SafeRoutingTable
}

// SafeRoutingTable implements a thread-safe routing table
type SafeRoutingTable struct {
	*sync.RWMutex
	table peer.RoutingTable
}

func (t SafeRoutingTable) add(key string, val string) {
	t.Lock()
	defer t.Unlock()
	t.table[key] = val
}

func (t SafeRoutingTable) remove(key string) {
	t.Lock()
	defer t.Unlock()
	delete(t.table, key)
}

func (t SafeRoutingTable) get(key string) (string, bool) {
	t.RLock()
	val, ok := t.table[key]
	t.RUnlock()
	return val, ok
}

func (t SafeRoutingTable) getAll() peer.RoutingTable {
	routingTable := peer.RoutingTable{}
	t.RLock()
	for key, value := range t.table {
		routingTable[key] = value
	}
	t.RUnlock()
	return routingTable
}

func NewSafeRoutingTable(addr string) *SafeRoutingTable {
	rt := SafeRoutingTable{&sync.RWMutex{}, peer.RoutingTable{}}
	rt.add(addr, addr)
	return &rt
}

// Start implements peer.Service
func (n *node) Start() error {
	//start a new loop to listen to the message (non-blocking)
	ctx, cancel := context.WithCancel(context.Background())
	n.stopSig = cancel
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				// use context to determine when to stop the goroutine
				return
			default:
				pkt, err := n.conf.Socket.Recv(time.Millisecond * 100)
				if err != nil {
					continue
				}

				pkt_dst := pkt.Header.Destination
				if pkt_dst == n.conf.Socket.GetAddress() {
					// use register to process the message if the node is dest
					err = n.conf.MessageRegistry.ProcessPacket(pkt)
					if err != nil {
						continue
					}
				} else {
					// relay to the next peer
					pkt.Header.RelayedBy = n.conf.Socket.GetAddress()
					next_peer, ok := n.routingTable.get(pkt_dst)
					if !ok {
						// no routing information. Just drop the packet
						continue
					}
					err = n.conf.Socket.Send(next_peer, pkt, time.Millisecond*100)
					if err != nil {
						continue
					}
				}
			}
		}
	}(ctx)

	// return once ready to use
	return nil
}

// Stop implements peer.Service
func (n *node) Stop() error {
	n.stopSig()
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
	next_peer, ok := n.routingTable.get(dest)
	if !ok {
		// no routing information. Just drop the packet
		return xerrors.Errorf("No routing information to %s", dest)
	}
	err := n.conf.Socket.Send(next_peer, pkt, time.Millisecond*100)
	return err
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

// callback function to handle received chat message
func (n *node) ExecChatMessage(msg types.Message, pkt transport.Packet) error {
	// cast the message to its actual type. You assume it is the right type.
	chatMsg, ok := msg.(*types.ChatMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	log.Info().Msgf("received a chat message from: %s. Msg: %s", pkt.Header.Source, chatMsg.Message)
	return nil
}
