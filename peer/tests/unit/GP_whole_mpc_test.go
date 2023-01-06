package unit

import (
	"crypto/rsa"
	"crypto/x509"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/permissioned-chain"
	"go.dedis.ch/cs438/transport/channel"
	"go.dedis.ch/cs438/types"
)

func Test_GP_ComputeExpression_Paxos_Add(t *testing.T) {
	/* Switch to Blockchain. No need to test on paxos */

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

	// call Calculate on nodeA. The MPC starts automatically
	mpcDone := make(chan struct{})
	var recvValue int
	go func() {
		ans, err := nodeA.Calculate("a+b", 10)
		recvValue = ans
		require.NoError(t, err)

		close(mpcDone)
	}()

	timeout := time.After(time.Second * 2)

	select {
	case <-mpcDone:
	case <-timeout:
		t.Error(t, "a result must have been computed")
	}

	// check equal to the expected ans
	require.Equal(t, valueA+valueB, recvValue)
}

func setup_n_peers_bc(t *testing.T, n int, maxTxn int,
	timeout string, gains []float64) []*z.TestNode {
	nodes := make([]*z.TestNode, n)

	transp := channel.NewTransport()

	for i := 0; i < n; i++ {
		node := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
		nodes[i] = &node
	}

	// generate key pairs
	addrs := make([]string, n)
	for i := 0; i < n; i++ {
		privkey1, err := crypto.GenerateKey()
		require.NoError(t, err)
		nodes[i].BCSetKeyPair(*privkey1)
		addr, err := nodes[i].BCGetAddress()
		require.NoError(t, err)
		addrs[i] = addr.Hex
	}

	// get encryption pubkeys
	pubkeys := make([]types.Pubkey, n)
	for i := 0; i < n; i++ {
		pubkeys[i] = nodes[i].GetPubkeyStore()[nodes[i].GetAddr()]
	}

	// add peer
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			nodes[i].AddPeer(nodes[j].GetAddr())
			// nodes[i].SetPubkeyEntry(nodes[j].GetAddr(), &pubkeys[j])
		}
	}

	// > init blockchain. Should success
	// all should have the block
	participants := make(map[string][]byte)
	for i, addr := range addrs {
		pubBytes, err := x509.MarshalPKIXPublicKey((*rsa.PublicKey)(&pubkeys[i]))
		require.NoError(t, err)
		participants[addr] = pubBytes
	}

	config := permissioned.NewChainConfig(
		participants,
		maxTxn, timeout, 1,
	)
	initialGain := make(map[string]float64)
	for i, gain := range gains {
		initialGain[addrs[i]] = gain
	}

	err := nodes[0].InitBlockchain(*config, initialGain)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 500)

	for _, node := range nodes {
		block0 := node.BCGetLatestBlock()
		require.NotNil(t, block0)
		require.Equal(t, uint(0), block0.Height)
	}

	return nodes
}

func Test_GP_ComputeExpression_Blockchain_Add(t *testing.T) {
	nodes := setup_n_peers_bc(t, 3, 1, "2h", []float64{100, 100, 100})
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[1]
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

	// call Calculate on nodeA. The MPC starts automatically
	mpcDone := make(chan struct{})
	var recvValue int
	go func() {
		ans, err := nodeA.Calculate("a+b", 10)
		recvValue = ans
		require.NoError(t, err)

		close(mpcDone)
	}()

	timeout := time.After(time.Second * 2)

	select {
	case <-mpcDone:
	case <-timeout:
		t.Error(t, "a result must have been computed")
	}

	// check equal to the expected ans
	require.Equal(t, valueA+valueB, recvValue)
}
