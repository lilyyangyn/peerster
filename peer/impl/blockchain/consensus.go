package blockchain

import (
	"sort"
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
	for peer := range config.Participants {
		account := permissioned.GetAccountFromWorldState(worldState, peer)
		c.records[peer] += account.GetBalance()

		if c.records[peer] >= maxCredit {
			maxCredit = c.records[peer]
		}
	}

	// sort to avoid multiple nodes have the same balance
	// FIXME: bias. Add a counter to do roubin round
	peerList := make([]string, 0)
	for peer := range c.records {
		if c.records[peer] == maxCredit {
			peerList = append(peerList, peer)
		}
	}
	sort.Strings(peerList)
	log.Info().Msgf("[Credit System] height=%d, %T",
		block.Height, c.records)

	// select next miner
	c.latestMiner = peerList[0]
	// clear miner's credits
	c.records[peerList[0]] = 0

	return peerList[0]
}
