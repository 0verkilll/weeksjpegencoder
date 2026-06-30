// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file contains tests for EXIF metadata preservation functionality.

package weeksjpegencoder

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

// =============================================================================
// Test Helpers for EXIF Tests
// =============================================================================

// createMinimalJPEGWithEXIF creates a minimal valid JPEG with embedded EXIF data.
// This is a manually crafted JPEG for testing EXIF parsing.
func createMinimalJPEGWithEXIF(exifContent []byte) []byte {
	var buf bytes.Buffer

	// SOI marker
	buf.Write([]byte{0xFF, 0xD8})

	// APP0 (JFIF) marker
	jfifData := []byte("JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")
	buf.Write([]byte{0xFF, 0xE0})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(jfifData)+2))
	buf.Write(jfifData)

	// APP1 (EXIF) marker with provided content
	if len(exifContent) > 0 {
		fullEXIF := append([]byte("Exif\x00\x00"), exifContent...)
		buf.Write([]byte{0xFF, 0xE1})
		_ = binary.Write(&buf, binary.BigEndian, uint16(len(fullEXIF)+2))
		buf.Write(fullEXIF)
	}

	// Minimal DQT marker
	dqt := make([]byte, 65)
	dqt[0] = 0x00 // Table ID 0, precision 0
	for i := 1; i < 65; i++ {
		dqt[i] = 16 // Simple quantization values
	}
	buf.Write([]byte{0xFF, 0xDB})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(dqt)+2))
	buf.Write(dqt)

	// SOF0 marker (baseline DCT, 8x8 image, 1 component for simplicity)
	sof0 := []byte{
		0x08,       // Precision: 8 bits
		0x00, 0x08, // Height: 8
		0x00, 0x08, // Width: 8
		0x01, // Number of components: 1
		0x01, // Component ID: 1
		0x11, // Sampling factors: 1x1
		0x00, // Quantization table ID: 0
	}
	buf.Write([]byte{0xFF, 0xC0})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(sof0)+2))
	buf.Write(sof0)

	// DHT marker (minimal DC table)
	dht := []byte{
		0x00,                         // DC table 0
		0, 1, 0, 0, 0, 0, 0, 0, 0, 0, // Bit counts
		0, 0, 0, 0, 0, 0, // Bit counts continued
		0x00, // Symbol
	}
	buf.Write([]byte{0xFF, 0xC4})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(dht)+2))
	buf.Write(dht)

	// SOS marker (start of scan)
	sos := []byte{
		0x01,       // Number of components
		0x01, 0x00, // Component 1, DC/AC table IDs
		0x00, 0x3F, // Spectral selection
		0x00, // Successive approximation
	}
	buf.Write([]byte{0xFF, 0xDA})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(sos)+2))
	buf.Write(sos)

	// Minimal scan data (single byte)
	buf.Write([]byte{0x00})

	// EOI marker
	buf.Write([]byte{0xFF, 0xD9})

	return buf.Bytes()
}

// createMinimalJPEGWithoutEXIF creates a minimal valid JPEG without EXIF.
func createMinimalJPEGWithoutEXIF() []byte {
	return createMinimalJPEGWithEXIF(nil)
}

// createExifTestImage creates a simple test image for EXIF encoding tests.
func createExifTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create a gradient pattern
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8(128)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

// =============================================================================
// Task 6.1: EXIF Preservation Tests
// =============================================================================

// TestParseEXIF_WithValidEXIF tests parsing EXIF from source image with APP1 marker.
func TestParseEXIF_WithValidEXIF(t *testing.T) {
	// Create test EXIF content (simulated TIFF header + minimal IFD)
	testEXIFContent := []byte{
		// TIFF header (little endian)
		0x49, 0x49, // "II" - little endian
		0x2A, 0x00, // TIFF magic number
		0x08, 0x00, 0x00, 0x00, // Offset to first IFD
		// Minimal IFD
		0x00, 0x00, // Zero entries (minimal valid IFD)
		0x00, 0x00, 0x00, 0x00, // Next IFD offset (none)
	}

	jpegData := createMinimalJPEGWithEXIF(testEXIFContent)

	// Parse EXIF
	exifData, err := ParseEXIFBytes(jpegData)
	if err != nil {
		t.Fatalf("ParseEXIFBytes() error = %v, want nil", err)
	}

	// Verify EXIF signature is present
	if len(exifData) < 6 {
		t.Fatalf("EXIF data too short: got %d bytes", len(exifData))
	}

	if !bytes.Equal(exifData[:6], []byte("Exif\x00\x00")) {
		t.Errorf("EXIF signature mismatch: got %v", exifData[:6])
	}

	// Verify EXIF content follows signature
	if !bytes.Equal(exifData[6:], testEXIFContent) {
		t.Errorf("EXIF content mismatch: got %v, want %v", exifData[6:], testEXIFContent)
	}
}

// TestParseEXIF_NoEXIF tests graceful handling when source has no EXIF.
func TestParseEXIF_NoEXIF(t *testing.T) {
	jpegData := createMinimalJPEGWithoutEXIF()

	exifData, err := ParseEXIFBytes(jpegData)

	if err != ErrNoEXIF {
		t.Errorf("ParseEXIFBytes() error = %v, want ErrNoEXIF", err)
	}

	if exifData != nil {
		t.Errorf("ParseEXIFBytes() returned data when none expected: %v", exifData)
	}
}

// TestParseEXIF_InvalidJPEG tests error handling for invalid JPEG data.
func TestParseEXIF_InvalidJPEG(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"too_short", []byte{0xFF}},
		{"wrong_magic", []byte{0x00, 0x00, 0x00, 0x00}},
		{"no_soi", []byte{0xFF, 0x00, 0xFF, 0xD9}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseEXIFBytes(tc.data)
			if err != ErrInvalidJPEG {
				t.Errorf("ParseEXIFBytes(%v) error = %v, want ErrInvalidJPEG", tc.data, err)
			}
		})
	}
}

// TestEncodePreservesEXIF tests that encoding preserves EXIF in output.
func TestEncodePreservesEXIF(t *testing.T) {
	// Create test EXIF content
	testEXIFContent := []byte{
		// TIFF header (little endian)
		0x49, 0x49, // "II" - little endian
		0x2A, 0x00, // TIFF magic number
		0x08, 0x00, 0x00, 0x00, // Offset to first IFD
		// Minimal IFD with one entry
		0x01, 0x00, // One entry
		0x0F, 0x01, // Tag: Make (0x010F)
		0x02, 0x00, // Type: ASCII
		0x05, 0x00, 0x00, 0x00, // Count: 5
		0x1A, 0x00, 0x00, 0x00, // Value offset
		0x00, 0x00, 0x00, 0x00, // Next IFD offset (none)
		// String data at offset 0x1A
		'T', 'e', 's', 't', 0x00,
	}

	sourceJPEG := createMinimalJPEGWithEXIF(testEXIFContent)
	img := createExifTestImage(64, 64)

	// Encode with EXIF preservation
	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithSourceImageBytes(sourceJPEG))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v", err)
	}

	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// Parse EXIF from output
	outputEXIF, err := ParseEXIFBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseEXIFBytes(output) error = %v", err)
	}

	// Verify EXIF content matches
	expectedEXIF := append([]byte("Exif\x00\x00"), testEXIFContent...)
	if !bytes.Equal(outputEXIF, expectedEXIF) {
		t.Errorf("EXIF content mismatch in output")
		t.Logf("Expected: %v", expectedEXIF)
		t.Logf("Got:      %v", outputEXIF)
	}
}

// TestEXIFAppearsAfterAPP0 tests that EXIF (APP1) appears after APP0 (JFIF) marker in output.
func TestEXIFAppearsAfterAPP0(t *testing.T) {
	testEXIFContent := []byte{
		0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	sourceJPEG := createMinimalJPEGWithEXIF(testEXIFContent)
	img := createExifTestImage(64, 64)

	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithSourceImageBytes(sourceJPEG))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v", err)
	}

	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	outputData := buf.Bytes()

	// Find marker positions
	app0Pos := -1
	app1Pos := -1

	for i := 0; i < len(outputData)-1; i++ {
		if outputData[i] == 0xFF {
			if outputData[i+1] == 0xE0 && app0Pos == -1 {
				app0Pos = i
			}
			if outputData[i+1] == 0xE1 && app1Pos == -1 {
				app1Pos = i
			}
		}
	}

	if app0Pos == -1 {
		t.Fatal("APP0 marker not found in output")
	}
	if app1Pos == -1 {
		t.Fatal("APP1 (EXIF) marker not found in output")
	}
	if app1Pos <= app0Pos {
		t.Errorf("APP1 (EXIF) at position %d should appear after APP0 at position %d", app1Pos, app0Pos)
	}

	t.Logf("Marker order correct: APP0 at %d, APP1 (EXIF) at %d", app0Pos, app1Pos)
}

// TestEXIFStructureIntegrity tests that EXIF structure integrity is maintained.
func TestEXIFStructureIntegrity(t *testing.T) {
	// Create realistic EXIF content with multiple IFD entries
	testEXIFContent := []byte{
		// TIFF header (big endian this time for variety)
		0x4D, 0x4D, // "MM" - big endian
		0x00, 0x2A, // TIFF magic number (big endian)
		0x00, 0x00, 0x00, 0x08, // Offset to first IFD

		// IFD0 with 3 entries
		0x00, 0x03, // 3 entries

		// Entry 1: ImageWidth (0x0100)
		0x01, 0x00, // Tag
		0x00, 0x03, // Type: SHORT
		0x00, 0x00, 0x00, 0x01, // Count
		0x00, 0x40, 0x00, 0x00, // Value: 64

		// Entry 2: ImageLength (0x0101)
		0x01, 0x01, // Tag
		0x00, 0x03, // Type: SHORT
		0x00, 0x00, 0x00, 0x01, // Count
		0x00, 0x40, 0x00, 0x00, // Value: 64

		// Entry 3: Orientation (0x0112)
		0x01, 0x12, // Tag
		0x00, 0x03, // Type: SHORT
		0x00, 0x00, 0x00, 0x01, // Count
		0x00, 0x01, 0x00, 0x00, // Value: 1 (normal)

		// Next IFD offset
		0x00, 0x00, 0x00, 0x00,
	}

	sourceJPEG := createMinimalJPEGWithEXIF(testEXIFContent)
	img := createExifTestImage(64, 64)

	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithSourceImageBytes(sourceJPEG))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v", err)
	}

	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// Extract EXIF from output
	outputEXIF, err := ParseEXIFBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseEXIFBytes(output) error = %v", err)
	}

	// Verify byte-for-byte match
	expectedEXIF := append([]byte("Exif\x00\x00"), testEXIFContent...)
	if !bytes.Equal(outputEXIF, expectedEXIF) {
		t.Errorf("EXIF structure was modified during encoding")
		t.Logf("Expected length: %d, Got length: %d", len(expectedEXIF), len(outputEXIF))

		// Find first difference
		for i := 0; i < len(outputEXIF) && i < len(expectedEXIF); i++ {
			if outputEXIF[i] != expectedEXIF[i] {
				t.Logf("First difference at byte %d: expected 0x%02X, got 0x%02X",
					i, expectedEXIF[i], outputEXIF[i])
				break
			}
		}
	}
}

// TestEncodeWithoutEXIFSource tests encoding when source has no EXIF (graceful handling).
func TestEncodeWithoutEXIFSource(t *testing.T) {
	sourceJPEG := createMinimalJPEGWithoutEXIF()
	img := createExifTestImage(64, 64)

	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithSourceImageBytes(sourceJPEG))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v", err)
	}

	// Encoding should succeed even without EXIF in source
	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// Output should be valid JPEG
	if len(buf.Bytes()) < 4 {
		t.Fatal("Output too short to be valid JPEG")
	}
	if buf.Bytes()[0] != 0xFF || buf.Bytes()[1] != 0xD8 {
		t.Error("Output missing SOI marker")
	}

	// Should not have APP1 EXIF marker in output
	_, err = ParseEXIFBytes(buf.Bytes())
	if err != ErrNoEXIF {
		t.Errorf("Expected no EXIF in output when source has none, got err=%v", err)
	}
}
