package mpc

import (
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

func (m *MPCModule) shamirSecretShare(value int, peers []int) (results []int, err error) {
	// no redundancy. assume participants will not down during MPC
	degree := len(peers)

	p := NewRandomPolynomial(value, degree)
	for idx, id := range peers {
		// id should not equal to zero, or the secret will be directly leaked
		if id == 0 {
			return []int{}, xerrors.Errorf("illegal input x equals to 0")
		}
		results[idx] = p.compute(id)
	}

	return results, nil
}

func (m *MPCModule) shareSecret(key string, peers []string) error {
	value, ok := m.valueDB.get(key)
	if !ok {
		return xerrors.Errorf("no valid value is found for key %s", key)
	}

	// generate the list of MPC id
	peerIDs := make([]int, len(peers))
	for idx, peer := range peers {
		id, ok := m.getPeerID(peer)
		if !ok {
			return xerrors.Errorf("no id for peer %s", peer)
		}
		peerIDs[idx] = id
	}

	// generate shared secrets
	results, err := m.shamirSecretShare(value, peerIDs)
	if err != nil {
		return err
	}

	// send shared secrets
	for idx, result := range results {
		err := m.sendShareMessage(peers[idx], peerIDs[idx], result)
		if err != nil {
			return err
		}
	}

	return nil
}

// sendShareMessage sends the share secret in encrypted message
func (m *MPCModule) sendShareMessage(peer string, id int, value int) error {
	shareMsg := types.MPCShareMessage{
		Origin: m.conf.Socket.GetAddress(),
		ID:     id,
		Value:  value,
	}
	shareMsgMarshal, err := m.CreateMsg(shareMsg)
	if err != nil {
		return err
	}

	return m.SendEncryptedMessage(shareMsgMarshal, peer)
}
