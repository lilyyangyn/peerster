package peer

import "go.dedis.ch/cs438/types"

type MPC interface {
	// start MPC with given expression and return the result
	Calculate(expression types.MPCExpression) (int, error)
}

func NewMPCExpression(exp string) *types.MPCExpression {
	expression := types.MPCExpression{}
	return &expression
}
