package cmd

import (
	"fmt"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	z "go.dedis.ch/cs438/internal/testing"
)

// -----------------------------------------------------------------------------
// Node CMD Prompt

//	var prompt = &survey.Select{
//		Message: "What do you want to do ?",
//		Options: actionOpts,
//	}
var basicActionOpts = []string{
	"🐋 Show Blockchain",
	"🦈 Add Peer",
	"🐊 Show Encryption Pubkey",
	"🐙 Refresh",
	"🍃 Exit",
}

var actionOpts = append([]string{
	"🦑 Start MPC",
}, basicActionOpts...)

var actions = map[string]func(*z.TestNode) error{
	actionOpts[0]:      startMPC,
	basicActionOpts[0]: getBCInfo,
	basicActionOpts[1]: addPeer,
	basicActionOpts[2]: getEnckey,
	basicActionOpts[3]: refresh,
	basicActionOpts[4]: exitNode,
}

// -----------------------------------------------------------------------------
// Perform actions

func performActions(node *z.TestNode) {
	var action string
	var enckeyAvailable = node.BCAllEncryptKeySet()
	for {
		opts := actionOpts
		if !enckeyAvailable {
			opts = basicActionOpts
			enckeyAvailable = node.BCAllEncryptKeySet()
		}

		prompt := &survey.Select{
			Message: "What do you want to do ?",
			Options: opts,
		}

		err := survey.AskOne(prompt, &action)
		if err != nil {
			printError(err)
			return
		}

		method := actions[action]
		err = method(node)
		if err != nil {
			printError(err)
		}
	}
}

// -----------------------------------------------------------------------------
// CMD Actions

func startMPC(node *z.TestNode) error {
	fmt.Println("Enter the expression: ")
	expr := ""
	fmt.Scanln(&expr)

	fmt.Printf("Your balance is %f\n", node.BCGetBalance())
	fmt.Println("Enter the budget: ")
	budgetStr := ""
	fmt.Scanln(&budgetStr)
	budget, err := strconv.ParseFloat(budgetStr, 64)
	if err != nil {
		return err
	}

	value, err := node.Calculate(expr, budget)
	if err != nil {
		return err
	}
	fmt.Printf("The result of %s is %d\n", expr, value)
	fmt.Printf("Your balance is %f\n", node.BCGetBalance())

	return nil
}

func addPeer(node *z.TestNode) error {
	fmt.Println("Enter the peer IP address: ")
	addr := ""
	fmt.Scanln(&addr)

	node.AddPeer(addr)
	return nil
}

func getEnckey(node *z.TestNode) error {
	pub, err := node.GetPubkeyString()
	if err != nil {
		return err
	}

	fmt.Println("Enc key: ", pub)
	return nil
}

func getBCInfo(node *z.TestNode) error {
	fmt.Println(node.BCSprintBlockchain())
	return nil
}

func refresh(node *z.TestNode) error {
	return nil
}
