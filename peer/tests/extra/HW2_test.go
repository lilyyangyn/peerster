package extra

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/transport/channel"
	"go.dedis.ch/cs438/types"
)

// E:2-1
//
// Checks that an error is returned when the resulting metafile is bigger than
// the chunksize.
func FuzzHW2MetafileSize(f *testing.F) {
	// chunkSize, fileSize
	f.Add(uint(0), uint(0))
	f.Add(uint(0), uint(30))
	f.Add(uint(30), uint(0))
	f.Add(uint(100), uint(100))
	f.Add(uint(99999), uint(99999))
	f.Add(uint(10), uint(99999))
	f.Add(uint(99999), uint(10))
	f.Add(uint(99999), uint(10))

	// max size for two chunks
	max := 64 + 64 + len(peer.MetafileSep)
	f.Add(uint(max), uint(max-1))
	f.Add(uint(max), uint(max))
	f.Add(uint(max), uint(max+1))
	f.Add(uint(max-1), uint(max))
	f.Add(uint(max+1), uint(max))

	transp := channel.NewTransport()

	f.Fuzz(func(t *testing.T, chunkSize uint, filesize uint) {
		maxSize := (int(chunkSize) + len(peer.MetafileSep)) / (64 + len(peer.MetafileSep)) * int(chunkSize)

		node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithChunkSize(chunkSize), z.WithAutostart(false))
		defer node1.Stop()

		file := make([]byte, filesize)

		buf := bytes.NewBuffer(file)

		_, err := node1.Upload(buf)
		if filesize > uint(maxSize) || chunkSize == 0 {
			// > can't accept a metafile whose size is greater than the
			// chunksize
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	})
}

// E:2-2
//
// Checks different inputs for the tagging function. The node should not return
// an error.
func FuzzHW2Tag(f *testing.F) {
	// name, metahash
	f.Add("", []byte{})
	f.Add("aa", []byte("bb"))
	f.Add("\b \xb5", []byte("0\xe7$V"))
	f.Add("", make([]byte, 257))
	f.Add(string(make([]byte, 257)), []byte{})

	transp := channel.NewTransport()

	node1 := z.NewTestNode(f, peerFac, transp, "127.0.0.1:0", z.WithChunkSize(2), z.WithAutostart(false))
	defer node1.Stop()

	f.Fuzz(func(t *testing.T, name string, mh []byte) {
		err := node1.Tag(name, hex.EncodeToString(mh))
		require.NoError(t, err)
	})
}

// E:2-3
//
// Sends multiple search request messages and checks that the recipient can
// still process the requests.
func FuzzHW2Search(f *testing.F) {
	// requestID, Origin, Pattern, Budget
	f.Add("", "", "", uint(0))
	f.Add("xx", "yy", "zz", uint(1))
	f.Add("", "\b \xb5", "\" gKVA\br", uint(0))
	f.Add("", "", "\" gKVA\br", uint(0))
	f.Add("", "0\xb5", "0\xe7$V", uint(0))
	f.Add("", "0", "0&&&&>$>>", uint(0))
	f.Add("\x9d\x1b\xd7", "", "", uint(0))
	f.Add("0", "", "0", uint(0))

	transp := channel.NewTransport()

	node1 := z.NewTestNode(f, peerFac, transp, "127.0.0.1:0", z.WithChunkSize(2))
	defer node1.Stop()

	sender, err := transp.CreateSocket("127.0.0.1:0")
	require.NoError(f, err)

	defer sender.Close()

	f.Fuzz(func(t *testing.T, requestID, origin, pattern string, budget uint) {
		search := types.SearchRequestMessage{
			RequestID: requestID,
			Origin:    origin,
			Pattern:   pattern,
			Budget:    budget,
		}

		src, dst := sender.GetAddress(), node1.GetAddr()

		transpMsg, err := defaultRegistry.MarshalMessage(search)
		require.NoError(t, err)

		header := transport.NewHeader(src, src, dst, 1)

		pkt := transport.Packet{
			Header: &header,
			Msg:    &transpMsg,
		}

		// > performs two sends to check that the node is still up

		err = sender.Send(dst, pkt, time.Second)
		require.NoError(t, err)

		err = sender.Send(dst, pkt, time.Second)
		require.NoError(t, err)
	})
}

// E:2-4
//
// Downloads a file from a remote peer that is supposed to have the file, but
// doesn't. In that case we are expecting the requester node to update its
// catalog and remove the erroneous entry.
func Test_HW2_DownloadWringCatalog(t *testing.T) {
	transp := channel.NewTransport()

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node2.Stop()

	node1.AddPeer(node2.GetAddr())
	node2.AddPeer(node1.GetAddr())

	// a random metahash
	mh := "6a0b1d67884e58786e97bc51544cbba4cc3e1279d8ff46da2fa32bcdb44a053e"

	// (wrongly) telling node1 that node2 has the data

	node1.UpdateCatalog(string(mh), node2.GetAddr())

	// > Node1 should get an error

	_, err := node1.Download(mh)
	require.Error(t, err)

	// > Node1 should have sent 1 data request

	n1outs := node1.GetOuts()

	require.Len(t, n1outs, 1)

	msg := z.GetDataRequest(t, n1outs[0].Msg)
	require.Equal(t, string(mh), msg.Key)
	reqID1 := msg.RequestID
	require.Equal(t, node2.GetAddr(), n1outs[0].Header.Destination)
	require.Equal(t, node1.GetAddr(), n1outs[0].Header.Source)
	require.Equal(t, node1.GetAddr(), n1outs[0].Header.RelayedBy)

	// > Node1 should have received 1 empty data reply

	n1ins := node1.GetIns()

	require.Len(t, n1ins, 1)

	msg2 := z.GetDataReply(t, n1ins[0].Msg)
	require.Equal(t, string(mh), msg2.Key)
	// > empty value since node2 doesn't have the metahash
	require.Equal(t, "", string(msg2.Value))
	require.Equal(t, reqID1, msg2.RequestID)
	require.Equal(t, node1.GetAddr(), n1ins[0].Header.Destination)
	require.Equal(t, node2.GetAddr(), n1ins[0].Header.Source)
	require.Equal(t, node2.GetAddr(), n1ins[0].Header.RelayedBy)

	// > Node2 should have received 1 data request

	n2ins := node2.GetIns()

	require.Len(t, n2ins, 1)

	msg = z.GetDataRequest(t, n2ins[0].Msg)
	require.Equal(t, string(mh), msg.Key)
	reqID1 = msg.RequestID
	require.Equal(t, node2.GetAddr(), n2ins[0].Header.Destination)
	require.Equal(t, node1.GetAddr(), n2ins[0].Header.Source)
	require.Equal(t, node1.GetAddr(), n2ins[0].Header.RelayedBy)

	// > Node2 should have sent 1 empty data reply

	n2outs := node2.GetOuts()

	require.Len(t, n2outs, 1)

	msg2 = z.GetDataReply(t, n2outs[0].Msg)
	require.Equal(t, string(mh), msg2.Key)
	require.Equal(t, "", string(msg2.Value))
	require.Equal(t, reqID1, msg2.RequestID)
	require.Equal(t, node1.GetAddr(), n2outs[0].Header.Destination)
	require.Equal(t, node2.GetAddr(), n2outs[0].Header.Source)
	require.Equal(t, node2.GetAddr(), n2outs[0].Header.RelayedBy)

	// > most important part: node1's catalog should be updated

	catalog := node1.GetCatalog()
	require.Empty(t, catalog[mh])
}

// E:2-5
//
// Checks the speed for N nodes to (U)pload, (T)ag, (S)earch, (R)esolve, and
// (D)ownload. Each nodes are connected to each other, and each node gets all
// files from the other nodes. Run as follow:
//
//	GLOG=no go test -v -benchtime 10x -bench ^BenchmarkUTSRD$ -run ^$ \
//		-benchmem -count 5 ./peer/tests/extra
func BenchmarkUTSRD(b *testing.B) {

	// Disable outputs to not penalize implementations that make use of it
	oldStdout := os.Stdout
	os.Stdout = nil

	defer func() {
		os.Stdout = oldStdout
	}()

	n := b.N
	rand.Seed(1)
	chunkSize := 2048
	maxSize := (chunkSize + len(peer.MetafileSep)) / (64 + len(peer.MetafileSep)) * chunkSize

	transp := channel.NewTransport()

	nodes := make([]z.TestNode, n)
	files := make([][]byte, n)
	metahashes := make([]string, n)
	names := make([]string, n)

	// Create N nodes.
	for i := range nodes {
		node := z.NewTestNode(b, peerFac, transp, "127.0.0.1:0", z.WithChunkSize(2048))
		defer node.Stop()

		nodes[i] = node
	}

	// Connect all nodes together.
	for i := range nodes {
		for j := range nodes {
			if i != j {
				nodes[i].AddPeer(nodes[j].GetAddr())
			}
		}
	}

	getFileAndName := func(fsize uint) ([]byte, string) {
		file := make([]byte, fsize)
		_, err := rand.Read(file)
		require.NoError(b, err)

		name := make([]byte, 8)
		_, err = rand.Read(name)
		require.NoError(b, err)

		return file, hex.EncodeToString(name)
	}

	// 1 (UT): (U)pload and (T)ag a file on each node
	for i, node := range nodes {
		file, name := getFileAndName(uint(rand.Intn(maxSize)))

		mh, err := node.Upload(bytes.NewBuffer(file))
		require.NoError(b, err)

		err = node.Tag(name, mh)
		require.NoError(b, err)

		files[i] = file
		metahashes[i] = mh
		names[i] = name
	}

	expandingRing := peer.ExpandingRing{
		Initial: uint(n),
		Factor:  2,
		Retry:   uint(n),
		Timeout: time.Second * 10,
	}

	// 2 (SRD): (S)earch, (R)esolve, and (D)ownload. Each node gets all the
	// files.
	for _, node := range nodes {
		for i, name := range names {
			match, err := node.SearchFirst(*regexp.MustCompile(name), expandingRing)
			require.NoError(b, err)
			require.Equal(b, name, match)

			mh := node.Resolve(match)
			require.Equal(b, metahashes[i], mh)

			data, err := node.Download(mh)
			require.NoError(b, err)

			require.Equal(b, files[i], data)
		}
	}
}
