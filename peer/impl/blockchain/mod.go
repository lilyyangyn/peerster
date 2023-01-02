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

	txnPool *TxnPool
	blkChan chan *permissioned.Block
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
	go m.Mine(ctx)
	go m.VerifyBlock(ctx)
	return nil
}

func (m *BlockchainModule) AcceptorDaemon(ctx context.Context) error {
	go m.VerifyBlock(ctx)
	return nil
}

func (m *BlockchainModule) StopMiner() {
	m.txnPool.Finish()
}

// -----------------------------------------------------------------------------
// Private Helpfer Functions
