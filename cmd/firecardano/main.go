package main

import (
	pbcardano "github.com/no-witness-labs/firehose-cardano/types/pb/sf/cardano/type/v1"
	firecore "github.com/streamingfast/firehose-core"
	fhCMD "github.com/streamingfast/firehose-core/cmd"
	info "github.com/streamingfast/firehose-core/firehose/info"
)

func main() {
	var version = "dev"
	firecore.UnsafeRunningFromFirecore = true
	firecore.UnsafeAllowExecutableNameToBeEmpty = true

	fhCMD.Main(&firecore.Chain[*pbcardano.Block]{
		ShortName:            "cardano",
		LongName:             "Cardano",
		FullyQualifiedModule: "github.com/no-witness-labs/firehose-cardano",
		Version:              version,
		BlockFactory:         func() firecore.Block { return new(pbcardano.Block) },
		ConsoleReaderFactory: firecore.NewConsoleReader,
		InfoResponseFiller:   info.DefaultInfoResponseFiller,
		Tools:                &firecore.ToolsConfig[*pbcardano.Block]{},
	})
}
