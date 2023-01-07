package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/AlecAivazis/survey/v2"
	z "go.dedis.ch/cs438/internal/testing"
)

var t = testing{}

// -----------------------------------------------------------------------------
// Init Prompt

var promptInit = &survey.Select{
	Message: "What do you want to do ?",
	Options: actionOptsInit,
}

var actionOptsInit = []string{
	"🌱 Start a new blockchain",
	"🌿 Use an existing blockchain",
	"🍃 Exit",
}

var actionsInit = map[string]func(*z.TestNode) error{
	actionOptsInit[0]: startNewBC,
	actionOptsInit[1]: startExistingBC,
	actionOptsInit[2]: exitNode,
}

// -----------------------------------------------------------------------------
// start node

func startNode(node *z.TestNode) error {
	pubkey, err := node.GetPubkeyString()
	if err != nil {
		return err
	}
	fmt.Println("##########################################")
	fmt.Println("######      Starting a MPCPeer      ######")
	fmt.Println("##########################################")
	fmt.Println("Node running on address: ", node.GetAddr())
	fmt.Println("Contact public key: ", pubkey)
	fmt.Println()

	// prompt to start a blockchain
	var action string
	for {
		err := survey.AskOne(promptInit, &action)
		if err != nil {
			return err
		}

		method := actionsInit[action]
		err = method(node)
		if err == nil {
			break
		}
		fmt.Println(err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Exit

func exitNode(node *z.TestNode) error {
	err := node.Stop()
	if err != nil {
		log.Fatalf("failed to stop node: %v", err)
		return err
	}

	fmt.Println("bye 👋")
	os.Exit(0)
	return nil
}

// -----------------------------------------------------------------------------
// Utils

// testing provides a simple implementation of the require.Testing interface.
// Needed because we use some the the testing utility functions.
type testing struct{}

func (testing) Errorf(format string, args ...interface{}) {
	fmt.Println("~~ERROR~~")
	fmt.Printf(format, args...)
}

func (testing) FailNow() {
	os.Exit(1)
}
