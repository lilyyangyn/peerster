package cmd

import (
	"fmt"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/permissioned-chain"
)

// -----------------------------------------------------------------------------
// Node CMD Prompt

//	var prompt = &survey.Select{
//		Message: "What do you want to do ?",
//		Options: actionOpts,
//	}

var basicActionOpts = []string{
	ShowAssets,
	AddAsset,
	ShowBalance,
	ShowBlockchain,
	AddPeer,
	ShowEncKey,
	Refresh,
	Exit,
}

var actionOpts = append([]string{
	MPCCalc,
}, basicActionOpts...)

// -----------------------------------------------------------------------------
// Perform actions

func performActions(node *z.TestNode) {
	var action string
	// var enckeyAvailable = node.BCAllEncryptKeySet()
	for {
		opts := actionOpts
		if !node.BCAllEncryptKeySet() {
			opts = basicActionOpts
			// enckeyAvailable = node.BCAllEncryptKeySet()
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

		method := actionMap[action]
		err = method(node, actionMap)
		if err != nil {
			printError(err)
		}
	}
}

// -----------------------------------------------------------------------------
// CMD Actions

func startMPC(node *z.TestNode, actionMap map[string]ActionFunc) error {
	fmt.Println("Enter the expression: ")
	expr := ""
	fmt.Scanln(&expr)

	printData("Your balance is %f\n", node.BCGetBalance())
	fmt.Println("Enter the budget: ")
	budgetStr := ""
	fmt.Scanln(&budgetStr)
	budget, err := strconv.ParseFloat(budgetStr, 64)
	if err != nil {
		return err
	}

	// local check for efficiency
	block := node.BCGetLatestBlock()
	if block == nil {
		printError(fmt.Errorf("blockchain not initialized"))
		return nil
	}
	_, total, err := permissioned.CalculateTotalPrice(block.States, expr)
	if err != nil {
		printError(err)
		return nil
	}
	if budget < total {
		printError(fmt.Errorf("budget not enough"))
		return nil
	}

	balance := node.BCGetBalance()
	if balance < total {
		printError(fmt.Errorf("balance not enough"))
		return nil
	}

	// calculate
	value, err := node.Calculate(expr, budget)
	if err != nil {
		printError(fmt.Errorf("calculation fails: %s", err))
		return nil
	}
	printData("The result of %s is %d\n", expr, value)
	printData("Your balance is %f\nPlease check balance later for the automatical deposit claim\n", node.BCGetBalance())

	return nil
}

func showAssets(node *z.TestNode, actionMap map[string]ActionFunc) error {
	records := node.GetAllPeerAssetPrices()

	hasValue := false
	for addr, record := range records {
		hasValue = true
		printData("Peer %s: \n", addr)
		for key, price := range record {
			printData("- Value: %s\t Price:%f\n", key, price)
		}
		printData("\n")
	}
	if !hasValue {
		printData("No assets in the network :(")
	}

	return nil
}

func addAsset(node *z.TestNode, actionMap map[string]ActionFunc) error {
	fmt.Println("Enter the key: ")
	key := ""
	fmt.Scanln(&key)

	fmt.Println("Enter the value: ")
	valueStr := ""
	fmt.Scanln(&valueStr)
	value, err := strconv.ParseInt(valueStr, 10, 32)
	if err != nil {
		return err
	}

	fmt.Println("Enter the price: ")
	priceStr := ""
	fmt.Scanln(&priceStr)
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return err
	}

	err = node.SetValueDBAsset(key, int(value), price)
	if err != nil {
		printError(fmt.Errorf("fail to set value: %s", err))
	}
	return nil
}

func addPeer(node *z.TestNode, actionMap map[string]ActionFunc) error {
	fmt.Println("Enter the peer IP address: ")
	addr := ""
	fmt.Scanln(&addr)

	node.AddPeer(addr)
	return nil
}

func getEnckey(node *z.TestNode, actionMap map[string]ActionFunc) error {
	pub, err := node.GetPubkeyString()
	if err != nil {
		return err
	}

	printData("Enc key: ", pub)
	return nil
}

func getBCBalance(node *z.TestNode, actionMap map[string]ActionFunc) error {
	printData("Your balance is %f\n", node.BCGetBalance())
	return nil
}

func refresh(node *z.TestNode, actionMap map[string]ActionFunc) error {
	return nil
}

// -----------------------------------------------------------------------------
// Blockchain Prompt

var NUM_BLOCKS_PER_PAGE = 5

var blockchainBasicPrompt = []string{
	ReturnPrevious,
	Exit,
}

func getBCInfo(node *z.TestNode, actionMap map[string]ActionFunc) error {
	// fmt.Println(node.BCSprintBlockchain())
	block := node.BCGetLatestBlock()
	if block == nil {
		return fmt.Errorf("fail to get blockchain info")
	}

	generateBlockPrompt := func(block *permissioned.Block) string {
		return fmt.Sprintf("🍀 Show {Height: %d, Hash: %s, NumTxn: %d}", block.Height, block.Hash(), len(block.Transactions))
	}

	opts := []string{}
	blockMap := map[string]*permissioned.Block{}

	for {

		// get blocks
		for i := 0; i < NUM_BLOCKS_PER_PAGE; i++ {
			if block == nil {
				break
			}
			opt := generateBlockPrompt(block)
			opts = append(opts, opt)
			blockMap[opt] = block
			block = node.BCGetBlock(block.PrevHash)
		}
		tempOpts := opts
		if block != nil {
			tempOpts = append(tempOpts, NextBlockchainPage)
		}
		tempOpts = append(tempOpts, blockchainBasicPrompt...)

		prompt := &survey.Select{
			Message: "What do you want to do ?",
			Options: tempOpts,
		}

		var action string
		err := survey.AskOne(prompt, &action)
		if err != nil {
			return err
		}

		if action == ReturnPrevious {
			return nil
		}
		if action == NextBlockchainPage {
			continue
		}
		if curBlock, ok := blockMap[action]; ok {
			err = getBlockDetails(curBlock, node, actionMap)
			if err != nil {
				printError(err)
			}
			continue
		}

		method := actionMap[action]
		err = method(node, actionMap)
		if err != nil {
			printError(err)
		}
	}
}

var NUM_TXN_PER_PAGE = 5

func getBlockDetails(block *permissioned.Block, node *z.TestNode, actionMap map[string]ActionFunc) error {
	printData("Block %s:\n", block.Hash())
	printData("- PreHash: %s\n", block.PrevHash)
	printData("- Height: %s\n", block.Hash())
	printData("- Miner: %s\n", block.Miner)
	printData("- StateHash: %s\n", block.StateHash)
	printData("- TxnHash: %s\n", block.TransationHash)
	printData("\n")

	generateTxnPrompt := func(txn *permissioned.SignedTransaction) string {
		return fmt.Sprintf("🍀 Show {TxnID: %s, Type: %s}", txn.Txn.ID, txn.Txn.Type)
	}

	opts := []string{}
	txnMap := map[string]*permissioned.SignedTransaction{}
	i := 0

	for {
		// get blocks
		for ; i < NUM_TXN_PER_PAGE && i < len(block.Transactions); i++ {
			opt := generateTxnPrompt(&block.Transactions[i])
			opts = append(opts, opt)
			txnMap[opt] = &block.Transactions[i]
		}
		tempOpts := opts
		if i < len(block.Transactions) {
			tempOpts = append(tempOpts, NextBlockchainPage)
		}
		tempOpts = append(tempOpts, blockchainBasicPrompt...)

		prompt := &survey.Select{
			Message: "What do you want to do ?",
			Options: tempOpts,
		}

		var action string
		err := survey.AskOne(prompt, &action)
		if err != nil {
			return err
		}

		if action == ReturnPrevious {
			return nil
		}
		if action == NextBlockchainPage {
			continue
		}
		if txn, ok := txnMap[action]; ok {
			for {
				printData("Txn %s:\n", txn.Txn.ID)
				printData("- Type: %s\n", txn.Txn.Type)
				printData("- Nonce: %d\n", txn.Txn.Nonce)
				printData("- From: %s\n", txn.Txn.From)
				printData("- To: %s\n", txn.Txn.To)
				printData("- Value: %f\n", txn.Txn.Value)
				if txn.Txn.Data != nil {
					switch vv := txn.Txn.Data.(type) {
					case permissioned.Describable:
						printData("\t Data: %s\n", vv.String())
					default:
						printData("\t Data: %#v\n", vv)
					}
				}

				prompt := &survey.Select{
					Options: blockchainBasicPrompt,
				}
				err := survey.AskOne(prompt, &action)
				if err != nil {
					return err
				}

				if action == ReturnPrevious {
					return nil
				}
				method := actionMap[action]
				err = method(node, actionMap)
				if err != nil {
					printError(err)
				}
			}
		}
		method := actionMap[action]
		err = method(node, actionMap)
		if err != nil {
			printError(err)
		}
	}
}
