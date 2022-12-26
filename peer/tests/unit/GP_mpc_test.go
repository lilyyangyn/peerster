package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/transport/channel"
	"go.dedis.ch/cs438/types"
)

func setup_n_peers(n int, t *testing.T) []z.TestNode {
	nodes := make([]z.TestNode, n)

	transp := channel.NewTransport()

	for i := 0; i < n; i++ {
		nodes[i] = z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	}

	pubkeys := make([]types.Pubkey, n)
	for i := 0; i < n; i++ {
		pubkeys[i] = nodes[i].GetPubkeyStore()[nodes[i].GetAddr()]
	}

	// add peer & setPubkey
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			nodes[i].AddPeer(nodes[j].GetAddr())
			nodes[i].SetPubkeyEntry(nodes[j].GetAddr(), &pubkeys[j])
		}
	}
	return nodes
}

func Test_GP_SHAMIR_SECRET_SHARE_SEND(t *testing.T) {

	nodes := setup_n_peers(3, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset and send sss
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA)
	require.NoError(t, err)
	go func() {
		_, err := nodeA.ComputeExpression("a", 3)
		require.NoError(t, err)
	}()
	time.Sleep(time.Second * 1)

	// check node A send sss Msg.
	nodeAOuts := nodeA.GetOuts()
	rumor := z.GetRumor(t, nodeAOuts[0].Msg)

	private := z.GetPrivate(t, rumor.Rumors[0].Msg)
	require.Len(t, private.Recipients, 1)
	require.Contains(t, private.Recipients, nodeA.GetAddr())

	_ = z.GetEncrypt(t, private.Msg)

	// check node b received sss msg
	nodeBIns := nodeB.GetIns()
	recvMsg := false
	for i := 0; i < len(nodeBIns); i++ {
		if nodeBIns[i].Msg.Type != "rumor" {
			continue
		}
		rumor = z.GetRumor(t, nodeBIns[i].Msg)
		require.Len(t, rumor.Rumors, 1)
		if rumor.Rumors[0].Msg.Type != "private" {
			continue
		}
		private = z.GetPrivate(t, rumor.Rumors[0].Msg)
		require.Len(t, private.Recipients, 1)

		_, found := private.Recipients[nodeB.GetAddr()]
		if !found {
			continue
		}

		_ = z.GetEncrypt(t, private.Msg)
		recvMsg = true
		break
	}
	require.True(t, recvMsg)

	// check node c received sss msg
	nodeCIns := nodeC.GetIns()
	recvMsg = false
	for i := 0; i < len(nodeCIns); i++ {
		if nodeCIns[i].Msg.Type != "rumor" {
			continue
		}
		rumor = z.GetRumor(t, nodeCIns[i].Msg)
		require.Len(t, rumor.Rumors, 1)
		if rumor.Rumors[0].Msg.Type != "private" {
			continue
		}
		private = z.GetPrivate(t, rumor.Rumors[0].Msg)
		require.Len(t, private.Recipients, 1)

		_, found := private.Recipients[nodeC.GetAddr()]
		if !found {
			continue
		}

		_ = z.GetEncrypt(t, private.Msg)
		recvMsg = true
		break
	}
	require.True(t, recvMsg)
}

func Test_GP_ComputeExpression_Single_Value_Send(t *testing.T) {
	nodes := setup_n_peers(3, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

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

	// check all received ans is equal
	recvValue := ans[0]
	for i := 0; i < 3; i++ {
		require.Equal(t, recvValue, ans[i])
	}

	// check equal to the expected ans
	// for i := 0; i < 3; i++ {
	// 	require.Equal(t, valueA, ans[i])
	// }

}

func Test_GP_ComputeExpression_Add(t *testing.T) {
	nodes := setup_n_peers(3, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB)
	require.NoError(t, err)

	// TODO now structure is all node will need to run compute Expression.
	// will change to only one node run expression
	ans := make([]int, 3)
	go func() {
		ansA, err := nodeA.ComputeExpression("a+b", 3)
		ans[0] = ansA
		require.NoError(t, err)
	}()
	go func() {
		ansB, err := nodeB.ComputeExpression("a+b", 3)
		ans[1] = ansB
		require.NoError(t, err)
	}()
	go func() {
		ansC, err := nodeC.ComputeExpression("a+b", 3)
		ans[2] = ansC
		require.NoError(t, err)
	}()

	time.Sleep(time.Second * 3)

	// check all received ans is equal
	recvValue := ans[0]
	for i := 0; i < 3; i++ {
		require.Equal(t, recvValue, ans[i])
	}

	// check equal to the expected ans
	// for i := 0; i < 3; i++ {
	// 	require.Equal(t, valueA+valueB, ans[i])
	// }
}

func Test_GP_ComputeExpression_Mult(t *testing.T) {
	nodes := setup_n_peers(3, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB)
	require.NoError(t, err)

	// TODO now structure is all node will need to run compute Expression.
	// will change to only one node run expression
	ans := make([]int, 3)
	go func() {
		ansA, err := nodeA.ComputeExpression("a*b", 3)
		ans[0] = ansA
		require.NoError(t, err)
	}()
	go func() {
		ansB, err := nodeB.ComputeExpression("a*b", 3)
		ans[1] = ansB
		require.NoError(t, err)
	}()
	go func() {
		ansC, err := nodeC.ComputeExpression("a*b", 3)
		ans[2] = ansC
		require.NoError(t, err)
	}()

	time.Sleep(time.Second * 3)

	// check all received ans is equal
	recvValue := ans[0]
	for i := 0; i < 3; i++ {
		require.Equal(t, recvValue, ans[i])
	}

	// check equal to the expected ans
	// for i := 0; i < 3; i++ {
	// 	require.Equal(t, valueA*valueB, ans[i])
	// }
}

func Test_GP_ComputeExpression_Complex(t *testing.T) {
	nodes := setup_n_peers(3, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA)
	require.NoError(t, err)

	valueB1 := 3
	err = nodeB.SetValueDBAsset("b1", valueB1)
	require.NoError(t, err)

	valueB2 := 2
	err = nodeB.SetValueDBAsset("b2", valueB2)
	require.NoError(t, err)

	// TODO now structure is all node will need to run compute Expression.
	// will change to only one node run expression
	ans := make([]int, 3)
	go func() {
		ansA, err := nodeA.ComputeExpression("(a+b1)*b2", 3)
		ans[0] = ansA
		require.NoError(t, err)
	}()
	go func() {
		ansB, err := nodeB.ComputeExpression("(a+b1)*b2", 3)
		ans[1] = ansB
		require.NoError(t, err)
	}()
	go func() {
		ansC, err := nodeC.ComputeExpression("(a+b1)*b2", 3)
		ans[2] = ansC
		require.NoError(t, err)
	}()

	time.Sleep(time.Second * 3)

	// check all received ans is equal
	recvValue := ans[0]
	for i := 0; i < 3; i++ {
		require.Equal(t, recvValue, ans[i])
	}

	// check equal to the expected ans
	// for i := 0; i < 3; i++ {
	// 	require.Equal(t, (valueA+valueB2)*valueB2, ans[i])
	// }
}

func Test_GP_ComputeExpression_Complete(t *testing.T) {

}
