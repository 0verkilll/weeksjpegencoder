// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements integration tests for the F5/James encoder support spec.
// These tests verify end-to-end workflows and fill critical coverage gaps.
// Migrated from jpeg/f5_integration_test.go with updates for standalone package.

package weeksjpegencoder

import (
	"bytes"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"sync"
	"testing"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// Strategic Integration Tests
// =============================================================================

// TestIntegration_F5EncodeSignatureExtractValidateCOM tests the end-to-end workflow:
// F5/James encode -> signature extract -> validate COM marker contains F5/James signature
func TestIntegration_F5EncodeSignatureExtractValidateCOM(t *testing.T) {
	// Create test image
	img := itgCreateTestImage(64, 64)

	// Step 1: Encode with F5 encoder (uses default F5/James COM marker)
	encodedData, err := WeeksEncodeToBytes(img, 75)
	if err != nil {
		t.Fatalf("WeeksEncodeToBytes failed: %v", err)
	}

	// Step 2: Extract JPEG signature from encoded data
	sig, err := jpeg.ExtractSignature(encodedData)
	if err != nil {
		t.Fatalf("ExtractSignature failed: %v", err)
	}

	// Step 3: Validate COM marker contains F5/James signature
	expectedSignature := "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech."

	foundF5Signature := false
	for _, comment := range sig.Comments {
		if comment == expectedSignature {
			foundF5Signature = true
			break
		}
	}

	if !foundF5Signature {
		t.Errorf("F5/James COM marker signature not found in extracted signature")
		t.Logf("Found comments: %v", sig.Comments)
	}

	// Also verify encoder hints can identify F5/James-style encoding
	if sig.EncoderHints != nil {
		t.Logf("Encoder hints: JFIF=%s", sig.EncoderHints.JFIFVersion)
	}
}

// TestIntegration_EncodeQualityDetectVerifyMatch tests:
// Encode at quality Q -> detect quality -> verify detected matches actual
func TestIntegration_EncodeQualityDetectVerifyMatch(t *testing.T) {
	img := itgCreateTestImage(128, 128)
	estimator := jpeg.NewQualityEstimator(nil)

	// Test a range of quality levels
	testQualities := []int{10, 25, 50, 75, 90, 95}

	for _, expectedQ := range testQualities {
		t.Run(itgQualityName(expectedQ), func(t *testing.T) {
			// Encode at specific quality
			encodedData, err := WeeksEncodeToBytes(img, expectedQ)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytes(Q%d) failed: %v", expectedQ, err)
			}

			// Detect quality from encoded data
			estimate, err := estimator.EstimateQuality(encodedData)
			if err != nil {
				t.Fatalf("EstimateQuality failed: %v", err)
			}

			// Verify match within +/-2 tolerance
			deviation := itgAbs(estimate.Quality - expectedQ)
			if deviation > 2 {
				t.Errorf("Quality mismatch: encoded at Q%d, detected as Q%d (deviation=%d)",
					expectedQ, estimate.Quality, deviation)
			} else {
				t.Logf("Q%d -> detected Q%d (deviation=%d, confidence=%.2f)",
					expectedQ, estimate.Quality, deviation, estimate.Confidence)
			}
		})
	}
}

// TestIntegration_SubsamplingModesPreserveQuality tests:
// Multiple subsampling modes preserve expected quality detection accuracy
// Uses standard mode to produce Go-decodable output.
func TestIntegration_SubsamplingModesPreserveQuality(t *testing.T) {
	img := itgCreateTestImage(128, 128)
	estimator := jpeg.NewQualityEstimator(nil)
	quality := 75

	subsamplingModes := []struct {
		mode jpeg.ChromaSubsamplingMode
		name string
	}{
		{jpeg.ChromaSubsampling420, "4:2:0"},
		{jpeg.ChromaSubsampling422, "4:2:2"},
		{jpeg.ChromaSubsampling444, "4:4:4"},
	}

	for _, ss := range subsamplingModes {
		t.Run(ss.name, func(t *testing.T) {
			// Encode with specific subsampling in standard mode (Go-decodable)
			var buf bytes.Buffer
			enc, err := NewWeeksEncoderWithOptions(&buf, quality, WithStandardMode())
			if err != nil {
				t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
			}
			enc.SetSubsampling(ss.mode)

			err = enc.Encode(img)
			if err != nil {
				t.Fatalf("Encode with %s failed: %v", ss.name, err)
			}

			// Detect quality
			estimate, err := estimator.EstimateQuality(buf.Bytes())
			if err != nil {
				t.Fatalf("EstimateQuality failed: %v", err)
			}

			// Verify quality detection accuracy
			deviation := itgAbs(estimate.Quality - quality)
			if deviation > 2 {
				t.Errorf("Quality detection failed for %s: expected Q%d +/-2, got Q%d",
					ss.name, quality, estimate.Quality)
			}

			// Verify image is decodable
			_, err = stdjpeg.Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Errorf("Encoded %s output not decodable: %v", ss.name, err)
			}
		})
	}
}

// TestIntegration_VerySmallImage8x8 tests encoding/decoding of the smallest possible MCU
// Uses standard mode to produce Go-decodable output.
func TestIntegration_VerySmallImage8x8(t *testing.T) {
	// 8x8 is a single MCU - the smallest meaningful JPEG block
	img := itgCreateTestImage(8, 8)

	qualities := []int{25, 50, 75, 100}

	for _, q := range qualities {
		t.Run(itgQualityName(q), func(t *testing.T) {
			// Encode in standard mode (Go-decodable)
			encodedData, err := WeeksEncodeToBytesStandard(img, q)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytesStandard(8x8, Q%d) failed: %v", q, err)
			}

			// Decode
			decoded, err := stdjpeg.Decode(bytes.NewReader(encodedData))
			if err != nil {
				t.Fatalf("Decode 8x8 Q%d failed: %v", q, err)
			}

			// Verify dimensions
			if decoded.Bounds().Dx() != 8 || decoded.Bounds().Dy() != 8 {
				t.Errorf("Dimension mismatch: expected 8x8, got %dx%d",
					decoded.Bounds().Dx(), decoded.Bounds().Dy())
			}

			// Verify valid JPEG structure
			if encodedData[0] != 0xFF || encodedData[1] != 0xD8 {
				t.Error("Missing SOI marker")
			}
			if encodedData[len(encodedData)-2] != 0xFF || encodedData[len(encodedData)-1] != 0xD9 {
				t.Error("Missing EOI marker")
			}
		})
	}
}

// TestIntegration_LargeImageWithinMemoryLimits tests encoding/decoding of large images
// Uses standard mode to produce Go-decodable output.
func TestIntegration_LargeImageWithinMemoryLimits(t *testing.T) {
	// Test with a reasonably large image (512x512)
	// This tests MCU processing across many blocks
	img := itgCreateTestImage(512, 512)

	// Encode at moderate quality in standard mode (Go-decodable)
	encodedData, err := WeeksEncodeToBytesStandard(img, 75)
	if err != nil {
		t.Fatalf("WeeksEncodeToBytesStandard(512x512) failed: %v", err)
	}

	// Decode
	decoded, err := stdjpeg.Decode(bytes.NewReader(encodedData))
	if err != nil {
		t.Fatalf("Decode 512x512 failed: %v", err)
	}

	// Verify dimensions
	if decoded.Bounds().Dx() != 512 || decoded.Bounds().Dy() != 512 {
		t.Errorf("Dimension mismatch: expected 512x512, got %dx%d",
			decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}

	// Verify file size is reasonable (not too large, not corrupt)
	if len(encodedData) < 1000 {
		t.Errorf("Encoded file suspiciously small: %d bytes", len(encodedData))
	}
	if len(encodedData) > 512*512*3 { // Should be much smaller than raw due to compression
		t.Errorf("Encoded file larger than uncompressed: %d bytes", len(encodedData))
	}

	t.Logf("512x512 image encoded to %d bytes", len(encodedData))
}

// TestIntegration_InvalidQualityValues tests error handling for invalid quality values
func TestIntegration_InvalidQualityValues(t *testing.T) {
	img := itgCreateTestImage(32, 32)

	invalidQualities := []struct {
		quality int
		name    string
	}{
		{0, "zero"},
		{-1, "negative_one"},
		{-100, "negative_hundred"},
		{101, "one_hundred_one"},
		{1000, "one_thousand"},
	}

	for _, tc := range invalidQualities {
		t.Run(tc.name, func(t *testing.T) {
			_, err := WeeksEncodeToBytes(img, tc.quality)
			if err == nil {
				t.Errorf("WeeksEncodeToBytes(quality=%d) should return error, got nil", tc.quality)
			}
		})
	}
}

// TestIntegration_NilImageInput tests error handling for nil image input
func TestIntegration_NilImageInput(t *testing.T) {
	// Test WeeksEncodeToBytes with nil
	_, err := WeeksEncodeToBytes(nil, 75)
	if err == nil {
		t.Error("WeeksEncodeToBytes(nil) should return error")
	}

	// Test encoder with nil
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	err = enc.Encode(nil)
	if err == nil {
		t.Error("Encoder.Encode(nil) should return error")
	}
}

// TestIntegration_ConcurrentEncodes tests thread safety with multiple concurrent encodes
func TestIntegration_ConcurrentEncodes(t *testing.T) {
	// Create shared test image
	img := itgCreateTestImage(64, 64)

	// Number of concurrent goroutines
	numGoroutines := 10
	numIterations := 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numIterations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				// Each goroutine encodes at different quality
				quality := 25 + (goroutineID*7)%75

				encoded, err := WeeksEncodeToBytesStandard(img, quality)
				if err != nil {
					errors <- err
					return
				}

				// Verify valid JPEG
				if len(encoded) < 100 {
					errors <- err
					return
				}

				// Verify decodable
				_, err = stdjpeg.Decode(bytes.NewReader(encoded))
				if err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent encode error: %v", err)
		errorCount++
	}

	if errorCount == 0 {
		t.Logf("Successfully completed %d concurrent encodes", numGoroutines*numIterations)
	}
}

// =============================================================================
// Additional Edge Case Tests
// =============================================================================

// TestIntegration_SignatureExtractionFromEncodedImage tests complete signature extraction
// from an F5-encoded image, verifying all signature components are present
func TestIntegration_SignatureExtractionFromEncodedImage(t *testing.T) {
	img := itgCreateTestImage(64, 64)

	// Encode
	encodedData, err := WeeksEncodeToBytes(img, 75)
	if err != nil {
		t.Fatalf("WeeksEncodeToBytes failed: %v", err)
	}

	// Extract full signature
	sig, err := jpeg.ExtractSignature(encodedData)
	if err != nil {
		t.Fatalf("ExtractSignature failed: %v", err)
	}

	// Verify all signature components are populated
	if sig.SOFInfo == nil {
		t.Error("SOFInfo should not be nil")
	} else {
		if sig.SOFInfo.Width != 64 || sig.SOFInfo.Height != 64 {
			t.Errorf("SOFInfo dimensions: got %dx%d, want 64x64",
				sig.SOFInfo.Width, sig.SOFInfo.Height)
		}
		if sig.SOFInfo.Precision != 8 {
			t.Errorf("SOFInfo precision: got %d, want 8", sig.SOFInfo.Precision)
		}
		if len(sig.SOFInfo.Components) != 3 {
			t.Errorf("SOFInfo components: got %d, want 3", len(sig.SOFInfo.Components))
		}
	}

	// Verify quantization tables exist
	if len(sig.QuantTables) < 2 {
		t.Errorf("Expected at least 2 quantization tables, got %d", len(sig.QuantTables))
	}

	// Verify Huffman tables exist
	if len(sig.HuffmanTables) < 4 {
		t.Errorf("Expected at least 4 Huffman tables, got %d", len(sig.HuffmanTables))
	}

	// Verify APP markers exist (should have APP0 for JFIF)
	if _, ok := sig.APPMarkers[0]; !ok {
		t.Error("APP0 marker should exist")
	}

	// Verify comments exist
	if len(sig.Comments) == 0 {
		t.Error("Expected at least one comment (F5/James signature)")
	}
}

// TestIntegration_EncoderDecoderRoundTrip tests that encoded images can be
// decoded by the jpeg package's Decode function (using standard library for decoding)
func TestIntegration_EncoderDecoderRoundTrip(t *testing.T) {
	testCases := []struct {
		name    string
		width   int
		height  int
		quality int
	}{
		{"small_low_quality", 32, 32, 25},
		{"small_high_quality", 32, 32, 95},
		{"medium_standard", 128, 128, 75},
		{"odd_dimensions", 33, 47, 75},
		{"large_moderate", 256, 256, 75},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test image
			img := itgCreateTestImage(tc.width, tc.height)

			// Encode with F5 encoder in standard mode for Go-decodable output
			encodedData, err := WeeksEncodeToBytesStandard(img, tc.quality)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytesStandard failed: %v", err)
			}

			// Decode with standard library (verifies interoperability)
			decoded, err := stdjpeg.Decode(bytes.NewReader(encodedData))
			if err != nil {
				t.Fatalf("Standard decoder failed: %v", err)
			}

			// Verify dimensions preserved
			if decoded.Bounds().Dx() != tc.width || decoded.Bounds().Dy() != tc.height {
				t.Errorf("Dimension mismatch: got %dx%d, want %dx%d",
					decoded.Bounds().Dx(), decoded.Bounds().Dy(), tc.width, tc.height)
			}

			// Also verify using jpeg package's signature extraction
			sig, err := jpeg.ExtractSignature(encodedData)
			if err != nil {
				t.Fatalf("ExtractSignature failed: %v", err)
			}

			if sig.SOFInfo.Width != tc.width || sig.SOFInfo.Height != tc.height {
				t.Errorf("SOF dimensions mismatch: got %dx%d, want %dx%d",
					sig.SOFInfo.Width, sig.SOFInfo.Height, tc.width, tc.height)
			}
		})
	}
}

// TestIntegration_F5JamesDetection tests that the jpeg package correctly
// identifies F5/James encoder from encoded images
func TestIntegration_F5JamesDetection(t *testing.T) {
	img := itgCreateTestImage(64, 64)

	// Encode with default F5/James signature
	encodedData, err := WeeksEncodeToBytes(img, 75)
	if err != nil {
		t.Fatalf("WeeksEncodeToBytes failed: %v", err)
	}

	// Use quality estimator to detect encoder
	estimator := jpeg.NewQualityEstimator(nil)
	estimate, err := estimator.EstimateQuality(encodedData)
	if err != nil {
		t.Fatalf("EstimateQuality failed: %v", err)
	}

	// F5/James uses standard libjpeg quantization tables, so it may be detected
	// as libjpeg or F5/James depending on COM marker detection
	t.Logf("Detected encoder: %s", estimate.DetectedEncoder.String())
	t.Logf("Quality estimate: %d (confidence: %.2f)", estimate.Quality, estimate.Confidence)

	// Extract signature and verify COM marker
	sig, err := jpeg.ExtractSignature(encodedData)
	if err != nil {
		t.Fatalf("ExtractSignature failed: %v", err)
	}

	// Verify F5/James signature is in comments
	f5SignatureFound := false
	for _, comment := range sig.Comments {
		if bytes.Contains([]byte(comment), []byte("James R. Weeks")) ||
			bytes.Contains([]byte(comment), []byte("BioElectroMech")) {
			f5SignatureFound = true
			break
		}
	}

	if !f5SignatureFound {
		t.Error("F5/James signature not found in COM markers")
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// itgCreateTestImage creates a test image with varied content
func itgCreateTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create gradient with some variation
			r := uint8((x * 255) / itgMax(width-1, 1))
			g := uint8((y * 255) / itgMax(height-1, 1))
			b := uint8(((x + y) * 127) / itgMax(width+height-2, 1))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}

// itgAbs returns absolute value of integer
func itgAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// itgMax returns larger of two integers
func itgMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// itgQualityName returns a test name for a quality level
func itgQualityName(q int) string {
	return "Q" + string(rune('0'+q/100)) + string(rune('0'+(q/10)%10)) + string(rune('0'+q%10))
}
