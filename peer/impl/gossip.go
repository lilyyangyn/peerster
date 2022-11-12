package impl

import (
	"context"
	"math/rand"
	"time"

	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

/** Feature Functions **/

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
	err := n.conf.MessageRegistry.ProcessPacket(pkt)
	if err != nil {
		return err
	}
	return nil
}

// HeartBeatMecahnism implements heartbeat mechanism to periodically notify self
func (n *node) HeartBeatMecahnism(interval time.Duration, ctx context.Context) error {
	if interval == 0 {
		// the heartbeat mechanism must not be activated
		return nil
	}
	heartbeatTicker := time.NewTicker(interval)
	go func() {
		n.SendHeartbeatMessage(types.EmptyMessage{})
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeatTicker.C:
				err := n.SendHeartbeatMessage(types.EmptyMessage{})
				if err != nil {
					continue
				}
			}
		}
	}()
	return nil
}

// AntiEntropyMechanism implements anti-entropy mechanism for gossip sync
func (n *node) AntiEntropyMechanism(interval time.Duration, ctx context.Context) error {
	if interval == 0 {
		// the anti-entropy mechanism must not be activated
		return nil
	}
	antiEntropyTicker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-antiEntropyTicker.C:
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

/** Message Handler **/

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
	// send ACK
	err := n.SendAckMessage(pkt.Header.RelayedBy, pkt.Header.PacketID)
	if err != nil {
		return err
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
	return nil
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
		err := n.SendStatusMessage(pkt.Header.RelayedBy)
		return err
	}
	if len(rumors) > 0 {
		// send all the missing Rumors (increasing seqID) in a single RumorsMessage
		payload := types.RumorsMessage{Rumors: rumors}
		msg, err := n.CreateMsg(payload)
		if err != nil {
			return err
		}
		err = n.SendToNeighbor(pkt.Header.RelayedBy, msg)
		// no expect of ACK
		return err
	}
	if !catchUp && len(rumors) == 0 {
		// Both peers have the same view. ContinueMongering
		return n.ContinueMongering(pkt.Header.RelayedBy)
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
	newMsg, err := n.CreateMsg(&ACKkMsg.Status)
	if err != nil {
		return err
	}
	newPkt := transport.Packet{
		Header: pkt.Header,
		Msg:    &newMsg,
	}
	return n.conf.MessageRegistry.ProcessPacket(newPkt)
	// return n.ProcessStatusMsg(&ACKkMsg.Status, pkt)
}

// ProcessEmptyMsg takes the empty message and do nothing
func (n *node) ProcessEmptyMsg(msg types.Message, pkt transport.Packet) error {
	return nil
}

/** Private Helpfer Functions **/

// CreateRumor creates a new rumor with expected seqID and add it to rumorsTable
func (n *node) CreateRumor(msg *transport.Message) types.Rumor {
	n.rumorsTable.Lock()
	defer n.rumorsTable.Unlock()
	expectedSeq := uint(len(n.rumorsTable.table[n.conf.Socket.GetAddress()])) + 1
	rumor := types.Rumor{
		Origin:   n.conf.Socket.GetAddress(),
		Sequence: expectedSeq,
		Msg:      msg,
	}
	n.rumorsTable.table[rumor.Origin] = append(n.rumorsTable.table[rumor.Origin], rumor)
	return rumor
}

// RegisterTimer registers a timer to resend the packet after a certain period
func (n *node) RegisterTimer(pkt *transport.Packet, duration time.Duration) {
	if duration == 0 {
		// no timer will be set
		return
	}
	done := make(chan struct{})
	timer := time.NewTimer(duration)
	go func() {
		select {
		case <-done:
			return
		case <-timer.C:
			close(done)
			n.timerController.remove(pkt.Header.PacketID)
			neighbor, ok := n.GetRandomNeighbor(pkt.Header.Destination)
			if !ok {
				return
			}
			header := transport.NewHeader(
				n.conf.Socket.GetAddress(),
				n.conf.Socket.GetAddress(),
				neighbor,
				0)
			msg := pkt.Msg.Copy()
			myPkt := transport.Packet{Header: &header, Msg: &msg}
			n.RegisterTimer(&myPkt, n.conf.AckTimeout)
			err := n.conf.Socket.Send(neighbor, myPkt, WriteTimeout)
			if err != nil {
				return
			}
		}
	}()
	n.timerController.add(pkt.Header.PacketID, done)
}

// CancelTimer cancels the registed timer based on packetID
func (n *node) CancelTimer(pktID string) {
	done, ok := n.timerController.get(pktID)
	if ok {
		<-done
		close(done)
		n.timerController.remove(pktID)
	}
}

// SendHeartbeatMessage sends a heartbeat message with the given payload
func (n *node) SendHeartbeatMessage(payload types.Message) error {
	msg, err := n.CreateMsg(payload)
	if err != nil {
		return err
	}
	return n.Broadcast(msg)
}

// SendAckMessage sends an ACK packet to neighbor
func (n *node) SendAckMessage(dst string, ACKPktID string) error {
	payload := types.AckMessage{AckedPacketID: ACKPktID, Status: types.StatusMessage(n.rumorsTable.getStatus())}
	msg, err := n.CreateMsg(payload)
	if err != nil {
		return err
	}
	err = n.SendToNeighbor(dst, msg)
	return err
}

// SendStatusMessage sends an status packet to neighbor
func (n *node) SendStatusMessage(dst string) error {
	payload := types.StatusMessage(n.rumorsTable.getStatus())
	msg, err := n.CreateMsg(payload)
	if err != nil {
		return err
	}
	err = n.SendToNeighbor(dst, msg)
	return err
}

// SendRumorsMessage unicasts an rumors packet
func (n *node) SendRumorsMessage(dst string, rumors *[]types.Rumor) error {
	payload := types.RumorsMessage{Rumors: *rumors}
	msg, err := n.CreateMsg(payload)
	if err != nil {
		return err
	}
	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		dst,
		0)
	pkt := transport.Packet{Header: &header, Msg: &msg}
	n.RegisterTimer(&pkt, n.conf.AckTimeout)
	err = n.conf.Socket.Send(dst, pkt, WriteTimeout)
	return err
}

// CheckSyncStatus compares the node status with the statusMessage for furthur syncing
func (n *node) CheckSyncStatus(statusMsg *types.StatusMessage) ([]types.Rumor, bool) {
	rumors := []types.Rumor{}
	catchUp := false
	for key, val := range *statusMsg {
		if n.rumorsTable.getExpectedSeq(key)-1 < val {
			// the remote peer has new rumors
			catchUp = true
		} else if n.rumorsTable.getExpectedSeq(key)-1 > val {
			// current noed has new rumors
			newRumors, ok := n.rumorsTable.getRumorsFrom(key, val+1)
			if ok {
				rumors = append(rumors, newRumors...)
			}
		}
	}
	myStatus := n.rumorsTable.getStatus()
	for key := range myStatus {
		_, ok := (*statusMsg)[key]
		if !ok {
			// in case the remote peer has no entire entry
			newRumors, _ := n.rumorsTable.getRumorsFrom(key, 1)
			rumors = append(rumors, newRumors...)
		}
	}
	return rumors, catchUp
}

// ContinueMongering implements the continue mongering mechanism
func (n *node) ContinueMongering(pktOrigin string) error {
	chance := rand.Float64()
	if chance > n.conf.ContinueMongering {
		// continue only with a certain probability
		return nil
	}
	randomNeighbor, ok := n.GetRandomNeighbor(pktOrigin)
	if ok {
		return n.SendStatusMessage(randomNeighbor)
	}
	return nil
}
