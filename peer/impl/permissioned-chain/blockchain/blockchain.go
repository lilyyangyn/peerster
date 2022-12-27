package blockchain

import "go.dedis.ch/cs438/storage"

var DUMMY_PREVHASH = make([]byte, 32)

type Blockchain struct {
}

func NewBlockchain() *Blockchain {
	bm := Blockchain{}
	return &bm
}

func (bc *Blockchain) InitGenesisBlock(config *ChainConfig) (Block, error) {
	// genesis block must be a config block
	return *bc.createConfigBlock(config), nil
}

func (bc *Blockchain) InitGenesisBlockFromYAML(path string) (Block, error) {
	// genesis block must be a config block
	config, err := ChainConfigFromYAML(path)
	if err != nil {
		return Block{}, err
	}
	return *bc.createConfigBlock(config), nil
}

func (bc *Blockchain) AppendBlock(block Block) error {
	// verify and append
	return nil
}

/** Feature Functions **/

/** Private Helpfer Functions **/

func (bc *Blockchain) createConfigBlock(config *ChainConfig) *Block {
	bb := NewBlockBuilder(ConfigType)
	bb.SetPrevHash(DUMMY_PREVHASH).SetHeight(0).SetState(storage.NewBasicKV()).AddConfig(config)
	return bb.Build()
}
