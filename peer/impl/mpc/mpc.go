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

func (mpc *MPC) computeAdd(x int, y int, z bool) (int, error) {
	// TODO: change to MPC add/substract
	if z {
		return x - y, nil
	}
	return x + y, nil
}

func (mpc *MPC) computeMult(x int, y int) (int, error) {
	// TODO: change to MPC mult
	return x * y, nil
}
