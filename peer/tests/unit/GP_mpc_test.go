package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/transport/channel"
)

func Test_GP_SHAMIR_SECRET_SHARE_SEND(t *testing.T) {

}

func Test_GP_ComputeExpression_Single_Value_Send(t *testing.T) {
	transp := channel.NewTransport()

	nodeA := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer nodeA.Stop()

	nodeB := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer nodeB.Stop()

	nodeC := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer nodeC.Stop()

	nodeA.AddPeer(nodeB.GetAddr())
	nodeA.AddPeer(nodeC.GetAddr())
	nodeB.AddPeer(nodeA.GetAddr())
	nodeB.AddPeer(nodeC.GetAddr())
	nodeC.AddPeer(nodeA.GetAddr())
	nodeC.AddPeer(nodeB.GetAddr())

	pubkeyA := nodeA.GetPubkeyStore()[nodeA.GetAddr()]
	pubkeyB := nodeB.GetPubkeyStore()[nodeB.GetAddr()]
	pubkeyC := nodeB.GetPubkeyStore()[nodeB.GetAddr()]

	nodeA.SetPubkeyEntry(nodeB.GetAddr(), &pubkeyB)
	nodeA.SetPubkeyEntry(nodeC.GetAddr(), &pubkeyC)
	nodeB.SetPubkeyEntry(nodeA.GetAddr(), &pubkeyA)
	nodeB.SetPubkeyEntry(nodeC.GetAddr(), &pubkeyC)
	nodeC.SetPubkeyEntry(nodeA.GetAddr(), &pubkeyA)
	nodeC.SetPubkeyEntry(nodeB.GetAddr(), &pubkeyB)

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA)
	require.NoError(t, err)

	// TODO now structure is all node will need to run compute Expression.
	// will change to only one node run expression
	ans := make([]int, 3)
	go func() {
		ansA, err := nodeA.ComputeExpression("a", 3)
		ans[0] = ansA
		require.NoError(t, err)
	}()
	go func() {
		ansB, err := nodeB.ComputeExpression("a", 3)
		ans[1] = ansB
		require.NoError(t, err)
	}()
	go func() {
		ansC, err := nodeC.ComputeExpression("a", 3)
		ans[2] = ansC
		require.NoError(t, err)
	}()

	time.Sleep(time.Second * 3)

	for i := 0; i < 3; i++ {
		require.Equal(t, valueA, ans[i])
	}

}

func Test_GP_ComputeExpression_Add(t *testing.T) {

}

func Test_GP_ComputeExpression_Mult(t *testing.T) {

}

func Test_GP_ComputeExpression_Complex(t *testing.T) {

}

func Test_GP_ComputeExpression_Complete(t *testing.T) {

}
