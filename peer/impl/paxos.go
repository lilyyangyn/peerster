package impl

import (
	"encoding/hex"
	"sync"
	"time"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer/impl/paxos"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type PaxosModule struct {
	*node
	*sync.Mutex
	cond *sync.Cond

	*paxos.MultiPaxos

	paxosPromiseChan chan PaxosResult
	paxosAcceptChan  chan PaxosResult
	paxosTLCAdvChan  chan PaxosResult

	hasSentTLC bool
}

func NewPaxosModule(n *node) *PaxosModule {
	lock := sync.Mutex{}
	m := PaxosModule{
		node:            n,
		Mutex:           &lock,
		cond:            sync.NewCond(&lock),
		MultiPaxos:      paxos.NewMultiPaxos(n.conf.PaxosID),
		paxosTLCAdvChan: make(chan PaxosResult, 5),
	}

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.PaxosPrepareMessage{}, m.ProcessPaxosPrepareMsg)
	m.conf.MessageRegistry.RegisterMessageCallback(types.PaxosPromiseMessage{}, m.ProcessPaxosPromiseMessage)
	m.conf.MessageRegistry.RegisterMessageCallback(types.PaxosProposeMessage{}, m.ProcessPaxosProposeMessage)
	m.conf.MessageRegistry.RegisterMessageCallback(types.PaxosAcceptMessage{}, m.ProcessPaxosAcceptMessage)
	m.conf.MessageRegistry.RegisterMessageCallback(types.TLCMessage{}, m.ProcessTLCMsg)

	return &m
}

/** Feature Functions **/

type PaxosResult struct {
	Step   uint
	Value  *types.PaxosValue
	Retry  bool
	Finish bool
	Err    error
}

func (m *PaxosModule) InitTagConensus(name string, mh string) (err error) {
	if val := m.conf.Storage.GetNamingStore().Get(name); len(val) > 0 {
		return xerrors.Errorf("%s already in the name store.", name)
	}

	for {
		m.Lock()
		if m.MultiPaxosState == paxos.Idle && m.AcceptID == 0 {
			m.Proposer = true
			m.MultiPaxosState = paxos.Busy
			m.paxosPromiseChan = make(chan PaxosResult, 1)
			m.paxosAcceptChan = make(chan PaxosResult, 1)
			// m.paxosTLCAdvChan = make(chan PaxosResult, 5)
			step := m.TLC
			m.Unlock()
			return m.proposeTag(name, mh, step)
		}
		m.cond.Wait()
		m.Unlock()
	}
}

func (m *PaxosModule) proposeTag(name string, mh string, step uint) error {
	defer func() {
		m.Lock()
		m.Proposer = false
		m.MultiPaxosState = paxos.Idle
		m.cond.Signal()
		m.Unlock()
	}()

	proposeVal := types.PaxosValue{
		UniqID:   xid.New().String(),
		Filename: name,
		Metahash: mh,
	}

	result := m.phaseOne(&proposeVal, step)
	if result.Err != nil {
		return result.Err
	}

	if result.Value.Filename == name && result.Value.Metahash == mh {
		return nil
	}
	return m.InitTagConensus(name, mh)
}

func (m *PaxosModule) phaseOne(val *types.PaxosValue, step uint) (result PaxosResult) {
	// fmt.Println(m.conf.Socket.GetAddress(), "ENTER PHASE ONE")
	m.Lock()
	m.JoinPhaseOne()
	promiseChan := m.paxosPromiseChan
	id := m.ProposeID
	m.Unlock()

	err := m.broadcastPaxosPrepareMessage(step, id)
	if err != nil {
		return PaxosResult{Err: err}
	}

	for {
		select {
		case result = <-promiseChan:
			if result.Step == step {
				return m.phaseTwo(val, step)
			}
		case result = <-m.paxosTLCAdvChan:
			if result.Step == step {
				result.Finish = true
				return result
			}
		case <-time.After(m.conf.PaxosProposerRetry):
			m.Lock()
			m.PaxosState = paxos.Init
			m.ProposeID += m.conf.TotalPeers
			m.Unlock()
			return m.phaseOne(val, step)
		}
	}
}

func (m *PaxosModule) phaseTwo(val *types.PaxosValue, step uint) (result PaxosResult) {
	// fmt.Println(m.conf.Socket.GetAddress(), "ENTER PHASE TWO")
	m.Lock()
	m.JoinPhaseTwo()
	acceptChan := m.paxosAcceptChan
	id := m.ProposeID

	valueToPropose := val
	if m.ValueInPromise != nil {
		valueToPropose = m.ValueInPromise
	}
	m.Unlock()

	err := m.broadcastPaxosProposeMessage(step, id, valueToPropose)
	if err != nil {
		return PaxosResult{Err: err}
	}

	for {
		select {
		case result = <-acceptChan:
			if result.Step == step {
				result.Finish = true
				return result
			}
		case result = <-m.paxosTLCAdvChan:
			if result.Step == step {
				result.Finish = true
				return result
			}
		case <-time.After(m.conf.PaxosProposerRetry):
			m.Lock()
			m.PaxosState = paxos.Init
			m.ProposeID += m.conf.TotalPeers
			m.Unlock()
			return m.phaseOne(val, step)
		}
	}
}

/** Message Handler **/

// ProcessPaxosPrepareMsg is a callback function to handle received paxos prepare message
func (m *PaxosModule) ProcessPaxosPrepareMsg(msg types.Message, pkt transport.Packet) (err error) {
	prepareMsg, ok := msg.(*types.PaxosPrepareMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	m.Lock()
	defer m.Unlock()

	// ignore wrong step
	if prepareMsg.Step != m.TLC {
		return nil
	}

	// ignore ID is not greater than MaxID
	if prepareMsg.ID <= m.MaxID {
		return nil
	}

	// fmt.Print("Prepare:", m.conf.Socket.GetAddress())
	// defer fmt.Println(" -- Prepare End:", m.conf.Socket.GetAddress())

	// update MaxID
	m.MaxID = prepareMsg.ID

	// respond with promise message
	err = m.broadcastPaxosPromiseMessage(
		prepareMsg.Source,
		prepareMsg.Step,
		prepareMsg.ID,
		m.AcceptID,
		m.AcceptValue,
	)

	return err
}

// ProcessPaxosPromiseMessage is a callback function to handle received paxos promise message
func (m *PaxosModule) ProcessPaxosPromiseMessage(msg types.Message, pkt transport.Packet) (err error) {
	promiseMsg, ok := msg.(*types.PaxosPromiseMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	m.Lock()
	defer m.Unlock()

	// ignore incorrect step
	if promiseMsg.Step != m.TLC {
		return nil
	}

	// ignore if proposer not in phase one
	if !m.Proposer || m.PaxosState != paxos.PhaseOne {
		if promiseMsg.ID < m.ProposeID && promiseMsg.ID%m.conf.TotalPeers == m.conf.PaxosID {
			return nil
		}
	}

	// record promise
	m.PromiseCounter++
	if promiseMsg.AcceptedID > m.MaxIDInPromise {
		m.MaxIDInPromise = promiseMsg.AcceptedID
		m.ValueInPromise = promiseMsg.AcceptedValue
		// m.AcceptID = promiseMsg.AcceptedID
		// m.AcceptValue = promiseMsg.AcceptedValue
	}
	if m.PromiseCounter < m.conf.PaxosThreshold(m.conf.TotalPeers) {
		return nil
	}
	m.PromiseCounter = 0
	// success = true

	// notify proposer
	result := PaxosResult{
		Step:   promiseMsg.Step,
		Finish: false,
	}
	m.paxosPromiseChan <- result

	return err
}

// ProcessPaxosProposeMessage is a callback function to handle received paxos propose message
func (m *PaxosModule) ProcessPaxosProposeMessage(msg types.Message, pkt transport.Packet) (err error) {
	proposeMsg, ok := msg.(*types.PaxosProposeMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	m.Lock()
	defer m.Unlock()

	// ignore incorrect step
	if proposeMsg.Step != m.TLC {
		return nil
	}

	// ignore ID isn't equal to MaxID
	if proposeMsg.ID != m.MaxID {
		return nil
	}

	// fmt.Print("Propose:", m.conf.Socket.GetAddress())
	// defer fmt.Println(" -- Propose End:", m.conf.Socket.GetAddress())

	// accept proposed value
	// m.AcceptID = proposeMsg.ID
	// m.AcceptValue = &proposeMsg.Value

	// respond with accept message
	err = m.broadcastPaxosAcceptMessage(proposeMsg.Step, proposeMsg.ID, &proposeMsg.Value)

	return err
}

// ProcessPaxosAcceptMessage is a callback function to handle received paxos accept message
func (m *PaxosModule) ProcessPaxosAcceptMessage(msg types.Message, pkt transport.Packet) (err error) {
	acceptMsg, ok := msg.(*types.PaxosAcceptMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	m.Lock()
	defer m.Unlock()

	// ignore incorrect step
	if acceptMsg.Step != m.TLC {
		return nil
	}

	// ignore if proposer not in phase two - only our message
	if m.Proposer && m.PaxosState != paxos.PhaseTwo {
		if acceptMsg.ID < m.ProposeID && acceptMsg.ID%m.conf.TotalPeers == m.conf.PaxosID {
			return nil
		}
	}

	// fmt.Print("Accept:", m.conf.Socket.GetAddress())
	// var success bool
	// defer func() {
	// 	fmt.Println(" -- Accept End:", m.conf.Socket.GetAddress(), m.Proposer, success)
	// }()

	// record accept
	uniqID := acceptMsg.Value.UniqID
	m.AcceptCounter[uniqID]++
	if m.PaxosState == paxos.Complete || m.AcceptCounter[uniqID] < m.conf.PaxosThreshold(m.conf.TotalPeers) {
		return nil
	}
	m.PaxosState = paxos.Complete
	// success = true

	// update accept value
	m.AcceptID = acceptMsg.ID
	m.AcceptValue = &acceptMsg.Value

	// notify proposer
	if m.Proposer {
		result := PaxosResult{
			Step:   acceptMsg.Step,
			Value:  &acceptMsg.Value,
			Finish: true,
		}
		m.paxosAcceptChan <- result
	}

	// send TLC message
	block := m.CreateTLCBlock(&acceptMsg.Value, m.conf.Storage.GetBlockchainStore().Get(storage.LastBlockKey))
	err = m.broadcastTLCMessage(acceptMsg.Step, block)
	if err == nil {
		m.hasSentTLC = true
	}

	return err
}

// ProcessTLCMsg is a callback function to handle received tlc message
func (m *PaxosModule) ProcessTLCMsg(msg types.Message, pkt transport.Packet) (err error) {
	tlcMsg, ok := msg.(*types.TLCMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	m.Lock()
	defer m.Unlock()

	// ignore past step
	if tlcMsg.Step < m.TLC {
		return nil
	}

	// fmt.Print("TLC:", m.conf.Socket.GetAddress())
	// var success bool
	// defer func() {
	// 	fmt.Println(" -- TLC End:", m.conf.Socket.GetAddress(), m.Proposer, success)
	// }()

	// record block
	m.BlockCounter[tlcMsg.Step]++
	m.Blocks[tlcMsg.Step] = &tlcMsg.Block
	if tlcMsg.Step != m.TLC || m.BlockCounter[m.TLC] < m.conf.PaxosThreshold(m.conf.TotalPeers) {
		return nil
	}
	m.BlockCounter[m.TLC] = 0
	// success = true

	if m.Proposer {
		result := PaxosResult{
			Step:   tlcMsg.Block.Index,
			Value:  &tlcMsg.Block.Value,
			Finish: true,
		}
		m.paxosTLCAdvChan <- result
	}

	err = m.advanceSession(&tlcMsg.Block, false)

	return err
}

/** Private Helpfer Functions **/

func (m *PaxosModule) advanceSession(block *types.BlockchainBlock, catchUp bool) (err error) {
	// fmt.Printf("\n%s: Clock %d\n", m.conf.Socket.GetAddress(), m.TLC)

	// append block
	blockKey := hex.EncodeToString(block.Hash)
	buf, err := block.Marshal()
	if err != nil {
		return err
	}
	m.conf.Storage.GetBlockchainStore().Set(blockKey, buf)
	m.conf.Storage.GetBlockchainStore().Set(storage.LastBlockKey, block.Hash)

	// set naming store and notify proposer
	m.conf.Storage.GetNamingStore().Set(block.Value.Filename, []byte(block.Value.Metahash))

	// broadcast TLC message if needed
	if !m.hasSentTLC && !catchUp {
		err = m.broadcastTLCMessage(m.TLC, block)
		if err != nil {
			return err
		}
	}

	delete(m.BlockCounter, m.TLC)
	delete(m.Blocks, m.TLC)

	// increse TLC
	m.TLC++
	m.Paxos = paxos.NewPaxos(m.conf.PaxosID)
	m.MultiPaxosState = paxos.Idle
	m.cond.Signal()

	// do catchup
	if m.BlockCounter[m.TLC] >= m.conf.PaxosThreshold(m.conf.TotalPeers) {
		err = m.advanceSession(m.Blocks[m.TLC], true)
	}

	return err
}

// broadcastPaxosPrepareMessage sends a paxos prepare message in private message
func (m *PaxosModule) broadcastPaxosPrepareMessage(step uint, id uint) error {
	prepare := types.PaxosPrepareMessage{
		Step:   step,
		ID:     id,
		Source: m.conf.Socket.GetAddress(),
	}
	marshalPrepare, err := m.CreateMsg(prepare)
	if err != nil {
		return err
	}
	err = m.Broadcast(marshalPrepare)

	return err
}

// broadcastPromiseMessage sends a paxos promise message in private message
func (m *PaxosModule) broadcastPaxosPromiseMessage(recipient string, step uint, id uint,
	acceptedID uint, acceptedValue *types.PaxosValue) error {
	promise := types.PaxosPromiseMessage{
		Step:          step,
		ID:            id,
		AcceptedID:    acceptedID,
		AcceptedValue: acceptedValue,
	}
	marshalPromise, err := m.CreateMsg(promise)
	if err != nil {
		return err
	}
	private := types.PrivateMessage{
		Recipients: map[string]struct{}{recipient: {}},
		Msg:        &marshalPromise,
	}
	marshalPrivate, err := m.CreateMsg(private)
	if err != nil {
		return err
	}
	err = m.Broadcast(marshalPrivate)

	return err
}

// broadcastPaxosProposeMessage sends a paxos propose message
func (m *PaxosModule) broadcastPaxosProposeMessage(step uint, id uint, value *types.PaxosValue) error {
	propose := types.PaxosProposeMessage{
		Step:  step,
		ID:    id,
		Value: *value,
	}
	marshalPropose, err := m.CreateMsg(propose)
	if err != nil {
		return err
	}
	err = m.Broadcast(marshalPropose)

	return err
}

// broadcastPaxosAcceptMessage sends a paxos accept message
func (m *PaxosModule) broadcastPaxosAcceptMessage(step uint, id uint, value *types.PaxosValue) error {
	accept := types.PaxosAcceptMessage{
		Step:  step,
		ID:    id,
		Value: *value,
	}
	marshalAccept, err := m.CreateMsg(accept)
	if err != nil {
		return err
	}
	err = m.Broadcast(marshalAccept)

	return err
}

// broadcastTLCMessage sends a TLC message
func (m *PaxosModule) broadcastTLCMessage(step uint, block *types.BlockchainBlock) error {
	tlc := types.TLCMessage{
		Step:  step,
		Block: *block,
	}
	marshalTLC, err := m.CreateMsg(tlc)
	if err != nil {
		return err
	}
	err = m.Broadcast(marshalTLC)

	return err
}
