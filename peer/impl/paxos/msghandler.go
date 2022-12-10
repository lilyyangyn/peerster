package paxos

import (
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

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
