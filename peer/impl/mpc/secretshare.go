package mpc

import (
	"math/big"

	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

func (m *MPCModule) shamirSecretShare(value int, peers []int) ([]int, error) {
	// no redundancy. assume participants will not down during MPC
	degree := len(peers)

	p := NewRandomPolynomial(value, degree)
	results := make([]int, degree)
	for idx, id := range peers {
		// id should not equal to zero, or the secret will be directly leaked
		if id == 0 {
			return []int{}, xerrors.Errorf("illegal input x equals to 0")
		}
		results[idx] = p.compute(id)
	}

	return results, nil
}

func (m *MPCModule) shareSecret(key string, mpc *MPC) error {
	// log.Printf("%s: start share secret, key: %s, peers: %s",
	// 	m.conf.Socket.GetAddress(), key, peers)

	value, ok := mpc.getValue(key)
	if !ok {
		return xerrors.Errorf("no valid value is found for key %s", key)
	}

	// generate the list of MPC id
	peerIDs, err := mpc.getPeerIDs()
	if err != nil {
		return err
	}

	// generate shared secrets
	// results, err := m.shamirSecretShare(value, peerIDs)
	results, err := m.shamirSecretShareZp(value, mpc.prime, peerIDs)
	if err != nil {
		return err
	}

	// log.Printf("%s: generated sss result: %s: %s", m.conf.Socket.GetAddress(), key, results)

	// send shared secrets
	peers := mpc.getParticipants()
	for idx, result := range results {
		err := m.sendShareMessage(
			mpc.id, peers[idx], int(peerIDs[idx].Uint64()), key+"|"+peers[idx], result)
		if err != nil {
			return err
		}
	}

	return nil
}

// sendShareMessage sends the share secret in encrypted message
func (m *MPCModule) sendShareMessage(uniqID string, peer string, id int, key string, value big.Int) error {
	shareMsg := types.MPCShareMessage{
		ReqID: uniqID,
		Value: types.MPCSecretValue{
			Owner: m.conf.Socket.GetAddress(),
			Key:   key,
			Value: value.Text(10),
		},
	}
	shareMsgMarshal, err := m.CreateMsg(shareMsg)
	if err != nil {
		return err
	}
	return m.SendEncryptedMessage(shareMsgMarshal, peer)
}
