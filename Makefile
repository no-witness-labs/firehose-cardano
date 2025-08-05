# Makefile for Cardano Block Fetcher

.PHONY: build run clean test lint fmt

# Build the block fetcher
build:
	@echo "Building block fetcher..."
	cd blockfetcher && go build -o ../bin/blockfetcher ./main.go

# Run the block fetcher
run: build
	@echo "Running block fetcher..."
	./bin/blockfetcher

# Run with environment variables
run-mainnet:
	@echo "Running block fetcher on mainnet..."
	BLOCK_FETCH_NETWORK=mainnet BLOCK_FETCH_ADDRESS=backbone.cardano.iog.io:3001 ./bin/blockfetcher

run-testnet:
	@echo "Running block fetcher on testnet..."
	BLOCK_FETCH_NETWORK=testnet BLOCK_FETCH_ADDRESS=backbone.cardano-testnet.iohkdev.io:3001 ./bin/blockfetcher

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
