package weeksjpegencoder

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"testing"
)

// parCreateTestImage builds a deterministic noisy RGBA image (lots of AC
// coefficients) so the parallel and sequential paths are exercised on
// non-trivial content.
func parCreateTestImage(w, h, seed int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := uint8((x*7 + y*13 + x*y + seed) & 255)
			g := uint8((x*3 + y*17 + 101 + seed) & 255)
			b := uint8((x*11 + y*5 + 200) & 255)
			img.SetRGBA(x, y, color.RGBA{r, g, b, 255})
		}
	}
	return img
}

func parEncode(t *testing.T, img image.Image, quality int, opts ...Option) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, quality, opts...)
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions: %v", err)
	}
	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return buf.Bytes()
}

// TestParallelEncodingByteIdentical verifies that parallel DCT+quantize produces
// byte-identical output to the sequential path across sizes, qualities, and
// worker counts. This is the core safety property: parallelism must never change
// the f5.jar-compatible output.
func TestParallelEncodingByteIdentical(t *testing.T) {
	sizes := [][2]int{{256, 256}, {333, 289}, {512, 512}, {200, 200}, {127, 129}}
	qualities := []int{1, 50, 75, 90, 100}
	workerCounts := []int{1, 2, 4, 8, 16}

	for _, sz := range sizes {
		img := parCreateTestImage(sz[0], sz[1], 7)
		for _, q := range qualities {
			seq := parEncode(t, img, q, WithParallelEncoding(false))
			for _, w := range workerCounts {
				par := parEncode(t, img, q, WithParallelEncoding(true), WithMaxWorkers(w))
				if !bytes.Equal(seq, par) {
					t.Errorf("%dx%d Q%d workers=%d: parallel output differs from sequential (seq=%d par=%d bytes)",
						sz[0], sz[1], q, w, len(seq), len(par))
				}
			}
		}
	}
}

// TestParallelEncodingMatchesDefault verifies the default encoder (parallel on)
// matches an explicitly-sequential encode — i.e. enabling parallelism by default
// did not change output.
func TestParallelEncodingMatchesDefault(t *testing.T) {
	img := parCreateTestImage(256, 256, 3)
	def := parEncode(t, img, 80) // default options (parallel on)
	seq := parEncode(t, img, 80, WithParallelEncoding(false))
	if !bytes.Equal(def, seq) {
		t.Errorf("default encode differs from sequential: def=%d seq=%d bytes", len(def), len(seq))
	}
}

// TestEncodeBatch verifies that batch encoding produces, for every item, output
// byte-identical to a standalone encode, and reports per-item errors correctly.
func TestEncodeBatch(t *testing.T) {
	const n = 12
	items := make([]BatchItem, n)
	bufs := make([]*bytes.Buffer, n)
	expected := make([][]byte, n)

	for i := 0; i < n; i++ {
		img := parCreateTestImage(128+i*8, 128, i)
		bufs[i] = &bytes.Buffer{}
		items[i] = BatchItem{Image: img, Writer: bufs[i], Quality: 75}
		// Standalone reference encode (default settings).
		expected[i] = parEncode(t, img, 75)
	}

	errs := EncodeBatch(items, 0)
	if len(errs) != n {
		t.Fatalf("EncodeBatch returned %d errors, want %d", len(errs), n)
	}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Errorf("item %d: unexpected error %v", i, errs[i])
			continue
		}
		if !bytes.Equal(bufs[i].Bytes(), expected[i]) {
			t.Errorf("item %d: batch output differs from standalone (batch=%d standalone=%d bytes)",
				i, bufs[i].Len(), len(expected[i]))
		}
	}
}

// TestEncodeBatchEmpty verifies the empty-batch edge case.
func TestEncodeBatchEmpty(t *testing.T) {
	if errs := EncodeBatch(nil, 4); len(errs) != 0 {
		t.Errorf("EncodeBatch(nil) returned %d errors, want 0", len(errs))
	}
}

// TestEncodeBatchWorkerCounts verifies output is stable regardless of the
// worker count used.
func TestEncodeBatchWorkerCounts(t *testing.T) {
	const n = 8
	makeItems := func() ([]BatchItem, []*bytes.Buffer) {
		items := make([]BatchItem, n)
		bufs := make([]*bytes.Buffer, n)
		for i := 0; i < n; i++ {
			bufs[i] = &bytes.Buffer{}
			items[i] = BatchItem{Image: parCreateTestImage(200, 200, i), Writer: bufs[i], Quality: 60}
		}
		return items, bufs
	}

	var golden [][]byte
	for _, workers := range []int{1, 2, 4, 8, 16} {
		items, bufs := makeItems()
		errs := EncodeBatch(items, workers)
		for i, err := range errs {
			if err != nil {
				t.Fatalf("workers=%d item %d: %v", workers, i, err)
			}
		}
		if golden == nil {
			golden = make([][]byte, n)
			for i := range bufs {
				golden[i] = append([]byte(nil), bufs[i].Bytes()...)
			}
			continue
		}
		for i := range bufs {
			if !bytes.Equal(bufs[i].Bytes(), golden[i]) {
				t.Errorf("workers=%d: item %d output differs from workers=1 (%s)",
					workers, i, fmt.Sprintf("%d vs %d bytes", bufs[i].Len(), len(golden[i])))
			}
		}
	}
}
