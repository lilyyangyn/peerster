package blockchain

import (
	"crypto"
	"strconv"

	"go.dedis.ch/cs438/storage"
	"golang.org/x/xerrors"
)

// Block represents the basic building block of a blockc hain
type Block struct {
	*BlockHeader
	States       storage.KVStore
	Transactions []*Transaction
}

// BlkType helps to defer different types of txns & blocks
type BlkType string

const (
	ConfigType BlkType = "config"
	TxnType    BlkType = "mpctxn"
)

// BlockHeader is the header of block in the blockchain
type BlockHeader struct {
	PrevHash []byte
	Height   uint
	Type     BlkType

	StateHash      []byte
	TransationHash []byte
}

// Hash computes the block hash based on data in block header
func (bh *BlockHeader) Hash() []byte {
	h := crypto.SHA256.New()
	h.Write(bh.PrevHash)
	h.Write([]byte(strconv.Itoa(int(bh.Height))))
	h.Write([]byte(bh.Type))

	h.Write(bh.StateHash)
	h.Write(bh.TransationHash)

	return h.Sum(nil)
}

// HasTxn checks whether the txn is included in the block
func (b *Block) HasTxn(txnID string) bool {
	for _, txn := range b.Transactions {
		if txn.ID == txnID {
			return true
		}
	}
	return false
}

type BlockBuilder struct {
	prevHash     []byte
	height       uint
	blockType    BlkType
	states       storage.KVStore
	transactions []*Transaction
}

func NewBlockBuilder(blockType BlkType) *BlockBuilder {
	return &BlockBuilder{
		blockType:    blockType,
		transactions: make([]*Transaction, 0),
	}
}

func (bb *BlockBuilder) AddTxn(txn *Transaction) error {
	if bb.blockType == ConfigType {
		return xerrors.Errorf("unable to append txn to a config block: %T", txn)
	}

	bb.transactions = append(bb.transactions, txn)
	return nil
}

func (bb *BlockBuilder) AddConfig(config *ChainConfig) error {
	if bb.blockType != ConfigType {
		return xerrors.Errorf("unable to add configuration to a txn block: %T", config)
	}

	bb.states.Put(StateConfigKey, *config)
	return nil
}

func (bb *BlockBuilder) SetPrevHash(prevHash []byte) *BlockBuilder {
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
	h := crypto.SHA256.New()
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
