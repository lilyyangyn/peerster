package unit

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/transport/channel"
	"go.dedis.ch/cs438/types"
)

// Check that a peer does nothing if it receives a prepare message with a wrong
// step.
func Test_GP_Paxos_Acceptor_Prepare_Wrong_Step(t *testing.T) {
	transp := channel.NewTransport()

	acceptor := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithTotalPeers(1),
		z.WithPaxosID(1), z.WithMPCPaxos(), z.WithDisableMPC())
	defer acceptor.Stop()

	proposer, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	acceptor.AddPeer(proposer.GetAddress())

	// sending a prepare with a wrong step

	prepare := types.PaxosPrepareMessage{
		Type:   types.PaxosTypeMPC,
		Step:   99, // wrong step
		ID:     1,
		Source: proposer.GetAddress(),
	}

	transpMsg, err := acceptor.GetRegistry().MarshalMessage(&prepare)
	require.NoError(t, err)

	header := transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), acceptor.GetAddr(), 0)

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = proposer.Send(acceptor.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second)

	// > acceptor must have ignored the message

	require.Len(t, acceptor.GetOuts(), 0)
	require.Equal(t, 0, acceptor.GetStorage().GetBlockchainStore().Len())
}

// Check that a peer does nothing if it receives a prepare message with a wrong
// ID.
func Test_GP_Paxos_Acceptor_Prepare_Wrong_ID(t *testing.T) {
	transp := channel.NewTransport()

	acceptor := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithTotalPeers(1),
		z.WithPaxosID(1), z.WithMPCPaxos(), z.WithMPCPaxos(), z.WithMPCPaxos(), z.WithDisableMPC())
	defer acceptor.Stop()

	proposer, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	acceptor.AddPeer(proposer.GetAddress())

	// sending a prepare with an ID too low

	prepare := types.PaxosPrepareMessage{
		Type:   types.PaxosTypeMPC,
		Step:   0,
		ID:     0, // ID too low
		Source: proposer.GetAddress(),
	}

	transpMsg, err := acceptor.GetRegistry().MarshalMessage(&prepare)
	require.NoError(t, err)

	header := transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), acceptor.GetAddr(), 0)

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = proposer.Send(acceptor.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second)

	// > acceptor must have ignored the message

	require.Len(t, acceptor.GetOuts(), 0)
	require.Equal(t, 0, acceptor.GetStorage().GetBlockchainStore().Len())
}

// Check that a peer sends back a promise if it receives a valid prepare
// message.
func Test_GP_Paxos_Acceptor_Prepare_Correct(t *testing.T) {
	transp := channel.NewTransport()

	acceptor := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithTotalPeers(1),
		z.WithPaxosID(1), z.WithMPCPaxos(), z.WithDisableMPC())
	defer acceptor.Stop()

	proposer, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	acceptor.AddPeer(proposer.GetAddress())

	// sending a prepare with a high ID, must then be taken into account

	prepare := types.PaxosPrepareMessage{
		Type:   types.PaxosTypeMPC,
		Step:   0,
		ID:     99,
		Source: proposer.GetAddress(),
	}

	transpMsg, err := acceptor.GetRegistry().MarshalMessage(&prepare)
	require.NoError(t, err)

	header := transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), acceptor.GetAddr(), 0)

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = proposer.Send(acceptor.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second * 2)

	// > acceptor must have sent a promise

	acceptorOuts := acceptor.GetOuts()
	require.Len(t, acceptorOuts, 1)

	rumor := z.GetRumor(t, acceptorOuts[0].Msg)
	require.Len(t, rumor.Rumors, 1)

	private := z.GetPrivate(t, rumor.Rumors[0].Msg)

	require.Len(t, private.Recipients, 1)
	require.Contains(t, private.Recipients, proposer.GetAddress())

	promise := z.GetPaxosPromise(t, private.Msg)

	require.Equal(t, uint(0), promise.AcceptedID)
	require.Nil(t, promise.AcceptedValue)
	require.Equal(t, uint(99), promise.ID)
	require.Equal(t, uint(0), promise.Step)

	// > no block added

	require.Equal(t, 0, acceptor.GetStorage().GetBlockchainStore().Len())
}

// Check that a peer does nothing if it receives a propose message with a wrong
// step.
func Test_GP_Paxos_Acceptor_Propose_Wrong_Step(t *testing.T) {
	transp := channel.NewTransport()

	acceptor := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithTotalPeers(1),
		z.WithPaxosID(1), z.WithMPCPaxos(), z.WithDisableMPC())
	defer acceptor.Stop()

	proposer, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	acceptor.AddPeer(proposer.GetAddress())

	// sending a propose with a wrong step

	budget := float64(10)
	expression := "a.v1+b.v1+c.v1"
	proposeVal, err := types.CreatePaxosValue(types.PaxosMPCValue{
		UniqID:     "xxx",
		Expression: expression,
		Budget:     budget,
	})
	require.NoError(t, err)

	propose := types.PaxosProposeMessage{
		Type:  types.PaxosTypeMPC,
		Step:  99, // wrong step
		ID:    1,
		Value: *proposeVal,
	}

	transpMsg, err := acceptor.GetRegistry().MarshalMessage(&propose)
	require.NoError(t, err)

	header := transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), acceptor.GetAddr(), 0)

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = proposer.Send(acceptor.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second)

	// > acceptor must have ignored the message

	require.Len(t, acceptor.GetOuts(), 0)
	require.Equal(t, 0, acceptor.GetStorage().GetBlockchainStore().Len())
}

// Check that a peer does nothing if it receives a propose message with a wrong
// ID.
func Test_GP_Paxos_Acceptor_Propose_Wrong_ID(t *testing.T) {
	transp := channel.NewTransport()

	acceptor := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithTotalPeers(1),
		z.WithPaxosID(1), z.WithMPCPaxos(), z.WithDisableMPC())
	defer acceptor.Stop()

	proposer, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	acceptor.AddPeer(proposer.GetAddress())

	// sending a propose with a wrong ID

	budget := float64(10)
	expression := "a.v1+b.v1+c.v1"
	proposeVal, err := types.CreatePaxosValue(types.PaxosMPCValue{
		UniqID:     "xxx",
		Expression: expression,
		Budget:     budget,
	})
	require.NoError(t, err)

	propose := types.PaxosProposeMessage{
		Type: types.PaxosTypeMPC,
		Step: 0,
		// ID too high: 0 is expected since MaxID of a proposer starts at 0 and
		// the proposer hasn't received any prepare, so its MaxID = 0.
		ID:    2,
		Value: *proposeVal,
	}

	transpMsg, err := acceptor.GetRegistry().MarshalMessage(&propose)
	require.NoError(t, err)

	header := transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), acceptor.GetAddr(), 0)

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = proposer.Send(acceptor.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second)

	// > acceptor must have ignored the message

	require.Len(t, acceptor.GetOuts(), 0)
	require.Equal(t, 0, acceptor.GetStorage().GetBlockchainStore().Len())
}

// Check that if an acceptor already promised, but receives a higher ID, then it
// must return the valid promised id and promised value.
func Test_GP_Paxos_Acceptor_Prepare_Already_Promised(t *testing.T) {
	transp := channel.NewTransport()

	acceptor := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithTotalPeers(2),
		z.WithPaxosID(1), z.WithMPCPaxos(), z.WithDisableMPC())
	defer acceptor.Stop()

	proposer, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	acceptor.AddPeer(proposer.GetAddress())

	prepare := types.PaxosPrepareMessage{
		Type: types.PaxosTypeMPC,
		Step: 0,
		ID:   5,
	}

	transpMsg, err := acceptor.GetRegistry().MarshalMessage(&prepare)
	require.NoError(t, err)

	header := transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), acceptor.GetAddr(), 0)

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = proposer.Send(acceptor.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second)

	// sending a propose, will make the proposer set its MaxID

	budget := float64(10)
	expression := "a.v1+b.v1+c.v1"
	proposeVal, err := types.CreatePaxosValue(types.PaxosMPCValue{
		UniqID:     "xxx",
		Expression: expression,
		Budget:     budget,
	})
	require.NoError(t, err)

	propose := types.PaxosProposeMessage{
		Type:  types.PaxosTypeMPC,
		Step:  0,
		ID:    5,
		Value: *proposeVal,
	}

	transpMsg, err = acceptor.GetRegistry().MarshalMessage(&propose)
	require.NoError(t, err)

	header = transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), acceptor.GetAddr(), 0)

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = proposer.Send(acceptor.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second)

	// > if the acceptor receives another prepare with a higher ID, it must
	// return the promise ID and promise value.

	prepare = types.PaxosPrepareMessage{
		Type: types.PaxosTypeMPC,
		Step: 0,
		ID:   9, // higher ID
	}

	transpMsg, err = acceptor.GetRegistry().MarshalMessage(&prepare)
	require.NoError(t, err)

	header = transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), acceptor.GetAddr(), 0)

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = proposer.Send(acceptor.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second)

	acceptorOuts := acceptor.GetOuts()

	found := false

	// > look for the paxospromise that contains the AcceptedID and
	// AcceptedValue.
	for _, e := range acceptorOuts {
		if e.Msg.Type != "rumor" {
			continue
		}

		rumor := z.GetRumor(t, e.Msg)
		if len(rumor.Rumors) != 1 || rumor.Rumors[0].Msg.Type != "private" {
			continue
		}

		private := z.GetPrivate(t, rumor.Rumors[0].Msg)
		if private.Msg.Type != "paxospromise" {
			continue
		}

		promise := z.GetPaxosPromise(t, private.Msg)
		if promise.AcceptedValue == nil {
			continue
		}

		require.Equal(t, uint(9), promise.ID)
		require.Equal(t, uint(0), promise.Step)
		require.Equal(t, uint(5), promise.AcceptedID)
		require.Equal(t, "xxx", promise.AcceptedValue.UniqID)

		value, err := types.ParsePaxosValueContent(promise.AcceptedValue)
		require.NoError(t, err)
		MPCvalue, ok := value.(*types.PaxosMPCValue)
		require.True(t, ok)
		require.Equal(t, expression, MPCvalue.Expression)
		require.Equal(t, budget, MPCvalue.Budget)

		found = true
		break
	}

	require.True(t, found)
}

// Check that a peer broadcast an accept if it receives a propose message with a
// correct ID and correct Step.
func Test_GP_Paxos_Acceptor_Propose_Correct(t *testing.T) {
	transp := channel.NewTransport()

	acceptor := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithTotalPeers(1),
		z.WithPaxosID(1), z.WithMPCPaxos(), z.WithDisableMPC())
	defer acceptor.Stop()

	proposer, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	acceptor.AddPeer(proposer.GetAddress())

	budget := float64(10)
	expression := "a.v1+b.v1+c.v1"
	proposeVal, err := types.CreatePaxosValue(types.PaxosMPCValue{
		UniqID:     "xxx",
		Expression: expression,
		Budget:     budget,
	})
	require.NoError(t, err)

	propose := types.PaxosProposeMessage{
		Type:  types.PaxosTypeMPC,
		Step:  0,
		ID:    0,
		Value: *proposeVal,
	}

	transpMsg, err := acceptor.GetRegistry().MarshalMessage(&propose)
	require.NoError(t, err)

	header := transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), acceptor.GetAddr(), 0)

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = proposer.Send(acceptor.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second)

	// > acceptor must have broadcasted an accept message. Must be the first
	// sent message in this case.

	acceptorOuts := acceptor.GetOuts()
	require.GreaterOrEqual(t, len(acceptorOuts), 1)

	rumor := z.GetRumor(t, acceptorOuts[0].Msg)
	require.Len(t, rumor.Rumors, 1)

	accept := z.GetPaxosAccept(t, rumor.Rumors[0].Msg)

	require.Equal(t, uint(0), accept.ID)
	require.Equal(t, uint(0), accept.Step)

	value, err := types.ParsePaxosValueContent(&accept.Value)
	require.NoError(t, err)
	MPCvalue, ok := value.(*types.PaxosMPCValue)
	require.True(t, ok)

	require.Equal(t, expression, MPCvalue.Expression)
	require.Equal(t, budget, MPCvalue.Budget)
	require.Equal(t, "xxx", accept.Value.UniqID)
}

// Check that a peer does nothing if it receives a promise with a wrong step.
func Test_GP_Paxos_Proposer_Prepare_Promise_Wrong_Step(t *testing.T) {
	transp := channel.NewTransport()

	paxosID := uint(9)

	// Two nodes needed for a consensus. Setting a special paxos ID.
	proposer := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0",
		z.WithPaxosProposerRetry(time.Hour),
		z.WithTotalPeers(2),
		z.WithPaxosID(paxosID),
		z.WithMPCPaxos(), z.WithDisableMPC())

	defer proposer.Stop()

	acceptor, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	proposer.AddPeer(acceptor.GetAddress())

	// making proposer propose

	go func() {
		_, err := proposer.Calculate("a.v1+b.v1+c.v1", 10)
		require.NoError(t, err)
	}()

	time.Sleep(time.Second)

	// > the socket must receive a paxos prepare

	packet, err := acceptor.Recv(time.Second)
	require.NoError(t, err)

	rumor := z.GetRumor(t, packet.Msg)
	require.Len(t, rumor.Rumors, 1)

	prepare := z.GetPaxosPrepare(t, rumor.Rumors[0].Msg)
	require.Equal(t, paxosID, prepare.ID)
	require.Equal(t, uint(0), prepare.Step)

	// > proposer has broadcasted the paxos propose and sent to itself a
	// promise.

	n1outs := proposer.GetOuts()
	require.Len(t, n1outs, 2)

	// sending back a promise with a wrong step

	promise := types.PaxosPromiseMessage{
		Type: types.PaxosTypeMPC,
		Step: 99,
		ID:   paxosID,
	}

	transpMsg, err := proposer.GetRegistry().MarshalMessage(&promise)
	require.NoError(t, err)

	header := transport.NewHeader(acceptor.GetAddress(), acceptor.GetAddress(), proposer.GetAddr(), 0)

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = acceptor.Send(proposer.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second)

	// > proposer must have ignored the message

	n1outs = proposer.GetOuts()

	require.Len(t, n1outs, 2)
	require.Equal(t, 0, proposer.GetStorage().GetBlockchainStore().Len())
	require.Equal(t, 0, proposer.GetStorage().GetNamingStore().Len())
}

// Check that a peer broadcast a propose when it gets enough promises.
func Test_GP_Paxos_Proposer_Prepare_Propose_Correct(t *testing.T) {
	transp := channel.NewTransport()

	paxosID := uint(9)

	// TWO nodes needed for a consensus. Setting a special paxos ID.
	proposer := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0",
		z.WithPaxosProposerRetry(time.Hour),
		z.WithTotalPeers(2),
		z.WithPaxosID(paxosID),
		z.WithAckTimeout(0),
		z.WithMPCPaxos(), z.WithDisableMPC())

	defer proposer.Stop()

	acceptor, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	proposer.AddPeer(acceptor.GetAddress())

	// making proposer propose

	go func() {
		_, err := proposer.Calculate("a.v1+b.v1+c.v1", 10)
		require.NoError(t, err)
	}()

	time.Sleep(time.Second)

	// > the socket must receive a paxos prepare

	packet, err := acceptor.Recv(time.Second * 3)
	require.NoError(t, err)

	rumor := z.GetRumor(t, packet.Msg)
	require.Len(t, rumor.Rumors, 1)

	prepare := z.GetPaxosPrepare(t, rumor.Rumors[0].Msg)
	require.Equal(t, paxosID, prepare.ID)
	require.Equal(t, uint(0), prepare.Step)

	// sending back a correct promise

	promise := types.PaxosPromiseMessage{
		Type: types.PaxosTypeMPC,
		Step: 0,
		ID:   paxosID,
	}

	transpMsg, err := proposer.GetRegistry().MarshalMessage(&promise)
	require.NoError(t, err)

	header := transport.NewHeader(acceptor.GetAddress(), acceptor.GetAddress(), proposer.GetAddr(), 0)

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = acceptor.Send(proposer.GetAddr(), packet, 0)
	require.NoError(t, err)

	go func() {
		// to fill the GetIns() array
		for {
			acceptor.Recv(0)
		}
	}()

	time.Sleep(time.Second * 3)

	// > proposer must have broadcasted a propose

	acceptorIns := acceptor.GetIns()

	proposes := getProposeMessagesFromRumors(t, acceptorIns, proposer.GetAddr())
	require.Len(t, proposes, 1)

	require.Equal(t, paxosID, proposes[0].ID)
	require.Equal(t, uint(0), proposes[0].Step)

	value, err := types.ParsePaxosValueContent(&proposes[0].Value)
	require.NoError(t, err)
	mpcvalue, ok := value.(*types.PaxosMPCValue)
	require.True(t, ok)

	require.Equal(t, "a.v1+b.v1+c.v1", mpcvalue.Expression)
	require.Equal(t, float64(10), mpcvalue.Budget)
}

// Check that a peer can differentiate paxos message from different paxos instance
// i.e. it will do nothing if the promise messages are from a Tag paxos even the number
// of promises it received reaches the threshold
func Test_GP_Paxos_Proposer_Prepare_Propose_Wrong_Type(t *testing.T) {
	transp := channel.NewTransport()

	paxosID := uint(9)

	// TWO nodes needed for a consensus. Setting a special paxos ID.
	proposer := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0",
		z.WithPaxosProposerRetry(time.Hour),
		z.WithTotalPeers(2),
		z.WithPaxosID(paxosID),
		z.WithAckTimeout(0),
		z.WithMPCPaxos(), z.WithDisableMPC())

	defer proposer.Stop()

	acceptor, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	proposer.AddPeer(acceptor.GetAddress())

	// making proposer propose

	go func() {
		_, err := proposer.Calculate("a.v1+b.v1+c.v1", 10)
		require.NoError(t, err)
	}()

	time.Sleep(time.Second)

	// > proposer has broadcasted the paxos propose and sent to itself a
	// promise.

	n1outs := proposer.GetOuts()
	require.Len(t, n1outs, 2)

	// > the socket must receive a paxos prepare

	packet, err := acceptor.Recv(time.Second * 3)
	require.NoError(t, err)

	rumor := z.GetRumor(t, packet.Msg)
	require.Len(t, rumor.Rumors, 1)

	prepare := z.GetPaxosPrepare(t, rumor.Rumors[0].Msg)
	require.Equal(t, paxosID, prepare.ID)
	require.Equal(t, uint(0), prepare.Step)

	// sending back a correct promise

	promise := types.PaxosPromiseMessage{
		Type: types.PaxosTypeTag,
		Step: 0,
		ID:   paxosID,
	}

	transpMsg, err := proposer.GetRegistry().MarshalMessage(&promise)
	require.NoError(t, err)

	header := transport.NewHeader(acceptor.GetAddress(), acceptor.GetAddress(), proposer.GetAddr(), 0)

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = acceptor.Send(proposer.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second * 3)

	// > proposer must have ignored the message

	n1outs = proposer.GetOuts()

	require.Len(t, n1outs, 2)
	require.Equal(t, 0, proposer.GetStorage().GetBlockchainStore().Len())
	require.Equal(t, 0, proposer.GetStorage().GetNamingStore().Len())
}

// If a peer doesn't receives enough TLC message it must not add a new block.
func Test_GP_TLC_Move_Step_Not_Enough(t *testing.T) {
	transp := channel.NewTransport()

	// Threshold = 2
	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithAckTimeout(0), z.WithTotalPeers(2),
		z.WithMPCPaxos(), z.WithDisableMPC())
	defer node1.Stop()

	socketX, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	node1.AddPeer(socketX.GetAddress())

	// send a first block, corresponding to the correct step

	// computed by hand
	blockHash := "9efc06df7e54b580ebb0e7d7e52cdf05773cf5165c2a2d1a52cdc9ab6fd442e0"
	previousHash := [32]byte{}

	budget := float64(10)
	expression := "a.v1+b.v1+c.v1"
	tlcVal, err := types.CreatePaxosValue(types.PaxosMPCValue{
		UniqID:     "xxx",
		Budget:     budget,
		Expression: expression,
	})
	require.NoError(t, err)

	tlc := types.TLCMessage{
		Type: types.PaxosTypeMPC,
		Step: 0,
		Block: types.BlockchainBlock{
			Index:    0,
			Hash:     z.MustDecode(blockHash),
			Value:    *tlcVal,
			PrevHash: previousHash[:],
		},
	}

	transpMsg, err := node1.GetRegistry().MarshalMessage(&tlc)
	require.NoError(t, err)

	header := transport.NewHeader(socketX.GetAddress(), socketX.GetAddress(), node1.GetAddr(), 0)

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = socketX.Send(node1.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Send a second block, but corresponding to the next step

	tlc.Step = 1

	transpMsg, err = node1.GetRegistry().MarshalMessage(&tlc)
	require.NoError(t, err)

	header = transport.NewHeader(socketX.GetAddress(), socketX.GetAddress(), node1.GetAddr(), 0)

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = socketX.Send(node1.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// > node1 must have nothing in its block store

	store := node1.GetStorage().GetBlockchainStore()
	require.Equal(t, 0, store.Len())
}

// If a peer receives enough TLC message it must then add a new block, and
// broadcast a TLC message (if not already done).
func Test_GP_TLC_Move_Step_OK(t *testing.T) {
	transp := channel.NewTransport()

	// Threshold = 2
	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithAckTimeout(0), z.WithTotalPeers(2), z.WithMPCPaxos(), z.WithDisableMPC())
	defer node1.Stop()

	socketX, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)

	node1.AddPeer(socketX.GetAddress())

	// send two TLC messages for the same step

	// computed by hand
	blockHash := "9efc06df7e54b580ebb0e7d7e52cdf05773cf5165c2a2d1a52cdc9ab6fd442e0"
	previousHash := [32]byte{}

	budget := float64(10)
	expression := "a.v1+b.v1+c.v1"
	tlcVal, err := types.CreatePaxosValue(types.PaxosMPCValue{
		UniqID:     "xxx",
		Budget:     budget,
		Expression: expression,
	})
	require.NoError(t, err)

	tlc := types.TLCMessage{
		Type: types.PaxosTypeMPC,
		Step: 0,
		Block: types.BlockchainBlock{
			Index:    0,
			Hash:     z.MustDecode(blockHash),
			Value:    *tlcVal,
			PrevHash: previousHash[:],
		},
	}

	transpMsg, err := node1.GetRegistry().MarshalMessage(&tlc)
	require.NoError(t, err)

	header := transport.NewHeader(socketX.GetAddress(), socketX.GetAddress(), node1.GetAddr(), 0)

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpMsg,
	}

	err = socketX.Send(node1.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second * 2)

	err = socketX.Send(node1.GetAddr(), packet, 0)
	require.NoError(t, err)

	time.Sleep(time.Second * 2)

	// > node1 must have a new block in its store

	store := node1.GetStorage().GetBlockchainStore()
	// one element is the last block hash, the other is the block
	require.Equal(t, 2, store.Len())

	blockBuf := store.Get(blockHash)

	var block types.BlockchainBlock
	err = block.Unmarshal(blockBuf)
	require.NoError(t, err)

	require.Equal(t, tlc.Block, block)

	// > node1 must have the block hash in the LasBlockKey store

	require.Equal(t, z.MustDecode(blockHash), store.Get(storage.MPCLastBlockKey))
}

// Given the following topology:
//
//	A -> B
//
// When A proposes a filename, then we are expecting the following message
// exchange:
//
//	A -> B: PaxosPrepare (broadcast, i.e also processed locally by A)
//
//	A -> A: PaxosPromise (broadcast-private)
//	B -> A: PaxosPromise (broadcast-private)
//
//	A -> B: PaxosPropose (broadcast, i.e also processed locally by A)
//
//	A -> A: PaxosAccept (broadcast)
//	B -> A: PaxosAccept (broadcast)
//
//	A -> B: TLC (broadcast)
//	B -> A: TLC (broadcast)
func Test_GP_MPC_Paxos_Simple_Consensus(t *testing.T) {
	transp := channel.NewTransport()

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithTotalPeers(2), z.WithPaxosID(1),
		z.WithMPCPaxos(), z.WithDisableMPC())
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithTotalPeers(2), z.WithPaxosID(2),
		z.WithMPCPaxos(), z.WithDisableMPC())
	defer node2.Stop()

	node1.AddPeer(node2.GetAddr())

	expression := "a.v1+b.v1+c.v1"
	budget := float64(10)
	_, err := node1.Calculate(expression, budget)
	require.NoError(t, err)

	time.Sleep(time.Second)

	// > node1 must have sent
	//
	//   - Rumor(1):PaxosPrepare
	//   - Rumor(2):Private:PaxosPromise
	//   - Rumor(3):PaxosPropose
	//   - Rumor(4):PaxosAccept
	//   - Rumor(5):TLC

	n1outs := node1.GetOuts()

	// >> Rumor(1):PaxosPrepare

	msg, pkt := getRumor(t, n1outs, 1)
	require.NotNil(t, msg)

	require.Equal(t, node1.GetAddr(), pkt.Source)
	require.Equal(t, node1.GetAddr(), pkt.RelayedBy)
	require.Equal(t, node2.GetAddr(), pkt.Destination)

	prepare := z.GetPaxosPrepare(t, msg)

	require.Equal(t, uint(1), prepare.ID)
	require.Equal(t, uint(0), prepare.Step)
	require.Equal(t, node1.GetAddr(), prepare.Source)

	// >> Rumor(2):Private:PaxosPromise

	msg, pkt = getRumor(t, n1outs, 2)
	require.NotNil(t, msg)

	require.Equal(t, node1.GetAddr(), pkt.Source)
	require.Equal(t, node1.GetAddr(), pkt.RelayedBy)
	require.Equal(t, node2.GetAddr(), pkt.Destination)

	private := z.GetPrivate(t, msg)

	require.Len(t, private.Recipients, 1)
	require.Contains(t, private.Recipients, node1.GetAddr())

	promise := z.GetPaxosPromise(t, private.Msg)

	require.Equal(t, uint(1), promise.ID)
	require.Equal(t, uint(0), promise.Step)
	// default "empty" value is 0
	require.Zero(t, promise.AcceptedID)
	require.Nil(t, promise.AcceptedValue)

	// >> Rumor(3):PaxosPropose

	msg, pkt = getRumor(t, n1outs, 3)
	require.NotNil(t, msg)

	require.Equal(t, node1.GetAddr(), pkt.Source)
	require.Equal(t, node1.GetAddr(), pkt.RelayedBy)
	require.Equal(t, node2.GetAddr(), pkt.Destination)

	propose := z.GetPaxosPropose(t, msg)

	require.Equal(t, uint(1), propose.ID)
	require.Equal(t, uint(0), propose.Step)

	value, err := types.ParsePaxosValueContent(&propose.Value)
	require.NoError(t, err)
	mpcvalue, ok := value.(*types.PaxosMPCValue)
	require.True(t, ok)

	require.Equal(t, expression, mpcvalue.Expression)
	require.Equal(t, budget, mpcvalue.Budget)

	// >> Rumor(4):PaxosAccept

	msg, pkt = getRumor(t, n1outs, 4)
	require.NotNil(t, msg)

	require.Equal(t, node1.GetAddr(), pkt.Source)
	require.Equal(t, node1.GetAddr(), pkt.RelayedBy)
	require.Equal(t, node2.GetAddr(), pkt.Destination)

	accept := z.GetPaxosAccept(t, msg)

	require.Equal(t, uint(1), accept.ID)
	require.Equal(t, uint(0), accept.Step)

	value, err = types.ParsePaxosValueContent(&accept.Value)
	require.NoError(t, err)
	mpcvalue, ok = value.(*types.PaxosMPCValue)
	require.True(t, ok)

	require.Equal(t, expression, mpcvalue.Expression)
	require.Equal(t, budget, mpcvalue.Budget)

	// >> Rumor(5):TLC

	msg, pkt = getRumor(t, n1outs, 5)
	require.NotNil(t, msg)

	require.Equal(t, node1.GetAddr(), pkt.Source)
	require.Equal(t, node1.GetAddr(), pkt.RelayedBy)
	require.Equal(t, node2.GetAddr(), pkt.Destination)

	tlc := z.GetTLC(t, msg)

	require.Equal(t, uint(0), tlc.Step)
	require.Equal(t, uint(0), tlc.Block.Index)
	require.Equal(t, make([]byte, 32), tlc.Block.PrevHash)

	value, err = types.ParsePaxosValueContent(&tlc.Block.Value)
	require.NoError(t, err)
	mpcvalue, ok = value.(*types.PaxosMPCValue)
	require.True(t, ok)

	require.Equal(t, expression, mpcvalue.Expression)
	require.Equal(t, budget, mpcvalue.Budget)

	// > node2 must have sent
	//
	//   - Rumor(1):Private:PaxosPromise
	//   - Rumor(2):PaxosAccept
	//   - Rumor(3):TLC

	n2outs := node2.GetOuts()

	// >> Rumor(1):Private:PaxosPromise

	msg, pkt = getRumor(t, n2outs, 1)
	require.NotNil(t, msg)

	require.Equal(t, node2.GetAddr(), pkt.Source)
	require.Equal(t, node2.GetAddr(), pkt.RelayedBy)
	require.Equal(t, node1.GetAddr(), pkt.Destination)

	private = z.GetPrivate(t, msg)

	require.Len(t, private.Recipients, 1)
	require.Contains(t, private.Recipients, node1.GetAddr())

	promise = z.GetPaxosPromise(t, private.Msg)

	require.Equal(t, uint(1), promise.ID)
	require.Equal(t, uint(0), promise.Step)
	// default "empty" value is 0
	require.Zero(t, promise.AcceptedID)
	require.Nil(t, promise.AcceptedValue)

	// >> Rumor(2):PaxosAccept

	msg, pkt = getRumor(t, n2outs, 2)
	require.NotNil(t, msg)

	require.Equal(t, node2.GetAddr(), pkt.Source)
	require.Equal(t, node2.GetAddr(), pkt.RelayedBy)
	require.Equal(t, node1.GetAddr(), pkt.Destination)

	accept = z.GetPaxosAccept(t, msg)

	require.Equal(t, uint(1), accept.ID)
	require.Equal(t, uint(0), accept.Step)

	value, err = types.ParsePaxosValueContent(&accept.Value)
	require.NoError(t, err)
	mpcvalue, ok = value.(*types.PaxosMPCValue)
	require.True(t, ok)

	require.Equal(t, expression, mpcvalue.Expression)
	require.Equal(t, budget, mpcvalue.Budget)

	// >> Rumor(3):TLC

	msg, pkt = getRumor(t, n2outs, 3)
	require.NotNil(t, msg)

	require.Equal(t, node2.GetAddr(), pkt.Source)
	require.Equal(t, node2.GetAddr(), pkt.RelayedBy)
	require.Equal(t, node1.GetAddr(), pkt.Destination)

	tlc = z.GetTLC(t, msg)

	require.Equal(t, uint(0), tlc.Step)
	require.Equal(t, uint(0), tlc.Block.Index)
	require.Equal(t, make([]byte, 32), tlc.Block.PrevHash)

	value, err = types.ParsePaxosValueContent(&tlc.Block.Value)
	require.NoError(t, err)
	mpcvalue, ok = value.(*types.PaxosMPCValue)
	require.True(t, ok)

	require.Equal(t, expression, mpcvalue.Expression)
	require.Equal(t, budget, mpcvalue.Budget)

	// > node1 blockchain store contains two elements

	bstore := node1.GetStorage().GetBlockchainStore()

	require.Equal(t, 2, bstore.Len())

	lastBlockHash := bstore.Get(storage.MPCLastBlockKey)
	lastBlock := bstore.Get(hex.EncodeToString(lastBlockHash))

	var block types.BlockchainBlock

	err = block.Unmarshal(lastBlock)
	require.NoError(t, err)

	require.Equal(t, uint(0), block.Index)
	require.Equal(t, make([]byte, 32), block.PrevHash)

	value, err = types.ParsePaxosValueContent(&block.Value)
	require.NoError(t, err)
	mpcvalue, ok = value.(*types.PaxosMPCValue)
	require.True(t, ok)

	require.Equal(t, expression, mpcvalue.Expression)
	require.Equal(t, budget, mpcvalue.Budget)

	// > node2 blockchain store contains two elements

	bstore = node2.GetStorage().GetBlockchainStore()

	require.Equal(t, 2, bstore.Len())

	lastBlockHash = bstore.Get(storage.MPCLastBlockKey)
	lastBlock = bstore.Get(hex.EncodeToString(lastBlockHash))

	err = block.Unmarshal(lastBlock)
	require.NoError(t, err)

	require.Equal(t, uint(0), block.Index)
	require.Equal(t, make([]byte, 32), block.PrevHash)

	value, err = types.ParsePaxosValueContent(&block.Value)
	require.NoError(t, err)
	mpcvalue, ok = value.(*types.PaxosMPCValue)
	require.True(t, ok)

	require.Equal(t, expression, mpcvalue.Expression)
	require.Equal(t, budget, mpcvalue.Budget)
}

// If there are 2 nodes but we set the TotalPeers to 3 and the threshold
// function to N, then there is no chance a consensus is reached. If we wait 6
// seconds, and the PaxosProposerRetry is set to 4 seconds, then the proposer
// must have retried once and sent in total 2 paxos prepare.
func Test_GP_MPC_Paxos_No_Consensus(t *testing.T) {
	transp := channel.NewTransport()

	threshold := func(i uint) int { return int(i) }

	// Threshold = 3
	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0",
		z.WithTotalPeers(3),
		z.WithPaxosID(1),
		z.WithPaxosThreshold(threshold),
		z.WithPaxosProposerRetry(time.Second*4),
		z.WithMPCPaxos(), z.WithDisableMPC())
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithTotalPeers(3),
		z.WithPaxosID(2), z.WithMPCPaxos(), z.WithDisableMPC())
	defer node2.Stop()

	node1.AddPeer(node2.GetAddr())
	node2.AddPeer(node1.GetAddr())

	MPCDone := make(chan struct{})
	timeout := time.After(time.Second * 6)

	go func() {
		_, err := node1.Calculate("a.v1+b.v1+c.v1", 10)
		require.NoError(t, err)

		close(MPCDone)
	}()

	var outs []transport.Packet

	select {
	case <-MPCDone:
		t.Error("MPC shouldn't work")
	case <-timeout:
		outs = node1.GetOuts()
	}

	// > the first rumor sent must be the paxos prepare

	msg, _ := getRumor(t, outs, 1)
	require.NotNil(t, msg)
	require.Equal(t, "paxosprepare", msg.Type)

	// > the second rumor sent must be the private rumor from A to A

	msg, _ = getRumor(t, outs, 2)
	require.NotNil(t, msg)

	private := z.GetPrivate(t, msg)
	require.Equal(t, "paxospromise", private.Msg.Type)

	// > the third rumor sent must be the second attempt with a paxos prepare

	msg, _ = getRumor(t, outs, 3)
	require.NotNil(t, msg)
	require.Equal(t, "paxosprepare", msg.Type)

	// > the fourth rumor sent must be the private rumor from A to A, in reply
	// to the second attempt

	msg, _ = getRumor(t, outs, 4)
	require.NotNil(t, msg)

	private = z.GetPrivate(t, msg)
	require.Equal(t, "paxospromise", private.Msg.Type)
}

// If there are 2 nodes but we set the TotalPeers to 3 and the threshold
// function to N, then there is no chance a consensus is reached. We then add a
// third node and a consensus should eventually be reached and name stores
// updated.
func Test_GP_MPC_Paxos_Eventual_Consensus(t *testing.T) {
	transp := channel.NewTransport()

	// Note: we are setting the antientropy on each peer to make sure all rumors
	// are spread among peers.

	// Threshold = 3

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0",
		z.WithTotalPeers(3),
		z.WithPaxosID(1),
		z.WithPaxosProposerRetry(time.Second*2),
		z.WithAntiEntropy(time.Second),
		z.WithMPCPaxos(), z.WithDisableMPC())
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0",
		z.WithTotalPeers(3),
		z.WithPaxosID(2),
		z.WithAntiEntropy(time.Second),
		z.WithMPCPaxos(), z.WithDisableMPC())
	defer node2.Stop()

	// Note: we set the heartbeat and antientropy so that node3 will annonce
	// itself and get rumors.
	node3 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0",
		z.WithTotalPeers(3),
		z.WithPaxosID(3),
		z.WithHeartbeat(time.Hour),
		z.WithAntiEntropy(time.Second),
		z.WithMPCPaxos(), z.WithDisableMPC())
	defer node3.Stop()

	node1.AddPeer(node2.GetAddr())
	node2.AddPeer(node1.GetAddr())

	MPCDone := make(chan struct{})

	expression := "a.v1+b.v1+c.v1"
	budget := float64(10)
	go func() {
		_, err := node1.Calculate(expression, budget)
		require.NoError(t, err)

		close(MPCDone)
	}()

	time.Sleep(time.Second * 3)

	select {
	case <-MPCDone:
		t.Error(t, "a consensus can't be reached")
	default:
	}

	// > Add a new peer: with 3 peers a consensus can now be reached. Node3 has
	// the heartbeat so it will annonce itself to node1.
	node3.AddPeer(node1.GetAddr())

	timeout := time.After(time.Second * 10)

	select {
	case <-MPCDone:
	case <-timeout:
		t.Error(t, "a consensus must have been reached")
	}

	// wait for rumors to be spread, especially TLC messages.
	time.Sleep(time.Second * 3)

	// > all nodes must have broadcasted 1 TLC message. There could be more sent
	// if the node replied to a status from a peer that missed the broadcast.

	tlcMsgs := getTLCMessagesFromRumors(t, node1.GetOuts(), node1.GetAddr())
	require.GreaterOrEqual(t, len(tlcMsgs), 1)

	tlcMsgs = getTLCMessagesFromRumors(t, node2.GetOuts(), node2.GetAddr())
	require.GreaterOrEqual(t, len(tlcMsgs), 1)

	tlcMsgs = getTLCMessagesFromRumors(t, node3.GetOuts(), node3.GetAddr())
	require.GreaterOrEqual(t, len(tlcMsgs), 1)
}

// Call the Calculate() function on multiple peers concurrently. The state should be
// consistent for all peers.
func Test_GP_MPC_Paxos_Consensus_Stress_Test(t *testing.T) {
	numMessages := 7
	numNodes := 3

	transp := channel.NewTransport()

	nodes := make([]z.TestNode, numNodes)

	for i := range nodes {
		node := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0",
			z.WithTotalPeers(uint(numNodes)),
			z.WithPaxosID(uint(i+1)),
			z.WithMPCPaxos(), z.WithDisableMPC())
		defer node.Stop()

		nodes[i] = node
	}

	for _, n1 := range nodes {
		for _, n2 := range nodes {
			n1.AddPeer(n2.GetAddr())
		}
	}

	wait := sync.WaitGroup{}
	wait.Add(numNodes)

	for k, node := range nodes {
		k := k
		go func(n z.TestNode) {
			defer wait.Done()

			for i := 0; i < numMessages; i++ {
				name := make([]byte, 12)
				rand.Read(name)

				_, err := n.Calculate(fmt.Sprintf("a.v%d+b.v%d+c.v%d", k, i, i), 10)
				require.NoError(t, err)

				time.Sleep(time.Duration(rand.Int63n(int64(time.Second))))
			}
		}(node)
	}

	wait.Wait()

	time.Sleep(time.Second * 2)

	lastHashes := map[string]struct{}{}

	for _, node := range nodes {
		store := node.GetStorage().GetBlockchainStore()
		require.Equal(t, numMessages*numNodes+1, store.Len())

		lastHashes[string(store.Get(storage.MPCLastBlockKey))] = struct{}{}

		z.ValidateBlockchain(t, store, storage.MPCLastBlockKey)

		z.DisplayLastBlockchainBlock(t, os.Stdout, node.GetStorage().GetBlockchainStore(), storage.MPCLastBlockKey)
	}

	// > all peers must have the same last hash
	require.Len(t, lastHashes, 1)
}
