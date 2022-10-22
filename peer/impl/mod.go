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
	n.rumorsTable = *NewSafeRumorsTable()
	n.timerController = *NewTimeController()

	// register handler
	n.conf.MessageRegistry.RegisterMessageCallback(types.ChatMessage{}, n.ProcessChatMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.PrivateMessage{}, n.ProcessPrivateMsg)
	n.conf.MessageRegistry.RegisterMessageCallback(types.StatusMessage{}, n.ProcessStatusMsg)
	n.conf.MessageRegistry.RegisterMessageCallback(types.RumorsMessage{}, n.ProcessRumorsMsg)
	n.conf.MessageRegistry.RegisterMessageCallback(types.AckMessage{}, n.ProcessAckMsg)

	return &n
}

// node implements a peer to build a Peerster system
//
// - implements peer.Peer
type node struct {
	peer.Peer
	conf peer.Configuration

	stopSig         context.CancelFunc
	routingTable    SafeRoutingTable
	rumorsTable     SafeRumorsTable
	timerController TimerController

	heartbeatTicker    *time.Ticker
	heartbeatStopSig   context.CancelFunc
	antiEntropyTicker  *time.Ticker
	antiEntropyStopSig context.CancelFunc
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

func (n *node) SendToNeighbor(dest string, msg transport.Message) error {
	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		dest,
		0)
	pkt := transport.Packet{Header: &header, Msg: &msg}
	err := n.conf.Socket.Send(dest, pkt, WriteTimeout)
	return err
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
	// sendout the message in rumor
	rumor := n.CreateRumor(&msg)
	neighbor, ok := n.GetRandomNeighbor("")
	if ok {
		// no available neighbors
		err := n.SendRumorsMessage(neighbor, &[]types.Rumor{rumor})
		if err != nil {
			return err
		}
	}
	// process the message locally
	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		0)
	pkt := transport.Packet{Header: &header, Msg: &msg}
	return n.conf.MessageRegistry.ProcessPacket(pkt)
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

// HeartBeatMecahnism implements heartbeat mechanism to periodically notify self
func (n *node) HeartBeatMecahnism(interval time.Duration) error {
	if interval == 0 {
		// the heartbeat mechanism must not be activated
		return nil
	}
	n.heartbeatTicker = time.NewTicker(interval)
	ctx, cancel := context.WithCancel(context.Background())
	n.heartbeatStopSig = cancel
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-n.heartbeatTicker.C:
				payload := types.EmptyMessage{}
				data, err := json.Marshal(&payload)
				if err != nil {
					continue
				}
				msg := transport.Message{Type: payload.Name(), Payload: data}
				err = n.Broadcast(msg)
				if err != nil {
					continue
				}
			}
		}
	}()
	return nil
}

// AntiEntropyMechanism implements anti-entropy mechanism for gossip sync
func (n *node) AntiEntropyMechanism(interval time.Duration) error {
	if interval == 0 {
		// the anti-entropy mechanism must not be activated
		return nil
	}
	n.antiEntropyTicker = time.NewTicker(interval)
	ctx, cancel := context.WithCancel(context.Background())
	n.antiEntropyStopSig = cancel
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-n.antiEntropyTicker.C:
				neighbor, ok := n.GetRandomNeighbor("")
				if !ok {
					// no available neighbor
					continue
				}
				err := n.SendStatusMessage(neighbor)
				if err != nil {
					continue
				}
			}
		}
	}()

	return nil
}

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

// ProcessPrivateMsg is a callback function to handle received private message
func (n *node) ProcessPrivateMsg(msg types.Message, pkt transport.Packet) error {
	privateMsg, ok := msg.(*types.PrivateMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	_, ok = privateMsg.Recipients[n.conf.Socket.GetAddress()]
	if ok {
		// process the message if in the recipients list
		newPkt := transport.Packet{
			Header: pkt.Header,
			Msg:    privateMsg.Msg,
		}
		err := n.conf.MessageRegistry.ProcessPacket(newPkt)
		return err
	}
	return nil
}

// ProcessRumorsMsg is a callback function to handle received rumors message
func (n *node) ProcessRumorsMsg(msg types.Message, pkt transport.Packet) error {
	rumorsMsg, ok := msg.(*types.RumorsMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}
	toNeighbor := false
	// process RumorsMsg
	for _, rumor := range rumorsMsg.Rumors {
		// ignore unexpected rumor. Otherwise, update table
		if n.rumorsTable.add(rumor) {
			toNeighbor = true
			// update routing table
			oldRelay, ok := n.routingTable.get(rumor.Origin)
			if !ok || oldRelay != rumor.Origin {
				// only update when the origin node is not neighbor
				n.SetRoutingEntry(rumor.Origin, pkt.Header.RelayedBy)
			}

			// process message
			newPkt := transport.Packet{
				Header: pkt.Header,
				Msg:    rumor.Msg,
			}
			err := n.conf.MessageRegistry.ProcessPacket(newPkt)
			if err != nil {
				continue
			}
		}
	}
	// send RumorsMsg to a random neighbor
	if toNeighbor {
		neighbor, ok := n.GetRandomNeighbor(pkt.Header.RelayedBy)
		if ok {
			err := n.SendRumorsMessage(neighbor, &rumorsMsg.Rumors)
			if err != nil {
				return err
			}
		}
	}
	// send ACK
	err := n.SendAckMessage(pkt.Header.RelayedBy, pkt.Header.PacketID)
	return err
}

// ProcessStatusMsg is a callback function to handle received status message
func (n *node) ProcessStatusMsg(msg types.Message, pkt transport.Packet) error {
	statusMsg, ok := msg.(*types.StatusMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}
	rumors, catchUp := n.CheckSyncStatus(statusMsg)

	if catchUp {
		// send a status message to the remote peer
		err := n.SendStatusMessage(pkt.Header.Source)
		return err
	}
	if len(rumors) > 0 {
		// send all the missing Rumors (increasing seqID) in a single RumorsMessage
		payload := types.RumorsMessage{Rumors: rumors}
		msg, err := n.CreateMsg(payload)
		if err != nil {
			return err
		}
		err = n.SendToNeighbor(pkt.Header.Source, msg)
		// no expect of ACK
		return err
	}
	if !catchUp && len(rumors) == 0 {
		// Both peers have the same view. ContinueMongering
		return n.ContinueMongering(pkt.Header.Source)
	}
	return nil
}

// ProcessAckMsg is a callback function to handle received ack message
func (n *node) ProcessAckMsg(msg types.Message, pkt transport.Packet) error {
	ACKkMsg, ok := msg.(*types.AckMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}
	// stop timer
	n.CancelTimer(pkt.Header.PacketID)
	// process status message
	return n.ProcessStatusMsg(&ACKkMsg.Status, pkt)
}
