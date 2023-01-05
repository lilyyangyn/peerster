package blockchain

import (
	"context"
	"sync"

	permissioned "go.dedis.ch/cs438/permissioned-chain"
)

// -----------------------------------------------------------------------------
// TxnPool

const POOL_CHAN_BUFFER_SIZE = 5

type TxnPool struct {
	*sync.Mutex
	ctx     context.Context
	channel chan *permissioned.SignedTransaction
	queue   []*permissioned.SignedTransaction
}

func NewTxnPool() *TxnPool {
	lock := sync.Mutex{}
	return &TxnPool{
		Mutex: &lock,
		ctx:   context.Background(),
		channel: make(chan *permissioned.SignedTransaction,
			POOL_CHAN_BUFFER_SIZE),
		queue: make([]*permissioned.SignedTransaction, 0),
	}
}

func (p *TxnPool) SetCtx(ctx context.Context) {
	p.Lock()
	defer p.Unlock()

	p.ctx = ctx
}

func (p *TxnPool) Push(txn *permissioned.SignedTransaction) {
	p.Lock()
	defer p.Unlock()

	if len(p.channel) < POOL_CHAN_BUFFER_SIZE {
		p.channel <- txn
		return
	}
	p.queue = append(p.queue, txn)
}

func (p *TxnPool) PushSeveral(txns []permissioned.SignedTransaction) {
	p.Lock()
	defer p.Unlock()

	for _, txn := range txns {
		p.queue = append(p.queue, &txn)
	}
}

func (p *TxnPool) Pull() <-chan *permissioned.SignedTransaction {
	p.Lock()
	defer p.Unlock()

	if len(p.channel) > 0 {
		return p.channel
	}

	i := 0
	queueLen := len(p.queue)
	for ; i < POOL_CHAN_BUFFER_SIZE && i < queueLen; i++ {
		p.channel <- p.queue[i]
	}
	p.queue = p.queue[i:]

	return p.channel
}

// -----------------------------------------------------------------------------
// SyncCenter

type SyncCenter struct {
	*sync.Mutex
	store map[string]chan error
}

func NewSyncCenter() *SyncCenter {
	return &SyncCenter{
		Mutex: &sync.Mutex{},
		store: map[string]chan error{},
	}
}

func (c *SyncCenter) Register(id string, channel chan error) {
	c.Lock()
	defer c.Unlock()

	c.store[id] = channel
}

func (c *SyncCenter) Notify(id string, err error) {
	c.Lock()
	defer c.Unlock()

	channel, ok := c.store[id]
	if !ok {
		return
	}

	channel <- err
	delete(c.store, id)
}
