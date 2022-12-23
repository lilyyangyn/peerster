package paxos

import (
	"go.dedis.ch/cs438/types"
)

type MultipaxosHandler interface {
	Callback(m *PaxosModule, content types.PaxosValueContent) error
}

type Multipaxos struct {
	TLC      uint
	occupied bool
	*Paxos

	BlockCounter map[uint]int
	Blocks       map[uint]*types.BlockchainBlock
}

func NewMultipaxos(id uint) *Multipaxos {
	multipaxos := Multipaxos{
		TLC:   0,
		Paxos: NewPaxos(id),

		BlockCounter: make(map[uint]int),
		Blocks:       make(map[uint]*types.BlockchainBlock),
	}

	return &multipaxos
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

	proposer     bool
	ProposeID    uint
	proposeValue *types.PaxosValue

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
		proposer:      false,
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
