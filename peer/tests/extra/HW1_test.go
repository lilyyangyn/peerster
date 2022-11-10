package extra

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cs438/registry/standard"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"

	z "go.dedis.ch/cs438/internal/testing"
)

// E:1-1
//
// Checks behavior with different, most of the time unexpected, sequences on
// rumors.
func FuzzHW1RumorSequence(f *testing.F) {
	f.Add(uint(0))
	f.Add(uint(1))
	f.Add(uint(2))
	f.Add(uint(10))
	f.Add(uint(1000))

	f.Fuzz(sendRumorScenario)
}

// E:1-2
//
// Checks behavior with different unexpected packetID.
func FuzzHW1AckPacketID(f *testing.F) {
	f.Add("")
	f.Add("abc")
	f.Add("Hello, 世界")

	f.Fuzz(sendAckScenario)
}

// E:1-3
//
// Sends N garbage messages to a node.
func Test_HW1_Multiple_Messages(t *testing.T) {
	n := 1000
	rand.Seed(0)

	transp := channelFac()

	fake := z.NewFakeMessage(t)
	handler, status := fake.GetHandler(t)

	sender, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	defer sender.Close()

	receiver := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithMessage(fake, handler), z.WithContinueMongering(0))
	defer receiver.Stop()

	src := sender.GetAddress()
	dst := receiver.GetAddr()

	err = spamNode(n, src, dst, sender)
	require.NoError(t, err)

	status.CheckNotCalled(t)
}

// E:1-4
//
// Spam each node with N garbage messages, and then checks that nodes are still
// able to broadcast.
func Test_HW1_Multiple_Messages_Two_Nodes(t *testing.T) {
	n := 1000
	rand.Seed(0)

	transp := channelFac()

	fake := z.NewFakeMessage(t)
	handler1, status1 := fake.GetHandler(t)
	handler2, status2 := fake.GetHandler(t)

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithMessage(fake, handler1), z.WithContinueMongering(0))
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithMessage(fake, handler2), z.WithContinueMongering(0))
	defer node2.Stop()

	sender, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	defer sender.Close()

	err = spamNode(n, node1.GetAddr(), node2.GetAddr(), sender)
	require.NoError(t, err)

	err = spamNode(n, node2.GetAddr(), node1.GetAddr(), sender)
	require.NoError(t, err)

	status1.CheckNotCalled(t)
	status2.CheckNotCalled(t)

	// > nodes should still work and be able to exchange messages

	err = node1.Broadcast(fake.GetNetMsg(t))
	require.NoError(t, err)

	err = node2.Broadcast(fake.GetNetMsg(t))
	require.NoError(t, err)

	time.Sleep(time.Second * 2)

	status1.CheckCalled(t)
	status2.CheckCalled(t)
}

// E:1-5
//
// Flood a node with n messages and wait on rumors to be processed. Rumors are
// sent with expected sequences, so they should be processed. From the root
// folder, you can run the benchmark as follow:
//
//	GLOG=no go test --bench BenchmarkSpamNode -benchtime 1000x -benchmem \
//		-count 5 ./peer/tests/extra
func BenchmarkSpamNode(b *testing.B) {

	// Disable outputs to not penalize implementations that make use of it
	oldStdout := os.Stdout
	os.Stdout = nil

	defer func() {
		os.Stdout = oldStdout
	}()

	n := b.N
	transp := channelFac()

	fake := z.NewFakeMessage(b)

	notifications := make(chan struct{}, n)

	handler := func(types.Message, transport.Packet) error {
		notifications <- struct{}{}
		return nil
	}

	sender, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(b, err)

	defer sender.Close()

	receiver := z.NewTestNode(b, peerFac, transp, "127.0.0.1:0", z.WithMessage(fake, handler), z.WithContinueMongering(0))
	defer receiver.Stop()

	src := sender.GetAddress()
	dst := receiver.GetAddr()

	currentRumorSeq := 0
	acketPackedID := make([]byte, 12)

	_, err = rand.Read(acketPackedID)
	require.NoError(b, err)

	for i := 0; i < n; i++ {
		// flip a coin to send either a rumor or an ack message
		coin := rand.Float64() > 0.5

		if coin {
			currentRumorSeq++

			err = sendRumor(fake, uint(currentRumorSeq), src, dst, sender)
			require.NoError(b, err)
		} else {
			err = sendAck(string(acketPackedID), src, dst, sender)
			require.NoError(b, err)
		}
	}

	// > wait on all the rumors to be processed

	for i := 0; i < currentRumorSeq; i++ {
		select {
		case <-notifications:
		case <-time.After(time.Second):
			b.Error("notification not received in time")
		}
	}
}

// sendRumorScenario sends a rumor to a recipient with a given sequence number.
// The embedded rumor message must be executed only when the sequence is 1.
func sendRumorScenario(t *testing.T, sequence uint) {
	transp := channelFac()

	sender, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	defer sender.Close()

	fake := z.NewFakeMessage(t)
	handler, status := fake.GetHandler(t)

	receiver := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithContinueMongering(0), z.WithMessage(fake, handler))
	defer receiver.Stop()

	src := sender.GetAddress()
	dst := receiver.GetAddr()

	err = sendRumor(fake, sequence, src, dst, sender)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 5)

	// > the only valid sequence is 1
	if sequence == 1 {
		status.CheckCalled(t)
	} else {
		status.CheckNotCalled(t)
	}
}

// sendAckScenario sends an ack message with a possibly wrong ackedPacketID.
func sendAckScenario(t *testing.T, ackedPacketID string) {
	transp := channelFac()

	sender, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	defer sender.Close()

	receiver := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithContinueMongering(0))
	defer receiver.Stop()

	src := sender.GetAddress()
	dst := receiver.GetAddr()

	err = sendAck(ackedPacketID, src, dst, sender)
	require.NoError(t, err)
}

func spamNode(n int, src, dst string, sender transport.Socket) error {
	chat := types.ChatMessage{
		Message: "Hello fellow peerster 👋",
	}

	for i := 0; i < n; i++ {
		// flip a coin to send either a rumor or an ack message
		coin := rand.Float64() > 0.5

		if coin {
			seq := rand.Uint64()

			// ensure the sequence is always an unexpected one
			if seq < 2 {
				seq += 2
			}

			err := sendRumor(chat, uint(seq), src, dst, sender)
			if err != nil {
				return xerrors.Errorf("failed to spam node: %v", err)
			}
		} else {
			acketPackedID := make([]byte, 12)
			rand.Read(acketPackedID)

			err := sendAck(string(acketPackedID), src, dst, sender)
			if err != nil {
				return xerrors.Errorf("failed to send ack: %v", err)
			}
		}
	}

	return nil
}

func sendRumor(msg types.Message, seq uint, src, dst string, sender transport.Socket) error {

	embedded, err := defaultRegistry.MarshalMessage(msg)
	if err != nil {
		return xerrors.Errorf("failed to marshal msg: %v", err)
	}

	rumor := types.Rumor{
		Origin:   src,
		Sequence: seq,
		Msg:      &embedded,
	}

	rumorsMessage := types.RumorsMessage{
		Rumors: []types.Rumor{rumor},
	}

	transpMsg, err := defaultRegistry.MarshalMessage(rumorsMessage)
	if err != nil {
		return xerrors.Errorf("failed to marshal transp msg: %v", err)
	}

	header := transport.NewHeader(src, src, dst, 1)

	pkt := transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = sender.Send(dst, pkt, 0)
	if err != nil {
		return xerrors.Errorf("failed to send message: %v", err)
	}

	return nil
}

func sendAck(ackedPacketID string, src, dst string, sender transport.Socket) error {
	ack := types.AckMessage{
		AckedPacketID: ackedPacketID,
	}

	registry := standard.NewRegistry()

	msg, err := registry.MarshalMessage(ack)
	if err != nil {
		return xerrors.Errorf("failed to marshal message: %v", err)
	}

	header := transport.NewHeader(src, src, dst, 1)

	pkt := transport.Packet{
		Header: &header,
		Msg:    &msg,
	}

	err = sender.Send(dst, pkt, 0)
	if err != nil {
		return xerrors.Errorf("failed to send message: %v", err)
	}

	return nil
}
