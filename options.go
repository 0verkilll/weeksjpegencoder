// Package weeksjpegencoder provides James R. Weeks-compatible JPEG encoding functionality.
//
// This file implements the functional options pattern for configuring WeeksEncoder.
// Options allow customization of components, comment, and subsampling mode.

package weeksjpegencoder

import (
	"bytes"
	"io"

	"github.com/0verkilll/jpeg"
)

// Option is a functional option for configuring WeeksEncoder.
// Options are applied in order when creating an encoder with NewWeeksEncoderWithOptions.
type Option func(*WeeksEncoder)

// WithQuantizer sets a custom Quantizer implementation.
// Use this to inject a mock quantizer for testing or a custom quantization strategy.
//
// Example:
//
//	customQuant, _ := NewIJGQuantizer(90)
//	enc, _ := NewWeeksEncoderWithOptions(w, 75, WithQuantizer(customQuant))
//
//goland:noinspection GoUnusedExportedFunction
func WithQuantizer(q Quantizer) Option {
	return func(e *WeeksEncoder) {
		e.quantizer = q
	}
}

// WithBlockEncoder sets a custom BlockEncoder implementation.
// Use this to inject a mock encoder for testing or an alternative entropy coding strategy.
//
// Example:
//
//	mockEncoder := &MockBlockEncoder{}
//	enc, _ := NewWeeksEncoderWithOptions(w, 75, WithBlockEncoder(mockEncoder))
//
//goland:noinspection GoUnusedExportedFunction
func WithBlockEncoder(be BlockEncoder) Option {
	return func(e *WeeksEncoder) {
		e.blockEncoder = be
	}
}

// WithBlockTap installs a callback invoked for every quantized DCT block
// immediately before entropy coding. The tap may mutate the block in place;
// the mutation is what gets encoded. Compose with WithBlockEncoder: the tap
// wraps whichever block encoder is in effect (default or custom).
//
// Used by F5 steganalysis pipelines to extract cover coefficients on the fly
// without round-tripping through JPEG decode, and to inject embedded
// coefficients in a second-pass encode for round-trip validation.
//
//goland:noinspection GoUnusedExportedFunction
func WithBlockTap(tap BlockTapFunc) Option {
	return func(e *WeeksEncoder) {
		e.blockTap = tap
	}
}

// WithBlockExtractor sets a custom BlockExtractor implementation.
// Use this to inject a mock extractor for testing or an alternative color space handler.
//
// Example:
//
//	customExtractor := NewYCbCrBlockExtractor(jpeg.ChromaSubsampling444)
//	enc, _ := NewWeeksEncoderWithOptions(w, 75, WithBlockExtractor(customExtractor))
//
//goland:noinspection GoUnusedExportedFunction
func WithBlockExtractor(bx BlockExtractor) Option {
	return func(e *WeeksEncoder) {
		e.blockExtractor = bx
	}
}

// WithDCT sets a custom DCT implementation.
// Use this to inject a mock DCT for testing or an alternative transform.
//
// Example:
//
//	mockDCT := &MockDCT{}
//	enc, _ := NewWeeksEncoderWithOptions(w, 75, WithDCT(mockDCT))
//
//goland:noinspection GoUnusedExportedFunction
func WithDCT(dct DCT) Option {
	return func(e *WeeksEncoder) {
		e.dctInterface = dct
	}
}

// WithComment sets a custom COM marker comment.
// If not specified, the default James R. Weeks signature is used.
//
// Example:
//
//	enc, _ := NewWeeksEncoderWithOptions(w, 75, WithComment("My Custom Comment"))
//
//goland:noinspection GoUnusedExportedFunction
func WithComment(comment string) Option {
	return func(e *WeeksEncoder) {
		e.comment = comment
	}
}

// WithSubsampling sets the chroma subsampling mode.
// This also updates the block extractor to use the new mode.
//
// Supported modes:
//   - jpeg.ChromaSubsampling420: 4:2:0 (default, most common)
//   - jpeg.ChromaSubsampling422: 4:2:2 (horizontal-only subsampling)
//   - jpeg.ChromaSubsampling444: 4:4:4 (no subsampling, highest quality)
//
// Example:
//
//	enc, _ := NewWeeksEncoderWithOptions(w, 75, WithSubsampling(jpeg.ChromaSubsampling444))
//
//goland:noinspection GoUnusedExportedFunction
func WithSubsampling(mode jpeg.ChromaSubsamplingMode) Option {
	return func(e *WeeksEncoder) {
		e.subsampling = mode
		// Update block extractor if it's the default type
		if _, ok := e.blockExtractor.(*YCbCrBlockExtractor); ok {
			e.blockExtractor = NewYCbCrBlockExtractor(mode)
		}
	}
}

// WithStandardMode disables James-compatible mode and uses standard JPEG encoding.
// In standard mode, the encoder:
//   - Applies level shift (subtracts 128) before DCT, as per ITU-T T.81
//   - Uses standard bit buffer layout
//   - Pads remaining bits with 1s (standard behavior)
//
// This produces output that is decodable by Go's standard image/jpeg decoder
// and other standard JPEG decoders, but is NOT byte-identical with the
// James R. Weeks Java encoder.
//
// Use this option when you need Go-decodable output for testing or when
// byte-compatibility with the James encoder is not required.
//
// Example:
//
//	enc, _ := NewWeeksEncoderWithOptions(w, 75, WithStandardMode())
//	// Output will be decodable by image/jpeg.Decode
//
//goland:noinspection GoUnusedExportedFunction
func WithStandardMode() Option {
	return func(e *WeeksEncoder) {
		e.useJamesCompatibleMode = false
	}
}

// =============================================================================
// Throughput Options
// =============================================================================

// WithParallelEncoding enables or disables multi-core encoding for
// James-compatible mode. When enabled (the default), the per-block forward DCT
// and quantization run across multiple goroutines while the entropy (Huffman)
// stage stays sequential, so the output remains byte-identical to f5.jar
// regardless of the worker count. Small images automatically fall back to the
// sequential path where parallelism would not pay off.
//
// Disable it for fully single-threaded, deterministic-scheduling encoding:
//
//	enc, _ := NewWeeksEncoderWithOptions(w, 75, WithParallelEncoding(false))
//
//goland:noinspection GoUnusedExportedFunction
func WithParallelEncoding(enabled bool) Option {
	return func(e *WeeksEncoder) {
		e.parallelEncoding = enabled
	}
}

// WithMaxWorkers caps the number of goroutines used for the parallel
// DCT+quantize phase. A value <= 0 means use runtime.GOMAXPROCS(0) (the
// default). The worker count never affects the output bytes — only throughput.
//
// Example (limit a batch worker to 4 cores per image so other images get CPU):
//
//	enc, _ := NewWeeksEncoderWithOptions(w, 75, WithMaxWorkers(4))
//
//goland:noinspection GoUnusedExportedFunction
func WithMaxWorkers(n int) Option {
	return func(e *WeeksEncoder) {
		e.maxWorkers = n
	}
}

// =============================================================================
// Metadata Preservation Options
// =============================================================================

// WithSourceImage extracts and preserves metadata (EXIF, ICC profile) from a source
// JPEG image. The metadata will be written to the output when encoding.
//
// This option parses the source image's APP1 marker for EXIF data and APP2 markers
// for ICC profile data, storing both for preservation. If the source has no EXIF
// or ICC data, the corresponding APP markers will not be written to the output
// (graceful handling).
//
// The source reader should provide valid JPEG data. If the source is not valid JPEG
// or cannot be read, the metadata fields will remain empty (no error during option
// application; encoding will proceed without metadata).
//
// Note: This function reads all data from the reader, so it cannot be used with
// streaming readers that do not support seeking. Use WithSourceImageBytes for
// byte slice inputs if you need to parse metadata multiple times.
//
// Example:
//
//	f, _ := os.Open("photo_with_metadata.jpg")
//	defer f.Close()
//	enc, _ := NewWeeksEncoderWithOptions(w, 75, WithSourceImage(f))
//
//goland:noinspection GoUnusedExportedFunction
func WithSourceImage(r io.Reader) Option {
	return func(e *WeeksEncoder) {
		// Read all data to allow parsing both EXIF and ICC
		data, err := io.ReadAll(r)
		if err != nil {
			return // Graceful handling - no metadata preserved
		}

		// Parse EXIF from source bytes
		exifData, err := ParseEXIFBytes(data)
		if err == nil && len(exifData) > 0 {
			e.exifData = exifData
			e.hasEXIF = true
		}

		// Parse ICC profile from source bytes
		iccData, err := ParseICCProfileBytes(data)
		if err == nil && len(iccData) > 0 {
			e.iccProfile = iccData
			e.hasICC = true
		}
	}
}

// WithSourceImageBytes extracts and preserves metadata (EXIF, ICC profile) from JPEG bytes.
// This is a convenience wrapper that parses both EXIF and ICC profile data from the
// source bytes.
//
// Example:
//
//	jpegData, _ := os.ReadFile("photo_with_metadata.jpg")
//	enc, _ := NewWeeksEncoderWithOptions(w, 75, WithSourceImageBytes(jpegData))
//
//goland:noinspection GoUnusedExportedFunction
func WithSourceImageBytes(data []byte) Option {
	return func(e *WeeksEncoder) {
		// Parse EXIF from source bytes
		exifData, err := ParseEXIFBytes(data)
		if err == nil && len(exifData) > 0 {
			e.exifData = exifData
			e.hasEXIF = true
		}

		// Parse ICC profile from source bytes
		iccData, err := ParseICCProfileBytes(data)
		if err == nil && len(iccData) > 0 {
			e.iccProfile = iccData
			e.hasICC = true
		}
	}
}

// WithEXIF directly sets EXIF data to be preserved in the output.
// The data should include the EXIF signature ("Exif\x00\x00") prefix.
//
// This option is useful when you have already extracted EXIF data and want to
// apply it to multiple encodings without re-parsing.
//
// Example:
//
//	exifData, _ := ParseEXIF(sourceReader)
//	enc1, _ := NewWeeksEncoderWithOptions(w1, 75, WithEXIF(exifData))
//	enc2, _ := NewWeeksEncoderWithOptions(w2, 90, WithEXIF(exifData))
//
//goland:noinspection GoUnusedExportedFunction
func WithEXIF(exifData []byte) Option {
	return func(e *WeeksEncoder) {
		if len(exifData) > 0 {
			// Verify EXIF signature
			if len(exifData) >= 6 && bytes.Equal(exifData[:6], exifSignature) {
				e.exifData = exifData
				e.hasEXIF = true
			}
		}
	}
}

// WithICCProfile directly sets ICC profile data to be preserved in the output.
// The data should be the raw ICC profile (without APP2 marker overhead).
//
// This option is useful when you have already extracted ICC profile data and want to
// apply it to multiple encodings without re-parsing.
//
// Example:
//
//	iccData, _ := ParseICCProfile(sourceReader)
//	enc1, _ := NewWeeksEncoderWithOptions(w1, 75, WithICCProfile(iccData))
//	enc2, _ := NewWeeksEncoderWithOptions(w2, 90, WithICCProfile(iccData))
//
//goland:noinspection GoUnusedExportedFunction
func WithICCProfile(iccProfile []byte) Option {
	return func(e *WeeksEncoder) {
		if len(iccProfile) > 0 {
			e.iccProfile = iccProfile
			e.hasICC = true
		}
	}
}
