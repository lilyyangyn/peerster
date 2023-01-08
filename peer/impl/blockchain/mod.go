package blockchain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	permissioned "go.dedis.ch/cs438/permissioned-chain"
	"go.dedis.ch/cs438/types"
)

type BlockchainModule struct {
	*message.MessageModule
	conf *peer.Configuration

	wallet *Wallet

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
	go m.txnPool.Daemon(ctx)
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
	participants := make(map[string]struct{})
	for p := range config.Participants {
		participants[p] = struct{}{}
	}
	err = m.broadcastBCBlkMessage(participants, &blk)
	if err != nil {
		return err
	}
	return nil
}

// SendTransaction signs and sends a transaction
func (m *BlockchainModule) SendTransaction(signedTxn *permissioned.SignedTransaction) error {
	// get config and send private message
	config := m.GetConfig()
	participants := make(map[string]struct{})
	for p := range config.Participants {
		participants[p] = struct{}{}
	}

	return m.broadcastBCTxnMessage(participants, signedTxn)
}

// GetChainAddress helps users to know the adress of the node
func (m *BlockchainModule) GetChainAddress() (permissioned.Address, error) {
	if m.wallet == nil {
		return permissioned.Address{},
			fmt.Errorf("node %s does not have an address yet",
				m.conf.Socket.GetAddress())
	}
	return m.wallet.GetAddress(), nil
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
	// Only assume pne-time initialize of wallet
	if m.wallet != nil {
		return fmt.Errorf("wallet already set. Account is not changable")
	}
	m.wallet = NewWallet(&privkey)

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

// RegisterTxnCallabck registers a callback function for a specific type of txn
// it will be called when the txn is in a newly appended block
func (m *BlockchainModule) RegisterTxnCallabck(txnType permissioned.TxnType, watcher watchCallbck) {
	m.watchRegistry.Register(txnType, watcher)
}

// SendPreMPCTransaction generates and sends a preMPC transaction
func (m *BlockchainModule) SendPreMPCTransaction(expression string, budget float64, prime string) (string, error) {
	signedTxn, err := m.wallet.PreMPCTxn(expression, budget, prime)
	if err != nil {
		return "", err
	}
	return signedTxn.Txn.ID, m.SendTransaction(signedTxn)
}

// SendPostMPCTransaction generates and sends a postMPC transaction
func (m *BlockchainModule) SendPostMPCTransaction(id string, result float64) (string, error) {
	signedTxn, err := m.wallet.PostMPCTxn(id, result)
	if err != nil {
		return "", err
	}
	return signedTxn.Txn.ID, m.SendTransaction(signedTxn)
}

// SprintBlockchain returns a description of the chain
func (m *BlockchainModule) SprintBlockchain() string {
	return m.Sprint()
}

// GetBlockTimeout returns the maximum timeout of a block
func (m *BlockchainModule) GetMaxBlockTime() time.Duration {
	config := m.GetConfig()
	return getBlockTimeout(&config) * time.Duration(config.MaxTxnsPerBlk)
}

// -----------------------------------------------------------------------------
// Private Helpfer Functions

// sync syncs blochain with the target node
func (m *BlockchainModule) sync(to string) error {
	id := xid.New().String()
	return m.sendBCAskSyncMessage(id, to)
}

// processBlk process a received block
func (m *BlockchainModule) processBlk(block *permissioned.Block) error {
	// if is genesis block. Directly set
	if block.Height == 0 {
		err := m.SetGenesisBlock(block)
		if err != nil {
			return err
		}

		log.Info().Msgf("init genesis block successfully")
		close(m.bcReadyChan)
		m.selectNextMiner(block)

		// send SetPubkey Txn
		if m.conf.DisableAnnoncePubkey {
			return nil
		}
		txnID, err := m.sendSetPubkeyTransaction()
		if err != nil {
			return err
		}
		log.Info().Msgf("send setPubkey txn %s", txnID)

		return nil
	}

	// Otherwise,append the block
	m.blkChan <- block
	return nil
}

// selectNextMiner selects the next Miner
// it notifies the minning daemon if the miner is us
func (m *BlockchainModule) selectNextMiner(block *permissioned.Block) {
	// select next miner
	nextMiner := m.cr.advanceAndSelect(block)
	log.Info().Msgf("Next miner on height %d is %s",
		block.Height+1, nextMiner)
	// notify miner to start if the next miner is myself
	if nextMiner == m.wallet.GetAddress().Hex {
		m.minerChan <- block.Height
	}
}

// getBlockTimeout returns the maxTimeout configured by chain config
func getBlockTimeout(config *permissioned.ChainConfig) time.Duration {
	duration, err := time.ParseDuration(config.WaitTimeout)
	if err != nil {
		log.Warn().Msgf(`Dangerous: unrecognize max block timeout. 
			Miner's max mining period can be infinitive`)
		duration = math.MaxInt64
	}
	return duration
}

func (m *BlockchainModule) sendSetPubkeyTransaction() (string, error) {
	pubkey, err := m.GetPubkeyString()
	if err != nil {
		return "", err
	}

	signedTxn, err := m.wallet.SetPubkeyTxn(pubkey)
	if err != nil {
		return "", err
	}
	return signedTxn.Txn.ID, m.SendTransaction(signedTxn)
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

// sendBCAskSyncMessage sends a BCAskSyncMessage in private msg
func (m *BlockchainModule) sendBCAskSyncMessage(id string,
	to string) error {
	askMsg := types.BCAskSyncMessage{
		UniqID:       id,
		Origin:       m.conf.Socket.GetAddress(),
		LatestHeight: m.GetLatestBlock().Height,
	}
	askMsgMarshal, err := m.CreateMsg(askMsg)
	if err != nil {
		return err
	}

	return m.Unicast(to, askMsgMarshal)
}

// sendBCSyncMessage sends a BCSyncMessage in private msg
func (m *BlockchainModule) sendBCSyncMessage(id string,
	blocks []permissioned.Block, to string) error {
	syncMsg := types.BCSyncMessage{
		UniqID: id,
		Origin: m.conf.Socket.GetAddress(),
		Blocks: blocks,
	}
	syncMsgMarshal, err := m.CreateMsg(syncMsg)
	if err != nil {
		return err
	}

	return m.Unicast(to, syncMsgMarshal)
}
