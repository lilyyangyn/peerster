package impl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer/impl/paxos"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type PaxosModule struct {
	*node
	*sync.RWMutex
	cond *sync.Cond

	*paxos.MultiPaxos

	resultChan chan *types.PaxosValue
	hasNotify  bool

	cancelTimer context.CancelFunc
	hasSentTLC  bool
}

func NewPaxosModule(n *node) *PaxosModule {
	lock := sync.RWMutex{}
	m := PaxosModule{
		node:       n,
		RWMutex:    &lock,
		cond:       sync.NewCond(&lock),
		MultiPaxos: paxos.NewMultiPaxos(n.conf.Storage.GetBlockchainStore()),
		resultChan: make(chan *types.PaxosValue),
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

func (m *PaxosModule) InitTagConensus(name string, mh string) (err error) {
	if val := m.conf.Storage.GetNamingStore().Get(name); len(val) > 0 {
		return xerrors.Errorf("%s already in the name store.", name)
	}

	proposeVal := types.PaxosValue{
		UniqID:   xid.New().String(),
		Filename: name,
		Metahash: mh,
	}

	var step uint
	var success bool
	var channel chan *types.PaxosValue
	for {
		m.Lock()
		step, success = m.InitNewPaxos(&proposeVal, m.conf.PaxosID)
		if success {
			channel = m.resultChan
			m.Unlock()
			break
		}
		m.cond.Wait()
		m.Unlock()
	}

	m.startPaxosTimer(1)
	err = m.broadcastPaxosPrepareMessage(step, m.conf.PaxosID)
	if err != nil {
		return err
	}

	result := <-channel
	if result.Filename == name && result.Metahash == mh {
		// fmt.Println(m.conf.Socket.GetAddress(), "finishes")
		return nil
	}
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
	valid := m.RecordID(prepareMsg.Step, prepareMsg.ID)
	// check validity
	if !valid {
		m.Unlock()
		return nil
	}

	isAccept, acceptID, accpetValue := m.GetAcceptedInfo()
	m.Unlock()

	// respond with promise message
	if isAccept {
		err = m.broadcastPaxosPromiseMessage(
			prepareMsg.Source,
			prepareMsg.Step,
			prepareMsg.ID,
			acceptID,
			accpetValue,
		)
	} else {
		err = m.broadcastPaxosPromiseMessage(
			prepareMsg.Source,
			prepareMsg.Step,
			prepareMsg.ID,
			0,
			nil,
		)
	}

	return err
}

// ProcessPaxosPromiseMessage is a callback function to handle received paxos promise message
func (m *PaxosModule) ProcessPaxosPromiseMessage(msg types.Message, pkt transport.Packet) (err error) {
	promiseMsg, ok := msg.(*types.PaxosPromiseMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	m.Lock()
	val, success := m.RecordPromise(
		promiseMsg.Step,
		promiseMsg.ID,
		promiseMsg.AcceptedID,
		promiseMsg.AcceptedValue,
		m.conf.PaxosThreshold(m.conf.TotalPeers),
	)
	m.Unlock()

	if success {
		// set timer
		m.startPaxosTimer(1)
		err = m.broadcastPaxosProposeMessage(promiseMsg.Step, promiseMsg.ID, val)
		// fmt.Println(m.conf.Socket.GetAddress(), "proposes")
	}

	return err
}

// ProcessPaxosProposeMessage is a callback function to handle received paxos propose message
func (m *PaxosModule) ProcessPaxosProposeMessage(msg types.Message, pkt transport.Packet) (err error) {
	proposeMsg, ok := msg.(*types.PaxosProposeMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	// check validity
	m.Lock()
	valid := m.Accept(proposeMsg.Step, proposeMsg.ID, &proposeMsg.Value)
	m.Unlock()
	if !valid {
		return nil
	}

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
	block, success := m.RecordAccept(
		acceptMsg.Step,
		acceptMsg.ID,
		&acceptMsg.Value,
		m.conf.PaxosThreshold(m.conf.TotalPeers),
	)
	m.Unlock()

	if success {
		m.stopPaxosTimer()
		m.sendResult(&acceptMsg.Value)
		err = m.broadcastTLCMessage(acceptMsg.Step, block)
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
	success, catchUp := m.RecordBlock(
		tlcMsg.Step,
		&tlcMsg.Block,
		m.conf.PaxosThreshold(m.conf.TotalPeers),
	)

	if success {
		m.stopPaxosTimer()
		err = m.nextPaxos(tlcMsg.Step, &tlcMsg.Block, catchUp)
		if err == nil {
			m.cond.Signal()
		}
	}

	return err
}

/** Private Helpfer Functions **/

func (m *PaxosModule) startPaxosTimer(exp uint) {
	ctx, cancel := context.WithCancel(context.Background())
	m.Lock()
	if m.cancelTimer != nil {
		m.cancelTimer()
	}
	m.cancelTimer = cancel
	m.Unlock()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(m.conf.PaxosProposerRetry * time.Duration(exp)):
			// exponential backoff
			m.retryPrepare(exp * 2)
		}
	}()

}

func (m *PaxosModule) stopPaxosTimer() {
	if m.cancelTimer != nil {
		m.cancelTimer()
		m.cancelTimer = nil
	}
}

func (m *PaxosModule) sendResult(val *types.PaxosValue) {
	if m.IsProposer() && !m.hasNotify {
		m.resultChan <- val
		m.hasNotify = true
	}
}

func (m *PaxosModule) retryPrepare(exp uint) (err error) {
	m.Lock()
	step, id, success := m.RetryPaxos(m.conf.TotalPeers)
	fmt.Printf("-----%s: %d-----\n", m.conf.Socket.GetAddress(), m.TLC)
	m.Unlock()

	if success {
		m.startPaxosTimer(exp)
		err = m.broadcastPaxosPrepareMessage(step, id)
		if err != nil {
			return err
		}

	}
	return nil
}

func (m *PaxosModule) nextPaxos(step uint, block *types.BlockchainBlock, catchUp bool) (err error) {
	fmt.Printf("%s: %d\n", m.conf.Socket.GetAddress(), step)

	m.AppendBlock(block)
	m.conf.Storage.GetNamingStore().Set(block.Value.Filename, []byte(block.Value.Metahash))

	if !m.hasSentTLC && !catchUp {
		go func() {
			err = m.broadcastTLCMessage(step, block)
			if err != nil {
				return
			}
		}()
	}

	m.hasNotify = false
	m.hasSentTLC = false
	m.resultChan = make(chan *types.PaxosValue)
	nextBlocks, success := m.AdvanceClock(block)
	if !success {
		// TODO: CHECK STRANGE CASE
		return nil
	}

	// catchup
	for _, block := range nextBlocks {
		success, catchUp = m.RecordBlock(block.Index, block, m.conf.PaxosThreshold(m.conf.TotalPeers))
		if success {
			err = m.nextPaxos(block.Index, block, catchUp)
			return err
		}
	}

	return nil
}

// broadcastPaxosPrepareMessage sends a paxos prepare message in private message in rumor
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

// broadcastPromiseMessage sends a paxos promise message in private message in rumor
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

// broadcastPaxosProposeMessage sends a paxos propose message in rumor
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

// broadcastPaxosAcceptMessage sends a paxos accept message in rumor
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

// broadcastTLCMessage sends a TLC message in rumor
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
	if err != nil {
		return err
	}

	m.Lock()
	m.hasSentTLC = true
	m.Unlock()

	return err
}
