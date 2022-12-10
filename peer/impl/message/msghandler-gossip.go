package message

import (
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// ProcessPrivateMsg is a callback function to handle received private message
func (m *GossipModule) ProcessPrivateMsg(msg types.Message, pkt transport.Packet) error {
	privateMsg, ok := msg.(*types.PrivateMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	_, ok = privateMsg.Recipients[m.conf.Socket.GetAddress()]
	if ok {
		// process the message if in the recipients list
		newPkt := transport.Packet{
			Header: pkt.Header,
			Msg:    privateMsg.Msg,
		}
		err := m.conf.MessageRegistry.ProcessPacket(newPkt)
		return err
	}
	return nil
}

// ProcessRumorsMsg is a callback function to handle received rumors message
func (m *GossipModule) ProcessRumorsMsg(msg types.Message, pkt transport.Packet) error {
	rumorsMsg, ok := msg.(*types.RumorsMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}
	toNeighbor := false
	// process RumorsMsg
	for _, rumor := range rumorsMsg.Rumors {
		// ignore unexpected rumor. Otherwise, update table
		if m.rumorsTable.add(rumor) {
			toNeighbor = true
			// update routing table
			// only update when the origin node is not neighbor
			m.routingTable.update(rumor.Origin, pkt.Header.RelayedBy)

			// process message
			newPkt := transport.Packet{
				Header: pkt.Header,
				Msg:    rumor.Msg,
			}
			err := m.conf.MessageRegistry.ProcessPacket(newPkt)
			if err != nil {
				return err
			}
		}
	}
	// send ACK
	err := m.sendAckMessage(pkt.Header.RelayedBy, pkt.Header.PacketID)
	if err != nil {
		return err
	}
	// send RumorsMsg to a random neighbor
	if toNeighbor {
		err = m.sendDirectMessageWithACK(map[string]struct{}{pkt.Header.RelayedBy: {}}, *pkt.Msg)
		if err != nil {
			return err
		}
	}
	return nil
}

// ProcessStatusMsg is a callback function to handle received status message
func (m *GossipModule) ProcessStatusMsg(msg types.Message, pkt transport.Packet) error {
	statusMsg, ok := msg.(*types.StatusMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}
	rumors, catchUp := m.checkSyncStatus(statusMsg)

	if catchUp {
		// send a status message to the remote peer
		err := m.sendStatusMessage(pkt.Header.RelayedBy)
		if err != nil {
			return err
		}
	}
	if len(rumors) > 0 {
		// send all the missing Rumors (increasing seqID) in a single RumorsMessage
		payload := types.RumorsMessage{Rumors: rumors}
		msg, err := m.conf.MessageRegistry.MarshalMessage(payload)
		if err != nil {
			return err
		}
		err = m.SendToNeighbor(pkt.Header.RelayedBy, msg)
		// no expect of ACK
		if err != nil {
			return err
		}
	}
	if !catchUp && len(rumors) == 0 {
		// Both peers have the same view. ContinueMongering
		return m.continueMongering(pkt.Header.RelayedBy)
	}
	return nil
}

// ProcessAckMsg is a callback function to handle received ack message
func (m *GossipModule) ProcessAckMsg(msg types.Message, pkt transport.Packet) error {
	ACKkMsg, ok := msg.(*types.AckMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}
	// stop timer
	m.cancelTimer(pkt.Header.PacketID)
	// process status message
	newMsg, err := m.conf.MessageRegistry.MarshalMessage(&ACKkMsg.Status)
	if err != nil {
		return err
	}
	newPkt := transport.Packet{
		Header: pkt.Header,
		Msg:    &newMsg,
	}
	return m.conf.MessageRegistry.ProcessPacket(newPkt)
}

// ProcessEmptyMsg takes the empty message and do nothing
func (m *GossipModule) ProcessEmptyMsg(msg types.Message, pkt transport.Packet) error {
	_, ok := msg.(*types.EmptyMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	return nil
}
