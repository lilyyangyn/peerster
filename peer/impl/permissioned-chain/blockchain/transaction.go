package blockchain

import (
	"crypto"
	"encoding/hex"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var StateConfigKey = "PermissionedChain-Config"

// ChainConfig represents the config of the permissioned chain
type ChainConfig struct {
	ID           string
	Participants []Address
	Threshold    int
	InitialGain  uint
}

// NewChainConfig creates a new config and computes its ID
func NewChainConfig(participant []Address, threshold int, initialGain uint) *ChainConfig {
	cc := ChainConfig{
		Participants: participant,
		Threshold:    threshold,
		InitialGain:  initialGain,
	}
	cc.ID = hex.EncodeToString(cc.Hash())

	return &cc
}

// ChainConfigFromYAML creates a new config based on yaml and computes its ID
func ChainConfigFromYAML(path string) (*ChainConfig, error) {
	yamlFile, err := os.ReadFile("conf.yaml")
	if err != nil {
		return nil, err
	}

	cc := ChainConfig{}
	err = yaml.Unmarshal(yamlFile, &cc)
	if err != nil {
		return nil, err
	}
	cc.ID = hex.EncodeToString(cc.Hash())

	return &cc, nil
}

// Hash computes the hash of the config
func (c *ChainConfig) Hash() []byte {
	h := crypto.SHA256.New()

	for _, participant := range c.Participants {
		h.Write(participant.Addr[:])
	}
	h.Write([]byte(fmt.Sprintf("%d", c.Threshold)))
	h.Write([]byte(fmt.Sprintf("%d", c.InitialGain)))

	return h.Sum(nil)
}

// Transaction represents the transaction that happens inside this chain
type Transaction struct {
	ID        string
	Nonce     uint
	PreMPC    bool
	Initiator Address
	Budget    float64
	Result    float64
}

// NewTransaction creates a new transaction and computes its ID
func NewTransaction(initiator Account, preMPC bool, budget float64, result float64) *Transaction {
	txn := Transaction{
		Nonce:     initiator.nonce,
		PreMPC:    preMPC,
		Initiator: *initiator.addr,
		Budget:    budget,
		Result:    result,
	}
	txn.ID = hex.EncodeToString(txn.Hash())

	return &txn
}

// Hash computes the hash of the transaction
func (txn *Transaction) Hash() []byte {
	h := crypto.SHA256.New()

	h.Write([]byte(fmt.Sprintf("%d", txn.Nonce)))
	h.Write([]byte(fmt.Sprintf("%t", txn.PreMPC)))
	h.Write(txn.Initiator.Addr[:])
	h.Write([]byte(fmt.Sprintf("%d", txn.Budget)))
	h.Write([]byte(fmt.Sprintf("%d", txn.Result)))

	return h.Sum(nil)
}
