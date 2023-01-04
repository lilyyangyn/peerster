package permissioned

import (
	"crypto/sha256"
	"encoding/hex"
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
	storage.Copyable

	ID           string
	Participants map[string]struct{}

	MaxTxnsPerBlk int
	WaitTimeout   string

	JoinThreshold float64
}

// NewChainConfig creates a new config and computes its ID
func NewChainConfig(participant map[string]struct{},
	maxTxnsPerBlk int, waitTimeout string, threshold float64) *ChainConfig {
	cc := ChainConfig{
		Participants: participant,

		MaxTxnsPerBlk: maxTxnsPerBlk,
		WaitTimeout:   waitTimeout,

		JoinThreshold: threshold,
	}
	cc.ID = cc.Hash()

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
	cc.ID = cc.Hash()

	return &cc, nil
}

// Hash computes the hash of the config
func (c *ChainConfig) Hash() string {
	h := sha256.New()

	keys := make([]string, 0, len(c.Participants))
	for k := range c.Participants {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, participant := range keys {
		h.Write([]byte(participant))
	}
	h.Write([]byte(fmt.Sprintf("%d", c.MaxTxnsPerBlk)))
	h.Write([]byte(fmt.Sprintf("%s", c.WaitTimeout)))
	h.Write([]byte(fmt.Sprintf("%f", c.JoinThreshold)))

	return hex.EncodeToString(h.Sum(nil))
}

// Copy implements Copyable.Copy
func (c *ChainConfig) Copy() storage.Copyable {
	participants := make(map[string]struct{})
	for key, val := range c.Participants {
		participants[key] = val
	}
	config := ChainConfig{
		ID:            c.ID,
		Participants:  participants,
		MaxTxnsPerBlk: c.MaxTxnsPerBlk,
		WaitTimeout:   c.WaitTimeout,
		JoinThreshold: c.JoinThreshold,
	}
	return &config
}
