package paxos

import (
	"github.com/rs/xid"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// NewMPCPaxos creates a new PaxosInstance that handles mpc consensus
func NewMPCPaxos(m *PaxosModule) *PaxosInstance {
	p := *NewPaxosInstance(m)
	p.Callback = m.mpcCallback
	p.threshold = m.mpcThreshold
	p.lastBlockKey = "MPC.LastBlockKey"

	return &p
}

func (m *PaxosInstance) InitMPCConcensus(budget float64, expression string) (err error) {
	if m.Type != types.PaxosTypeMPC {
		return xerrors.Errorf("invalid operation")
	}

	// TODO: check expression not exists in records

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
	return m.InitMPCConcensus(budget, expression)
}

/** Private Helpfer Functions **/

func (m *PaxosInstance) proposeMPC(budget float64, expression string, step uint) error {
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
	return m.InitMPCConcensus(budget, expression)
}

// mpcThreshold calculates the threshold to enter next paxos stage
func (m *PaxosModule) mpcThreshold() int {
	return int(m.conf.TotalPeers)
}

// mpcCallback is a callback function that gets called when TLC advances
func (m *PaxosModule) mpcCallback(value *types.PaxosValue) error {
	// TODO: start MPC
	return nil
}