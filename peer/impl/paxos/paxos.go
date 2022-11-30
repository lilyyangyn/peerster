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

type PaxosState int

type Paxos struct {
	maxID uint
	state PaxosState

	isPropose  bool
	proposeID  uint
	proposeVal *types.PaxosValue
	firstVal   *types.PaxosValue

	isAccept    bool
	acceptID    uint
	acceptValue *types.PaxosValue

	promiseCounter       int
	maxAcceptIDInPromise uint

	acceptIDs map[string]int
}

func NewPaxos() *Paxos {
	paxos := Paxos{
		state:     Init,
		maxID:     0,
		isPropose: false,
		acceptIDs: map[string]int{},
	}

	return &paxos
}

func (paxos *Paxos) IsProposer() bool {
	return paxos.isPropose
}

func (paxos *Paxos) GetProposedInfo() (bool, uint, *types.PaxosValue) {
	return paxos.isPropose, paxos.proposeID, paxos.proposeVal
}

func (paxos *Paxos) GetAcceptedInfo() (bool, uint, *types.PaxosValue) {
	return paxos.isAccept, paxos.acceptID, paxos.acceptValue
}

/** Private Helpfer Functions **/

func (paxos *Paxos) setFirstValue(val *types.PaxosValue) {
	paxos.firstVal = val
}

func (paxos *Paxos) joinPhaseOne(id uint) bool {
	if paxos.state == Complete {
		return false
	}

	paxos.state = PhaseOne
	paxos.promiseCounter = 0
	paxos.maxAcceptIDInPromise = 0
	if id > 0 {
		paxos.isPropose = true
		paxos.proposeID = id
		paxos.proposeVal = paxos.firstVal
	}

	return true
}

func (paxos *Paxos) joinPhaseTwo() bool {
	if paxos.state == Complete {
		return false
	}
	paxos.state = PhaseTwo
	return true
}

func (paxos *Paxos) recordID(id uint) bool {
	if id > paxos.maxID {
		paxos.maxID = id
		return true
	}
	return false
}

func (paxos *Paxos) recordPromise(id uint, acceptedID uint,
	acceptedValue *types.PaxosValue, threshold int) bool {
	// only when in phase one
	if paxos.state != PhaseOne {
		return false
	}
	// should be promised to the prepare mesage by ourself
	if id != paxos.proposeID {
		return false
	}

	// update value to acceptedValue
	if acceptedID > paxos.maxAcceptIDInPromise {
		paxos.maxAcceptIDInPromise = acceptedID
		paxos.proposeVal = acceptedValue
	}
	paxos.promiseCounter++
	if paxos.promiseCounter >= threshold {
		paxos.joinPhaseTwo()
		return true
	}

	return false
}

func (paxos *Paxos) RecordAccept(id uint, value *types.PaxosValue, threshold int) bool {
	// for proposer: only when in phase two
	if paxos.isPropose && paxos.state != PhaseTwo {
		if id == paxos.proposeID {
			return false
		}
	}

	paxos.acceptIDs[value.UniqID]++
	if paxos.acceptIDs[value.UniqID] >= threshold {
		paxos.state = Complete
		return true
	}

	return false
}

func (paxos *Paxos) accept(id uint, value *types.PaxosValue) bool {
	if id == paxos.maxID {
		paxos.isAccept = true
		paxos.acceptID = id
		paxos.acceptValue = value
		return true
	}
	return false
}
