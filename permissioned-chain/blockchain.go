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
	worldState := storage.NewBasicKV()
	for participant, _ := range config.Participants {
		account := *NewAccount(*NewAddressFromHex(participant))
		if amount, ok := initialGain[participant]; ok {
			account.balance = amount
		}
		worldState.Put(participant, account)
	}
	worldState.Put(STATE_CONFIG_KEY, *config)

	// genesis block must be a block with a config txn
	bb := NewBlockBuilder(BlkTypeConfig)
	bb.SetPrevHash(DUMMY_PREVHASH).
		SetHeight(0).
		SetMiner(ZeroAddress.Hex).
		SetState(worldState)
	block := bb.Build()

	bc.Lock()
	defer bc.Unlock()

	if len(bc.blocksStore) > 0 {
		panic("unable to init an already existing chain")
	}
	bc.blocksStore[block.Hash()] = block
	bc.latestBlock = block

	return *block, nil
}

// GetBlockStore returns a copy of the whole chain
func (bc *Blockchain) GetBlockStore() map[string]Block {
	store := make(map[string]Block)
	bc.RLock()
	defer bc.RUnlock()

	for key, block := range bc.blocksStore {
		store[key] = *block
	}
	return store
}

// GetLatestBlock returns the latest block
func (bc *Blockchain) GetLatestBlock() Block {
	bc.RLock()
	defer bc.RUnlock()

	return *bc.latestBlock
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
)

// HasTxn checks if the target transaction is in blockchain
func (bc *Blockchain) HasTxn(txnID string) bool {
	bc.RLock()
	defer bc.RUnlock()
	curBlock := bc.latestBlock

	ok := true
	for ok {
		if curBlock.HasTxn(txnID) {
			return true
		}
		curBlock, ok = bc.blocksStore[curBlock.PrevHash]
	}
	return false
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

	// try append to see if at the correct position
	status := bc.checkBlockHeight(block)
	if status == BlockCompareStale || status == BlockCompareAdvance {
		return fmt.Errorf("block too new or too old. Expected: %d, Got: %d",
			bc.latestBlock.Height, block.Height)
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
