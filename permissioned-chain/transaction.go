package permissioned

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"go.dedis.ch/cs438/storage"
)

// -----------------------------------------------------------------------------
// Transaction

type TxnType string

const (
	TxnTypeCoinbase TxnType = "txn-coinbase"
	TxnTypePreMPC   TxnType = "txn-prempc"
	TxnTypePostMPC  TxnType = "txn-postmpc"

	TxnTypeInitConfig TxnType = "txn-initconfig"
	TxnTypeSetPubkey  TxnType = "txn-regenckey"
)

var txnHandlerStore = map[TxnType]func(storage.KVStore, *ChainConfig, *Transaction) error{
	TxnTypeCoinbase: execCoinbase,
	TxnTypePreMPC:   execPreMPC,
	TxnTypePostMPC:  execPostMPC,

	TxnTypeInitConfig: execInitConfig,
	TxnTypeSetPubkey:  execRegEnckey,
}

var txnUnmarshalerStore = map[TxnType]func(json.RawMessage) (interface{}, error){
	TxnTypeCoinbase: unmarshalCoinbase,
	TxnTypePreMPC:   unmarshalPreMPC,
	TxnTypePostMPC:  unmarshalPostMPC,

	TxnTypeInitConfig: unmarshalInitConfig,
	TxnTypeSetPubkey:  unmarshalRegEnckey,
}

// -----------------------------------------------------------------------------
// Transaction

// Transaction represents the transaction that happens inside this chain
type Transaction struct {
	ID    string
	Nonce uint
	From  string
	To    string
	Type  TxnType
	Value float64
	Data  interface{}
}

// NewTransaction creates a new transaction and computes its ID
func NewTransaction(from *Account, to *Address, txntype TxnType,
	value float64, data interface{}) *Transaction {
	txn := Transaction{
		Nonce: from.nonce,
		From:  from.addr.Hex,
		To:    to.Hex,
		Type:  txntype,
		Value: value,
		Data:  data,
	}
	txn.ID = txn.Hash()

	return &txn
}

// HashBytes computes the hash of the transaction
func (txn *Transaction) HashBytes() []byte {
	h := sha256.New()

	h.Write([]byte(fmt.Sprintf("%d", txn.Nonce)))
	h.Write([]byte(txn.From))
	h.Write([]byte(txn.To))
	h.Write([]byte(txn.Type))
	h.Write([]byte(fmt.Sprintf("%f", txn.Value)))

	switch hh := txn.Data.(type) {
	case Hashable:
		h.Write([]byte(hh.Hash()))
	default:
		bytes, err := json.Marshal(txn.Data)
		if err != nil {
			panic(err)
		}
		h.Write(bytes)
	}

	return h.Sum(nil)
}

// Hash computes the hex-encoded hash of the transaction
func (txn *Transaction) Hash() string {
	return hex.EncodeToString(txn.HashBytes())
}

// String returns a description string for the transaction
func (txn *Transaction) String() string {
	return fmt.Sprintf("{%s: from=%s, id=%s}", txn.Type, txn.Hash(), txn.ID)
}

// Unmarshal helps to unmarshal the data part
func (txn *Transaction) Unmarshal() error {
	if txn.Data == nil {
		return nil
	}

	dict, ok := txn.Data.(map[string]interface{})
	if !ok {
		// simple type, no need to do futhur operations
		return nil
	}
	jsonbody, err := json.Marshal(dict)
	if err != nil {
		return err
	}

	unmarshaler, ok := txnUnmarshalerStore[txn.Type]
	if !ok {
		return fmt.Errorf("invalid transaction type: %s", txn.Type)
	}

	data, err := unmarshaler(jsonbody)
	if err != nil {
		return err
	}

	txn.Data = data
	return nil
}

// Exec executes the transaction based on the input worldState
func (txn *Transaction) Exec(worldState storage.KVStore) error {
	config := GetConfigFromWorldState(worldState)

	// check nonce
	err := checkNonce(worldState, txn)
	if err != nil {
		return err
	}

	// execute handler
	handler, ok := txnHandlerStore[txn.Type]
	if !ok {
		return fmt.Errorf("invalid transaction type: %s", txn.Type)
	}
	err = handler(worldState, config, txn)
	if err != nil {
		return err
	}

	// advance nonce to avoid replay attack
	return updateNonce(worldState, txn.From)
}

// -----------------------------------------------------------------------------
// Signed Transaction

type SignedTransaction struct {
	Txn       Transaction
	Signature []byte
}

// Sign creates a signature for the trasaction using the given private key
func (txn *Transaction) Sign(privateKey *ecdsa.PrivateKey) (*SignedTransaction, error) {
	// no signature if no key is provided
	if privateKey == nil {
		return &SignedTransaction{Txn: *txn}, nil
	}

	signature, err := crypto.Sign(txn.HashBytes(), privateKey)
	if err != nil {
		return nil, err
	}

	return &SignedTransaction{Txn: *txn, Signature: signature}, nil
}

// HashBytes computes the hashbytes of the signed transaction
func (signedTxn *SignedTransaction) HashBytes() []byte {
	h := sha256.New()

	h.Write(signedTxn.Txn.HashBytes())
	h.Write([]byte("||"))
	h.Write(signedTxn.Signature)

	return h.Sum(nil)
}

// Hash computes the hash of the signed transaction
func (signedTxn *SignedTransaction) Hash() string {
	return hex.EncodeToString(signedTxn.HashBytes())
}

// String returns a description string for the transaction
func (signedTxn *SignedTransaction) String() string {
	txn := signedTxn.Txn
	return fmt.Sprintf("{%s(signed): from=%s, id=%s, sig=%s}",
		txn.Type, txn.From, txn.Hash(), hex.EncodeToString(signedTxn.Signature))
}

// Verify verify the signature and then execute to see whether the result is consistent with worldState
func (signedTxn *SignedTransaction) Verify(worldState storage.KVStore) error {
	txn := signedTxn.Txn
	config := GetConfigFromWorldState(worldState)

	// verify origin is inside the chain
	if !CheckPariticipation(worldState, config, txn.From) {
		return fmt.Errorf("address %s is not a participant of the permissined chain", txn.From)
	}

	// verify signature
	if txn.Type != TxnTypeCoinbase && txn.Type != TxnTypeInitConfig {
		digestHash := txn.HashBytes()
		publicKey, err := crypto.SigToPub(digestHash, signedTxn.Signature)
		if err != nil {
			return err
		}
		addr := NewAddress(publicKey)
		if addr.Hex != txn.From {
			return fmt.Errorf("transaction %s is not signed by sender %s", signedTxn.Txn.ID, signedTxn.Txn.From)
		}
		// verify sig input needs to be in [R || S] format
		sigValid := crypto.VerifySignature(crypto.FromECDSAPub(publicKey), digestHash, signedTxn.Signature[:len(signedTxn.Signature)-1])
		if !sigValid {
			return fmt.Errorf("transaction %s has invalid signature from %s", signedTxn.Txn.ID, signedTxn.Txn.From)
		}
	}

	// execute txn
	stateCopy := worldState.Copy()
	err := txn.Exec(stateCopy)
	if err == nil {
		// check before real execution
		_ = txn.Exec(worldState)
	}

	return err
}

// -----------------------------------------------------------------------------
// Transaction Polymophism - Coinbase

func NewTransactionCoinbase(to Address, value float64) *Transaction {
	return NewTransaction(
		NewAccount(ZeroAddress),
		&to,
		TxnTypeCoinbase,
		value,
		nil,
	)
}

func execCoinbase(worldState storage.KVStore, config *ChainConfig, txn *Transaction) error {
	account := GetAccountFromWorldState(worldState, txn.To)
	account.balance += txn.Value
	worldState.Put(account.addr.Hex, *account)
	return nil
}

func unmarshalCoinbase(data json.RawMessage) (interface{}, error) {
	return nil, nil
}

// -----------------------------------------------------------------------------
// Utilities

type Describable interface {
	String() string
}

type Hashable interface {
	Hash() string
}

func CheckPariticipation(worldState storage.KVStore, config *ChainConfig, addrID string) bool {
	if addrID == ZeroAddress.Hex {
		return true
	}
	if config == nil {
		return false
	}
	if _, ok := config.Participants[addrID]; ok {
		return true
	}
	return false
}

func GetAccountFromWorldState(worldState storage.KVStore, key string) *Account {
	object, ok := worldState.Get(key)
	if !ok {
		return NewAccount(*NewAddressFromHex(key))
	}
	account := object.(Account)
	return &account
}

func GetConfigFromWorldState(worldState storage.KVStore) *ChainConfig {
	object, ok := worldState.Get(STATE_CONFIG_KEY)
	if !ok {
		// panic(fmt.Errorf("config not exists"))
		return nil
	}
	config := object.(ChainConfig)
	return &config
}

func mpcKeyFromUniqID(uniqID string) string {
	return fmt.Sprintf("ongoging-mpc-%s", uniqID)
}

func checkNonce(worldState storage.KVStore, txn *Transaction) error {
	// Do nothing to zeroaddress
	if txn.From == ZeroAddress.Hex {
		return nil
	}

	account := GetAccountFromWorldState(worldState, txn.From)
	if account.nonce != txn.Nonce {
		return fmt.Errorf("transaction %s has invalid nonce from %s. Expected: %d, Got: %d",
			txn.ID, txn.From, account.nonce, txn.Nonce)
	}
	return nil
}

func updateNonce(worldState storage.KVStore, accountID string) error {
	// Do nothing to zeroaddress
	if accountID == ZeroAddress.Hex {
		return nil
	}

	account := GetAccountFromWorldState(worldState, accountID)
	account.nonce++

	err := worldState.Put(accountID, *account)
	if err != nil {
		panic(err)
	}
	return nil
}

func lockBalance(worldState storage.KVStore, accountID string, amount float64) error {
	account := GetAccountFromWorldState(worldState, accountID)
	if account.balance < amount {
		return fmt.Errorf("Initiator(%s) balance not enough. Remain balance: %f",
			accountID, account.balance)
	}

	// lock balance
	account.balance -= amount
	account.lockedBalance += amount
	err := worldState.Put(accountID, *account)
	if err != nil {
		panic(err)
	}
	return nil
}

func claimAward(worldState storage.KVStore, from *Account, to string, amount float64) error {
	account := from
	if from.addr.Hex != to {
		account = GetAccountFromWorldState(worldState, to)
	}

	if from.lockedBalance < amount {
		return fmt.Errorf("%s's locked balance not enough. Expected: %f. Got: %f",
			from.addr.Hex, amount, from.lockedBalance)
	}
	from.lockedBalance -= amount
	account.balance += amount
	err := worldState.Put(from.addr.Hex, *from)
	if err != nil {
		panic(err)
	}
	err = worldState.Put(account.addr.Hex, *account)
	if err != nil {
		panic(err)
	}
	return nil
}
