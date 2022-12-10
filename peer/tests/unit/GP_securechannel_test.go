package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/transport/channel"
)

func Test_GP_Encryption_Send(t *testing.T) {
	transp := channel.NewTransport()

	fake := z.NewFakeMessage(t)
	handler1, status1 := fake.GetHandler(t)
	handler2, status2 := fake.GetHandler(t)

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithMessage(fake, handler1), z.WithHeartbeat(time.Millisecond*500))
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithMessage(fake, handler2))
	defer node2.Stop()

	node1.AddPeer(node2.GetAddr())

	time.Sleep(time.Second * 2)

	n1Ins := node1.GetIns()
	n2Ins := node2.GetIns()

	n1Outs := node1.GetOuts()
	n2Outs := node2.GetOuts()

	// > n1 should have received at least an ack message from n2

	require.Greater(t, len(n1Ins), 0)
	pkt := n1Ins[0]
	require.Equal(t, "ack", pkt.Msg.Type)

	// > n2 should have received at least a rumor from n1

	require.Greater(t, len(n2Ins), 0)

	pkt = n2Ins[0]
	require.Equal(t, node2.GetAddr(), pkt.Header.Destination)
	require.Equal(t, node1.GetAddr(), pkt.Header.RelayedBy)
	require.Equal(t, node1.GetAddr(), pkt.Header.Source)

	rumor := z.GetRumor(t, pkt.Msg)
	require.Len(t, rumor.Rumors, 1)
	pubkey1 := z.GetPubkey(t, rumor.Rumors[0].Msg)
	require.Equal(t, pubkey1.Origin, node1.GetAddr())

	// > n1 should have sent at least 1 packet to n2

	require.Greater(t, len(n1Outs), 0)

	pkt = n1Outs[0]
	require.Equal(t, node2.GetAddr(), pkt.Header.Destination)
	require.Equal(t, node1.GetAddr(), pkt.Header.RelayedBy)
	require.Equal(t, node1.GetAddr(), pkt.Header.Source)

	require.Equal(t, "rumor", pkt.Msg.Type)

	rumor = z.GetRumor(t, pkt.Msg)
	require.Len(t, rumor.Rumors, 1)
	pubkey2 := z.GetPubkey(t, rumor.Rumors[0].Msg)
	require.Equal(t, pubkey2.Origin, node1.GetAddr())

	// > n2 should have sent at least one ack packet

	require.Greater(t, len(n2Outs), 0)
	pkt = n2Outs[0]
	require.Equal(t, "ack", pkt.Msg.Type)

	// n1 send encrypted message to n2

	err := node2.SendEncryptedMessage(fake.GetNetMsg(t), node2.GetAddr())
	require.NoError(t, err)

	time.Sleep(time.Second * 2)

	// TODO: check why status1 fails
	status1.CheckCalled(t)
	status2.CheckCalled(t)
}
