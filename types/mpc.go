package types

import "fmt"

// -----------------------------------------------------------------------------
// MPCShareMessage

// NewEmpty implements types.Message.
func (m MPCShareMessage) NewEmpty() Message {
	return &MPCShareMessage{}
}

// Name implements types.Message.
func (MPCShareMessage) Name() string {
	return "mpcshare"
}

// String implements types.Message.
func (m MPCShareMessage) String() string {
	return fmt.Sprintf("mpc secret share for %d from %s", m.ID, m.Origin)
}

// HTML implements types.Message.
func (m MPCShareMessage) HTML() string {
	return fmt.Sprintf("mpc secret share for %d from %s", m.ID, m.Origin)
}
