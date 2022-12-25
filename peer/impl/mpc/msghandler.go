package mpc

import (
	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// ProcessShareMsg is a callback function to handle received secret share message
func (m *MPCModule) ProcessMPCShareMsg(msg types.Message, pkt transport.Packet) error {
	secretMsg, ok := msg.(*types.MPCShareMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	// ignore message with wrong id
	if secretMsg.ReqID != m.mpc.id {
		return nil
	}

	log.Info().Msgf("mpc value for req %d, %s:%d",
		secretMsg.ReqID, secretMsg.Value.Key, secretMsg.Value.Value)

	// MPC operation
	m.mpc.addValue(secretMsg.Value.Key, secretMsg.Value.Value)

	return nil
}
