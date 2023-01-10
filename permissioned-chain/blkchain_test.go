package permissioned

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func Test_BC_Init_Genesis(t *testing.T) {
	config := *NewChainConfig(
		map[string]string{
			"addr1": "",
			"addr2": "",
			"addr3": "",
		},
		10, "2h", 0, 10,
	)
	initialGain := map[string]float64{
		"addr2": 100,
		"addr3": 10,
		"addr4": 100000,
	}

	bc := NewBlockchain()
	block, err := bc.InitGenesisBlock(&config, initialGain)
	require.NoError(t, err)
	err = bc.SetGenesisBlock(&block)
	require.NoError(t, err)

	latestBlock := bc.GetLatestBlock()
	require.Equal(t, block.Hash(), latestBlock.Hash())

	worldState := block.States
	newConfig := GetConfigFromWorldState(worldState)
	require.Equal(t, config.Hash(), newConfig.Hash())
	account1 := GetAccountFromWorldState(worldState, "addr1")
	require.Equal(t, float64(0), account1.balance)
	account2 := GetAccountFromWorldState(worldState, "addr2")
	require.Equal(t, float64(100), account2.balance)
	account3 := GetAccountFromWorldState(worldState, "addr3")
	require.Equal(t, float64(10), account3.balance)
	account4 := GetAccountFromWorldState(worldState, "addr4")
	require.Equal(t, float64(0), account4.balance)
}

func Test_BC_Append_Correct(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))

	// create blockchain
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		10, "2h", 0, 10,
	)
	initialGain := map[string]float64{
		account.addr.Hex: 1000,
	}
	bc := NewBlockchain()
	block0, err := bc.InitGenesisBlock(&config, initialGain)
	require.NoError(t, err)
	err = bc.SetGenesisBlock(&block0)
	require.NoError(t, err)
	latestBlock := bc.GetLatestBlock()
	require.Equal(t, block0.Hash(), latestBlock.Hash())

	worldstate := block0.GetWorldStateCopy()

	txn1, err := NewTransactionRegAssets(&account, map[string]float64{
		"key1": 1,
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn1.Verify(worldstate)
	require.NoError(t, err)

	bb := NewBlockBuilder()
	bb.SetPrevHash(block0.Hash()).SetHeight(block0.Height + 1).
		SetMiner(account.addr.Hex).SetState(worldstate)
	bb.AddTxn(txn1)
	block1 := bb.Build()

	result := bc.CheckBlockHeight(block1)
	require.Equal(t, BlockCompareMatched, result)

	err = bc.AppendBlock(block1)
	require.NoError(t, err)

	latestBlock = bc.GetLatestBlock()
	require.Equal(t, block1.Hash(), latestBlock.Hash())
}

func Test_BC_Append_Check_Fail(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))

	// create blockchain
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		10, "2h", 0, 10,
	)
	initialGain := map[string]float64{
		account.addr.Hex: 1000,
	}
	bc := NewBlockchain()
	block0, err := bc.InitGenesisBlock(&config, initialGain)
	require.NoError(t, err)
	err = bc.SetGenesisBlock(&block0)
	require.NoError(t, err)
	latestBlock := bc.GetLatestBlock()
	require.Equal(t, block0.Hash(), latestBlock.Hash())

	worldstate := block0.GetWorldStateCopy()

	txn1, err := NewTransactionRegAssets(&account, map[string]float64{
		"key1": 1,
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn1.Verify(worldstate)
	require.NoError(t, err)

	// > block too stale

	bb := NewBlockBuilder()
	bb.SetPrevHash(block0.Hash()).SetHeight(block0.Height).
		SetMiner(account.addr.Hex).SetState(worldstate)
	bb.AddTxn(txn1)
	block1 := bb.Build()

	result := bc.CheckBlockHeight(block1)
	require.Equal(t, BlockCompareStale, result)

	// > block too advance

	bb = NewBlockBuilder()
	bb.SetPrevHash(block0.Hash()).SetHeight(block0.Height + 2).
		SetMiner(account.addr.Hex).SetState(worldstate)
	bb.AddTxn(txn1)
	block1 = bb.Build()

	result = bc.CheckBlockHeight(block1)
	require.Equal(t, BlockCompareAdvance, result)

	// > mismatch prevhash

	bb = NewBlockBuilder()
	bb.SetPrevHash(DUMMY_PREVHASH).SetHeight(block0.Height + 1).
		SetMiner(account.addr.Hex).SetState(worldstate)
	bb.AddTxn(txn1)
	block1 = bb.Build()

	result = bc.CheckBlockHeight(block1)
	require.Equal(t, BlockCompareInvalidHash, result)
}

func Test_BC_Append_Verify_Fail(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))

	// create blockchain
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		10, "2h", 0, 10,
	)
	initialGain := map[string]float64{
		account.addr.Hex: 1000,
	}
	bc := NewBlockchain()
	block0, err := bc.InitGenesisBlock(&config, initialGain)
	require.NoError(t, err)
	err = bc.SetGenesisBlock(&block0)
	require.NoError(t, err)
	latestBlock := bc.GetLatestBlock()
	require.Equal(t, block0.Hash(), latestBlock.Hash())

	worldstate := block0.GetWorldStateCopy()

	txn1, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     1001,
		Expression: "a",
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn1.Verify(worldstate)
	require.Error(t, err)

	bb := NewBlockBuilder()
	bb.SetPrevHash(block0.Hash()).SetHeight(block0.Height + 1).
		SetMiner(account.addr.Hex).SetState(worldstate)
	bb.AddTxn(txn1)
	block1 := bb.Build()

	result := bc.CheckBlockHeight(block1)
	require.Equal(t, BlockCompareMatched, result)

	err = bc.AppendBlock(block1)
	require.Error(t, err)

	latestBlock = bc.GetLatestBlock()
	require.Equal(t, block0.Hash(), latestBlock.Hash())
}

func Test_BC_Has_Txn(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))

	// create blockchain
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		10, "2h", 0, 10,
	)
	initialGain := map[string]float64{
		account.addr.Hex: 1000,
	}
	bc := NewBlockchain()
	block0, err := bc.InitGenesisBlock(&config, initialGain)
	require.NoError(t, err)
	err = bc.SetGenesisBlock(&block0)
	require.NoError(t, err)

	// first block has txn1 and txn2
	worldstate := block0.GetWorldStateCopy()
	txn1, err := NewTransactionRegAssets(&account, map[string]float64{
		"b": 1,
	}).Sign(privKey)
	require.NoError(t, err)
	account.nonce++
	err = txn1.Verify(worldstate)
	require.NoError(t, err)

	txn2, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     10,
		Expression: "b",
	}).Sign(privKey)
	require.NoError(t, err)
	account.nonce++
	err = txn2.Verify(worldstate)
	require.NoError(t, err)

	bb := NewBlockBuilder()
	bb.SetPrevHash(block0.Hash()).SetHeight(block0.Height + 1).
		SetMiner(account.addr.Hex).SetState(worldstate)
	bb.AddTxn(txn1)
	bb.AddTxn(txn2)
	block1 := bb.Build()

	err = bc.AppendBlock(block1)
	require.NoError(t, err)

	// second block has txn3
	worldstate = block1.GetWorldStateCopy()
	txn3, err := NewTransactionPostMPC(&account, MPCRecord{
		UniqID: txn2.Txn.ID,
		Result: "",
	}).Sign(privKey)
	require.NoError(t, err)
	account.nonce++
	err = txn3.Verify(worldstate)
	require.NoError(t, err)

	bb = NewBlockBuilder()
	bb.SetPrevHash(block1.Hash()).SetHeight(block1.Height + 1).
		SetMiner(account.addr.Hex).SetState(worldstate)
	bb.AddTxn(txn3)
	block2 := bb.Build()

	err = bc.AppendBlock(block2)
	require.NoError(t, err)

	// > txn1, txn2, txn3 should be in blockchain

	ok := bc.GetTxn(txn1.Txn.ID)
	require.NotNil(t, ok)
	ok = bc.GetTxn(txn2.Txn.ID)
	require.NotNil(t, ok)
	ok = bc.GetTxn(txn3.Txn.ID)
	require.NotNil(t, ok)

	// > txn4 should not be in blockchain

	txn4 := NewTransactionPostMPC(&account, MPCRecord{
		UniqID: txn2.Txn.ID,
		Result: "",
	})
	ok = bc.GetTxn(txn4.ID)
	require.Nil(t, ok)
}

func Test_BC_Append_Genesis(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))

	// create blockchain
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		10, "2h", 0, 10,
	)
	initialGain := map[string]float64{
		account.addr.Hex: 1000,
	}
	bc := NewBlockchain()
	block0, err := bc.InitGenesisBlock(&config, initialGain)
	require.NoError(t, err)
	err = bc.SetGenesisBlock(&block0)
	require.NoError(t, err)

	// cannot init twice
	err = bc.SetGenesisBlock(&block0)
	require.Error(t, err)

	// append the genesis block to a new blockchain
	newBC := NewBlockchain()
	err = newBC.SetGenesisBlock(&block0)
	require.NoError(t, err)

	require.Equal(t, bc.GetLatestBlock().Hash(),
		newBC.GetLatestBlock().Hash())
}
