package mpc

import (
	"log"

	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// initMPCConcensus inits a paxos to consensus on mpc value
func (m *MPCModule) initMPCConcensus(uniqID string, budget float64, expression string, prime string) (err error) {
	if m.paxos.Type != types.PaxosTypeMPC {
		return xerrors.Errorf("invalid operation")
	}

	// TODO: check balance

	if step, ok := m.paxos.CheckAndWait(); ok {
		return m.proposeMPC(uniqID, budget, expression, prime, step)
	}
	return m.initMPCConcensus(uniqID, budget, expression, prime)
}

/** Private Helpfer Functions **/

// proposeMPC starts a new paxos starting from phase one
func (m *MPCModule) proposeMPC(uniqID string, budget float64, expression string, prime string, step uint) error {
	// TODO: use public key hash?
	initiator := m.GetPubkey().N.String()
	proposeValContent := types.PaxosMPCValue{
		UniqID:     uniqID,
		Initiator:  initiator,
		Budget:     budget,
		Expression: expression,
		Prime:      prime,
	}
	proposeVal, err := types.CreatePaxosValue(proposeValContent)
	if err != nil {
		return err
	}

	result := m.paxos.StartFromPhaseOne(proposeVal, step)
	content, err := types.ParsePaxosValueContent(result.Value)
	if err != nil {
		return err
	}
	mpcValue, ok := content.(*types.PaxosMPCValue)
	if !ok {
		return xerrors.Errorf("wrong type: %T", content)
	}

	if mpcValue.Initiator == initiator && mpcValue.Expression == expression {
		return nil
	}
	return m.initMPCConcensus(uniqID, budget, expression, prime)
}

// mpcThreshold calculates the threshold to enter next paxos stage
func (m *MPCModule) mpcThreshold() int {
	return int(m.conf.TotalPeers)
}

// mpcCallback is a callback function that gets called when TLC advances
func (m *MPCModule) mpcCallback(value *types.PaxosValue) error {
	content, err := types.ParsePaxosValueContent(value)
	if err != nil {
		return err
	}
	mpcval, ok := content.(*types.PaxosMPCValue)
	if !ok {
		return xerrors.Errorf("paxosvalue wrong type: %T", content)
	}

	resultChan := make(chan MPCResult, 1)
	if m.conf.DisableMPC {
		mpc := MPC{id: mpcval.UniqID, resultChan: resultChan}
		m.Lock()
		m.mpcstore[mpc.id] = &mpc
		m.Unlock()
		log.Printf("%s: MPC stops", m.conf.Socket.GetAddress())
		resultChan <- MPCResult{result: 0, err: nil}
		return nil
	}

	// init MPC instance and start MPC
	log.Printf("%s: Consensus Reached!Start MPC!", m.conf.Socket.GetAddress())
	err = m.InitMPC(mpcval.UniqID, mpcval.Prime, mpcval.Initiator, mpcval.Expression, resultChan)
	if err != nil {
		return err
	}
	go func() {
		val, err := m.ComputeExpression(mpcval.UniqID, mpcval.Expression, mpcval.Prime)
		resultChan <- MPCResult{result: val, err: err}
	}()

	return nil
}
