package unit

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/permissioned-chain"
	"go.dedis.ch/cs438/transport/channel"
)

func Test_GP_BC_Init(t *testing.T) {
	transp := channel.NewTransport()

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node2.Stop()

	node1.AddPeer(node2.GetAddr())

	// generate key pairs

	privkey1, err := crypto.GenerateKey()
	require.NoError(t, err)
	node1.BCSetKeyPair(*privkey1)
	addr1, err := node1.BCGetAddress()
	require.NoError(t, err)

	privkey2, err := crypto.GenerateKey()
	require.NoError(t, err)
	node2.BCSetKeyPair(*privkey2)
	addr2, err := node2.BCGetAddress()
	require.NoError(t, err)

	// > init blockchain on node1. Should success

	config := permissioned.NewChainConfig(
		map[string]struct{}{
			addr1.Hex: {},
			addr2.Hex: {},
		},
		10, "2h", 1,
	)
	require.Len(t, config.Participants, 2)
	err = node1.InitBlockchain(*config, nil)
	require.NoError(t, err)

	time.Sleep(time.Second * 2)

	// > node1 sent out the correct block message

	node1Outs := node1.GetOuts()
	rumor := z.GetRumor(t, node1Outs[0].Msg)

	private := z.GetBCPrivate(t, rumor.Rumors[0].Msg)
	require.Len(t, private.Recipients, 2)
	require.Contains(t, private.Recipients, addr1.Hex)
	require.Contains(t, private.Recipients, addr2.Hex)

	blkMsg1 := z.GetBCBlk(t, private.Msg)
	require.Equal(t, node1.GetAddr(), blkMsg1.Origin)
	require.Equal(t, uint(0), blkMsg1.BlkHeader.Height)
	require.Equal(t, permissioned.ZeroAddress.Hex, blkMsg1.BlkHeader.Miner)
	require.Equal(t, permissioned.DUMMY_PREVHASH, blkMsg1.BlkHeader.PrevHash)

	// node2 got the correct block message

	node2Ins := node2.GetIns()
	rumor = z.GetRumor(t, node2Ins[0].Msg)

	private = z.GetBCPrivate(t, rumor.Rumors[0].Msg)
	require.Len(t, private.Recipients, 2)
	require.Contains(t, private.Recipients, addr1.Hex)
	require.Contains(t, private.Recipients, addr2.Hex)

	blkMsg2 := z.GetBCBlk(t, private.Msg)
	require.Equal(t, node1.GetAddr(), blkMsg2.Origin)
	require.Equal(t, blkMsg1.BlkHeader.Hash(), blkMsg2.BlkHeader.Hash())

	// both nodes have its genesis block
	blk := node1.BCGetLatestBlock()
	require.NotNil(t, blk)
	require.Equal(t, blkMsg2.BlkHeader.Hash(), blk.Hash())

	blk = node2.BCGetLatestBlock()
	require.NotNil(t, blk)
	require.Equal(t, blkMsg2.BlkHeader.Hash(), blk.Hash())
}

func Test_GP_BC_Mine_Block(t *testing.T) {
	transp := channel.NewTransport()

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node1.Stop()

	sock2, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)
	defer sock2.Close()

	node1.AddPeer(sock2.GetAddress())

	go func() {
		for {
			sock2.Recv(time.Second * 3)
		}
	}()

	// generate key pairs

	privkey1, err := crypto.GenerateKey()
	require.NoError(t, err)
	node1.BCSetKeyPair(*privkey1)
	addr1, err := node1.BCGetAddress()
	require.NoError(t, err)
	addrTest := permissioned.NewAddress(&privkey1.PublicKey)
	require.Equal(t, addrTest.Hex, addr1.Hex)
	account1 := permissioned.NewAccount(addr1)

	privkey2, err := crypto.GenerateKey()
	require.NoError(t, err)
	addr2 := permissioned.NewAddress(&privkey2.PublicKey)

	// > init blockchain on node1. Should success

	config := permissioned.NewChainConfig(
		map[string]struct{}{
			addr1.Hex: {},
			addr2.Hex: {},
		},
		1, "2h", 1,
	)
	initialGain := map[string]float64{
		addr1.Hex: 10000,
	}
	require.Len(t, config.Participants, 2)
	err = node1.InitBlockchain(*config, initialGain)
	require.NoError(t, err)

	time.Sleep(time.Second * 2)

	// > node1 has genesis block

	block0 := node1.BCGetLatestBlock()
	require.NotNil(t, block0)

	// > send Tx to node1. A new block need to be mined

	txn1 := permissioned.NewTransactionPreMPC(account1,
		permissioned.MPCRecord{
			Initiator:  account1.GetAddress().Hex,
			Budget:     10,
			Expression: "a",
		})
	require.Equal(t, addr1.Hex, txn1.From)
	signedTxn, err := txn1.Sign(privkey1)
	require.NoError(t, err)

	worldState := block0.States.Copy()
	err = signedTxn.Verify(worldState, config)
	require.NoError(t, err)

	err = node1.BCSendTransaction(signedTxn)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 200)

	// > node1 needs to send a tx msg

	node1Outs := node1.GetOuts()
	require.GreaterOrEqual(t, len(node1Outs), 2)
	require.Equal(t, sock2.GetAddress(), node1Outs[1].Header.Destination)
	rumor := z.GetRumor(t, node1Outs[1].Msg)

	private := z.GetBCPrivate(t, rumor.Rumors[0].Msg)
	require.Len(t, private.Recipients, 2)
	require.Contains(t, private.Recipients, addr1.Hex)
	require.Contains(t, private.Recipients, addr2.Hex)

	txnMsg1 := z.GetBCTxn(t, private.Msg)
	require.Equal(t, node1.GetAddr(), txnMsg1.Origin)
	require.Equal(t, txnMsg1.Txn.Txn.Type, permissioned.TxnTypePreMPC)

	// sock2 got the correct txn message

	node2Ins := sock2.GetIns()
	require.GreaterOrEqual(t, len(node2Ins), 2)
	rumor = z.GetRumor(t, node2Ins[1].Msg)

	private = z.GetBCPrivate(t, rumor.Rumors[0].Msg)
	require.Len(t, private.Recipients, 2)
	require.Contains(t, private.Recipients, addr1.Hex)
	require.Contains(t, private.Recipients, addr2.Hex)

	txnMsg2 := z.GetBCTxn(t, private.Msg)
	require.Equal(t, node1.GetAddr(), txnMsg2.Origin)
	require.Equal(t, txnMsg1.Txn.Txn.Type, permissioned.TxnTypePreMPC)

	time.Sleep(time.Second * 1)

	// > node1 should send out a new block

	node1Outs = node1.GetOuts()
	require.GreaterOrEqual(t, len(node1Outs), 3)
	rumor = z.GetRumor(t, node1Outs[2].Msg)

	private = z.GetBCPrivate(t, rumor.Rumors[0].Msg)
	require.Len(t, private.Recipients, 2)
	require.Contains(t, private.Recipients, addr1.Hex)
	require.Contains(t, private.Recipients, addr2.Hex)

	blkMsg1 := z.GetBCBlk(t, private.Msg)
	require.Equal(t, node1.GetAddr(), blkMsg1.Origin)
	require.Equal(t, uint(1), blkMsg1.BlkHeader.Height)
	require.Equal(t, addr1.Hex, blkMsg1.BlkHeader.Miner)
	require.Equal(t, block0.Hash(), blkMsg1.BlkHeader.PrevHash)

	// > node1 should append the new block

	block1 := node1.BCGetLatestBlock()
	require.NotNil(t, block1)
	require.Equal(t, blkMsg1.BlkHeader.Hash(), block1.Hash())
}
