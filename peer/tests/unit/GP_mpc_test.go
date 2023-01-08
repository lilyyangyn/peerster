package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/transport/channel"
	"go.dedis.ch/cs438/types"
)

func setup_n_peers(n int, t *testing.T, opt ...z.Option) []z.TestNode {
	nodes := make([]z.TestNode, n)

	transp := channel.NewTransport()

	for i := 0; i < n; i++ {
		node := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0",
			z.WithMPCPaxos(),
			z.WithTotalPeers(uint(n)), z.WithPaxosID(uint(i+1)))
		nodes[i] = node
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
	err := nodeA.SetValueDBAsset("a", valueA, 0)
	require.NoError(t, err)

	// all node will run compute Expression simultaneously.
	prime := "1000000009"
	uniqID := "test"
	expression := "a"

	// init the information for all nodes
	for _, n := range nodes {
		err := n.InitMPC(uniqID, prime, nodeA.GetAddr(), expression)
		require.NoError(t, err)
	}

	ans := make([]int, 3)
	go func() {
		ansA, err := nodeA.ComputeExpression("test", "a", prime)
		ans[0] = ansA
		require.NoError(t, err)
	}()
	go func() {
		ansB, err := nodeB.ComputeExpression("test", "a", prime)
		ans[1] = ansB
		require.NoError(t, err)
	}()
	go func() {
		ansC, err := nodeC.ComputeExpression("test", "a", prime)
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
	for i := 0; i < 3; i++ {
		require.Equal(t, valueA, ans[i])
	}

}

func Test_GP_ComputeExpression_Add_Simple(t *testing.T) {
	nodes := setup_n_peers(3, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA, 0)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB, 0)
	require.NoError(t, err)

	// all node will need to run compute Expression simultaneously.
	// will change to only one node run expression
	prime := "1000000009"
	uniqID := "test"
	expression := "a+b"

	// init the information for all nodes
	for _, n := range nodes {
		err := n.InitMPC(uniqID, prime, nodeA.GetAddr(), expression)
		require.NoError(t, err)
	}

	ans := make([]int, 3)
	go func() {
		ansA, err := nodeA.ComputeExpression("test", "a+b", prime)
		ans[0] = ansA
		require.NoError(t, err)
	}()
	go func() {
		ansB, err := nodeB.ComputeExpression("test", "a+b", prime)
		ans[1] = ansB
		require.NoError(t, err)
	}()
	go func() {
		ansC, err := nodeC.ComputeExpression("test", "a+b", prime)
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
	for i := 0; i < 3; i++ {
		require.Equal(t, valueA+valueB, ans[i])
	}
}

func Test_GP_ComputeExpression_Add_Hard(t *testing.T) {
	nodes := setup_n_peers(3, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA, 0)
	require.NoError(t, err)

	valueAA := 4
	err = nodeA.SetValueDBAsset("aa", valueAA, 0)
	require.NoError(t, err)

	valueB := 8
	err = nodeB.SetValueDBAsset("b", valueB, 0)
	require.NoError(t, err)

	valueC := 9
	err = nodeC.SetValueDBAsset("c", valueC, 0)
	require.NoError(t, err)

	// all node will need to run compute Expression simultaneously.
	// will change to only one node run expression
	prime := "1000000009"
	uniqID := "test"
	expression := "a+b+aa+c+a"

	// init the information for all nodes
	for _, n := range nodes {
		err := n.InitMPC(uniqID, prime, nodeA.GetAddr(), expression)
		require.NoError(t, err)
	}

	ans := make([]int, 3)
	go func() {
		ansA, err := nodeA.ComputeExpression("test", "a+b+c+aa+a", prime)
		ans[0] = ansA
		require.NoError(t, err)
	}()
	go func() {
		ansB, err := nodeB.ComputeExpression("test", "a+b+c+aa+a", prime)
		ans[1] = ansB
		require.NoError(t, err)
	}()
	go func() {
		ansC, err := nodeC.ComputeExpression("test", "a+b+c+aa+a", prime)
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
	for i := 0; i < 3; i++ {
		require.Equal(t, valueA+valueB+valueC+valueAA+valueA, ans[i])
	}
}

func Test_GP_ComputeExpression_Mult_Simple(t *testing.T) {
	nodes := setup_n_peers(3, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA, 0)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB, 0)
	require.NoError(t, err)

	// all node will need to run compute Expression simultaneously.
	prime := "1000000009"
	uniqID := "test"
	expression := "a*b"

	// init the information for all nodes
	for _, n := range nodes {
		err := n.InitMPC(uniqID, prime, nodeA.GetAddr(), expression)
		require.NoError(t, err)
	}

	ans := make([]int, 3)
	go func() {
		ansA, err := nodeA.ComputeExpression("test", "a*b", prime)
		ans[0] = ansA
		require.NoError(t, err)
	}()
	go func() {
		ansB, err := nodeB.ComputeExpression("test", "a*b", prime)
		ans[1] = ansB
		require.NoError(t, err)
	}()
	go func() {
		ansC, err := nodeC.ComputeExpression("test", "a*b", prime)
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
	for i := 0; i < 3; i++ {
		require.Equal(t, valueA*valueB, ans[i])
	}
}

func Test_GP_ComputeExpression_Mult_Hard(t *testing.T) {
	nodes := setup_n_peers(3, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA, 0)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB, 0)
	require.NoError(t, err)

	valueC := 4
	err = nodeC.SetValueDBAsset("c", valueC, 0)
	require.NoError(t, err)

	// all node will need to run compute Expression simultaneously.
	prime := "1000000009"
	uniqID := "test"
	expression := "a*b*c"

	// init the information for all nodes
	for _, n := range nodes {
		err := n.InitMPC(uniqID, prime, nodeA.GetAddr(), expression)
		require.NoError(t, err)
	}

	ans := make([]int, 3)
	go func() {
		ansA, err := nodeA.ComputeExpression("test", "a*b*c", prime)
		ans[0] = ansA
		require.NoError(t, err)
	}()
	go func() {
		ansB, err := nodeB.ComputeExpression("test", "a*b*c", prime)
		ans[1] = ansB
		require.NoError(t, err)
	}()
	go func() {
		ansC, err := nodeC.ComputeExpression("test", "a*b*c", prime)
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
	for i := 0; i < 3; i++ {
		require.Equal(t, valueA*valueB*valueC, ans[i])
	}
}

func Test_GP_ComputeExpression_Complex_1(t *testing.T) {
	nodes := setup_n_peers(3, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA, 0)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB, 0)
	require.NoError(t, err)

	valueC := 4
	err = nodeC.SetValueDBAsset("c", valueC, 0)
	require.NoError(t, err)

	// all node will need to run compute Expression simultaneously.
	prime := "1000000009"
	uniqID := "test"
	expression := "(a+b)*(c+b)*(a+c)"

	// init the information for all nodes
	for _, n := range nodes {
		err := n.InitMPC(uniqID, prime, nodeA.GetAddr(), expression)
		require.NoError(t, err)
	}

	ans := make([]int, 3)
	go func() {
		ansA, err := nodeA.ComputeExpression("test", "(a+b)*(c+b)*(a+c)", prime)
		ans[0] = ansA
		require.NoError(t, err)
	}()
	go func() {
		ansB, err := nodeB.ComputeExpression("test", "(a+b)*(c+b)*(a+c)", prime)
		ans[1] = ansB
		require.NoError(t, err)
	}()
	go func() {
		ansC, err := nodeC.ComputeExpression("test", "(a+b)*(c+b)*(a+c)", prime)
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
	for i := 0; i < 3; i++ {
		require.Equal(t, (valueA+valueB)*(valueB+valueC)*(valueA+valueC), ans[i])
	}
}

func Test_GP_ComputeExpression_Complex_2(t *testing.T) {
	nodes := setup_n_peers(4, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	nodeD := nodes[3]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()
	defer nodeD.Stop()

	// nodeA set asset
	valueA1 := 5
	err := nodeA.SetValueDBAsset("a1", valueA1, 0)
	require.NoError(t, err)

	valueA2 := 9
	err = nodeA.SetValueDBAsset("a2", valueA2, 0)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB, 0)
	require.NoError(t, err)

	valueC := 4
	err = nodeC.SetValueDBAsset("c", valueC, 0)
	require.NoError(t, err)

	// all node will need to run compute Expression simultaneously.
	prime := "1000000009"
	uniqID := "test"
	expression := "(a1*a2 + b +c)/2"

	// init the information for all nodes
	for _, n := range nodes {
		err := n.InitMPC(uniqID, prime, nodeA.GetAddr(), expression)
		require.NoError(t, err)
	}

	ans := make([]int, 4)
	go func() {
		ansA, err := nodeA.ComputeExpression("test", expression, prime)
		ans[0] = ansA
		require.NoError(t, err)
	}()
	go func() {
		ansB, err := nodeB.ComputeExpression("test", expression, prime)
		ans[1] = ansB
		require.NoError(t, err)
	}()
	go func() {
		ansC, err := nodeC.ComputeExpression("test", expression, prime)
		ans[2] = ansC
		require.NoError(t, err)
	}()
	go func() {
		ansD, err := nodeD.ComputeExpression("test", expression, prime)
		ans[3] = ansD
		require.NoError(t, err)
	}()

	time.Sleep(time.Second * 3)

	// check all received ans is equal
	recvValue := ans[0]
	for i := 0; i < 4; i++ {
		require.Equal(t, recvValue, ans[i])
	}

	// check equal to the expected ans
	for i := 0; i < 4; i++ {
		require.Equal(t, int((valueA1*valueA2+valueB+valueC)/2), ans[i])
	}
}

func Test_GP_ComputeExpression_Multiple_Time(t *testing.T) {
	nodes := setup_n_peers(3, t)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA, 0)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB, 0)
	require.NoError(t, err)

	valueC := 4
	err = nodeC.SetValueDBAsset("c", valueC, 0)
	require.NoError(t, err)

	// all node will need to run compute Expression simultaneously.
	prime := "1000000009"
	uniqID := []string{"test1", "test2", "test3"}
	expression := []string{"a*b*c", "a+b+c", "c*c-a*b"}

	// init the information for all nodes
	for i := 0; i < len(uniqID); i++ {
		for _, n := range nodes {
			err := n.InitMPC(uniqID[i], prime, nodeA.GetAddr(), expression[i])
			require.NoError(t, err)
		}
	}

	ans := make([][]int, len(uniqID))
	for i := range ans {
		ans[i] = []int{0, 0, 0}
	}

	for i := 0; i < len(uniqID); i++ {
		ii := i
		go func() {
			ansA, err := nodeA.ComputeExpression(uniqID[ii], expression[ii], prime)
			ans[ii][0] = ansA
			require.NoError(t, err)
		}()
	}
	for i := 0; i < len(uniqID); i++ {
		ii := i
		go func() {
			ansB, err := nodeB.ComputeExpression(uniqID[ii], expression[ii], prime)
			ans[ii][1] = ansB
			require.NoError(t, err)
		}()
	}
	for i := 0; i < len(uniqID); i++ {
		ii := i
		go func() {
			ansC, err := nodeC.ComputeExpression(uniqID[ii], expression[ii], prime)
			ans[ii][2] = ansC
			require.NoError(t, err)
		}()
	}

	time.Sleep(time.Second * 5)

	// check all received ans is equal
	for i := 0; i < len(uniqID); i++ {
		recvValue := ans[i][0]
		for j := 0; j < 3; j++ {
			require.Equal(t, recvValue, ans[i][j])
		}
	}

	// check equal to the expected ans
	expected_ans := []int{valueA * valueB * valueC, valueA + valueB + valueC, valueC*valueC - valueA*valueB}
	for i := 0; i < len(uniqID); i++ {
		for j := 0; j < 3; j++ {
			require.Equal(t, expected_ans[i], ans[i][j])
		}
	}
}
