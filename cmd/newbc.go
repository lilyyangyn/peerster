package cmd

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/permissioned-chain"
)

// -----------------------------------------------------------------------------
// Prompt Account

var promptAccount = &survey.Select{
	Message: "What do you want to do ?",
	Options: actionOptsAccount,
}

var actionOptsAccount = []string{
	"🌱 Generate a new blockchain account",
	"🌿 Use an existing blockchain account",
	"🍃 Exit",
}

var actionsAccount = map[string]func(node *z.TestNode) error{
	actionOptsAccount[0]: createAccount,
	actionOptsAccount[1]: loadAccount,
	actionOptsAccount[2]: exitNode,
}

// -----------------------------------------------------------------------------
// Start New Blockchain

func startNewBC(node *z.TestNode) error {
	// initialize blockchain account
	var action string
	for {
		err := survey.AskOne(promptAccount, &action)
		if err != nil {
			return err
		}

		method := actionsAccount[action]
		err = method(node)
		if err != nil {
			fmt.Println("err:", err)
		}

		addr, err := node.BCGetAddress()
		if err != nil {
			fmt.Println("err:", err)
			continue
		}

		fmt.Println("#######################################################")
		fmt.Println("########      Creating Blockchain Account      ########")
		fmt.Println("#######################################################")
		fmt.Println("Node account' address: ", addr.Hex)
		fmt.Println()

		break

	}

	// read config and initialize blockchain
	for {
		err := loadConfigAndInit(node)
		if err == nil {
			break
		}
		fmt.Println("err:", err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Initialize account

// Create new key pair
func createAccount(node *z.TestNode) error {
	fmt.Println("Where do you want to store the account? Enter the path:  ")
	fp := ""
	fmt.Scanln(&fp)

	// Attempt to create it
	var d []byte
	err := os.WriteFile(fp, d, 0600)
	if err != nil {
		return err
	}
	os.Remove(fp) // delete it

	// Generate keypairs
	err = node.BCGenerateKeyPair(fp)

	return err
}

// Read a key pair from filesystem
func loadAccount(node *z.TestNode) error {
	fmt.Println("Enter the path to your privkey file:  ")
	fp := ""
	fmt.Scanln(&fp)

	// Generate keypairs
	err := node.BCLoadKeyPair(fp)
	return err
}

// -----------------------------------------------------------------------------
// Initialize blockchain

// load config and initialize blockchain
func loadConfigAndInit(node *z.TestNode) error {
	fmt.Println("Enter the path to blockchain config: ")
	fp := ""
	fmt.Scanln(&fp)

	// load config
	config, err := permissioned.ChainConfigFromYAML(fp)
	if err != nil {
		return err
	}

	// node must be inside the chain participants
	addr, err := node.BCGetAddress()
	if err != nil {
		return err
	}
	_, ok := config.Participants[addr.Hex]
	if !ok {
		return fmt.Errorf("you should be in the chain participant list")
	}

	// initial Gain is equal per account. (100)
	// FIXME: customize setting?
	initialGain := make(map[string]float64)
	for key := range config.Participants {
		initialGain[key] = 100
	}

	// init blockchain
	err = node.InitBlockchain(*config, initialGain)
	return err
}
