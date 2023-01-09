package types

import (
	"encoding/json"
	"fmt"

	"golang.org/x/xerrors"
)

// -----------------------------------------------------------------------------

type PaxosType string

const (
	PaxosTypeTag PaxosType = "PaxosTagValue"
	PaxosTypeMPC           = "PaxosMPCValue"
)

// -----------------------------------------------------------------------------

// PaxosValue defines the value on which Paxos makes a consensus.
type PaxosValue struct {
	Type    PaxosType
	UniqID  string
	Content json.RawMessage
}

// String returns a string representation.
func (p PaxosValue) String() string {
	c, ok := PaxosValueContents[p.Type]
	if !ok {
		return fmt.Sprintf("{paxosvalue: %s - [Unknown]}", p.Type)
	}

	content := c.NewEmpty()
	err := json.Unmarshal(p.Content, &content)
	if err != nil {
		return fmt.Sprintf("{paxosvalue: %s - [Unknown]}", p.Type)
	}

	return fmt.Sprintf("{paxosvalue: %s - [%s]}", p.Type, content.String())
}

// -----------------------------------------------------------------------------

type PaxosValueContent interface {
	NewEmpty() PaxosValueContent
	Name() PaxosType
	String() string
	UniqIdentifier() string
}

var PaxosValueContents = map[PaxosType]PaxosValueContent{
	PaxosTypeTag: PaxosTagValue{},
	PaxosTypeMPC: PaxosMPCValue{},
}

func ParsePaxosValueContent(value *PaxosValue) (PaxosValueContent, error) {
	c, ok := PaxosValueContents[value.Type]
	if !ok {
		return nil, xerrors.Errorf("paxos value not registered: %s", value.Type)
	}

	content := c.NewEmpty()
	err := json.Unmarshal(value.Content, &content)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal content: %v", err)
	}

	return content, nil
}

func CreatePaxosValue(content PaxosValueContent) (*PaxosValue, error) {
	c, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}

	return &PaxosValue{
		Type:    content.Name(),
		UniqID:  content.UniqIdentifier(),
		Content: c,
	}, nil
}

// -----------------------------------------------------------------------------
// TagValue

type PaxosTagValue struct {
	// UniqID is used to group and count same values. Use xid.New().String() to
	// generate it.
	UniqID string

	Filename string
	Metahash string
}

// NewEmpty implements types.PaxosValueContent.
func (v PaxosTagValue) NewEmpty() PaxosValueContent {
	return &PaxosTagValue{}
}

// Name implements types.PaxosValueContent.
func (v PaxosTagValue) Name() PaxosType {
	return PaxosTypeTag
}

// String implements types.PaxosValueContent.
func (v PaxosTagValue) String() string {
	return fmt.Sprintf("(paxos Tag - {Filename: %s, Metahash: %s})", v.Filename, v.Metahash)
}

// UniqID implements types.PaxosTagValue.
func (v PaxosTagValue) UniqIdentifier() string {
	return v.UniqID
}

// -----------------------------------------------------------------------------
// MPCValue

type PaxosMPCValue struct {
	UniqID     string
	Initiator  string
	Budget     float64
	Expression string
	Prime      string
}

// NewEmpty implements types.PaxosValueContent.
func (v PaxosMPCValue) NewEmpty() PaxosValueContent {
	return &PaxosMPCValue{}
}

// Name implements types.PaxosValueContent.
func (v PaxosMPCValue) Name() PaxosType {
	return PaxosTypeMPC
}

// String implements types.PaxosValueContent.
func (v PaxosMPCValue) String() string {
	return fmt.Sprintf("(paxos MPC - {Initiator: %s, Budget: %f})", v.Initiator, v.Budget)
}

// UniqID implements types.PaxosTagValue.
func (v PaxosMPCValue) UniqIdentifier() string {
	return v.UniqID
}
