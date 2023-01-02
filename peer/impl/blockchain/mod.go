package blockchain

import (
	"context"

	"go.dedis.ch/cs438/peer"
	permissioned "go.dedis.ch/cs438/peer/impl/blockchain/permissionedchain"
	"go.dedis.ch/cs438/peer/impl/message"
)

type BlockchainModule struct {
	*message.MessageModule
	conf *peer.Configuration

	account    *permissioned.Account
	blockchain *permissioned.Blockchain
}

func NewBlockchainModule(conf *peer.Configuration, messageModule *message.MessageModule) *BlockchainModule {
	m := BlockchainModule{
		MessageModule: messageModule,
		conf:          conf,
	}

	// message registery

	return &m
}

// -----------------------------------------------------------------------------
// Feature Functions

func (m *BlockchainModule) MiningDaemon(ctx context.Context) error {
	return nil
}

// -----------------------------------------------------------------------------
// Private Helpfer Functions
