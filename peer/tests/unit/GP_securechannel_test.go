package unit

import (
	"testing"
	"time"

	"encoding/json"

	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/transport/channel"

	"go.dedis.ch/cs438/types"
)

func Test_GP_SecureChannel_Encryption_Send(t *testing.T) {
	transp := channel.NewTransport()

	fake := z.NewFakeMessage(t)
	handler1, status1 := fake.GetHandler(t)
	handler2, status2 := fake.GetHandler(t)

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithMessage(fake, handler1))
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithMessage(fake, handler2))
	defer node2.Stop()

	pubkey2 := node2.GetPubkeyStore()[node2.GetAddr()]
	node1.AddPeer(node2.GetAddr())
	node1.SetPubkeyEntry(node2.GetAddr(), &pubkey2)

	// n1 send encrypted message to n2

	err := node1.SendEncryptedMessage(fake.GetNetMsg(t), node2.GetAddr())
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 200)

	// n2 successfully decrypt the message and process it

	status1.CheckNotCalled(t)
	status2.CheckCalled(t)
}

func Test_GP_SecureChannel_Pubkey_Handler(t *testing.T) {
	transp := channel.NewTransport()

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node2.Stop()

	node1.AddPeer(node2.GetAddr())

	pubkey1 := node1.GetPubkeyStore()[node1.GetAddr()]

	pubkeyMsg := types.PubkeyMessage{
		Origin: node1.GetAddr(),
		Pubkey: pubkey1,
	}
	pubkeyMsgMarshal, err := json.Marshal(&pubkeyMsg)
	require.NoError(t, err)
	msg := transport.Message{Type: pubkeyMsg.Name(), Payload: pubkeyMsgMarshal}
	err = node1.Unicast(node2.GetAddr(), msg)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 200)

	// > node2 should have receive one message and send nothing

	n2Ins := node2.GetIns()
	n2Outs := node2.GetOuts()

	require.Len(t, n2Ins, 1)
	pkt := n2Ins[0]
	require.Equal(t, "pubkey", pkt.Msg.Type)
	pubkeyMsgReceived := z.GetPubkey(t, pkt.Msg)
	require.Equal(t, pubkeyMsg.Origin, pubkeyMsgReceived.Origin)
	require.Equal(t, pubkeyMsg.Pubkey, pubkeyMsgReceived.Pubkey)

	require.Len(t, n2Outs, 0)

	// > node2 should have node1's pubkey in its pubkey store

	store2 := node2.GetPubkeyStore()
	pub1, ok := store2[node1.GetAddr()]
	require.True(t, ok)
	require.Equal(t, pubkey1, pub1)
}

func Test_GP_SecureChannel_Pubkey_Distribution(t *testing.T) {
	getTest := func(transp transport.Transport) func(*testing.T) {
		return func(t *testing.T) {
			opts := []z.Option{
				// at least every peer will send a heartbeat message on start,
				// which will make everyone to have an entry in its routing
				// table to every one else, thanks to the antientropy.
				z.WithHeartbeat(time.Second * 200),
				z.WithAntiEntropy(time.Second * 5),
				z.WithAckTimeout(time.Second * 10),
			}

			nodeA := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", opts...)
			defer nodeA.Stop()

			nodeB := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", opts...)
			defer nodeB.Stop()

			nodeC := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", opts...)
			defer nodeC.Stop()

			pubkeyA, ok := nodeA.GetPubkeyStore()[nodeA.GetAddr()]
			require.True(t, ok)
			pubkeyB, ok := nodeB.GetPubkeyStore()[nodeB.GetAddr()]
			require.True(t, ok)
			pubkeyC, ok := nodeC.GetPubkeyStore()[nodeC.GetAddr()]
			require.True(t, ok)

			nodeA.AddPeer(nodeB.GetAddr())
			nodeB.AddPeer(nodeA.GetAddr())
			nodeB.AddPeer(nodeC.GetAddr())

			// Wait for the anti-entropy to take effect, i.e. everyone gets the
			// heartbeat message from everyone else.
			time.Sleep(time.Second * 10)

			// > nodeA should have all pubkeys

			storeA := nodeA.GetPubkeyStore()
			require.Len(t, storeA, 3)

			pubBA, ok := storeA[nodeB.GetAddr()]
			require.True(t, ok)
			require.Equal(t, pubkeyB, pubBA)

			pubCA, ok := storeA[nodeC.GetAddr()]
			require.True(t, ok)
			require.Equal(t, pubkeyC, pubCA)

			// > nodeB should have all pubkeys

			storeB := nodeB.GetPubkeyStore()
			require.Len(t, storeB, 3)

			pubAB, ok := storeB[nodeA.GetAddr()]
			require.True(t, ok)
			require.Equal(t, pubkeyA, pubAB)

			pubCB, ok := storeB[nodeC.GetAddr()]
			require.True(t, ok)
			require.Equal(t, pubkeyC, pubCB)

			// > nodeC should have all pubkeys

			storeC := nodeC.GetPubkeyStore()
			require.Len(t, storeC, 3)

			pubAC, ok := storeC[nodeA.GetAddr()]
			require.True(t, ok)
			require.Equal(t, pubkeyA, pubAC)

			pubBC, ok := storeC[nodeB.GetAddr()]
			require.True(t, ok)
			require.Equal(t, pubkeyB, pubBC)
		}
	}
	t.Run("channel transport", getTest(channelFac()))
	t.Run("UDP transport", getTest(udpFac()))
}
