package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/no-witness-labs/firehose-cardano/codec"
	"go.uber.org/zap"
)

// Simple output structure for pretty JSON printing
type jsonBlock struct {
	Number    uint64 `json:"number"`
	ID        string `json:"id"`
	ParentNum uint64 `json:"parent_num"`
	ParentID  string `json:"parent_id"`
	LibNum    uint64 `json:"lib_num"`
	Timestamp uint64 `json:"timestamp"`
	PayloadLn int    `json:"payload_len"`
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("received shutdown signal")
		cancel()
	}()

	reader := codec.NewConsoleReader(logger)
	reader.Start(ctx, os.Stdin)

	logger.Info("console reader started, waiting for FIRE BLOCK lines on stdin")
	printTicker := time.NewTicker(30 * time.Second)
	defer printTicker.Stop()

	for {
		select {
		case blk, ok := <-reader.Blocks():
			if !ok {
				logger.Info("blocks channel closed, exiting")
				return
			}
			out := jsonBlock{
				Number:    blk.Number,
				ID:        blk.Id,
				ParentNum: blk.ParentNum,
				ParentID:  blk.ParentId,
				LibNum:    blk.LibNum,
				Timestamp: blk.Timestamp,
				PayloadLn: len(blk.RawPayload),
			}
			b, _ := json.Marshal(out)
			fmt.Println(string(b))
		case err := <-reader.Errors():
			if err != nil {
				logger.Warn("parse error", zap.Error(err))
			}
		case <-printTicker.C:
			logger.Info("still running - waiting for more blocks")
		case <-ctx.Done():
			logger.Info("context done, shutting down")
			reader.Close()
			return
		}
	}
}
