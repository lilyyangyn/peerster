package permissioned

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.dedis.ch/cs438/storage"
)

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

	key := AssetsKeyFromUniqID(txn.From)
	oldAssets := GetAssetsFromWorldState(worldState, txn.From)
	oldAssets.Add(assets)

	err := worldState.Put(key, *oldAssets)
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
	for asset := range r.Assets {
		assets = append(assets, asset)
	}
	sort.Strings(assets)
	for _, asset := range assets {
		h.Write([]byte(asset))
		h.Write([]byte(fmt.Sprintf("%f", r.Assets[asset])))
	}

	return hex.EncodeToString(h.Sum(nil))
}

func NewAssetsRecord(owner string) *AssetsRecord {
	return &AssetsRecord{
		Owner:  owner,
		Assets: map[string]float64{},
	}
}

func (r AssetsRecord) Add(newAssets map[string]float64) {
	for newAsset, price := range newAssets {
		r.Assets[newAsset] = price
	}
}

func GetAssetsFromWorldState(worldState storage.KVStore, owner string) *AssetsRecord {
	object, ok := worldState.Get(AssetsKeyFromUniqID(owner))
	if !ok {
		return NewAssetsRecord(owner)
	}
	assets := object.(AssetsRecord)
	return &assets
}

func AssetsKeyFromUniqID(uniqID string) string {
	return fmt.Sprintf("assets|%s", uniqID)
}

func GetAllAssetsFromWorldState(worldState storage.KVStore) map[string]map[string]float64 {
	assets := make(map[string]map[string]float64)
	config := GetConfigFromWorldState(worldState)
	for participate := range config.Participants {
		prices := GetAssetsFromWorldState(worldState, participate).Assets
		if len(prices) == 0 {
			continue
		}
		assets[participate] = prices
	}
	return assets
}

// -----------------------------------------------------------------------------
// Utils - Expression Parser

func GetPostfixAndVariables(expr string) ([]string, map[string]struct{}, error) {
	// change infix to postfix
	postfix, err := infixToPostfix(expr)
	if err != nil {
		return []string{}, map[string]struct{}{}, err
	}

	variablesNeed := map[string]struct{}{}
	for _, exp := range postfix {
		var IsVariableName = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`).MatchString
		if IsVariableName(exp) {
			variablesNeed[exp] = struct{}{}
		}
	}
	return postfix, variablesNeed, nil
}

func infixToPostfix(infix string) ([]string, error) {
	// '+', "-", is not used as a unary operation (i.e., "+1", "-(2 + 3)"", "-1", "3-(-2)" are invalid).
	infix = strings.ReplaceAll(infix, " ", "")
	var NoInValidChar = regexp.MustCompile(`^[a-zA-Z0-9_\+\-\*\/\^()\.]+$`).MatchString
	if !NoInValidChar(infix) {
		return []string{}, fmt.Errorf("infix contains illegal character")
	}

	var IsVariableName = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`).MatchString
	s := Stack{}
	postfix := []string{}
	valid := true

	curVariable := ""
	for _, char := range infix {
		opchar := string(char)
		// if scanned character is operand, add it to output string
		if IsVariableName(opchar) {
			curVariable += opchar
			continue
		} else {
			if curVariable != "" {
				postfix = append(postfix, curVariable)
			}
			curVariable = ""
		}

		if char == '(' {
			s.Push(opchar)
		} else if char == ')' {
			for s.Top() != "(" {
				postfix = append(postfix, s.Top())
				valid = valid && s.Pop()
			}
			valid = valid && s.Pop()
		} else {
			for !s.IsEmpty() && prec(opchar) <= prec(s.Top()) {
				postfix = append(postfix, s.Top())
				valid = valid && s.Pop()
			}
			s.Push(opchar)
		}

		if !valid {
			return []string{}, fmt.Errorf("infix is invalid")
		}
	}
	if curVariable != "" {
		postfix = append(postfix, curVariable)
	}
	// Pop all the remaining elements from the stack
	for !s.IsEmpty() {
		postfix = append(postfix, s.Top())
		s.Pop()
	}
	return postfix, nil
}

// --------------------------------------------------------
// Util Struct - Stack

type Stack []string

// IsEmpty: check if stack is empty
func (st *Stack) IsEmpty() bool {
	return len(*st) == 0
}

// Push a new value onto the stack
func (st *Stack) Push(str string) {
	*st = append(*st, str) //Simply append the new value to the end of the stack
}

// Remove top element of stack. Return false if stack is empty.
func (st *Stack) Pop() bool {
	if st.IsEmpty() {
		return false
	} else {
		index := len(*st) - 1 // Get the index of top most element.
		*st = (*st)[:index]   // Remove it from the stack by slicing it off.
		return true
	}
}

// Return top element of stack. Return false if stack is empty.
func (st *Stack) Top() string {
	if st.IsEmpty() {
		return ""
	} else {
		index := len(*st) - 1   // Get the index of top most element.
		element := (*st)[index] // Index onto the slice and obtain the element.
		return element
	}
}

// Function to return precedence of operators
func prec(s string) int {
	if s == "^" {
		return 3
	} else if (s == "/") || (s == "*") {
		return 2
	} else if (s == "+") || (s == "-") {
		return 1
	} else {
		return -1
	}
}
