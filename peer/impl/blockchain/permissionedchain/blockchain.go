package permissioned

import (
	"encoding/hex"
	"fmt"
	"sync"

	"go.dedis.ch/cs438/storage"
)

var DUMMY_PREVHASH = hex.EncodeToString(make([]byte, 32))

type Blockchain struct {
	*sync.Mutex
	blocksStore  map[string]*Block
	latestBlocks *Block // only allow one level of inconsistency
}

func NewBlockchain() *Blockchain {
	bm := Blockchain{
		Mutex:        &sync.Mutex{},
		blocksStore:  map[string]*Block{},
		latestBlocks: nil,
	}
	return &bm
}

// InitGenesisBlock inits a new blockchain with the given config
func (bc *Blockchain) InitGenesisBlock(config *ChainConfig) (Block, error) {
	// genesis block must be a block with a config txn
	bb := NewBlockBuilder(BlkTypeConfig)
	bb.SetPrevHash(DUMMY_PREVHASH).SetHeight(0).SetState(storage.NewBasicKV()).AddConfig(config)
	block := bb.Build()

	bc.Lock()
	defer bc.Unlock()
	bc.blocksStore[block.Hash()] = block
	bc.latestBlocks = block

	return *block, nil
}

// AppendBlock appends a new block to the blockchain
func (bc *Blockchain) AppendBlock(block Block) error {
	// block can only be the latest height or latest height+1
	bc.Lock()
	defer bc.Unlock()

	lastHeight := bc.latestBlocks.Height
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
		return fmt.Errorf("block too new or too old. Current height: %d, Got: %d", lastHeight, block.Height)
	}

	// extends the blockchain
	// for _, blk := range bc.latestBlocks {
	if bc.latestBlocks.Hash() == block.PrevHash {
		bc.latestBlocks = &block
	}
	// }

	return nil
}
