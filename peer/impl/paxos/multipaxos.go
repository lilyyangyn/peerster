package paxos

import (
	"crypto"
	"strconv"

	"go.dedis.ch/cs438/types"
)

type MultiPaxos struct {
	TLC      uint
	Occupied bool
	*Paxos

	BlockCounter map[uint]int
	Blocks       map[uint]*types.BlockchainBlock
}

func NewMultiPaxos(id uint) *MultiPaxos {
	multipaxos := MultiPaxos{
		TLC:   0,
		Paxos: NewPaxos(id),

		BlockCounter: make(map[uint]int),
		Blocks:       make(map[uint]*types.BlockchainBlock),
	}

	return &multipaxos
}

func (multipaxos *MultiPaxos) createTLCBlock(val *types.PaxosValue, prevHash []byte) *types.BlockchainBlock {
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

const (
	Init = iota
	PhaseOne
	PhaseTwo
)

type StateMachine int
type Paxos struct {
	MaxID      uint
	PaxosState StateMachine

	Proposer     bool
	ProposeID    uint
	ProposeValue *types.PaxosValue

	AcceptID    uint
	AcceptValue *types.PaxosValue

	PromiseCounter int
	MaxIDInPromise uint
	ValueInPromise *types.PaxosValue

	AcceptCounter map[string]int
}

func NewPaxos(id uint) *Paxos {
	paxos := Paxos{
		PaxosState:    Init,
		MaxID:         0,
		Proposer:      false,
		ProposeID:     id,
		AcceptCounter: map[string]int{},
	}

	return &paxos
}

func (paxos *Paxos) joinPhaseOne() {
	paxos.PaxosState = PhaseOne
	paxos.PromiseCounter = 0
	paxos.MaxIDInPromise = 0
	paxos.ValueInPromise = nil
}

func (paxos *Paxos) joinPhaseTwo() {
	paxos.PaxosState = PhaseTwo
}
