package types

type MPCSecretValue struct {
	Owner string // node identifier
	Key   string // key in the node DB
	Value int
}

// MPCShareMessage describes a message for MPC secret sharing.
type MPCShareMessage struct {
	ReqID int
	Value MPCSecretValue
}

// MPCInterpolationMessage describes a message for MPC interpolation.
type MPCInterpolationMessage struct {
	ReqID int
	Owner string
	Value int
}

type MPCExpression struct {
}
