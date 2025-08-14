package codec

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// FirehoseBlockLine represents a parsed FIRE BLOCK console line
type FirehoseBlockLine struct {
	Number     uint64
	Id         string
	ParentNum  uint64
	ParentId   string
	LibNum     uint64
	Timestamp  uint64
	PayloadRaw []byte // raw decoded bytes (expected CBOR of block)
	RawLine    string // full original line
}

type ConsoleReader struct {
	out    chan *FirehoseBlockLine
	errs   chan error
	logger *zap.Logger
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewConsoleReader(logger *zap.Logger) *ConsoleReader {
	return &ConsoleReader{
		out:    make(chan *FirehoseBlockLine, 100),
		errs:   make(chan error, 10),
		logger: logger,
	}
}

// Start begins consuming lines from r until ctx is done or EOF
func (c *ConsoleReader) Start(parent context.Context, r io.Reader) {
	ctx, cancel := context.WithCancel(parent)
	c.cancel = cancel
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		scanner := bufio.NewScanner(r)
		// Increase scanner buffer in case of large payload lines
		buf := make([]byte, 0, 4*1024)
		scanner.Buffer(buf, 8*1024*1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					c.sendErr(err)
				}
				return
			}
			line := scanner.Text()
			if strings.HasPrefix(line, "FIRE BLOCK ") {
				blk, err := parseFireBlockLine(line)
				if err != nil {
					c.sendErr(fmt.Errorf("parse fire block line: %w", err))
					continue
				}
				select {
				case c.out <- blk:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
}

func (c *ConsoleReader) sendErr(err error) {
	select {
	case c.errs <- err:
	default:
		if c.logger != nil {
			c.logger.Debug("dropping console reader error", zap.Error(err))
		}
	}
}

// Blocks returns a channel of parsed FIRE BLOCK lines
func (c *ConsoleReader) Blocks() <-chan *FirehoseBlockLine { return c.out }

// Errors returns a channel of asynchronous parse errors
func (c *ConsoleReader) Errors() <-chan error { return c.errs }

// Close stops the reader and closes output channels when done
func (c *ConsoleReader) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	close(c.out)
	close(c.errs)
}

func parseFireBlockLine(line string) (*FirehoseBlockLine, error) {
	// Expected format:
	// FIRE BLOCK <number> <id> <parentNum> <parentId> <libNum> <timestamp> <base64>
	parts := strings.SplitN(line, " ", 9)
	if len(parts) < 8 { // payload may be absent if instrumentation not sending it yet
		return nil, errors.New("invalid FIRE BLOCK line (too few parts)")
	}
	if parts[0] != "FIRE" || parts[1] != "BLOCK" {
		return nil, errors.New("line does not start with FIRE BLOCK")
	}
	number, err := parseUint(parts[2])
	if err != nil {
		return nil, fmt.Errorf("number: %w", err)
	}
	id := parts[3]
	parentNum, err := parseUint(parts[4])
	if err != nil {
		return nil, fmt.Errorf("parentNum: %w", err)
	}
	parentId := parts[5]
	libNum, err := parseUint(parts[6])
	if err != nil {
		return nil, fmt.Errorf("libNum: %w", err)
	}
	timestamp, err := parseUint(parts[7])
	if err != nil {
		return nil, fmt.Errorf("timestamp: %w", err)
	}
	var payloadB64 string
	if len(parts) >= 9 {
		payloadB64 = strings.TrimSpace(parts[8])
	}
	var payload []byte
	if payloadB64 != "" {
		payload, err = base64.StdEncoding.DecodeString(payloadB64)
		if err != nil {
			return nil, fmt.Errorf("base64 decode: %w", err)
		}
	}
	return &FirehoseBlockLine{
		Number:     number,
		Id:         id,
		ParentNum:  parentNum,
		ParentId:   parentId,
		LibNum:     libNum,
		Timestamp:  timestamp,
		PayloadRaw: payload,
		RawLine:    line,
	}, nil
}

func parseUint(s string) (uint64, error) { return strconv.ParseUint(s, 10, 64) }
