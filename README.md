# firehose-cardano

A Firehose instrumentation for the Cardano blockchain, providing unified tools for fetching and processing Cardano blocks.



## Overview

This project provides a unified CLI (`firecardano`) that includes:

- **blockfetcher**: Connects to Cardano nodes and fetches blocks in Firehose format
- **console-reader**: Parses Firehose FIRE BLOCK lines and outputs structured JSON
- **Firehose gRPC API**: Streams Cardano blocks via gRPC with proper protobuf types

![firehose-cardano.gif](/docs/images/firehose-cardano.gif)
## Features

- ✅ **Real-time block streaming** from Cardano mainnet, testnet, and preview networks
- ✅ **Proper protobuf integration** with `sf.cardano.type.v1.Block` types
- ✅ **gRPC API** for programmatic access to Cardano block data
- ✅ **Development environment** with local pipeline testing
- ✅ **Multiple network support** (mainnet, preview, preprod)

## Installation

### Prerequisites
- Go 1.21+
- `protoc` & `protoc-gen-go`
- Rust (for substreams)
- `substreams` CLI tool

### Build
```bash
git clone https://github.com/no-witness-labs/firehose-cardano.git
cd firehose-cardano
make gen-proto
make build
```

## How to Run

### Fetch blocks
```bash
# Mainnet (default)
./bin/blockfetcher -address=backbone.cardano.iog.io:3001 -network=mainnet

# Preview testnet
./bin/blockfetcher -address=backbone.cardano-preview.iog.io:3001 -network=preview -network-magic=2

# Preprod testnet
./bin/blockfetcher -address=backbone.cardano-preprod.iog.io:3001 -network=preprod -network-magic=1

# Local socket connection
./bin/blockfetcher -socket-path=/var/cardano/node.socket -network=mainnet

# Custom start point
./bin/blockfetcher -address=backbone.cardano.iog.io:3001 -start-slot=164636374 -start-hash=71a1e62336b566d31115dee65ed0a506b4bc10c2bbb7a37cedddb2d97dd31b1d

# All available options
./bin/blockfetcher -h
```

### Console reader
```bash
./bin/firecardano console-reader
# or pipe:
./bin/blockfetcher -address=backbone.cardano.iog.io:3001 | ./bin/firecardano console-reader
```

### Firehose gRPC API
```bash
./devel/localnet/start.sh
# gRPC: localhost:10010
```

### Substreams

#### Installation
```bash
# Install Rust (if not already installed)
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
rustup target add wasm32-unknown-unknown

# Install substreams CLI
curl -sSL https://github.com/streamingfast/substreams/releases/download/v1.10.6/substreams_linux_x86_64.tar.gz | tar -xz -C /tmp
sudo mv /tmp/substreams /usr/local/bin/
```

#### Build and Run
```bash
# Build the Rust WASM module
make build-substreams

# Pack the substreams
make pack-substreams

# Run the substream
substreams run -e 127.0.0.1:10016 substreams/cardano-v0.1.0.spkg map_blocks --stop-block 0 --plaintext
```
