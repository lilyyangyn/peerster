package impl

import (
	"encoding/json"
	"math/rand"

	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// ProcessPkt processes packet received
func (n *node) ProcessPkt(pkt transport.Packet) error {
	pktDst := pkt.Header.Destination
	if pktDst == n.conf.Socket.GetAddress() {
		// use register to process the message if the node is dest
		// go func() {
		err := n.conf.MessageRegistry.ProcessPacket(pkt)
		if err != nil {
			return err
			// return
		}
		// }()
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

// GetRoutingInfo gets routing information from routing table or error if entry not exists
func (n *node) GetRoutingInfo(dst string) (string, error) {
	nextHop, ok := n.routingTable.get(dst)
	if !ok {
		// no routing information. Just drop the packet
		return "", xerrors.Errorf("No routing information to %s", dst)
	}
	return nextHop, nil
}

// GetNeighbors returns a list of all neighbors
func (n *node) GetNeighbors(exclude map[string]struct{}) (neighbors []string) {
	n.routingTable.RLock()
	neighbors = []string{}
	for key, val := range n.routingTable.table {
		if key == n.conf.Socket.GetAddress() {
			continue
		}
		if _, ok := exclude[key]; ok {
			continue
		}
		if key == val {
			neighbors = append(neighbors, key)
		}
	}
	n.routingTable.RUnlock()

	return neighbors
}

// GetRandomNeighbor randomly returns a neighbor
func (n *node) GetRandomNeighbor(exclude map[string]struct{}) (string, bool) {
	neighbors := n.GetNeighbors(exclude)
	if len(neighbors) == 0 {
		return "", false
	}

	return neighbors[rand.Intn(len(neighbors))], true
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

// SendToNeighbor randomly select a neighbor and send the packet
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
