package message

import (
	"context"
	"math/rand"
	"time"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type GossipModule struct {
	*MessageModule
	conf            *peer.Configuration
	rumorsTable     SafeRumorsTable
	timerController TimerController
}

func NewGossipModule(conf *peer.Configuration, messageModue *MessageModule) *GossipModule {
	m := GossipModule{
		MessageModule:   messageModue,
		conf:            conf,
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
	rumor := m.createRumor(&msg)
	err := m.sendRumorsMessage("", &[]types.Rumor{rumor})
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

/** Daemon **/

// HeartBeatMecahnism implements heartbeat mechanism to periodically notify self
func (m *MessageModule) HeartBeatDaemon(ctx context.Context, interval time.Duration) error {
	if interval == 0 {
		// the heartbeat mechanism must not be activated
		return nil
	}
	heartbeatTicker := time.NewTicker(interval)
	err := m.sendHeartbeatMessage(types.EmptyMessage{})
	if err != nil {
		return err
	}

	go func() {
	out:
		for {
			select {
			case <-ctx.Done():
				heartbeatTicker.Stop()
				break out
			case <-heartbeatTicker.C:
				err := m.sendHeartbeatMessage(types.EmptyMessage{})
				if err != nil {
					continue
				}
			}
		}
	}()

	return nil
}

// AntiEntropyMechanism implements anti-entropy mechanism for gossip sync
func (m *MessageModule) AntiEntropyDaemon(ctx context.Context, interval time.Duration) error {
	if interval == 0 {
		// the anti-entropy mechanism must not be activated
		return nil
	}
	antiEntropyTicker := time.NewTicker(interval)

	go func() {
	out:
		for {
			select {
			case <-ctx.Done():
				antiEntropyTicker.Stop()
				break out
			case <-antiEntropyTicker.C:
				neighbor, ok := m.GetRandomNeighbor(map[string]struct{}{})
				if !ok {
					// no available neighbor
					continue
				}
				err := m.sendStatusMessage(neighbor)
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

/** Private Helpfer Functions **/

// createRumor creates a new rumor with expected seqID and add it to rumorsTable
func (m *GossipModule) createRumor(msg *transport.Message) types.Rumor {
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

// cancelTimer cancels the registed timer based on packetID
func (m *GossipModule) cancelTimer(pktID string) {
	done, ok := m.timerController.getAndRemove(pktID)
	if ok {
		done <- struct{}{}
	}
}

// sendHeartbeatMessage sends a heartbeat message with the given payload
func (m *GossipModule) sendHeartbeatMessage(payload types.Message) error {
	msg, err := m.conf.MessageRegistry.MarshalMessage(payload)
	if err != nil {
		return err
	}
	return m.Broadcast(msg)
}

// sendAckMessage sends an ACK packet to neighbor
func (m *GossipModule) sendAckMessage(dst string, ACKPktID string) error {
	payload := types.AckMessage{AckedPacketID: ACKPktID, Status: types.StatusMessage(m.rumorsTable.getStatus())}
	msg, err := m.conf.MessageRegistry.MarshalMessage(payload)
	if err != nil {
		return err
	}
	err = m.SendToNeighbor(dst, msg)
	return err
}

// sendStatusMessage sends an status packet to neighbor
func (m *GossipModule) sendStatusMessage(dst string) error {
	payload := types.StatusMessage(m.rumorsTable.getStatus())
	msg, err := m.conf.MessageRegistry.MarshalMessage(payload)
	if err != nil {
		return err
	}
	err = m.SendToNeighbor(dst, msg)
	return err
}

// sendRumorsMessage sends an rumors packet with ack
func (m *GossipModule) sendRumorsMessage(src string, rumors *[]types.Rumor) error {
	payload := types.RumorsMessage{Rumors: *rumors}
	msg, err := m.conf.MessageRegistry.MarshalMessage(payload)
	if err != nil {
		return err
	}
	return m.sendDirectMessageWithACK(map[string]struct{}{src: {}}, msg)
}

// sendDirectMessageWithACK sends a message to neighbor and wait for ack asynchronously
func (m *GossipModule) sendDirectMessageWithACK(exclude map[string]struct{}, msg transport.Message) (err error) {
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
			_ = m.sendDirectMessageWithACK(exclude, msg)
		}
	}()
	return nil
}

// checkSyncStatus compares the node status with the statusMessage for furthur syncing
func (m *GossipModule) checkSyncStatus(statusMsg *types.StatusMessage) ([]types.Rumor, bool) {
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

// continueMongering implements the continue mongering mechanism
func (m *GossipModule) continueMongering(pktOrigin string) error {
	chance := rand.Float64()
	if chance > m.conf.ContinueMongering {
		// continue only with a certain probability
		return nil
	}
	randomNeighbor, ok := m.GetRandomNeighbor(map[string]struct{}{pktOrigin: {}})
	if ok {
		return m.sendStatusMessage(randomNeighbor)
	}
	return nil
}
