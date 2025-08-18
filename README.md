# firehose-cardano

A Firehose instrumentation for the Cardano blockchain, providing unified tools for fetching and processing Cardano blocks.

![firehose-cardano.gif](/docs/images/firehose-cardano.gif)

## Overview

This project provides a unified CLI (`firecardano`) that includes:

- **blockfetcher**: Connects to Cardano nodes and fetches blocks in Firehose format
- **console-reader**: Parses Firehose FIRE BLOCK lines and outputs structured JSON
- **Firehose gRPC API**: Streams Cardano blocks via gRPC with proper protobuf types

## Features

- ✅ **Real-time block streaming** from Cardano mainnet, testnet, and preview networks
- ✅ **Proper protobuf integration** with `sf.cardano.type.v1.Block` types
- ✅ **gRPC API** for programmatic access to Cardano block data
- ✅ **Development environment** with local pipeline testing
- ✅ **Multiple network support** (mainnet, preview, preprod)

## Installation

### Prerequisites

- Go 1.21 or later
- Protocol Buffers compiler (`protoc`)
- `protoc-gen-go` plugin

```bash
# Install protoc (macOS)
brew install protobuf

# Install protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

### Build from source

```bash
git clone https://github.com/no-witness-labs/firehose-cardano.git
cd firehose-cardano

# Generate protobuf types
make gen-proto

# Build the CLI
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

#### Firehose gRPC API

Start the Firehose pipeline to serve blocks via gRPC:

```bash
# Start the full Firehose pipeline
./devel/localnet/start.sh
```

The gRPC server will be available on `localhost:10010` with these services:

- `sf.bstream.v1.BlockStream/Blocks` - Stream Cardano blocks
- `sf.headinfo.v1.HeadInfo/GetHeadInfo` - Get current head block info

**Query examples using grpcurl:**

```bash
# Get current head block information
grpcurl -plaintext localhost:10010 sf.headinfo.v1.HeadInfo/GetHeadInfo

# Stream the latest 5 blocks
grpcurl -plaintext -d '{"burst": 5}' localhost:10010 sf.bstream.v1.BlockStream/Blocks

# List available services
grpcurl -plaintext localhost:10010 list
```

**Example response:**

```json
{
  "number": "12273301",
  "id": "f3941431064f41cd785d52d42c39036c4ab53c682964f6d270843ef926138613",
  "parentId": "9e8255512dc50e33f9b8f38d65ad11758640c1e7b93124c56d4b2126d6ea6f3a",
  "timestamp": "2025-08-18T10:36:28Z",
  "libNum": "12273193",
  "parentNum": "12273300",
  "payload": {
    "@type": "type.googleapis.com/sf.cardano.type.v1.Block",
    // Cardano block data with transactions, metadata, etc.
  }
}
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
