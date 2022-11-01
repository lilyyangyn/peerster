package impl

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

const ReadTimeout = time.Millisecond * 100
const WriteTimeout = time.Millisecond * 100

// NewPeer creates a new peer. You can change the content and location of this
// function but you MUST NOT change its signature and package location.
func NewPeer(conf peer.Configuration) peer.Peer {
	// here you must return a struct that implements the peer.Peer functions.
	// Therefore, you are free to rename and change it as you want.
	n := node{}
	n.conf = conf
	n.stopSig = nil
	n.routingTable = *NewSafeRoutingTable(n.conf.Socket.GetAddress())

	// gossip-related
	n.rumorsTable = *NewSafeRumorsTable()
	n.timerController = *NewTimeController()

	// datasharing-related
	n.catalog = *NewSafeCatalog()
	n.dataChannels = *NewSafeChannTable()

	n.RegisterMessageHandler()

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

	rumorsTable        SafeRumorsTable
	timerController    TimerController
	heartbeatTicker    *time.Ticker
	heartbeatStopSig   context.CancelFunc
	antiEntropyTicker  *time.Ticker
	antiEntropyStopSig context.CancelFunc

	catalog      SafeCatalog
	dataChannels SafeChannTable
}

/** Feature Functions **/

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
				pkt, err := n.conf.Socket.Recv(ReadTimeout)
				if err != nil {
					continue
				}
				err = n.ProcessPkt(pkt)
				if err != nil {
					continue
				}
			}
		}
	}(ctx)

	err := n.HeartBeatMecahnism(n.conf.HeartbeatInterval)
	if err != nil {
		return err
	}
	err = n.AntiEntropyMechanism(n.conf.AntiEntropyInterval)
	// return once ready to use
	return err
}

// Stop implements peer.Service
func (n *node) Stop() error {
	if n.stopSig == nil {
		return nil
	}

	if n.conf.HeartbeatInterval != 0 {
		n.heartbeatStopSig()
		n.heartbeatTicker.Stop()
	}
	if n.conf.AntiEntropyInterval != 0 {
		n.antiEntropyStopSig()
		n.antiEntropyTicker.Stop()
	}
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
	nextPeer, err := n.GetRoutingInfo(dest)
	if err != nil {
		return err
	}
	err = n.conf.Socket.Send(nextPeer, pkt, WriteTimeout)
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

/** Message Handler **/

// ProcessChatMessage is a callback function to handle received chat message
func (n *node) ProcessChatMessage(msg types.Message, pkt transport.Packet) error {
	// cast the message to its actual type. You assume it is the right type.
	chatMsg, ok := msg.(*types.ChatMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	log.Info().Msgf("%s received a chat message from: %s. Msg: %s",
		n.conf.Socket.GetAddress(),
		pkt.Header.Source,
		chatMsg.Message)
	return nil
}

/** Private Helpfer Functions **/

// ProcessPkt processes packet received
func (n *node) ProcessPkt(pkt transport.Packet) error {
	pktDst := pkt.Header.Destination
	if pktDst == n.conf.Socket.GetAddress() {
		// use register to process the message if the node is dest
		err := n.conf.MessageRegistry.ProcessPacket(pkt)
		if err != nil {
			return err
		}
	} else {
		// relay to the next peer
		pkt.Header.RelayedBy = n.conf.Socket.GetAddress()
		nextPeer, err := n.GetRoutingInfo(pktDst)
		if err != nil {
			// no routing information. Just drop the packet
			return err
		}
		err = n.conf.Socket.Send(nextPeer, pkt, WriteTimeout)
		if err != nil {
			return err
		}
	}
	return nil
}

// RegisterMessageHandler registers handlers for different message types
func (n *node) RegisterMessageHandler() {
	n.conf.MessageRegistry.RegisterMessageCallback(types.ChatMessage{}, n.ProcessChatMessage)

	// gossip-related
	n.conf.MessageRegistry.RegisterMessageCallback(types.PrivateMessage{}, n.ProcessPrivateMsg)
	n.conf.MessageRegistry.RegisterMessageCallback(types.StatusMessage{}, n.ProcessStatusMsg)
	n.conf.MessageRegistry.RegisterMessageCallback(types.RumorsMessage{}, n.ProcessRumorsMsg)
	n.conf.MessageRegistry.RegisterMessageCallback(types.AckMessage{}, n.ProcessAckMsg)

	// datasharing-related
	n.conf.MessageRegistry.RegisterMessageCallback(types.DataRequestMessage{}, n.ProcessDataRequestMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.DataReplyMessage{}, n.ProcessDataReplyMessage)
}

// GetRoutingInfo gets routing information from routing table or error if entry not exists
func (n *node) GetRoutingInfo(dst string) (string, error) {
	nextHop, ok := n.routingTable.get(dst)
	if !ok {
		// no routing information. Just drop the packet
		return "", xerrors.Errorf("No routing information to %s", dst)
	}
	return nextHop, nil
}

// CreateMsg creates a new transport message for the given payload
func (n *node) CreateMsg(payload types.Message) (transport.Message, error) {
	data, err := json.Marshal(&payload)
	if err != nil {
		return transport.Message{}, err
	}
	msg := transport.Message{Type: payload.Name(), Payload: data}
	return msg, nil
}
