package paxos

import (
	"crypto"
	"strconv"

	"go.dedis.ch/cs438/types"
)

// const (
// 	Idle = iota
// 	Busy
// )

// type MultiPaxosStateMachine int

type MultiPaxos struct {
	TLC uint
	*Paxos

	// MultiPaxosState MultiPaxosStateMachine

	BlockCounter map[uint]int
	Blocks       map[uint]*types.BlockchainBlock
}

func NewMultiPaxos(id uint) *MultiPaxos {
	multipaxos := MultiPaxos{
		TLC:   0,
		Paxos: NewPaxos(id),

		// MultiPaxosState: Idle,

		BlockCounter: make(map[uint]int),
		Blocks:       make(map[uint]*types.BlockchainBlock),
	}

	return &multipaxos
}

func (multipaxos *MultiPaxos) CreateTLCBlock(val *types.PaxosValue, prevHash []byte) *types.BlockchainBlock {
	if len(prevHash) == 0 {
		prevHash = make([]byte, 32)
	}
	// compute block hash
	currClock := multipaxos.TLC
	// create block
	block := &types.BlockchainBlock{
		Index:    currClock,
		Value:    *val,
		PrevHash: prevHash,
	}

	h := crypto.SHA256.New()
	h.Write([]byte(strconv.Itoa(int(block.Index))))
	h.Write([]byte(block.Value.UniqID))
	h.Write([]byte(block.Value.Filename))
	h.Write([]byte(block.Value.Metahash))
	h.Write(block.PrevHash)
	blockHash := h.Sum(nil)
	block.Hash = blockHash

	return block
}
