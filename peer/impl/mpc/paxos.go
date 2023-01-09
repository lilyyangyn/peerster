package mpc

import (
	"fmt"
	"math/big"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/types"
)

// initMPCConcensus inits a paxos to consensus on mpc value
func (m *MPCModule) initMPCConcensus(uniqID string, budget float64, expression string, prime string) (err error) {
	if m.paxos.Type != types.PaxosTypeMPC {
		return fmt.Errorf("invalid operation")
	}

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
		return fmt.Errorf("wrong type: %T", content)
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
		return fmt.Errorf("paxosvalue wrong type: %T", content)
	}

	if m.conf.DisableMPC {
		mpc := NewMPC(mpcval.UniqID, big.Int{}, "", "")
		m.mpcCenter.RegisterMPC(mpc.id, mpc)
		m.mpcCenter.InformMPCStart(mpc.id)
		err = m.mpcCenter.InformMPCComplete(mpc.id, MPCResult{result: 0, err: nil})
		return err
	}

	// init MPC instance and start MPC
	log.Info().Msgf("%s: Consensus Reached!Start MPC!", m.conf.Socket.GetAddress())
	err = m.InitMPCWithPaxos(mpcval.UniqID, mpcval.Prime, mpcval.Initiator, mpcval.Expression)
	m.mpcCenter.InformMPCStart(mpcval.UniqID)
	if err != nil {
		err = m.mpcCenter.InformMPCComplete(mpcval.UniqID, MPCResult{result: 0, err: err})
		return err
	}
	go func() {
		val, err := m.ComputeExpression(mpcval.UniqID, mpcval.Expression, mpcval.Prime)
		err = m.mpcCenter.InformMPCComplete(mpcval.UniqID, MPCResult{result: val, err: err})
		if err != nil {
			log.Err(err).Send()
		}
	}()

	return nil
}
