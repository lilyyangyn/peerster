package mpc

import (
	"log"

	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// ProcessShareMsg is a callback function to handle received secret share message
func (m *MPCModule) ProcessShareMsg(msg types.Message, pkt transport.Packet) error {
	secretMsg, ok := msg.(*types.MPCShareMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	// ignore message with wrong id
	if secretMsg.ID != m.id {
		return nil
	}

	// decrypt value
	valueInByte, err := m.DecryptAsymetric(secretMsg.Value)
	if err != nil {
		return err
	}
	log.Printf("decrypted value %s from %s", valueInByte, secretMsg.Origin)

	// TODO: convert int to byte
	// TODO: MPC operation
	return nil
}
