# firehose-cardano

A Firehose instrumentation for the Cardano blockchain, providing unified tools for fetching and processing Cardano blocks.

## Overview

This project provides a unified CLI (`firecardano`) that includes:

- **blockfetcher**: Connects to Cardano nodes and fetches blocks in Firehose format
- **console-reader**: Parses Firehose FIRE BLOCK lines and outputs structured JSON

## Installation

### Build from source

```bash
git clone https://github.com/no-witness-labs/firehose-cardano.git
cd firehose-cardano
make build
```

This will create the `bin/firecardano` binary.

## Usage

### CLI Commands

#### Blockfetcher

Fetch blocks from a Cardano network:

```bash
# Basic usage
./bin/firecardano blockfetcher

# With environment variables
BLOCK_FETCH_NETWORK=mainnet \
BLOCK_FETCH_ADDRESS=backbone.cardano.iog.io:3001 \
BLOCK_FETCH_PIPELINE_LIMIT=10 \
./bin/firecardano blockfetcher
```

#### Console Reader

Parse Firehose FIRE BLOCK lines from stdin:

```bash
# Basic usage
./bin/firecardano console-reader

# In a pipeline (typical usage)
./bin/firecardano blockfetcher | ./bin/firecardano console-reader
```

### Environment Variables

The blockfetcher subcommand supports these environment variables:

- `BLOCK_FETCH_ADDRESS`: Cardano node address (default: `backbone.cardano.iog.io:3001`)
- `BLOCK_FETCH_NETWORK`: Network name - `mainnet`, `preview`, or `preprod` (default: `mainnet`)
- `BLOCK_FETCH_NETWORK_MAGIC`: Network magic number (default: auto-detect from network)
- `BLOCK_FETCH_PIPELINE_LIMIT`: Pipeline limit for block fetching (default: `10`)

### Makefile Targets

```bash
# Build the CLI
make build

# Run blockfetcher
make run-blockfetcher

# Run console-reader
make run-console-reader

# Run on mainnet
make run-mainnet

# Run on testnet
make run-testnet

# Run tests
make test

# Clean build artifacts
make clean
```

## Development

### Local Development Environment

Use the development scripts for local testing:

```bash
# Build and run the pipeline
./devel/localnet/start.sh

# Build only (no execution)
./devel/localnet/start.sh -b

# Stop the pipeline
./devel/localnet/stop.sh
```

The development environment creates a pipeline that fetches blocks and processes them through the console reader, with logs stored in `devel/localnet/logs/`.

### Project Structure

```text
cmd/firecardano/           # Main CLI application
├── main.go               # CLI entry point
└── cli/                  # CLI subcommands
    ├── root.go          # Root command definition
    ├── blockfetcher.go  # Blockfetcher subcommand
    └── console_reader.go # Console-reader subcommand

devel/localnet/           # Development environment
├── start.sh             # Start development pipeline
└── stop.sh              # Stop development pipeline

codec/                    # Block parsing and processing
types/pb/cardano/         # Protocol buffer definitions
```
