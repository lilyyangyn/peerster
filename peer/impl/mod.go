package impl

import (
	"context"
	"crypto/ecdsa"
	"io"
	"math/rand"
	"regexp"
	"time"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/blockchain"
	"go.dedis.ch/cs438/peer/impl/datashare"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/peer/impl/mpc"
	"go.dedis.ch/cs438/peer/impl/paxos"
	permissioned "go.dedis.ch/cs438/permissioned-chain"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

// NewPeer creates a new peer. You can change the content and location of this
// function but you MUST NOT change its signature and package location.
func NewPeer(conf peer.Configuration) peer.Peer {
	// here you must return a struct that implements the peer.Peer functions.
	// Therefore, you are free to rename and change it as you want.
	n := node{
		conf:    conf,
		stopSig: nil,
	}

	n.message = message.NewMessageModule(&conf)
	n.paxos = paxos.NewPaxosModule(&conf, n.message)
	n.datasharing = datashare.NewDataSharingModule(&conf, n.message, n.paxos)
	n.blockchain = blockchain.NewBlockchainModule(&conf, n.message)
	n.mpc = mpc.NewMPCModule(&conf, n.message, n.paxos, n.blockchain)

	return &n
}

// node implements a peer to build a Peerster system
//
// - implements peer.Peer
type node struct {
	peer.Peer
	conf peer.Configuration

	message     *message.MessageModule
	paxos       *paxos.PaxosModule
	datasharing *datashare.DataSharingModule
	mpc         *mpc.MPCModule
	blockchain  *blockchain.BlockchainModule

	stopSig context.CancelFunc
}

/** Feature Functions **/

// Start implements peer.Service
func (n *node) Start() error {
	//start a new loop to listen to the message (non-blocking)
	rand.Seed(time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	n.stopSig = cancel

	err := n.message.MessagingDaemon(ctx)
	if err != nil {
		return err
	}

	err = n.message.HeartBeatDaemon(ctx, n.conf.HeartbeatInterval)
	if err != nil {
		return err
	}

	err = n.message.AntiEntropyDaemon(ctx, n.conf.AntiEntropyInterval)
	if err != nil {
		return err
	}

	err = n.blockchain.MiningDaemon(ctx)
	if err != nil {
		return err
	}

	// return once ready to use
	return nil
}

// Stop implements peer.Service
func (n *node) Stop() error {
	if n.stopSig != nil {
		n.stopSig()
	}

	return nil
}

// Unicast implements peer.Messaging
func (n *node) Unicast(dest string, msg transport.Message) error {
	return n.message.Unicast(dest, msg)
}

// Broadcast implements peer.Messaging
func (n *node) Broadcast(msg transport.Message) error {
	return n.message.Broadcast(msg)
}

// AddPeer implements peer.Service
func (n *node) AddPeer(addr ...string) {
	for _, peerAddr := range addr {
		// add self should have no effct
		if peerAddr == n.conf.Socket.GetAddress() {
			continue
		}
		// otherwise, update the routing table
		n.SetRoutingEntry(peerAddr, peerAddr)
	}
}

// GetRoutingTable implements peer.Service
func (n *node) GetRoutingTable() peer.RoutingTable {
	return n.message.GetRoutingTable()
}

// SetRoutingEntry implements peer.Service
func (n *node) SetRoutingEntry(origin, relayAddr string) {
	n.message.SetRoutingEntry(origin, relayAddr)
}

// Upload implements peer.Upload
func (n *node) Upload(data io.Reader) (metahash string, err error) {
	return n.datasharing.Upload(data)
}

// Download implements peer.Download
func (n *node) Download(metahash string) (data []byte, err error) {
	return n.datasharing.Download(metahash)
}

// Tag implements peer.Tag
func (n *node) Tag(name string, mh string) error {
	return n.datasharing.Tag(name, mh)
}

// Resolve implements peer.Resolve
func (n *node) Resolve(name string) string {
	return n.datasharing.Resolve(name)
}

// GetCatalog implements peer.GetCatalog
func (n *node) GetCatalog() peer.Catalog {
	return n.datasharing.GetCatalog()
}

// UpdateCatalog implements peer.UpdateCatalog
func (n *node) UpdateCatalog(key string, peer string) {
	n.datasharing.UpdateCatalog(key, peer)
}

// SearchAll implements peer.SearchAll
func (n *node) SearchAll(reg regexp.Regexp, budget uint, timeout time.Duration) (names []string, err error) {
	return n.datasharing.SearchAll(reg, budget, timeout)
}

// SearchFirst implements peer.SearchFirst
func (n *node) SearchFirst(pattern regexp.Regexp, conf peer.ExpandingRing) (name string, err error) {
	return n.datasharing.SearchFirst(pattern, conf)
}

// GetPubkeyString implements peer.GetPubkeyString
func (n *node) GetPubkeyString() (string, error) {
	return n.message.GetPubkeyString()
}

// SendEncryptedMessage implements peer.SendEncryptedMessage
func (n *node) SendEncryptedMessage(msg transport.Message, to string) error {
	return n.message.SendEncryptedMessage(msg, to)
}

// SetPubkeyEntry implements peer.SetPubkeyEntry
func (n *node) SetPubkeyEntry(origin string, pubkey *types.Pubkey) {
	n.message.SetPubkeyEntry(origin, pubkey)
}

// GetPubkeyStore implements peer.GetPubkeyStore
func (n *node) GetPubkeyStore() peer.PubkeyStore {
	return n.message.GetPubkeyStore()
}

// InitMPC implements peer.InitMPC
func (n *node) InitMPC(uniqID string, prime string, initiator string,
	expression string) error {
	return n.mpc.InitMPCWithPaxos(uniqID, prime, initiator, expression)
}

// GetPubkeyStore implements peer.ComputeExpression
func (n *node) ComputeExpression(uniqID string, expr string, prime string) (int, error) {
	return n.mpc.ComputeExpression(uniqID, expr, prime)
}

// GetPubkeyStore implements peer.SetValueDBAsset
func (n *node) SetValueDBAsset(key string, value int, price float64) error {
	return n.mpc.SetValueDBAsset(key, value, price)
}

// ShowAllPeerAssets implements peer.ShowAllPeerAssets
func (n *node) GetAllPeerAssetPrices() map[string]map[string]float64 {
	return n.mpc.GetPeerAssetPrices()
}

// Calculate implements peer.Calculate
func (n *node) Calculate(expression string, budget float64) (int, error) {
	return n.mpc.Calculate(expression, budget)
}

// InitBlockchain implements peer.InitBlockchain
func (n *node) InitBlockchain(config permissioned.ChainConfig, initialGain map[string]float64) error {
	return n.blockchain.InitBlockchain(config, initialGain)
}

// BCWaitBlock implements peer.BCWaitBlock
func (n *node) BCWaitBlock() *permissioned.Block {
	return n.blockchain.WaitBlock()
}

// BCSendTransaction implements peer.BCSendTransaction
func (n *node) BCSendTransaction(txn *permissioned.SignedTransaction) error {
	return n.blockchain.SendTransaction(txn)
}

// BCHasTransaction implements peer.BCHasTransaction
func (n *node) BCHasTransaction(txnID string) bool {
	return n.blockchain.GetTxn(txnID) != nil
}

// BCGetTransaction implements peer.BCGetTransaction
func (n *node) BCGetTransaction(txnID string) *permissioned.SignedTransaction {
	return n.blockchain.GetTxn(txnID)
}

// BCGetLatestBlock implements peer.BCGetLatestBlock
func (n *node) BCGetLatestBlock() *permissioned.Block {
	return n.blockchain.GetLatestBlock()
}

// BCGetBlock implements peer.BCGetBlock
func (n *node) BCGetBlock(blockID string) *permissioned.Block {
	return n.blockchain.GetBlock(blockID)
}

// BCGetAddress implements peer.BCGetAddress
func (n *node) BCGetAddress() (permissioned.Address, error) {
	return n.blockchain.GetChainAddress()
}

// BCGetBalance implements peer.BCGetBalance
func (n *node) BCGetBalance() float64 {
	return n.blockchain.GetAccountBalance()
}

// BCGenerateKeyPair implements peer.BCGenerateKeyPair
func (n *node) BCGenerateKeyPair(path string) error {
	return n.blockchain.GenerateKeyPair(path)
}

// BCSetKeyPair implements peer.BCSetKeyPair
func (n *node) BCSetKeyPair(privkey ecdsa.PrivateKey) error {
	return n.blockchain.SetKeyPair(privkey)
}

// BCLoadKeyPair implements peer.BCLoadKeyPair
func (n *node) BCLoadKeyPair(path string) error {
	return n.blockchain.LoadKeyPair(path)
}

// BCAllEncryptKeySet implements peer.BCAllEncryptKeySet
func (n *node) BCAllEncryptKeySet() bool {
	return n.blockchain.AllEncryptKeySet()
}

// BCSprintBlockchain implements peer.BCSprintBlockchain
func (n *node) BCSprintBlockchain() string {
	return n.blockchain.SprintBlockchain()
}
