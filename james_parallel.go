// Package weeksjpegencoder provides f5.jar-compatible JPEG encoding.
//
// This file implements optional multi-core encoding for James-compatible mode.
//
// f5.jar's JpegEncoder.WriteCompressedData is itself a two-phase pipeline: it
// first DCT-and-quantizes every block into a flat coefficient array, then runs
// a separate Huffman pass over that array. We mirror that split so we can run
// phase one (per-block forward DCT + quantization — fully independent work)
// across goroutines, while keeping phase two (entropy coding, which carries
// differential DC predictors and a serial bit buffer) strictly sequential in
// scan order. Because the entropy phase sees the same coefficients in the same
// order no matter how many workers ran phase one, the output is byte-identical.

package weeksjpegencoder

import (
	"sync"

	"github.com/0verkilll/jpeg"
)

// parallelBlockThreshold is the minimum number of 8x8 blocks before the
// parallel DCT+quantize path is used. Below this the goroutine/coordination
// overhead outweighs the gain (small images already encode in microseconds).
// For 4:2:0 there are ~6 blocks per chroma-block cell, so 768 blocks is roughly
// a 176x176 image and up.
const parallelBlockThreshold = 768

// jamesBlockJob describes one 8x8 block to extract and transform. The slot
// index is the job's position in the scan-order job slice.
type jamesBlockJob struct {
	comp         int // 0=Y, 1=Cb, 2=Cr
	xpos         int // chroma-block column origin (c*8)
	ypos         int // chroma-block row origin (r*8)
	yblockoffset int // i*8 within the MCU
	xblockoffset int // j*8 within the MCU
}

// jamesBlockDims carries the per-image geometry the block extraction needs.
type jamesBlockDims struct {
	imageWidth  int
	imageHeight int
	hSampFactor [3]int
	vSampFactor [3]int
}

// quantizeJamesBlock extracts one 8x8 block per f5.jar's indexing, runs the
// integrated forward DCT + quantization, and returns the zigzag-ordered
// quantized coefficients. It reads only the (immutable) pre-converted YCbCr
// planes and the quantizer's divisor tables, and writes only local arrays, so
// it is safe to call concurrently from multiple goroutines.
func (e *WeeksEncoder) quantizeJamesBlock(ext *JamesBlockExtractor, jb jamesBlockJob, d jamesBlockDims) [64]int {
	var block [64]float64
	vSamp := d.vSampFactor[jb.comp]
	hSamp := d.hSampFactor[jb.comp]
	yLimit := d.imageHeight / 2 * vSamp
	xLimit := d.imageWidth / 2 * hSamp

	for a := 0; a < 8; a++ {
		for b := 0; b < 8; b++ {
			ia := jb.ypos*vSamp + jb.yblockoffset + a
			ib := jb.xpos*hSamp + jb.xblockoffset + b

			// Bounds clamp exactly like f5.jar's WriteCompressedData.
			if yLimit <= ia {
				ia = yLimit - 1
			}
			if xLimit <= ib {
				ib = xLimit - 1
			}

			switch jb.comp {
			case 0:
				block[a*8+b] = ext.getY(ia, ib)
			case 1:
				block[a*8+b] = ext.getCb(ia, ib)
			case 2:
				block[a*8+b] = ext.getCr(ia, ib)
			}
		}
	}

	quantized := e.jamesQuantizer.ForwardDCTAndQuantize(&block, jb.comp == 0)

	var zz [64]int
	for k := 0; k < 64; k++ {
		zz[k] = quantized[jpeg.ZigzagOrder[k]]
	}
	return zz
}

// encodeJamesSequential runs both phases in a single goroutine. This is the
// path used for small images and whenever parallel encoding is disabled.
func (e *WeeksEncoder) encodeJamesSequential(
	blockEncoder BlockEncoder,
	ext *JamesBlockExtractor,
	jobs []jamesBlockJob,
	dims jamesBlockDims,
	dcPred [3]int,
) error {
	for i := range jobs {
		zz := e.quantizeJamesBlock(ext, jobs[i], dims)
		newDC, err := blockEncoder.EncodeBlock(&zz, dcPred[jobs[i].comp], jobs[i].comp == 0)
		if err != nil {
			return err
		}
		dcPred[jobs[i].comp] = newDC
	}
	return blockEncoder.Flush()
}

// encodeJamesParallel runs the DCT+quantize phase across `workers` goroutines,
// each owning a contiguous slice of the job list and writing only its own slots
// of the shared coeffs buffer (no contention). It then entropy-codes the blocks
// sequentially in scan order, producing byte-identical output to the sequential
// path.
func (e *WeeksEncoder) encodeJamesParallel(
	blockEncoder BlockEncoder,
	ext *JamesBlockExtractor,
	jobs []jamesBlockJob,
	dims jamesBlockDims,
	dcPred [3]int,
	workers int,
) error {
	n := len(jobs)
	coeffs := make([][64]int, n)

	// Phase 1 — parallel forward DCT + quantization.
	chunk := (n + workers - 1) / workers
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		start := w * chunk
		if start >= n {
			break
		}
		end := start + chunk
		if end > n {
			end = n
		}
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for idx := start; idx < end; idx++ {
				coeffs[idx] = e.quantizeJamesBlock(ext, jobs[idx], dims)
			}
		}(start, end)
	}
	wg.Wait()

	// Phase 2 — sequential entropy coding in exact scan order.
	for idx := 0; idx < n; idx++ {
		comp := jobs[idx].comp
		newDC, err := blockEncoder.EncodeBlock(&coeffs[idx], dcPred[comp], comp == 0)
		if err != nil {
			return err
		}
		dcPred[comp] = newDC
	}
	return blockEncoder.Flush()
}
