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
