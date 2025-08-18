package pbcardano

import (
	"encoding/hex"
	"time"

	"github.com/streamingfast/bstream"
)

// Implement firecore.Block interface methods

func (b *Block) GetFirehoseBlockID() string {
	if b.Header != nil && len(b.Header.Hash) > 0 {
		return hex.EncodeToString(b.Header.Hash)
	}
	return ""
}

func (b *Block) GetFirehoseBlockNumber() uint64 {
	if b.Header != nil {
		return b.Header.Height
	}
	return 0
}

func (b *Block) GetFirehoseBlockParentID() string {
	// Note: We don't have parent hash in the current schema
	// This will need to be updated when we have the full block schema
	return ""
}

func (b *Block) GetFirehoseBlockParentNumber() uint64 {
	blockNum := b.GetFirehoseBlockNumber()
	if blockNum == 0 {
		return 0
	}
	return blockNum - 1
}

func (b *Block) GetFirehoseBlockTime() time.Time {
	if b.Timestamp > 0 {
		// Timestamp is in milliseconds, convert to time.Time
		return time.Unix(int64(b.Timestamp/1000), int64(b.Timestamp%1000)*1000000)
	}
	// Fallback: return current time if no timestamp is available
	return time.Now()
}

// Optional: Implement BlockLIBNumDerivable interface for LIB support
func (b *Block) GetFirehoseBlockLIBNum() uint64 {
	blockNum := b.GetFirehoseBlockNumber()
	// For Cardano, blocks are typically considered final after a certain number of confirmations
	// Using a conservative estimate of 108 blocks (about 36 hours on mainnet)
	if blockNum <= 108 {
		return 0
	}
	return blockNum - 108
}

// Additional helper method for bstream compatibility
func (b *Block) AsRef() bstream.BlockRef {
	return bstream.NewBlockRef(b.GetFirehoseBlockID(), b.GetFirehoseBlockNumber())
}
