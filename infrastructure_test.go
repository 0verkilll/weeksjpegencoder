// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file contains tests for test infrastructure verification (Task 4.1).
// These tests verify that:
// - Testdata generator produces valid test images
// - Benchmark infrastructure compiles and runs
// - Fuzz test infrastructure compiles

package weeksjpegencoder

import (
	"bytes"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"testing"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// Task 4.1: Focused Tests for Test Infrastructure
// =============================================================================

// TestInfrastructure_TestdataGeneratorProducesValidImages tests that the
// testdata generator produces valid test images that can be encoded and decoded.
func TestInfrastructure_TestdataGeneratorProducesValidImages(t *testing.T) {
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{"8x8 minimal MCU", 8, 8},
		{"64x64 standard", 64, 64},
		{"100x75 non-square", 100, 75},
		{"17x23 odd dimensions", 17, 23},
		{"256x256 larger", 256, 256},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate test image using helper
			img := generateTestPatternImage(tc.width, tc.height)

			// Verify image has correct dimensions
			bounds := img.Bounds()
			if bounds.Dx() != tc.width || bounds.Dy() != tc.height {
				t.Errorf("Image dimensions mismatch: got %dx%d, want %dx%d",
					bounds.Dx(), bounds.Dy(), tc.width, tc.height)
			}

			// Verify image can be encoded
			data, err := WeeksEncodeToBytesStandard(img, 75)
			if err != nil {
				t.Fatalf("Failed to encode generated image: %v", err)
			}

			// Verify encoded data is valid JPEG
			if len(data) < 4 {
				t.Fatal("Encoded data too short")
			}
			if data[0] != 0xFF || data[1] != 0xD8 {
				t.Error("Missing SOI marker")
			}
			if data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
				t.Error("Missing EOI marker")
			}

			// Verify image can be decoded
			decoded, err := stdjpeg.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("Failed to decode encoded image: %v", err)
			}

			// Verify decoded dimensions match
			decodedBounds := decoded.Bounds()
			if decodedBounds.Dx() != tc.width || decodedBounds.Dy() != tc.height {
				t.Errorf("Decoded dimensions mismatch: got %dx%d, want %dx%d",
					decodedBounds.Dx(), decodedBounds.Dy(), tc.width, tc.height)
			}
		})
	}
}

// TestInfrastructure_BenchmarkInfrastructureCompiles tests that benchmark
// infrastructure compiles and runs without errors.
func TestInfrastructure_BenchmarkInfrastructureCompiles(t *testing.T) {
	// Test that all components needed for benchmarks work
	t.Run("encoder creation for benchmark", func(t *testing.T) {
		var buf bytes.Buffer
		enc, err := NewWeeksEncoder(&buf, 75)
		if err != nil {
			t.Fatalf("NewWeeksEncoder failed: %v", err)
		}
		if enc == nil {
			t.Fatal("Encoder is nil")
		}
	})

	t.Run("image generation for benchmark", func(t *testing.T) {
		sizes := []struct {
			name   string
			width  int
			height int
		}{
			{"icon", 64, 64},
			{"thumbnail", 256, 256},
		}

		for _, size := range sizes {
			img := generateTestPatternImage(size.width, size.height)
			if img == nil {
				t.Errorf("Failed to generate %s image", size.name)
			}

			// Verify it can be encoded (benchmark will do this)
			_, err := WeeksEncodeToBytesStandard(img, 75)
			if err != nil {
				t.Errorf("Failed to encode %s image: %v", size.name, err)
			}
		}
	})

	t.Run("quality estimation setup", func(t *testing.T) {
		// Generate image and encode
		img := generateTestPatternImage(64, 64)
		data, err := WeeksEncodeToBytesStandard(img, 75)
		if err != nil {
			t.Fatalf("WeeksEncodeToBytes failed: %v", err)
		}

		// Create quality estimator
		estimator := jpeg.NewQualityEstimator(nil)
		if estimator == nil {
			t.Fatal("Quality estimator is nil")
		}

		// Estimate quality (benchmark operation)
		estimate, err := estimator.EstimateQuality(data)
		if err != nil {
			t.Fatalf("EstimateQuality failed: %v", err)
		}
		if estimate.Quality < 1 || estimate.Quality > 100 {
			t.Errorf("Invalid quality estimate: %d", estimate.Quality)
		}
	})

	t.Run("signature extraction setup", func(t *testing.T) {
		img := generateTestPatternImage(64, 64)
		data, err := WeeksEncodeToBytesStandard(img, 75)
		if err != nil {
			t.Fatalf("WeeksEncodeToBytes failed: %v", err)
		}

		// Extract signature (benchmark operation)
		sig, err := jpeg.ExtractSignature(data)
		if err != nil {
			t.Fatalf("ExtractSignature failed: %v", err)
		}
		if sig == nil {
			t.Fatal("Signature is nil")
		}
	})
}

// TestInfrastructure_FuzzInfrastructureCompiles tests that fuzz test
// infrastructure components compile and work correctly.
func TestInfrastructure_FuzzInfrastructureCompiles(t *testing.T) {
	// Test fuzz helper functions work
	t.Run("uniform table creation", func(t *testing.T) {
		table := fuzzCreateUniformTable(128)
		for i := 0; i < 64; i++ {
			if table[i] != 128 {
				t.Errorf("table[%d] = %d, want 128", i, table[i])
			}
		}
	})

	t.Run("random table creation", func(t *testing.T) {
		table := fuzzCreateRandomTable(42)
		// Verify all values are in valid range (1-255)
		for i := 0; i < 64; i++ {
			if table[i] < 1 || table[i] > 255 {
				t.Errorf("table[%d] = %d, out of range [1, 255]", i, table[i])
			}
		}

		// Verify reproducibility
		table2 := fuzzCreateRandomTable(42)
		for i := 0; i < 64; i++ {
			if table[i] != table2[i] {
				t.Errorf("Random table not reproducible at index %d", i)
			}
		}
	})

	t.Run("test image creation for fuzz", func(t *testing.T) {
		img := fuzzCreateTestImage(32, 32)
		if img == nil {
			t.Fatal("Image is nil")
		}

		bounds := img.Bounds()
		if bounds.Dx() != 32 || bounds.Dy() != 32 {
			t.Errorf("Dimensions: got %dx%d, want 32x32", bounds.Dx(), bounds.Dy())
		}
	})

	t.Run("fuzz encoder no panic on edge cases", func(t *testing.T) {
		// Test that encoder doesn't panic on various inputs
		testCases := []struct {
			name    string
			width   int
			height  int
			quality int
		}{
			{"1x1 image", 1, 1, 50},
			{"odd dimensions", 17, 23, 75},
			{"quality 1", 8, 8, 1},
			{"quality 100", 8, 8, 100},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic: %v", r)
					}
				}()

				img := fuzzCreateTestImage(tc.width, tc.height)
				_, _ = WeeksEncodeToBytesStandard(img, tc.quality)
			})
		}
	})

	t.Run("quality estimation from tables no panic", func(t *testing.T) {
		estimator := jpeg.NewQualityEstimator(nil)

		// Various table configurations
		testCases := []struct {
			name   string
			tables map[int][64]int
		}{
			{"empty tables", map[int][64]int{}},
			{"all zeros", map[int][64]int{0: {}}},
			{"all ones", map[int][64]int{0: fuzzCreateUniformTable(1)}},
			{"standard tables", map[int][64]int{
				0: jpeg.StandardLuminanceQuantTable,
				1: jpeg.StandardChrominanceQuantTable,
			}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic on %s: %v", tc.name, r)
					}
				}()

				_, _ = estimator.EstimateQualityFromTables(tc.tables)
			})
		}
	})
}

// TestInfrastructure_AllHelperFunctions tests that all helper functions work.
func TestInfrastructure_AllHelperFunctions(t *testing.T) {
	t.Run("generateTestPatternImage", func(t *testing.T) {
		img := generateTestPatternImage(64, 64)
		if img == nil {
			t.Fatal("Image is nil")
		}

		// Check that image has varied content (not all same color)
		colors := make(map[color.RGBA]bool)
		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := img.RGBAAt(x, y)
				colors[c] = true
			}
		}

		// Test pattern should have many unique colors
		if len(colors) < 10 {
			t.Errorf("Test pattern has too few unique colors: %d", len(colors))
		}
	})

	t.Run("generateSyntheticTestPattern", func(t *testing.T) {
		img := generateSyntheticTestPattern(64, 64)
		if img == nil {
			t.Fatal("Image is nil")
		}

		bounds := img.Bounds()
		if bounds.Dx() != 64 || bounds.Dy() != 64 {
			t.Errorf("Wrong dimensions: got %dx%d", bounds.Dx(), bounds.Dy())
		}
	})
}

// =============================================================================
// Helper Functions for Test Infrastructure
// =============================================================================

// generateTestPatternImage creates a test image with varied content including
// gradients, edges, and flat areas for comprehensive testing.
func generateTestPatternImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var r, g, b uint8

			// Divide image into quadrants with different patterns
			quadX := x < width/2
			quadY := y < height/2

			if quadX && quadY {
				// Top-left: Smooth gradient
				r = uint8((x * 255) / infraMax(width, 1))
				g = uint8((y * 255) / infraMax(height, 1))
				b = 128
			} else if !quadX && quadY {
				// Top-right: High-frequency checkerboard
				blockSize := 8
				isWhite := ((x/blockSize)+(y/blockSize))%2 == 0
				if isWhite {
					r, g, b = 255, 255, 255
				} else {
					r, g, b = 0, 0, 0
				}
				//goland:noinspection GoDfaConstantCondition
			} else if quadX && !quadY {
				// Bottom-left: Vertical stripes
				stripeWidth := 16
				intensity := 0
				if (x/stripeWidth)%2 == 0 {
					intensity = 255
				}
				r = uint8(intensity)
				g = uint8(intensity)
				b = uint8(intensity)
			} else {
				// Bottom-right: Mixed pattern
				noise := (x*7 + y*13) % 256
				base := (x + y) % 256
				r = uint8((noise + base) / 2)
				g = uint8((256 - noise + base) / 2)
				b = uint8((noise + 256 - base) / 2)
			}

			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}

// generateSyntheticTestPattern creates a synthetic test pattern.
func generateSyntheticTestPattern(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Simple gradient pattern
			r := uint8((x * 255) / infraMax(width, 1))
			g := uint8((y * 255) / infraMax(height, 1))
			b := uint8(((x + y) * 127) / infraMax(width+height, 1))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}

// fuzzCreateUniformTable creates a quantization table with all values set to val.
func fuzzCreateUniformTable(val int) [64]int {
	var table [64]int
	for i := range table {
		table[i] = val
	}
	return table
}

// fuzzCreateRandomTable creates a pseudo-random quantization table using a seed.
func fuzzCreateRandomTable(seed int) [64]int {
	var table [64]int
	state := seed
	for i := range table {
		// Simple LCG for reproducible random values
		state = (state*1103515245 + 12345) & 0x7FFFFFFF
		table[i] = (state % 255) + 1 // Values 1-255
	}
	return table
}

// fuzzCreateTestImage creates a test image with varied content.
func fuzzCreateTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x * 255) / infraMax(width-1, 1))
			g := uint8((y * 255) / infraMax(height-1, 1))
			b := uint8(((x + y) * 127) / infraMax(width+height-2, 1))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}

// infraMax returns the larger of two integers.
func infraMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
