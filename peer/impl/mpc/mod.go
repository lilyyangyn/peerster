package mpc

import (
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
)

type MPCModule struct {
	*message.MessageModule
	conf *peer.Configuration
}

func NewMPCModule(conf *peer.Configuration, messageModule *message.MessageModule) *MPCModule {
	m := MPCModule{
		MessageModule: messageModule,
		conf:          conf,
	}

	// message registery

	return &m
}
