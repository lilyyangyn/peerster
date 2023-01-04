package blockchain

import (
	"fmt"

	"go.dedis.ch/cs438/permissioned-chain"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

// ProcessBCPrivateMsg is a callback function to handle the received BCPrivateMessage
func (m *BlockchainModule) ProcessBCPrivateMsg(msg types.Message, pkt transport.Packet) error {
	privMsg, ok := msg.(*types.BCPrivateMessage)
	if !ok {
		return fmt.Errorf("wrong type: %T", msg)
	}

	_, ok = privMsg.Recipients[m.account.GetAddress().Hex]
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

	if txnMsg.Txn.Txn.From == permissioned.ZeroAddress.Hex {
		return fmt.Errorf("cannot receive transaction created by zeroAddress")
	}

	err := txnMsg.Txn.Txn.Unmarshal()
	if err != nil {
		return err
	}
	txn := txnMsg.Txn

	m.txnPool.Push(&txn)

	return nil
}

// ProcessBCBlkMsg is a callback function to handle the received BCBlkMessage
func (m *BlockchainModule) ProcessBCBlkMsg(msg types.Message, pkt transport.Packet) error {
	blkMsg, ok := msg.(*types.BCBlkMessage)
	if !ok {
		return fmt.Errorf("wrong type: %T", msg)
	}

	// rebuild the block
	txns := make([]permissioned.SignedTransaction, 0, len(blkMsg.Txns))
	for _, txn := range blkMsg.Txns {
		err := txn.Txn.Unmarshal()
		if err != nil {
			fmt.Println(err)
			return err
		}
		txns = append(txns, txn)
	}
	block := permissioned.Block{
		BlockHeader:  &blkMsg.BlkHeader,
		Transactions: txns,
	}

	// if is genesis block. Directly set
	if blkMsg.BlkHeader.Height == 0 {
		err := m.SetGenesisBlock(&block)
		if err == nil {
			close(m.bcReadyChan)
		}
		return nil
	}

	// Otherwise,append the block
	m.blkChan <- &block

	return nil
}
