package blockchain

import (
	"sync"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/permissioned-chain"
)

type CreditRecords struct {
	*sync.RWMutex
	records     map[string]float64
	latestMiner string
}

func NewCreditRecords() *CreditRecords {
	return &CreditRecords{
		RWMutex: &sync.RWMutex{},
		records: map[string]float64{},
	}
}

func (c *CreditRecords) getLatestMiner() string {
	c.RLock()
	defer c.RUnlock()

	return c.latestMiner
}

func (c *CreditRecords) advanceAndSelect(block *permissioned.Block) string {
	worldState := block.States
	config := permissioned.GetConfigFromWorldState(worldState)

	c.Lock()
	defer c.Unlock()

	// top up credits
	var maxCredit float64 = 0
	maxPeer := ""
	for peer := range config.Participants {
		account := permissioned.GetAccountFromWorldState(worldState, peer)
		c.records[peer] += account.GetBalance()

		if c.records[peer] >= maxCredit {
			maxCredit = c.records[peer]
			maxPeer = peer
		}
	}
	log.Info().Msgf("[Credit System] height=%d, %T",
		block.Height, c.records)

	// select next miner
	c.latestMiner = maxPeer
	// clear miner's credits
	c.records[maxPeer] = 0

	return maxPeer
}
