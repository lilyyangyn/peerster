package blockchain

import (
	"context"
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	permissioned "go.dedis.ch/cs438/permissioned-chain"
	"go.dedis.ch/cs438/types"
)

type BlockchainModule struct {
	*message.MessageModule
	conf *peer.Configuration

	account *permissioned.Account
	privKey *ecdsa.PrivateKey

	blockchain *permissioned.Blockchain
	txnPool    *TxnPool

	blkChan     chan *permissioned.Block
	bcReadyChan chan struct{}
}

func NewBlockchainModule(conf *peer.Configuration, messageModule *message.MessageModule) *BlockchainModule {
	m := BlockchainModule{
		MessageModule: messageModule,
		conf:          conf,

		txnPool:     NewTxnPool(),
		blkChan:     make(chan *permissioned.Block, 5),
		bcReadyChan: make(chan struct{}),
	}

	// message registery

	return &m
}

// -----------------------------------------------------------------------------
// Feature Functions

// MiningDaemon starts a new minor daemon
func (m *BlockchainModule) MiningDaemon(ctx context.Context) error {
	m.txnPool.SetCtx(ctx)
	go m.Mine(ctx, m.txnPool)
	go m.VerifyBlock(ctx)
	return nil
}

// InitBlockchain inits a new blockchain with the given config
func (m *BlockchainModule) InitBlockchain(config permissioned.ChainConfig, initialGain map[string]float64) error {
	bc := permissioned.NewBlockchain()
	blk, err := bc.InitGenesisBlock(&config, initialGain)
	if err != nil {
		return err
	}
	close(m.bcReadyChan)

	// broadcast the genesis block
	err = m.broadcastBCBlkMessage(config.Participants, &blk)
	if err != nil {
		return err
	}
	return nil
}

// SendTransaction signs and sends a transaction
func (m *BlockchainModule) SendTransaction(txn *permissioned.Transaction) error {
	// sign txn
	signedTxn, err := txn.Sign(m.privKey)
	if err != nil {
		return err
	}

	// get config and send private message
	config := m.blockchain.GetConfig()
	return m.broadcastBCTxnMessage(config.Participants, signedTxn)
}

// HasTransaction checks if the transaction is in the blockchain
func (m *BlockchainModule) HasTransaction(txnID string) bool {
	return m.blockchain.HasTxn(txnID)
}

// GenerateKeyPair generates an ECDSA key pair and write it in the file
func (m *BlockchainModule) GenerateKeyPair(path string) error {
	privKey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}
	m.privKey = privKey
	return crypto.SaveECDSA(path, privKey)
}

// LoadKeyPair loads an ECDSA key pair from file
func (m *BlockchainModule) LoadKeyPair(path string) error {
	privkey, err := crypto.LoadECDSA(path)
	if err != nil {
		return err
	}

	m.privKey = privkey
	return nil
}

// -----------------------------------------------------------------------------
// Private Helpfer Functions

// broadcastBCTxnMessage broadcast a BCTxnMessage in private msg
func (m *BlockchainModule) broadcastBCTxnMessage(participants map[string]struct{},
	txn *permissioned.SignedTransaction) error {
	txnMsg := types.BCTxnMessag{
		Origin: m.conf.Socket.GetAddress(),
		Txn:    *txn,
	}
	txnMsgMarshal, err := m.CreateMsg(txnMsg)
	if err != nil {
		return err
	}

	// wrap in private msg
	privMsg := types.BCPrivateMessage{
		Recipients: participants,
		Msg:        &txnMsgMarshal,
	}
	privMsgMarshal, err := m.CreateMsg(privMsg)
	if err != nil {
		return err
	}

	// send in rumor
	return m.Broadcast(privMsgMarshal)
}

// broadcastBCBlkMessage broadcast a BCBlkMessage in private msg
func (m *BlockchainModule) broadcastBCBlkMessage(participants map[string]struct{},
	block *permissioned.Block) error {
	txnMsg := types.BCBlkMessage{
		Origin: m.conf.Socket.GetAddress(),
		Blk:    *block,
	}
	txnMsgMarshal, err := m.CreateMsg(txnMsg)
	if err != nil {
		return err
	}

	// wrap in private msg
	privMsg := types.BCPrivateMessage{
		Recipients: participants,
		Msg:        &txnMsgMarshal,
	}
	privMsgMarshal, err := m.CreateMsg(privMsg)
	if err != nil {
		return err
	}

	// send in rumor
	return m.Broadcast(privMsgMarshal)
}
