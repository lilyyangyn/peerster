package mpc

import (
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/peer/impl/paxos"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type MPCModule struct {
	*message.MessageModule
	conf *peer.Configuration

	valueDB *ValueDB
	*MPC

	*paxos.PaxosInstance
}

func NewMPCModule(conf *peer.Configuration, messageModule *message.MessageModule, paxosModule *paxos.PaxosModule) *MPCModule {
	m := MPCModule{
		MessageModule: messageModule,
		conf:          conf,
		valueDB:       NewValueDB(),
	}
	instance, err := paxosModule.CreateNewPaxos(
		types.PaxosTypeMPC,
		"MPC.LastBlockKey",
		m.mpcThreshold,
		m.mpcCallback,
	)
	if err != nil {
		panic(err)
	}
	m.PaxosInstance = instance

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.MPCShareMessage{}, m.ProcessMPCShareMsg)

	return &m
}

/** Feature Functions **/

// StartMPC start a new MPC from making consensus on budget and expression.
// It will then initiate the MPC automatically
func (m *MPCModule) StartMPC(budget float64, expression string) (int, error) {
	err := m.InitMPCConcensus(budget, expression)
	if err != nil {
		return -1, err
	}

	return -1, nil
}

func (m *MPCModule) SetMPCValue(key string, value int) error {
	ok := m.valueDB.add(key, value)
	if !ok {
		return xerrors.Errorf("key for MPC value already used")
	}

	return nil
}

/** Private Helpfer Functions **/
