package paxos

import (
	"github.com/rs/xid"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

func (m *PaxosModule) InitMPCConensus(budget float64, expression string) (err error) {
	if m.valueType != PaxosMPC {
		return xerrors.Errorf("invalid operation")
	}

	// TODO: check balance

	m.Lock()
	if !m.Occupied {
		m.Occupied = true
		m.Proposer = true
		m.paxosPromiseChan = make(chan paxosResult, 3)
		step := m.TLC
		m.Unlock()
		return m.proposeMPC(budget, expression, step)
	}
	m.cond.Wait()
	m.Unlock()
	return m.InitMPCConensus(budget, expression)
}

/** Private Helpfer Functions **/

func (m *PaxosModule) proposeMPC(budget float64, expression string, step uint) error {
	// TODO: use public key hash?
	initiator := m.GetPubkey().N.String()
	proposeValContent := types.PaxosMPCValue{
		UniqID:     xid.New().String(),
		Initiator:  initiator,
		Budget:     budget,
		Expression: expression,
	}
	proposeVal, err := types.CreatePaxosValue(proposeValContent)
	if err != nil {
		return err
	}

	result := m.startFromPhaseOne(proposeVal, step)
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
	return m.InitMPCConensus(budget, expression)
}

// tagThreshold calculates the threshold to enter next paxos stage
func (m *PaxosModule) mpcThreshold() int {
	return int(m.conf.TotalPeers)
}

// tagCallback is a callback function that gets called when TLC advances
func (m *PaxosModule) mpcCallback(value *types.PaxosValue) error {
	return nil
}
