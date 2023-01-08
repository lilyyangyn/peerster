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
	addCliCmd(command)
	addDaemonCmd(command)

	err := command.Execute()
	if err != nil {
		panic(err)
	}
}

// addCliCmd starts a node with customization
func addCliCmd(command *cobra.Command) {
	var port int
	// var opts []z.Option

	startCmd := &cobra.Command{
		Use:   "cli",
		Short: "Start a MPCPeer with interactive CLI",
		Long:  "Start a MPCPeer with interactive CLI, init blockchain and perform MPC requests",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
			cli.StartCMD(port, false)
		},
	}

	startCmd.Flags().IntVarP(&port, "port", "p", 0, "Start node on a customized port")

	command.AddCommand(startCmd)
}

// addStartCmd starts a node with customization
func addDaemonCmd(command *cobra.Command) {
	var port int
	// var opts []z.Option

	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Start a MPCPeer as daemon",
		Long:  "Start a MPCPeer as daemon, init blockchain and perform MPC requests",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
			cli.StartCMD(port, true)
		},
	}

	daemonCmd.Flags().IntVarP(&port, "port", "p", 0, "Start node on a customized port")

	command.AddCommand(daemonCmd)
}
