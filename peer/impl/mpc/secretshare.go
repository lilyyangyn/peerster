package mpc

import (
	"github.com/rs/zerolog/log"
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

func (m *MPCModule) shareSecret(key string, peers []string) error {
	log.Info().Msgf("%s: start share secret, key: %s, peers: %s",
		m.conf.Socket.GetAddress(), key, peers)

	value, ok := m.mpc.getValue(key)
	if !ok {
		return xerrors.Errorf("no valid value is found for key %s", key)
	}

	// generate the list of MPC id
	peerIDs, err := m.mpc.getPeerIDs(peers)
	if err != nil {
		return err
	}

	// generate shared secrets
	results, err := m.shamirSecretShare(value, peerIDs)
	if err != nil {
		return err
	}

	// log.Info().Msgf("%s: generated sss result: %s: %s", m.conf.Socket.GetAddress(), key, results)

	// send shared secrets
	for idx, result := range results {
		err := m.sendShareMessage(
			peers[idx], peerIDs[idx], key+"|"+peers[idx], result)
		if err != nil {
			return err
		}
	}

	return nil
}

// sendShareMessage sends the share secret in encrypted message
func (m *MPCModule) sendShareMessage(peer string, id int, key string, value int) error {
	shareMsg := types.MPCShareMessage{
		ReqID: m.mpc.id,
		Value: types.MPCSecretValue{
			Owner: m.conf.Socket.GetAddress(),
			Key:   key,
			Value: value,
		},
	}
	shareMsgMarshal, err := m.CreateMsg(shareMsg)
	if err != nil {
		return err
	}

	return m.SendEncryptedMessage(shareMsgMarshal, peer)
}
