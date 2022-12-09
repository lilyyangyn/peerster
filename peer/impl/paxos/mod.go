package paxos

import (
	"encoding/hex"
	"log"
	"sync"
	"time"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type PaxosModule struct {
	*message.MessageModule
	conf *peer.Configuration

	*sync.Mutex
	cond *sync.Cond

	*MultiPaxos

	paxosPromiseChan chan paxosResult
	paxosTLCAdvChan  chan paxosResult

	hasSentTLC bool
}

func NewPaxosModule(conf *peer.Configuration, messageModule *message.MessageModule) *PaxosModule {
	lock := sync.Mutex{}
	m := PaxosModule{
		MessageModule: messageModule,
		conf:          conf,

		Mutex:           &lock,
		cond:            sync.NewCond(&lock),
		MultiPaxos:      NewMultiPaxos(conf.PaxosID),
		paxosTLCAdvChan: make(chan paxosResult, 50),
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

type paxosResult struct {
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

	m.Lock()
	if !m.Occupied {
		m.Occupied = true
		m.Proposer = true
		m.paxosPromiseChan = make(chan paxosResult, 3)
		step := m.TLC
		m.Unlock()
		return m.proposeTag(name, mh, step)
	}
	m.cond.Wait()
	m.Unlock()
	return m.InitTagConensus(name, mh)
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
	if !m.Proposer || m.PaxosState != PhaseOne {
		if promiseMsg.ID != m.ProposeID {
			return nil
		}
	}

	// record promise
	m.PromiseCounter++
	if promiseMsg.AcceptedID > m.MaxIDInPromise {
		m.MaxIDInPromise = promiseMsg.AcceptedID
		m.ValueInPromise = promiseMsg.AcceptedValue
	}
	if m.PromiseCounter != m.conf.PaxosThreshold(m.conf.TotalPeers) {
		return nil
	}
	m.PromiseCounter = 0

	// notify proposer
	result := paxosResult{
		Step:   promiseMsg.Step,
		Finish: false,
	}
	m.paxosPromiseChan <- result

	// start phase two
	m.joinPhaseTwo()
	value := m.ValueInPromise
	if value == nil {
		value = m.ProposeValue
	}
	err = m.broadcastPaxosProposeMessage(promiseMsg.Step, promiseMsg.ID, value)

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

	// accept proposed value
	m.AcceptID = proposeMsg.ID
	m.AcceptValue = &proposeMsg.Value

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
	if m.Proposer && m.PaxosState != PhaseTwo {
		if acceptMsg.ID < m.ProposeID && acceptMsg.ID%m.conf.TotalPeers == m.conf.PaxosID {
			return nil
		}
	}

	// record accept
	uniqID := acceptMsg.Value.UniqID
	m.AcceptCounter[uniqID]++
	if m.AcceptCounter[uniqID] != m.conf.PaxosThreshold(m.conf.TotalPeers) {
		return nil
	}
	m.AcceptCounter[uniqID] = 0

	// update accept value
	m.AcceptID = acceptMsg.ID
	m.AcceptValue = &acceptMsg.Value

	// notify proposer
	// if m.Proposer {
	// 	result := PaxosResult{
	// 		Step:   acceptMsg.Step,
	// 		Value:  &acceptMsg.Value,
	// 		Finish: true,
	// 	}
	// 	m.paxosAcceptChan <- result
	// }

	// send TLC message
	block := m.createTLCBlock(&acceptMsg.Value,
		m.conf.Storage.GetBlockchainStore().Get(storage.LastBlockKey))
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

	// record block
	m.BlockCounter[tlcMsg.Step]++
	m.Blocks[tlcMsg.Step] = &tlcMsg.Block
	if tlcMsg.Step != m.TLC || m.BlockCounter[m.TLC] != m.conf.PaxosThreshold(m.conf.TotalPeers) {
		return nil
	}
	m.BlockCounter[m.TLC] = 0

	err = m.advanceSession(&tlcMsg.Block, false)

	return err
}

/** Private Helpfer Functions **/

// proposeTag starts a new paxos starting from phase one
func (m *PaxosModule) proposeTag(name string, mh string, step uint) error {
	proposeVal := types.PaxosValue{
		UniqID:   xid.New().String(),
		Filename: name,
		Metahash: mh,
	}

	result := m.startFromPhaseOne(&proposeVal, step)

	if result.Value.Filename == name && result.Value.Metahash == mh {
		return nil
	}
	return m.InitTagConensus(name, mh)
}

// startFromPhaseOne init phase one by sending a prepare message
func (m *PaxosModule) startFromPhaseOne(val *types.PaxosValue, step uint) (result paxosResult) {
	m.Lock()
	m.ProposeValue = val
	m.Proposer = true
	m.joinPhaseOne()
	promiseChan := m.paxosPromiseChan
	id := m.ProposeID
	m.Unlock()

	_ = m.broadcastPaxosPrepareMessage(step, id)

	timer := time.After(m.conf.PaxosProposerRetry)
	for {
	PaxosSelect:
		select {
		case result = <-promiseChan:
			if result.Step < step {
				break PaxosSelect
			}
			timer = time.After(m.conf.PaxosProposerRetry)
		case result = <-m.paxosTLCAdvChan:
			if result.Step < step {
				break PaxosSelect
			}
			m.Lock()
			if m.Occupied {
				m.Occupied = false
				m.cond.Signal()
			}
			m.Unlock()
			result.Finish = true
			return result
		case <-timer:
			m.Lock()
			m.PaxosState = Init
			m.ProposeID += m.conf.TotalPeers
			m.Unlock()
			return m.startFromPhaseOne(val, step)
		}
	}
}

// advanceSession advances TLC and renew the paxos
func (m *PaxosModule) advanceSession(block *types.BlockchainBlock, catchUp bool) (err error) {
	log.Printf("%s: Clock %d\n", m.conf.Socket.GetAddress(), m.TLC)

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
		m.hasSentTLC = true
		err = m.broadcastTLCMessage(m.TLC, block)
		if err != nil {
			return err
		}
	}

	if m.Proposer {
		result := paxosResult{
			Step:   block.Index,
			Value:  &block.Value,
			Finish: true,
		}
		m.paxosTLCAdvChan <- result
	}

	// increse TLC
	m.TLC++
	m.Paxos = NewPaxos(m.conf.PaxosID)
	m.Occupied = false
	// m.Proposer = false
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
