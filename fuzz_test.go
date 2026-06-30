// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements fuzz testing infrastructure for the F5/James encoder.
// Migrated from jpeg/f5_fuzz_benchmark_test.go with updates for standalone package.
//
// Fuzzing targets:
//   - FuzzQualityEstimation: Tests quality estimation with random quantization tables
//   - FuzzJPEGSignature: Tests signature extraction with malformed JPEG data
//   - FuzzWeeksEncoder: Tests F5 encoder with random images and quality levels
//   - FuzzRoundTrip: Tests encode-decode cycles with random images
//   - FuzzNewWeeksEncoderQuality: Tests quality parameter validation
//   - FuzzMalformedImage: Tests handling of malformed image.Image implementations
//   - FuzzPixelPatterns: Tests DCT edge cases with various pixel patterns
//   - FuzzDimensionRoundTrip: Tests dimension fuzzing with decode verification

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
// Verification Tests for Fuzzing Infrastructure
// =============================================================================

// TestFuzzQualityEstimation_NoPanic verifies that quality estimation doesn't panic
// on random/malformed quantization tables.
func TestFuzzQualityEstimation_NoPanic(t *testing.T) {
	testCases := []struct {
		name   string
		tables map[int][64]int
	}{
		{
			name:   "empty_tables",
			tables: map[int][64]int{},
		},
		{
			name: "all_zeros",
			tables: map[int][64]int{
				0: {},
				1: {},
			},
		},
		{
			name: "all_max_values",
			tables: map[int][64]int{
				0: fzCreateUniformTable(255),
				1: fzCreateUniformTable(255),
			},
		},
		{
			name: "all_ones",
			tables: map[int][64]int{
				0: fzCreateUniformTable(1),
				1: fzCreateUniformTable(1),
			},
		},
		{
			name: "random_values",
			tables: map[int][64]int{
				0: fzCreateRandomTable(42),
				1: fzCreateRandomTable(43),
			},
		},
		{
			name: "single_table",
			tables: map[int][64]int{
				0: jpeg.StandardLuminanceQuantTable,
			},
		},
		{
			name: "high_table_ids",
			tables: map[int][64]int{
				3: jpeg.StandardLuminanceQuantTable,
			},
		},
		{
			name: "negative_values",
			tables: map[int][64]int{
				0: fzCreateUniformTable(-1),
			},
		},
		{
			name: "large_values",
			tables: map[int][64]int{
				0: fzCreateUniformTable(65535),
			},
		},
	}

	estimator := jpeg.NewQualityEstimator(nil)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic - capture any panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("EstimateQualityFromTables panicked with %s: %v", tc.name, r)
				}
			}()

			_, err := estimator.EstimateQualityFromTables(tc.tables)
			_ = err // Error is acceptable, panic is not
		})
	}
}

// TestFuzzJPEGSignature_NoPanic verifies that signature extraction doesn't panic
// on malformed JPEG data.
func TestFuzzJPEGSignature_NoPanic(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "empty_data",
			data: []byte{},
		},
		{
			name: "single_byte",
			data: []byte{0xFF},
		},
		{
			name: "no_soi",
			data: []byte{0x00, 0x00, 0x00},
		},
		{
			name: "soi_only",
			data: []byte{0xFF, 0xD8},
		},
		{
			name: "soi_eoi",
			data: []byte{0xFF, 0xD8, 0xFF, 0xD9},
		},
		{
			name: "truncated_marker_length",
			data: []byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00},
		},
		{
			name: "invalid_marker_length_zero",
			data: []byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x00},
		},
		{
			name: "marker_length_too_short",
			data: []byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x01},
		},
		{
			name: "truncated_dqt_data",
			data: []byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x43, 0x00},
		},
		{
			name: "all_ff_bytes",
			data: bytes.Repeat([]byte{0xFF}, 100),
		},
		{
			name: "random_garbage",
			data: []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0},
		},
		{
			name: "very_large_marker_length",
			data: []byte{0xFF, 0xD8, 0xFF, 0xDB, 0xFF, 0xFF},
		},
		{
			name: "multiple_soi",
			data: []byte{0xFF, 0xD8, 0xFF, 0xD8, 0xFF, 0xD8},
		},
		{
			name: "nested_markers",
			data: []byte{0xFF, 0xD8, 0xFF, 0xFE, 0x00, 0x10, 0xFF, 0xDB, 0x00, 0x05, 0x00, 0x01, 0x02},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ExtractSignature panicked with %s: %v", tc.name, r)
				}
			}()

			_, _ = jpeg.ExtractSignature(tc.data)
			// Error is acceptable, panic is not
		})
	}
}

// TestFuzzEncoder_NoPanic verifies that the F5 encoder doesn't panic
// on random images and quality levels.
func TestFuzzEncoder_NoPanic(t *testing.T) {
	testCases := []struct {
		name    string
		width   int
		height  int
		quality int
	}{
		{"8x8_q50", 8, 8, 50},
		{"1x1_q1", 1, 1, 1},
		{"1x1_q100", 1, 1, 100},
		{"16x16_q75", 16, 16, 75},
		{"8x8_q1", 8, 8, 1},
		{"8x8_q100", 8, 8, 100},
		{"17x17_q50", 17, 17, 50}, // Non-multiple of 8
		{"64x64_q90", 64, 64, 90},
		{"7x9_q25", 7, 9, 25}, // Odd dimensions
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("WeeksEncodeToBytes panicked with %s: %v", tc.name, r)
				}
			}()

			img := fzCreateTestImage(tc.width, tc.height)
			_, _ = WeeksEncodeToBytesStandard(img, tc.quality)
			// Error is acceptable for invalid quality, panic is not
		})
	}

	// Test invalid quality values (should return error, not panic)
	t.Run("invalid_quality_0", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("WeeksEncodeToBytes panicked with quality 0: %v", r)
			}
		}()
		img := fzCreateTestImage(8, 8)
		_, err := WeeksEncodeToBytesStandard(img, 0)
		if err == nil {
			t.Error("Expected error for quality 0")
		}
	})

	t.Run("invalid_quality_101", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("WeeksEncodeToBytes panicked with quality 101: %v", r)
			}
		}()
		img := fzCreateTestImage(8, 8)
		_, err := WeeksEncodeToBytesStandard(img, 101)
		if err == nil {
			t.Error("Expected error for quality 101")
		}
	})

	t.Run("nil_image", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("WeeksEncodeToBytes panicked with nil image: %v", r)
			}
		}()
		_, err := WeeksEncodeToBytes(nil, 50)
		if err == nil {
			t.Error("Expected error for nil image")
		}
	})
}

// =============================================================================
// Go 1.18+ Native Fuzz Test Targets
// =============================================================================

// FuzzQualityEstimation tests quality estimation with random quantization tables.
// It verifies that the estimator doesn't panic on edge cases and malformed data.
func FuzzQualityEstimation(f *testing.F) {
	// Add seed corpus for quantization table bytes
	// Standard luminance table at Q50
	f.Add([]byte{
		16, 11, 10, 16, 24, 40, 51, 61,
		12, 12, 14, 19, 26, 58, 60, 55,
		14, 13, 16, 24, 40, 57, 69, 56,
		14, 17, 22, 29, 51, 87, 80, 62,
		18, 22, 37, 56, 68, 109, 103, 77,
		24, 35, 55, 64, 81, 104, 113, 92,
		49, 64, 78, 87, 103, 121, 120, 101,
		72, 92, 95, 98, 112, 100, 103, 99,
	})

	// All ones (Q100)
	f.Add(bytes.Repeat([]byte{1}, 64))

	// All 255s (Q1)
	f.Add(bytes.Repeat([]byte{255}, 64))

	// All zeros (edge case)
	f.Add(bytes.Repeat([]byte{0}, 64))

	// Random values
	f.Add([]byte{
		23, 45, 67, 89, 12, 34, 56, 78,
		90, 11, 22, 33, 44, 55, 66, 77,
		88, 99, 10, 20, 30, 40, 50, 60,
		70, 80, 90, 100, 110, 120, 130, 140,
		150, 160, 170, 180, 190, 200, 210, 220,
		230, 240, 250, 5, 15, 25, 35, 45,
		55, 65, 75, 85, 95, 105, 115, 125,
		135, 145, 155, 165, 175, 185, 195, 205,
	})

	// Empty data
	f.Add([]byte{})

	// Partial table (less than 64 bytes)
	f.Add([]byte{16, 11, 10, 16, 24})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Create quantization table from fuzz data
		tables := make(map[int][64]int)

		if len(data) >= 64 {
			// Use first 64 bytes as luminance table
			var lumTable [64]int
			for i := 0; i < 64; i++ {
				lumTable[i] = int(data[i])
			}
			tables[0] = lumTable

			// If we have 128+ bytes, use next 64 as chrominance
			if len(data) >= 128 {
				var chromTable [64]int
				for i := 0; i < 64; i++ {
					chromTable[i] = int(data[64+i])
				}
				tables[1] = chromTable
			}
		} else if len(data) > 0 {
			// Use available bytes, pad with defaults
			var table [64]int
			for i := 0; i < 64; i++ {
				if i < len(data) {
					table[i] = int(data[i])
				} else {
					table[i] = 16 // Default value
				}
			}
			tables[0] = table
		}

		// Should not panic regardless of input
		estimator := jpeg.NewQualityEstimator(nil)
		_, err := estimator.EstimateQualityFromTables(tables)
		_ = err // Error is acceptable, panic is not
	})
}

// FuzzJPEGSignature tests JPEG signature extraction with malformed data.
// It ensures the signature extractor handles all edge cases gracefully.
func FuzzJPEGSignature(f *testing.F) {
	// Minimal valid JPEG (SOI + EOI)
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xD9})

	// JPEG with APP0 marker
	f.Add([]byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xE0, 0x00, 0x10, // APP0 with length
		0x4A, 0x46, 0x49, 0x46, 0x00, // "JFIF\0"
		0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00,
		0xFF, 0xD9, // EOI
	})

	// JPEG with COM marker (F5/James signature)
	f.Add([]byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xFE, 0x00, 0x45, // COM with length
		'J', 'P', 'E', 'G', ' ', 'E', 'n', 'c', 'o', 'd', 'e', 'r', ' ',
		'C', 'o', 'p', 'y', 'r', 'i', 'g', 'h', 't', ' ', '1', '9', '9', '8', ',', ' ',
		'J', 'a', 'm', 'e', 's', ' ', 'R', '.', ' ', 'W', 'e', 'e', 'k', 's', ' ',
		'a', 'n', 'd', ' ', 'B', 'i', 'o', 'E', 'l', 'e', 'c', 't', 'r', 'o', 'M', 'e', 'c', 'h', '.',
		0xFF, 0xD9, // EOI
	})

	// JPEG with DQT marker
	dqtData := []byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x43, 0x00}
	dqtData = append(dqtData, bytes.Repeat([]byte{16}, 64)...)
	dqtData = append(dqtData, 0xFF, 0xD9)
	f.Add(dqtData)

	// Empty data
	f.Add([]byte{})

	// Just SOI
	f.Add([]byte{0xFF, 0xD8})

	// No SOI marker
	f.Add([]byte{0x00, 0x00, 0xFF, 0xD9})

	// Truncated marker
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00})

	// All 0xFF bytes
	f.Add(bytes.Repeat([]byte{0xFF}, 100))

	// Random garbage
	f.Add([]byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0})

	f.Fuzz(func(_ *testing.T, data []byte) {
		// Should not panic regardless of input
		_, _ = jpeg.ExtractSignature(data)
	})
}

// FuzzWeeksEncoder tests the F5 encoder with random images and quality levels.
// It verifies that encoding doesn't panic on edge cases.
//
// Task 4.1: Enhanced with additional corpus seeds for edge cases.
func FuzzWeeksEncoder(f *testing.F) {
	// Seed corpus: width, height, quality, fill pattern

	// Task 4.1.1: 1x1 image (smallest valid)
	f.Add(uint8(1), uint8(1), uint8(50), []byte{128})

	// Task 4.1.2: 8x8 image (single MCU)
	f.Add(uint8(8), uint8(8), uint8(50), []byte{128, 128, 128})

	// Task 4.1.3: 7x9 image (non-8-multiple)
	f.Add(uint8(7), uint8(9), uint8(50), []byte{100, 150, 200})

	// Task 4.1.4: 15x17 image (MCU boundary edge case)
	f.Add(uint8(15), uint8(17), uint8(75), []byte{50, 100, 150})

	// Task 4.1.5: 128x128 image (maximum tested size)
	f.Add(uint8(128), uint8(128), uint8(75), []byte{64, 128, 192})

	// Original seed corpus entries
	f.Add(uint8(64), uint8(64), uint8(100), []byte{255, 255, 255})
	f.Add(uint8(16), uint8(16), uint8(75), []byte{0, 128, 255})
	f.Add(uint8(17), uint8(17), uint8(50), []byte{100, 150, 200}) // Non-8 multiple
	f.Add(uint8(0), uint8(0), uint8(50), []byte{})                // Zero size
	f.Add(uint8(255), uint8(255), uint8(50), []byte{128})         // Large

	// Additional edge case seeds
	f.Add(uint8(1), uint8(1), uint8(1), []byte{0})       // Minimum quality
	f.Add(uint8(1), uint8(1), uint8(100), []byte{255})   // Maximum quality
	f.Add(uint8(9), uint8(9), uint8(50), []byte{0, 255}) // Alternating pattern
	f.Add(uint8(31), uint8(31), uint8(50), []byte{128})  // Near MCU boundary
	f.Add(uint8(33), uint8(33), uint8(50), []byte{128})  // Just over MCU boundary

	f.Fuzz(func(t *testing.T, width, height, quality uint8, fillPattern []byte) {
		// Limit dimensions to prevent memory issues
		w := int(width)
		h := int(height)
		if w > 128 {
			w = 128
		}
		if h > 128 {
			h = 128
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}

		// Quality must be 1-100
		q := int(quality)
		if q < 1 {
			q = 1
		}
		if q > 100 {
			q = 100
		}

		// Create image with fuzz pattern
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		idx := 0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				var r, g, b uint8 = 128, 128, 128
				if len(fillPattern) > 0 {
					r = fillPattern[idx%len(fillPattern)]
					g = fillPattern[(idx+1)%len(fillPattern)]
					b = fillPattern[(idx+2)%len(fillPattern)]
					idx++
				}
				img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}

		// Should not panic
		_, _ = WeeksEncodeToBytesStandard(img, q)
	})
}

// FuzzRoundTrip tests encoding followed by decoding to verify no panics occur.
//
// Task 4.5: Enhanced with dimension fuzzing.
func FuzzRoundTrip(f *testing.F) {
	// Seed corpus: width, height, quality

	// Task 4.5.4: Problematic dimension combinations
	f.Add(uint8(1), uint8(1), uint8(50))     // Minimum
	f.Add(uint8(7), uint8(7), uint8(50))     // Non-MCU-aligned
	f.Add(uint8(8), uint8(8), uint8(50))     // Single block
	f.Add(uint8(9), uint8(9), uint8(50))     // Just over block
	f.Add(uint8(15), uint8(15), uint8(50))   // Just under 16
	f.Add(uint8(16), uint8(16), uint8(75))   // Exact MCU for 420
	f.Add(uint8(17), uint8(17), uint8(50))   // Just over MCU for 420
	f.Add(uint8(31), uint8(31), uint8(50))   // Two MCUs minus 1
	f.Add(uint8(32), uint8(32), uint8(90))   // Exact two MCUs
	f.Add(uint8(33), uint8(33), uint8(50))   // Two MCUs plus 1
	f.Add(uint8(64), uint8(64), uint8(25))   // Multiple MCUs
	f.Add(uint8(100), uint8(100), uint8(75)) // Larger

	// Asymmetric dimensions
	f.Add(uint8(1), uint8(64), uint8(50))  // Tall and thin
	f.Add(uint8(64), uint8(1), uint8(50))  // Wide and short
	f.Add(uint8(17), uint8(9), uint8(50))  // Asymmetric odd
	f.Add(uint8(15), uint8(33), uint8(50)) // Asymmetric MCU boundary

	// Quality extremes
	f.Add(uint8(16), uint8(16), uint8(1))   // Minimum quality
	f.Add(uint8(16), uint8(16), uint8(100)) // Maximum quality

	f.Fuzz(func(t *testing.T, width, height, quality uint8) {
		// Task 4.5.1: Fuzz dimensions in range 1-256 (clamped for memory)
		w := int(width)
		h := int(height)
		if w > 256 {
			w = 256
		}
		if h > 256 {
			h = 256
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}

		q := int(quality)
		if q < 1 {
			q = 1
		}
		if q > 100 {
			q = 100
		}

		// Create test image
		img := fzCreateTestImage(w, h)

		// Encode
		encoded, err := WeeksEncodeToBytesStandard(img, q)
		if err != nil {
			return // Encoding error is acceptable
		}

		// Task 4.5.2: Verify encoded output is decodable by image/jpeg
		decoded, err := stdjpeg.Decode(bytes.NewReader(encoded))
		if err != nil {
			t.Errorf("Decoding failed for %dx%d Q%d: %v", w, h, q, err)
			return
		}

		// Task 4.5.3: Verify decoded dimensions match original
		bounds := decoded.Bounds()
		if bounds.Dx() != w || bounds.Dy() != h {
			t.Errorf("Dimension mismatch: got %dx%d, want %dx%d",
				bounds.Dx(), bounds.Dy(), w, h)
		}
	})
}

// =============================================================================
// Task 4.2: Fuzz target for NewWeeksEncoder quality fuzzing
// =============================================================================

// FuzzNewWeeksEncoderQuality tests quality parameter validation across the full range.
// It verifies proper error handling for invalid values and no panics.
func FuzzNewWeeksEncoderQuality(f *testing.F) {
	// Task 4.2.4: Corpus seeds for boundary values
	f.Add(int32(0))   // Just below valid range
	f.Add(int32(1))   // Minimum valid
	f.Add(int32(100)) // Maximum valid
	f.Add(int32(101)) // Just above valid range

	// Task 4.2.1 & 4.2.2: Full range testing
	f.Add(int32(-1))          // Negative
	f.Add(int32(-100))        // More negative
	f.Add(int32(-2147483648)) // int32 min
	f.Add(int32(50))          // Mid-range valid
	f.Add(int32(200))         // Above range
	f.Add(int32(255))         // uint8 max
	f.Add(int32(256))         // Just over uint8
	f.Add(int32(1000))        // Large positive
	f.Add(int32(2147483647))  // int32 max

	f.Fuzz(func(t *testing.T, quality int32) {
		defer func() {
			// Task 4.2.3: Verify no panic from any input
			if r := recover(); r != nil {
				t.Errorf("NewWeeksEncoder panicked with quality %d: %v", quality, r)
			}
		}()

		var buf bytes.Buffer
		enc, err := NewWeeksEncoder(&buf, int(quality))

		// Validate expected behavior
		if quality >= 1 && quality <= 100 {
			// Valid quality should succeed
			if err != nil {
				t.Errorf("NewWeeksEncoder(%d) returned unexpected error: %v", quality, err)
			}
			if enc == nil {
				t.Errorf("NewWeeksEncoder(%d) returned nil encoder for valid quality", quality)
			}
		} else {
			// Invalid quality should return error
			if err == nil {
				t.Errorf("NewWeeksEncoder(%d) should return error for invalid quality", quality)
			}
			if enc != nil {
				t.Errorf("NewWeeksEncoder(%d) should return nil encoder on error", quality)
			}
		}
	})
}

// =============================================================================
// Task 4.3: Fuzz target for malformed image.Image implementations
// =============================================================================

// invalidBoundsImage returns invalid (negative or swapped) bounds.
type invalidBoundsImage struct {
	boundsType int // 0=negative width, 1=negative height, 2=swapped min/max
}

func (img invalidBoundsImage) ColorModel() color.Model { return color.RGBAModel }
func (img invalidBoundsImage) Bounds() image.Rectangle {
	switch img.boundsType {
	case 0:
		return image.Rect(10, 0, 5, 10) // Negative width (Max.X < Min.X)
	case 1:
		return image.Rect(0, 10, 10, 5) // Negative height (Max.Y < Min.Y)
	case 2:
		return image.Rect(100, 100, 50, 50) // Both negative
	default:
		return image.Rect(0, 0, 0, 0) // Zero dimensions
	}
}

//goland:noinspection GoUnusedParameter
func (img invalidBoundsImage) At(_x, _y int) color.Color {
	return color.RGBA{R: 128, G: 128, B: 128, A: 255}
}

// inconsistentBoundsImage returns different bounds on each call.
type inconsistentBoundsImage struct {
	callCount *int
}

func (img inconsistentBoundsImage) ColorModel() color.Model { return color.RGBAModel }
func (img inconsistentBoundsImage) Bounds() image.Rectangle {
	*img.callCount++
	// Vary bounds based on call count
	size := (*img.callCount % 3) + 1 // 1, 2, or 3
	return image.Rect(0, 0, size*8, size*8)
}
func (img inconsistentBoundsImage) At(x, y int) color.Color {
	return color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255}
}

// unexpectedColorImage returns unexpected color types.
type unexpectedColorImage struct {
	colorType int // 0=nil, 1=custom type
}

func (img unexpectedColorImage) ColorModel() color.Model { return color.RGBAModel }
func (img unexpectedColorImage) Bounds() image.Rectangle { return image.Rect(0, 0, 16, 16) }
func (img unexpectedColorImage) At(x, y int) color.Color {
	if img.colorType == 0 {
		// This shouldn't happen in practice, but test defensive handling
		return color.RGBA{R: 0, G: 0, B: 0, A: 0}
	}
	// Return a color with unusual values
	return color.RGBA64{
		R: uint16(x * 256),
		G: uint16(y * 256),
		B: uint16((x + y) * 128),
		A: 65535,
	}
}

// FuzzMalformedImage tests handling of malformed image.Image implementations.
func FuzzMalformedImage(f *testing.F) {
	// Corpus seeds for different malformed image types
	f.Add(uint8(0)) // Invalid bounds (negative width)
	f.Add(uint8(1)) // Invalid bounds (negative height)
	f.Add(uint8(2)) // Invalid bounds (swapped)
	f.Add(uint8(3)) // Zero dimensions
	f.Add(uint8(4)) // Inconsistent bounds
	f.Add(uint8(5)) // Unexpected color return
	f.Add(uint8(6)) // Unusual color type

	f.Fuzz(func(t *testing.T, malformationType uint8) {
		defer func() {
			// Task 4.3.4: Verify encoder handles malformed implementations gracefully
			if r := recover(); r != nil {
				t.Errorf("Encoder panicked with malformed image type %d: %v", malformationType, r)
			}
		}()

		var img image.Image

		switch malformationType % 7 {
		case 0:
			// Task 4.3.1: Invalid bounds - negative width
			img = invalidBoundsImage{boundsType: 0}
		case 1:
			// Task 4.3.1: Invalid bounds - negative height
			img = invalidBoundsImage{boundsType: 1}
		case 2:
			// Task 4.3.1: Invalid bounds - both negative
			img = invalidBoundsImage{boundsType: 2}
		case 3:
			// Task 4.3.1: Invalid bounds - zero dimensions
			img = invalidBoundsImage{boundsType: 3}
		case 4:
			// Task 4.3.2: Inconsistent bounds on multiple calls
			callCount := 0
			img = inconsistentBoundsImage{callCount: &callCount}
		case 5:
			// Task 4.3.3: Unexpected color return (zero alpha)
			img = unexpectedColorImage{colorType: 0}
		case 6:
			// Task 4.3.3: Unexpected color type (RGBA64)
			img = unexpectedColorImage{colorType: 1}
		}

		// Attempt to encode - should not panic, error is acceptable
		_, _ = WeeksEncodeToBytesStandard(img, 75)
	})
}

// TestMalformedImageHandling provides explicit tests for malformed images.
func TestMalformedImageHandling(t *testing.T) {
	t.Run("invalid_bounds_negative_width", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Encoder panicked: %v", r)
			}
		}()
		img := invalidBoundsImage{boundsType: 0}
		_, err := WeeksEncodeToBytesStandard(img, 75)
		if err == nil {
			t.Log("No error returned for invalid bounds (acceptable if handled)")
		}
	})

	t.Run("invalid_bounds_negative_height", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Encoder panicked: %v", r)
			}
		}()
		img := invalidBoundsImage{boundsType: 1}
		_, err := WeeksEncodeToBytesStandard(img, 75)
		if err == nil {
			t.Log("No error returned for invalid bounds (acceptable if handled)")
		}
	})

	t.Run("inconsistent_bounds", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Encoder panicked: %v", r)
			}
		}()
		callCount := 0
		img := inconsistentBoundsImage{callCount: &callCount}
		_, _ = WeeksEncodeToBytesStandard(img, 75)
		// Just verify no panic
	})

	t.Run("unexpected_color_type", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Encoder panicked: %v", r)
			}
		}()
		img := unexpectedColorImage{colorType: 1}
		_, err := WeeksEncodeToBytesStandard(img, 75)
		if err != nil {
			t.Logf("Error returned: %v", err)
		}
	})
}

// =============================================================================
// Task 4.4: Fuzz target for random pixel fill patterns
// =============================================================================

// FuzzPixelPatterns tests DCT edge cases with various pixel patterns.
func FuzzPixelPatterns(f *testing.F) {
	// Corpus seeds for different pattern types and dimensions

	// Task 4.4.1: Uniform color fills
	f.Add(uint8(0), uint8(32), uint8(32), uint8(0), uint8(0), uint8(0))       // Black
	f.Add(uint8(0), uint8(32), uint8(32), uint8(255), uint8(255), uint8(255)) // White
	f.Add(uint8(0), uint8(32), uint8(32), uint8(128), uint8(128), uint8(128)) // Gray

	// Task 4.4.2: Gradient patterns
	f.Add(uint8(1), uint8(64), uint8(64), uint8(0), uint8(0), uint8(0)) // Horizontal gradient
	f.Add(uint8(2), uint8(64), uint8(64), uint8(0), uint8(0), uint8(0)) // Vertical gradient
	f.Add(uint8(3), uint8(64), uint8(64), uint8(0), uint8(0), uint8(0)) // Diagonal gradient

	// Task 4.4.3: Random noise patterns
	f.Add(uint8(4), uint8(32), uint8(32), uint8(42), uint8(0), uint8(0)) // Noise seed 42
	f.Add(uint8(4), uint8(32), uint8(32), uint8(99), uint8(0), uint8(0)) // Noise seed 99

	// Task 4.4.4: Checkerboard patterns
	f.Add(uint8(5), uint8(32), uint8(32), uint8(0), uint8(255), uint8(0)) // B/W checker
	f.Add(uint8(5), uint8(64), uint8(64), uint8(0), uint8(128), uint8(0)) // B/Gray checker

	// Task 4.4.5: High-frequency patterns (exercise DCT edge cases)
	f.Add(uint8(6), uint8(32), uint8(32), uint8(1), uint8(0), uint8(0))  // Fine checker
	f.Add(uint8(6), uint8(64), uint8(64), uint8(2), uint8(0), uint8(0))  // Coarse checker
	f.Add(uint8(7), uint8(32), uint8(32), uint8(4), uint8(0), uint8(0))  // Horizontal stripes
	f.Add(uint8(8), uint8(32), uint8(32), uint8(4), uint8(0), uint8(0))  // Vertical stripes
	f.Add(uint8(9), uint8(32), uint8(32), uint8(8), uint8(0), uint8(0))  // Grid pattern
	f.Add(uint8(10), uint8(32), uint8(32), uint8(0), uint8(0), uint8(0)) // Sinusoidal

	f.Fuzz(func(t *testing.T, patternType, width, height, param1, param2, param3 uint8) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Encoder panicked with pattern type %d: %v", patternType, r)
			}
		}()

		// Limit dimensions
		w := int(width)
		h := int(height)
		if w > 128 {
			w = 128
		}
		if h > 128 {
			h = 128
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}

		img := image.NewRGBA(image.Rect(0, 0, w, h))

		switch patternType % 11 {
		case 0:
			// Uniform fill
			fzFillUniform(img, param1, param2, param3)
		case 1:
			// Horizontal gradient
			fzFillHorizontalGradient(img)
		case 2:
			// Vertical gradient
			fzFillVerticalGradient(img)
		case 3:
			// Diagonal gradient
			fzFillDiagonalGradient(img)
		case 4:
			// Random noise
			fzFillNoise(img, int(param1))
		case 5:
			// Checkerboard
			fzFillCheckerboard(img, param1, param2)
		case 6:
			// Fine checker (high frequency)
			blockSize := int(param1)
			if blockSize < 1 {
				blockSize = 1
			}
			fzFillCheckerboardBlocks(img, blockSize)
		case 7:
			// Horizontal stripes
			stripeHeight := int(param1)
			if stripeHeight < 1 {
				stripeHeight = 1
			}
			fzFillHorizontalStripes(img, stripeHeight)
		case 8:
			// Vertical stripes
			stripeWidth := int(param1)
			if stripeWidth < 1 {
				stripeWidth = 1
			}
			fzFillVerticalStripes(img, stripeWidth)
		case 9:
			// Grid pattern
			gridSize := int(param1)
			if gridSize < 1 {
				gridSize = 1
			}
			fzFillGrid(img, gridSize)
		case 10:
			// Sinusoidal pattern
			fzFillSinusoidal(img)
		}

		// Encode with different quality levels
		qualities := []int{1, 50, 100}
		for _, q := range qualities {
			encoded, err := WeeksEncodeToBytesStandard(img, q)
			if err != nil {
				continue // Error is acceptable
			}

			// Verify decodable
			_, err = stdjpeg.Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Errorf("Pattern %d, quality %d not decodable: %v", patternType, q, err)
			}
		}
	})
}

// TestPixelPatterns provides explicit tests for various pixel patterns.
func TestPixelPatterns(t *testing.T) {
	patterns := []struct {
		name string
		fill func(*image.RGBA)
	}{
		{"uniform_black", func(img *image.RGBA) { fzFillUniform(img, 0, 0, 0) }},
		{"uniform_white", func(img *image.RGBA) { fzFillUniform(img, 255, 255, 255) }},
		{"horizontal_gradient", fzFillHorizontalGradient},
		{"vertical_gradient", fzFillVerticalGradient},
		{"diagonal_gradient", fzFillDiagonalGradient},
		{"random_noise", func(img *image.RGBA) { fzFillNoise(img, 42) }},
		{"checkerboard", func(img *image.RGBA) { fzFillCheckerboard(img, 0, 255) }},
		{"fine_checker", func(img *image.RGBA) { fzFillCheckerboardBlocks(img, 1) }},
		{"horizontal_stripes", func(img *image.RGBA) { fzFillHorizontalStripes(img, 4) }},
		{"vertical_stripes", func(img *image.RGBA) { fzFillVerticalStripes(img, 4) }},
		{"grid", func(img *image.RGBA) { fzFillGrid(img, 8) }},
		{"sinusoidal", fzFillSinusoidal},
	}

	dimensions := []struct {
		w, h int
	}{
		{8, 8},   // Single block
		{16, 16}, // Single MCU
		{17, 17}, // Non-aligned
		{64, 64}, // Multiple MCUs
	}

	for _, pattern := range patterns {
		for _, dim := range dimensions {
			name := pattern.name + "_" + fzIntToStr(dim.w) + "x" + fzIntToStr(dim.h)
			t.Run(name, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Pattern %s panicked: %v", pattern.name, r)
					}
				}()

				img := image.NewRGBA(image.Rect(0, 0, dim.w, dim.h))
				pattern.fill(img)

				encoded, err := WeeksEncodeToBytesStandard(img, 75)
				if err != nil {
					t.Fatalf("Encoding failed: %v", err)
				}

				decoded, err := stdjpeg.Decode(bytes.NewReader(encoded))
				if err != nil {
					t.Fatalf("Decoding failed: %v", err)
				}

				bounds := decoded.Bounds()
				if bounds.Dx() != dim.w || bounds.Dy() != dim.h {
					t.Errorf("Dimension mismatch: got %dx%d, want %dx%d",
						bounds.Dx(), bounds.Dy(), dim.w, dim.h)
				}
			})
		}
	}
}

// =============================================================================
// Helper Functions for Fuzz Tests (prefixed with fz to avoid conflicts)
// =============================================================================

// fzCreateUniformTable creates a quantization table with all values set to val.
func fzCreateUniformTable(val int) [64]int {
	var table [64]int
	for i := range table {
		table[i] = val
	}
	return table
}

// fzCreateRandomTable creates a pseudo-random quantization table using a seed.
func fzCreateRandomTable(seed int) [64]int {
	var table [64]int
	state := seed
	for i := range table {
		// Simple LCG for reproducible random values
		state = (state*1103515245 + 12345) & 0x7FFFFFFF
		table[i] = (state % 255) + 1 // Values 1-255
	}
	return table
}

// fzCreateTestImage creates a test image with varied content.
func fzCreateTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create varied content: gradients and patterns
			r := uint8((x * 255) / fzMax(width-1, 1))
			g := uint8((y * 255) / fzMax(height-1, 1))
			b := uint8(((x + y) * 127) / fzMax(width+height-2, 1))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}

// fzMax returns the larger of two integers.
func fzMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// fzIntToStr converts an integer to string without fmt.
func fzIntToStr(n int) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// Pattern fill functions for Task 4.4

// fzFillUniform fills with a uniform color.
func fzFillUniform(img *image.RGBA, r, g, b uint8) {
	bounds := img.Bounds()
	c := color.RGBA{R: r, G: g, B: b, A: 255}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.Set(x, y, c)
		}
	}
}

// fzFillHorizontalGradient fills with a horizontal gradient.
func fzFillHorizontalGradient(img *image.RGBA) {
	bounds := img.Bounds()
	w := bounds.Dx()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			val := uint8(0)
			if w > 1 {
				val = uint8(((x - bounds.Min.X) * 255) / (w - 1))
			}
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}
}

// fzFillVerticalGradient fills with a vertical gradient.
func fzFillVerticalGradient(img *image.RGBA) {
	bounds := img.Bounds()
	h := bounds.Dy()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		val := uint8(0)
		if h > 1 {
			val = uint8(((y - bounds.Min.Y) * 255) / (h - 1))
		}
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}
}

// fzFillDiagonalGradient fills with a diagonal gradient.
func fzFillDiagonalGradient(img *image.RGBA) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	maxDist := w + h - 2
	if maxDist < 1 {
		maxDist = 1
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dist := (x - bounds.Min.X) + (y - bounds.Min.Y)
			val := uint8((dist * 255) / maxDist)
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}
}

// fzFillNoise fills with pseudo-random noise.
func fzFillNoise(img *image.RGBA, seed int) {
	bounds := img.Bounds()
	state := seed
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Simple LCG
			state = (state*1103515245 + 12345) & 0x7FFFFFFF
			val := uint8(state % 256)
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}
}

// fzFillCheckerboard fills with a pixel-level checkerboard.
func fzFillCheckerboard(img *image.RGBA, c1, c2 uint8) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			val := c1
			if (x+y)%2 == 1 {
				val = c2
			}
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}
}

// fzFillCheckerboardBlocks fills with block-level checkerboard.
func fzFillCheckerboardBlocks(img *image.RGBA, blockSize int) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			blockX := (x - bounds.Min.X) / blockSize
			blockY := (y - bounds.Min.Y) / blockSize
			val := uint8(0)
			if (blockX+blockY)%2 == 0 {
				val = 255
			}
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}
}

// fzFillHorizontalStripes fills with horizontal stripes.
func fzFillHorizontalStripes(img *image.RGBA, stripeHeight int) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		stripeNum := (y - bounds.Min.Y) / stripeHeight
		val := uint8(0)
		if stripeNum%2 == 0 {
			val = 255
		}
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}
}

// fzFillVerticalStripes fills with vertical stripes.
func fzFillVerticalStripes(img *image.RGBA, stripeWidth int) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			stripeNum := (x - bounds.Min.X) / stripeWidth
			val := uint8(0)
			if stripeNum%2 == 0 {
				val = 255
			}
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}
}

// fzFillGrid fills with a grid pattern.
func fzFillGrid(img *image.RGBA, gridSize int) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			relX := (x - bounds.Min.X) % gridSize
			relY := (y - bounds.Min.Y) % gridSize
			val := uint8(255)
			if relX == 0 || relY == 0 {
				val = 0
			}
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}
}

// fzFillSinusoidal fills with a sinusoidal pattern (approximated).
func fzFillSinusoidal(img *image.RGBA) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Precompute a simple sine approximation lookup
	// Using integer math to avoid math package dependency
	sinLookup := make([]int, 256)
	for i := 0; i < 256; i++ {
		// Approximate sin using a polynomial
		// sin(x) for x in [0, 2*pi] mapped to [0, 256]
		// Using: sin(x) ~ x - x^3/6 for small x, scaled
		angle := (i * 628) / 256 // Scale to [0, 2*pi * 100]
		// Shift to [-pi, pi] range
		if angle > 314 {
			angle = angle - 628
		}
		// Approximate sin * 127 + 128
		sin := angle - (angle*angle*angle)/(6*10000)
		val := (sin * 127) / 314
		sinLookup[i] = val + 128
		if sinLookup[i] < 0 {
			sinLookup[i] = 0
		}
		if sinLookup[i] > 255 {
			sinLookup[i] = 255
		}
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Create sinusoidal pattern
			idx := 0
			if w > 1 {
				idx = ((x - bounds.Min.X) * 256) / w
			}
			idy := 0
			if h > 1 {
				idy = ((y - bounds.Min.Y) * 256) / h
			}

			combined := (idx + idy) % 256
			val := uint8(sinLookup[combined])
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}
}
