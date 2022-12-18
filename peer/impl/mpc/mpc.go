package mpc

import "golang.org/x/xerrors"

// MPC handlers mpc related information
type MPC struct {
	id    int
	peers map[string]int
}

func (mpc *MPC) getPeerID(peer string) (int, bool) {
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

func NewMPC() *MPC {
	return &MPC{
		peers: map[string]int{},
	}
}
