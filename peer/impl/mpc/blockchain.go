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

	record := txn.Data.(permissioned.MPCPropose)
	err := m.InitMPC(txn.ID, record.Prime, record.Initiator, record.Expression)
	if err != nil {
		return err
	}

	go func() {
		val, err := m.ComputeExpression(txn.ID, record.Expression, record.Prime)
		err = m.mpcCenter.Inform(txn.ID, MPCResult{result: val, err: err})
		if err != nil {
			log.Err(err).Send()
		}
	}()
	return nil
}
