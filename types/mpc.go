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
	return fmt.Sprintf("{mpc secretshare for round %s: %s-%s}", m.ReqID, m.Value.Owner, m.Value.Key)
}

// HTML implements types.Message.
func (m MPCShareMessage) HTML() string {
	return fmt.Sprintf("{mpc secretshare for round %s: %s-%s}", m.ReqID, m.Value.Owner, m.Value.Key)
}

// -----------------------------------------------------------------------------
// MPCInterpolationMessage

// NewEmpty implements types.Message.
func (m MPCInterpolationMessage) NewEmpty() Message {
	return &MPCInterpolationMessage{}
}

// Name implements types.Message.
func (MPCInterpolationMessage) Name() string {
	return "mpcinterpolation"
}

// String implements types.Message.
func (m MPCInterpolationMessage) String() string {
	return fmt.Sprintf("{mpc interpolation for round %s from %s}", m.ReqID, m.Owner)
}

// HTML implements types.Message.
func (m MPCInterpolationMessage) HTML() string {
	return fmt.Sprintf("{mpc interpolation for round %s from %s}", m.ReqID, m.Owner)
}
