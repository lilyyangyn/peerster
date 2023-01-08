package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/peer/impl"
	"go.dedis.ch/cs438/transport/udp"
)

// -----------------------------------------------------------------------------
// Node CMD Prompt

var prompt = &survey.Select{
	Message: "What do you want to do ?",
	Options: actionOpts,
}

var actionOpts = []string{
	"🌱 Start MPC",
	"🍃 Exit",
}

var actions = map[string]func(*z.TestNode) error{
	actionOptsInit[0]: startNewBC,
	actionOptsInit[1]: startExistingBC,
}

// -----------------------------------------------------------------------------
// Start CMD

func StartCMD(daemon bool, customOpts ...z.Option) {
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
	node := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", opts...)

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		exitNode(&node)
		os.Exit(1)
	}()

	err := startNode(&node)
	if err != nil {
		fmt.Println(err)
		return
	}

	if daemon {
		return
	}

	var action string
	for {
		err := survey.AskOne(prompt, &action)
		if err != nil {
			fmt.Println(err)
			return
		}

		method := actions[action]
		err = method(&node)
		if err != nil {
			fmt.Println(err)
		}
	}
}
