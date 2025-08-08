package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	"github.com/kelseyhightower/envconfig"
	"google.golang.org/protobuf/proto"
)

type Config struct {
	Address       string `envconfig:"ADDRESS" default:"backbone.cardano.iog.io:3001"`
	Network       string `envconfig:"NETWORK" default:"mainnet"`
	NetworkMagic  uint32 `envconfig:"NETWORK_MAGIC" default:"0"`
	PipelineLimit uint32 `envconfig:"PIPELINE_LIMIT" default:"10"`
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
	f.logger.Printf("FIRE INIT 3.0 %s\n", f.blockTypeURL)
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
	f.logger.Println(fireBlock)
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
	config     *Config
	connection *ouroboros.Connection
	logger     *log.Logger
	slogger    *slog.Logger
	firehose   *FirehoseInstrumentation
	slotConfig SlotConfig
}

func NewBlockFetcher(cfg *Config, logger *log.Logger) *BlockFetcher {
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

func loadConfig() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Process("BLOCK_FETCH", cfg); err != nil {
		return nil, fmt.Errorf("failed to process environment config: %w", err)
	}
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
		network, ok := ouroboros.NetworkByName(bf.config.Network)
		if !ok {
			return fmt.Errorf("invalid network specified: %s", bf.config.Network)
		}
		bf.config.NetworkMagic = network.NetworkMagic
	}
	return nil
}

func (bf *BlockFetcher) PrintBlockInfo(block ledger.Block) {
	switch v := block.(type) {
	case *ledger.ByronEpochBoundaryBlock:
		bf.logger.Printf(
			"Block: era = Byron (EBB), epoch = %d, id = %s\n",
			v.BlockHeader.ConsensusData.Epoch,
			v.Hash(),
		)
	case *ledger.ByronMainBlock:
		bf.logger.Printf(
			"Block: era = Byron, epoch = %d, slot = %d, id = %s\n",
			v.BlockHeader.ConsensusData.SlotId.Epoch,
			v.SlotNumber(),
			v.Hash(),
		)
	case ledger.Block:
		bf.logger.Printf(
			"Block: era = %s, slot = %d, block_no = %d, id = %s\n",
			v.Era().Name,
			v.SlotNumber(),
			v.BlockNumber(),
			v.Hash(),
		)
	}

	bf.logger.Printf(
		"Minted by: %s (%s)\n",
		block.IssuerVkey().PoolId(),
		block.IssuerVkey().Hash(),
	)
	bf.logger.Println("Transactions:")
	for _, tx := range block.Transactions() {
		bf.logger.Printf("- Hash: %s\n", tx.Hash())
		if tx.Metadata() != nil {
			bf.logger.Printf(
				"  Metadata: %#v (%x)\n",
				tx.Metadata().Value(),
				tx.Metadata().Cbor(),
			)
		}
		if len(tx.Inputs()) > 0 {
			bf.logger.Println("  Inputs:")
			for _, input := range tx.Inputs() {
				bf.logger.Printf(
					"  - index = %d, id = %s\n",
					input.Index(),
					input.Id(),
				)
			}
		}
		if len(tx.Outputs()) > 0 {
			bf.logger.Println("  Outputs:")
			for _, output := range tx.Outputs() {
				bf.logger.Printf(
					"  - address = %s, amount = %d, cbor (hex) = %x\n",
					output.Address(),
					output.Amount(),
					output.Cbor(),
				)
				assets := output.Assets()
				if assets != nil {
					bf.logger.Println("  - Assets:")
					for _, policyId := range assets.Policies() {
						for _, assetName := range assets.Assets(policyId) {
							bf.logger.Printf(
								"    - Asset: name = %s, amount = %d, policy = %s\n",
								assetName,
								assets.Asset(policyId, assetName),
								policyId,
							)
						}
					}
				}
				datum := output.Datum()
				if datum != nil {
					jsonData, err := json.Marshal(datum)
					if err != nil {
						bf.logger.Printf(
							"  - Datum: (hex) %x\n",
							datum.Cbor(),
						)
					} else {
						bf.logger.Printf(
							"  - Datum: %s\n",
							jsonData,
						)
					}
				}
			}
		}
		if len(tx.Collateral()) > 0 {
			bf.logger.Println("  Collateral inputs:")
			for _, input := range tx.Collateral() {
				bf.logger.Printf(
					"  - index = %d, id = %s\n",
					input.Index(),
					input.Id(),
				)
			}
		}
		if len(tx.Certificates()) > 0 {
			bf.logger.Println("  Certificates:")
			for _, cert := range tx.Certificates() {
				bf.logger.Printf("  - %T\n", cert)
			}
		}
		if tx.AssetMint() != nil {
			bf.logger.Println("  Asset mints:")
			assets := tx.AssetMint()
			for _, policyId := range assets.Policies() {
				for _, assetName := range assets.Assets(policyId) {
					bf.logger.Printf(
						"    - Asset: name = %s, amount = %d, policy = %s\n",
						assetName,
						assets.Asset(policyId, assetName),
						policyId,
					)
				}
			}
		}
	}
	bf.logger.Println()
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
		block, err = bf.connection.BlockFetch().Client.GetBlock(common.NewPoint(blockSlot, blockHash))
		if err != nil {
			return fmt.Errorf("failed to fetch block: %w", err)
		}
	}
	if block != nil {
		return bf.processBlock(block)
	}
	return nil
}

func (bf *BlockFetcher) chainSyncRollBackwardHandler(
	ctx chainsync.CallbackContext,
	point common.Point,
	tip chainsync.Tip,
) error {
	bf.logger.Printf("roll backward: point = %#v, tip = %#v\n", point, tip)
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
		return fmt.Errorf("failed to resolve network magic: %w", err)
	}

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
		ouroboros.WithNodeToNode(true),
		ouroboros.WithKeepAlive(true),
		ouroboros.WithChainSyncConfig(bf.buildChainSyncConfig()),
		ouroboros.WithLogger(bf.slogger),
	)
	if err != nil {
		return fmt.Errorf("failed to create ouroboros connection: %w", err)
	}

	if err := conn.Dial("tcp", bf.config.Address); err != nil {
		return fmt.Errorf("failed to dial connection to %s: %w", bf.config.Address, err)
	}
	bf.connection = conn
	bf.logger.Printf("Connected to Cardano node at %s", bf.config.Address)
	return nil
}

func (bf *BlockFetcher) start(ctx context.Context) error {
	tip, err := bf.connection.ChainSync().Client.GetCurrentTip()
	if err != nil {
		return fmt.Errorf("failed to get current tip: %w", err)
	}

	bf.logger.Printf("Starting sync from tip: %#v", tip)

	point := tip.Point
	if err := bf.connection.ChainSync().Client.Sync([]common.Point{point}); err != nil {
		return fmt.Errorf("failed to start sync: %w", err)
	}

	<-ctx.Done()
	return ctx.Err()
}

func (bf *BlockFetcher) close() error {
	if bf.connection != nil {
		if err := bf.connection.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
		bf.logger.Println("Connection closed successfully")
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
	logger := log.New(os.Stdout, "[BlockFetcher] ", log.LstdFlags|log.Lshortfile)

	cfg, err := loadConfig()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	logger.Printf("Starting Cardano Block Fetcher with Firehose instrumentation: Address=%s, Network=%s, NetworkMagic=%d, PipelineLimit=%d",
		cfg.Address, cfg.Network, cfg.NetworkMagic, cfg.PipelineLimit)

	fetcher := NewBlockFetcher(cfg, logger)

	fetcher.firehose.Init()

	if err := fetcher.Run(); err != nil && err != context.Canceled {
		logger.Fatalf("Block fetcher failed: %v", err)
	}

	logger.Println("Block fetcher shutdown complete")
}
