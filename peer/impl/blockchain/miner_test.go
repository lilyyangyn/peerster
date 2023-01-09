package blockchain

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	permissioned "go.dedis.ch/cs438/permissioned-chain"
)

func Test_BC_Miner_Create_Blk_Success(t *testing.T) {
	// init key pair
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *permissioned.NewAccount(*permissioned.NewAddress(&pubkey))

	// init worldstate
	config := *permissioned.NewChainConfig(
		map[string]string{account.GetAddress().Hex: ""},
		1, "2h", 1, 10,
	)
	initialGain := map[string]float64{
		account.GetAddress().Hex: 1000,
	}
	bc := permissioned.NewBlockchain()
	block0, err := bc.InitGenesisBlock(&config, initialGain)
	require.NoError(t, err)
	err = bc.SetGenesisBlock(&block0)
	require.NoError(t, err)

	// init txnPool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := NewTxnPool()
	go pool.Daemon(ctx)
	txn1, err := permissioned.NewTransactionRegAssets(&account, map[string]float64{
		"key1": 1,
	}).Sign(privKey)
	require.NoError(t, err)
	pool.Push(txn1)

	blkDone := make(chan struct{})
	var block *permissioned.Block
	go func() {
		blk := createBlock(ctx, pool, account.GetAddress().Hex,
			&block0)
		block = blk

		close(blkDone)
	}()

	timeout := time.After(time.Millisecond * 200)

	select {
	case <-blkDone:
	case <-timeout:
		t.Error(t, "a block must be built")
	}

	require.NotNil(t, block)
	err = bc.AppendBlock(block)
	require.NoError(t, err)
}

func Test_BC_Miner_Create_Blk_Success_Resume(t *testing.T) {
	// init key pair
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *permissioned.NewAccount(*permissioned.NewAddress(&pubkey))

	// init worldstate
	config := *permissioned.NewChainConfig(
		map[string]string{account.GetAddress().Hex: ""},
		1, "2h", 1, 10,
	)
	initialGain := map[string]float64{
		account.GetAddress().Hex: 1000,
	}
	bc := permissioned.NewBlockchain()
	block0, err := bc.InitGenesisBlock(&config, initialGain)
	require.NoError(t, err)
	err = bc.SetGenesisBlock(&block0)
	require.NoError(t, err)

	// init txnPool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := NewTxnPool()
	go pool.Daemon(ctx)

	blkDone := make(chan struct{})
	var block *permissioned.Block
	go func() {
		blk := createBlock(ctx, pool, account.GetAddress().Hex,
			&block0)
		block = blk

		close(blkDone)
	}()

	timeout := time.After(time.Second * 3)

	select {
	case <-blkDone:
		t.Error(t, "a block must not be built")
	case <-timeout:
	}

	txn1, err := permissioned.NewTransactionRegAssets(&account, map[string]float64{
		"key1": 1,
	}).Sign(privKey)
	require.NoError(t, err)
	pool.Push(txn1)

	timeout = time.After(time.Millisecond * 200)

	select {
	case <-blkDone:
	case <-timeout:
		t.Error(t, "a block must be built")
	}

	require.NotNil(t, block)
	err = bc.AppendBlock(block)
	require.NoError(t, err)
}

func Test_BC_Miner_Create_Blk_Success_Timeout(t *testing.T) {
	// init key pair
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *permissioned.NewAccount(*permissioned.NewAddress(&pubkey))

	// init worldstate
	config := *permissioned.NewChainConfig(
		map[string]string{account.GetAddress().Hex: ""},
		2, "2s", 1, 10,
	)
	initialGain := map[string]float64{
		account.GetAddress().Hex: 1000,
	}
	bc := permissioned.NewBlockchain()
	block0, err := bc.InitGenesisBlock(&config, initialGain)
	require.NoError(t, err)
	err = bc.SetGenesisBlock(&block0)
	require.NoError(t, err)

	// init txnPool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := NewTxnPool()
	go pool.Daemon(ctx)
	txn1, err := permissioned.NewTransactionRegAssets(&account, map[string]float64{
		"key1": 1,
	}).Sign(privKey)
	require.NoError(t, err)
	pool.Push(txn1)

	blkDone := make(chan struct{})
	var block *permissioned.Block
	go func() {
		blk := createBlock(ctx, pool, account.GetAddress().Hex,
			&block0)
		block = blk

		close(blkDone)
	}()

	timeout := time.After(time.Second * 1)

	select {
	case <-blkDone:
		t.Error(t, "a block must not be built")
	case <-timeout:
	}

	timeout = time.After(time.Second * 2)

	select {
	case <-blkDone:
	case <-timeout:
		t.Error(t, "a block must be built")
		return
	}

	require.Equal(t, len(block.Transactions), 1)

	err = bc.AppendBlock(block)
	require.NoError(t, err)
}

func Test_BC_Miner_Create_Blk_Ctx_Stop(t *testing.T) {
	// init key pair
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *permissioned.NewAccount(*permissioned.NewAddress(&pubkey))

	// init worldstate
	config := *permissioned.NewChainConfig(
		map[string]string{account.GetAddress().Hex: ""},
		2, "2h", 1, 10,
	)
	initialGain := map[string]float64{
		account.GetAddress().Hex: 1000,
	}
	bc := permissioned.NewBlockchain()
	block0, err := bc.InitGenesisBlock(&config, initialGain)
	require.NoError(t, err)
	err = bc.SetGenesisBlock(&block0)
	require.NoError(t, err)

	// init txnPool
	ctx, cancel := context.WithCancel(context.Background())

	pool := NewTxnPool()
	go pool.Daemon(ctx)
	txn1, err := permissioned.NewTransactionRegAssets(&account, map[string]float64{
		"key1": 1,
	}).Sign(privKey)
	require.NoError(t, err)
	pool.Push(txn1)

	blkDone := make(chan struct{})
	var block *permissioned.Block
	go func() {
		blk := createBlock(ctx, pool, account.GetAddress().Hex,
			&block0)
		block = blk

		close(blkDone)
	}()

	timeout := time.After(time.Second * 2)

	select {
	case <-blkDone:
		t.Error(t, "a block must not be built")
	case <-timeout:
	}

	cancel()

	select {
	case <-blkDone:
	case <-timeout:
		t.Error(t, "a block must be built")
		return
	}

	require.Nil(t, block)
}
