// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file contains cross-package integration tests that verify the weeksjpegencoder
// package works correctly with the jpeg package after the package extraction.
//
// These tests are part of Task Group 6: Cross-Package Integration Testing
// for the F5 Encoder Package Extraction spec.

package weeksjpegencoder

import (
	"bytes"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"math"
	"testing"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// Task 6.2: Cross-Package Integration Tests
// =============================================================================

// TestCrossPackage_WeeksEncoderProducesDecodableJPEG verifies that weeksjpegencoder produces
// JPEG data that the jpeg package can analyze and process correctly.
// Uses standard mode to produce Go-decodable output.
func TestCrossPackage_WeeksEncoderProducesDecodableJPEG(t *testing.T) {
	testCases := []struct {
		name    string
		width   int
		height  int
		quality int
	}{
		{"small_64x64_q75", 64, 64, 75},
		{"medium_128x128_q50", 128, 128, 50},
		{"large_256x256_q90", 256, 256, 90},
		{"non_multiple_100x100_q75", 100, 100, 75},
		{"minimal_8x8_q50", 8, 8, 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test image
			img := cpiCreateTestImage(tc.width, tc.height)

			// Encode with weeksjpegencoder in standard mode (Go-decodable)
			encoded, err := WeeksEncodeToBytesStandard(img, tc.quality)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytesStandard failed: %v", err)
			}

			// Verify jpeg package can detect format
			format, err := jpeg.DetectFormat(encoded)
			if err != nil {
				t.Errorf("jpeg.DetectFormat failed: %v", err)
			}
			if format != jpeg.FormatBaselineJPEG {
				t.Errorf("Expected FormatBaselineJPEG, got %v", format)
			}

			// Verify jpeg package can extract signature
			sig, err := jpeg.ExtractSignature(encoded)
			if err != nil {
				t.Fatalf("jpeg.ExtractSignature failed: %v", err)
			}

			// Verify dimensions in signature match
			if sig.SOFInfo.Width != tc.width || sig.SOFInfo.Height != tc.height {
				t.Errorf("Signature dimensions mismatch: got %dx%d, want %dx%d",
					sig.SOFInfo.Width, sig.SOFInfo.Height, tc.width, tc.height)
			}

			// Verify standard library can decode
			decoded, err := stdjpeg.Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("Standard library decode failed: %v", err)
			}

			// Verify decoded dimensions match original
			bounds := decoded.Bounds()
			if bounds.Dx() != tc.width || bounds.Dy() != tc.height {
				t.Errorf("Decoded dimensions mismatch: got %dx%d, want %dx%d",
					bounds.Dx(), bounds.Dy(), tc.width, tc.height)
			}
		})
	}
}

// TestCrossPackage_DecodedImageMatchesOriginal verifies that decoded image
// matches the original within JPEG compression tolerance (PSNR >= 30dB for Q75+).
// Uses standard mode to produce Go-decodable output.
func TestCrossPackage_DecodedImageMatchesOriginal(t *testing.T) {
	testCases := []struct {
		name       string
		quality    int
		minPSNR    float64
		strictness string
	}{
		{"high_quality_95", 95, 35.0, "high"},
		{"standard_quality_75", 75, 30.0, "standard"},
		{"moderate_quality_50", 50, 25.0, "moderate"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test image with known content
			original := cpiCreateGradientImage(128, 128)

			// Encode with weeksjpegencoder in standard mode (Go-decodable)
			encoded, err := WeeksEncodeToBytesStandard(original, tc.quality)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytesStandard failed: %v", err)
			}

			// Decode with standard library
			decoded, err := stdjpeg.Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Calculate PSNR
			psnr := cpiCalculatePSNR(original, decoded)
			if psnr < tc.minPSNR {
				t.Errorf("PSNR too low: got %.2f dB, want >= %.2f dB for %s quality",
					psnr, tc.minPSNR, tc.strictness)
			}

			t.Logf("Q%d PSNR: %.2f dB (min required: %.2f dB)", tc.quality, psnr, tc.minPSNR)
		})
	}
}

// TestCrossPackage_F5JamesSignatureDetection verifies that jpeg.AnalyzeSignature
// and quality estimation correctly identify F5/James encoder output.
func TestCrossPackage_F5JamesSignatureDetection(t *testing.T) {
	// Create and encode test image
	img := cpiCreateTestImage(64, 64)
	encoded, err := WeeksEncodeToBytes(img, 75)
	if err != nil {
		t.Fatalf("WeeksEncodeToBytes failed: %v", err)
	}

	// Extract JPEG signature
	sig, err := jpeg.ExtractSignature(encoded)
	if err != nil {
		t.Fatalf("ExtractSignature failed: %v", err)
	}

	// Verify F5/James COM marker signature is present
	expectedSignature := "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech."
	foundSignature := false
	for _, comment := range sig.Comments {
		if comment == expectedSignature {
			foundSignature = true
			break
		}
	}
	if !foundSignature {
		t.Error("F5/James COM marker signature not found in extracted signature")
		t.Logf("Found comments: %v", sig.Comments)
	}

	// Verify encoder hints identify F5/James-style encoding
	if sig.EncoderHints != nil && len(sig.EncoderHints.CommentStrings) > 0 {
		foundInHints := false
		for _, comment := range sig.EncoderHints.CommentStrings {
			if comment == expectedSignature {
				foundInHints = true
				break
			}
		}
		if foundInHints {
			t.Log("F5/James signature found in EncoderHints.CommentStrings")
		}
	}

	// Verify quality estimation works
	estimator := jpeg.NewQualityEstimator(nil)
	estimate, err := estimator.EstimateQuality(encoded)
	if err != nil {
		t.Fatalf("EstimateQuality failed: %v", err)
	}

	// Quality should be detected within +/- 2 of actual
	if cpiAbs(estimate.Quality-75) > 2 {
		t.Errorf("Quality detection inaccurate: expected 75 +/-2, got %d", estimate.Quality)
	}
	t.Logf("Quality estimate: %d (actual: 75), confidence: %.2f", estimate.Quality, estimate.Confidence)
}

// TestCrossPackage_EncoderQualityMatchesDetection verifies that encoder quality
// matches what jpeg quality estimator detects across the full range.
func TestCrossPackage_EncoderQualityMatchesDetection(t *testing.T) {
	img := cpiCreateTestImage(128, 128)
	estimator := jpeg.NewQualityEstimator(nil)

	// Test key quality levels
	testQualities := []int{10, 25, 50, 75, 90, 100}

	for _, expectedQ := range testQualities {
		t.Run(cpiQualityName(expectedQ), func(t *testing.T) {
			// Encode at specific quality
			encoded, err := WeeksEncodeToBytes(img, expectedQ)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytes(Q%d) failed: %v", expectedQ, err)
			}

			// Detect quality
			estimate, err := estimator.EstimateQuality(encoded)
			if err != nil {
				t.Fatalf("EstimateQuality failed: %v", err)
			}

			// Verify within +/-2 tolerance
			deviation := cpiAbs(estimate.Quality - expectedQ)
			if deviation > 2 {
				t.Errorf("Quality mismatch: encoded Q%d, detected Q%d (deviation %d, expected <= 2)",
					expectedQ, estimate.Quality, deviation)
			} else {
				t.Logf("Q%d -> detected Q%d (deviation %d, confidence %.2f)",
					expectedQ, estimate.Quality, deviation, estimate.Confidence)
			}
		})
	}
}

// TestCrossPackage_MultipleSubsamplingModesValid tests that multiple subsampling
// modes (420, 422, 444) produce valid JPEGs recognizable by jpeg package.
// Uses standard mode to produce Go-decodable output.
func TestCrossPackage_MultipleSubsamplingModesValid(t *testing.T) {
	img := cpiCreateTestImage(64, 64)

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
			enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithStandardMode())
			if err != nil {
				t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
			}
			enc.SetSubsampling(ss.mode)

			err = enc.Encode(img)
			if err != nil {
				t.Fatalf("Encode with %s failed: %v", ss.name, err)
			}

			encoded := buf.Bytes()

			// Verify jpeg package can detect format
			format, err := jpeg.DetectFormat(encoded)
			if err != nil {
				t.Errorf("jpeg.DetectFormat failed for %s: %v", ss.name, err)
			}
			if format != jpeg.FormatBaselineJPEG {
				t.Errorf("Expected FormatBaselineJPEG for %s, got %v", ss.name, format)
			}

			// Verify jpeg package can extract signature
			sig, err := jpeg.ExtractSignature(encoded)
			if err != nil {
				t.Fatalf("ExtractSignature failed for %s: %v", ss.name, err)
			}

			// Verify component count (should be 3 for YCbCr)
			if len(sig.SOFInfo.Components) != 3 {
				t.Errorf("Expected 3 components for %s, got %d", ss.name, len(sig.SOFInfo.Components))
			}

			// Verify standard library can decode
			decoded, err := stdjpeg.Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("Standard decode failed for %s: %v", ss.name, err)
			}

			// Verify dimensions
			if decoded.Bounds().Dx() != 64 || decoded.Bounds().Dy() != 64 {
				t.Errorf("Dimension mismatch for %s: got %dx%d, want 64x64",
					ss.name, decoded.Bounds().Dx(), decoded.Bounds().Dy())
			}

			t.Logf("%s encoded to %d bytes", ss.name, len(encoded))
		})
	}
}

// TestCrossPackage_VariousQualityLevelsQuantization verifies that various quality
// levels (1, 50, 75, 100) produce correct quantization tables detectable by jpeg package.
// Uses standard mode to produce Go-decodable output.
func TestCrossPackage_VariousQualityLevelsQuantization(t *testing.T) {
	img := cpiCreateTestImage(64, 64)

	qualityLevels := []int{1, 50, 75, 100}
	var prevSize int

	for _, q := range qualityLevels {
		t.Run(cpiQualityName(q), func(t *testing.T) {
			// Encode at specific quality in standard mode (Go-decodable)
			encoded, err := WeeksEncodeToBytesStandard(img, q)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytesStandard(Q%d) failed: %v", q, err)
			}

			// Extract signature
			sig, err := jpeg.ExtractSignature(encoded)
			if err != nil {
				t.Fatalf("ExtractSignature failed: %v", err)
			}

			// Verify quantization tables exist
			if len(sig.QuantTables) < 2 {
				t.Errorf("Expected at least 2 quantization tables for Q%d, got %d", q, len(sig.QuantTables))
			}

			// Verify file size correlates with quality (higher quality = larger file)
			currentSize := len(encoded)
			if prevSize > 0 && currentSize <= prevSize {
				t.Logf("Warning: Q%d file size (%d) is not larger than lower quality (%d)",
					q, currentSize, prevSize)
			}

			// Verify standard library can decode
			_, err = stdjpeg.Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("Standard decode failed for Q%d: %v", q, err)
			}

			t.Logf("Q%d: %d bytes, %d quant tables", q, currentSize, len(sig.QuantTables))
			prevSize = currentSize
		})
	}
}

// TestCrossPackage_EdgeCasesSmallImages tests edge cases with small images
// (1x1, 8x8) to verify cross-package compatibility.
// Uses standard mode to produce Go-decodable output.
func TestCrossPackage_EdgeCasesSmallImages(t *testing.T) {
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{"1x1_single_pixel", 1, 1},
		{"8x8_single_mcu", 8, 8},
		{"7x7_sub_mcu", 7, 7},
		{"9x9_mcu_plus_one", 9, 9},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create small test image
			img := cpiCreateTestImage(tc.width, tc.height)

			// Encode with weeksjpegencoder in standard mode (Go-decodable)
			encoded, err := WeeksEncodeToBytesStandard(img, 75)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytesStandard(%s) failed: %v", tc.name, err)
			}

			// Verify jpeg package can detect and analyze
			format, err := jpeg.DetectFormat(encoded)
			if err != nil {
				t.Errorf("DetectFormat failed for %s: %v", tc.name, err)
			}
			if format != jpeg.FormatBaselineJPEG {
				t.Errorf("Expected FormatBaselineJPEG for %s, got %v", tc.name, format)
			}

			// Extract signature
			sig, err := jpeg.ExtractSignature(encoded)
			if err != nil {
				t.Fatalf("ExtractSignature failed for %s: %v", tc.name, err)
			}

			// Verify dimensions
			if sig.SOFInfo.Width != tc.width || sig.SOFInfo.Height != tc.height {
				t.Errorf("Signature dimensions mismatch for %s: got %dx%d, want %dx%d",
					tc.name, sig.SOFInfo.Width, sig.SOFInfo.Height, tc.width, tc.height)
			}

			// Verify decodable
			decoded, err := stdjpeg.Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("Decode failed for %s: %v", tc.name, err)
			}

			// Verify decoded dimensions
			if decoded.Bounds().Dx() != tc.width || decoded.Bounds().Dy() != tc.height {
				t.Errorf("Decoded dimensions mismatch for %s: got %dx%d, want %dx%d",
					tc.name, decoded.Bounds().Dx(), decoded.Bounds().Dy(), tc.width, tc.height)
			}

			t.Logf("%s: encoded to %d bytes", tc.name, len(encoded))
		})
	}
}

// TestCrossPackage_EdgeCasesLargeImages tests large images (up to 4096x4096)
// to verify cross-package compatibility at scale.
// Uses standard mode to produce Go-decodable output.
func TestCrossPackage_EdgeCasesLargeImages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large image tests in short mode")
	}

	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{"512x512_standard", 512, 512},
		{"1024x768_landscape", 1024, 768},
		{"2048x2048_large", 2048, 2048},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create large test image
			img := cpiCreateTestImage(tc.width, tc.height)

			// Encode with weeksjpegencoder in standard mode (Go-decodable)
			encoded, err := WeeksEncodeToBytesStandard(img, 75)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytesStandard(%s) failed: %v", tc.name, err)
			}

			// Verify jpeg package can detect and analyze
			format, err := jpeg.DetectFormat(encoded)
			if err != nil {
				t.Errorf("DetectFormat failed for %s: %v", tc.name, err)
			}
			if format != jpeg.FormatBaselineJPEG {
				t.Errorf("Expected FormatBaselineJPEG for %s, got %v", tc.name, format)
			}

			// Extract signature (should work even for large images)
			sig, err := jpeg.ExtractSignature(encoded)
			if err != nil {
				t.Fatalf("ExtractSignature failed for %s: %v", tc.name, err)
			}

			// Verify dimensions
			if sig.SOFInfo.Width != tc.width || sig.SOFInfo.Height != tc.height {
				t.Errorf("Signature dimensions mismatch for %s: got %dx%d, want %dx%d",
					tc.name, sig.SOFInfo.Width, sig.SOFInfo.Height, tc.width, tc.height)
			}

			// Verify decodable
			decoded, err := stdjpeg.Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("Decode failed for %s: %v", tc.name, err)
			}

			// Verify decoded dimensions
			if decoded.Bounds().Dx() != tc.width || decoded.Bounds().Dy() != tc.height {
				t.Errorf("Decoded dimensions mismatch for %s: got %dx%d, want %dx%d",
					tc.name, decoded.Bounds().Dx(), decoded.Bounds().Dy(), tc.width, tc.height)
			}

			// Calculate compression ratio
			uncompressedSize := tc.width * tc.height * 3
			compressionRatio := float64(uncompressedSize) / float64(len(encoded))
			t.Logf("%s: %d bytes (compression ratio: %.1f:1)", tc.name, len(encoded), compressionRatio)
		})
	}
}

// TestCrossPackage_WeeksEncoderUsesExportedAPIsCorrectly verifies that weeksjpegencoder
// correctly uses all exported jpeg package APIs without panics or errors.
func TestCrossPackage_WeeksEncoderUsesExportedAPIsCorrectly(t *testing.T) {
	// Test that encoder doesn't panic with various valid inputs
	testCases := []struct {
		name    string
		width   int
		height  int
		quality int
		ss      jpeg.ChromaSubsamplingMode
	}{
		{"min_quality", 32, 32, 1, jpeg.ChromaSubsampling420},
		{"max_quality", 32, 32, 100, jpeg.ChromaSubsampling420},
		{"444_mode", 32, 32, 75, jpeg.ChromaSubsampling444},
		{"422_mode", 32, 32, 75, jpeg.ChromaSubsampling422},
		{"420_mode", 32, 32, 75, jpeg.ChromaSubsampling420},
		{"odd_dimensions", 33, 47, 75, jpeg.ChromaSubsampling420},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test image
			img := cpiCreateTestImage(tc.width, tc.height)

			// f5.jar only supports 4:2:0; exercise 4:2:2/4:4:4 through the
			// standard-mode path which honours arbitrary sampling factors.
			var buf bytes.Buffer
			opts := []Option{}
			if tc.ss != jpeg.ChromaSubsampling420 {
				opts = append(opts, WithStandardMode())
			}
			enc, err := NewWeeksEncoderWithOptions(&buf, tc.quality, opts...)
			if err != nil {
				t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
			}
			enc.SetSubsampling(tc.ss)

			// Encode (should not panic)
			err = enc.Encode(img)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Verify output is valid
			encoded := buf.Bytes()
			if len(encoded) < 100 {
				t.Errorf("Encoded output suspiciously small: %d bytes", len(encoded))
			}

			// Verify SOI marker
			if encoded[0] != 0xFF || encoded[1] != 0xD8 {
				t.Error("Missing SOI marker")
			}

			// Verify EOI marker
			if encoded[len(encoded)-2] != 0xFF || encoded[len(encoded)-1] != 0xD9 {
				t.Error("Missing EOI marker")
			}

			// Verify jpeg package can process the output
			_, err = jpeg.ExtractSignature(encoded)
			if err != nil {
				t.Errorf("jpeg.ExtractSignature failed: %v", err)
			}
		})
	}

	// Test error handling for invalid inputs
	t.Run("invalid_quality_0", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := NewWeeksEncoder(&buf, 0)
		if err == nil {
			t.Error("Expected error for quality 0")
		}
	})

	t.Run("invalid_quality_101", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := NewWeeksEncoder(&buf, 101)
		if err == nil {
			t.Error("Expected error for quality 101")
		}
	})

	t.Run("nil_image", func(t *testing.T) {
		var buf bytes.Buffer
		enc, err := NewWeeksEncoder(&buf, 75)
		if err != nil {
			t.Fatalf("NewWeeksEncoder failed: %v", err)
		}
		err = enc.Encode(nil)
		if err == nil {
			t.Error("Expected error for nil image")
		}
	})
}

// =============================================================================
// Helper Functions (prefixed with cpi = cross package integration)
// =============================================================================

// cpiCreateTestImage creates a test image with varied content for integration testing.
func cpiCreateTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create gradient with some variation
			r := uint8((x * 255) / cpiMax(width-1, 1))
			g := uint8((y * 255) / cpiMax(height-1, 1))
			b := uint8(((x + y) * 127) / cpiMax(width+height-2, 1))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}

// cpiCreateGradientImage creates a smooth gradient image for PSNR testing.
func cpiCreateGradientImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create diagonal gradient
			r := uint8((x + y) * 255 / cpiMax(width+height-2, 1))
			g := uint8(math.Abs(float64(x-width/2)) * 255 / float64(cpiMax(width/2, 1)))
			b := uint8(math.Abs(float64(y-height/2)) * 255 / float64(cpiMax(height/2, 1)))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}

// cpiCalculatePSNR calculates Peak Signal-to-Noise Ratio between two images.
func cpiCalculatePSNR(img1, img2 image.Image) float64 {
	bounds := img1.Bounds()
	bounds2 := img2.Bounds()
	if bounds.Dx() != bounds2.Dx() || bounds.Dy() != bounds2.Dy() {
		return 0
	}

	var mse float64
	count := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, _ := img1.At(x, y).RGBA()
			r2, g2, b2, _ := img2.At(x, y).RGBA()

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

// cpiAbs returns absolute value of integer.
func cpiAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// cpiMax returns the larger of two integers.
func cpiMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// cpiQualityName returns a test name for a quality level.
func cpiQualityName(q int) string {
	return "Q" + string(rune('0'+q/100)) + string(rune('0'+(q/10)%10)) + string(rune('0'+q%10))
}
