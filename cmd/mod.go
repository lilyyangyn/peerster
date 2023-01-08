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
// Start CMD

func StartCMD(ip string, daemon bool, customOpts ...z.Option) {
	if len(ip) == 0 {
		ip = "127.0.0.1:6050"
	}

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
		exitNode(&node)
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

func printError(err error) {
	fmt.Println("🐡 Ops. Something is wrong: ", err)
}
