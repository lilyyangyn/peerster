package mpc

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/permissioned-chain"
)

func (m *MPCModule) PreMPCTxnCallback(config *permissioned.ChainConfig, txn *permissioned.Transaction) error {
	if txn.Type != permissioned.TxnTypePreMPC {
		return fmt.Errorf("invalid txn type. Expected: %s. Got: %s",
			permissioned.TxnTypePreMPC, txn.Type)
	}

	// add addr -> pubkey map
	err := m.pubkeyStore.Add(config.Participants)
	if err != nil {
		err = m.mpcCenter.Inform(txn.ID, MPCResult{result: 0, err: err})
		return err
	}

	// init MPC
	propose := txn.Data.(permissioned.MPCPropose)
	err = m.initMPCWithBlockchain(txn.ID, config, &propose)
	if err != nil {
		err = m.mpcCenter.Inform(txn.ID, MPCResult{result: 0, err: err})
		return err
	}

	// start MPC
	go func() {
		val, err := m.ComputeExpression(txn.ID, propose.Expression, propose.Prime)
		err = m.mpcCenter.Inform(txn.ID, MPCResult{result: val, err: err})
		if err != nil {
			log.Err(err).Send()
		}

		// TODO: postMPC txn
		postID, err := m.bcModule.SendPostMPCTransaction(txn.ID, float64(val))
		if err != nil {
			log.Err(err).Send()
		} else {
			log.Info().Msgf("send postMPC txn %s", postID)
		}
	}()

	return nil
}
