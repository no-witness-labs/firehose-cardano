package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	ouroboros "github.com/blinklabs-io/gouroboros"
	"github.com/blinklabs-io/gouroboros/ledger"
	"github.com/blinklabs-io/gouroboros/protocol/chainsync"
	"github.com/blinklabs-io/gouroboros/protocol/common"
	"google.golang.org/protobuf/proto"
)

type BlockFetcherConfig struct {
	Address       string `toml:"address"`
	SocketPath    string `toml:"socket_path"`
	Network       string `toml:"network"`
	NetworkMagic  uint32 `toml:"network_magic"`
	PipelineLimit uint32 `toml:"pipeline_limit"`
	StartSlot     uint64 `toml:"start_slot"`
	StartHash     string `toml:"start_hash"`
}

func (c *BlockFetcherConfig) setDefaults() {
	if c.Address == "" && c.SocketPath == "" {
		c.Address = "backbone.cardano.iog.io:3001"
	}
	if c.Network == "" {
		c.Network = "mainnet"
	}
	if c.PipelineLimit == 0 {
		c.PipelineLimit = 10
	}
}

type SlotConfig struct {
	ZeroTime   int64
	ZeroSlot   uint64
	SlotLength int64
}

var SlotConfigNetwork = map[string]SlotConfig{
	"mainnet": {ZeroTime: 1596059091000, ZeroSlot: 4492800, SlotLength: 1000},
	"preview": {ZeroTime: 1666656000000, ZeroSlot: 0, SlotLength: 1000},
	"preprod": {ZeroTime: 1654041600000 + 1728000000, ZeroSlot: 86400, SlotLength: 1000},
}

func slotToBeginUnixTime(slot uint64, slotConfig SlotConfig) int64 {
	msAfterBegin := int64(slot-slotConfig.ZeroSlot) * slotConfig.SlotLength
	return slotConfig.ZeroTime + msAfterBegin
}

type FirehoseInstrumentation struct {
	blockTypeURL string
	logger       *log.Logger
	slotConfig   SlotConfig
}

func NewFirehoseInstrumentation(blockTypeURL string, logger *log.Logger, slotConfig SlotConfig) *FirehoseInstrumentation {
	return &FirehoseInstrumentation{
		blockTypeURL: blockTypeURL,
		logger:       logger,
		slotConfig:   slotConfig,
	}
}

func (f *FirehoseInstrumentation) Init() {
	fmt.Printf("FIRE INIT 3.0 %s\n", f.blockTypeURL)
}

func (f *FirehoseInstrumentation) OutputBlock(block ledger.Block) error {
	blockNumber := block.BlockNumber()
	blockHash := block.Hash()
	parentNumber := blockNumber - 1
	parentHash := block.Header().PrevHash()
	libNum := blockNumber - 108
	timestampMs := slotToBeginUnixTime(block.SlotNumber(), f.slotConfig)
	timestamp := timestampMs * 1000000

	blockData, err := f.serializeBlock(block)
	if err != nil {
		return fmt.Errorf("failed to serialize block: %w", err)
	}
	encodedData := base64.StdEncoding.EncodeToString(blockData)

	fireBlock := fmt.Sprintf(
		"FIRE BLOCK %d %s %d %s %d %d %s",
		blockNumber,  // Block slot number
		blockHash,    // Block hash
		parentNumber, // Parent block slot number
		parentHash,   // Parent block hash
		libNum,       // Last irreversible block number
		timestamp,    // Block timestamp (nanoseconds)
		encodedData,  // Base64 encoded block payload
	)
	fmt.Println(fireBlock)
	return nil
}

func (f *FirehoseInstrumentation) serializeBlock(block ledger.Block) ([]byte, error) {
	utxoBlock, err := block.Utxorpc()
	if err != nil {
		return nil, fmt.Errorf("failed to get UTXO RPC: %w", err)
	}
	data, err := proto.Marshal(utxoBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal UTXO block: %w", err)
	}
	return data, nil
}

type BlockFetcher struct {
	config     *BlockFetcherConfig
	connection *ouroboros.Connection
	logger     *log.Logger
	slogger    *slog.Logger
	firehose   *FirehoseInstrumentation
	slotConfig SlotConfig
}

func NewBlockFetcher(cfg *BlockFetcherConfig, logger *log.Logger) *BlockFetcher {
	slotConfig, exists := SlotConfigNetwork[cfg.Network]
	if !exists {
		slotConfig = SlotConfigNetwork["mainnet"]
		logger.Printf("Warning: Unknown network '%s', defaulting to mainnet slot config", cfg.Network)
	}

	firehose := NewFirehoseInstrumentation("type.googleapis.com/sf.cardano.type.v1.Block", logger, slotConfig)

	slogger := slog.Default()

	return &BlockFetcher{
		config:     cfg,
		logger:     logger,
		slogger:    slogger,
		firehose:   firehose,
		slotConfig: slotConfig,
	}
}

func loadConfig(configPath string) (*BlockFetcherConfig, error) {
	cfg := &BlockFetcherConfig{}

	if configPath != "" {
		if _, err := toml.DecodeFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to decode TOML config file %s: %w", configPath, err)
		}
	}

	// Set defaults for any unspecified values
	cfg.setDefaults()

	return cfg, nil
}

func (bf *BlockFetcher) processBlock(block ledger.Block) error {
	if err := bf.firehose.OutputBlock(block); err != nil {
		return err
	}
	return nil
}

func (bf *BlockFetcher) resolveNetworkMagic() error {
	if bf.config.NetworkMagic == 0 {
		fmt.Println("Resolving network magic...", bf.config.Network)
		network, ok := ouroboros.NetworkByName(bf.config.Network)
		if !ok {
			return fmt.Errorf("invalid network specified: %s", bf.config.Network)
		}
		bf.config.NetworkMagic = network.NetworkMagic
	}
	return nil
}

func (bf *BlockFetcher) getStartPoint() (common.Point, error) {
	// If both start slot and hash are provided, use them
	if bf.config.StartSlot != 0 && bf.config.StartHash != "" {
		hash, err := hex.DecodeString(bf.config.StartHash)
		if err != nil {
			return common.Point{}, fmt.Errorf("failed to decode start hash: %w", err)
		}
		point := common.NewPoint(bf.config.StartSlot, hash)
		bf.logger.Printf("Using configured start point: slot=%d, hash=%s", bf.config.StartSlot, bf.config.StartHash)
		return point, nil
	}

	// Otherwise, get current tip
	tip, err := bf.connection.ChainSync().Client.GetCurrentTip()
	if err != nil {
		return common.Point{}, fmt.Errorf("failed to get current tip: %w", err)
	}
	bf.logger.Printf("Using current tip as start point: %#v", tip)
	return tip.Point, nil
}

func (bf *BlockFetcher) chainSyncRollForwardHandler(
	ctx chainsync.CallbackContext,
	blockType uint,
	blockData any,
	tip chainsync.Tip,
) error {
	var block ledger.Block
	switch v := blockData.(type) {
	case ledger.Block:
		block = v
	case ledger.BlockHeader:
		blockSlot := v.SlotNumber()
		blockHash := v.Hash().Bytes()
		var err error
		if bf.connection == nil {
			return fmt.Errorf("ouroboros connection is nil")
		}
		block, err = bf.connection.BlockFetch().Client.GetBlock(
			common.NewPoint(blockSlot, blockHash),
		)
		if err != nil {
			return fmt.Errorf("failed to fetch block: %w", err)
		}
	default:
		return fmt.Errorf("unexpected block data type: %T", blockData)
	}

	if err := bf.processBlock(block); err != nil {
		return fmt.Errorf("failed to process block: %w", err)
	}

	return nil
}

func (bf *BlockFetcher) chainSyncRollBackwardHandler(
	ctx chainsync.CallbackContext,
	point common.Point,
	tip chainsync.Tip,
) error {
	bf.logger.Printf("ChainSync roll backward: point = %#v, tip = %#v", point, tip)
	return nil
}

func (bf *BlockFetcher) buildChainSyncConfig() chainsync.Config {
	return chainsync.NewConfig(
		chainsync.WithRollForwardFunc(bf.chainSyncRollForwardHandler),
		chainsync.WithRollBackwardFunc(bf.chainSyncRollBackwardHandler),
		chainsync.WithPipelineLimit(int(bf.config.PipelineLimit)),
	)
}

func (bf *BlockFetcher) connect(ctx context.Context) error {
	if err := bf.resolveNetworkMagic(); err != nil {
		return err
	}

	var protocol, address string
	var isN2N bool
	if bf.config.Address != "" {
		protocol, address = "tcp", bf.config.Address
		isN2N = true
	} else if bf.config.SocketPath != "" {
		protocol, address = "unix", bf.config.SocketPath
	}

	bf.logger.Printf("Connecting to [%s] %s (network magic: %d)", protocol, address, bf.config.NetworkMagic)

	errorChan := make(chan error, 1)

	go func() {
		for {
			select {
			case err := <-errorChan:
				if err != nil {
					bf.logger.Printf("Connection error: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	conn, err := ouroboros.NewConnection(
		ouroboros.WithNetworkMagic(bf.config.NetworkMagic),
		ouroboros.WithErrorChan(errorChan),
		ouroboros.WithNodeToNode(isN2N),
		ouroboros.WithKeepAlive(true),
		ouroboros.WithChainSyncConfig(bf.buildChainSyncConfig()),
		ouroboros.WithLogger(bf.slogger),
	)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	if err := conn.Dial(protocol, address); err != nil {
		return fmt.Errorf("failed to dial [%s] %s: %w", protocol, address, err)
	}

	bf.connection = conn
	bf.logger.Printf("Successfully connected to %s", bf.config.Address)
	return nil
}

func (bf *BlockFetcher) start(ctx context.Context) error {
	if bf.connection == nil {
		return fmt.Errorf("not connected")
	}

	bf.logger.Println("Starting chain sync...")

	point, err := bf.getStartPoint()
	if err != nil {
		return fmt.Errorf("failed to get start point: %w", err)
	}

	// Start chain sync from the determined point
	if err := bf.connection.ChainSync().Client.Sync([]common.Point{point}); err != nil {
		return fmt.Errorf("chain sync failed: %w", err)
	}

	// Wait for context cancellation (shutdown signal)
	<-ctx.Done()
	bf.logger.Println("Context cancelled, stopping chain sync...")

	return ctx.Err()
}

func (bf *BlockFetcher) close() error {
	if bf.connection != nil {
		bf.logger.Println("Closing connection...")
		if err := bf.connection.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
		bf.connection = nil
	}
	return nil
}

func (bf *BlockFetcher) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		bf.logger.Printf("Received signal %v, shutting down gracefully...", sig)
		cancel()
	}()

	if err := bf.connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer func() {
		if err := bf.close(); err != nil {
			bf.logger.Printf("Error during cleanup: %v", err)
		}
	}()

	return bf.start(ctx)
}

func main() {
	logger := log.New(os.Stderr, "[BlockFetcher] ", log.LstdFlags|log.Lshortfile)

	configPath := flag.String("config", "", "Path to TOML configuration file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	logger.Printf("Starting Cardano Block Fetcher: Address=%s, Network=%s, NetworkMagic=%d, PipelineLimit=%d, StartSlot=%d, StartHash=%s",
		cfg.Address, cfg.Network, cfg.NetworkMagic, cfg.PipelineLimit, cfg.StartSlot, cfg.StartHash)

	fetcher := NewBlockFetcher(cfg, logger)

	fetcher.firehose.Init()

	if err := fetcher.Run(); err != nil && err != context.Canceled {
		logger.Fatalf("Block fetcher failed: %v", err)
	}

	logger.Println("Block fetcher shutdown complete")
}
