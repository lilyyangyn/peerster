package cmd

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	z "go.dedis.ch/cs438/internal/testing"
)

var t = testing{}

// -----------------------------------------------------------------------------
// Prompt Account

var promptAccount = &survey.Select{
	Message: "What do you want to do ?",
	Options: actionOptsAccount,
}

var actionOptsAccount = []string{
	GenerateBCAccount,
	LoadBCAccount,
	Exit,
}

// -----------------------------------------------------------------------------
// Init Prompt

var promptInit = &survey.Select{
	Message: "What do you want to do ?",
	Options: actionOptsInit,
}

var actionOptsInit = []string{
	StartNewBC,
	StartExistingBC,
	Exit,
}

// -----------------------------------------------------------------------------
// start node

func startBlockchain(node *z.TestNode) error {
	// initialize blockchain account
	err := setAccount(node, actionMap)
	if err != nil {
		return err
	}

	// prompt to start a blockchain
	var action string
	for {
		err := survey.AskOne(promptInit, &action)
		if err != nil {
			return err
		}

		method := actionMap[action]
		err = method(node, actionMap)
		if err == nil {
			break
		}
		printError(err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Exit

func exitNode(node *z.TestNode, actionMap map[string]ActionFunc) error {
	err := node.Stop()
	if err != nil {
		printError(fmt.Errorf("failed to stop node: %v", err))
		return err
	}

	fmt.Println("bye 👋")
	os.Exit(0)
	return nil
}

// -----------------------------------------------------------------------------
// Utils

func setAccount(node *z.TestNode, actionMap map[string]ActionFunc) error {
	var action string
	for {
		err := survey.AskOne(promptAccount, &action)
		if err != nil {
			return err
		}

		method := actionMap[action]
		err = method(node, actionMap)
		if err != nil {
			printError(err)
			continue
		}

		addr, err := node.BCGetAddress()
		if err != nil {
			printError(err)
			continue
		}

		fmt.Println("#######################################################")
		fmt.Println("########      Creating Blockchain Account      ########")
		fmt.Println("#######################################################")
		fmt.Println("Node account' address: ", addr.Hex)
		fmt.Println()
		break
	}
	return nil
}

// testing provides a simple implementation of the require.Testing interface.
// Needed because we use some the the testing utility functions.
type testing struct{}

func (testing) Errorf(format string, args ...interface{}) {
	printError(fmt.Errorf(format, args...))
}

func (testing) FailNow() {
	os.Exit(1)
}
