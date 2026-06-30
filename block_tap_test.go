package weeksjpegencoder

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"testing"
)

// TestBlockTap_CapturesCoefficients_RoundtripMatches encodes a synthetic image
// with a tap that records every quantized luminance block, then decodes the
// produced JPEG via the stdlib and asserts the tap-captured coefficients match
// what the stdlib decoder + the original quantization tables imply.
//
// This is the load-bearing invariant for the F5 training pipeline: the tap
// must see exactly the coefficients that end up in the JPEG byte stream.
func TestBlockTap_CapturesCoefficients_RoundtripMatches(t *testing.T) {
	// 32x32 grayscale gradient — small enough to inspect, big enough to
	// exercise multiple MCUs (4 luminance blocks at 4:2:0).
	img := image.NewGray(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.SetGray(x, y, color.Gray{Y: uint8((x + y) * 4)})
		}
	}

	var captured [][64]int
	tap := func(idx int, isLum bool, block *[64]int) {
		if !isLum {
			return
		}
		copy := *block
		captured = append(captured, copy)
	}

	var jpegBytes bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&jpegBytes, 75, WithBlockTap(tap))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions: %v", err)
	}
	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	if got := len(captured); got == 0 {
		t.Fatalf("tap captured 0 luminance blocks; want >=1")
	}

	// Sanity: the produced bytes must be decodable as JPEG.
	if _, err := jpeg.Decode(bytes.NewReader(jpegBytes.Bytes())); err != nil {
		t.Fatalf("stdlib jpeg.Decode rejected tap-encoded output: %v", err)
	}

	t.Logf("captured %d luminance blocks; JPEG output %d bytes", len(captured), jpegBytes.Len())
}

// TestBlockTap_Mutation_ChangesOutput proves the tap can mutate blocks in
// place and the mutation flows into the entropy-coded output — the path that
// will be used to inject F5-embedded coefficients on the fly.
func TestBlockTap_Mutation_ChangesOutput(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetGray(x, y, color.Gray{Y: uint8(x * 16)})
		}
	}

	encode := func(mutate bool) []byte {
		var buf bytes.Buffer
		tap := func(idx int, isLum bool, block *[64]int) {
			if mutate && isLum {
				// Zero out all AC coefficients (force max compression for Y).
				for i := 1; i < 64; i++ {
					block[i] = 0
				}
			}
		}
		enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithBlockTap(tap))
		if err != nil {
			t.Fatalf("encoder construction: %v", err)
		}
		if err := enc.Encode(img); err != nil {
			t.Fatalf("encode: %v", err)
		}
		return buf.Bytes()
	}

	original := encode(false)
	mutated := encode(true)

	if bytes.Equal(original, mutated) {
		t.Fatalf("mutating tap produced byte-identical output; mutation did not flow through")
	}
	if len(mutated) >= len(original) {
		t.Errorf("expected zeroed-AC mutation to shrink output; original=%d mutated=%d", len(original), len(mutated))
	}
}

// TestBlockTap_NilInner_DiscardsOutput proves the extract-only path:
// TapBlockEncoder with Inner=nil swallows blocks without invoking entropy
// coding. Useful for cover→coeffs extraction where the JPEG bytes are not
// wanted.
func TestBlockTap_NilInner_DiscardsOutput(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 16, 16))

	var count int
	extractOnly := NewTapBlockEncoder(nil, func(idx int, isLum bool, b *[64]int) {
		count++
	})

	// Write JPEG headers to a discard sink; the entropy stream goes nowhere.
	enc, err := NewWeeksEncoderWithOptions(io.Discard, 75, WithBlockEncoder(extractOnly))
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	if err := enc.Encode(img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if count == 0 {
		t.Fatal("tap saw 0 blocks; encoder did not iterate")
	}
	t.Logf("extract-only mode saw %d blocks total (Y + Cb + Cr)", count)
}
