package permissioned

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
)

// -----------------------------------------------------------------------------
// Address

var ZeroAddress = Address{}

type Address struct {
	Hex string
}

func NewAddress(pubkey *ecdsa.PublicKey) *Address {
	addressBytes := crypto.PubkeyToAddress(*pubkey).Hex()

	return &Address{Hex: addressBytes}
}

func NewAddressFromHex(pubkeyhex string) *Address {
	return &Address{Hex: pubkeyhex}
}

// -----------------------------------------------------------------------------
// Account

type Account struct {
	// *sync.RWMutex
	addr          Address
	balance       float64
	lockedBalance float64
	nonce         uint
}

func NewAccount(addr Address) *Account {
	return &Account{
		// RWMutex: &sync.RWMutex{},
		addr:    addr,
		balance: 0,
		nonce:   0,
	}
}

func (ac *Account) GetAddress() Address {
	return ac.addr
}

func (ac *Account) GetBalance() float64 {
	return ac.balance
}

// func (ac *Account) GetNonce() uint {
// 	ac.RLock()
// 	defer ac.RUnlock()

// 	return ac.nonce
// }
