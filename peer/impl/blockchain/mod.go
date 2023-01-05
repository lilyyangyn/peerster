package blockchain

import (
	"context"
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog/log"
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

	*permissioned.Blockchain
	txnPool       *TxnPool
	watchRegistry *WatchRegistry
	cr            *CreditRecords

	blkChan     chan *permissioned.Block
	bcReadyChan chan struct{}
	minerChan   chan uint
}

func NewBlockchainModule(conf *peer.Configuration, messageModule *message.MessageModule) *BlockchainModule {
	m := BlockchainModule{
		MessageModule: messageModule,
		conf:          conf,

		Blockchain:    permissioned.NewBlockchain(),
		txnPool:       NewTxnPool(),
		watchRegistry: NewWatchRegistry(),
		cr:            NewCreditRecords(),
		blkChan:       make(chan *permissioned.Block, 5),
		bcReadyChan:   make(chan struct{}),
		minerChan:     make(chan uint, 5),
	}

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.BCPrivateMessage{}, m.ProcessBCPrivateMsg)
	m.conf.MessageRegistry.RegisterMessageCallback(types.BCBlkMessage{}, m.ProcessBCBlkMsg)
	m.conf.MessageRegistry.RegisterMessageCallback(types.BCTxnMessag{}, m.ProcessBCTxnMsg)

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

	// broadcast the genesis block
	err = m.broadcastBCBlkMessage(config.Participants, &blk)
	if err != nil {
		return err
	}
	return nil
}

// SendTransaction signs and sends a transaction
func (m *BlockchainModule) SendTransaction(signedTxn *permissioned.SignedTransaction) error {
	// TODO: check from?
	// TODO: wallet sevice

	// get config and send private message
	config := m.GetConfig()
	return m.broadcastBCTxnMessage(config.Participants, signedTxn)
}

// GetAddress helps users to know the adress of the node
func (m *BlockchainModule) GetAddress() (permissioned.Address, error) {
	if m.account == nil {
		return permissioned.Address{},
			fmt.Errorf("node %s does not have an address yet",
				m.conf.Socket.GetAddress())
	}
	return m.account.GetAddress(), nil
}

// GenerateKeyPair generates an ECDSA key pair and write it in the file
func (m *BlockchainModule) GenerateKeyPair(path string) error {
	privkey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}
	err = m.SetKeyPair(*privkey)
	if err != nil {
		return err
	}
	return crypto.SaveECDSA(path, privkey)
}

// SetKeyPair sets an ECDSA key pair to the node
func (m *BlockchainModule) SetKeyPair(privkey ecdsa.PrivateKey) error {
	m.privKey = &privkey
	address := permissioned.NewAddress(&privkey.PublicKey)
	m.account = permissioned.NewAccount(*address)

	return nil
}

// LoadKeyPair loads an ECDSA key pair from file
func (m *BlockchainModule) LoadKeyPair(path string) error {
	privkey, err := crypto.LoadECDSA(path)
	if err != nil {
		return err
	}

	return m.SetKeyPair(*privkey)
}

// SendPreMPCTransaction generates and sends a preMPC transaction
func (m *BlockchainModule) SendPreMPCTransaction(expression string, budget float64) error {
	record := permissioned.MPCRecord{
		Initiator:  m.account.GetAddress().Hex,
		Budget:     budget,
		Expression: expression,
	}
	signedTxn, err := permissioned.NewTransactionPreMPC(m.account, record).Sign(m.privKey)
	if err != nil {
		return err
	}
	return m.SendTransaction(signedTxn)
}

// SendPostMPCTransaction generates and sends a postMPC transaction
func (m *BlockchainModule) SendPostMPCTransaction(id string, result float64) error {
	// TODO: update account nonce?
	record := permissioned.MPCRecord{
		UniqID: id,
		Result: result,
	}
	signedTxn, err := permissioned.NewTransactionPostMPC(m.account, record).Sign(m.privKey)
	if err != nil {
		return err
	}
	return m.SendTransaction(signedTxn)
}

// -----------------------------------------------------------------------------
// Private Helpfer Functions

// selectNextMiner selects the next Miner
// it notifies the minning daemon if the miner is us
func (m *BlockchainModule) selectNextMiner(block *permissioned.Block) {
	// select next miner
	nextMiner := m.cr.advanceAndSelect(block)
	log.Info().Msgf("Next miner on height %d is %s",
		block.Height+1, nextMiner)
	// notify miner to start if the next miner is myself
	if nextMiner == m.account.GetAddress().Hex {
		m.minerChan <- block.Height
	}
}

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

	blkMsg := types.BCBlkMessage{
		Origin:    m.conf.Socket.GetAddress(),
		BlkHeader: *block.BlockHeader,
		Txns:      block.Transactions,
	}
	blkMsgMarshal, err := m.CreateMsg(blkMsg)
	if err != nil {
		return err
	}

	// wrap in private msg
	privMsg := types.BCPrivateMessage{
		Recipients: participants,
		Msg:        &blkMsgMarshal,
	}
	privMsgMarshal, err := m.CreateMsg(privMsg)
	if err != nil {
		return err
	}

	// send in rumor
	return m.Broadcast(privMsgMarshal)
}
