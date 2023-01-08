package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/peer/impl"
	"go.dedis.ch/cs438/transport/udp"
)

// -----------------------------------------------------------------------------
// Action Define

const (
	StartNewBC      = "🌱 Start a new blockchain"
	StartExistingBC = "🌿 Use an existing blockchain"

	GenerateBCAccount = "🌱 Generate a new blockchain account"
	LoadBCAccount     = "🌿 Use an existing blockchain account"

	MPCCalc = "🦑 MPC Calculate"

	ShowAssets     = "🐥 Show Assets"
	AddAsset       = "🐣 Add Asset to Database"
	ShowBalance    = "🐳 Show Balance"
	ShowBlockchain = "🐋 Show Blockchain"
	AddPeer        = "🦈 Add Peer"
	ShowEncKey     = "🐊 Show Encryption Pubkey"
	Refresh        = "🐙 Refresh"

	NextBlockchainPage = "🍀 More..."
	ReturnPrevious     = "🎋 Return back"

	Exit = "🍃 Exit"
)

type ActionFunc func(*z.TestNode, map[string]ActionFunc) error

var actionMap = map[string]ActionFunc{
	StartNewBC:      startNewBC,
	StartExistingBC: startExistingBC,

	GenerateBCAccount: createAccount,
	LoadBCAccount:     loadAccount,

	MPCCalc: startMPC,

	ShowAssets:     showAssets,
	AddAsset:       addAsset,
	ShowBalance:    getBCBalance,
	ShowBlockchain: getBCInfo,
	AddPeer:        addPeer,
	ShowEncKey:     getEnckey,
	Refresh:        refresh,

	Exit: exitNode,
}

// -----------------------------------------------------------------------------
// Start CMD

func StartCMD(port int, daemon bool, customOpts ...z.Option) {
	if port == 0 {
		port = 6050
	}

	ip := fmt.Sprintf("127.0.0.1:%d", port)

	transp := udp.NewUDP()
	peerFac := impl.NewPeer

	// set options
	basicOpts := []z.Option{
		z.WithAckTimeout(time.Second * 30),
		z.WithHeartbeat(0),
		z.WithAntiEntropy(time.Second * time.Duration(5+rand.Intn(5))),
	}
	opts := append(basicOpts, customOpts...)

	// start a node
	node := z.NewTestNode(t, peerFac, transp, ip, opts...)
	fmt.Println("#######################################################")
	fmt.Println("######             Starting a MPCPeer            ######")
	fmt.Println("#######################################################")
	fmt.Println("Node running on address: ", node.GetAddr())
	fmt.Println()

	// catch interrupt signal
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		exitNode(&node, actionMap)
		os.Exit(1)
	}()

	// initialize chain
	err := startBlockchain(&node)
	if err != nil {
		printError(err)
		return
	}

	if daemon {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		// TODO: start httpserver
		return
	}

	// perform action loop
	performActions(&node)
}

func printData(format string, a ...interface{}) {
	fmt.Printf(Yellow(format), a...)
}

func printError(err error) {
	fmt.Printf(Red("Ops. Something is wrong: %s"), err)
}
