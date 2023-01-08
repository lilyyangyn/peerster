package main

import (
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	cli "go.dedis.ch/cs438/cmd"
)

func main() {
	command := &cobra.Command{
		Use: "mpcpeer",
	}
	addStartCmd(command)

	err := command.Execute()
	if err != nil {
		panic(err)
	}
}

// addStartCmd starts a node with customization
func addStartCmd(command *cobra.Command) {
	// var opts []z.Option
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a MPCPeer",
		Long:  "Start a MPCPeer, init blockchain and perform MPC requests",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
			cli.StartCMD(false)
		},
	}
	command.AddCommand(startCmd)
}
