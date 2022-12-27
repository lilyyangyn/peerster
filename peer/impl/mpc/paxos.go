package mpc

import (
	"github.com/rs/xid"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// initMPCConcensus inits a paxos to consensus on mpc value
func (m *MPCModule) initMPCConcensus(budget float64, expression string, prime string) (err error) {
	if m.Type != types.PaxosTypeMPC {
		return xerrors.Errorf("invalid operation")
	}

	// TODO: check balance

	if step, ok := m.CheckAndWait(); ok {
		return m.proposeMPC(budget, expression, prime, step)
	}
	return m.initMPCConcensus(budget, expression, prime)
}

/** Private Helpfer Functions **/

// proposeMPC starts a new paxos starting from phase one
func (m *MPCModule) proposeMPC(budget float64, expression string, prime string, step uint) error {
	// TODO: use public key hash?
	initiator := m.GetPubkey().N.String()
	proposeValContent := types.PaxosMPCValue{
		UniqID:     xid.New().String(),
		Initiator:  initiator,
		Budget:     budget,
		Expression: expression,
		Prime:      prime,
	}
	proposeVal, err := types.CreatePaxosValue(proposeValContent)
	if err != nil {
		return err
	}

	result := m.StartFromPhaseOne(proposeVal, step)
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
	return m.initMPCConcensus(budget, expression, prime)
}

// mpcThreshold calculates the threshold to enter next paxos stage
func (m *MPCModule) mpcThreshold() int {
	return int(m.conf.TotalPeers)
}

// mpcCallback is a callback function that gets called when TLC advances
func (m *MPCModule) mpcCallback(value *types.PaxosValue) error {
	// TODO: start MPC
	return nil
}
