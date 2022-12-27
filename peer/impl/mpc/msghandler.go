package mpc

import (
	"math/big"

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

	log.Printf("%s: mpc value for req %d, %s:%s",
		m.conf.Socket.GetAddress(), secretMsg.ReqID, secretMsg.Value.Key, secretMsg.Value.Value)

	// MPC operation
	valueBig, _ := new(big.Int).SetString(secretMsg.Value.Value, 10)
	m.mpc.addValue(secretMsg.Value.Key, *valueBig)

	return nil
}

// ProcessShareMsg is a callback function to handle received secret share message
func (m *MPCModule) ProcessMPCInterpolationMsg(msg types.Message, pkt transport.Packet) error {
	interpolationMsg, ok := msg.(*types.MPCInterpolationMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	// ignore message with wrong id
	if interpolationMsg.ReqID != m.mpc.id {
		return nil
	}

	log.Printf("%s: interpolation msg req: %d, owner: %s, value: %s",
		m.conf.Socket.GetAddress(), interpolationMsg.ReqID, interpolationMsg.Owner, interpolationMsg.Value)

	// Add to intermediate value
	valueBig, _ := new(big.Int).SetString(interpolationMsg.Value, 10)
	m.mpc.addValue(interpolationMsg.Owner+"|InterpolationResult", *valueBig)

	return nil
}
