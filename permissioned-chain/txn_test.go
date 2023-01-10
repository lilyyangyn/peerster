package permissioned

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cs438/storage"
)

func Test_Txn_Sign(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := NewAccount(*NewAddress(&pubkey))

	// create signature
	txn := NewTransactionPreMPC(account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     10,
		Expression: "a",
	})
	require.Equal(t, account.addr.Hex, txn.From)
	signedTxn, err := txn.Sign(privKey)
	require.NoError(t, err)

	// verify signature
	digestHash := txn.HashBytes()
	publickey, err := crypto.SigToPub(digestHash, signedTxn.Signature)
	require.NoError(t, err)
	require.Equal(t, pubkey, *publickey)

	valid := crypto.VerifySignature(crypto.FromECDSAPub(&pubkey), digestHash,
		signedTxn.Signature[:len(signedTxn.Signature)-1])
	require.True(t, valid)
}

func Test_Txn_Execution_PreMPC_Correct(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))
	account.balance = 20
	require.Equal(t, float64(0), account.lockedBalance)

	// create signedTxn
	var budget float64 = 10
	txn := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     budget,
		Expression: "a",
	})

	// create worldstate
	worldState := storage.NewBasicKV()
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		1, "2h", 0, 10,
	)
	worldState.Put(STATE_CONFIG_KEY, config)
	worldState.Put(account.addr.Hex, account)
	asset := NewAssetsRecord(account.addr.Hex)
	asset.Add(map[string]float64{"a": budget})
	worldState.Put(AssetsKeyFromUniqID(account.addr.Hex), *asset)
	stateCopy := worldState.Copy()

	// Execute transaction
	err = txn.Exec(worldState)
	require.NoError(t, err)

	// verify worldState
	require.NotEqual(t, stateCopy.Hash(), worldState.Hash())
	newAccount := GetAccountFromWorldState(worldState, txn.From)
	require.Equal(t, account.addr.Hex, newAccount.addr.Hex)
	require.Equal(t, account.balance, newAccount.balance+budget)
	require.Equal(t, newAccount.lockedBalance, budget)
	require.Equal(t, account.nonce+1, newAccount.nonce)
	mpcendorse, err := GetMPCEndorsementFromWorldState(worldState, mpcKeyFromUniqID(txn.ID))
	require.NoError(t, err)
	require.Equal(t, 1, len(mpcendorse.Peers))
	require.Equal(t, 0, len(mpcendorse.Endorsers))
	require.Equal(t, budget, mpcendorse.Budget[account.addr.Hex])
	require.True(t, mpcendorse.Locked)
}

func Test_Txn_Execution_PreMPC_InCorrect(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	pubkey := privKey.PublicKey
	account := *NewAccount(*NewAddress(&pubkey))
	account.balance = 200
	require.Equal(t, float64(0), account.lockedBalance)

	// create signedTxn
	var budget float64 = 10
	txn := NewTransactionPreMPC(&account, MPCPropose{
		Initiator:  account.addr.Hex,
		Budget:     budget,
		Expression: "a",
	})

	// create worldstate
	worldState := storage.NewBasicKV()
	config := *NewChainConfig(
		map[string]string{account.addr.Hex: ""},
		1, "2h", 0, 10,
	)
	worldState.Put(STATE_CONFIG_KEY, config)
	asset := NewAssetsRecord(account.addr.Hex)
	asset.Add(map[string]float64{"a": budget})
	worldState.Put(AssetsKeyFromUniqID(account.addr.Hex), *asset)

	// > balance not enough should fail

	testAccount := Account{
		addr:          account.addr,
		balance:       0,
		lockedBalance: account.lockedBalance,
		nonce:         account.nonce,
	}
	worldState.Put(account.addr.Hex, testAccount)
	stateCopy := worldState.Copy()
	err = txn.Exec(worldState)
	require.Error(t, err)
	// worldState should not change
	require.Equal(t, stateCopy.Hash(), worldState.Hash())

	// > double execuation should fail

	testAccount = Account{
		addr:          account.addr,
		balance:       account.balance,
		lockedBalance: account.lockedBalance,
		nonce:         account.nonce + 1,
	}
	worldState.Put(account.addr.Hex, account)
	//  execution should success
	err = txn.Exec(worldState)
	require.NoError(t, err)
	stateCopy = worldState.Copy()
	// second execution should fail
	err = txn.Exec(worldState)
	require.Error(t, err)

	// worldState should not change for second execution
	require.Equal(t, stateCopy.Hash(), worldState.Hash())
}

func Test_Txn_Execution_PostMPC_Correct(t *testing.T) {
	account := *NewAccount(*NewAddressFromHex("account1"))

	// create initiator
	initiator := *NewAccount(*NewAddressFromHex("initiator"))
	initiator.nonce = 1
	initiator.lockedBalance = 200

	// create signedTxn
	var budget float64 = 10
	var uniqID = "test"
	record := MPCRecord{
		UniqID: uniqID,
		Result: "",
	}

	// create worldstate
	worldState := storage.NewBasicKV()
	config := *NewChainConfig(
		map[string]string{
			account.addr.Hex:   "",
			initiator.addr.Hex: ""},
		1, "2h", 0, 10,
	)
	worldState.Put(STATE_CONFIG_KEY, config)
	worldState.Put(initiator.addr.Hex, initiator)
	worldState.Put(account.addr.Hex, account)
	worldState.Put(mpcKeyFromUniqID(uniqID), MPCEndorsement{
		Peers:     config.Participants,
		Endorsers: map[string]struct{}{},
		Budget: map[string]float64{
			account.addr.Hex:   budget,
			initiator.addr.Hex: budget},
		Initiator: initiator.addr.Hex,
		Locked:    true,
	})

	//> Execute first transaction

	txn := NewTransactionPostMPC(&account, record)
	err := txn.Exec(worldState)
	require.NoError(t, err)

	// > verify worldState

	newAccount := GetAccountFromWorldState(worldState, txn.From)
	require.Equal(t, account.addr.Hex, newAccount.addr.Hex)
	// threshold not reached. No award will be collect
	require.Equal(t, account.balance, newAccount.balance)
	require.Equal(t, account.lockedBalance, newAccount.lockedBalance)
	require.Equal(t, account.nonce+1, newAccount.nonce)

	newInitiator := GetAccountFromWorldState(worldState, initiator.addr.Hex)
	require.Equal(t, initiator.addr.Hex, newInitiator.addr.Hex)
	require.Equal(t, initiator.balance, newInitiator.balance)
	// threshold not reached. No award will be sent
	require.Equal(t, initiator.lockedBalance, newInitiator.lockedBalance)
	require.Equal(t, initiator.nonce, newInitiator.nonce)

	mpcendorse, err := GetMPCEndorsementFromWorldState(worldState, mpcKeyFromUniqID("test"))
	require.NoError(t, err)
	require.Equal(t, 1, len(mpcendorse.Endorsers))
	_, ok := mpcendorse.Endorsers[account.addr.Hex]
	require.True(t, ok)
	require.True(t, mpcendorse.Locked)

	//> Execute second transaction

	txn = NewTransactionPostMPC(&initiator, record)
	err = txn.Exec(worldState)
	require.NoError(t, err)

	// > verify worldState

	newAccount = GetAccountFromWorldState(worldState, account.addr.Hex)
	require.Equal(t, account.addr.Hex, newAccount.addr.Hex)
	// threshold reached. Award must be collected
	require.Equal(t, account.balance+budget, newAccount.balance)
	require.Equal(t, account.lockedBalance, newAccount.lockedBalance)
	require.Equal(t, account.nonce+1, newAccount.nonce)

	newInitiator = GetAccountFromWorldState(worldState, txn.From)
	require.Equal(t, initiator.addr.Hex, newInitiator.addr.Hex)
	// threshold reached. Award must be collected
	require.Equal(t, initiator.balance+budget, newInitiator.balance)
	// threshold reached. Award must be sent
	require.Equal(t, initiator.lockedBalance,
		newInitiator.lockedBalance+budget*2)
	require.Equal(t, initiator.nonce+1, newInitiator.nonce)

	_, ok = worldState.Get(mpcKeyFromUniqID("test"))
	require.False(t, ok)
}

func Test_Txn_Execution_PostMPC_InCorrect(t *testing.T) {
	account := *NewAccount(*NewAddressFromHex("account1"))
	account2 := *NewAccount(*NewAddressFromHex("account2"))

	// create initiator
	initiator := *NewAccount(*NewAddressFromHex("initiator"))
	initiator.nonce = 1
	initiator.lockedBalance = 200

	// create signedTxn
	var budget float64 = 10
	var uniqID = "test"
	record := MPCRecord{
		UniqID: uniqID,
		Result: "",
	}

	// create worldstate
	worldState := storage.NewBasicKV()
	config := *NewChainConfig(
		map[string]string{
			account.addr.Hex:   "",
			initiator.addr.Hex: ""},
		1, "2h", 0, 10,
	)
	worldState.Put(STATE_CONFIG_KEY, config)
	worldState.Put(initiator.addr.Hex, initiator)
	worldState.Put(account.addr.Hex, account)

	// > non-existing MPC should fail

	stateCopy := worldState.Copy()
	txn := NewTransactionPostMPC(&account, record)
	err := txn.Exec(worldState)
	require.Error(t, err)
	require.Equal(t, stateCopy.Hash(), worldState.Hash())

	// > non-participant should fail

	worldState.Put(mpcKeyFromUniqID(uniqID), MPCEndorsement{
		Peers:     config.Participants,
		Endorsers: map[string]struct{}{},
		Budget: map[string]float64{
			account.addr.Hex:   budget,
			initiator.addr.Hex: budget},
		Locked: true,
	})
	worldState.Put(account2.addr.Hex, account2)

	stateCopy = worldState.Copy()
	txn = NewTransactionPostMPC(&account2, record)
	err = txn.Exec(worldState)
	require.Error(t, err)
	require.Equal(t, stateCopy.Hash(), worldState.Hash())

	// > double claim (even with different nonce) should fail

	// first claim should succeed
	txn = NewTransactionPostMPC(&account, record)
	err = txn.Exec(worldState)
	require.NoError(t, err)
	// second claim should fail
	stateCopy = worldState.Copy()
	newAccount := *GetAccountFromWorldState(worldState, account.addr.Hex)
	require.Equal(t, account.addr.Hex, newAccount.addr.Hex)
	require.Equal(t, account.nonce+1, newAccount.nonce)
	txn = NewTransactionPostMPC(&newAccount, record)
	err = txn.Exec(worldState)
	require.Error(t, err)
	require.Equal(t, stateCopy.Hash(), worldState.Hash())
}
