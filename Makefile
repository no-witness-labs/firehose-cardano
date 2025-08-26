# Makefile for Firehose Cardano

.PHONY: build run clean test lint fmt run-blockfetcher run-console-reader run-mainnet run-testnet gen-proto build-substreams pack-substreams

# Generate protobuf Go files
gen-proto:
	@echo "Generating protobuf Go files..."
	@mkdir -p types/pb
	protoc --proto_path=proto --go_out=types/pb --go_opt=paths=source_relative proto/sf/cardano/type/v1/type.proto

# Build all the CLI
build: build-blockfetcher build-firecardano

# Build the blockfetcher CLI
build-blockfetcher:
	@echo "Building blockfetcher CLI..."
	go build -o bin/blockfetcher ./cmd/blockfetcher

# Build the firecardano CLI
build-firecardano:
	@echo "Building firecardano CLI..."
	go build -o bin/firecardano ./cmd/firecardano

# Run the blockfetcher subcommand
run-blockfetcher: build
	@echo "Running blockfetcher..."
	./bin/firecardano blockfetcher

# Run the console-reader subcommand
run-console-reader: build
	@echo "Running console-reader..."
	./bin/firecardano console-reader

# Legacy target for backward compatibility
run: run-blockfetcher

# Run blockfetcher with mainnet configuration
run-mainnet: build
	@echo "Running blockfetcher on mainnet..."
	./bin/blockfetcher -config blockfetcher.toml

# Run blockfetcher with testnet configuration
run-testnet: build
	@echo "Running blockfetcher on testnet..."
	./bin/blockfetcher -config blockfetcher-testnet.toml

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Lint code
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Build substreams Rust module
build-substreams:
	@echo "Building substreams Rust module..."
	cd substreams && cargo build --release --target wasm32-unknown-unknown

# Pack substreams
pack-substreams: build-substreams
	@echo "Packing substreams..."
	substreams pack substreams/substreams.yaml

# Create bin directory
bin:
	mkdir -p bin
