package permissioned

import (
	"crypto/ecdsa"
	"sync"
)

// -----------------------------------------------------------------------------
// Address

var ZeroAddress = Address{}

type Address struct {
	Hex string
}

func NewAddress(pubkey *ecdsa.PublicKey) *Address {
	// TODO
	// h := sha256.New()

	return &Address{}
}

// -----------------------------------------------------------------------------
// Account

type Account struct {
	*sync.RWMutex

	addr          Address
	balance       float64
	lockedBalance float64
	nonce         uint
}

func NewAccount(addr Address) *Account {
	return &Account{
		RWMutex: &sync.RWMutex{},
		addr:    addr,
		balance: 0,
		nonce:   0,
	}
}

func (ac *Account) GetAddress() Address {
	ac.RLock()
	defer ac.RUnlock()

	return ac.addr
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
