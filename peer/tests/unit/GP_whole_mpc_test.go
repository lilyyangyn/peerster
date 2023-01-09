package unit

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/permissioned-chain"
	"go.dedis.ch/cs438/transport/channel"
	"go.dedis.ch/cs438/types"
)

// -----------------------------------------------------------------------------
// Paxos MPC

func Test_GP_MPC_Paxos_Add(t *testing.T) {
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

// -----------------------------------------------------------------------------
// Blockchain MPC

func Test_GP_MPC_Pure_BC_Single(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	nodes, addrs := setup_n_peers_bc(t, 3, 3, "2s", []float64{100}, true, true)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	mpcDone := make(chan struct{})
	go func() {
		err := nodeA.SetValueDBAsset("a", 1, 1)
		require.NoError(t, err)
		err = nodeB.SetValueDBAsset("b", 1, 6)
		require.NoError(t, err)

		time.Sleep(time.Millisecond * 200)

		_, err = nodeA.Calculate("a+b", 10)
		require.NoError(t, err)

		close(mpcDone)
	}()

	timeout := time.After(time.Second * 3)

	select {
	case <-mpcDone:
	case <-timeout:
		t.Error(t, "calculation must finish")
	}

	time.Sleep(time.Second * 1)

	// > verify all nodes got two blocks
	block2a := nodeA.BCGetLatestBlock()
	require.NotNil(t, block2a)
	require.Equal(t, uint(2), block2a.Height)
	block1a := nodeA.BCGetBlock(block2a.PrevHash)
	require.NotNil(t, block1a)
	require.Equal(t, uint(1), block1a.Height)

	block2b := nodeB.BCGetLatestBlock()
	require.NotNil(t, block2b)
	require.Equal(t, uint(2), block2b.Height)
	block1b := nodeB.BCGetBlock(block2b.PrevHash)
	require.NotNil(t, block1b)
	require.Equal(t, uint(1), block1b.Height)

	block2c := nodeC.BCGetLatestBlock()
	require.NotNil(t, block2c)
	require.Equal(t, uint(2), block2c.Height)
	block1c := nodeC.BCGetBlock(block2c.PrevHash)
	require.NotNil(t, block1c)
	require.Equal(t, uint(1), block1c.Height)

	// > verify blockchain are the same

	require.Equal(t, block2a.Hash(), block2b.Hash())
	require.Equal(t, block2a.Hash(), block2c.Hash())

	// > verify balance are correct at last
	worldstate := block2a.GetWorldStateCopy()
	accountA := permissioned.GetAccountFromWorldState(worldstate, addrs[0])
	require.Equal(t, float64(92), accountA.GetBalance())
	accountB := permissioned.GetAccountFromWorldState(worldstate, addrs[1])
	require.Equal(t, float64(7), accountB.GetBalance())
	accountC := permissioned.GetAccountFromWorldState(worldstate, addrs[2])
	require.Equal(t, float64(1), accountC.GetBalance())

	// > verify balance are correct before MPC
	worldstate = block1a.GetWorldStateCopy()
	accountA = permissioned.GetAccountFromWorldState(worldstate, addrs[0])
	require.Equal(t, float64(90), accountA.GetBalance())
	accountB = permissioned.GetAccountFromWorldState(worldstate, addrs[1])
	require.Equal(t, float64(0), accountB.GetBalance())
	accountC = permissioned.GetAccountFromWorldState(worldstate, addrs[2])
	require.Equal(t, float64(0), accountC.GetBalance())
}

func Test_GP_MPC_Pure_BC_Multiple(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	nodes, addrs := setup_n_peers_bc(t, 3, 3, "2h", []float64{100}, true, true)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	err := nodeA.SetValueDBAsset("a", 1, 1)
	require.NoError(t, err)
	err = nodeB.SetValueDBAsset("b", 1, 2)
	require.NoError(t, err)
	err = nodeC.SetValueDBAsset("c", 1, 3)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	block1 := nodeA.BCGetLatestBlock()
	require.NotNil(t, block1)
	require.Equal(t, uint(1), block1.Height)

	mpcDone := make(chan struct{})
	mpcCount := make(chan struct{})
	go func() {
		count := 0
		for {
			<-mpcCount
			count++
			if count == 3 {
				close(mpcDone)
			}
		}
	}()

	go func() {
		// A: + 1 + 1 = 2
		// B: + 1 + 2 = 3
		// C: + 1     = 1
		_, err := nodeA.Calculate("a+b", 10)
		require.NoError(t, err)

		mpcCount <- struct{}{}
	}()
	go func() {
		// A: + 1 + 1 = 2
		// B: + 1     = 1
		// C: + 1 + 3 = 4
		_, err := nodeA.Calculate("a+c", 10)
		require.NoError(t, err)

		mpcCount <- struct{}{}
	}()
	go func() {
		// A: + 1     = 1
		// B: + 1 + 2 = 3
		// C: + 1 + 3 = 4
		_, err := nodeA.Calculate("b+c", 10)
		require.NoError(t, err)

		mpcCount <- struct{}{}
	}()

	timeout := time.After(time.Second * 3)

	select {
	case <-mpcDone:
	case <-timeout:
		t.Error(t, "calculation must finish")
	}

	time.Sleep(time.Second * 1)

	// fmt.Println(nodeA.BCSprintBlockchain())

	// > verify all nodes got four blocks
	blockA := nodeA.BCGetLatestBlock()
	require.NotNil(t, blockA)
	require.Equal(t, uint(5), blockA.Height)

	blockB := nodeB.BCGetLatestBlock()
	require.NotNil(t, blockB)
	require.Equal(t, uint(5), blockB.Height)

	blockC := nodeC.BCGetLatestBlock()
	require.NotNil(t, blockC)
	require.Equal(t, uint(5), blockC.Height)

	// > verify blockchain are the same

	require.Equal(t, blockA.Hash(), blockB.Hash())
	require.Equal(t, blockA.Hash(), blockC.Hash())

	// > verify balance are correct at last
	worldstate := blockA.GetWorldStateCopy()
	accountA := permissioned.GetAccountFromWorldState(worldstate, addrs[0])
	require.Equal(t, float64(84), accountA.GetBalance())
	accountB := permissioned.GetAccountFromWorldState(worldstate, addrs[1])
	require.Equal(t, float64(7), accountB.GetBalance())
	accountC := permissioned.GetAccountFromWorldState(worldstate, addrs[2])
	require.Equal(t, float64(9), accountC.GetBalance())
}

func Test_GP_MPC_Pure_BC_Double_Spend(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	nodes, addrs := setup_n_peers_bc(t, 3, 3, "2s", []float64{7}, true, true)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	err := nodeA.SetValueDBAsset("a", 1, 1)
	require.NoError(t, err)
	err = nodeB.SetValueDBAsset("b", 1, 2)
	require.NoError(t, err)
	err = nodeC.SetValueDBAsset("c", 1, 3)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	block1 := nodeA.BCGetLatestBlock()
	require.NotNil(t, block1)
	require.Equal(t, uint(1), block1.Height)

	mpcDone := make(chan struct{})

	go func() {
		// > first one should success
		// A : + 1 + 1 = 2
		// B : + 1 + 2 = 3
		// C : + 1     = 1
		_, err := nodeA.Calculate("a+b", 6)
		require.NoError(t, err)

		// > Second one should fail
		// A : + 1 + 1 = 2
		// B : + 1     = 1
		// C : + 1 + 3 = 4
		_, err = nodeA.Calculate("a+c", 7)
		require.Error(t, err)

		mpcDone <- struct{}{}
	}()

	timeout := time.After(time.Second * 9)

	select {
	case <-mpcDone:
	case <-timeout:
		t.Error(t, "calculation must finish")
	}

	time.Sleep(time.Second * 1)

	// fmt.Println(nodeA.BCSprintBlockchain())

	// > verify all nodes got four blocks
	blockA := nodeA.BCGetLatestBlock()
	require.NotNil(t, blockA)
	require.Equal(t, uint(3), blockA.Height)

	blockB := nodeB.BCGetLatestBlock()
	require.NotNil(t, blockB)
	require.Equal(t, uint(3), blockB.Height)

	blockC := nodeC.BCGetLatestBlock()
	require.NotNil(t, blockC)
	require.Equal(t, uint(3), blockC.Height)

	// > verify blockchain are the same

	require.Equal(t, blockA.Hash(), blockB.Hash())
	require.Equal(t, blockA.Hash(), blockC.Hash())

	// > verify balance are correct at last
	worldstate := blockA.GetWorldStateCopy()
	accountA := permissioned.GetAccountFromWorldState(worldstate, addrs[0])
	require.Equal(t, float64(3), accountA.GetBalance())
	accountB := permissioned.GetAccountFromWorldState(worldstate, addrs[1])
	require.Equal(t, float64(3), accountB.GetBalance())
	accountC := permissioned.GetAccountFromWorldState(worldstate, addrs[2])
	require.Equal(t, float64(1), accountC.GetBalance())

}

func Test_GP_MPC_BC_ADD_Simple(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	nodes, _ := setup_n_peers_bc(t, 3, 1, "2s", []float64{100}, false, true)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA, 1)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB, 2)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 200)

	// call Calculate on nodeA. The MPC starts automatically
	mpcDone := make(chan struct{})
	var recvValue int
	go func() {
		// A : + 1 + 1 = 2
		// B : + 1 + 2 = 3
		// C : + 1     = 1
		ans, err := nodeA.Calculate("a+b", 10)
		recvValue = ans
		require.NoError(t, err)

		close(mpcDone)
	}()

	timeout := time.After(time.Second * 5)

	select {
	case <-mpcDone:
	case <-timeout:
		t.Error(t, "a result must have been computed")
	}

	// check equal to the expected ans
	require.Equal(t, valueA+valueB, recvValue)
}

func Test_GP_MPC_BC_MULT_Simple(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	nodes, _ := setup_n_peers_bc(t, 3, 1, "2s", []float64{100}, false, true)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	// nodeA set asset
	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA, 1)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB, 2)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 200)

	// call Calculate on nodeA. The MPC starts automatically
	mpcDone := make(chan struct{})
	var recvValue int
	go func() {
		// A : + 1 + 1 = 2
		// B : + 1 + 2 = 3
		// C : + 1     = 1
		ans, err := nodeA.Calculate("a*b", 10)
		recvValue = ans
		require.NoError(t, err)

		close(mpcDone)
	}()

	timeout := time.After(time.Second * 10)

	select {
	case <-mpcDone:
	case <-timeout:
		t.Error(t, "a result must have been computed")
	}

	// check equal to the expected ans
	require.Equal(t, valueA*valueB, recvValue)
}

func Test_GP_MPC_BC_COMPLEX(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	nodes, _ := setup_n_peers_bc(t, 3, 1, "2s", []float64{0, 0, 100}, false, true)
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

	valueB1 := 3
	err = nodeB.SetValueDBAsset("b1", valueB1, 1)
	require.NoError(t, err)

	valueB2 := 4
	err = nodeB.SetValueDBAsset("b2", valueB2, 1)
	require.NoError(t, err)

	valueC := 4
	err = nodeC.SetValueDBAsset("c", valueC, 1)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 200)

	// call Calculate on nodeA. The MPC starts automatically
	mpcDone := make(chan struct{})
	var recvValue int
	go func() {
		fmt.Println(nodeC.GetAddr())
		ans, err := nodeC.Calculate("a*b1 + b2*c*b2 + a+c", 10)
		recvValue = ans
		// fmt.Println(nodeC.BCSprintBlockchain())
		require.NoError(t, err)

		close(mpcDone)
	}()

	timeout := time.After(time.Second * 5)

	select {
	case <-mpcDone:
	case <-timeout:
		t.Error(t, "a result must have been computed")
	}

	// check equal to the expected ans
	require.Equal(t, valueA*valueB1+valueB2*valueC*valueB2+valueA+valueC, recvValue)
}

func Test_GP_MPC_BC_Multiple(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	nodes, addrs := setup_n_peers_bc(t, 3, 1, "5h", []float64{200}, false, true)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA, 0)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB, 0)
	require.NoError(t, err)

	valueC := 4
	err = nodeC.SetValueDBAsset("c", valueC, 0)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 200)

	mpcDone := make(chan struct{})
	mpcCount := make(chan struct{})
	go func() {
		count := 0
		for {
			<-mpcCount
			count++
			if count == 3 {
				close(mpcDone)
			}
		}
	}()

	go func() {
		_, err := nodeA.Calculate("a+b", 10)
		require.NoError(t, err)

		mpcCount <- struct{}{}
	}()

	go func() {
		_, err := nodeA.Calculate("a+c", 10)
		require.NoError(t, err)

		mpcCount <- struct{}{}
	}()

	go func() {
		_, err := nodeA.Calculate("b+c", 10)
		require.NoError(t, err)

		mpcCount <- struct{}{}
	}()

	timeout := time.After(time.Second * 3)

	select {
	case <-mpcDone:
	case <-timeout:
		t.Error(t, "calculation must finish")
	}

	time.Sleep(time.Second * 5)

	// fmt.Println(nodeA.BCSprintBlockchain())

	// > verify all nodes got four blocks
	blockA := nodeA.BCGetLatestBlock()
	require.NotNil(t, blockA)
	require.Equal(t, uint(15), blockA.Height)

	blockB := nodeB.BCGetLatestBlock()
	require.NotNil(t, blockB)
	require.Equal(t, uint(15), blockB.Height)

	blockC := nodeC.BCGetLatestBlock()
	require.NotNil(t, blockC)
	require.Equal(t, uint(15), blockC.Height)

	// > verify blockchain are the same

	require.Equal(t, blockA.Hash(), blockB.Hash())
	require.Equal(t, blockA.Hash(), blockC.Hash())

	// > verify balance are correct at last
	worldstate := blockA.GetWorldStateCopy()
	accountA := permissioned.GetAccountFromWorldState(worldstate, addrs[0])
	require.Equal(t, float64(194), accountA.GetBalance())
	accountB := permissioned.GetAccountFromWorldState(worldstate, addrs[1])
	require.Equal(t, float64(3), accountB.GetBalance())
	accountC := permissioned.GetAccountFromWorldState(worldstate, addrs[2])
	require.Equal(t, float64(3), accountC.GetBalance())
}

func Test_GP_MPC_BC_MULT_Simple_With_Pubkey_Txn(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	nodes, _ := setup_n_peers_bc(t, 3, 1, "2s", []float64{100}, false, true)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA, 0)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB, 0)
	require.NoError(t, err)

	// wait for the public key to be distributed
	time.Sleep(time.Millisecond * 500)

	require.True(t, nodeB.BCAllEncryptKeySet())

	// call Calculate on nodeA. The MPC starts automatically
	mpcDone := make(chan struct{})
	var recvValue int
	go func() {
		ans, err := nodeA.Calculate("a*b", 10)
		recvValue = ans
		require.NoError(t, err)

		close(mpcDone)
	}()

	timeout := time.After(time.Second * 10)

	select {
	case <-mpcDone:
	case <-timeout:
		t.Error(t, "a result must have been computed")
	}

	// check equal to the expected ans
	require.Equal(t, valueA*valueB, recvValue)
}
func Test_GP_MPC_BC_Stress_Multiple(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	nodes, addrs := setup_n_peers_bc(t, 3, 3, "5s", []float64{200}, false, true)
	nodeA := nodes[0]
	nodeB := nodes[1]
	nodeC := nodes[2]
	defer nodeA.Stop()
	defer nodeB.Stop()
	defer nodeC.Stop()

	valueA := 5
	err := nodeA.SetValueDBAsset("a", valueA, 0)
	require.NoError(t, err)

	valueB := 3
	err = nodeB.SetValueDBAsset("b", valueB, 0)
	require.NoError(t, err)

	valueC := 4
	err = nodeC.SetValueDBAsset("c", valueC, 0)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 200)

	mpcDone := make(chan struct{})
	mpcCount := make(chan struct{})
	go func() {
		count := 0
		for {
			<-mpcCount
			count++
			if count == 3 {
				close(mpcDone)
			}
		}
	}()

	go func() {
		_, err := nodeA.Calculate("a+b", 10)
		require.NoError(t, err)

		mpcCount <- struct{}{}
	}()
	go func() {
		_, err := nodeA.Calculate("a+c", 10)
		require.NoError(t, err)

		mpcCount <- struct{}{}
	}()
	go func() {
		_, err := nodeA.Calculate("b+c", 10)
		require.NoError(t, err)

		mpcCount <- struct{}{}
	}()

	timeout := time.After(time.Second * 3)

	select {
	case <-mpcDone:
	case <-timeout:
		t.Error(t, "calculation must finish")
	}

	time.Sleep(time.Second * 7)

	fmt.Println(nodeA.BCSprintBlockchain())

	// > verify all nodes got four blocks
	blockA := nodeA.BCGetLatestBlock()
	require.NotNil(t, blockA)
	require.Equal(t, uint(5), blockA.Height)

	blockB := nodeB.BCGetLatestBlock()
	require.NotNil(t, blockB)
	require.Equal(t, uint(5), blockB.Height)

	blockC := nodeC.BCGetLatestBlock()
	require.NotNil(t, blockC)
	require.Equal(t, uint(5), blockC.Height)

	// > verify blockchain are the same

	require.Equal(t, blockA.Hash(), blockB.Hash())
	require.Equal(t, blockA.Hash(), blockC.Hash())

	// > verify balance are correct at last
	worldstate := blockA.GetWorldStateCopy()
	accountA := permissioned.GetAccountFromWorldState(worldstate, addrs[0])
	require.Equal(t, float64(194), accountA.GetBalance())
	accountB := permissioned.GetAccountFromWorldState(worldstate, addrs[1])
	require.Equal(t, float64(3), accountB.GetBalance())
	accountC := permissioned.GetAccountFromWorldState(worldstate, addrs[2])
	require.Equal(t, float64(3), accountC.GetBalance())
}

// -----------------------------------------------------------------------------
// Helper

func setup_n_peers_bc(t *testing.T, n int, maxTxn int,
	timeout string, gains []float64, disableMPC bool, disablePubkeyTxn bool) ([]*z.TestNode, []string) {
	nodes := make([]*z.TestNode, n)

	transp := channel.NewTransport()

	opt := []z.Option{
		z.WithMPCMaxWaitBlock(1),
	}

	if disableMPC {
		opt = append(opt, z.WithDisableMPC())
	}
	if disablePubkeyTxn {
		opt = append(opt, z.WithDisableAnnonceEnckey())
	}
	for i := 0; i < n; i++ {
		node := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0",
			opt...)
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
		fmt.Printf("-----%s : %s--------\n", nodes[i].GetAddr(), addr)
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
	participants := make(map[string]string)
	if disablePubkeyTxn {
		for i, addr := range addrs {
			pubBytes, err := x509.MarshalPKIXPublicKey((*rsa.PublicKey)(&pubkeys[i]))
			require.NoError(t, err)
			participants[addr] = hex.EncodeToString(pubBytes)
		}
	} else {
		for _, addr := range addrs {
			participants[addr] = ""
		}
	}

	config := permissioned.NewChainConfig(
		participants,
		maxTxn, timeout, 1, 1,
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

	return nodes, addrs
}
