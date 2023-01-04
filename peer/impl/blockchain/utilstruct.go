package blockchain

import (
	"context"
	"sync"

	permissioned "go.dedis.ch/cs438/permissioned-chain"
)

type TxnPool struct {
	*sync.Mutex
	*sync.Cond
	ctx   context.Context
	queue []*permissioned.SignedTransaction
}

func NewTxnPool(ctx context.Context) *TxnPool {
	lock := sync.Mutex{}
	return &TxnPool{
		Mutex: &lock,
		Cond:  sync.NewCond(&lock),
		ctx:   ctx,
		queue: make([]*permissioned.SignedTransaction, 0),
	}
}

func (p *TxnPool) Push(txn *permissioned.SignedTransaction) {
	p.Lock()
	defer p.Unlock()

	p.queue = append(p.queue, txn)
	p.Signal()
}

func (p *TxnPool) PushSeveral(txns []permissioned.SignedTransaction) {
	p.Lock()
	defer p.Unlock()

	for _, txn := range txns {
		p.queue = append(p.queue, &txn)
	}
	p.Broadcast()
}

func (p *TxnPool) Pull() *permissioned.SignedTransaction {
	p.Lock()
	defer p.Unlock()

out:
	for {
		select {
		case <-p.ctx.Done():
			return nil
		default:
			if len(p.queue) != 0 {
				break out
			}
			// block until there is a value
			p.Wait()
		}
	}

	txn := p.queue[0]
	p.queue = p.queue[1:]
	return txn
}

func (p *TxnPool) Finish() {
	p.Broadcast()
}
