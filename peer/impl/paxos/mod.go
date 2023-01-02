package paxos

import (
	"sync"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type PaxosModule struct {
	*message.MessageModule
	conf *peer.Configuration

	*sync.RWMutex
	instances map[types.PaxosType]*PaxosInstance
}

func NewPaxosModule(conf *peer.Configuration, messageModule *message.MessageModule) *PaxosModule {
	m := PaxosModule{
		MessageModule: messageModule,
		conf:          conf,

		RWMutex:   &sync.RWMutex{},
		instances: make(map[types.PaxosType]*PaxosInstance),
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

func (m *PaxosModule) CreateNewPaxos(paxosType types.PaxosType,
	lastBlockKey string, threshold func() int,
	callback func(*types.PaxosValue) error) (*PaxosInstance, error) {

	m.Lock()
	defer m.Unlock()

	_, ok := m.instances[paxosType]
	if ok {
		return nil, xerrors.Errorf("paxos type already exists. No new paxos instance will be created.")
	}

	instance := NewPaxosInstance(m)
	instance.Type = paxosType
	instance.threshold = threshold
	instance.callback = callback
	instance.lastBlockKey = lastBlockKey

	m.instances[paxosType] = instance
	return instance, nil
}

func (m *PaxosModule) GetPaxos(paxosType types.PaxosType) (*PaxosInstance, bool) {
	m.RLock()
	defer m.RUnlock()

	instance, ok := m.instances[paxosType]
	return instance, ok
}
