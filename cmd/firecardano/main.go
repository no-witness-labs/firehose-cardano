package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/no-witness-labs/firehose-cardano/cmd/firecardano/cli"
)

// Commit sha1 value, injected via go build `ldflags` at build time
var commit = ""

// Version value, injected via go build `ldflags` at build time
var version = "dev"

// Date value, injected via go build `ldflags` at build time
var date = time.Now().Format(time.RFC3339)

func init() {
	cli.RootCmd.Version = versionString()
}

func main() {
	if err := cli.RootCmd.Execute(); err != nil {
		// Use fmt.Printf instead of logger since we don't want to set up logging in main
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func versionString() string {
	var labels []string
	if len(commit) >= 7 {
		labels = append(labels, fmt.Sprintf("Commit %s", commit[0:7]))
	}

	if date != "" {
		labels = append(labels, fmt.Sprintf("Built %s", date))
	}

	if len(labels) == 0 {
		return version
	}

	return fmt.Sprintf("%s (%s)", version, strings.Join(labels, ", "))
}
