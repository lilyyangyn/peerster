package peer

type MPC interface {
	// starts MPC with given expression and budget to pay
	// and returns the result
	Calculate(expression string, budget float64) (int, error)
}
