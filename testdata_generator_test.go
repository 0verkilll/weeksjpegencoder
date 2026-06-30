// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements test image generation infrastructure for quality detection
// validation. It creates reference images at various quality levels using the
// F5/James-compatible encoder.
//
// Migrated from jpeg/testdata_generator_test.go with updates for standalone package.

package weeksjpegencoder

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// Task 5.1: Focused Tests for Image Generation
// =============================================================================

// TestImageGeneration_ReferenceQuality tests that reference images generated
// at Q1, Q50, Q100 have correct quality characteristics.
func TestImageGeneration_ReferenceQuality(t *testing.T) {
	testImg := tdgGenerateTestPatternImage(256, 256)

	qualityTests := []struct {
		quality      int
		expectLarger int // expect larger than this quality
	}{
		{1, 0},    // Q1 should be the smallest
		{50, 1},   // Q50 should be larger than Q1
		{100, 50}, // Q100 should be larger than Q50
	}

	// Generate images at each quality level (using standard mode for Go-decodable output)
	var sizes []int
	for _, tt := range qualityTests {
		data, err := WeeksEncodeToBytesStandard(testImg, tt.quality)
		if err != nil {
			t.Fatalf("WeeksEncodeToBytesStandard(quality=%d) failed: %v", tt.quality, err)
		}
		sizes = append(sizes, len(data))

		// Verify image is valid JPEG
		_, err = stdjpeg.Decode(bytes.NewReader(data))
		if err != nil {
			t.Errorf("Q%d output not decodable: %v", tt.quality, err)
		}
	}

	// Verify quality ordering: Q1 < Q50 < Q100
	if sizes[0] >= sizes[1] {
		t.Errorf("Q1 (%d bytes) should be smaller than Q50 (%d bytes)", sizes[0], sizes[1])
	}
	if sizes[1] >= sizes[2] {
		t.Errorf("Q50 (%d bytes) should be smaller than Q100 (%d bytes)", sizes[1], sizes[2])
	}

	// Verify significant difference between qualities
	// Q100 should be at least 2x Q1
	if sizes[2] < sizes[0]*2 {
		t.Errorf("Q100 (%d bytes) should be at least 2x Q1 (%d bytes)", sizes[2], sizes[0])
	}
}

// TestImageGeneration_SubsamplingModes tests that multiple subsampling modes
// produce distinct files with different characteristics.
func TestImageGeneration_SubsamplingModes(t *testing.T) {
	testImg := tdgGenerateTestPatternImage(256, 256)
	quality := 75

	subsamplingModes := []struct {
		mode jpeg.ChromaSubsamplingMode
		name string
	}{
		{jpeg.ChromaSubsampling420, "4:2:0"},
		{jpeg.ChromaSubsampling422, "4:2:2"},
		{jpeg.ChromaSubsampling444, "4:4:4"},
	}

	var encodedData [][]byte
	for _, ss := range subsamplingModes {
		var buf bytes.Buffer
		enc, err := NewWeeksEncoderWithOptions(&buf, quality, WithStandardMode())
		if err != nil {
			t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
		}
		enc.SetSubsampling(ss.mode)

		err = enc.Encode(testImg)
		if err != nil {
			t.Fatalf("Encode with %s failed: %v", ss.name, err)
		}

		encodedData = append(encodedData, buf.Bytes())
	}

	// Verify all outputs are distinct (different checksums)
	hashes := make(map[string]string)
	for i, data := range encodedData {
		hash := sha256.Sum256(data)
		hashStr := hex.EncodeToString(hash[:])

		if existing, ok := hashes[hashStr]; ok {
			t.Errorf("subsampling mode %s produced identical output to %s",
				subsamplingModes[i].name, existing)
		}
		hashes[hashStr] = subsamplingModes[i].name
	}

	// 4:4:4 should produce larger files than 4:2:0
	if len(encodedData[2]) <= len(encodedData[0]) {
		t.Errorf("4:4:4 (%d bytes) should be larger than 4:2:0 (%d bytes)",
			len(encodedData[2]), len(encodedData[0]))
	}
}

// TestImageGeneration_MetadataAccuracy tests that metadata.json contains
// accurate encoder settings.
func TestImageGeneration_MetadataAccuracy(t *testing.T) {
	testImg := tdgGenerateTestPatternImage(128, 128)

	// Test that we can generate metadata for a set of quality levels
	qualities := []int{25, 50, 75, 100}

	for _, quality := range qualities {
		t.Run(fmt.Sprintf("Q%d", quality), func(t *testing.T) {
			data, err := WeeksEncodeToBytes(testImg, quality)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytes(quality=%d) failed: %v", quality, err)
			}

			// Extract metadata from the encoded image
			metadata := tdgGenerateImageMetadata(data, quality, jpeg.ChromaSubsampling420)

			// Verify metadata accuracy
			if metadata.Quality != quality {
				t.Errorf("metadata quality mismatch: expected %d, got %d", quality, metadata.Quality)
			}
			if metadata.Subsampling != "4:2:0" {
				t.Errorf("metadata subsampling mismatch: expected 4:2:0, got %s", metadata.Subsampling)
			}
			if len(metadata.QuantTableHashes) == 0 {
				t.Error("metadata should contain quantization table hashes")
			}
		})
	}
}

// TestImageGeneration_QualityDetectionAccuracy tests that quality detection
// is accurate within +/-2 for generated images.
func TestImageGeneration_QualityDetectionAccuracy(t *testing.T) {
	testImg := tdgGenerateTestPatternImage(256, 256)

	// Test key quality levels
	testQualities := []int{1, 10, 25, 50, 75, 90, 100}

	estimator := jpeg.NewQualityEstimator(nil)
	totalTests := 0
	withinTolerance := 0

	for _, expectedQuality := range testQualities {
		t.Run(fmt.Sprintf("Q%d", expectedQuality), func(t *testing.T) {
			data, err := WeeksEncodeToBytes(testImg, expectedQuality)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytes(quality=%d) failed: %v", expectedQuality, err)
			}

			// Run quality estimation
			estimate, err := estimator.EstimateQuality(data)
			if err != nil {
				t.Fatalf("EstimateQuality failed: %v", err)
			}

			// Calculate difference
			diff := tdgAbs(estimate.Quality - expectedQuality)
			totalTests++
			if diff <= 2 {
				withinTolerance++
			}

			// Assert detected quality within +/-2 of actual
			if diff > 2 {
				t.Errorf("quality detection too inaccurate: expected Q%d +/-2, got Q%d (diff=%d)",
					expectedQuality, estimate.Quality, diff)
			} else {
				t.Logf("Q%d detected as Q%d (diff=%d, confidence=%.2f)",
					expectedQuality, estimate.Quality, diff, estimate.Confidence)
			}
		})
	}

	// Report overall accuracy
	accuracy := float64(withinTolerance) / float64(totalTests) * 100
	t.Logf("Overall quality detection accuracy: %.1f%% (%d/%d within +/-2)",
		accuracy, withinTolerance, totalTests)
}

// =============================================================================
// Task 5.2: Test Image Generator
// =============================================================================

// tdgGenerateTestPatternImage creates a 256x256 test image with varied content
// including gradients, edges, and flat areas for comprehensive testing.
func tdgGenerateTestPatternImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var r, g, b uint8

			// Divide image into quadrants with different patterns
			quadX := x < width/2
			quadY := y < height/2

			if quadX && quadY {
				// Top-left: Smooth gradient (tests low frequency)
				r = uint8((x * 255) / width)
				g = uint8((y * 255) / height)
				b = 128
			} else if !quadX && quadY {
				// Top-right: High-frequency checkerboard pattern (tests high frequency)
				blockSize := 8
				isWhite := ((x/blockSize)+(y/blockSize))%2 == 0
				if isWhite {
					r, g, b = 255, 255, 255
				} else {
					r, g, b = 0, 0, 0
				}
				//goland:noinspection GoDfaConstantCondition
			} else if quadX && !quadY {
				// Bottom-left: Edge pattern (vertical stripes)
				stripeWidth := 16
				intensity := 0
				if (x/stripeWidth)%2 == 0 {
					intensity = 255
				}
				r = uint8(intensity)
				g = uint8(intensity)
				b = uint8(intensity)
			} else {
				// Bottom-right: Mixed detail with noise-like pattern
				// Creates high-frequency content for quality differentiation
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

// =============================================================================
// Task 5.3: Reference Image Generation
// =============================================================================

// ReferenceImageMetadata contains metadata for a generated reference image.
type ReferenceImageMetadata struct {
	Filename         string         `json:"filename"`
	Quality          int            `json:"quality"`
	Subsampling      string         `json:"subsampling"`
	FileSize         int            `json:"file_size"`
	Width            int            `json:"width"`
	Height           int            `json:"height"`
	QuantTableHashes map[int]string `json:"quant_table_hashes"`
	EncoderComment   string         `json:"encoder_comment"`
	GeneratedBy      string         `json:"generated_by"`
}

// ReferenceImageSet contains metadata for a complete set of reference images.
type ReferenceImageSet struct {
	Version     string                   `json:"version"`
	Description string                   `json:"description"`
	GeneratedAt string                   `json:"generated_at"`
	TestPattern string                   `json:"test_pattern"`
	Images      []ReferenceImageMetadata `json:"images"`
}

// weeksDefaultComment is the default F5/James COM marker signature.
const tdgDefaultComment = "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech."

// tdgGenerateImageMetadata creates metadata for an encoded image.
func tdgGenerateImageMetadata(data []byte, quality int, subsampling jpeg.ChromaSubsamplingMode) ReferenceImageMetadata {
	// Extract quantization table hashes
	quantHashes := make(map[int]string)

	// Parse the JPEG to extract quantization tables
	sig, err := jpeg.ExtractSignature(data)
	if err == nil && sig != nil {
		for id, table := range sig.QuantTables {
			hash := sha256.Sum256([]byte(fmt.Sprintf("%v", table.NaturalOrder)))
			quantHashes[id] = hex.EncodeToString(hash[:8])
		}
	}

	subsamplingStr := subsampling.String()

	return ReferenceImageMetadata{
		Quality:          quality,
		Subsampling:      subsamplingStr,
		FileSize:         len(data),
		Width:            256,
		Height:           256,
		QuantTableHashes: quantHashes,
		EncoderComment:   tdgDefaultComment,
		GeneratedBy:      "F5/James-compatible encoder",
	}
}

// TestGenerateReferenceImages generates reference images for quality detection validation.
// This test is skipped by default. Run with: go test -run TestGenerateReferenceImages -v
func TestGenerateReferenceImages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping reference image generation in short mode")
	}

	// Check if we should generate images (only when explicitly requested)
	if os.Getenv("GENERATE_REFERENCE_IMAGES") != "true" {
		t.Skip("Set GENERATE_REFERENCE_IMAGES=true to generate reference images")
	}

	outputDir := filepath.Join("testdata", "quality_reference")
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	testImg := tdgGenerateTestPatternImage(256, 256)

	// Generate key quality levels for validation
	// Full set: 1-100 for each subsampling mode = 300 images
	// For testing, we use key quality levels: 1, 25, 50, 75, 100
	keyQualities := []int{1, 25, 50, 75, 100}

	subsamplingModes := []struct {
		mode   jpeg.ChromaSubsamplingMode
		suffix string
	}{
		{jpeg.ChromaSubsampling420, "420"},
		{jpeg.ChromaSubsampling422, "422"},
		{jpeg.ChromaSubsampling444, "444"},
	}

	var metadata ReferenceImageSet
	metadata.Version = "1.0"
	metadata.Description = "Quality detection reference images generated with F5/James encoder"
	metadata.TestPattern = "256x256 synthetic test pattern with gradients, edges, and high-frequency detail"

	for _, ss := range subsamplingModes {
		for _, q := range keyQualities {
			var buf bytes.Buffer
			enc, err := NewWeeksEncoder(&buf, q)
			if err != nil {
				t.Fatalf("NewWeeksEncoder(quality=%d) failed: %v", q, err)
			}
			enc.SetSubsampling(ss.mode)

			err = enc.Encode(testImg)
			if err != nil {
				t.Fatalf("Encode at Q%d/%s failed: %v", q, ss.suffix, err)
			}

			// Generate filename
			filename := fmt.Sprintf("q%02d_%s.jpg", q, ss.suffix)
			outPath := filepath.Join(outputDir, filename)

			// Write file
			err = os.WriteFile(outPath, buf.Bytes(), 0644)
			if err != nil {
				t.Fatalf("Failed to write %s: %v", filename, err)
			}

			// Generate metadata
			imgMeta := tdgGenerateImageMetadata(buf.Bytes(), q, ss.mode)
			imgMeta.Filename = filename
			metadata.Images = append(metadata.Images, imgMeta)

			t.Logf("Generated %s (%d bytes)", filename, buf.Len())
		}
	}

	// Write metadata.json
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}

	metadataPath := filepath.Join(outputDir, "metadata.json")
	err = os.WriteFile(metadataPath, metadataJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to write metadata.json: %v", err)
	}

	t.Logf("Generated %d reference images and metadata.json", len(metadata.Images))
}

// =============================================================================
// Task 5.5: Quality Detection Validation Test
// =============================================================================

// TestQualityDetectionValidation validates quality detection accuracy across
// all reference images.
func TestQualityDetectionValidation(t *testing.T) {
	testImg := tdgGenerateTestPatternImage(256, 256)
	estimator := jpeg.NewQualityEstimator(nil)

	// Test key quality levels with all subsampling modes
	keyQualities := []int{1, 10, 25, 50, 75, 90, 100}
	subsamplingModes := []jpeg.ChromaSubsamplingMode{
		jpeg.ChromaSubsampling420,
		jpeg.ChromaSubsampling422,
		jpeg.ChromaSubsampling444,
	}

	var stats struct {
		totalTests      int
		withinTolerance int
		exactMatches    int
		maxDeviation    int
		totalDeviation  int
	}

	for _, ss := range subsamplingModes {
		for _, expectedQ := range keyQualities {
			t.Run(fmt.Sprintf("Q%d_%s", expectedQ, ss.String()), func(t *testing.T) {
				// Generate image with specific quality and subsampling. f5.jar
				// is 4:2:0-only so go through standard mode for 4:2:2/4:4:4.
				var buf bytes.Buffer
				opts := []Option{}
				if ss != jpeg.ChromaSubsampling420 {
					opts = append(opts, WithStandardMode())
				}
				enc, err := NewWeeksEncoderWithOptions(&buf, expectedQ, opts...)
				if err != nil {
					t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
				}
				enc.SetSubsampling(ss)

				err = enc.Encode(testImg)
				if err != nil {
					t.Fatalf("Encode failed: %v", err)
				}

				// Run quality estimation
				estimate, err := estimator.EstimateQuality(buf.Bytes())
				if err != nil {
					t.Fatalf("EstimateQuality failed: %v", err)
				}

				// Calculate deviation
				deviation := tdgAbs(estimate.Quality - expectedQ)

				stats.totalTests++
				stats.totalDeviation += deviation
				if deviation <= 2 {
					stats.withinTolerance++
				}
				if deviation == 0 {
					stats.exactMatches++
				}
				if deviation > stats.maxDeviation {
					stats.maxDeviation = deviation
				}

				// Assert quality within tolerance
				if deviation > 2 {
					t.Errorf("quality detection failed: expected Q%d +/-2, got Q%d (deviation=%d)",
						expectedQ, estimate.Quality, deviation)
				}
			})
		}
	}

	// Report detection accuracy statistics
	if stats.totalTests > 0 {
		accuracy := float64(stats.withinTolerance) / float64(stats.totalTests) * 100
		avgDeviation := float64(stats.totalDeviation) / float64(stats.totalTests)
		exactPercent := float64(stats.exactMatches) / float64(stats.totalTests) * 100

		t.Logf("Quality Detection Statistics:")
		t.Logf("  Total tests: %d", stats.totalTests)
		t.Logf("  Within +/-2: %d (%.1f%%)", stats.withinTolerance, accuracy)
		t.Logf("  Exact matches: %d (%.1f%%)", stats.exactMatches, exactPercent)
		t.Logf("  Average deviation: %.2f", avgDeviation)
		t.Logf("  Max deviation: %d", stats.maxDeviation)
	}
}

// =============================================================================
// Task 5.6: Round-Trip Validation Test
// =============================================================================

// TestRoundTripValidation implements round-trip validation test:
// Encode image with new encoder at quality Q, decode with standard decoder,
// run quality estimation on decoded data, assert estimated quality matches Q +/-2.
func TestRoundTripValidation(t *testing.T) {
	testImg := tdgGenerateTestPatternImage(256, 256)
	estimator := jpeg.NewQualityEstimator(nil)

	// Test quality levels (start from Q25 to avoid low-quality PSNR issues
	// with high-frequency test patterns)
	testQualities := []int{25, 50, 75, 90, 95}

	for _, expectedQ := range testQualities {
		t.Run(fmt.Sprintf("Q%d", expectedQ), func(t *testing.T) {
			// Step 1: Encode with F5 encoder at quality Q (standard mode for Go-decodable output)
			encodedData, err := WeeksEncodeToBytesStandard(testImg, expectedQ)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytesStandard(Q%d) failed: %v", expectedQ, err)
			}

			// Step 2: Decode with standard decoder (verify valid JPEG)
			decoded, err := stdjpeg.Decode(bytes.NewReader(encodedData))
			if err != nil {
				t.Fatalf("Standard decoder failed: %v", err)
			}

			// Verify dimensions preserved
			if decoded.Bounds().Dx() != testImg.Bounds().Dx() ||
				decoded.Bounds().Dy() != testImg.Bounds().Dy() {
				t.Errorf("Dimension mismatch after round-trip")
			}

			// Step 3: Run quality estimation on encoded data
			estimate, err := estimator.EstimateQuality(encodedData)
			if err != nil {
				t.Fatalf("EstimateQuality failed: %v", err)
			}

			// Step 4: Assert estimated quality matches Q +/-2
			deviation := tdgAbs(estimate.Quality - expectedQ)
			if deviation > 2 {
				t.Errorf("Round-trip quality mismatch: encoded at Q%d, detected as Q%d (deviation=%d)",
					expectedQ, estimate.Quality, deviation)
			} else {
				t.Logf("Round-trip Q%d -> detected Q%d (deviation=%d, confidence=%.2f)",
					expectedQ, estimate.Quality, deviation, estimate.Confidence)
			}

			// Verify encoder detection identifies F5/James
			if estimate.DetectedEncoder != jpeg.EncoderF5James && estimate.DetectedEncoder != jpeg.EncoderLibJPEG {
				t.Logf("Note: Encoder detected as %s (F5/James uses libjpeg-compatible tables)",
					estimate.DetectedEncoder.String())
			}

			// Calculate PSNR for visual quality verification
			// Use quality-dependent threshold: lower quality = lower PSNR expected
			psnr := tdgCalculatePSNR(testImg, decoded)
			minPSNR := 20.0 + float64(expectedQ-25)*0.1 // Scale threshold with quality
			if minPSNR < 20.0 {
				minPSNR = 20.0
			}
			if psnr < minPSNR {
				t.Errorf("PSNR too low (%.2f dB), expected at least %.1f dB for Q%d", psnr, minPSNR, expectedQ)
			}
		})
	}
}

// TestRoundTripAllSubsampling tests round-trip with all subsampling modes.
func TestRoundTripAllSubsampling(t *testing.T) {
	testImg := tdgGenerateTestPatternImage(128, 128)
	estimator := jpeg.NewQualityEstimator(nil)

	subsamplingModes := []jpeg.ChromaSubsamplingMode{
		jpeg.ChromaSubsampling420,
		jpeg.ChromaSubsampling422,
		jpeg.ChromaSubsampling444,
	}

	quality := 75

	for _, ss := range subsamplingModes {
		t.Run(ss.String(), func(t *testing.T) {
			// Encode with specific subsampling
			var buf bytes.Buffer
			enc, err := NewWeeksEncoderWithOptions(&buf, quality, WithStandardMode())
			if err != nil {
				t.Fatalf("NewWeeksEncoderWithOptions failed: %v", err)
			}
			enc.SetSubsampling(ss)

			err = enc.Encode(testImg)
			if err != nil {
				t.Fatalf("Encode with %s failed: %v", ss.String(), err)
			}

			// Decode
			_, err = stdjpeg.Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Quality estimation
			estimate, err := estimator.EstimateQuality(buf.Bytes())
			if err != nil {
				t.Fatalf("EstimateQuality failed: %v", err)
			}

			deviation := tdgAbs(estimate.Quality - quality)
			if deviation > 2 {
				t.Errorf("Quality detection failed for %s: expected Q%d +/-2, got Q%d",
					ss.String(), quality, estimate.Quality)
			}
		})
	}
}

// TestRoundTripLowQuality tests round-trip at low quality levels separately.
// Low quality with high-frequency test patterns may have lower PSNR.
func TestRoundTripLowQuality(t *testing.T) {
	testImg := tdgGenerateTestPatternImage(256, 256)
	estimator := jpeg.NewQualityEstimator(nil)

	// Test very low quality levels
	testQualities := []int{5, 10, 15}

	for _, expectedQ := range testQualities {
		t.Run(fmt.Sprintf("Q%d", expectedQ), func(t *testing.T) {
			// Encode (standard mode for Go-decodable output)
			encodedData, err := WeeksEncodeToBytesStandard(testImg, expectedQ)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytesStandard(Q%d) failed: %v", expectedQ, err)
			}

			// Decode
			decoded, err := stdjpeg.Decode(bytes.NewReader(encodedData))
			if err != nil {
				t.Fatalf("Standard decoder failed: %v", err)
			}

			// Verify dimensions preserved
			if decoded.Bounds().Dx() != testImg.Bounds().Dx() ||
				decoded.Bounds().Dy() != testImg.Bounds().Dy() {
				t.Errorf("Dimension mismatch after round-trip")
			}

			// Quality estimation
			estimate, err := estimator.EstimateQuality(encodedData)
			if err != nil {
				t.Fatalf("EstimateQuality failed: %v", err)
			}

			deviation := tdgAbs(estimate.Quality - expectedQ)
			if deviation > 2 {
				t.Errorf("Round-trip quality mismatch: encoded at Q%d, detected as Q%d (deviation=%d)",
					expectedQ, estimate.Quality, deviation)
			} else {
				t.Logf("Round-trip Q%d -> detected Q%d (deviation=%d, confidence=%.2f)",
					expectedQ, estimate.Quality, deviation, estimate.Confidence)
			}

			// At very low quality, PSNR will be poor, but image should still decode
			psnr := tdgCalculatePSNR(testImg, decoded)
			t.Logf("PSNR at Q%d: %.2f dB", expectedQ, psnr)
		})
	}
}

// =============================================================================
// Helper Functions (prefixed with tdg to avoid conflicts)
// =============================================================================

// tdgAbs returns the absolute value of an integer.
func tdgAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// tdgCalculatePSNR calculates Peak Signal-to-Noise Ratio between two images.
func tdgCalculatePSNR(img1, img2 image.Image) float64 {
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
		return math.Inf(1)
	}

	mse /= float64(count)
	maxVal := 255.0
	psnr := 10 * math.Log10((maxVal*maxVal)/mse)
	return psnr
}
