package peer

import (
	"crypto/ecdsa"

	permissioned "go.dedis.ch/cs438/permissioned-chain"
)

type PermissionedChain interface {
	// InitBlockchain inits a new blockchain
	// by creating and distributing a genesis block
	// with the given config
	InitBlockchain(config permissioned.ChainConfig, initialGain map[string]float64) error

	// BCSendTransaction signs the given transaction
	// and broadcast it to the network in private message
	BCSendTransaction(txn *permissioned.SignedTransaction) error

	// BCHasTransaction checks if the transaction is
	// recorded in the blockchain
	BCHasTransaction(txnID string) bool

	// BCGetTransaction returns the requested transaction from
	// the blockchain. It returns nil if txn not exists
	BCGetTransaction(txnID string) *permissioned.SignedTransaction

	// BCWaitBlock blocks the thread until the genesis block is found
	BCWaitBlock() *permissioned.Block

	// BCGetLastBlock returns the latest block of the blockchain
	// it returns nil if blockchain not initialize
	BCGetLatestBlock() *permissioned.Block

	// BCGetBlock returns the requested block of the blockchain
	// it returns nil if blockchain not initialize or block not exists
	BCGetBlock(blockID string) *permissioned.Block

	// BCGetAddress helps users to know the adress of the node
	// it returns an error if the node does not join the chain
	// or not yep have an address
	BCGetAddress() (permissioned.Address, error)

	// BCGetBalance returns the current balance of the node's account
	BCGetBalance() float64

	// BCGenerateKeyPair generates an ECDSA key pair
	// and write it in the file
	BCGenerateKeyPair(path string) error

	// BCSetKeyPair sets an ECDSA key pair to the node
	BCSetKeyPair(privkey ecdsa.PrivateKey) error

	// BCLoadKeyPair loads an ECDSA key pair from file
	BCLoadKeyPair(path string) error

	// BCAllEncryptKeySet checks if all encryption keys of nodes are registered
	BCAllEncryptKeySet() bool

	// BCSprintBlockchain returns a decription string of the blockchain
	BCSprintBlockchain() string
}
