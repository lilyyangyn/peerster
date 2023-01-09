package cmd

import (
	"fmt"

	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/permissioned-chain"
)

// -----------------------------------------------------------------------------
// Use an existing blockchain

func startExistingBC(node *z.TestNode, actionMap map[string]ActionFunc) error {
	fmt.Println("Waiting for the genesis block")
	genesis := node.BCWaitBlock()
	config := permissioned.GetConfigFromWorldState(genesis.States)
	fmt.Println("Got genesis block. Chain config: ", config)

	return nil
}
