package peer

type MPC interface {
	// starts MPC with given expression and budget to pay
	// and returns the result
	Calculate(expression string, budget float64) (int, error)

	// start MPC with given expression and return the result
	// ComputeExpression(expression MPCExpression) (int, error)
	ComputeExpression(expr string, budget uint) (int, error)

	// SetValueDBAsset set the asset of the peers. Overwrites it if the entry
	// already exists.
	SetValueDBAsset(key string, value int) error
}
