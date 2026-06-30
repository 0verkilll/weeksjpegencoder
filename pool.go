// Package weeksjpegencoder provides f5.jar-compatible JPEG encoding.
//
// This file implements memory pooling for the reusable zigzag block buffer used
// in the encode hot path. Using sync.Pool reduces allocations when encoding many
// blocks (and across many images) without external synchronization.
//
// Thread Safety:
// sync.Pool Get/Put are safe for concurrent use, which keeps the door open for
// parallel MCU encoding without additional locking.

package weeksjpegencoder

import (
	"sync"
)

// intBlockArrayPool provides reusable *[64]int buffers for quantized 8x8 blocks.
// The fixed-size array is compatible with the BlockEncoder.EncodeBlock interface.
var intBlockArrayPool = sync.Pool{
	New: func() interface{} {
		return new([64]int)
	},
}

// getIntBlockArray retrieves a *[64]int buffer from the pool.
// The buffer contents are NOT zeroed - the caller must overwrite all 64 entries.
//
// Example usage:
//
//	block := getIntBlockArray()
//	defer putIntBlockArray(block)
//	newDC, err := encoder.EncodeBlock(block, prevDC, isLuminance)
func getIntBlockArray() *[64]int {
	return intBlockArrayPool.Get().(*[64]int)
}

// putIntBlockArray returns a *[64]int buffer to the pool.
// The buffer must not be used after calling this function.
func putIntBlockArray(block *[64]int) {
	if block == nil {
		return
	}
	intBlockArrayPool.Put(block)
}
