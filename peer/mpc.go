package peer

type MPC interface {
	// start MPC with given expression and return the result
	Calculate(expression MPCExpression) (int, error)
}

type MPCExpression struct {
}

func NewMPCExpression(exp string) *MPCExpression {
	expression := MPCExpression{}
	return &expression
}
