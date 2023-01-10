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

	// calculate the total price need for the transaction
	prices, totalPrice, err := CalculateTotalPrice(worldState, record.Expression)
	if err != nil {
		return err
	}
	if totalPrice > txn.Value {
		return fmt.Errorf("price not enough to pay for MPC. Expected: %f. Got: %f", totalPrice, txn.Value)
	}

	// lock balance to avoid double spending
	// FIXME: now do not support customized fee
	err = lockBalance(worldState, txn.From, totalPrice)
	if err != nil {
		return err
	}
	// add MPC record to worldState
	// use txnHash has uniqID
	err = worldState.Put(mpcKeyFromUniqID(txn.Hash()), MPCEndorsement{
		Peers:     config.Participants,
		Endorsers: map[string]struct{}{},
		Initiator: txn.From,
		Budget:    prices,
		Fee:       config.MPCParticipationGain,
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
	Result string
}

// String implements Describable.String()
func (p MPCRecord) String() string {
	return fmt.Sprintf("UniqID: %s, ResultHash: %s\n", p.UniqID, p.Result)
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
// Utilities - MPC

var AWARD_UNLOCK_THRESHOLD = 0.5

type MPCEndorsement struct {
	// FIXME: not copy peers. Use Config ID
	Peers     map[string]string
	Endorsers map[string]struct{}
	Initiator string
	Locked    bool
	Budget    map[string]float64
	Fee       float64
}

// Hash implements Hashable.Hash()
func (e MPCEndorsement) Hash() string {
	h := sha256.New()

	keys := make([]string, 0, len(e.Peers))
	for k := range e.Peers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, participant := range keys {
		h.Write([]byte(participant))
	}

	keys = make([]string, 0, len(e.Endorsers))
	for k := range e.Endorsers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, participant := range keys {
		h.Write([]byte(participant))
	}

	h.Write([]byte(e.Initiator))
	h.Write([]byte(fmt.Sprintf("%t", e.Locked)))

	keys = make([]string, 0, len(e.Budget))
	for k := range e.Endorsers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, participant := range keys {
		h.Write([]byte(participant))
		h.Write([]byte(fmt.Sprintf("%f", e.Budget[participant])))
	}

	h.Write([]byte(fmt.Sprintf("%f", e.Fee)))

	return hex.EncodeToString(h.Sum(nil))
}

// Copy implements Copyable.Copy()
func (e MPCEndorsement) Copy() storage.Copyable {
	endorsers := map[string]struct{}{}
	for endorser := range e.Endorsers {
		endorsers[endorser] = struct{}{}
	}
	budget := map[string]float64{}
	for k, v := range e.Budget {
		budget[k] = v
	}
	endorsement := MPCEndorsement{
		Peers:     e.Peers,
		Endorsers: endorsers,
		Initiator: e.Initiator,
		Budget:    budget,
		Fee:       e.Fee,
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

func CalculateTotalPrice(worldState storage.KVStore, expression string) (map[string]float64, float64, error) {
	prices, err := calculateExprPrices(worldState, expression)
	if err != nil {
		return nil, 0, err
	}
	var valuePrice float64 = 0
	for _, price := range prices {
		valuePrice += price
	}

	config := GetConfigFromWorldState(worldState)
	totalPrice := valuePrice + config.MPCParticipationGain*float64(len(config.Participants))

	return prices, totalPrice, nil
}

func mpcKeyFromUniqID(uniqID string) string {
	return fmt.Sprintf("ongoging-mpc|%s", uniqID)
}

func calculateExprPrices(worldState storage.KVStore, expression string) (map[string]float64, error) {
	_, variables, err := GetPostfixAndVariables(expression)
	if err != nil {
		return nil, err
	}

	// FIXME: now only support all variable names to be distinct
	assets := GetAllAssetsFromWorldState(worldState)

	prices := make(map[string]float64)
	for peer, peerAssets := range assets {
		for asset := range variables {
			price, ok := peerAssets[asset]
			if !ok {
				continue
			}
			delete(variables, asset)

			if price > 0 {
				prices[peer] += price
			}
		}
	}
	if len(variables) > 0 {
		fmt.Println(variables)
		missing := ""
		for v := range variables {
			missing += fmt.Sprintf(", %s", v)
		}
		missing = missing[2:]
		return nil, fmt.Errorf("expression \"%s\" needs missing variables: {%s}", expression, missing)
	}

	return prices, nil
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
		err := claimAward(worldState, initiator, accountID, endorsement.Budget[accountID]+endorsement.Fee)
		if err != nil {
			return err
		}
		if len(endorsement.Endorsers) == len(endorsement.Peers) {
			err := worldState.Del(key)
			if err != nil {
				panic(err)
			}
		}
		return nil
	}

	threshold := float64(len(endorsement.Peers)) * AWARD_UNLOCK_THRESHOLD
	if float64(len(endorsement.Endorsers)) > threshold {
		for endorser := range endorsement.Endorsers {
			err = claimAward(worldState, initiator, endorser, endorsement.Budget[endorser]+endorsement.Fee)
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
