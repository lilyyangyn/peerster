package permissioned

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"go.dedis.ch/cs438/storage"
)

// -----------------------------------------------------------------------------
// Transaction Polymophism - PreMPC

type MPCPropose struct {
	Initiator  string
	Budget     float64
	Expression string
	Prime      string
}

// String implements Describable.String()
func (p MPCPropose) String() string {
	return fmt.Sprintf("Initiator: %s, Budget: %f, Expression: %s, Prime: %s\n",
		p.Initiator, p.Budget, p.Expression, p.Prime)
}

func NewTransactionPreMPC(initiator *Account, data MPCPropose) *Transaction {
	return NewTransaction(
		initiator,
		&ZeroAddress,
		TxnTypePreMPC,
		data.Budget,
		data,
	)
}

func execPreMPC(worldState storage.KVStore, config *ChainConfig, txn *Transaction) error {
	record := txn.Data.(MPCPropose)
	if record.Budget != txn.Value || record.Initiator != txn.From {
		return fmt.Errorf("Transaction data inconsistent")
	}

	// lock balance to avoid double spending
	err := lockBalance(worldState, txn.From, record.Budget*float64(len(config.Participants)))
	if err != nil {
		return err
	}
	// add MPC record to worldState
	// use txnHash has uniqID
	err = worldState.Put(mpcKeyFromUniqID(txn.Hash()), MPCEndorsement{
		Peers:     config.Participants,
		Endorsers: map[string]struct{}{},
		Initiator: txn.From,
		Budget:    record.Budget,
		Locked:    true,
	})
	if err != nil {
		panic(err)
	}

	return nil
}

func unmarshalPreMPC(data json.RawMessage) (interface{}, error) {
	var p MPCPropose
	err := json.Unmarshal(data, &p)

	return p, err
}

// -----------------------------------------------------------------------------
// Transaction Polymophism - PostMPC

type MPCRecord struct {
	UniqID string
	Result float64
}

// String implements Describable.String()
func (p MPCRecord) String() string {
	return fmt.Sprintf("UniqID: %s, Result: %f\n", p.UniqID, p.Result)
}

func NewTransactionPostMPC(from *Account, data MPCRecord) *Transaction {
	return NewTransaction(
		from,
		&ZeroAddress,
		TxnTypePostMPC,
		0,
		data,
	)
}

func execPostMPC(worldState storage.KVStore, config *ChainConfig, txn *Transaction) error {
	record := txn.Data.(MPCRecord)

	// update endorsement information, collect awawrd if threshold is reached
	err := updateMPCEndorsement(worldState, mpcKeyFromUniqID(record.UniqID), txn.From)
	if err != nil {
		return err
	}

	return nil
}

func unmarshalPostMPC(data json.RawMessage) (interface{}, error) {
	var p MPCRecord
	err := json.Unmarshal(data, &p)

	return p, err
}

// -----------------------------------------------------------------------------
// Transaction Polymophism - RegAssets

func NewTransactionRegAssets(from *Account, assets map[string]float64) *Transaction {
	return NewTransaction(
		from,
		&ZeroAddress,
		TxnTypeRegAssets,
		0,
		assets,
	)
}

func execRegAssets(worldState storage.KVStore, config *ChainConfig, txn *Transaction) error {
	assets := txn.Data.(map[string]float64)

	key := assetsKeyFromUniqID(txn.From)
	oldAssets := GetAssetsFromWorldState(worldState, key)
	oldAssets.Add(assets)

	err := worldState.Put(key, oldAssets)
	if err != nil {
		panic(err)
	}

	return nil
}

func unmarshalRegAssets(data json.RawMessage) (interface{}, error) {
	var r map[string]float64
	err := json.Unmarshal(data, &r)

	return r, err
}

// -----------------------------------------------------------------------------
// Utilities - Assets

type AssetsRecord struct {
	Owner  string
	Assets map[string]float64
}

// Copy implements Copyable.Copy()
func (r AssetsRecord) Copy() storage.Copyable {
	assets := map[string]float64{}
	for asset, price := range r.Assets {
		assets[asset] = price
	}
	record := AssetsRecord{
		Owner:  r.Owner,
		Assets: assets,
	}
	return record
}

// Hash implements Hashable.Hash
func (r AssetsRecord) Hash() string {
	h := sha256.New()

	h.Write([]byte(r.Owner))
	assets := make([]string, 0, len(r.Assets))
	for asset, _ := range r.Assets {
		assets = append(assets, asset)
	}
	sort.Strings(assets)
	for _, asset := range assets {
		h.Write([]byte(asset))
		h.Write([]byte(fmt.Sprintf("%f", r.Assets[asset])))
	}

	return hex.EncodeToString(h.Sum(nil))
}

func NewAssetsRecord() *AssetsRecord {
	return &AssetsRecord{
		Assets: map[string]float64{},
	}
}

func (r AssetsRecord) Add(newAssets map[string]float64) {
	for newAsset, price := range newAssets {
		r.Assets[newAsset] = price
	}
}

func GetAssetsFromWorldState(worldState storage.KVStore, key string) *AssetsRecord {
	object, ok := worldState.Get(key)
	if !ok {
		return NewAssetsRecord()
	}
	assets := object.(AssetsRecord)
	return &assets
}

func assetsKeyFromUniqID(uniqID string) string {
	return fmt.Sprintf("assets|%s", uniqID)
}

// -----------------------------------------------------------------------------
// Utilities - MPC

var AWARD_UNLOCK_THRESHOLD = 0.5

type MPCEndorsement struct {
	// FIXME: not copy peers. Use Config ID
	Peers     map[string]string
	Endorsers map[string]struct{}
	Initiator string
	Budget    float64
	Locked    bool
}

// Copy implements Copyable.Copy()
func (e MPCEndorsement) Copy() storage.Copyable {
	endorsers := map[string]struct{}{}
	for endorser := range e.Endorsers {
		endorsers[endorser] = struct{}{}
	}
	endorsement := MPCEndorsement{
		Peers:     e.Peers,
		Endorsers: endorsers,
		Initiator: e.Initiator,
		Budget:    e.Budget,
		Locked:    e.Locked,
	}
	return endorsement
}

func GetMPCEndorsementFromWorldState(worldState storage.KVStore, key string) (*MPCEndorsement, error) {
	object, ok := worldState.Get(key)
	if !ok {
		return nil, fmt.Errorf("MPC endorsement not exists")
	}
	endorsement := object.(MPCEndorsement)
	return &endorsement, nil
}

func mpcKeyFromUniqID(uniqID string) string {
	return fmt.Sprintf("ongoging-mpc|%s", uniqID)
}

func updateMPCEndorsement(worldState storage.KVStore, key string, accountID string) error {
	endorsement, err := GetMPCEndorsementFromWorldState(worldState, key)
	if err != nil {
		return fmt.Errorf("%s endorses a non-existing MPC %s", accountID, key)
	}
	if _, ok := endorsement.Peers[accountID]; !ok {
		return fmt.Errorf("%s does not participant in MPC %s. Potentially an attack", accountID, key)
	}
	if _, ok := endorsement.Endorsers[accountID]; ok {
		return fmt.Errorf("%s has already endorsed in MPC %s. Potentially a double-claim", accountID, key)
	}

	endorsement.Endorsers[accountID] = struct{}{}
	initiator := GetAccountFromWorldState(worldState, endorsement.Initiator)
	if !endorsement.Locked {
		if len(endorsement.Endorsers) == len(endorsement.Peers) {
			err := worldState.Del(key)
			if err != nil {
				panic(err)
			}
		}
		return claimAward(worldState, initiator, accountID, endorsement.Budget)
	}

	threshold := float64(len(endorsement.Peers)) * AWARD_UNLOCK_THRESHOLD
	if float64(len(endorsement.Endorsers)) > threshold {
		for endorser := range endorsement.Endorsers {
			err = claimAward(worldState, initiator, endorser, endorsement.Budget)
			if err != nil {
				return err
			}
		}

		// delete if fully claimed
		if len(endorsement.Endorsers) == len(endorsement.Peers) {
			err := worldState.Del(key)
			if err != nil {
				panic(err)
			}
			return nil
		}

		// unlock endorsement
		endorsement.Locked = false
		err = worldState.Put(key, *endorsement)
		if err != nil {
			panic(err)
		}
	}
	return nil
}
