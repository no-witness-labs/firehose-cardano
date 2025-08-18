package cli

import (
	"github.com/spf13/cobra"
	"github.com/streamingfast/dlauncher/flags"
)

var (
	allFlags = map[string]bool{}

	RootCmd = &cobra.Command{
		Use:   "firecardano",
		Short: "Firehose services for Cardano blockchains",
		// Version:  // set by cmd/main.go
	}
)

func init() {
	cobra.OnInitialize(func() {
		allFlags = flags.AutoBind(RootCmd, "firecardano")
	})

	// Add subcommands
	RootCmd.AddCommand(consoleReaderCmd)
	RootCmd.AddCommand(blockfetcherCmd)
}
