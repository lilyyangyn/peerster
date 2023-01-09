package message

import (
	"context"
	"math/rand"
	"time"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
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
func (m *GossipModule) HeartBeatDaemon(ctx context.Context, interval time.Duration) error {
	if interval == 0 {
		// the heartbeat mechanism must not be activated
		return nil
	}
	heartbeatTicker := time.NewTicker(interval)

	generateHeartbeatMsg := func() (msg types.Message) {
		msg, ok := m.createPubkeyMsg()
		if !ok {
			msg = &types.EmptyMessage{}
		}
		return msg
	}
	msg := generateHeartbeatMsg()

	err := m.sendHeartbeatMessage(msg)
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
				err := m.sendHeartbeatMessage(msg)
				if err != nil {
					continue
				}
			}
		}
	}()

	return nil
}

// AntiEntropyMechanism implements anti-entropy mechanism for gossip sync
func (m *GossipModule) AntiEntropyDaemon(ctx context.Context, interval time.Duration) error {
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
		return nil
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
