package blockchain

import (
	"sync"

	"go.dedis.ch/cs438/permissioned-chain"
)

type WatchRegistry struct {
	*sync.RWMutex
	store map[permissioned.TxnType]func(txn *permissioned.Transaction) error
}

func NewWatchRegistry() *WatchRegistry {
	r := WatchRegistry{
		RWMutex: &sync.RWMutex{},
		store:   map[permissioned.TxnType]func(txn *permissioned.Transaction) error{},
	}
	return &r
}

func (r *WatchRegistry) Register(txnType permissioned.TxnType,
	watcher func(txn *permissioned.Transaction) error) {
	r.Lock()
	defer r.Unlock()

	r.store[txnType] = watcher
}

func (r *WatchRegistry) Tell(txn *permissioned.Transaction) error {
	r.RLock()
	defer r.RUnlock()

	watcher, ok := r.store[txn.Type]
	if !ok {
		return nil
	}

	return watcher(txn)
}
