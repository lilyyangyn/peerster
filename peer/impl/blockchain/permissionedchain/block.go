package permissioned

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	"go.dedis.ch/cs438/storage"
)

// -----------------------------------------------------------------------------
// Block

// Block represents the basic building block of a blockc hain
type Block struct {
	*BlockHeader
	States       storage.KVStore
	Transactions []*SignedTransaction
}

// BlkType helps to defer different types of txns & blocks
type BlkType string

const (
	BlkTypeConfig BlkType = "blk-config"
	BlkTypeTxn    BlkType = "blk-txn"
)

// -----------------------------------------------------------------------------
// BlockHeader

// BlockHeader is the header of block in the blockchain
type BlockHeader struct {
	PrevHash string
	Height   uint
	Type     BlkType

	StateHash      []byte
	TransationHash []byte
}

// Hash computes the block hash based on data in block header
func (bh *BlockHeader) Hash() string {
	h := sha256.New()
	h.Write([]byte(bh.PrevHash))
	h.Write([]byte(strconv.Itoa(int(bh.Height))))
	h.Write([]byte(bh.Type))

	h.Write(bh.StateHash)
	h.Write(bh.TransationHash)

	return hex.EncodeToString(h.Sum(nil))
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

// -----------------------------------------------------------------------------
// BlockBuilder

type BlockBuilder struct {
	prevHash     string
	height       uint
	blockType    BlkType
	states       storage.KVStore
	transactions []*SignedTransaction
}

func NewBlockBuilder(blockType BlkType) *BlockBuilder {
	return &BlockBuilder{
		blockType:    blockType,
		transactions: make([]*SignedTransaction, 0),
	}
}

func (bb *BlockBuilder) AddTxn(txn *SignedTransaction) error {
	if bb.blockType == BlkTypeConfig {
		return fmt.Errorf("unable to append txn to a config block: %T", txn.Txn)
	}

	bb.transactions = append(bb.transactions, txn)
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

func (bb *BlockBuilder) SetState(state storage.KVStore) *BlockBuilder {
	bb.states = state
	return bb
}

func (bb *BlockBuilder) Build() *Block {
	h := sha256.New()
	for _, txn := range bb.transactions {
		h.Write(txn.Hash())
	}

	header := BlockHeader{
		PrevHash:       bb.prevHash,
		Height:         bb.height,
		StateHash:      []byte(bb.states.Hash()),
		TransationHash: h.Sum(nil),
	}

	return &Block{
		BlockHeader:  &header,
		States:       bb.states,
		Transactions: bb.transactions,
	}
}
