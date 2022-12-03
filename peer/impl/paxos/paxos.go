package paxos

import (
	"go.dedis.ch/cs438/types"
)

const (
	Init = iota
	PhaseOne
	PhaseTwo
	Complete
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

func (paxos *Paxos) JoinPhaseOne() {
	paxos.PaxosState = PhaseOne
	paxos.PromiseCounter = 0
	paxos.MaxIDInPromise = 0
	paxos.ValueInPromise = nil
}

func (paxos *Paxos) JoinPhaseTwo() {
	paxos.PaxosState = PhaseTwo
}
