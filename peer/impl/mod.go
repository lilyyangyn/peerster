package impl

import (
	"context"
	"encoding/json"
	"math/rand"
	"sync"
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
func (t SafeRumorsTable) getRumorsFrom(key string, seqId uint) ([]types.Rumor, bool) {
	rumors := []types.Rumor{}
	t.RLock()
	length := uint(len(t.table[key]))
	if seqId > length {
		return rumors, false
	}
	for i := seqId - 1; i < length; i++ {
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

func (t TimerController) add(pktId string, done chan struct{}) {
	t.Lock()
	defer t.Unlock()
	t.table[pktId] = done
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

	n.HeartBeatMecahnism(n.conf.HeartbeatInterval)
	n.AntiEntropyMechanism(n.conf.AntiEntropyInterval)
	// return once ready to use
	return nil
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
	} else {
		return neighbors[rand.Intn(len(neighbors))], true
	}
}

// CreateRumor creates a new rumor with expected seqId and add it to rumorsTable
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
func (n *node) CancelTimer(pktId string) error {
	done, ok := n.timerController.get(pktId)
	if ok {
		<-done
		close(done)
		n.timerController.remove(pktId)
	}
	return nil
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

// ProcessChatMessage is a callback function to handle received chat message
func (n *node) ProcessChatMessage(msg types.Message, pkt transport.Packet) error {
	// cast the message to its actual type. You assume it is the right type.
	chatMsg, ok := msg.(*types.ChatMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	log.Info().Msgf("%s received a chat message from: %s. Msg: %s", n.conf.Socket.GetAddress(), pkt.Header.Source, chatMsg.Message)
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
		// if n.conf.Socket.GetAddress() == "127.0.0.1:1" {
		// 	fmt.Println(rumor.Origin, rumor.Sequence)
		// }
		if n.rumorsTable.add(rumor) {
			// if n.conf.Socket.GetAddress() == "127.0.0.1:1" {
			// 	fmt.Println("valid")
			// }
			toNeighbor = true
			// update routing table
			oldRelay, ok := n.routingTable.get(rumor.Origin)
			if !ok || oldRelay != rumor.Origin {
				// only update when the origin node is not neighbor
				n.routingTable.add(rumor.Origin, pkt.Header.RelayedBy)
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
		neighbor, ok := n.GetRandomNeighbor(pkt.Header.Source)
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
	for key, _ := range *&myStatus {
		_, ok = (*statusMsg)[key]
		if !ok {
			// in case the remote peer has no entire entry
			newRumors, _ := n.rumorsTable.getRumorsFrom(key, 1)
			rumors = append(rumors, newRumors...)
		}
	}

	if catchUp {
		// send a status message to the remote peer
		err := n.SendStatusMessage(pkt.Header.Source)
		return err
	}
	if len(rumors) > 0 {
		// send all the missing Rumors (increasing seqId) in a single RumorsMessage
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
		chance := rand.Float64()
		if chance > n.conf.ContinueMongering {
			// continue only with a certain probability
			return nil
		}
		randomNeighbor, ok := n.GetRandomNeighbor(pkt.Header.Source)
		if ok {
			payload := types.StatusMessage(myStatus)
			msg, err := n.CreateMsg(payload)
			if err != nil {
				return err
			}
			n.SendToNeighbor(randomNeighbor, msg)
		}
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
