// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file contains tests for the functional options pattern implementation.

package weeksjpegencoder

import (
	"bytes"
	"testing"

	"github.com/0verkilll/jpeg"
)

// TestNewWeeksEncoderWithOptions_NoOptions tests that NewWeeksEncoderWithOptions
// with no options creates an encoder with default behavior.
func TestNewWeeksEncoderWithOptions_NoOptions(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v, want nil", err)
	}

	// Verify encoder is created with defaults
	if enc == nil {
		t.Fatal("NewWeeksEncoderWithOptions() returned nil encoder")
	}

	// Verify default quality
	if enc.quality != 75 {
		t.Errorf("quality = %d, want 75", enc.quality)
	}

	// Verify default comment
	if enc.comment != weeksDefaultComment {
		t.Errorf("comment = %q, want %q", enc.comment, weeksDefaultComment)
	}

	// Verify default subsampling
	if enc.subsampling != jpeg.ChromaSubsampling420 {
		t.Errorf("subsampling = %v, want ChromaSubsampling420", enc.subsampling)
	}

	// Verify default components are created
	if enc.quantizer == nil {
		t.Error("quantizer is nil, want non-nil")
	}
	if enc.blockExtractor == nil {
		t.Error("blockExtractor is nil, want non-nil")
	}
}

// TestWithQuantizer tests that WithQuantizer option injects a custom quantizer.
func TestWithQuantizer(t *testing.T) {
	var buf bytes.Buffer

	// Create a mock quantizer with custom tables
	mockQuant := &MockQuantizer{
		LumTable:   [64]int{1, 2, 3, 4, 5, 6, 7, 8}, // Custom values
		ChromTable: [64]int{10, 20, 30, 40},
	}

	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithQuantizer(mockQuant))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v, want nil", err)
	}

	// Verify custom quantizer was injected
	if enc.quantizer != mockQuant {
		t.Error("quantizer was not set to custom mock")
	}

	// Verify we can get the custom table
	lumTable := enc.quantizer.GetQuantTable(true)
	if lumTable[0] != 1 || lumTable[1] != 2 {
		t.Errorf("GetQuantTable() returned unexpected values: got %v", lumTable[:8])
	}
}

// TestWithBlockEncoder tests that WithBlockEncoder option injects a custom encoder.
func TestWithBlockEncoder(t *testing.T) {
	var buf bytes.Buffer

	// Create a mock block encoder
	mockEncoder := &MockBlockEncoder{}

	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithBlockEncoder(mockEncoder))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v, want nil", err)
	}

	// Verify custom block encoder was injected
	if enc.blockEncoder != mockEncoder {
		t.Error("blockEncoder was not set to custom mock")
	}
}

// TestWithBlockExtractor tests that WithBlockExtractor option injects a custom extractor.
func TestWithBlockExtractor(t *testing.T) {
	var buf bytes.Buffer

	// Create a mock block extractor with fixed output
	mockExtractor := &MockBlockExtractor{
		FixedBlock: [64]float64{100, 110, 120, 130}, // Custom values
	}

	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithBlockExtractor(mockExtractor))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v, want nil", err)
	}

	// Verify custom block extractor was injected
	if enc.blockExtractor != mockExtractor {
		t.Error("blockExtractor was not set to custom mock")
	}
}

// TestWithComment tests that WithComment option sets a custom comment.
func TestWithComment(t *testing.T) {
	var buf bytes.Buffer
	customComment := "My Custom JPEG Comment"

	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithComment(customComment))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v, want nil", err)
	}

	// Verify custom comment was set
	if enc.comment != customComment {
		t.Errorf("comment = %q, want %q", enc.comment, customComment)
	}
}

// TestWithSubsampling tests that WithSubsampling option sets the subsampling mode.
func TestWithSubsampling(t *testing.T) {
	testCases := []struct {
		name     string
		mode     jpeg.ChromaSubsamplingMode
		wantMode jpeg.ChromaSubsamplingMode
	}{
		{"444", jpeg.ChromaSubsampling444, jpeg.ChromaSubsampling444},
		{"422", jpeg.ChromaSubsampling422, jpeg.ChromaSubsampling422},
		{"420", jpeg.ChromaSubsampling420, jpeg.ChromaSubsampling420},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithSubsampling(tc.mode))
			if err != nil {
				t.Fatalf("NewWeeksEncoderWithOptions() error = %v, want nil", err)
			}

			if enc.subsampling != tc.wantMode {
				t.Errorf("subsampling = %v, want %v", enc.subsampling, tc.wantMode)
			}
		})
	}
}

// TestNewWeeksEncoderWithOptions_InvalidQuality tests that invalid quality values
// return appropriate errors.
func TestNewWeeksEncoderWithOptions_InvalidQuality(t *testing.T) {
	testCases := []struct {
		name    string
		quality int
	}{
		{"zero", 0},
		{"negative", -1},
		{"over100", 101},
		{"large", 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			_, err := NewWeeksEncoderWithOptions(&buf, tc.quality)
			if err == nil {
				t.Errorf("NewWeeksEncoderWithOptions(quality=%d) error = nil, want error", tc.quality)
			}
		})
	}
}

// TestNewWeeksEncoderWithOptions_MultipleOptions tests applying multiple options.
func TestNewWeeksEncoderWithOptions_MultipleOptions(t *testing.T) {
	var buf bytes.Buffer
	customComment := "Multi-option test"
	mockQuant := &MockQuantizer{LumTable: [64]int{42}}
	mockExtractor := &MockBlockExtractor{}

	enc, err := NewWeeksEncoderWithOptions(&buf, 75,
		WithComment(customComment),
		WithQuantizer(mockQuant),
		WithBlockExtractor(mockExtractor),
		WithSubsampling(jpeg.ChromaSubsampling444),
	)
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v, want nil", err)
	}

	// Verify all options were applied
	if enc.comment != customComment {
		t.Errorf("comment = %q, want %q", enc.comment, customComment)
	}
	if enc.quantizer != mockQuant {
		t.Error("quantizer was not set to custom mock")
	}
	if enc.blockExtractor != mockExtractor {
		t.Error("blockExtractor was not set to custom mock")
	}
	if enc.subsampling != jpeg.ChromaSubsampling444 {
		t.Errorf("subsampling = %v, want ChromaSubsampling444", enc.subsampling)
	}
}

// TestNewWeeksEncoder_BackwardCompatibility tests that the existing NewWeeksEncoder
// function remains backward compatible.
func TestNewWeeksEncoder_BackwardCompatibility(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder() error = %v, want nil", err)
	}

	// Verify encoder is created with defaults
	if enc == nil {
		t.Fatal("NewWeeksEncoder() returned nil encoder")
	}
	if enc.quality != 75 {
		t.Errorf("quality = %d, want 75", enc.quality)
	}
	if enc.comment != weeksDefaultComment {
		t.Errorf("comment = %q, want %q", enc.comment, weeksDefaultComment)
	}
	if enc.subsampling != jpeg.ChromaSubsampling420 {
		t.Errorf("subsampling = %v, want ChromaSubsampling420", enc.subsampling)
	}
	if enc.quantizer == nil {
		t.Error("quantizer is nil, want non-nil")
	}
	if enc.blockExtractor == nil {
		t.Error("blockExtractor is nil, want non-nil")
	}
}

// TestWithDCT tests that WithDCT option injects a custom DCT implementation.
func TestWithDCT(t *testing.T) {
	var buf bytes.Buffer

	// Create a mock DCT
	mockDCT := &MockDCT{}

	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithDCT(mockDCT))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v, want nil", err)
	}

	// Verify custom DCT was injected - check via the dctInterface field
	if enc.dctInterface != mockDCT {
		t.Error("dct was not set to custom mock")
	}
}
