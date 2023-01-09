package permissioned

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cs438/storage"
)

func Test_Block_Build(t *testing.T) {
	transactionNum := 10
	transactions := make([]SignedTransaction, transactionNum)
	for i := 0; i < transactionNum; i++ {
		signedTxn := SignedTransaction{
			Txn: Transaction{
				ID: fmt.Sprintf("%d", i),
			},
		}
		transactions[i] = signedTxn
	}

	prevHash := "fffffff"
	var height uint = 1
	miner := "miner1"
	expectedBlock := Block{
		BlockHeader: &BlockHeader{
			PrevHash: prevHash,
			Height:   height,
			Miner:    miner,
		},
		States:       storage.NewBasicKV(),
		Transactions: transactions,
	}
	expectedBlock.StateHash = hex.EncodeToString(expectedBlock.States.Hash())
	h := sha256.New()
	for _, txn := range transactions {
		h.Write(txn.HashBytes())
	}
	expectedBlock.TransationHash = hex.EncodeToString(h.Sum(nil))

	bb := NewBlockBuilder()
	bb.SetPrevHash(prevHash).SetHeight(height).SetMiner(miner).SetState(storage.NewBasicKV())
	for _, txn := range transactions {
		err := bb.AddTxn(&txn)
		require.NoError(t, err)
	}
	block := bb.Build()

	require.Equal(t, expectedBlock.PrevHash, block.PrevHash)
	require.Equal(t, expectedBlock.Height, block.Height)
	require.Equal(t, expectedBlock.Miner, block.Miner)
	require.Equal(t, expectedBlock.StateHash, block.StateHash)
	require.Equal(t, expectedBlock.TransationHash, block.TransationHash)
	require.Equal(t, expectedBlock.Hash(), block.Hash())
}

func Test_Block_Verify_Correct(t *testing.T) {
	// init account
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))
	account.balance = 15

	// create worldstate
	worldState := storage.NewBasicKV()
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		10, "2h", 0, 10,
	)
	worldState.Put(STATE_CONFIG_KEY, config)
	worldState.Put(account.addr.Hex, account)
	asset := NewAssetsRecord(account.addr.Hex)
	var budgetA float64 = 10
	asset.Add(map[string]float64{"a": budgetA})
	var budgetB float64 = 5
	asset.Add(map[string]float64{"b": budgetB})
	worldState.Put(AssetsKeyFromUniqID(account.addr.Hex), *asset)
	stateCopy := worldState.Copy()

	txn1, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     budgetA,
		Expression: "a",
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn1.Verify(worldState)
	require.NoError(t, err)

	account.nonce++
	txn2, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     budgetB,
		Expression: "b",
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn2.Verify(worldState)
	require.NoError(t, err)

	bb := NewBlockBuilder()
	bb.SetPrevHash("").SetHeight(0).SetMiner(account.addr.Hex).
		SetState(worldState)

	err = bb.AddTxn(txn1)
	require.NoError(t, err)
	err = bb.AddTxn(txn2)
	require.NoError(t, err)

	block := bb.Build()

	err = block.Verify(stateCopy)
	require.NoError(t, err)
}

func Test_Block_Verify_Invalid_Miner(t *testing.T) {
	// init account
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))
	account.balance = 15

	// create worldstate
	worldState := storage.NewBasicKV()
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		10, "2h", 0, 10,
	)
	worldState.Put(STATE_CONFIG_KEY, config)
	worldState.Put(account.addr.Hex, account)
	asset := NewAssetsRecord(account.addr.Hex)
	var budgetA float64 = 10
	asset.Add(map[string]float64{"a": budgetA})
	var budgetB float64 = 5
	asset.Add(map[string]float64{"b": budgetB})
	worldState.Put(AssetsKeyFromUniqID(account.addr.Hex), *asset)
	stateCopy := worldState.Copy()

	txn1, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     budgetA,
		Expression: "a",
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn1.Verify(worldState)
	require.NoError(t, err)

	account.nonce++
	txn2, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     budgetB,
		Expression: "b",
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn2.Verify(worldState)
	require.NoError(t, err)

	bb := NewBlockBuilder()
	bb.SetPrevHash("").SetHeight(0).
		SetMiner(NewAddressFromHex("fake").Hex).
		SetState(worldState)

	err = bb.AddTxn(txn1)
	require.NoError(t, err)
	err = bb.AddTxn(txn2)
	require.NoError(t, err)

	block := bb.Build()

	err = block.Verify(stateCopy)
	require.Error(t, err)
}

func Test_Block_Verify_Invalid_TXN_Hash(t *testing.T) {
	// init account
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))
	account.balance = 15

	// create worldstate
	worldState := storage.NewBasicKV()
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		10, "2h", 0, 10,
	)
	worldState.Put(STATE_CONFIG_KEY, config)
	worldState.Put(account.addr.Hex, account)
	asset := NewAssetsRecord(account.addr.Hex)
	var budgetA float64 = 10
	asset.Add(map[string]float64{"a": budgetA})
	var budgetB float64 = 5
	asset.Add(map[string]float64{"b": budgetB})
	worldState.Put(AssetsKeyFromUniqID(account.addr.Hex), *asset)
	stateCopy := worldState.Copy()

	txn1, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     budgetA,
		Expression: "a",
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn1.Verify(worldState)
	require.NoError(t, err)

	account.nonce++
	txn2, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     budgetB,
		Expression: "b",
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn2.Verify(worldState)
	require.NoError(t, err)

	bb := NewBlockBuilder()
	bb.SetPrevHash("").SetHeight(0).
		SetMiner(account.addr.Hex).
		SetState(worldState)

	err = bb.AddTxn(txn1)
	require.NoError(t, err)
	err = bb.AddTxn(txn2)
	require.NoError(t, err)

	block := bb.Build()
	block.TransationHash = "1234566789"

	err = block.Verify(stateCopy)
	require.Error(t, err)
}

func Test_Block_Verify_Invalid_TXN(t *testing.T) {
	// init account
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))
	account.balance = 15

	// create worldstate
	worldState := storage.NewBasicKV()
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		10, "2h", 0, 10,
	)
	worldState.Put(STATE_CONFIG_KEY, config)
	worldState.Put(account.addr.Hex, account)
	asset := NewAssetsRecord(account.addr.Hex)
	var budgetA float64 = 10
	asset.Add(map[string]float64{"a": budgetA})
	var budgetB float64 = 10
	asset.Add(map[string]float64{"b": budgetB})
	worldState.Put(AssetsKeyFromUniqID(account.addr.Hex), *asset)
	stateCopy := worldState.Copy()

	txn1, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     budgetA,
		Expression: "a",
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn1.Verify(worldState)
	require.NoError(t, err)

	account.nonce++
	txn2, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     budgetB,
		Expression: "b",
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn2.Verify(worldState)
	require.Error(t, err)

	bb := NewBlockBuilder()
	bb.SetPrevHash("").SetHeight(0).
		SetMiner(account.addr.Hex).
		SetState(worldState)

	err = bb.AddTxn(txn1)
	require.NoError(t, err)
	err = bb.AddTxn(txn2)
	require.NoError(t, err)

	block := bb.Build()

	err = block.Verify(stateCopy)
	require.Error(t, err)
}

func Test_Block_Verify_Inconsistent_State(t *testing.T) {
	// init account
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))
	account.balance = 15

	// create worldstate
	worldState := storage.NewBasicKV()
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		10, "2h", 0, 10,
	)
	worldState.Put(STATE_CONFIG_KEY, config)
	worldState.Put(account.addr.Hex, account)
	asset := NewAssetsRecord(account.addr.Hex)
	var budgetA float64 = 10
	asset.Add(map[string]float64{"a": budgetA})
	var budgetB float64 = 5
	asset.Add(map[string]float64{"b": budgetB})
	worldState.Put(AssetsKeyFromUniqID(account.addr.Hex), *asset)
	stateCopy := worldState.Copy()

	txn1, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     10,
		Expression: "a",
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn1.Verify(worldState)
	require.NoError(t, err)

	account.nonce++
	txn2, err := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     5,
		Expression: "b",
	}).Sign(privKey)
	require.NoError(t, err)
	err = txn2.Verify(worldState)
	require.NoError(t, err)

	bb := NewBlockBuilder()
	bb.SetPrevHash("").SetHeight(0).
		SetMiner(account.addr.Hex).
		SetState(worldState)

	err = bb.AddTxn(txn1)
	require.NoError(t, err)

	block := bb.Build()

	err = block.Verify(stateCopy)
	require.Error(t, err)
}
