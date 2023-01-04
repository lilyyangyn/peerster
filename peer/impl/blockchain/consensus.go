package blockchain

import (
	"sync"

	"go.dedis.ch/cs438/permissioned-chain"
	"go.dedis.ch/cs438/storage"
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

func (c *CreditRecords) advanceAndSelect(worldState storage.KVStore) string {
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
			maxPeer = peer
		}
	}

	// select next miner
	c.latestMiner = maxPeer
	// clear miner's credits
	c.records[maxPeer] = 0

	return maxPeer
}
