package permissioned

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	"go.dedis.ch/cs438/storage"
)

// -----------------------------------------------------------------------------
// BlockHeader

// BlockHeader is the header of block in the blockchain
type BlockHeader struct {
	PrevHash string
	Height   uint
	Type     BlkType
	Miner    string

	StateHash      string
	TransationHash string
}

// Hash computes the block hash based on data in block header
func (bh *BlockHeader) Hash() string {
	h := sha256.New()
	h.Write([]byte(bh.PrevHash))
	h.Write([]byte(strconv.Itoa(int(bh.Height))))
	h.Write([]byte(bh.Type))
	h.Write([]byte(bh.Miner))

	h.Write([]byte(bh.StateHash))
	h.Write([]byte(bh.TransationHash))

	return hex.EncodeToString(h.Sum(nil))
}

// -----------------------------------------------------------------------------
// Block

// Block represents the basic building block of a blockc hain
type Block struct {
	*BlockHeader
	States       storage.KVStore
	Transactions []SignedTransaction
}

// BlkType helps to defer different types of txns & blocks
type BlkType string

const (
	BlkTypeConfig BlkType = "blk-config"
	BlkTypeTxn    BlkType = "blk-txn"
)

// GetWorldState returns a copy of block's world state
func (b *Block) GetWorldState() storage.KVStore {
	return b.States.Copy()
}

// GetConfig returns a copy of blockchain's config
func (b *Block) GetConfig() ChainConfig {
	return *getConfigFromWorldState(b.States)
}

// HasTxn checks whether the txn is included in the block
func (b *Block) HasTxn(txnID string) bool {
	for _, signedTxn := range b.Transactions {
		if signedTxn.Txn.ID == txnID {
			return true
		}
	}
	return false
}

// Verify verifies if a block is valid
func (b *Block) Verify(worldState storage.KVStore) error {
	config := getConfigFromWorldState(worldState)

	// check miner
	if _, ok := config.Participants[b.Miner]; !ok {
		return fmt.Errorf("miner %s is not a participant of the permissined chain", b.Miner)
	}

	// check hash
	h := sha256.New()
	for _, txn := range b.Transactions {
		h.Write(txn.Hash())
	}
	if b.TransationHash != hex.EncodeToString(h.Sum(nil)) {
		return fmt.Errorf("block %s has inconsistent transaction hash", b.Hash())
	}
	if b.StateHash != hex.EncodeToString(b.States.Hash()) {
		return fmt.Errorf("block %s has inconsistent state hash", b.Hash())
	}

	// check state
	for _, txn := range b.Transactions {
		err := txn.Verify(worldState, config)
		if err != nil {
			return fmt.Errorf("block %s has invalid transaction", b.Hash())
		}
	}
	if hex.EncodeToString(worldState.Hash()) != b.StateHash {
		return fmt.Errorf("block %s has different execution result from expected", b.Hash())
	}

	return nil
}

// -----------------------------------------------------------------------------
// BlockBuilder

type BlockBuilder struct {
	prevHash     string
	height       uint
	blockType    BlkType
	miner        string
	states       storage.KVStore
	transactions []SignedTransaction
	maxTxnCount  int
}

func NewBlockBuilder(blockType BlkType, maxTxnCount int) *BlockBuilder {
	return &BlockBuilder{
		blockType:    blockType,
		maxTxnCount:  maxTxnCount,
		transactions: make([]SignedTransaction, 0),
	}
}

func (bb *BlockBuilder) AddTxn(txn *SignedTransaction) error {
	if bb.blockType == BlkTypeConfig {
		return fmt.Errorf("unable to append txn to a config block: %T", txn.Txn)
	}
	if len(bb.transactions) >= bb.maxTxnCount {
		return fmt.Errorf("reached maximal number of txns. unable to append more")
	}

	bb.transactions = append(bb.transactions, *txn)
	return nil
}

func (bb *BlockBuilder) AddConfig(config *ChainConfig) error {
	if bb.blockType != BlkTypeConfig {
		return fmt.Errorf("unable to add configuration to a txn block: %T", config)
	}

	bb.states.Put(STATE_CONFIG_KEY, *config)
	return nil
}

func (bb *BlockBuilder) SetPrevHash(prevHash string) *BlockBuilder {
	bb.prevHash = prevHash
	return bb
}

func (bb *BlockBuilder) SetHeight(height uint) *BlockBuilder {
	bb.height = height
	return bb
}

func (bb *BlockBuilder) SetMiner(miner string) *BlockBuilder {
	bb.miner = miner
	return bb
}

func (bb *BlockBuilder) SetState(state storage.KVStore) *BlockBuilder {
	bb.states = state
	return bb
}

func (bb *BlockBuilder) Build() *Block {
	h := sha256.New()
	for _, txn := range bb.transactions {
		h.Write(txn.Hash())
	}
	txnHash := h.Sum(nil)

	header := BlockHeader{
		PrevHash:       bb.prevHash,
		Height:         bb.height,
		Type:           bb.blockType,
		Miner:          bb.miner,
		StateHash:      hex.EncodeToString(bb.states.Hash()),
		TransationHash: hex.EncodeToString(txnHash),
	}

	return &Block{
		BlockHeader:  &header,
		States:       bb.states,
		Transactions: bb.transactions,
	}
}
