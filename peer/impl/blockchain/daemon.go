package blockchain

import (
	"context"
	"fmt"
	"time"

	permissioned "go.dedis.ch/cs438/peer/impl/blockchain/permissionedchain"
)

// -----------------------------------------------------------------------------
// Miner

func (m *BlockchainModule) Mine(ctx context.Context) {
	m.txnPool = NewTxnPool(ctx)
out:
	for {
		select {
		case <-ctx.Done():
			return
		default:
			latestBlock := m.blockchain.GetLatestBlock()
			newBlock := m.createBlock(ctx, &latestBlock)

			// validate block
			if err := m.blockchain.ValidateBlock(newBlock); err != nil {
				// reprocess txn if fail to validate block
				m.txnPool.PushSeveral(newBlock.Transactions)
				continue out
			}

			// TODO: broadcast block
		}
	}
}

func (m *BlockchainModule) createBlock(ctx context.Context,
	prevBlock *permissioned.Block) *permissioned.Block {

	worldState := prevBlock.GetWorldState()
	config := prevBlock.GetConfig()
	blkBuilder := permissioned.NewBlockBuilder(permissioned.BlkTypeTxn, config.MaxTxnsPerBlk)
	blkBuilder.SetPrevHash(prevBlock.Hash()).SetHeight(prevBlock.Height + 1)
	txnCount := 0
out:
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Duration(config.WaitTimeout)):
			break out
		default:
			signedTxn := m.txnPool.Pull()
			err := signedTxn.Verify(worldState, &config)
			if err != nil {
				continue
			}
			blkBuilder.AddTxn(signedTxn)
			txnCount++

			if txnCount == config.MaxTxnsPerBlk {
				break out
			}
		}
	}

	blkBuilder.SetState(worldState)
	// TODO: consensus: PoW? PoS?

	return blkBuilder.Build()
}

// -----------------------------------------------------------------------------
// Accepter

func (m *BlockchainModule) VerifyBlock(ctx context.Context) {
	m.blkChan = make(chan *permissioned.Block, 10)
	for {
		select {
		case <-ctx.Done():
			return
		case block := <-m.blkChan:
			// TODO: validate consensus
			fmt.Println(block)
		}
	}
}
