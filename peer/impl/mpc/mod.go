package mpc

import (
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type MPCModule struct {
	*message.MessageModule
	conf *peer.Configuration

	valueDB *ValueDB
	*MPC
}

func NewMPCModule(conf *peer.Configuration, messageModule *message.MessageModule) *MPCModule {
	m := MPCModule{
		MessageModule: messageModule,
		conf:          conf,
		valueDB:       NewValueDB(),
	}

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.MPCShareMessage{}, m.ProcessMPCShareMsg)

	return &m
}

/** Feature Functions **/

func (m *MPCModule) SetMPCValue(key string, value int) error {
	ok := m.valueDB.add(key, value)
	if !ok {
		return xerrors.Errorf("key for MPC value already used")
	}

	return nil
}

/** Private Helpfer Functions **/
