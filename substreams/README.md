# Cardano Substreams

This Substreams package provides modules for extracting and transforming Cardano blockchain data.

## Modules

### `map_blocks`

Maps Cardano blocks to their basic information including header, body, and timestamp data.

**Input:** `sf.substreams.v1.Blocks`
**Output:** `sf.cardano.type.v1.Block`

## Usage

```bash
# Run the substreams against a Firehose endpoint
substreams run cardano-v0.1.0.spkg map_blocks -e <firehose-endpoint>

# Example with local endpoint
substreams run cardano-v0.1.0.spkg map_blocks -e localhost:10010
```

## Development

To build and package:

```bash
# Build the Rust module
cargo build --target wasm32-unknown-unknown --release

# Package the substreams
substreams pack substreams.yaml
```

## Requirements

- Rust with wasm32-unknown-unknown target
- Substreams CLI
- Access to a Cardano Firehose endpoint
