package permissioned

import (
	"encoding/json"
	"fmt"

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
// Utilities

var AwardUnlockThreshold = 0.5

type MPCEndorsement struct {
	// FIXME: not copy peers. Use Config ID
	Peers     map[string]struct{}
	Endorsers map[string]struct{}
	Initiator string
	Budget    float64
	Locked    bool
}

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

	threshold := float64(len(endorsement.Peers)) * AwardUnlockThreshold
	if float64(len(endorsement.Endorsers)) > threshold {
		for endorser := range endorsement.Endorsers {
			err = claimAward(worldState, initiator, endorser, endorsement.Budget)
			if err != nil {
				return err
			}
		}
		endorsement.Locked = false
		if len(endorsement.Endorsers) == len(endorsement.Peers) {
			err := worldState.Del(key)
			if err != nil {
				panic(err)
			}
		}
	}
	return nil
}
