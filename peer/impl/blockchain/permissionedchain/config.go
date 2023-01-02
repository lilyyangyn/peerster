package permissioned

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// config

var STATE_CONFIG_KEY = "PermissionedChain-Config"

// ChainConfig represents the config of the permissioned chain
type ChainConfig struct {
	ID           string
	Participants map[string]struct{}
	Threshold    int
	InitialGain  uint
}

// NewChainConfig creates a new config and computes its ID
func NewChainConfig(participant map[string]struct{}, threshold int, initialGain uint) *ChainConfig {
	cc := ChainConfig{
		Participants: participant,
		Threshold:    threshold,
		InitialGain:  initialGain,
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

	for participant, _ := range c.Participants {
		h.Write([]byte(participant))
	}
	h.Write([]byte(fmt.Sprintf("%d", c.Threshold)))
	h.Write([]byte(fmt.Sprintf("%d", c.InitialGain)))

	return hex.EncodeToString(h.Sum(nil))
}