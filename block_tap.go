package weeksjpegencoder

// BlockTapFunc is called for every zigzag-ordered quantized block immediately
// before it would be entropy-coded. The tap may mutate the block in place; the
// mutation is what gets passed to the inner encoder (if any).
//
// blockIndex is a monotonically increasing counter starting at 0 across all
// components and MCUs in encode order. isLuminance is true for Y blocks and
// false for Cb/Cr.
//
// Used by F5 steganalysis training pipelines to (a) extract cover-image
// quantized DCT coefficients without paying the entropy-coding cost, and
// (b) inject F5-modified coefficients in a second pass to produce a valid
// stego JPEG byte stream.
type BlockTapFunc func(blockIndex int, isLuminance bool, block *[64]int)

// TapBlockEncoder implements BlockEncoder by invoking Tap on each block,
// then forwarding the (possibly mutated) block to Inner.
//
// If Inner is nil the block is discarded after the tap fires — useful for the
// extract-only path where the caller wants quantized coefficients but no JPEG
// output. In that mode the encoder driver still writes JPEG headers; pass
// io.Discard as the encoder writer if the bytes are not wanted.
type TapBlockEncoder struct {
	Inner BlockEncoder
	Tap   BlockTapFunc
	idx   int
}

// NewTapBlockEncoder constructs a TapBlockEncoder. Either Inner or Tap (or
// both) may be set; both nil is a no-op.
func NewTapBlockEncoder(inner BlockEncoder, tap BlockTapFunc) *TapBlockEncoder {
	return &TapBlockEncoder{Inner: inner, Tap: tap}
}

// EncodeBlock satisfies BlockEncoder. The tap fires first; if Inner is non-nil
// the (possibly mutated) block is forwarded to it.
func (t *TapBlockEncoder) EncodeBlock(block *[64]int, prevDC int, isLuminance bool) (int, error) {
	if t.Tap != nil {
		t.Tap(t.idx, isLuminance, block)
	}
	t.idx++
	if t.Inner != nil {
		return t.Inner.EncodeBlock(block, prevDC, isLuminance)
	}
	return block[0], nil
}

// Flush satisfies BlockEncoder. No-op if Inner is nil.
func (t *TapBlockEncoder) Flush() error {
	if t.Inner != nil {
		return t.Inner.Flush()
	}
	return nil
}

// Reset zeroes the block index counter so the same tap can be reused across
// multiple Encode calls.
func (t *TapBlockEncoder) Reset() {
	t.idx = 0
}

var _ BlockEncoder = (*TapBlockEncoder)(nil)
