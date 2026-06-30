// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements benchmark tests for the F5/James encoder.
// Migrated from jpeg/f5_fuzz_benchmark_test.go with updates for standalone package.
//
// Benchmark tests:
//   - BenchmarkQualityEstimation: Measures quality detection performance
//   - BenchmarkSignatureExtraction: Measures metadata extraction performance
//   - BenchmarkEncoderDetection: Measures encoder identification performance
//   - BenchmarkEncode: Measures JPEG encoding at various quality levels
//   - BenchmarkEncodeSubsampling: Compares subsampling mode performance
//   - BenchmarkRoundTrip: Measures encode-decode cycle performance

package weeksjpegencoder

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"testing"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// Verification Tests for Benchmark Infrastructure
// =============================================================================

// TestBenchmarks_ProduceMeaningfulData verifies that benchmarks execute correctly
// and produce meaningful timing data.
func TestBenchmarks_ProduceMeaningfulData(t *testing.T) {
	// Create test image
	img := bmCreateTestImage(64, 64)

	// Encode to JPEG for quality estimation tests
	jpegData, err := WeeksEncodeToBytes(img, 75)
	if err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}

	// Test that quality estimation completes in reasonable time
	t.Run("quality_estimation_completes", func(t *testing.T) {
		estimator := jpeg.NewQualityEstimator(nil)
		estimate, err := estimator.EstimateQuality(jpegData)
		if err != nil {
			t.Fatalf("Quality estimation failed: %v", err)
		}
		if estimate.Quality < 1 || estimate.Quality > 100 {
			t.Errorf("Invalid quality estimate: %d", estimate.Quality)
		}
	})

	// Test that signature extraction completes
	t.Run("signature_extraction_completes", func(t *testing.T) {
		sig, err := jpeg.ExtractSignature(jpegData)
		if err != nil {
			t.Fatalf("Signature extraction failed: %v", err)
		}
		if sig == nil {
			t.Error("Signature should not be nil")
		}
	})

	// Test that encoding completes at various quality levels
	qualityLevels := []int{50, 75, 90}
	for _, q := range qualityLevels {
		t.Run(fmt.Sprintf("encoding_completes_Q%d", q), func(t *testing.T) {
			encoded, err := WeeksEncodeToBytes(img, q)
			if err != nil {
				t.Fatalf("Encoding failed at Q%d: %v", q, err)
			}
			if len(encoded) == 0 {
				t.Errorf("Encoding produced empty output at Q%d", q)
			}
		})
	}

	// Test round-trip completes (uses standard mode for Go-decodable output)
	t.Run("roundtrip_completes", func(t *testing.T) {
		encoded, err := WeeksEncodeToBytesStandard(img, 75)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

		decoded, err := stdjpeg.Decode(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		if decoded == nil {
			t.Error("Decoded image should not be nil")
		}
	})
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// BenchmarkQualityEstimation measures single-file quality detection performance.
func BenchmarkQualityEstimation(b *testing.B) {
	// Create test JPEG data
	img := bmCreateTestImage(256, 256)
	jpegData, err := WeeksEncodeToBytes(img, 75)
	if err != nil {
		b.Fatalf("Failed to create test JPEG: %v", err)
	}

	estimator := jpeg.NewQualityEstimator(nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := estimator.EstimateQuality(jpegData)
		if err != nil {
			b.Fatalf("Quality estimation failed: %v", err)
		}
	}
}

// BenchmarkSignatureExtraction measures metadata extraction performance.
func BenchmarkSignatureExtraction(b *testing.B) {
	img := bmCreateTestImage(256, 256)
	jpegData, err := WeeksEncodeToBytes(img, 75)
	if err != nil {
		b.Fatalf("Failed to create test JPEG: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := jpeg.ExtractSignature(jpegData)
		if err != nil {
			b.Fatalf("Signature extraction failed: %v", err)
		}
	}
}

// BenchmarkEncoderDetection measures encoder family identification performance.
func BenchmarkEncoderDetection(b *testing.B) {
	img := bmCreateTestImage(256, 256)
	jpegData, err := WeeksEncodeToBytes(img, 75)
	if err != nil {
		b.Fatalf("Failed to create test JPEG: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sig, err := jpeg.ExtractSignature(jpegData)
		if err != nil {
			b.Fatalf("Extraction failed: %v", err)
		}
		_ = sig.EncoderHints
	}
}

// BenchmarkEncode measures JPEG encoding at various quality levels.
func BenchmarkEncode(b *testing.B) {
	img := bmCreateTestImage(256, 256)

	qualities := []int{50, 75, 90}
	for _, q := range qualities {
		b.Run(fmt.Sprintf("Q%d", q), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := WeeksEncodeToBytes(img, q)
				if err != nil {
					b.Fatalf("Encoding failed at Q%d: %v", q, err)
				}
			}
		})
	}
}

// BenchmarkEncodeSubsampling compares 4:2:0, 4:2:2, 4:4:4 encoding performance.
func BenchmarkEncodeSubsampling(b *testing.B) {
	img := bmCreateTestImage(256, 256)

	subsamplingModes := []struct {
		name string
		mode jpeg.ChromaSubsamplingMode
	}{
		{"420", jpeg.ChromaSubsampling420},
		{"422", jpeg.ChromaSubsampling422},
		{"444", jpeg.ChromaSubsampling444},
	}

	for _, ss := range subsamplingModes {
		b.Run(ss.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var buf bytes.Buffer
				enc, err := NewWeeksEncoder(&buf, 75)
				if err != nil {
					b.Fatalf("Failed to create encoder: %v", err)
				}
				enc.SetSubsampling(ss.mode)
				err = enc.Encode(img)
				if err != nil {
					b.Fatalf("Encoding failed with %s: %v", ss.name, err)
				}
			}
		})
	}
}

// BenchmarkRoundTrip measures encode-decode cycle performance.
// Note: Uses standard encoding mode (not James-compatible) because the James
// encoder produces output that Go's standard JPEG decoder cannot read
// (due to no level shift - see DEC-004 in decisions.md).
func BenchmarkRoundTrip(b *testing.B) {
	img := bmCreateTestImage(256, 256)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Encode using standard mode (decodable by Go's image/jpeg)
		encoded, err := WeeksEncodeToBytesStandard(img, 75)
		if err != nil {
			b.Fatalf("Encoding failed: %v", err)
		}

		// Decode
		_, err = stdjpeg.Decode(bytes.NewReader(encoded))
		if err != nil {
			b.Fatalf("Decoding failed: %v", err)
		}
	}
}

// BenchmarkEncode_ImageSizes benchmarks encoding with various image sizes.
func BenchmarkEncode_ImageSizes(b *testing.B) {
	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{"64x64_icon", 64, 64},
		{"256x256_thumbnail", 256, 256},
		{"1024x1024_web", 1024, 1024},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			img := bmCreateTestImage(size.width, size.height)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := WeeksEncodeToBytes(img, 75)
				if err != nil {
					b.Fatalf("Encoding failed for %s: %v", size.name, err)
				}
			}
		})
	}
}

// BenchmarkQualityEstimation_ImageSizes benchmarks quality estimation with various sizes.
func BenchmarkQualityEstimation_ImageSizes(b *testing.B) {
	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{"64x64_icon", 64, 64},
		{"256x256_thumbnail", 256, 256},
		{"1024x1024_web", 1024, 1024},
	}

	estimator := jpeg.NewQualityEstimator(nil)

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			img := bmCreateTestImage(size.width, size.height)
			jpegData, err := WeeksEncodeToBytes(img, 75)
			if err != nil {
				b.Fatalf("Failed to create test JPEG: %v", err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := estimator.EstimateQuality(jpegData)
				if err != nil {
					b.Fatalf("Quality estimation failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSignatureExtraction_ImageSizes benchmarks signature extraction with various sizes.
func BenchmarkSignatureExtraction_ImageSizes(b *testing.B) {
	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{"64x64_icon", 64, 64},
		{"256x256_thumbnail", 256, 256},
		{"1024x1024_web", 1024, 1024},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			img := bmCreateTestImage(size.width, size.height)
			jpegData, err := WeeksEncodeToBytes(img, 75)
			if err != nil {
				b.Fatalf("Failed to create test JPEG: %v", err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := jpeg.ExtractSignature(jpegData)
				if err != nil {
					b.Fatalf("Signature extraction failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkNewWeeksEncoder measures encoder creation overhead.
func BenchmarkNewWeeksEncoder(b *testing.B) {
	qualities := []int{25, 50, 75, 100}

	for _, q := range qualities {
		b.Run(fmt.Sprintf("Q%d", q), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var buf bytes.Buffer
				_, err := NewWeeksEncoder(&buf, q)
				if err != nil {
					b.Fatalf("NewWeeksEncoder failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkEncode_AllQualities benchmarks encoding across the full quality range.
func BenchmarkEncode_AllQualities(b *testing.B) {
	img := bmCreateTestImage(128, 128)
	qualities := []int{1, 10, 25, 50, 75, 90, 100}

	for _, q := range qualities {
		b.Run(fmt.Sprintf("Q%d", q), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := WeeksEncodeToBytes(img, q)
				if err != nil {
					b.Fatalf("Encoding failed at Q%d: %v", q, err)
				}
			}
		})
	}
}

// =============================================================================
// Helper Functions for Benchmarks (prefixed with bm to avoid conflicts)
// =============================================================================

// bmCreateTestImage creates a test image with varied content.
func bmCreateTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create varied content: gradients and patterns
			r := uint8((x * 255) / bmMax(width-1, 1))
			g := uint8((y * 255) / bmMax(height-1, 1))
			b := uint8(((x + y) * 127) / bmMax(width+height-2, 1))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}

// bmMax returns the larger of two integers.
func bmMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
