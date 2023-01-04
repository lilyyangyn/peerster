package blockchain

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"
	permissioned "go.dedis.ch/cs438/permissioned-chain"
)

// -----------------------------------------------------------------------------
// Miner

func (m *BlockchainModule) Mine(ctx context.Context, txnPool *TxnPool) {
	// wait until the blockchain's genesis block is set
	<-m.bcReadyChan
	log.Info().Msgf("Start mining")
out:
	for {
		select {
		case <-ctx.Done():
			return
		default:
			latestBlock := m.GetLatestBlock()
			config := latestBlock.GetConfig()
			newBlock := createBlock(ctx, txnPool,
				m.account.GetAddress().Hex, latestBlock, &config)
			if newBlock == nil {
				continue
			}

			// validate block
			if m.CheckBlockHeight(newBlock) != permissioned.BlockCompareMatched {
				log.Error().Msgf("mined block has an invalid block height %d", newBlock.Height)
				// put the transactions back to the pool
				m.txnPool.PushSeveral(newBlock.Transactions)
				continue out
			}

			// broadcast block
			err := m.broadcastBCBlkMessage(config.Participants, newBlock)
			if err != nil {
				log.Err(err).Send()
			}
		}
	}
}

func createBlock(ctx context.Context, txnPool *TxnPool,
	miner string, prevBlock *permissioned.Block,
	config *permissioned.ChainConfig) *permissioned.Block {

	worldState := prevBlock.GetWorldStateCopy()
	blkBuilder := permissioned.NewBlockBuilder()
	blkBuilder.SetPrevHash(prevBlock.Hash()).
		SetHeight(prevBlock.Height + 1).
		SetMiner(miner)
	txnCount := 0

	duration, err := time.ParseDuration(config.WaitTimeout)
	if err != nil {
		log.Warn().Msgf(`Dangerous: unrecognize max block timeout. 
			Miner's max mining period can be infinitive`)
		duration = math.MaxInt64
	}

out:
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(duration):
			fmt.Println("-----------------------")
			break out
		case signedTxn := <-txnPool.Pull():
			// coinbase trasaction can only be created by miner
			if signedTxn.Txn.Type == permissioned.TxnTypeCoinbase {
				continue
			}

			err := signedTxn.Verify(worldState, config)
			if err != nil {
				log.Err(err).Send()
				continue
			}
			err = blkBuilder.AddTxn(signedTxn)
			if err != nil {
				break out
			}
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
	log.Info().Msgf("Start verifying")

	m.blkChan = make(chan *permissioned.Block, 10)
	for {
		select {
		case <-ctx.Done():
			return
		case block := <-m.blkChan:
			// TODO: validate consensus
			// TODO: catchup

			err := m.AppendBlock(block)
			if err != nil {
				log.Err(err).Send()
			}
		}
	}
}
