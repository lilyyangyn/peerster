package permissioned

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"go.dedis.ch/cs438/storage"
	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// config

var STATE_CONFIG_KEY = "PermissionedChain-Config"

// ChainConfig represents the config of the permissioned chain
type ChainConfig struct {
	Participants map[string]string

	MaxTxnsPerBlk int
	WaitTimeout   string

	MPCParticipationGain float64

	JoinThreshold float64
}

// NewChainConfig creates a new config and computes its ID
func NewChainConfig(participant map[string]string,
	maxTxnsPerBlk int, waitTimeout string, mpcGain float64, threshold float64) *ChainConfig {
	cc := ChainConfig{
		Participants: participant,

		MaxTxnsPerBlk: maxTxnsPerBlk,
		WaitTimeout:   waitTimeout,

		MPCParticipationGain: mpcGain,

		JoinThreshold: threshold,
	}

	return &cc
}

// ChainConfigFromYAML creates a new config based on yaml and computes its ID
func ChainConfigFromYAML(path string) (*ChainConfig, error) {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cc ChainConfig
	err = yaml.Unmarshal(yamlFile, &cc)
	if err != nil {
		return nil, err
	}

	return &cc, nil
}

// Hash implements Hashable.Hash
func (c ChainConfig) Hash() string {
	h := sha256.New()

	keys := make([]string, 0, len(c.Participants))
	for k := range c.Participants {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, participant := range keys {
		h.Write([]byte(participant))
		h.Write([]byte(c.Participants[participant]))
	}
	h.Write([]byte(fmt.Sprintf("%d", c.MaxTxnsPerBlk)))
	h.Write([]byte(c.WaitTimeout))

	h.Write([]byte(fmt.Sprintf("%f", c.MPCParticipationGain)))

	h.Write([]byte(fmt.Sprintf("%f", c.JoinThreshold)))

	return hex.EncodeToString(h.Sum(nil))
}

// String implements Describable.String()
func (c ChainConfig) String() string {
	participants := "["
	for peer := range c.Participants {
		participants += fmt.Sprintf("%s, ", peer)
	}
	participants = participants[:len(participants)-2] + "]"
	description := fmt.Sprintf(`Participants: %s, MaxNumTxn: %d, 
	MaxBlockWaitTime: %s, MPCParticipationGain %f, JoinThreshold: %f, `,
		participants, c.MaxTxnsPerBlk, c.WaitTimeout, c.MPCParticipationGain, c.JoinThreshold)
	return description

}

// Copy implements Copyable.Copy
func (c ChainConfig) Copy() storage.Copyable {
	participants := make(map[string]string)
	for key, val := range c.Participants {
		participants[key] = val
	}
	config := ChainConfig{
		Participants:         participants,
		MaxTxnsPerBlk:        c.MaxTxnsPerBlk,
		WaitTimeout:          c.WaitTimeout,
		MPCParticipationGain: c.MPCParticipationGain,
		JoinThreshold:        c.JoinThreshold,
	}
	return config
}

// -----------------------------------------------------------------------------
// Transaction Polymophism - InitConfig

func NewTransactionInitConfig(config *ChainConfig) *Transaction {
	return NewTransaction(
		NewAccount(ZeroAddress),
		&ZeroAddress,
		TxnTypeInitConfig,
		0,
		config.Copy().(ChainConfig),
	)
}

func execInitConfig(worldState storage.KVStore, config *ChainConfig, txn *Transaction) error {
	if _, ok := worldState.Get(STATE_CONFIG_KEY); ok {
		return fmt.Errorf("fail to init chain config, Config already exists")
	}

	newConfig := txn.Data.(ChainConfig)
	worldState.Put(STATE_CONFIG_KEY, newConfig)
	return nil
}

func unmarshalInitConfig(data json.RawMessage) (interface{}, error) {
	var c ChainConfig
	err := json.Unmarshal(data, &c)

	return c, err
}

// -----------------------------------------------------------------------------
// Transaction Polymophism - RegEnckey

func NewTransactionRegEnckey(peer *Account, pubkey string) *Transaction {
	return NewTransaction(
		peer,
		&ZeroAddress,
		TxnTypeSetPubkey,
		0,
		pubkey,
	)
}

func execRegEnckey(worldState storage.KVStore, config *ChainConfig, txn *Transaction) error {
	if len(config.Participants[txn.From]) > 0 {
		return fmt.Errorf("public key for account %s is already set", txn.From)
	}

	pubkey := txn.Data.(string)
	config.Participants[txn.From] = pubkey

	return nil
}

func unmarshalRegEnckey(data json.RawMessage) (interface{}, error) {
	var c ChainConfig
	err := json.Unmarshal(data, &c)

	return c, err
}
