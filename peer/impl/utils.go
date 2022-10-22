package impl

import (
	"encoding/json"
	"math/rand"
	"sync"
	"time"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

/** Safe Structure **/

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

type RumorsTable map[string][]types.Rumor

// SafeRoutingTable implements a thread-safe routing table
type SafeRumorsTable struct {
	*sync.RWMutex
	table RumorsTable
}

func (t SafeRumorsTable) add(rumor types.Rumor) bool {
	t.Lock()
	defer t.Unlock()

	if uint(len(t.table[rumor.Origin]))+1 != rumor.Sequence {
		return false
	}
	t.table[rumor.Origin] = append(t.table[rumor.Origin], rumor)
	return true
}
func (t SafeRumorsTable) getExpectedSeq(key string) uint {
	t.RLock()
	rumors := t.table[key]
	t.RUnlock()
	return uint(len(rumors)) + 1
}
func (t SafeRumorsTable) getRumorsFrom(key string, seqID uint) ([]types.Rumor, bool) {
	rumors := []types.Rumor{}
	t.RLock()
	length := uint(len(t.table[key]))
	if seqID > length {
		return rumors, false
	}
	for i := seqID - 1; i < length; i++ {
		rumors = append(rumors, t.table[key][i])
	}
	t.RUnlock()
	return rumors, true
}
func (t SafeRumorsTable) getStatus() map[string]uint {
	statusTable := make(map[string]uint)
	t.RLock()
	for key, value := range t.table {
		statusTable[key] = uint(len(value))
	}
	t.RUnlock()
	return statusTable
}
func NewSafeRumorsTable() *SafeRumorsTable {
	rt := SafeRumorsTable{&sync.RWMutex{}, RumorsTable{}}
	return &rt
}

type TimerTable map[string]chan struct{}

// TimerController implements a thread-safe table for timer
type TimerController struct {
	*sync.RWMutex
	table TimerTable
}

func (t TimerController) add(pktID string, done chan struct{}) {
	t.Lock()
	defer t.Unlock()
	t.table[pktID] = done
}
func (t TimerController) remove(key string) {
	t.Lock()
	defer t.Unlock()
	delete(t.table, key)
}
func (t TimerController) get(key string) (chan struct{}, bool) {
	t.RLock()
	val, ok := t.table[key]
	t.RUnlock()
	return val, ok
}
func NewTimeController() *TimerController {
	rt := TimerController{&sync.RWMutex{}, TimerTable{}}
	return &rt
}

/** Private Helpfer Functions **/

// GetRoutingInfo gets routing information from routing table or error if entry not exists
func (n *node) GetRoutingInfo(dst string) (string, error) {
	nextHop, ok := n.routingTable.get(dst)
	if !ok {
		// no routing information. Just drop the packet
		return "", xerrors.Errorf("No routing information to %s", dst)
	}
	return nextHop, nil
}

// GetRandomNeighbor randomly returns a neighbor
func (n *node) GetRandomNeighbor(exclude string) (string, bool) {
	n.routingTable.RLock()
	neighbors := []string{}
	for key, val := range n.routingTable.table {
		if key == n.conf.Socket.GetAddress() {
			continue
		}
		if key == exclude {
			continue
		}
		if key == val {
			neighbors = append(neighbors, key)
		}
	}
	n.routingTable.RUnlock()
	if len(neighbors) == 0 {
		return "", false
	}
	return neighbors[rand.Intn(len(neighbors))], true
}

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

// CreateMsg creates a new transport message for the given payload
func (n *node) CreateMsg(payload types.Message) (transport.Message, error) {
	data, err := json.Marshal(&payload)
	if err != nil {
		return transport.Message{}, err
	}
	msg := transport.Message{Type: payload.Name(), Payload: data}
	return msg, nil
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
