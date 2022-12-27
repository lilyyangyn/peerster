package blockchain

import (
	"sync"

	"go.dedis.ch/cs438/types"
)

type Address struct {
	Addr [8]byte
	Hex  string
}

func NewAddress(pubkey types.Pubkey) *Address {
	// TODO
	return &Address{}
}

type Account struct {
	*sync.RWMutex

	addr          *Address
	balance       float64
	lockedBalance float64
	nonce         uint
}

func NewAccount(pubkey types.Pubkey) *Account {
	return &Account{
		RWMutex: &sync.RWMutex{},
		addr:    NewAddress(pubkey),
		balance: 0,
		nonce:   0,
	}
}

func (ac *Account) GetAddress() Address {
	ac.RLock()
	defer ac.RUnlock()

	return *ac.addr
}

func (ac *Account) GetAvailableBalance() float64 {
	ac.RLock()
	defer ac.RUnlock()

	return ac.balance - ac.lockedBalance
}

func (ac *Account) GetNonce() uint {
	ac.RLock()
	defer ac.RUnlock()

	return ac.nonce
}
