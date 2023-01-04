package permissioned

import (
	"encoding/hex"
	"fmt"
	"sync"

	"go.dedis.ch/cs438/storage"
)

var DUMMY_PREVHASH = hex.EncodeToString(make([]byte, 32))

type Blockchain struct {
	*sync.RWMutex
	blocksStore map[string]*Block
	latestBlock *Block // only allow one level of inconsistency
}

func NewBlockchain() *Blockchain {
	bm := Blockchain{
		RWMutex:     &sync.RWMutex{},
		blocksStore: map[string]*Block{},
		latestBlock: nil,
	}
	return &bm
}

// InitGenesisBlock inits a new blockchain with the given config
func (bc *Blockchain) InitGenesisBlock(config *ChainConfig,
	initialGain map[string]float64) (Block, error) {
	bb := NewBlockBuilder()

	worldState := storage.NewBasicKV()
	// add config
	signedTxn, err := NewTransactionInitConfig(config).Sign(nil)
	if err != nil {
		return Block{}, err
	}
	bb.AddTxn(signedTxn)
	worldState.Put(STATE_CONFIG_KEY, *config)

	// init accounts
	zeroAccount := *NewAccount(ZeroAddress)
	for participant := range config.Participants {
		if amount, ok := initialGain[participant]; ok {
			account := *NewAccount(*NewAddressFromHex(participant))
			account.balance = amount
			signedTxn, err := NewTransactionCoinbase(account.addr, amount).Sign(nil)
			if err != nil {
				return Block{}, err
			}
			zeroAccount.nonce++

			bb.AddTxn(signedTxn)
			worldState.Put(participant, account)
		}
	}

	// genesis block must be a block with a config txn
	bb.SetPrevHash(DUMMY_PREVHASH).
		SetHeight(0).
		SetMiner(ZeroAddress.Hex).
		SetState(worldState)
	block := bb.Build()

	return *block, nil
}

// SetGenesisBlock sets genesis block for the blockchain
func (bc *Blockchain) SetGenesisBlock(block *Block) error {
	if block.PrevHash != DUMMY_PREVHASH || block.Height != 0 {
		return fmt.Errorf("genesis block needs to be prevHash=%s and height=0", DUMMY_PREVHASH)
	}

	bc.Lock()
	defer bc.Unlock()

	if len(bc.blocksStore) > 0 {
		return fmt.Errorf("chain already initialized")
	}

	err := block.Verify(storage.NewBasicKV())
	if err != nil {
		return err
	}

	bc.blocksStore[block.Hash()] = block
	bc.latestBlock = block
	return nil
}

// GetBlock returns a copy of the whole chain
func (bc *Blockchain) GetBlock(blockID string) *Block {
	bc.RLock()
	defer bc.RUnlock()

	return bc.blocksStore[blockID]
}

// GetLatestBlock returns the latest block
func (bc *Blockchain) GetLatestBlock() *Block {
	bc.RLock()
	defer bc.RUnlock()

	return bc.latestBlock
}

// GetConfig returns the latest config
func (bc *Blockchain) GetConfig() ChainConfig {
	bc.RLock()
	defer bc.RUnlock()

	return *GetConfigFromWorldState(bc.latestBlock.States)
}

type BlockHeightCompareResult int

const (
	BlockCompareMatched BlockHeightCompareResult = iota
	BlockCompareStale
	BlockCompareAdvance
	BlockCompareInvalidHash
	BlockCompareNotInitialize
)

// GetTxn checks if the target transaction is in blockchain
func (bc *Blockchain) GetTxn(txnID string) *SignedTransaction {
	bc.RLock()
	defer bc.RUnlock()

	curBlock := bc.latestBlock
	if curBlock == nil {
		return nil
	}

	ok := true
	for ok {
		if txn := curBlock.GetTxn(txnID); txn != nil {
			return txn
		}
		curBlock, ok = bc.blocksStore[curBlock.PrevHash]
	}
	return nil
}

// CheckBlockHeight checks if a block can be appended to the end of chain
func (bc *Blockchain) CheckBlockHeight(block *Block) BlockHeightCompareResult {
	// block can only be the latest height+1
	bc.RLock()
	defer bc.RUnlock()

	return bc.checkBlockHeight(block)
}

// AppendBlock appends a new block to the blockchain
func (bc *Blockchain) AppendBlock(block *Block) error {
	bc.Lock()
	defer bc.Unlock()

	if bc.latestBlock == nil {
		return fmt.Errorf("need to set genesis block first")
	}

	// try append to see if at the correct position
	status := bc.checkBlockHeight(block)
	if status == BlockCompareStale || status == BlockCompareAdvance {
		return fmt.Errorf("block too new or too old. Expected: %d, Got: %d",
			bc.latestBlock.Height+1, block.Height)
	}
	if status == BlockCompareInvalidHash {
		return fmt.Errorf("invalid parent hash. Expected: %s, Got: %s",
			bc.latestBlock.Hash(), block.PrevHash)
	}

	// verify block
	err := block.Verify(bc.latestBlock.States.Copy())
	if err != nil {
		return err
	}

	// extends the blockchain
	bc.blocksStore[block.Hash()] = block
	bc.latestBlock = block

	return nil
}

// checkBlockHeight is a helper funcion of TryAppendBlock
func (bc *Blockchain) checkBlockHeight(block *Block) BlockHeightCompareResult {
	if bc.latestBlock == nil {
		return BlockCompareNotInitialize
	}

	// block can only be the latest height+1
	lastHeight := bc.latestBlock.Height
	// lastPrevBlck := bc.latestBlocks.PrevHash
	// if height equal, then it is a fork
	// if lastHeight == block.Height {
	// 	if block.PrevHash != lastPrevBlck {
	// 		return fmt.Errorf("block with invalid prev hash. Expected: %s, Got: %s", lastPrevBlck, block.PrevHash)
	// 	}
	// 	bc.latestBlocks = append(bc.latestBlocks, &block)
	// 	return nil
	// }
	// if height != lastHeight, then invalid block, too new or too old
	if lastHeight+1 > block.Height {
		return BlockCompareStale
	}
	if lastHeight+1 < block.Height {
		return BlockCompareAdvance
	}
	if prevhash := bc.latestBlock.Hash(); prevhash != block.PrevHash {
		return BlockCompareInvalidHash
	}
	return BlockCompareMatched
}
