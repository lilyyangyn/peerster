package blockchain

import (
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
)

type BlockchainModule struct {
	*message.MessageModule
	conf *peer.Configuration
}

func NewBlockchainModule(conf *peer.Configuration, messageModule *message.MessageModule) *BlockchainModule {
	m := BlockchainModule{
		MessageModule: messageModule,
		conf:          conf,
	}

	// message registery

	return &m
}
