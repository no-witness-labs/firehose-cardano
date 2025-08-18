package cli

import (
	"github.com/spf13/cobra"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	firecore "github.com/streamingfast/firehose-core"
	fhCMD "github.com/streamingfast/firehose-core/cmd"
	info "github.com/streamingfast/firehose-core/firehose/info"
)

var consoleReaderCmd = &cobra.Command{
	Use:   "console-reader",
	Short: "Read and parse Cardano blockchain data from console input",
	Long:  "Reads FIRE BLOCK lines from stdin and outputs parsed block information as JSON",
	RunE:  consoleReaderE,
}

func consoleReaderE(cmd *cobra.Command, args []string) error {
	var version = "dev"
	firecore.UnsafeRunningFromFirecore = true
	firecore.UnsafeAllowExecutableNameToBeEmpty = true

	fhCMD.Main(&firecore.Chain[*pbbstream.Block]{
		ShortName:            "console-reader",
		LongName:             "Cardano",
		FullyQualifiedModule: "github.com/no-witness-labs/firehose-cardano",
		Version:              version,
		BlockFactory:         func() firecore.Block { return new(pbbstream.Block) },
		ConsoleReaderFactory: firecore.NewConsoleReader,
		InfoResponseFiller:   info.DefaultInfoResponseFiller,
		Tools:                &firecore.ToolsConfig[*pbbstream.Block]{},
	})
	return nil
}

// func consoleReaderE(cmd *cobra.Command, args []string) error {
// 	logger, _ := zap.NewDevelopment()
// 	defer logger.Sync()

// 	logger.Info("Starting console reader")

// 	// Create a dummy tracer for firehose-core (it expects one)
// 	_, tracer := logging.PackageLogger("console-reader", "github.com/no-witness-labs/firehose-cardano")

// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	sigCh := make(chan os.Signal, 1)
// 	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
// 	go func() {
// 		<-sigCh
// 		logger.Info("received shutdown signal")
// 		cancel()
// 	}()

// 	lines := make(chan string, 1000)

// 	consoleReader, err := firecore.NewConsoleReader(lines, nil, logger, tracer)
// 	if err != nil {
// 		return fmt.Errorf("creating console reader: %w", err)
// 	}

// 	go func() {
// 		defer close(lines)
// 		scanner := bufio.NewScanner(os.Stdin)
// 		for scanner.Scan() {
// 			select {
// 			case lines <- scanner.Text():
// 			case <-ctx.Done():
// 				return
// 			}
// 		}
// 	}()

// 	logger.Info("console reader started, waiting for FIRE BLOCK lines on stdin")
// 	printTicker := time.NewTicker(30 * time.Second)
// 	defer printTicker.Stop()

// 	// Process blocks from console reader
// 	go func() {
// 		for {
// 			block, err := consoleReader.ReadBlock()
// 			// consoleReader.printStats()
// 			if err != nil {
// 				logger.Error("error reading block", zap.Error(err))
// 				return
// 			}
// 			if block != nil {
// 				logger.Info("received block",
// 					zap.Uint64("number", block.Number),
// 					zap.String("id", block.Id),
// 					zap.Uint64("parent_number", block.ParentNum),
// 					zap.String("parent_id", block.ParentId),
// 				)
// 			}
// 		}
// 	}()

// 	for {
// 		select {
// 		case <-printTicker.C:
// 			logger.Info("still running - waiting for more blocks")
// 		case <-ctx.Done():
// 			logger.Info("context done, shutting down")
// 			return nil
// 		case <-consoleReader.Done():
// 			logger.Info("console reader done, exiting")
// 			return nil
// 		}
// 	}
// }
