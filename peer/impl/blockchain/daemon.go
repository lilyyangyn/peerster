package blockchain

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	permissioned "go.dedis.ch/cs438/permissionedchain"
)

// -----------------------------------------------------------------------------
// Miner

func (m *BlockchainModule) Mine(ctx context.Context, txnPool *TxnPool) {
out:
	for {
		select {
		case <-ctx.Done():
			return
		default:
			latestBlock := m.blockchain.GetLatestBlock()
			config := latestBlock.GetConfig()
			newBlock := createBlock(ctx, txnPool, &latestBlock, &config)
			if newBlock == nil {
				return
			}

			// validate block
			if m.blockchain.CheckBlockHeight(newBlock) != permissioned.BlockCompareMatched {
				// put the transactions back to the pool
				m.txnPool.PushSeveral(newBlock.Transactions)
				continue out
			}

			// broadcast block
			err := m.broadcastBCBlkMessage(config.Participants, newBlock)
			if err != nil {
				log.Err(err)
			}
		}
	}
}

func createBlock(ctx context.Context, txnPool *TxnPool,
	prevBlock *permissioned.Block,
	config *permissioned.ChainConfig) *permissioned.Block {

	worldState := prevBlock.GetWorldState()
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
			signedTxn := txnPool.Pull()
			err := signedTxn.Verify(worldState, config)
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
			// TODO: catchup

			err := m.blockchain.AppendBlock(block)
			if err != nil {
				log.Err(err)
			}
		}
	}
}
