package mpc

import (
	"fmt"
	"math/big"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/permissioned-chain"
)

func (m *MPCModule) PreMPCTxnCallback(config *permissioned.ChainConfig, txn *permissioned.Transaction) error {
	if txn.Type != permissioned.TxnTypePreMPC {
		return fmt.Errorf("invalid txn type. Expected: %s. Got: %s",
			permissioned.TxnTypePreMPC, txn.Type)
	}

	if m.conf.DisableMPC {
		mpc := NewMPC(txn.ID, big.Int{}, "", "")
		m.mpcCenter.RegisterMPC(mpc.id, mpc)
		m.mpcCenter.InformMPCStart(mpc.id)
		err := m.mpcCenter.InformMPCComplete(mpc.id, MPCResult{result: 0, err: nil})
		if err != nil {
			return err
		}

		// postMPC txn
		postID, err := m.bcModule.SendPostMPCTransaction(txn.ID, float64(0))
		if err == nil {
			log.Info().Msgf("send postMPC txn %s for MPC %s", postID, txn.ID)
		}
		return err
	}

	propose := txn.Data.(permissioned.MPCPropose)
	log.Info().Msgf("PreMPC Txn %s is confirmed. Start MPC {%s}...", txn.ID, propose.Expression)

	// add addr -> pubkey map
	err := m.pubkeyStore.Add(config.Participants)
	if err != nil {
		err = m.mpcCenter.InformMPCComplete(txn.ID, MPCResult{result: 0, err: err})
		return err
	}

	// init MPC
	err = m.initMPCWithBlockchain(txn.ID, config, &propose)
	m.mpcCenter.InformMPCStart(txn.ID)
	if err != nil {
		err = m.mpcCenter.InformMPCComplete(txn.ID, MPCResult{result: 0, err: err})
		return err
	}

	// start MPC
	val, err := m.ComputeExpression(txn.ID, propose.Expression, propose.Prime)
	if err != nil {
		return err
	}
	err = m.mpcCenter.InformMPCComplete(txn.ID, MPCResult{result: val, err: err})
	if err != nil {
		return err
	}

	// postMPC txn
	postID, err := m.bcModule.SendPostMPCTransaction(txn.ID, float64(val))
	if err != nil {
		return err
	} else {
		log.Info().Msgf("send postMPC txn %s for MPC %s", postID, txn.ID)
	}

	return nil
}
