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

	if m.wallet == nil {
		return nil
	}

	_, ok = privMsg.Recipients[m.wallet.GetAddress().Hex]
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
			return err
		}
		txns = append(txns, txn)
	}
	block := permissioned.Block{
		BlockHeader:  &blkMsg.BlkHeader,
		Transactions: txns,
	}

	return m.processBlk(&block)
}

// ProcessBCAskSyncMsg is a callback function to handle the received BCAskSyncMessage
func (m *BlockchainModule) ProcessBCAskSyncMsg(msg types.Message, pkt transport.Packet) error {
	askMsg, ok := msg.(*types.BCAskSyncMessage)
	if !ok {
		return fmt.Errorf("wrong type: %T", msg)
	}

	// fetch blocks until the latest height
	fullBlks := m.GetBlockUntil(askMsg.LatestHeight)
	blocks := make([]permissioned.Block, 0, len(fullBlks))
	for i := len(fullBlks) - 1; i > -1; i-- {
		block := permissioned.Block{
			BlockHeader:  fullBlks[i].BlockHeader,
			Transactions: fullBlks[i].Transactions,
		}
		blocks = append(blocks, block)
	}

	// send sync
	err := m.sendBCSyncMessage(askMsg.UniqID, blocks, askMsg.Origin)

	return err
}

// ProcessBCSyncMsg is a callback function to handle the received BCSyncMessage
func (m *BlockchainModule) ProcessBCSyncMsg(msg types.Message, pkt transport.Packet) error {
	synMsg, ok := msg.(*types.BCSyncMessage)
	if !ok {
		return fmt.Errorf("wrong type: %T", msg)
	}

	// process sync
	latestBlk := m.GetLatestBlock()
	for _, block := range synMsg.Blocks {
		if block.Height <= latestBlk.Height {
			continue
		}
		err := m.processBlk(&block)
		if err != nil {
			return err
		}
	}

	return nil
}
