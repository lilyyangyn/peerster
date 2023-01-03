package blockchain

import (
	"fmt"

	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

// ProcessBCPrivateMsg is a callback function to handle the received BCPrivateMessage
func (m *BlockchainModule) ProcessBCPrivateMsg(msg types.Message, pkt transport.Packet) error {
	privMsg, ok := msg.(*types.BCPrivateMessage)
	if !ok {
		return fmt.Errorf("wrong type: %T", msg)
	}

	_, ok = privMsg.Recipients[m.account.GetAddress()]
	if ok {
		// process the message if in the recipients list
		newPkt := transport.Packet{
			Header: pkt.Header,
			Msg:    privMsg.Msg,
		}
		err := m.conf.MessageRegistry.ProcessPacket(newPkt)
		return err
	}
	return nil
}

// ProcessBCTxnMsg is a callback function to handle the received BCTxnMessag
func (m *BlockchainModule) ProcessBCTxnMsg(msg types.Message, pkt transport.Packet) error {
	txnMsg, ok := msg.(*types.BCTxnMessag)
	if !ok {
		return fmt.Errorf("wrong type: %T", msg)
	}

	m.txnPool.Push(&txnMsg.Txn)

	return nil
}

// ProcessBCBlkMsg is a callback function to handle the received BCBlkMessage
func (m *BlockchainModule) ProcessBCBlkMsg(msg types.Message, pkt transport.Packet) error {
	blkMsg, ok := msg.(*types.BCBlkMessage)
	if !ok {
		return fmt.Errorf("wrong type: %T", msg)
	}

	m.blkChan <- &blkMsg.Blk

	return nil
}
