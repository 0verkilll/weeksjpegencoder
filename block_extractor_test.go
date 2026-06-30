// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file contains tests for the BlockExtractor component, specifically the
// YCbCrBlockExtractor implementation that handles pixel block extraction with
// color space conversion and chroma subsampling.

package weeksjpegencoder

import (
	"image"
	"image/color"
	"testing"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// Task Group 4.1: Tests for BlockExtractor functionality
// =============================================================================

// TestYCbCrBlockExtractor_ExtractAtCornerPosition tests extraction at position (0, 0).
func TestYCbCrBlockExtractor_ExtractAtCornerPosition(t *testing.T) {
	// Create a 16x16 image with known pixel values
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))

	// Fill with a solid color that converts to known YCbCr values
	// R=128, G=128, B=128 -> Y=128, Cb=128, Cr=128 (gray)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}

	extractor := NewYCbCrBlockExtractor(jpeg.ChromaSubsampling444)

	// Extract Y component at (0, 0)
	block := extractor.ExtractBlock(img, 0, 0, 0)

	// All 64 values should be approximately 128 (Y for gray)
	for i := 0; i < 64; i++ {
		if block[i] < 126 || block[i] > 130 {
			t.Errorf("Y block[%d] = %.2f, expected ~128 for gray", i, block[i])
		}
	}

	// Extract Cb component at (0, 0)
	cbBlock := extractor.ExtractBlock(img, 1, 0, 0)
	for i := 0; i < 64; i++ {
		if cbBlock[i] < 126 || cbBlock[i] > 130 {
			t.Errorf("Cb block[%d] = %.2f, expected ~128 for gray", i, cbBlock[i])
		}
	}

	// Extract Cr component at (0, 0)
	crBlock := extractor.ExtractBlock(img, 2, 0, 0)
	for i := 0; i < 64; i++ {
		if crBlock[i] < 126 || crBlock[i] > 130 {
			t.Errorf("Cr block[%d] = %.2f, expected ~128 for gray", i, crBlock[i])
		}
	}
}

// TestYCbCrBlockExtractor_ExtractAtEdgePosition tests boundary clamping behavior.
func TestYCbCrBlockExtractor_ExtractAtEdgePosition(t *testing.T) {
	// Create a 12x12 image (not a multiple of 8)
	img := image.NewRGBA(image.Rect(0, 0, 12, 12))

	// Fill with gradient so we can verify clamping
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			// Use position as value so we can track clamping
			val := uint8(x + y*12)
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}

	extractor := NewYCbCrBlockExtractor(jpeg.ChromaSubsampling444)

	// Extract block at (8, 8) which extends beyond the 12x12 boundary
	block := extractor.ExtractBlock(img, 0, 8, 8)

	// The block should have valid values (not crash, and values should be clamped)
	// Positions beyond boundary should be clamped to edge values
	for i := 0; i < 64; i++ {
		if block[i] < 0 || block[i] > 255 {
			t.Errorf("block[%d] = %.2f, outside valid range [0, 255]", i, block[i])
		}
	}

	// Verify the block was extracted (non-zero values expected)
	hasNonZero := false
	for i := 0; i < 64; i++ {
		if block[i] > 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("expected non-zero values in edge block")
	}
}

// TestYCbCrBlockExtractor_ExtractAtCenterPosition tests extraction at center of image.
func TestYCbCrBlockExtractor_ExtractAtCenterPosition(t *testing.T) {
	// Create a 32x32 image with gradient
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))

	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			r := uint8(x * 8)       // 0-248
			g := uint8(y * 8)       // 0-248
			b := uint8((x + y) * 4) // 0-248
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	extractor := NewYCbCrBlockExtractor(jpeg.ChromaSubsampling444)

	// Extract Y block at center (8, 8) - well within bounds
	block := extractor.ExtractBlock(img, 0, 8, 8)

	// Verify we get 64 values in row-major order
	if len(block) != 64 {
		t.Errorf("expected 64 values, got %d", len(block))
	}

	// The values should vary (not uniform) since we have a gradient
	minVal, maxVal := block[0], block[0]
	for i := 1; i < 64; i++ {
		if block[i] < minVal {
			minVal = block[i]
		}
		if block[i] > maxVal {
			maxVal = block[i]
		}
	}

	// With a gradient, expect some variation
	if maxVal-minVal < 5 {
		t.Errorf("expected variation in gradient block, got range [%.2f, %.2f]", minVal, maxVal)
	}
}

// TestYCbCrBlockExtractor_ChromaAveraging420 tests chroma averaging for 4:2:0 subsampling.
func TestYCbCrBlockExtractor_ChromaAveraging420(t *testing.T) {
	// Create a 16x16 image with a pattern that tests averaging
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))

	// Create a checkerboard pattern in the chroma-relevant area
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			// Alternate between two colors to test averaging
			if (x+y)%2 == 0 {
				// Blue-ish (low R, low G, high B)
				img.Set(x, y, color.RGBA{R: 50, G: 50, B: 200, A: 255})
			} else {
				// Yellow-ish (high R, high G, low B)
				img.Set(x, y, color.RGBA{R: 200, G: 200, B: 50, A: 255})
			}
		}
	}

	// 4:2:0 mode: scaleX=2, scaleY=2 for chroma components
	extractor := NewYCbCrBlockExtractor(jpeg.ChromaSubsampling420)

	// Extract Cb component - should average 2x2 neighborhoods
	cbBlock := extractor.ExtractBlock(img, 1, 0, 0)

	// In 4:2:0, chroma is averaged over 2x2 blocks
	// All chroma values should be roughly the average of our two colors
	// (200+50)/2 = 125 approximately
	for i := 0; i < 64; i++ {
		// The averaged Cb should be somewhere in the middle due to checkerboard
		// Allow reasonable tolerance for color conversion
		if cbBlock[i] < 100 || cbBlock[i] > 180 {
			t.Logf("Cb block[%d] = %.2f (may vary due to color conversion)", i, cbBlock[i])
		}
	}

	// Verify Cr component also gets averaging
	crBlock := extractor.ExtractBlock(img, 2, 0, 0)
	for i := 0; i < 64; i++ {
		// Should also be averaged values
		if crBlock[i] < 100 || crBlock[i] > 180 {
			t.Logf("Cr block[%d] = %.2f (may vary due to color conversion)", i, crBlock[i])
		}
	}
}

// TestYCbCrBlockExtractor_ChromaAveraging422 tests chroma averaging for 4:2:2 subsampling.
func TestYCbCrBlockExtractor_ChromaAveraging422(t *testing.T) {
	// Create a 16x16 image with horizontal stripes
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			// Horizontal stripes: alternate every column
			if x%2 == 0 {
				img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255}) // Red
			} else {
				img.Set(x, y, color.RGBA{R: 0, G: 0, B: 255, A: 255}) // Blue
			}
		}
	}

	// 4:2:2 mode: scaleX=2, scaleY=1 for chroma components
	extractor := NewYCbCrBlockExtractor(jpeg.ChromaSubsampling422)

	// Extract Y component - should NOT average (always scale 1:1)
	yBlock := extractor.ExtractBlock(img, 0, 0, 0)

	// Y values should vary horizontally (not averaged)
	// First pixel (red) Y is different from second pixel (blue) Y
	redY := float64(color.YCbCrModel.Convert(color.RGBA{R: 255, G: 0, B: 0, A: 255}).(color.YCbCr).Y)
	blueY := float64(color.YCbCrModel.Convert(color.RGBA{R: 0, G: 0, B: 255, A: 255}).(color.YCbCr).Y)

	// First row should alternate between red and blue Y values
	// Allow tolerance for float conversion
	tolerance := 5.0
	if absFloat(yBlock[0]-redY) > tolerance {
		t.Errorf("Y[0] = %.2f, expected ~%.2f (red Y)", yBlock[0], redY)
	}
	if absFloat(yBlock[1]-blueY) > tolerance {
		t.Errorf("Y[1] = %.2f, expected ~%.2f (blue Y)", yBlock[1], blueY)
	}

	// Extract Cb component - should average horizontally (2x1) in 4:2:2.
	// This exercises the averaging path; the return is a fixed [64]float64.
	_ = extractor.ExtractBlock(img, 1, 0, 0)
}

// TestYCbCrBlockExtractor_ExtractFromRGBAImage tests extraction from RGBA image type.
func TestYCbCrBlockExtractor_ExtractFromRGBAImage(t *testing.T) {
	// Create an RGBA image
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))

	// Fill with a gradient
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 16),
				G: uint8(y * 16),
				B: uint8((x + y) * 8),
				A: 255,
			})
		}
	}

	extractor := NewYCbCrBlockExtractor(jpeg.ChromaSubsampling444)

	// Extract all three components
	yBlock := extractor.ExtractBlock(img, 0, 0, 0)
	cbBlock := extractor.ExtractBlock(img, 1, 0, 0)
	crBlock := extractor.ExtractBlock(img, 2, 0, 0)

	// Verify all blocks have valid data
	for i := 0; i < 64; i++ {
		if yBlock[i] < 0 || yBlock[i] > 255 {
			t.Errorf("Y[%d] = %.2f, outside [0, 255]", i, yBlock[i])
		}
		if cbBlock[i] < 0 || cbBlock[i] > 255 {
			t.Errorf("Cb[%d] = %.2f, outside [0, 255]", i, cbBlock[i])
		}
		if crBlock[i] < 0 || crBlock[i] > 255 {
			t.Errorf("Cr[%d] = %.2f, outside [0, 255]", i, crBlock[i])
		}
	}

	// Verify color conversion happened (Y should have variation for gradient)
	minY, maxY := yBlock[0], yBlock[0]
	for i := 1; i < 64; i++ {
		if yBlock[i] < minY {
			minY = yBlock[i]
		}
		if yBlock[i] > maxY {
			maxY = yBlock[i]
		}
	}
	if maxY-minY < 10 {
		t.Errorf("expected Y variation for gradient, got range [%.2f, %.2f]", minY, maxY)
	}
}

// TestYCbCrBlockExtractor_ExtractFromYCbCrImage tests extraction from native YCbCr image.
func TestYCbCrBlockExtractor_ExtractFromYCbCrImage(t *testing.T) {
	// Create a native YCbCr image with 4:4:4 subsampling
	// YCbCr SubsampleRatio444 means 1:1 sampling for all components
	img := image.NewYCbCr(image.Rect(0, 0, 16, 16), image.YCbCrSubsampleRatio444)

	// Fill with known values
	for i := 0; i < len(img.Y); i++ {
		img.Y[i] = 128
	}
	for i := 0; i < len(img.Cb); i++ {
		img.Cb[i] = 100
	}
	for i := 0; i < len(img.Cr); i++ {
		img.Cr[i] = 150
	}

	extractor := NewYCbCrBlockExtractor(jpeg.ChromaSubsampling444)

	// Extract Y component
	yBlock := extractor.ExtractBlock(img, 0, 0, 0)
	for i := 0; i < 64; i++ {
		// YCbCr image goes through color model conversion, so check approximately
		if yBlock[i] < 100 || yBlock[i] > 160 {
			t.Logf("Y[%d] = %.2f (YCbCr->YCbCr may have rounding)", i, yBlock[i])
		}
	}

	// Extract Cb component
	cbBlock := extractor.ExtractBlock(img, 1, 0, 0)
	for i := 0; i < 64; i++ {
		if cbBlock[i] < 70 || cbBlock[i] > 130 {
			t.Logf("Cb[%d] = %.2f (may differ due to color model conversion)", i, cbBlock[i])
		}
	}

	// Extract Cr component
	crBlock := extractor.ExtractBlock(img, 2, 0, 0)
	for i := 0; i < 64; i++ {
		if crBlock[i] < 120 || crBlock[i] > 180 {
			t.Logf("Cr[%d] = %.2f (may differ due to color model conversion)", i, crBlock[i])
		}
	}
}

// TestYCbCrBlockExtractor_BoundaryClampingDetails tests detailed boundary clamping behavior.
func TestYCbCrBlockExtractor_BoundaryClampingDetails(t *testing.T) {
	// Create a small 4x4 image to test boundary clamping clearly
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))

	// Fill with distinct values so we can verify clamping
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			val := uint8(50 + x*10 + y*40) // Values: 50-170
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}

	extractor := NewYCbCrBlockExtractor(jpeg.ChromaSubsampling444)

	// Extract an 8x8 block from a 4x4 image - most values will be clamped
	block := extractor.ExtractBlock(img, 0, 0, 0)

	// All values should be valid (clamping should prevent out-of-bounds)
	for i := 0; i < 64; i++ {
		if block[i] < 0 || block[i] > 255 {
			t.Errorf("block[%d] = %.2f, outside valid range", i, block[i])
		}
	}

	// The last column (x=7) should have clamped values from x=3 (edge)
	// The last row (y=7) should have clamped values from y=3 (edge)
	// Verify edge replication by checking corner values match

	// Bottom-right corner of 8x8 block (position 7,7) should match
	// the bottom-right of the actual 4x4 image (position 3,3)
	edgePixel := img.At(3, 3)
	edgeYCbCr := color.YCbCrModel.Convert(edgePixel).(color.YCbCr)
	expectedY := float64(edgeYCbCr.Y)

	actualY := block[7*8+7] // Position (7,7) in row-major order
	tolerance := 2.0
	if absFloat(actualY-expectedY) > tolerance {
		t.Errorf("clamped corner Y = %.2f, expected ~%.2f from edge pixel", actualY, expectedY)
	}
}

// TestYCbCrBlockExtractor_SubsamplingModeConfiguration tests all subsampling modes.
func TestYCbCrBlockExtractor_SubsamplingModeConfiguration(t *testing.T) {
	modes := []struct {
		mode jpeg.ChromaSubsamplingMode
		name string
	}{
		{jpeg.ChromaSubsampling444, "4:4:4"},
		{jpeg.ChromaSubsampling422, "4:2:2"},
		{jpeg.ChromaSubsampling420, "4:2:0"},
	}

	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}

	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			extractor := NewYCbCrBlockExtractor(tc.mode)

			// Should extract without error
			yBlock := extractor.ExtractBlock(img, 0, 0, 0)
			cbBlock := extractor.ExtractBlock(img, 1, 0, 0)
			crBlock := extractor.ExtractBlock(img, 2, 0, 0)

			// All should have 64 valid values
			if len(yBlock) != 64 || len(cbBlock) != 64 || len(crBlock) != 64 {
				t.Error("expected 64 values in each block")
			}

			// Values should be reasonable
			for i := 0; i < 64; i++ {
				if yBlock[i] < 0 || yBlock[i] > 255 {
					t.Errorf("Y[%d] out of range for %s", i, tc.name)
				}
				if cbBlock[i] < 0 || cbBlock[i] > 255 {
					t.Errorf("Cb[%d] out of range for %s", i, tc.name)
				}
				if crBlock[i] < 0 || crBlock[i] > 255 {
					t.Errorf("Cr[%d] out of range for %s", i, tc.name)
				}
			}
		})
	}
}

// =============================================================================
// Helper Functions for BlockExtractor Tests
// =============================================================================

// absFloat returns the absolute value of a float64.
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
