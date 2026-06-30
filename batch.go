// Package weeksjpegencoder provides f5.jar-compatible JPEG encoding.
//
// This file implements batch encoding, which is the most effective way to use
// multiple cores for throughput: each image is fully independent, so encoding a
// batch across a worker pool scales nearly linearly with core count (far better
// than intra-image parallelism, which is limited by the sequential entropy stage
// and the fraction of time spent in the parallelizable DCT — see james_parallel.go).

package weeksjpegencoder

import (
	"image"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
)

// BatchItem describes one image to encode in EncodeBatch.
type BatchItem struct {
	// Image is the source image to encode.
	Image image.Image
	// Writer receives the encoded JPEG bytes for this item.
	Writer io.Writer
	// Quality is the JPEG quality (1-100) for this item.
	Quality int
	// Options are optional per-item encoder options (e.g. WithComment,
	// WithSubsampling). They are applied after the batch defaults, so a caller
	// can override anything — including re-enabling intra-image parallelism.
	Options []Option
}

// EncodeBatch encodes many images concurrently across a pool of goroutines and
// returns a slice of errors parallel to items (errs[i] is the result for
// items[i], nil on success).
//
// workers controls the pool size; a value <= 0 uses runtime.GOMAXPROCS(0).
// Each image is encoded on a single goroutine with intra-image parallelism
// disabled (so the cores are spent across images rather than oversubscribed
// within one image). This is the recommended path for high-throughput encoding
// of many images — it scales nearly linearly with core count.
//
// Every item still produces byte-identical output to a standalone encode; only
// the wall-clock throughput changes. Items are independent, so one item's error
// does not stop the others.
//
// Example:
//
//	items := make([]weeksjpegencoder.BatchItem, len(images))
//	for i, img := range images {
//	    items[i] = weeksjpegencoder.BatchItem{Image: img, Writer: outFiles[i], Quality: 75}
//	}
//	errs := weeksjpegencoder.EncodeBatch(items, 0) // 0 = all cores
//	for i, err := range errs {
//	    if err != nil { log.Printf("image %d failed: %v", i, err) }
//	}
//
//goland:noinspection GoUnusedExportedFunction
func EncodeBatch(items []BatchItem, workers int) []error {
	errs := make([]error, len(items))
	if len(items) == 0 {
		return errs
	}
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > len(items) {
		workers = len(items)
	}

	var next int64 = -1
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(atomic.AddInt64(&next, 1))
				if i >= len(items) {
					return
				}
				errs[i] = encodeBatchItem(items[i])
			}
		}()
	}
	wg.Wait()
	return errs
}

// encodeBatchItem encodes a single batch item with intra-image parallelism
// disabled (the batch already keeps all cores busy across images). Caller
// options are applied last so they can override the batch default.
func encodeBatchItem(item BatchItem) error {
	opts := make([]Option, 0, len(item.Options)+1)
	opts = append(opts, WithParallelEncoding(false))
	opts = append(opts, item.Options...)

	enc, err := NewWeeksEncoderWithOptions(item.Writer, item.Quality, opts...)
	if err != nil {
		return err
	}
	return enc.Encode(item.Image)
}
