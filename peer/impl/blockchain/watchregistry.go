package blockchain

import (
	"sync"

	"go.dedis.ch/cs438/permissioned-chain"
)

type watchCallbck func(config *permissioned.ChainConfig,
	txn *permissioned.Transaction) error

type WatchRegistry struct {
	*sync.RWMutex
	store map[permissioned.TxnType]watchCallbck
}

func NewWatchRegistry() *WatchRegistry {
	r := WatchRegistry{
		RWMutex: &sync.RWMutex{},
		store:   map[permissioned.TxnType]watchCallbck{},
	}
	return &r
}

func (r *WatchRegistry) Register(txnType permissioned.TxnType,
	watcher watchCallbck) {
	r.Lock()
	defer r.Unlock()

	r.store[txnType] = watcher
}

func (r *WatchRegistry) Tell(config *permissioned.ChainConfig, txn *permissioned.Transaction) error {
	r.RLock()
	defer r.RUnlock()

	watcher, ok := r.store[txn.Type]
	if !ok {
		return nil
	}

	return watcher(config, txn)
}
