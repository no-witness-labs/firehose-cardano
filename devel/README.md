# Development Environment (localnet)

This directory contains helper scripts to run a simple Firehose Cardano instrumentation pipeline locally.

## Scripts

- `localnet/start.sh` Build & launch the block fetcher piped into the console reader.
- `localnet/stop.sh` Stop the running pipeline.

## Usage

Build only:

```bash
./devel/localnet/start.sh -b
```

Run (will build then start):

```bash
./devel/localnet/start.sh
```

Custom configuration:

```bash
# Use a custom config file
CONFIG_FILE=blockfetcher-testnet.toml ./devel/localnet/start.sh

# Or create your own config file
CONFIG_FILE=my-custom-config.toml ./devel/localnet/start.sh
```

Logs:

- Block fetcher raw FIRE lines: `devel/localnet/logs/blockfetcher.log`
- Parsed blocks output: `devel/localnet/logs/console-reader.log`

Stop:

```bash
./devel/localnet/stop.sh
```
