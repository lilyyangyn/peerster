package permissioned

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"go.dedis.ch/cs438/storage"
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
		addr:          addr,
		balance:       0,
		lockedBalance: 0,
		nonce:         0,
	}
}

// Copy implements Copyable.Copy()
func (ac Account) Copy() storage.Copyable {
	account := Account{
		addr:          *NewAddressFromHex(ac.addr.Hex),
		balance:       ac.balance,
		lockedBalance: ac.lockedBalance,
		nonce:         ac.nonce,
	}
	return account
}

// String implements Describable.String()
func (ac Account) String() string {
	return fmt.Sprintf("Addr: %s, Balance: %f, Locked: %f, Nonce: %d\n",
		ac.addr.Hex, ac.balance, ac.lockedBalance, ac.nonce)
}

func (ac *Account) GetAddress() Address {
	return ac.addr
}

func (ac *Account) GetBalance() float64 {
	return ac.balance
}

func (ac *Account) IncreaseNonce() {
	ac.nonce++
}

func (ac *Account) GetNonce() float64 {
	return ac.balance
}
