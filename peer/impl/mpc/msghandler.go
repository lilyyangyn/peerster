package mpc

import (
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
	if secretMsg.ReqID != m.id {
		return nil
	}

	//log.Printf("mpc value for round %d: %s-%s", secretMsg.Value, secretMsg.Value.Owner, secretMsg.Value.Key)

	// TODO: MPC operation
	return nil
}
