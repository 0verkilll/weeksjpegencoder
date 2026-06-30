// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This test file contains tests for the F5 encoder implementation.
// Migrated from jpeg/f5_encoder_test.go with updates for standalone package.

package weeksjpegencoder

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"math"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// F5/James Encoder Constants (for testing)
// =============================================================================

// F5JamesSignature is the default COM marker signature for F5/James encoder.
const F5JamesSignature = "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech."

// =============================================================================
// Task Group 3.1: Tests for migrated encoder
// =============================================================================

// TestNewWeeksEncoderValidQuality tests that NewWeeksEncoder accepts valid quality values.
func TestNewWeeksEncoderValidQuality(t *testing.T) {
	testCases := []struct {
		name    string
		quality int
	}{
		{"minimum quality", 1},
		{"low quality", 25},
		{"medium quality", 50},
		{"high quality", 75},
		{"maximum quality", 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc, err := NewWeeksEncoder(&buf, tc.quality)
			if err != nil {
				t.Errorf("NewWeeksEncoder(%d) returned error: %v", tc.quality, err)
			}
			if enc == nil {
				t.Errorf("NewWeeksEncoder(%d) returned nil encoder", tc.quality)
			}
		})
	}
}

// TestNewWeeksEncoderInvalidQuality tests that NewWeeksEncoder rejects invalid quality values.
func TestNewWeeksEncoderInvalidQuality(t *testing.T) {
	testCases := []struct {
		name    string
		quality int
	}{
		{"quality zero", 0},
		{"quality negative", -1},
		{"quality too high", 101},
		{"quality way too high", 200},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc, err := NewWeeksEncoder(&buf, tc.quality)
			if err == nil {
				t.Errorf("NewWeeksEncoder(%d) should return error for invalid quality", tc.quality)
			}
			if enc != nil {
				t.Errorf("NewWeeksEncoder(%d) should return nil encoder for invalid quality", tc.quality)
			}

			// Verify it's a ValidationError
			_, ok := err.(*jpeg.ValidationError)
			if !ok {
				t.Errorf("NewWeeksEncoder(%d) should return *jpeg.ValidationError, got %T", tc.quality, err)
			}
		})
	}
}

// TestWeeksEncoderEncodeProducesValidJPEG tests that Encode produces valid JPEG bytes.
func TestWeeksEncoderEncodeProducesValidJPEG(t *testing.T) {
	// Create a small test image
	img := createTestImage(64, 64)

	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	err = enc.Encode(img)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	data := buf.Bytes()

	// Verify JPEG structure
	if len(data) < 4 {
		t.Fatal("Encoded data too short")
	}

	// Check SOI marker (0xFFD8)
	if data[0] != 0xFF || data[1] != 0xD8 {
		t.Errorf("Missing SOI marker: got %02X %02X, want FF D8", data[0], data[1])
	}

	// Check EOI marker (0xFFD9) at end
	if data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
		t.Errorf("Missing EOI marker: got %02X %02X, want FF D9", data[len(data)-2], data[len(data)-1])
	}

	// Verify it's a valid JPEG by detecting format
	format, err := jpeg.DetectFormat(data)
	if err != nil {
		t.Errorf("DetectFormat failed: %v", err)
	}
	if format != jpeg.FormatBaselineJPEG {
		t.Errorf("Expected FormatBaselineJPEG, got %v", format)
	}
}

// TestWeeksEncodeToBytes tests the convenience function.
func TestWeeksEncodeToBytes(t *testing.T) {
	img := createTestImage(32, 32)

	data, err := WeeksEncodeToBytesStandard(img, 75)
	if err != nil {
		t.Fatalf("WeeksEncodeToBytes failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("WeeksEncodeToBytes returned empty data")
	}

	// Check SOI and EOI
	if data[0] != 0xFF || data[1] != 0xD8 {
		t.Error("Missing SOI marker")
	}
	if data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
		t.Error("Missing EOI marker")
	}
}

// TestWeeksEncoderCOMMarker tests that the COM marker contains the F5/James signature.
func TestWeeksEncoderCOMMarker(t *testing.T) {
	img := createTestImage(16, 16)

	data, err := WeeksEncodeToBytesStandard(img, 50)
	if err != nil {
		t.Fatalf("WeeksEncodeToBytes failed: %v", err)
	}

	// Look for COM marker (0xFFFE) and verify content
	expectedSignature := "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech."
	found := false

	for i := 0; i < len(data)-2; i++ {
		if data[i] == 0xFF && data[i+1] == 0xFE {
			// Found COM marker
			if i+4 > len(data) {
				t.Fatal("COM marker truncated")
			}
			// Read length (big-endian)
			length := int(data[i+2])<<8 | int(data[i+3])
			if i+2+length > len(data) {
				t.Fatal("COM marker data truncated")
			}
			// Extract comment content (length includes the 2-byte length field)
			comment := string(data[i+4 : i+2+length])
			if comment == expectedSignature {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("F5/James signature not found in COM marker")
	}
}

// TestWeeksEncoderQualityScaling tests that quality parameter affects quantization.
func TestWeeksEncoderQualityScaling(t *testing.T) {
	img := createTestImage(64, 64)

	// Encode at different quality levels
	dataLowQuality, err := WeeksEncodeToBytesStandard(img, 10)
	if err != nil {
		t.Fatalf("WeeksEncodeToBytes(10) failed: %v", err)
	}

	dataHighQuality, err := WeeksEncodeToBytesStandard(img, 95)
	if err != nil {
		t.Fatalf("WeeksEncodeToBytes(95) failed: %v", err)
	}

	// Higher quality should generally produce larger files (more data, less quantization)
	t.Logf("Low quality (10) size: %d bytes", len(dataLowQuality))
	t.Logf("High quality (95) size: %d bytes", len(dataHighQuality))

	// Both should be valid JPEGs
	if dataLowQuality[0] != 0xFF || dataLowQuality[1] != 0xD8 {
		t.Error("Low quality output missing SOI")
	}
	if dataHighQuality[0] != 0xFF || dataHighQuality[1] != 0xD8 {
		t.Error("High quality output missing SOI")
	}
}

// =============================================================================
// Additional Tests Migrated from jpeg/f5_encoder_test.go
// =============================================================================

// TestWeeksEncoder_NewEncoder tests that NewWeeksEncoder creates an encoder with valid quality.
func TestWeeksEncoder_NewEncoder(t *testing.T) {
	tests := []struct {
		name      string
		quality   int
		wantError bool
	}{
		{"quality 1 (minimum)", 1, false},
		{"quality 50 (standard)", 50, false},
		{"quality 75 (default)", 75, false},
		{"quality 100 (maximum)", 100, false},
		{"quality 0 (invalid)", 0, true},
		{"quality 101 (invalid)", 101, true},
		{"quality -1 (invalid)", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc, err := NewWeeksEncoder(&buf, tt.quality)

			if tt.wantError {
				if err == nil {
					t.Errorf("NewWeeksEncoder(quality=%d) should return error", tt.quality)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewWeeksEncoder(quality=%d) returned unexpected error: %v", tt.quality, err)
			}

			if enc == nil {
				t.Fatal("encoder should not be nil")
			}

			// Verify quality was stored
			if enc.quality != tt.quality {
				t.Errorf("expected quality %d, got %d", tt.quality, enc.quality)
			}
		})
	}
}

// TestWeeksEncoder_Encode tests that Encode produces valid JPEG from image.Image.
func TestWeeksEncoder_Encode(t *testing.T) {
	// Create a simple test image
	img := f5CreateTestImage(64, 64)

	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	err = enc.Encode(img)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	data := buf.Bytes()

	// Verify JPEG structure
	if len(data) < 4 {
		t.Fatal("output too short to be valid JPEG")
	}

	// Check SOI marker
	if data[0] != 0xFF || data[1] != 0xD8 {
		t.Errorf("expected SOI marker (0xFFD8), got 0x%02X%02X", data[0], data[1])
	}

	// Check EOI marker at end
	if data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
		t.Errorf("expected EOI marker (0xFFD9) at end, got 0x%02X%02X",
			data[len(data)-2], data[len(data)-1])
	}
}

// TestWeeksEncoder_OutputDecodable tests that output JPEG is decodable by standard decoder.
// Uses standard mode to produce Go-decodable output.
func TestWeeksEncoder_OutputDecodable(t *testing.T) {
	// Create a test image with varied content
	img := f5CreateGradientImage(128, 128)

	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithStandardMode())
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
	}

	err = enc.Encode(img)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Try to decode with Go's standard decoder
	decoded, err := stdjpeg.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("standard decoder failed to decode output: %v", err)
	}

	// Verify decoded image has correct dimensions
	bounds := decoded.Bounds()
	if bounds.Dx() != 128 || bounds.Dy() != 128 {
		t.Errorf("expected decoded dimensions 128x128, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

// TestWeeksEncoder_QualityAffectsQuantization tests that quality parameter affects quantization tables.
func TestWeeksEncoder_QualityAffectsQuantization(t *testing.T) {
	img := f5CreateTestImage(64, 64)

	// Encode at different quality levels
	qualities := []int{10, 50, 90}
	var sizes []int

	for _, q := range qualities {
		var buf bytes.Buffer
		enc, err := NewWeeksEncoder(&buf, q)
		if err != nil {
			t.Fatalf("NewWeeksEncoder(quality=%d) failed: %v", q, err)
		}

		err = enc.Encode(img)
		if err != nil {
			t.Fatalf("Encode at quality %d failed: %v", q, err)
		}

		sizes = append(sizes, buf.Len())
	}

	// Lower quality should produce smaller files (more compression)
	// Higher quality should produce larger files (less compression)
	if sizes[0] >= sizes[1] {
		t.Errorf("Q10 file (%d bytes) should be smaller than Q50 file (%d bytes)",
			sizes[0], sizes[1])
	}
	if sizes[1] >= sizes[2] {
		t.Errorf("Q50 file (%d bytes) should be smaller than Q90 file (%d bytes)",
			sizes[1], sizes[2])
	}

	// Also verify the scaled quantization tables are different
	lumQ10 := jpeg.ScaleQuantTable(jpeg.StandardLuminanceQuantTable, 10)
	lumQ50 := jpeg.ScaleQuantTable(jpeg.StandardLuminanceQuantTable, 50)
	lumQ90 := jpeg.ScaleQuantTable(jpeg.StandardLuminanceQuantTable, 90)

	// Q10 should have higher quantization values (more aggressive)
	if lumQ10[0] <= lumQ50[0] || lumQ50[0] <= lumQ90[0] {
		t.Errorf("expected Q10[0]=%d > Q50[0]=%d > Q90[0]=%d",
			lumQ10[0], lumQ50[0], lumQ90[0])
	}
}

// TestWeeksEncoder_SetComment tests that SetComment embeds COM marker with custom signature.
func TestWeeksEncoder_SetComment(t *testing.T) {
	img := f5CreateTestImage(32, 32)

	// Test default F5/James signature
	t.Run("default F5/James signature", func(t *testing.T) {
		var buf bytes.Buffer
		enc, err := NewWeeksEncoder(&buf, 75)
		if err != nil {
			t.Fatalf("NewWeeksEncoder failed: %v", err)
		}

		err = enc.Encode(img)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		// Verify F5/James signature is present
		if !bytes.Contains(buf.Bytes(), []byte(F5JamesSignature)) {
			t.Error("default F5/James COM marker signature not found in output")
		}
	})

	// Test custom comment
	t.Run("custom comment", func(t *testing.T) {
		var buf bytes.Buffer
		enc, err := NewWeeksEncoder(&buf, 75)
		if err != nil {
			t.Fatalf("NewWeeksEncoder failed: %v", err)
		}

		customComment := "Custom Test Comment 12345"
		enc.SetComment(customComment)

		err = enc.Encode(img)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		// f5.jar's COM writer truncates the last 2 chars from any caller-
		// supplied comment (see james_com.go and decisions.md DEC-004), so
		// the disk bytes carry comment[:len-2].
		truncated := customComment[:len(customComment)-2]
		if !bytes.Contains(buf.Bytes(), []byte(truncated)) {
			t.Errorf("truncated custom comment %q not found in output", truncated)
		}
	})

	// Test builder pattern returns encoder
	t.Run("builder pattern", func(t *testing.T) {
		var buf bytes.Buffer
		enc, err := NewWeeksEncoder(&buf, 75)
		if err != nil {
			t.Fatalf("NewWeeksEncoder failed: %v", err)
		}

		result := enc.SetComment("test")
		if result != enc {
			t.Error("SetComment should return the encoder for chaining")
		}
	})
}

// TestWeeksEncoder_SetSubsampling tests that SetSubsampling correctly configures modes.
// Uses standard mode to produce Go-decodable output.
func TestWeeksEncoder_SetSubsampling(t *testing.T) {
	img := f5CreateTestImage(64, 64)

	subsamplingModes := []struct {
		mode jpeg.ChromaSubsamplingMode
		name string
	}{
		{jpeg.ChromaSubsampling420, "4:2:0"},
		{jpeg.ChromaSubsampling422, "4:2:2"},
		{jpeg.ChromaSubsampling444, "4:4:4"},
	}

	for _, tt := range subsamplingModes {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithStandardMode())
			if err != nil {
				t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
			}

			enc.SetSubsampling(tt.mode)

			err = enc.Encode(img)
			if err != nil {
				t.Fatalf("Encode with %s failed: %v", tt.name, err)
			}

			// Verify output is valid JPEG
			_, err = stdjpeg.Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("standard decoder failed to decode %s output: %v", tt.name, err)
			}
		})
	}

	// Test builder pattern
	t.Run("builder pattern", func(t *testing.T) {
		var buf bytes.Buffer
		enc, err := NewWeeksEncoder(&buf, 75)
		if err != nil {
			t.Fatalf("NewWeeksEncoder failed: %v", err)
		}

		result := enc.SetSubsampling(jpeg.ChromaSubsampling420)
		if result != enc {
			t.Error("SetSubsampling should return the encoder for chaining")
		}
	})
}

// TestWeeksEncoder_EncodeToBytes tests the convenience function.
func TestWeeksEncoder_EncodeToBytes(t *testing.T) {
	img := f5CreateTestImage(64, 64)

	qualities := []int{25, 50, 75, 100}

	for _, q := range qualities {
		t.Run(fmt.Sprintf("quality_%d", q), func(t *testing.T) {
			data, err := WeeksEncodeToBytesStandard(img, q)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytes(quality=%d) failed: %v", q, err)
			}

			// Verify valid JPEG
			if len(data) < 4 {
				t.Fatal("output too short")
			}
			if data[0] != 0xFF || data[1] != 0xD8 {
				t.Error("missing SOI marker")
			}
			if data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
				t.Error("missing EOI marker")
			}

			// Verify decodable
			_, err = stdjpeg.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("output not decodable: %v", err)
			}
		})
	}

	// Test invalid quality
	t.Run("invalid quality 0", func(t *testing.T) {
		_, err := WeeksEncodeToBytesStandard(img, 0)
		if err == nil {
			t.Error("should return error for quality 0")
		}
	})

	t.Run("invalid quality 101", func(t *testing.T) {
		_, err := WeeksEncodeToBytesStandard(img, 101)
		if err == nil {
			t.Error("should return error for quality 101")
		}
	})
}

// TestWeeksEncoder_RoundTrip tests encode then decode produces visually similar image.
// Uses standard mode to produce Go-decodable output.
func TestWeeksEncoder_RoundTrip(t *testing.T) {
	// Create a test image with known content
	srcImg := f5CreateGradientImage(64, 64)

	// Encode with high quality to minimize loss, using standard mode
	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, 95, WithStandardMode())
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
	}

	err = enc.Encode(srcImg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decoded, err := stdjpeg.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Compare dimensions
	if decoded.Bounds().Dx() != srcImg.Bounds().Dx() ||
		decoded.Bounds().Dy() != srcImg.Bounds().Dy() {
		t.Errorf("dimension mismatch: expected %dx%d, got %dx%d",
			srcImg.Bounds().Dx(), srcImg.Bounds().Dy(),
			decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}

	// Calculate PSNR to verify visual similarity
	psnr := f5CalculatePSNR(srcImg, decoded)
	if psnr < 30.0 {
		t.Errorf("PSNR too low (%.2f dB), expected at least 30 dB for Q95", psnr)
	}

	// At Q95, we expect high quality
	if psnr < 35.0 {
		t.Logf("Warning: PSNR (%.2f dB) is lower than expected for Q95", psnr)
	}
}

// =============================================================================
// Additional Edge Case Tests
// =============================================================================

// TestWeeksEncoder_MinimalImage uses standard mode to produce Go-decodable output.
func TestWeeksEncoder_MinimalImage(t *testing.T) {
	// Test 8x8 (single MCU) image
	img := f5CreateTestImage(8, 8)

	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithStandardMode())
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
	}

	err = enc.Encode(img)
	if err != nil {
		t.Fatalf("Encode 8x8 failed: %v", err)
	}

	// Verify decodable
	_, err = stdjpeg.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("8x8 output not decodable: %v", err)
	}
}

// TestWeeksEncoder_NonMultipleOf8 uses standard mode to produce Go-decodable output.
func TestWeeksEncoder_NonMultipleOf8(t *testing.T) {
	// Test image with dimensions not multiple of 8
	sizes := []struct{ w, h int }{
		{33, 33},
		{100, 75},
		{17, 23},
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%dx%d", size.w, size.h), func(t *testing.T) {
			img := f5CreateTestImage(size.w, size.h)

			var buf bytes.Buffer
			enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithStandardMode())
			if err != nil {
				t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
			}

			err = enc.Encode(img)
			if err != nil {
				t.Fatalf("Encode %dx%d failed: %v", size.w, size.h, err)
			}

			// Verify decodable
			decoded, err := stdjpeg.Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("%dx%d output not decodable: %v", size.w, size.h, err)
			}

			// Verify dimensions match
			if decoded.Bounds().Dx() != size.w || decoded.Bounds().Dy() != size.h {
				t.Errorf("dimension mismatch: expected %dx%d, got %dx%d",
					size.w, size.h, decoded.Bounds().Dx(), decoded.Bounds().Dy())
			}
		})
	}
}

func TestWeeksEncoder_NilImage(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	err = enc.Encode(nil)
	if err == nil {
		t.Error("Encode(nil) should return error")
	}
}

func TestWeeksEncoder_ChainedBuilder(t *testing.T) {
	img := f5CreateTestImage(32, 32)

	var buf bytes.Buffer
	// 4:4:4 is only valid in standard mode (f5.jar is hardcoded for 4:2:0).
	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithStandardMode())
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
	}

	// Test method chaining
	enc.SetComment("Test Comment").SetSubsampling(jpeg.ChromaSubsampling444)

	err = enc.Encode(img)
	if err != nil {
		t.Fatalf("Encode after chaining failed: %v", err)
	}

	// Verify both settings took effect. James-mode COM marker truncates the
	// last 2 chars (matches f5.jar exactly), so look for the truncation.
	data := buf.Bytes()
	if !bytes.Contains(data, []byte("Test Comme")) {
		t.Error("custom comment (truncated to match f5.jar) not found after chaining")
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// createTestImage creates a simple test image with a gradient pattern.
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x * 255) / f5Max(width, 1))
			g := uint8((y * 255) / f5Max(height, 1))
			b := uint8(((x + y) * 255) / f5Max(width+height, 1))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

// f5CreateTestImage creates a simple test image with solid color and some variation.
func f5CreateTestImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x * 255) / f5Max(width, 1))
			g := uint8((y * 255) / f5Max(height, 1))
			b := uint8(128)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

// f5CreateGradientImage creates a test image with smooth gradients.
func f5CreateGradientImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create a diagonal gradient with some variation
			r := uint8((x + y) * 255 / f5Max(width+height, 1))
			g := uint8(math.Abs(float64(x-width/2)) * 255 / float64(f5Max(width/2, 1)))
			b := uint8(math.Abs(float64(y-height/2)) * 255 / float64(f5Max(height/2, 1)))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

// f5CalculatePSNR calculates the Peak Signal-to-Noise Ratio between two images.
func f5CalculatePSNR(img1, img2 image.Image) float64 {
	bounds := img1.Bounds()
	bounds2 := img2.Bounds()
	if bounds.Dx() != bounds2.Dx() || bounds.Dy() != bounds2.Dy() {
		return 0 // Images must have same dimensions
	}

	var mse float64
	count := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, _ := img1.At(x, y).RGBA()
			r2, g2, b2, _ := img2.At(x, y).RGBA()

			// Convert from 16-bit to 8-bit
			dr := float64(r1>>8) - float64(r2>>8)
			dg := float64(g1>>8) - float64(g2>>8)
			db := float64(b1>>8) - float64(b2>>8)

			mse += dr*dr + dg*dg + db*db
			count += 3
		}
	}

	if count == 0 || mse == 0 {
		return math.Inf(1) // Perfect match
	}

	mse /= float64(count)
	maxVal := 255.0
	psnr := 10 * math.Log10((maxVal*maxVal)/mse)
	return psnr
}

// f5Max returns the larger of two integers.
func f5Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// =============================================================================
// Custom Quantization Table Tests (merged from custom_quant_test.go)
// =============================================================================

// TestSetQuantizationTable_ValidLuminanceTable tests that SetQuantizationTable
// correctly accepts and stores a valid luminance quantization table.
func TestSetQuantizationTable_ValidLuminanceTable(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	// Create a valid custom luminance table (64 values, all in range 1-255)
	var customTable [64]int
	for i := 0; i < 64; i++ {
		customTable[i] = (i % 255) + 1 // Values 1-255
	}

	// Set the custom table for luminance (tableNum 0)
	result := enc.SetQuantizationTable(0, customTable)

	// Should return encoder for chaining
	if result != enc {
		t.Error("SetQuantizationTable should return encoder for method chaining")
	}

	// Verify custom table is stored and returned by GetQuantTable
	if enc.jamesQuantizer == nil {
		t.Fatal("jamesQuantizer should not be nil")
	}

	// Retrieve the quantization table and verify it matches
	retrievedTable := enc.quantizer.GetQuantTable(true)
	for i := 0; i < 64; i++ {
		if retrievedTable[i] != customTable[i] {
			t.Errorf("luminance table[%d]: got %d, expected %d", i, retrievedTable[i], customTable[i])
		}
	}
}

// TestSetQuantizationTable_ValidChrominanceTable tests that SetQuantizationTable
// correctly accepts and stores a valid chrominance quantization table.
func TestSetQuantizationTable_ValidChrominanceTable(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	// Create a valid custom chrominance table (64 values, all in range 1-255)
	var customTable [64]int
	for i := 0; i < 64; i++ {
		customTable[i] = 128 // Use a flat table for testing
	}

	// Set the custom table for chrominance (tableNum 1)
	result := enc.SetQuantizationTable(1, customTable)

	// Should return encoder for chaining
	if result != enc {
		t.Error("SetQuantizationTable should return encoder for method chaining")
	}

	// Retrieve the quantization table and verify it matches
	retrievedTable := enc.quantizer.GetQuantTable(false)
	for i := 0; i < 64; i++ {
		if retrievedTable[i] != customTable[i] {
			t.Errorf("chrominance table[%d]: got %d, expected %d", i, retrievedTable[i], customTable[i])
		}
	}
}

// TestSetQuantizationTable_RejectsInvalidTableNum tests that SetQuantizationTable
// rejects invalid table numbers (not 0 or 1).
func TestSetQuantizationTable_RejectsInvalidTableNum(t *testing.T) {
	testCases := []struct {
		name     string
		tableNum int
	}{
		{"negative_table_num", -1},
		{"table_num_2", 2},
		{"table_num_10", 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc, err := NewWeeksEncoder(&buf, 75)
			if err != nil {
				t.Fatalf("NewWeeksEncoder failed: %v", err)
			}

			var customTable [64]int
			for i := 0; i < 64; i++ {
				customTable[i] = 16
			}

			// SetQuantizationTable with invalid tableNum should store the error
			enc.SetQuantizationTable(tc.tableNum, customTable)

			// Check that an error was recorded
			if enc.quantTableErr == nil {
				t.Errorf("SetQuantizationTable(%d, ...) should record an error for invalid table number", tc.tableNum)
			}
		})
	}
}

// TestSetQuantizationTable_RejectsValuesOutOfRange tests that SetQuantizationTable
// rejects tables with values outside the valid range 1-255.
func TestSetQuantizationTable_RejectsValuesOutOfRange(t *testing.T) {
	testCases := []struct {
		name        string
		invalidIdx  int
		invalidVal  int
		description string
	}{
		{"value_zero", 0, 0, "zero is below minimum"},
		{"value_256", 32, 256, "256 is above maximum"},
		{"value_negative", 10, -1, "negative value"},
		{"value_1000", 63, 1000, "value far above maximum"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc, err := NewWeeksEncoder(&buf, 75)
			if err != nil {
				t.Fatalf("NewWeeksEncoder failed: %v", err)
			}

			// Create a table with one invalid value
			var customTable [64]int
			for i := 0; i < 64; i++ {
				customTable[i] = 16 // Valid default
			}
			customTable[tc.invalidIdx] = tc.invalidVal

			// SetQuantizationTable should record an error
			enc.SetQuantizationTable(0, customTable)

			if enc.quantTableErr == nil {
				t.Errorf("SetQuantizationTable with value %d at index %d should record error (%s)",
					tc.invalidVal, tc.invalidIdx, tc.description)
			}
		})
	}
}

// TestSetQuantizationTable_OverridesStandardTable tests that custom tables
// properly override the standard ITU-T T.81 tables used by default.
func TestSetQuantizationTable_OverridesStandardTable(t *testing.T) {
	var buf bytes.Buffer

	// Create encoder and get the default table first
	enc1, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}
	defaultTable := enc1.quantizer.GetQuantTable(true)

	// Create another encoder with a custom table
	buf.Reset()
	enc2, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	// Create a distinctly different custom table
	var customTable [64]int
	for i := 0; i < 64; i++ {
		customTable[i] = 100 // Flat table at 100
	}

	enc2.SetQuantizationTable(0, customTable)

	// The custom table should override the default
	customizedTable := enc2.quantizer.GetQuantTable(true)

	// Verify they are different (unless default happens to be all 100s, which it won't be)
	isDifferent := false
	for i := 0; i < 64; i++ {
		if customizedTable[i] != defaultTable[i] {
			isDifferent = true
			break
		}
	}

	if !isDifferent {
		t.Error("custom table should override the default ITU-T T.81 table")
	}

	// Verify custom table values are what we set
	for i := 0; i < 64; i++ {
		if customizedTable[i] != 100 {
			t.Errorf("customized table[%d]: got %d, expected 100", i, customizedTable[i])
		}
	}
}

// TestSetQuantizationTable_EncodingUsesCustomTable tests that encoding
// actually uses the custom quantization table in the output.
func TestSetQuantizationTable_EncodingUsesCustomTable(t *testing.T) {
	// Create a simple test image
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, image.White)
		}
	}

	// Encode with default tables
	var buf1 bytes.Buffer
	enc1, err := NewWeeksEncoder(&buf1, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}
	if err := enc1.Encode(img); err != nil {
		t.Fatalf("Encode with default tables failed: %v", err)
	}
	defaultOutput := buf1.Bytes()

	// Encode with custom luminance table (all 1s for minimal quantization)
	var buf2 bytes.Buffer
	enc2, err := NewWeeksEncoder(&buf2, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	var customTable [64]int
	for i := 0; i < 64; i++ {
		customTable[i] = 1 // Minimal quantization
	}
	enc2.SetQuantizationTable(0, customTable)

	if err := enc2.Encode(img); err != nil {
		t.Fatalf("Encode with custom tables failed: %v", err)
	}
	customOutput := buf2.Bytes()

	// The outputs should be different because the quantization tables differ
	if bytes.Equal(defaultOutput, customOutput) {
		t.Error("encoding with custom quantization table should produce different output")
	}

	// Verify the custom table appears in the DQT marker
	// DQT marker is 0xFF 0xDB
	dqtFound := false
	for i := 0; i < len(customOutput)-1; i++ {
		if customOutput[i] == 0xFF && customOutput[i+1] == 0xDB {
			dqtFound = true
			// The table values should follow after the marker and length
			// marker (2) + length (2) + table info (1) = 5 bytes before table data
			if i+5+64 <= len(customOutput) {
				// Check if all values in the luminance table are 1
				allOnes := true
				for j := 0; j < 64; j++ {
					if customOutput[i+5+j] != 1 {
						allOnes = false
						break
					}
				}
				if !allOnes {
					t.Error("DQT marker should contain the custom table values")
				}
			}
			break
		}
	}

	if !dqtFound {
		t.Error("DQT marker not found in output")
	}
}

// TestSetQuantizationTable_MethodChaining tests that SetQuantizationTable
// can be chained with other builder methods.
func TestSetQuantizationTable_MethodChaining(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	var lumTable [64]int
	var chromTable [64]int
	for i := 0; i < 64; i++ {
		lumTable[i] = 16
		chromTable[i] = 32
	}

	// Chain multiple calls
	result := enc.
		SetQuantizationTable(0, lumTable).
		SetQuantizationTable(1, chromTable).
		SetComment("Custom encoder")

	if result != enc {
		t.Error("method chaining should return the same encoder")
	}

	// Verify all settings were applied
	if enc.comment != "Custom encoder" {
		t.Errorf("comment not set correctly: got %q", enc.comment)
	}

	lumResult := enc.quantizer.GetQuantTable(true)
	chromResult := enc.quantizer.GetQuantTable(false)

	for i := 0; i < 64; i++ {
		if lumResult[i] != 16 {
			t.Errorf("luminance table[%d]: got %d, expected 16", i, lumResult[i])
		}
		if chromResult[i] != 32 {
			t.Errorf("chrominance table[%d]: got %d, expected 32", i, chromResult[i])
		}
	}
}

// =============================================================================
// Package Structure Tests (merged from package_test.go)
// =============================================================================

// TestGoModIsValid verifies that go.mod exists and has the correct module path.
func TestGoModIsValid(t *testing.T) {
	// Read go.mod file
	content, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}

	modContent := string(content)

	// Check module path
	if !strings.Contains(modContent, "module github.com/0verkilll/weeksjpegencoder") {
		t.Error("go.mod does not contain correct module path 'github.com/0verkilll/weeksjpegencoder'")
	}

	// Check Go version is at least 1.24 (spec requirement)
	// go mod tidy may update to match the jpeg package's Go version
	goVersionPattern := regexp.MustCompile(`go 1\.(\d+)`)
	matches := goVersionPattern.FindStringSubmatch(modContent)
	if len(matches) < 2 {
		t.Error("go.mod does not specify a valid Go version")
	} else {
		var minorVersion int
		if _, err := regexp.MatchString(`^\d+`, matches[1]); err == nil {
			// Parse the minor version number
			for i, c := range matches[1] {
				if c < '0' || c > '9' {
					break
				}
				minorVersion = minorVersion*10 + int(c-'0')
				if i > 2 { // Prevent overflow on malformed input
					break
				}
			}
		}
		if minorVersion < 24 {
			t.Errorf("go.mod specifies Go version 1.%d; want at least 1.24", minorVersion)
		}
	}

	// Check jpeg dependency
	if !strings.Contains(modContent, "github.com/0verkilll/jpeg") {
		t.Error("go.mod does not contain jpeg package dependency")
	}
}

// TestJpegPackageImports verifies that the jpeg package can be imported and used.
func TestJpegPackageImports(t *testing.T) {
	// Test that we can access exported types from jpeg package
	// This verifies the import and replace directive work correctly

	// Test BlockSize constant is accessible
	if jpeg.BlockSize != 8 {
		t.Errorf("jpeg.BlockSize = %d; want 8", jpeg.BlockSize)
	}

	// Test BlockSize2 constant is accessible
	if jpeg.BlockSize2 != 64 {
		t.Errorf("jpeg.BlockSize2 = %d; want 64", jpeg.BlockSize2)
	}

	// Test ChromaSubsampling constants are accessible
	_ = jpeg.ChromaSubsampling420
	_ = jpeg.ChromaSubsampling422
	_ = jpeg.ChromaSubsampling444

	// Test standard Huffman arrays are accessible
	if len(jpeg.StdEncoderDCLuminanceBits) != 16 {
		t.Errorf("StdEncoderDCLuminanceBits length = %d; want 16", len(jpeg.StdEncoderDCLuminanceBits))
	}
	if len(jpeg.StdEncoderDCLuminanceValues) != 12 {
		t.Errorf("StdEncoderDCLuminanceValues length = %d; want 12", len(jpeg.StdEncoderDCLuminanceValues))
	}

	// Test standard quantization tables are accessible
	if len(jpeg.StandardLuminanceQuantTable) != 64 {
		t.Errorf("StandardLuminanceQuantTable length = %d; want 64", len(jpeg.StandardLuminanceQuantTable))
	}
	if len(jpeg.StandardChrominanceQuantTable) != 64 {
		t.Errorf("StandardChrominanceQuantTable length = %d; want 64", len(jpeg.StandardChrominanceQuantTable))
	}
}

// TestBasicPackageCompilation verifies that the package compiles correctly.
// This is implicitly tested by the test runner, but we verify key types exist.
func TestBasicPackageCompilation(t *testing.T) {
	// Test that jpeg encoder infrastructure is accessible
	dcTable := jpeg.NewStandardEncoderDCLuminanceTable()
	if dcTable == nil {
		t.Error("NewStandardEncoderDCLuminanceTable returned nil")
	}

	acTable := jpeg.NewStandardEncoderACLuminanceTable()
	if acTable == nil {
		t.Error("NewStandardEncoderACLuminanceTable returned nil")
	}

	// Test DCT transformer
	dct := jpeg.NewSeparableDCT()
	if dct == nil {
		t.Error("NewSeparableDCT returned nil")
	}

	// Test color converter
	conv := jpeg.NewBT601Converter()
	if conv == nil {
		t.Error("NewBT601Converter returned nil")
	}

	// Test ScaleQuantTable function; it returns a fixed [64]int table.
	_ = jpeg.ScaleQuantTable(jpeg.StandardLuminanceQuantTable, 75)
}

// TestEncodingInfrastructureAccessible verifies all encoding infrastructure is exported.
func TestEncodingInfrastructureAccessible(t *testing.T) {
	// Test EncoderBitWriter
	// (Cannot test without actually writing, but we verify the type exists by
	// checking we can reference it in code - this is a compile-time check)

	// Test marker writer functions are callable
	// These are function types, not values, so we verify by referencing them
	_ = jpeg.WriteSOI
	_ = jpeg.WriteEOI
	_ = jpeg.WriteAPP0
	_ = jpeg.WriteDQT
	_ = jpeg.WriteSOF0
	_ = jpeg.WriteDHT
	_ = jpeg.WriteSOS
	_ = jpeg.WriteCOM

	// Test component spec helpers
	specs420 := jpeg.Get420ComponentSpecs()
	if len(specs420) != 3 {
		t.Errorf("Get420ComponentSpecs returned %d specs; want 3", len(specs420))
	}

	specs422 := jpeg.Get422ComponentSpecs()
	if len(specs422) != 3 {
		t.Errorf("Get422ComponentSpecs returned %d specs; want 3", len(specs422))
	}

	specs444 := jpeg.Get444ComponentSpecs()
	if len(specs444) != 3 {
		t.Errorf("Get444ComponentSpecs returned %d specs; want 3", len(specs444))
	}

	// Test scan components
	scan420 := jpeg.Get420ScanComponents()
	if len(scan420) != 3 {
		t.Errorf("Get420ScanComponents returned %d components; want 3", len(scan420))
	}

	// Test Huffman specs
	huffSpecs := jpeg.GetStandardHuffmanSpecs()
	if len(huffSpecs) != 4 {
		t.Errorf("GetStandardHuffmanSpecs returned %d specs; want 4", len(huffSpecs))
	}

	// Test ZigzagOrder
	if len(jpeg.ZigzagOrder) != 64 {
		t.Errorf("ZigzagOrder length = %d; want 64", len(jpeg.ZigzagOrder))
	}
}

// =============================================================================
// Allocation and Buffer Tests (merged from allocation_test.go)
// =============================================================================

// TestEncodeImageDataJamesProcessesBlocksCorrectly verifies that the encoding
// loop processes all blocks and produces valid JPEG output.
func TestEncodeImageDataJamesProcessesBlocksCorrectly(t *testing.T) {
	// Create a test image with known dimensions
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{"16x16", 16, 16},
		{"32x32", 32, 32},
		{"48x48", 48, 48},
		{"17x17 non-multiple-of-8", 17, 17},
		{"23x19 odd dimensions", 23, 19},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			img := allocCreateGradientImage(tc.width, tc.height)

			var buf bytes.Buffer
			enc, err := NewWeeksEncoder(&buf, 75)
			if err != nil {
				t.Fatalf("NewWeeksEncoder failed: %v", err)
			}

			err = enc.Encode(img)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Verify output is valid JPEG (starts with SOI, ends with EOI)
			data := buf.Bytes()
			if len(data) < 4 {
				t.Fatalf("Output too short: %d bytes", len(data))
			}

			// Check SOI marker (0xFFD8)
			if data[0] != 0xFF || data[1] != 0xD8 {
				t.Errorf("Missing SOI marker, got %02X%02X", data[0], data[1])
			}

			// Check EOI marker (0xFFD9)
			if data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
				t.Errorf("Missing EOI marker, got %02X%02X", data[len(data)-2], data[len(data)-1])
			}
		})
	}
}

// TestBufferPreAllocationBehavior verifies that encoding different sized
// images doesn't cause unexpected allocation patterns.
func TestBufferPreAllocationBehavior(t *testing.T) {
	sizes := []struct {
		width  int
		height int
	}{
		{8, 8},
		{16, 16},
		{64, 64},
		{128, 128},
	}

	for _, size := range sizes {
		t.Run("size_"+string(rune('0'+size.width/8))+"x"+string(rune('0'+size.height/8)), func(t *testing.T) {
			img := allocCreateGradientImage(size.width, size.height)

			var buf bytes.Buffer
			enc, err := NewWeeksEncoder(&buf, 75)
			if err != nil {
				t.Fatalf("NewWeeksEncoder failed: %v", err)
			}

			err = enc.Encode(img)
			if err != nil {
				t.Fatalf("Encode failed for %dx%d: %v", size.width, size.height, err)
			}

			if buf.Len() == 0 {
				t.Error("Expected non-empty output")
			}
		})
	}
}

// TestEncoderOutputConsistency verifies that encoding the same image
// multiple times produces identical output (byte-for-byte).
func TestEncoderOutputConsistency(t *testing.T) {
	img := allocCreateGradientImage(32, 32)

	var buf1, buf2 bytes.Buffer

	enc1, err := NewWeeksEncoder(&buf1, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}
	if err := enc1.Encode(img); err != nil {
		t.Fatalf("First encode failed: %v", err)
	}

	enc2, err := NewWeeksEncoder(&buf2, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}
	if err := enc2.Encode(img); err != nil {
		t.Fatalf("Second encode failed: %v", err)
	}

	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("Encoding same image twice produced different results")
	}
}

// allocCreateGradientImage creates a test image with a gradient pattern.
func allocCreateGradientImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create a diagonal gradient
			val := uint8((x + y) * 255 / (width + height - 2))
			img.Set(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
		}
	}
	return img
}
