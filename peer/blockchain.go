package peer

import permissioned "go.dedis.ch/cs438/peer/impl/blockchain/permissionedchain"

type PermissionedChain interface {
	InitBlockchain(permissioned.ChainConfig) error
	GenerateKeyPair(path string)
	LoadKeyPair(path string)
}
