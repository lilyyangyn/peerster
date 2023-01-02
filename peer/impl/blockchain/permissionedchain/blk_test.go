package permissioned

import (
	"testing"
)

func Test_block_builder(t *testing.T) {
	// privKey, err := crypto.GenerateKey()
	// require.NoError(t, err)
	// pubkey := privKey.PublicKey

	transactionNum := 10
	transactions := make([]SignedTransaction, transactionNum)
	for i := 0; i < transactionNum; i++ {
		transactions[i] = SignedTransaction{}
	}
}
