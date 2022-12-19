package peer

type MPC interface {
	// start MPC with given expression and return the result
	// ComputeExpression(expression MPCExpression) (int, error)
	ComputeExpression(expr string, budget uint) (int, error)

	// SetValueDBAsset set the asset of the peers. Overwrites it if the entry
	// already exists.
	SetValueDBAsset(key string, value int) error
}

type MPCExpression struct {
}

func NewMPCExpression(exp string) *MPCExpression {
	expression := MPCExpression{}
	return &expression
}
