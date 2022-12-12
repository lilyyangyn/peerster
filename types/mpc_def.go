package types

// MPCShareMessage describes a message for MPC secret sharing.
type MPCShareMessage struct {
	Origin string
	ID     int
	Value  []byte
}
