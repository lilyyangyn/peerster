package peer

import (
	permissioned "go.dedis.ch/cs438/permissionedchain"
)

type PermissionedChain interface {
	// InitBlockchain inits a new blockchain
	// by creating and distributing a genesis block
	// with the given config
	InitBlockchain(config permissioned.ChainConfig, initialGain map[string]float64) error

	// SendTransaction signs the given transaction
	// and broadcast it to the network in private message
	SendTransaction(txn *permissioned.Transaction) error

	// HasTransaction checks if the transaction is
	// recorded in the blockchain
	HasTransaction(txnID string) bool

	// GenerateKeyPair generates an ECDSA key pair
	// and write it in the file
	GenerateKeyPair(path string) error

	// LoadKeyPair loads an ECDSA key pair from file
	LoadKeyPair(path string) error
}
