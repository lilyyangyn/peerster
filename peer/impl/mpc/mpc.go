package mpc

// MPC handlers mpc related information
type MPC struct {
	id    int
	peers map[string]int
}

func (mpc *MPC) getPeerID(peer string) (int, bool) {
	id, ok := mpc.peers[peer]
	return id, ok
}
func NewMPC() *MPC {
	return &MPC{
		peers: map[string]int{},
	}
}
