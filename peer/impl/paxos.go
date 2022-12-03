package impl

import (
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

	paxosPromiseChan chan PaxosResult
	paxosAcceptChan  chan PaxosResult
	paxosTLCAdvChan  chan PaxosResult

	hasSentTLC bool
}

func NewPaxosModule(n *node) *PaxosModule {
	lock := sync.RWMutex{}
	m := PaxosModule{
		node:            n,
		RWMutex:         &lock,
		cond:            sync.NewCond(&lock),
		MultiPaxos:      paxos.NewMultiPaxos(n.conf.Storage.GetBlockchainStore()),
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

	proposeVal := types.PaxosValue{
		UniqID:   xid.New().String(),
		Filename: name,
		Metahash: mh,
	}

	var step uint
	for {
		m.Lock()
		if m.StartPropose(&proposeVal, m.conf.PaxosID) {
			m.paxosPromiseChan = make(chan PaxosResult, 1)
			m.paxosAcceptChan = make(chan PaxosResult, 1)
			step = m.TLC
			m.Unlock()
			break
		}
		m.cond.Wait()
		m.Unlock()
	}

	return m.proposeTag(&proposeVal, m.conf.PaxosID, step)
}

func (m *PaxosModule) proposeTag(value *types.PaxosValue, id uint, step uint) error {
	name, mh := value.Filename, value.Metahash
	// fm.Println(m.conf.Socket.GetAddress(), "JOIN PHASE ONE!", id)
	phaseOneResult := m.phaseOne(id, step)
	if phaseOneResult.Err != nil {
		// fm.Println("{Phase 1}", phaseOneResult.Err)
		return phaseOneResult.Err
		// return m.InitTagConensus(name, mh)
	}
	if phaseOneResult.Retry {
		// fmt.Println(m.conf.Socket.GetAddress(), "{Phase 1}", "Retry")
		return m.proposeTag(value, id+m.conf.TotalPeers, step)
	}
	if phaseOneResult.Finish {
		// fm.Println(m.conf.Socket.GetAddress(), "{Phase 1}", "TLC ENDs")
		if phaseOneResult.Value.Filename == name && phaseOneResult.Value.Metahash == mh {
			// fmt.Println(m.conf.Socket.GetAddress(), "phase1: Finishes!")
			return nil
		}
		return m.InitTagConensus(name, mh)
	}

	// fm.Println(m.conf.Socket.GetAddress(), "JOIN PHASE TWO!")
	phaseTwoResult := m.phaseTwo(phaseOneResult.Value, id, phaseOneResult.Step)
	if phaseTwoResult.Err != nil {
		// fm.Println(m.conf.Socket.GetAddress(), "{Phase 2}", phaseTwoResult.Err)
		return phaseTwoResult.Err
		// return m.InitTagConensus(name, mh)
	}
	if phaseTwoResult.Retry {
		// fmt.Println(m.conf.Socket.GetAddress(), "{Phase 2}", "Retry")
		return m.proposeTag(value, id+m.conf.TotalPeers, step)
	}

	if phaseTwoResult.Value.Filename == name && phaseTwoResult.Value.Metahash == mh {
		// fmt.Println(m.conf.Socket.GetAddress(), "phase2: Finishes!")
		return nil
	}
	return m.InitTagConensus(name, mh)
}

func (m *PaxosModule) phaseOne(id uint, step uint) (result PaxosResult) {
	m.Lock()
	success := m.JoinPhaseOne(id)
	promiseChan := m.paxosPromiseChan
	acceptChan := m.paxosAcceptChan
	tlcAdvChan := m.paxosTLCAdvChan
	m.Unlock()

	if success {
		err := m.broadcastPaxosPrepareMessage(step, id)
		if err != nil {
			result.Err = err
			return result
		}
	}

	timer := time.NewTimer(m.conf.PaxosProposerRetry)
loop:
	for {
		select {
		case result = <-promiseChan:
		case result = <-tlcAdvChan:
			result.Finish = true
		case result = <-acceptChan:
			result.Finish = true
		case <-timer.C:
			result = PaxosResult{Retry: true}
			break loop
		}
		if result.Step == step {
			timer.Stop()
			break loop
		}
	}
	return result
}

func (m *PaxosModule) phaseTwo(val *types.PaxosValue, id uint, step uint) (result PaxosResult) {
	m.Lock()
	success := m.JoinPhaseTwo()
	acceptChan := m.paxosAcceptChan
	tlcAdvChan := m.paxosTLCAdvChan
	m.Unlock()

	if success {
		err := m.broadcastPaxosProposeMessage(step, id, val)
		if err != nil {
			return PaxosResult{Err: err}
		}
	}

	timer := time.NewTimer(m.conf.PaxosProposerRetry)
loop:
	for {
		select {
		case result = <-acceptChan:
			result.Finish = true
		case result = <-tlcAdvChan:
			result.Finish = true
		case <-timer.C:
			result = PaxosResult{Retry: true}
			break loop
		}
		if result.Step == step {
			timer.Stop()
			break loop
		}
	}

	return result
}

/** Message Handler **/

// ProcessPaxosPrepareMsg is a callback function to handle received paxos prepare message
func (m *PaxosModule) ProcessPaxosPrepareMsg(msg types.Message, pkt transport.Packet) (err error) {
	prepareMsg, ok := msg.(*types.PaxosPrepareMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	var valid bool
	// defer func() {
	// 	// fmt.Printf("%s---------%s----------Prepare End %t %d\n",
	//	//pkt.Header.PacketID, m.conf.Socket.GetAddress(), valid, prepareMsg.Step)
	// }()

	m.Lock()
	defer m.Unlock()
	// fmt.Printf("%s---------%s----------Prepare Start %d %d\n",
	// pkt.Header.PacketID, m.conf.Socket.GetAddress(), m.TLC, prepareMsg.Step)

	valid = m.RecordID(prepareMsg.Step, prepareMsg.ID)

	// check validity
	if !valid {
		return nil
	}

	isAccept, acceptID, accpetValue := m.GetAcceptedInfo()

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

	// var success bool
	// defer func() {
	// 	fmt.Printf("%s---------%s----------Promise End %t %d\n",
	//	pkt.Header.PacketID, m.conf.Socket.GetAddress(), success, promiseMsg.Step)
	// }()

	m.Lock()
	defer m.Unlock()
	// fmt.Printf("%s---------%s----------Promise Start\n", pkt.Header.PacketID, m.conf.Socket.GetAddress())

	value, success := m.RecordPromise(
		promiseMsg.Step,
		promiseMsg.ID,
		promiseMsg.AcceptedID,
		promiseMsg.AcceptedValue,
		m.conf.PaxosThreshold(m.conf.TotalPeers),
	)
	isProposer := m.IsProposer()
	channel := m.paxosPromiseChan

	if success {
		if isProposer {
			result := PaxosResult{
				Step:   promiseMsg.Step,
				Value:  value,
				Finish: false,
			}
			channel <- result
		}
	}

	return err
}

// ProcessPaxosProposeMessage is a callback function to handle received paxos propose message
func (m *PaxosModule) ProcessPaxosProposeMessage(msg types.Message, pkt transport.Packet) (err error) {
	proposeMsg, ok := msg.(*types.PaxosProposeMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	var valid bool
	// defer func() {
	// 	fmt.Printf("%s---------%s----------Propose End %t\n", pkt.Header.PacketID, m.conf.Socket.GetAddress(), valid)
	// }()

	// fmt.Printf("%s---------%s----------Propose Start\n", pkt.Header.PacketID, m.conf.Socket.GetAddress())

	// check validity
	m.Lock()
	defer m.Unlock()
	valid = m.Accept(proposeMsg.Step, proposeMsg.ID, &proposeMsg.Value)

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

	// var success bool
	// defer func() {
	// 	fmt.Printf("%s---------%s----------Accept End %t\n", pkt.Header.PacketID, m.conf.Socket.GetAddress(), success)
	// }()

	m.Lock()
	defer m.Unlock()

	block, success := m.RecordAccept(
		acceptMsg.Step,
		acceptMsg.ID,
		&acceptMsg.Value,
		m.conf.PaxosThreshold(m.conf.TotalPeers),
	)

	if success {
		if m.IsProposer() {
			result := PaxosResult{
				Step:   acceptMsg.Step,
				Value:  &acceptMsg.Value,
				Finish: true,
			}
			m.paxosAcceptChan <- result
		}
		m.hasSentTLC = true
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

	// var success bool
	// defer func() {
	// 	fmt.Printf("%s---------%s----------TLC End %t %d\n",
	// 		pkt.Header.PacketID, m.conf.Socket.GetAddress(), success, tlcMsg.Step)
	// }()

	m.Lock()
	defer m.Unlock()
	// fmt.Printf("%s---------%s----------TLC Start %d\n", pkt.Header.PacketID, m.conf.Socket.GetAddress(), m.TLC)

	success, catchUp := m.RecordBlock(
		tlcMsg.Step,
		&tlcMsg.Block,
		m.conf.PaxosThreshold(m.conf.TotalPeers),
	)

	if success {
		if m.IsProposer() {
			result := PaxosResult{
				Step:   tlcMsg.Block.Index,
				Value:  &tlcMsg.Block.Value,
				Finish: true,
			}
			m.paxosTLCAdvChan <- result
		}

		err = m.nextPaxos(tlcMsg.Step, &tlcMsg.Block, catchUp)
		if err == nil {
			m.cond.Broadcast()
		}
	}

	return err
}

/** Private Helpfer Functions **/

func (m *PaxosModule) nextPaxos(step uint, block *types.BlockchainBlock, catchUp bool) (err error) {
	// fmt.Printf("%s: Clock %d\n", m.conf.Socket.GetAddress(), step)

	err = m.AppendBlock(block)
	if err != nil {
		return err
	}

	m.conf.Storage.GetNamingStore().Set(block.Value.Filename, []byte(block.Value.Metahash))

	if !m.hasSentTLC && !catchUp {
		err = m.broadcastTLCMessage(step, block)
		if err != nil {
			return
		}
	}

	nextBlocks, success := m.AdvanceClock()
	if !success {
		return nil
	}

	m.hasSentTLC = false

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

// broadcastPaxosPrepareMessage sends a paxos prepare message in private message in fmt
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

// broadcastPromiseMessage sends a paxos promise message in private message in fmt
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

// broadcastPaxosProposeMessage sends a paxos propose message in fmt
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

// broadcastPaxosAcceptMessage sends a paxos accept message in fmt
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

// broadcastTLCMessage sends a TLC message in fm
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

	return err
}
