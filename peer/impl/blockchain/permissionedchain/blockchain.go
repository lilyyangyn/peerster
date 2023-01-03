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
func (bc *Blockchain) InitGenesisBlock(config *ChainConfig, initialGain map[string]float64) (Block, error) {
	worldState := storage.NewBasicKV()
	for participant, _ := range config.Participants {
		account := NewAccount(*NewAddressFromHex(participant))
		account.balance = initialGain[participant]
	}
	worldState.Put(STATE_CONFIG_KEY, *config)

	// genesis block must be a block with a config txn
	bb := NewBlockBuilder(BlkTypeConfig, config.MaxTxnsPerBlk)
	bb.SetPrevHash(DUMMY_PREVHASH).
		SetHeight(0).
		SetMiner(ZeroAddress.Hex).
		SetState(storage.NewBasicKV())
	block := bb.Build()

	bc.Lock()
	defer bc.Unlock()
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
	return *getConfigFromWorldState(bc.latestBlock.States)
}

func (bc *Blockchain) ValidateBlock(block *Block) error {
	// block can only be the latest height+1
	bc.RLock()
	defer bc.RUnlock()

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
	if lastHeight+1 != block.Height {
		return fmt.Errorf("block too new or too old. Expected: %d, Got: %d", lastHeight, block.Height)
	}
	if prevhash := bc.latestBlock.Hash(); prevhash != block.PrevHash {
		return fmt.Errorf("invalid parent hash. Expected: %s, Got: %s", prevhash, block.PrevHash)
	}
	return nil
}

// AppendBlock appends a new block to the blockchain
func (bc *Blockchain) AppendBlock(block *Block) error {
	// block can only be the latest height+1
	bc.Lock()
	defer bc.Unlock()

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
	if lastHeight+1 != block.Height {
		return fmt.Errorf("block too new or too old. Expected: %d, Got: %d", lastHeight, block.Height)
	}

	// extends the blockchain
	// for _, blk := range bc.latestBlocks {
	prevhash := bc.latestBlock.Hash()
	if prevhash == block.PrevHash {
		bc.latestBlock = block
		return nil
	}
	// }

	return fmt.Errorf("invalid parent hash. Expected: %s, Got: %s", prevhash, block.PrevHash)
}
