package mpc

import (
	"fmt"
	"math/big"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

// ProcessShareMsg is a callback function to handle received secret share message
func (m *MPCModule) ProcessMPCShareMsg(msg types.Message, pkt transport.Packet) error {
	secretMsg, ok := msg.(*types.MPCShareMessage)
	if !ok {
		return fmt.Errorf("wrong type: %T", msg)
	}

	// ignore message with wrong id
	// mpc := m.mpcCenter.GetMPC(secretMsg.ReqID)

	log.Debug().Msgf("%s: mpc value for req %s, %s:%s",
		m.conf.Socket.GetAddress(), secretMsg.ReqID, secretMsg.Value.Key, secretMsg.Value.Value)

	// MPC operation
	valueBig, ok := new(big.Int).SetString(secretMsg.Value.Value, 10)
	if !ok {
		return fmt.Errorf("fail to parse big integer: %s", secretMsg.Value.Value)
	}
	// mpc.addValue(secretMsg.Value.Key, *valueBig)

	m.mpcCenter.AddValue(secretMsg.ReqID, secretMsg.Value.Key, *valueBig)

	return nil
}

// ProcessShareMsg is a callback function to handle received secret share message
func (m *MPCModule) ProcessMPCInterpolationMsg(msg types.Message, pkt transport.Packet) error {
	interpolationMsg, ok := msg.(*types.MPCInterpolationMessage)
	if !ok {
		return fmt.Errorf("wrong type: %T", msg)
	}

	// ignore message with wrong id
	// mpc := m.mpcCenter.GetMPC(interpolationMsg.ReqID)

	log.Debug().Msgf("%s: interpolation msg req: %s, owner: %s, value: %s",
		m.conf.Socket.GetAddress(), interpolationMsg.ReqID, interpolationMsg.Owner, interpolationMsg.Value)

	// Add to intermediate value
	valueBig, ok := new(big.Int).SetString(interpolationMsg.Value, 10)
	if !ok {
		return fmt.Errorf("fail to parse big integer: %s", interpolationMsg.Value)
	}
	// mpc.addValue(interpolationMsg.Owner+"|InterpolationResult", *valueBig)

	m.mpcCenter.AddValue(interpolationMsg.ReqID,
		interpolationMsg.Owner+"|InterpolationResult",
		*valueBig)

	return nil
}
