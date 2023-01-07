package blockchain

import (
	"context"
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
		case prevHeight := <-m.minerChan:
			latestBlock := m.GetLatestBlock()
			if prevHeight != latestBlock.Height {
				log.Error().Msgf("miner mining on incoorect height. Expected: %d. Got: %d",
					prevHeight, latestBlock.Height)
				continue
			}

			log.Info().Msgf("Mining on height=%d...", prevHeight+1)
			config := latestBlock.GetConfig()
			newBlock := createBlock(ctx, txnPool,
				m.wallet.GetAddress().Hex, latestBlock, &config)
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
			log.Info().Msgf("Mined block %s on height=%d. Broadcasting...",
				newBlock.Hash(), newBlock.Height)

			// broadcast block
			participants := make(map[string]struct{})
			for p := range config.Participants {
				participants[p] = struct{}{}
			}
			err := m.broadcastBCBlkMessage(participants, newBlock)
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

	duration := getBlockTimeout(config)
	timeout := time.After(duration)
out:
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timeout:
			if txnCount > 0 {
				// if any txn, then produce the block
				break out
			}
		case signedTxn := <-txnPool.channel:
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

			// reset timeout if a txn is successfuly append
			timeout = time.After(duration)
		}
	}

	blkBuilder.SetState(worldState)

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
			log.Info().Msgf("Verifying block %s on height=%d...",
				block.Hash(), block.Height)

			// validate consensus
			expectedMiner := m.cr.getLatestMiner()
			if expectedMiner != block.Miner {
				log.Error().Msgf("invalid miner. Expected: %s. Got: %s",
					expectedMiner, block.Miner)
				continue
			}

			result := m.CheckBlockHeight(block)
			if result == permissioned.BlockCompareAdvance ||
				result == permissioned.BlockCompareNotInitialize {
				// block too advance. Syncing
				log.Info().Msgf("receive advance block on height %d. Syncing...", block.Height)
				err := m.sync(block.Miner)
				if err != nil {
					log.Err(err).Send()
				}
				continue
			} else if result != permissioned.BlockCompareMatched {
				log.Error().Msgf("block %s is invalid. Error code: %d",
					block.Hash(), result)
				continue
			}

			// append the block
			err := m.AppendBlock(block)
			if err != nil {
				log.Err(err).Send()
				continue
			}
			log.Info().Msgf("Append Block %s successfully",
				block.Hash())
			// notify outside for the received transactions
			go func() {
				m.wallet.Sync(block.States)

				config := permissioned.GetConfigFromWorldState(block.States)
				for _, signedTxn := range block.Transactions {
					err = m.watchRegistry.Tell(config, &signedTxn.Txn)
					if err != nil {
						log.Err(err).Send()
					}
				}
			}()

			// select next miner
			m.selectNextMiner(block)
		}
	}
}
