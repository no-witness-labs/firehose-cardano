package cli

import (
	"github.com/spf13/cobra"
	"github.com/streamingfast/dlauncher/flags"
	"github.com/streamingfast/logging"
)

var (
	zlog, _  = logging.RootLogger("firecardano", "github.com/no-witness-labs/firehose-cardano/cmd/firecardano")
	allFlags = map[string]bool{}

	RootCmd = &cobra.Command{
		Use:   "firecardano",
		Short: "Firehose services for Cardano blockchains",
		// Version:  // set by cmd/main.go
	}
)

func Main() {
	cobra.OnInitialize(func() {
		allFlags = flags.AutoBind(RootCmd, "firecardano")
	})
}
