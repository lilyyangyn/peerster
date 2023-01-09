package mpc

import (
	"fmt"
	"math/big"
	"sync"
	"time"
)

type MPCState int

const (
	MPCStateAlive MPCState = iota
	MPCStateFinish
)

// MPC handlers mpc related information
type MPC struct {
	*sync.RWMutex
	id           string
	prime        big.Int
	initiator    string
	expression   string
	participants []string
	peers        map[string]int
	interStore   map[string]big.Int

	state  MPCState
	result *MPCResult
}

type MPCResult struct {
	result int
	err    error
}

func (mpc *MPC) getParticipants() []string {
	mpc.RLock()
	defer mpc.RUnlock()

	participants := mpc.participants
	return participants
}

func (mpc *MPC) addPeers(peersMap map[string]int) error {
	mpc.Lock()
	defer mpc.Unlock()

	if mpc.state == MPCStateFinish {
		return fmt.Errorf("MPC has already finished")
	}

	for peer, id := range peersMap {
		mpc.peers[peer] = id
	}
	return nil
}

func (mpc *MPC) addParticipants(participants []string) error {
	mpc.Lock()
	defer mpc.Unlock()

	if mpc.state == MPCStateFinish {
		return fmt.Errorf("MPC has already finished")
	}

	mpc.participants = append(mpc.participants, participants...)
	return nil
}

func (mpc *MPC) getPeerID(peer string) (int, bool) {
	mpc.RLock()
	defer mpc.RUnlock()

	if mpc.state == MPCStateFinish {
		return 0, false
	}

	id, ok := mpc.peers[peer]
	return id, ok
}

func (mpc *MPC) addValue(key string, value big.Int) bool {
	mpc.Lock()
	defer mpc.Unlock()

	if mpc.state == MPCStateFinish {
		return false
	}

	mpc.interStore[key] = value
	return true
}

func (mpc *MPC) addValues(values map[string]big.Int) bool {
	mpc.Lock()
	defer mpc.Unlock()

	if mpc.state == MPCStateFinish {
		return false
	}

	for k, v := range values {
		mpc.interStore[k] = v
	}

	return true
}

func (mpc *MPC) getValue(key string) (big.Int, bool) {
	mpc.RLock()
	defer mpc.RUnlock()

	if mpc.state == MPCStateFinish {
		return big.Int{}, false
	}

	value, ok := mpc.interStore[key]
	return value, ok
}

func (mpc *MPC) getResult() (*MPCResult, error) {
	mpc.RLock()
	defer mpc.RUnlock()

	if mpc.state != MPCStateFinish {
		return nil, fmt.Errorf("mpc has not finished yet")
	}

	return mpc.result, nil
}

func (mpc *MPC) finalize(result MPCResult) error {
	mpc.Lock()
	defer mpc.Unlock()

	// if mpc.state == MPCStateFinish {
	// 	return fmt.Errorf("cannot finalize an already finished MPC")
	// }

	mpc.state = MPCStateFinish
	mpc.result = &result

	// clear states
	// mpc.peers = nil
	// mpc.interStore = nil

	return nil
}

func (mpc *MPC) getPeerIDs() ([]big.Int, error) {
	peers := mpc.getParticipants()
	peerIDs := make([]big.Int, len(peers))
	for idx, peer := range peers {
		id, ok := mpc.getPeerID(peer)
		if !ok {
			return []big.Int{}, fmt.Errorf("no id for peer %s", peer)
		}
		peerIDs[idx] = *big.NewInt(int64(id))
	}
	return peerIDs, nil
}

func (mpc *MPC) waitValueFromTemp(key string) big.Int {
	value, ok := mpc.getValue(key)
	for !ok {
		// Busy wait here
		time.Sleep(time.Millisecond * 1)
		value, ok = mpc.getValue(key)
	}
	return value
}

func NewMPC(id string, prime big.Int, initiator string,
	expression string) *MPC {
	return &MPC{
		RWMutex:      &sync.RWMutex{},
		id:           id,
		prime:        prime,
		initiator:    initiator,
		expression:   expression,
		participants: []string{},
		peers:        map[string]int{},
		interStore:   map[string]big.Int{},
	}
}
