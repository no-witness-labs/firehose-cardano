package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	ouroboros "github.com/blinklabs-io/gouroboros"
	"github.com/blinklabs-io/gouroboros/ledger"
	"github.com/blinklabs-io/gouroboros/protocol/chainsync"
	"github.com/blinklabs-io/gouroboros/protocol/common"
	"google.golang.org/protobuf/proto"
)

type BlockFetcherConfig struct {
	Address       string
	SocketPath    string
	Network       string
	NetworkMagic  uint32
	PipelineLimit uint32
	StartSlot     uint64
	StartHash     string
	CursorFile    string
}

type CursorPoint struct {
	Slot uint64 `json:"slot"`
	Hash string `json:"hash"`
}

type CursorState struct {
	Points []CursorPoint `json:"points"`
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

func shouldStoreCursorPoint(currentSlot, baseSlot uint64) bool {
	if baseSlot == 0 {
		return true
	}

	distance := currentSlot - baseSlot

	return distance <= 10 ||
		(distance <= 1000 && distance%10 == 0) ||
		distance%100 == 0
}

func (bf *BlockFetcher) addCursorPoint(point common.Point) {
	if bf.baseSlot == 0 {
		bf.baseSlot = point.Slot
		bf.cursorPoints = []common.Point{point}
		return
	}

	if shouldStoreCursorPoint(point.Slot, bf.baseSlot) {
		bf.cursorPoints = append([]common.Point{point}, bf.cursorPoints...)

		if len(bf.cursorPoints) > 200 {
			bf.cursorPoints = bf.cursorPoints[:200]
		}
	}
}

func loadCursorState(filename string) (*CursorState, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cursor CursorState
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, err
	}

	return &cursor, nil
}

func saveCursorState(filename string, points []common.Point) error {
	cursorPoints := make([]CursorPoint, len(points))
	for i, point := range points {
		cursorPoints[i] = CursorPoint{
			Slot: point.Slot,
			Hash: hex.EncodeToString(point.Hash),
		}
	}

	cursor := CursorState{
		Points: cursorPoints,
	}

	data, err := json.MarshalIndent(cursor, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
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
	libNum := blockNumber - 2160
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
	config       *BlockFetcherConfig
	connection   *ouroboros.Connection
	logger       *log.Logger
	slogger      *slog.Logger
	firehose     *FirehoseInstrumentation
	slotConfig   SlotConfig
	cursorPoints []common.Point // Store points using exponential strategy
	baseSlot     uint64         // Base slot for exponential calculation
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

func parseFlags() *BlockFetcherConfig {
	cfg := &BlockFetcherConfig{}

	flag.StringVar(&cfg.Address, "address", "", "Cardano node address (e.g., backbone.cardano.iog.io:3001)")
	flag.StringVar(&cfg.SocketPath, "socket-path", "", "Unix socket path for local node connection")
	flag.StringVar(&cfg.Network, "network", "mainnet", "Network: mainnet, preview, preprod")
	flag.StringVar(&cfg.CursorFile, "cursor-file", "", "File to store/read cursor state for resuming (e.g., cursor.json)")
	flag.Func("network-magic", "Network magic number (0 = auto-detect)", func(s string) error {
		if s == "" {
			cfg.NetworkMagic = 0
			return nil
		}
		var val uint64
		if _, err := fmt.Sscanf(s, "%d", &val); err != nil {
			return err
		}
		cfg.NetworkMagic = uint32(val)
		return nil
	})
	flag.Func("pipeline-limit", "Number of concurrent block fetch requests (default: 10)", func(s string) error {
		if s == "" {
			cfg.PipelineLimit = 10
			return nil
		}
		var val uint64
		if _, err := fmt.Sscanf(s, "%d", &val); err != nil {
			return err
		}
		cfg.PipelineLimit = uint32(val)
		return nil
	})
	flag.Uint64Var(&cfg.StartSlot, "start-slot", 0, "Starting slot number (0 = current tip)")
	flag.StringVar(&cfg.StartHash, "start-hash", "", "Starting block hash (empty = use current tip)")

	flag.Parse()

	cfg.setDefaults()

	if cfg.CursorFile != "" {
		if cursor, err := loadCursorState(cfg.CursorFile); err == nil && len(cursor.Points) > 0 {
			latestPoint := cursor.Points[0]
			cfg.StartSlot = latestPoint.Slot
			cfg.StartHash = latestPoint.Hash
		}
	}

	return cfg
}

func (bf *BlockFetcher) processBlock(block ledger.Block) error {
	if err := bf.firehose.OutputBlock(block); err != nil {
		return err
	}

	if bf.config.CursorFile != "" {
		hashBytes := block.Hash()
		currentPoint := common.NewPoint(block.SlotNumber(), hashBytes[:])

		bf.addCursorPoint(currentPoint)

		if err := saveCursorState(bf.config.CursorFile, bf.cursorPoints); err != nil {
			bf.logger.Printf("Warning: Failed to save cursor state: %v", err)
		}
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

func (bf *BlockFetcher) getStartPoints() ([]common.Point, error) {
	if bf.config.CursorFile != "" {
		if cursor, err := loadCursorState(bf.config.CursorFile); err == nil && len(cursor.Points) > 0 {
			points := make([]common.Point, 0, len(cursor.Points))
			for _, cp := range cursor.Points {
				hash, err := hex.DecodeString(cp.Hash)
				if err != nil {
					bf.logger.Printf("Warning: Failed to decode cursor hash %s: %v", cp.Hash, err)
					continue
				}
				point := common.NewPoint(cp.Slot, hash)
				points = append(points, point)
			}
			if len(points) > 0 {
				bf.logger.Printf("Using cursor points for intersection (count=%d, latest slot=%d)", len(points), points[0].Slot)
				return points, nil
			}
		}
	}

	if bf.config.StartSlot != 0 && bf.config.StartHash != "" {
		hash, err := hex.DecodeString(bf.config.StartHash)
		if err != nil {
			return nil, fmt.Errorf("failed to decode start hash: %w", err)
		}
		point := common.NewPoint(bf.config.StartSlot, hash)
		bf.logger.Printf("Using configured start point: slot=%d, hash=%s", bf.config.StartSlot, bf.config.StartHash)
		return []common.Point{point}, nil
	}

	tip, err := bf.connection.ChainSync().Client.GetCurrentTip()
	if err != nil {
		return nil, fmt.Errorf("failed to get current tip: %w", err)
	}
	bf.logger.Printf("Using current tip as start point: %#v", tip)
	return []common.Point{tip.Point}, nil
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

	points, err := bf.getStartPoints()
	if err != nil {
		return fmt.Errorf("failed to get start points: %w", err)
	}

	if err := bf.connection.ChainSync().Client.Sync(points); err != nil {
		return fmt.Errorf("chain sync failed: %w", err)
	}

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

	cfg := parseFlags()

	logger.Printf("Starting Cardano Block Fetcher: Address=%s, Network=%s, NetworkMagic=%d, PipelineLimit=%d, StartSlot=%d, StartHash=%s",
		cfg.Address, cfg.Network, cfg.NetworkMagic, cfg.PipelineLimit, cfg.StartSlot, cfg.StartHash)

	fetcher := NewBlockFetcher(cfg, logger)

	fetcher.firehose.Init()

	if err := fetcher.Run(); err != nil && err != context.Canceled {
		logger.Fatalf("Block fetcher failed: %v", err)
	}

	logger.Println("Block fetcher shutdown complete")
}
