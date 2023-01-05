package types

import (
	"crypto"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// PaxosPrepareMessage defines a prepare message in Paxos
//
// - implements types.Message
// - implemented in HW3
type PaxosPrepareMessage struct {
	Type PaxosType
	Step uint
	ID   uint
	// Source is the address of the peer that sends the prepare
	Source string
}

// PaxosPromiseMessage defines a promise message in Paxos
//
// - implements types.Message
// - implemented in HW3
type PaxosPromiseMessage struct {
	Type PaxosType

	Step uint
	ID   uint

	// Irrelevant if the proposer hasn't accepted any value
	AcceptedID uint
	// Must be nil if the proposer hasn't accepted any value
	AcceptedValue *PaxosValue
}

// PaxosProposeMessage defines a propose message in Paxos
//
// - implements types.Message
// - implemented in HW3
type PaxosProposeMessage struct {
	Type PaxosType

	Step  uint
	ID    uint
	Value PaxosValue
}

// PaxosAcceptMessage defines an accept message in Paxos
//
// - implements types.Message
// - implemented in HW3
type PaxosAcceptMessage struct {
	Type PaxosType

	Step  uint
	ID    uint
	Value PaxosValue
}

// TLCMessage defines a TLC message
//
// - implements types.Message
// - implemented in HW3
type TLCMessage struct {
	Type PaxosType

	Step  uint
	Block BlockchainBlock
}

// BlockchainBlock defines the content of a block in the blockchain.
type BlockchainBlock struct {
	// Index is the index of the block in the blockchain, starting at 0 for the
	// first block.
	Index uint

	// Hash is SHA256(Index || v.UniqID || v.Filename || v.Metahash || Prevhash)
	// use crypto/sha256
	Hash []byte

	Value PaxosValue

	// PrevHash is the SHA256 hash of the previous block
	PrevHash []byte
}

// CreateTLCBlock generates a new BlockchainBlock
func CreateTLCBlock(currClock uint, val *PaxosValue, prevHash []byte) *BlockchainBlock {
	if len(prevHash) == 0 {
		prevHash = make([]byte, 32)
	}

	// create block
	block := &BlockchainBlock{
		Index:    currClock,
		Value:    *val,
		PrevHash: prevHash,
	}

	// compute block hash
	h := crypto.SHA256.New()
	h.Write([]byte(strconv.Itoa(int(block.Index))))
	h.Write([]byte(block.Value.UniqID))
	h.Write([]byte(block.Value.Type))
	h.Write([]byte(block.Value.Content))
	// h.Write([]byte(block.Value.UniqID))
	// h.Write([]byte(block.Value.Filename))
	// h.Write([]byte(block.Value.Metahash))
	h.Write(block.PrevHash)
	blockHash := h.Sum(nil)
	block.Hash = blockHash

	return block
}

// Marshal marshals the BlobkchainBlock into a byte representation. Must be used
// to store blocks in the blockchain store.
func (b *BlockchainBlock) Marshal() ([]byte, error) {
	return json.Marshal(b)
}

// Unmarshal unmarshals the data into the current instance. To unmarshal a
// block:
//
//	var block BlockchainBlock
//	err := block.Unmarshal(buf)
func (b *BlockchainBlock) Unmarshal(data []byte) error {
	return json.Unmarshal(data, b)
}

// String returns a string representation of a blokchain block.
func (b BlockchainBlock) String() string {
	return fmt.Sprintf("{block n°%d H(%x) - %v - %x", b.Index, b.Hash[:4],
		b.Value, b.PrevHash[:4])
}

// DisplayBlock writes a rich string representation of a block
func (b BlockchainBlock) DisplayBlock(out io.Writer) {
	crop := func(s string) string {
		if len(s) > 10 {
			return s[:8] + "..."
		}
		return s
	}

	max := func(s ...string) int {
		max := 0
		for _, se := range s {
			if len(se) > max {
				max = len(se)
			}
		}
		return max
	}

	pad := func(n int, s ...*string) {
		for _, se := range s {
			*se = fmt.Sprintf("%-*s", n, *se)
		}
	}

	row1 := fmt.Sprintf("%d | %x", b.Index, b.Hash[:6])
	row2 := fmt.Sprintf("T | %s", crop(string(b.Value.Type)))
	row3 := fmt.Sprintf("V | %s", crop(b.Value.String()))
	row5 := fmt.Sprintf("<- %x", b.PrevHash[:6])

	m := max(row1, row2, row3, row5)
	pad(m, &row1, &row2, &row3, &row5)

	fmt.Fprintf(out, "\n┌%s┐\n", strings.Repeat("─", m+2))
	fmt.Fprintf(out, "│ %s │\n", row1)
	fmt.Fprintf(out, "│%s│\n", strings.Repeat("─", m+2))
	fmt.Fprintf(out, "│ %s │\n", row2)
	fmt.Fprintf(out, "│ %s │\n", row3)
	fmt.Fprintf(out, "│%s│\n", strings.Repeat("─", m+2))
	fmt.Fprintf(out, "│ %s │\n", row5)
	fmt.Fprintf(out, "└%s┘\n", strings.Repeat("─", m+2))
}
