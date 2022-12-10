package message

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

const ReadTimeout = time.Millisecond * 100
const WriteTimeout = time.Millisecond * 100

type MessageModule struct {
	conf *peer.Configuration

	routingTable SafeRoutingTable

	*GossipModule
	*EncryptionModule
}

func NewMessageModule(conf *peer.Configuration) *MessageModule {
	m := MessageModule{
		conf:         conf,
		routingTable: *NewSafeRoutingTable(conf.Socket.GetAddress()),
	}
	m.GossipModule = NewGossipModule(conf, &m)
	m.EncryptionModule = NewEncryptionModule(conf, &m)

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.ChatMessage{}, m.ProcessChatMessage)
	m.conf.MessageRegistry.RegisterMessageCallback(types.PrivateMessage{}, m.ProcessPrivateMsg)
	m.conf.MessageRegistry.RegisterMessageCallback(types.EmptyMessage{}, m.ProcessEmptyMsg)

	return &m
}

/** Feature Functions **/

// Unicast implements peer.Messaging
func (m *MessageModule) Unicast(dest string, msg transport.Message) error {
	header := transport.NewHeader(
		m.conf.Socket.GetAddress(),
		m.conf.Socket.GetAddress(),
		dest,
		0)
	pkt := transport.Packet{Header: &header, Msg: &msg}
	// Send the msg even if the dst is self
	nextPeer, err := m.getRoutingInfo(dest)
	if err != nil {
		return err
	}
	err = m.conf.Socket.Send(nextPeer, pkt, WriteTimeout)
	return err
}

// AddPeer implements peer.Service
func (m *MessageModule) AddPeer(addr ...string) {
	for _, peerAddr := range addr {
		// add self should have no effct
		if peerAddr == m.conf.Socket.GetAddress() {
			continue
		}
		// otherwise, update the routing table
		m.SetRoutingEntry(peerAddr, peerAddr)
	}
}

// GetRoutingTable implements peer.Service
func (m *MessageModule) GetRoutingTable() peer.RoutingTable {
	return m.routingTable.getAll()
}

// SetRoutingEntry implements peer.Service
func (m *MessageModule) SetRoutingEntry(origin, relayAddr string) {
	// Delete the record if no relayAddr
	if relayAddr == "" {
		m.routingTable.remove(origin)
		return
	}
	// Otherwise, update the table
	m.routingTable.add(origin, relayAddr)
}

// CreateMsg creates a new transport message for the given payload
func (m *MessageModule) CreateMsg(payload types.Message) (transport.Message, error) {
	data, err := json.Marshal(&payload)
	if err != nil {
		return transport.Message{}, err
	}
	msg := transport.Message{Type: payload.Name(), Payload: data}
	return msg, nil
}

// GetNeighbors returns a list of all neighbors
func (m *MessageModule) GetNeighbors(exclude map[string]struct{}) (neighbors []string) {
	m.routingTable.RLock()
	neighbors = []string{}
	for key, val := range m.routingTable.table {
		if key == m.conf.Socket.GetAddress() {
			continue
		}
		if _, ok := exclude[key]; ok {
			continue
		}
		if key == val {
			neighbors = append(neighbors, key)
		}
	}
	m.routingTable.RUnlock()

	return neighbors
}

// GetRandomNeighbor randomly returns a neighbor
func (m *MessageModule) GetRandomNeighbor(exclude map[string]struct{}) (string, bool) {
	neighbors := m.GetNeighbors(exclude)
	if len(neighbors) == 0 {
		return "", false
	}

	return neighbors[rand.Intn(len(neighbors))], true
}

// SendToNeighbor randomly select a neighbor and send the packet
func (m *MessageModule) SendToNeighbor(dest string, msg transport.Message) error {
	header := transport.NewHeader(
		m.conf.Socket.GetAddress(),
		m.conf.Socket.GetAddress(),
		dest,
		0)
	pkt := transport.Packet{Header: &header, Msg: &msg}
	err := m.conf.Socket.Send(dest, pkt, WriteTimeout)
	return err
}

/** Daemon **/

// MessagingDaemon starts a new loop to listen to the message
func (m *MessageModule) MessagingDaemon(ctx context.Context) error {
	go func() {
	out:
		for {
			select {
			case <-ctx.Done():
				// use context to determine when to stop the goroutine
				break out
			default:
				pkt, err := m.conf.Socket.Recv(ReadTimeout)
				if err != nil {
					continue
				}
				err = m.processPkt(pkt)
				if err != nil {
					// return
					continue
				}
			}
		}
	}()

	return nil
}

/** Private Helpfer Functions **/

// processPkt processes packet received
func (m *MessageModule) processPkt(pkt transport.Packet) error {
	pktDst := pkt.Header.Destination
	if pktDst == m.conf.Socket.GetAddress() {
		// use register to process the message if the node is dest
		// go func() {
		err := m.conf.MessageRegistry.ProcessPacket(pkt)
		if err != nil {
			return err
			// return
		}
		// }()
	} else {
		// relay to the next peer
		pkt.Header.RelayedBy = m.conf.Socket.GetAddress()
		nextPeer, err := m.getRoutingInfo(pktDst)
		if err != nil {
			// no routing information. Just drop the packet
			return err
		}
		err = m.conf.Socket.Send(nextPeer, pkt, WriteTimeout)
		if err != nil {
			return err
		}
	}
	return nil
}

// getRoutingInfo gets routing information from routing table or error if entry not exists
func (m *MessageModule) getRoutingInfo(dst string) (string, error) {
	nextHop, ok := m.routingTable.get(dst)
	if !ok {
		// no routing information. Just drop the packet
		return "", xerrors.Errorf("No routing information to %s", dst)
	}
	return nextHop, nil
}
