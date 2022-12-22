package paxos

import (
	"encoding/hex"
	"log"
	"sync"
	"time"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
)

type PaxosModule struct {
	*message.MessageModule
	conf *peer.Configuration

	valueType PaxosType

	*sync.Mutex
	cond *sync.Cond

	*Multipaxos
	threshold    func() int
	callback     func(*types.PaxosValue) error
	lastBlockKey string

	paxosPromiseChan chan paxosResult
	paxosTLCAdvChan  chan paxosResult

	hasSentTLC bool
}

type PaxosType int

const (
	PaxosTag PaxosType = iota
	PaxosMPC
)

func NewPaxosModule(conf *peer.Configuration, messageModule *message.MessageModule, paxosType PaxosType) *PaxosModule {
	lock := sync.Mutex{}
	m := PaxosModule{
		MessageModule: messageModule,
		conf:          conf,

		valueType:       paxosType,
		Mutex:           &lock,
		cond:            sync.NewCond(&lock),
		Multipaxos:      NewMultipaxos(conf.PaxosID),
		paxosTLCAdvChan: make(chan paxosResult, 50),
	}

	switch paxosType {
	case PaxosTag:
		{
			m.callback = m.tagCallback
			m.threshold = m.tagThreshold
			m.lastBlockKey = storage.LastBlockKey
			break
		}
	case PaxosMPC:
		{
			m.callback = m.mpcCallback
			m.threshold = m.mpcThreshold
			m.lastBlockKey = "MPCLastKey"
			break
		}
	default:
		log.Fatal("Unrecognized paxos type")
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

/** Private Helpfer Functions **/

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
	m.conf.Storage.GetBlockchainStore().Set(m.lastBlockKey, block.Hash)

	// do callback
	err = m.callback(&block.Value)
	if err != nil {
		return err
	}

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
	if m.BlockCounter[m.TLC] >= m.threshold() {
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
