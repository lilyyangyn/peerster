package blockchain

import (
	"context"
	"sync"

	permissioned "go.dedis.ch/cs438/permissioned-chain"
)

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
