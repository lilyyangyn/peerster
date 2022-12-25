package mpc

import (
	"sync"

	"golang.org/x/xerrors"
)

// MPC handlers mpc related information
type MPC struct {
	*sync.RWMutex
	id         int
	peers      map[string]int
	interStore map[string]int
}

func (mpc *MPC) addPeers(peersMap map[string]int) error {
	mpc.Lock()
	defer mpc.Unlock()
	for peer, id := range peersMap {
		mpc.peers[peer] = id
	}
	return nil
}

func (mpc *MPC) getPeerID(peer string) (int, bool) {
	mpc.RLock()
	defer mpc.RUnlock()
	id, ok := mpc.peers[peer]
	return id, ok
}

func (mpc *MPC) getPeerIDs(peers []string) ([]int, error) {
	peerIDs := make([]int, len(peers))
	for idx, peer := range peers {
		id, ok := mpc.getPeerID(peer)
		if !ok {
			return []int{}, xerrors.Errorf("no id for peer %s", peer)
		}
		peerIDs[idx] = id
	}
	return peerIDs, nil
}

func (mpc *MPC) addValue(key string, value int) bool {
	mpc.Lock()
	defer mpc.Unlock()
	old, ok := mpc.interStore[key]
	if ok && old != value {
		return false
	}
	mpc.interStore[key] = value
	return true
}
func (mpc *MPC) getValue(key string) (int, bool) {
	mpc.RLock()
	defer mpc.RUnlock()
	value, ok := mpc.interStore[key]
	return value, ok
}

func NewMPC(id int) *MPC {
	return &MPC{
		&sync.RWMutex{},
		id,
		map[string]int{},
		map[string]int{},
	}
}
