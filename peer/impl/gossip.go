package impl

import (
	"math/rand"
	"time"

	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type GossipModule struct {
	*node
	rumorsTable     SafeRumorsTable
	timerController TimerController
}

func NewGossipModule(n *node) *GossipModule {
	m := GossipModule{
		node:            n,
		rumorsTable:     *NewSafeRumorsTable(),
		timerController: *NewTimeController(),
	}

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.PrivateMessage{}, m.ProcessPrivateMsg)
	m.conf.MessageRegistry.RegisterMessageCallback(types.StatusMessage{}, m.ProcessStatusMsg)
	m.conf.MessageRegistry.RegisterMessageCallback(types.RumorsMessage{}, m.ProcessRumorsMsg)
	m.conf.MessageRegistry.RegisterMessageCallback(types.AckMessage{}, m.ProcessAckMsg)
	m.conf.MessageRegistry.RegisterMessageCallback(types.EmptyMessage{}, m.ProcessEmptyMsg)

	return &m
}

/** Feature Functions **/

// Broadcast implements peer.Messaging
func (m *GossipModule) Broadcast(msg transport.Message) error {
	// sendout the message in rumor
	rumor := m.CreateRumor(&msg)
	err := m.SendRumorsMessage("", &[]types.Rumor{rumor})
	if err != nil {
		return err
	}

	go func() {
		// process the message locally
		header := transport.NewHeader(
			m.conf.Socket.GetAddress(),
			m.conf.Socket.GetAddress(),
			m.conf.Socket.GetAddress(),
			0)
		pkt := transport.Packet{Header: &header, Msg: &msg}
		err := m.conf.MessageRegistry.ProcessPacket(pkt)
		if err != nil {
			return
			// return err
		}
	}()

	return nil
}

/** Message Handler **/

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
	err := m.SendAckMessage(pkt.Header.RelayedBy, pkt.Header.PacketID)
	if err != nil {
		return err
	}
	// send RumorsMsg to a random neighbor
	if toNeighbor {
		err = m.SendDirectMessageWithACK(map[string]struct{}{pkt.Header.RelayedBy: {}}, *pkt.Msg)
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
	rumors, catchUp := m.CheckSyncStatus(statusMsg)

	if catchUp {
		// send a status message to the remote peer
		err := m.SendStatusMessage(pkt.Header.RelayedBy)
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
		return m.ContinueMongering(pkt.Header.RelayedBy)
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
	m.CancelTimer(pkt.Header.PacketID)
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

/** Private Helpfer Functions **/

// CreateRumor creates a new rumor with expected seqID and add it to rumorsTable
func (m *GossipModule) CreateRumor(msg *transport.Message) types.Rumor {
	m.rumorsTable.Lock()
	defer m.rumorsTable.Unlock()
	expectedSeq := uint(len(m.rumorsTable.table[m.conf.Socket.GetAddress()])) + 1
	rumor := types.Rumor{
		Origin:   m.conf.Socket.GetAddress(),
		Sequence: expectedSeq,
		Msg:      msg,
	}
	m.rumorsTable.table[rumor.Origin] = append(m.rumorsTable.table[rumor.Origin], rumor)
	return rumor
}

// CancelTimer cancels the registed timer based on packetID
func (m *GossipModule) CancelTimer(pktID string) {
	done, ok := m.timerController.getAndRemove(pktID)
	if ok {
		done <- struct{}{}
	}
}

// SendHeartbeatMessage sends a heartbeat message with the given payload
func (m *GossipModule) SendHeartbeatMessage(payload types.Message) error {
	msg, err := m.conf.MessageRegistry.MarshalMessage(payload)
	if err != nil {
		return err
	}
	return m.Broadcast(msg)
}

// SendAckMessage sends an ACK packet to neighbor
func (m *GossipModule) SendAckMessage(dst string, ACKPktID string) error {
	payload := types.AckMessage{AckedPacketID: ACKPktID, Status: types.StatusMessage(m.rumorsTable.getStatus())}
	msg, err := m.conf.MessageRegistry.MarshalMessage(payload)
	if err != nil {
		return err
	}
	err = m.SendToNeighbor(dst, msg)
	return err
}

// SendStatusMessage sends an status packet to neighbor
func (m *GossipModule) SendStatusMessage(dst string) error {
	payload := types.StatusMessage(m.rumorsTable.getStatus())
	msg, err := m.conf.MessageRegistry.MarshalMessage(payload)
	if err != nil {
		return err
	}
	err = m.SendToNeighbor(dst, msg)
	return err
}

// SendRumorsMessage sends an rumors packet with ack
func (m *GossipModule) SendRumorsMessage(src string, rumors *[]types.Rumor) error {
	payload := types.RumorsMessage{Rumors: *rumors}
	msg, err := m.conf.MessageRegistry.MarshalMessage(payload)
	if err != nil {
		return err
	}
	return m.SendDirectMessageWithACK(map[string]struct{}{src: {}}, msg)
}

// SendDirectMessageWithACK sends a message to neighbor and wait for ack asynchronously
func (m *GossipModule) SendDirectMessageWithACK(exclude map[string]struct{}, msg transport.Message) (err error) {
	neighbor, ok := m.GetRandomNeighbor(exclude)
	if !ok {
		return
	}
	header := transport.NewHeader(
		m.conf.Socket.GetAddress(),
		m.conf.Socket.GetAddress(),
		neighbor,
		0)
	pkt := transport.Packet{Header: &header, Msg: &msg}
	err = m.conf.Socket.Send(neighbor, pkt, WriteTimeout)
	if err != nil {
		return err
	}

	if m.conf.AckTimeout == 0 {
		// no timer will be set
		return nil
	}
	exclude[neighbor] = struct{}{}
	done := make(chan struct{}, 2)
	m.timerController.add(pkt.Header.PacketID, done)
	go func() {
		select {
		case <-done:
		case <-time.After(m.conf.AckTimeout):
			m.timerController.remove(pkt.Header.PacketID)
			_ = m.SendDirectMessageWithACK(exclude, msg)
		}
	}()
	return nil
}

// CheckSyncStatus compares the node status with the statusMessage for furthur syncing
func (m *GossipModule) CheckSyncStatus(statusMsg *types.StatusMessage) ([]types.Rumor, bool) {
	rumors := []types.Rumor{}
	catchUp := false
	for key, val := range *statusMsg {
		if m.rumorsTable.getExpectedSeq(key)-1 < val {
			// the remote peer has new rumors
			catchUp = true
		} else if m.rumorsTable.getExpectedSeq(key)-1 > val {
			// current noed has new rumors
			newRumors, ok := m.rumorsTable.getRumorsFrom(key, val+1)
			if ok {
				rumors = append(rumors, newRumors...)
			}
		}
	}
	myStatus := m.rumorsTable.getStatus()
	for key := range myStatus {
		_, ok := (*statusMsg)[key]
		if !ok {
			// in case the remote peer has no entire entry
			newRumors, _ := m.rumorsTable.getRumorsFrom(key, 1)
			rumors = append(rumors, newRumors...)
		}
	}
	return rumors, catchUp
}

// ContinueMongering implements the continue mongering mechanism
func (m *GossipModule) ContinueMongering(pktOrigin string) error {
	chance := rand.Float64()
	if chance > m.conf.ContinueMongering {
		// continue only with a certain probability
		return nil
	}
	randomNeighbor, ok := m.GetRandomNeighbor(map[string]struct{}{pktOrigin: {}})
	if ok {
		return m.SendStatusMessage(randomNeighbor)
	}
	return nil
}
