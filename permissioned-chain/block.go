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
	Miner    string

	StateHash      string
	TransationHash string
}

// Hash computes the block hash based on data in block header
func (bh *BlockHeader) Hash() string {
	h := sha256.New()
	h.Write([]byte(bh.PrevHash))
	h.Write([]byte(strconv.Itoa(int(bh.Height))))
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

// GetWorldStateCopy returns a copy of block's world state
func (b *Block) GetWorldStateCopy() storage.KVStore {
	return b.States.Copy()
}

// GetConfig returns a copy of blockchain's config
func (b *Block) GetConfig() ChainConfig {
	return *GetConfigFromWorldState(b.States)
}

// GetTxn checks whether the txn is included in the block
func (b *Block) GetTxn(txnID string) *SignedTransaction {
	for _, txn := range b.Transactions {
		if txn.Txn.ID == txnID {
			return &txn
		}
	}
	return nil
}

// Verify verifies if a block is valid
func (b *Block) Verify(worldState storage.KVStore) error {
	config := GetConfigFromWorldState(worldState)

	// check miner
	if !CheckPariticipation(worldState, config, b.Miner) {
		return fmt.Errorf("miner %s is not a participant of the permissined chain", b.Miner)
	}

	// check # transaction not exceeds the limit (except for genesis)
	if b.Height > 0 && len(b.Transactions) > config.MaxTxnsPerBlk {
		return fmt.Errorf("too many transactions in block %s. Max: %d. Got: %d",
			b.Hash(), config.MaxTxnsPerBlk, len(b.Transactions))
	}

	// check hash
	h := sha256.New()
	for _, txn := range b.Transactions {
		h.Write(txn.HashBytes())
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if b.TransationHash != hash {
		return fmt.Errorf("block %s has inconsistent transaction hash", b.Hash())
	}

	// check state
	for _, txn := range b.Transactions {
		err := txn.Verify(worldState)
		if err != nil {
			return fmt.Errorf("block %s has invalid transaction: %t", b.Hash(), err)
		}
	}
	if hex.EncodeToString(worldState.Hash()) != b.StateHash {
		fmt.Println(worldState)
		return fmt.Errorf("block %s has different execution result from expected", b.Hash())
	}
	// set state
	b.States = worldState

	return nil
}

// DescribeTransactions return a string to describe txn info
func (b *Block) DescribeTransactions() string {
	description := ""
	for _, txn := range b.Transactions {
		description += txn.String() + " "
	}
	return fmt.Sprintf("%d Txns: [%s]", len(b.Transactions), description)
}

// -----------------------------------------------------------------------------
// BlockBuilder

type BlockBuilder struct {
	prevHash     string
	height       uint
	miner        string
	states       storage.KVStore
	transactions []SignedTransaction
}

func NewBlockBuilder() *BlockBuilder {
	return &BlockBuilder{
		transactions: make([]SignedTransaction, 0),
	}
}

func (bb *BlockBuilder) AddTxn(txn *SignedTransaction) error {
	bb.transactions = append(bb.transactions, *txn)
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
		h.Write(txn.HashBytes())
	}
	txnHash := h.Sum(nil)

	header := BlockHeader{
		PrevHash:       bb.prevHash,
		Height:         bb.height,
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
