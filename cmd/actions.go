package cmd

import (
	"fmt"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	z "go.dedis.ch/cs438/internal/testing"
)

// -----------------------------------------------------------------------------
// Node CMD Prompt

var prompt = &survey.Select{
	Message: "What do you want to do ?",
	Options: actionOpts,
}

var actionOpts = []string{
	"🦑 Start MPC",
	"🐋 Show Blockchain",
	"🦈 Add Peer",
	"🦭 Show Encryption Pubkey",
	"🍃 Exit",
}

var actions = map[string]func(*z.TestNode) error{
	actionOpts[0]: startMPC,
	actionOpts[1]: getBCInfo,
	actionOpts[2]: addPeer,
	actionOpts[3]: getEnckey,
	actionOpts[4]: exitNode,
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
