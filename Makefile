# Makefile for Firehose Cardano

.PHONY: build run clean test lint fmt run-blockfetcher run-console-reader run-mainnet run-testnet gen-proto

# Generate protobuf Go files
gen-proto:
	@echo "Generating protobuf Go files..."
	@mkdir -p types/pb
	protoc --proto_path=proto --go_out=types/pb --go_opt=paths=source_relative proto/sf/cardano/type/v1/type.proto

# Build the firecardano CLI
build:
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

# Run blockfetcher with mainnet environment variables
run-mainnet: build
	@echo "Running blockfetcher on mainnet..."
	BLOCK_FETCH_NETWORK=mainnet BLOCK_FETCH_ADDRESS=backbone.cardano.iog.io:3001 ./bin/firecardano blockfetcher

# Run blockfetcher with testnet environment variables
run-testnet: build
	@echo "Running blockfetcher on testnet..."
	BLOCK_FETCH_NETWORK=testnet BLOCK_FETCH_ADDRESS=backbone.cardano-testnet.iohkdev.io:3001 ./bin/firecardano blockfetcher

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

# Create bin directory
bin:
	mkdir -p bin
