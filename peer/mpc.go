package peer

type MPC interface {
	// starts MPC with given expression and budget to pay
	// and returns the result
	Calculate(expression string, budget float64) (int, error)

	// start MPC with given expression and return the result
	// ComputeExpression(expression MPCExpression) (int, error)
	ComputeExpression(uniqID string, expr string, prime string) (int, error)

	// SetValueDBAsset set the asset of the peers. Overwrites it if the entry
	// already exists.
	SetValueDBAsset(key string, value int, price float64) error

	// GetAllPeerAssetPrices returns the assets inside the network with the keys and prices
	GetAllPeerAssetPrices() map[string]map[string]float64

	// InitMPC inits a MPC instance for mpc before computation.
	InitMPC(uniqID string, prime string, initiator string, expression string) error
}

type MPCConsensus int

const (
	MPCConsensusPaxos MPCConsensus = iota
	MPCConsensusBC
)
