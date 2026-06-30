// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file contains tests for ICC color profile preservation functionality.

package weeksjpegencoder

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

// =============================================================================
// Test Helpers for ICC Tests
// =============================================================================

// createMinimalJPEGWithICC creates a minimal valid JPEG with embedded ICC profile.
// This is a manually crafted JPEG for testing ICC parsing.
func createMinimalJPEGWithICC(iccProfile []byte) []byte {
	var buf bytes.Buffer

	// SOI marker
	_, _ = buf.Write([]byte{0xFF, 0xD8})

	// APP0 (JFIF) marker
	jfifData := []byte("JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")
	_, _ = buf.Write([]byte{0xFF, 0xE0})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(jfifData)+2))
	_, _ = buf.Write(jfifData)

	// APP2 (ICC) marker(s) with provided profile
	if len(iccProfile) > 0 {
		// Calculate segments needed
		numSegments := (len(iccProfile) + iccMaxSegmentData - 1) / iccMaxSegmentData
		if numSegments == 0 {
			numSegments = 1
		}

		offset := 0
		for segNum := 1; segNum <= numSegments; segNum++ {
			chunkSize := iccMaxSegmentData
			remaining := len(iccProfile) - offset
			if remaining < chunkSize {
				chunkSize = remaining
			}

			// Build segment: signature + seq/count + data
			var segData bytes.Buffer
			_, _ = segData.Write(iccSignature)       // "ICC_PROFILE\x00"
			_ = segData.WriteByte(byte(segNum))      // sequence number
			_ = segData.WriteByte(byte(numSegments)) // total count
			_, _ = segData.Write(iccProfile[offset : offset+chunkSize])

			_, _ = buf.Write([]byte{0xFF, 0xE2})
			_ = binary.Write(&buf, binary.BigEndian, uint16(segData.Len()+2))
			_, _ = buf.Write(segData.Bytes())

			offset += chunkSize
		}
	}

	// Minimal DQT marker
	dqt := make([]byte, 65)
	dqt[0] = 0x00 // Table ID 0, precision 0
	for i := 1; i < 65; i++ {
		dqt[i] = 16 // Simple quantization values
	}
	_, _ = buf.Write([]byte{0xFF, 0xDB})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(dqt)+2))
	_, _ = buf.Write(dqt)

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
	_, _ = buf.Write([]byte{0xFF, 0xC0})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(sof0)+2))
	_, _ = buf.Write(sof0)

	// DHT marker (minimal DC table)
	dht := []byte{
		0x00,                         // DC table 0
		0, 1, 0, 0, 0, 0, 0, 0, 0, 0, // Bit counts
		0, 0, 0, 0, 0, 0, // Bit counts continued
		0x00, // Symbol
	}
	_, _ = buf.Write([]byte{0xFF, 0xC4})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(dht)+2))
	_, _ = buf.Write(dht)

	// SOS marker (start of scan)
	sos := []byte{
		0x01,       // Number of components
		0x01, 0x00, // Component 1, DC/AC table IDs
		0x00, 0x3F, // Spectral selection
		0x00, // Successive approximation
	}
	_, _ = buf.Write([]byte{0xFF, 0xDA})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(sos)+2))
	_, _ = buf.Write(sos)

	// Minimal scan data (single byte)
	_, _ = buf.Write([]byte{0x00})

	// EOI marker
	_, _ = buf.Write([]byte{0xFF, 0xD9})

	return buf.Bytes()
}

// createMinimalJPEGWithBothMetadata creates a JPEG with both EXIF and ICC.
func createMinimalJPEGWithBothMetadata(exifContent, iccProfile []byte) []byte {
	var buf bytes.Buffer

	// SOI marker
	_, _ = buf.Write([]byte{0xFF, 0xD8})

	// APP0 (JFIF) marker
	jfifData := []byte("JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")
	_, _ = buf.Write([]byte{0xFF, 0xE0})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(jfifData)+2))
	_, _ = buf.Write(jfifData)

	// APP1 (EXIF) marker
	if len(exifContent) > 0 {
		fullEXIF := append([]byte("Exif\x00\x00"), exifContent...)
		buf.Write([]byte{0xFF, 0xE1})
		_ = binary.Write(&buf, binary.BigEndian, uint16(len(fullEXIF)+2))
		buf.Write(fullEXIF)
	}

	// APP2 (ICC) marker(s)
	if len(iccProfile) > 0 {
		numSegments := (len(iccProfile) + iccMaxSegmentData - 1) / iccMaxSegmentData
		if numSegments == 0 {
			numSegments = 1
		}

		offset := 0
		for segNum := 1; segNum <= numSegments; segNum++ {
			chunkSize := iccMaxSegmentData
			remaining := len(iccProfile) - offset
			if remaining < chunkSize {
				chunkSize = remaining
			}

			var segData bytes.Buffer
			_, _ = segData.Write(iccSignature)
			_ = segData.WriteByte(byte(segNum))
			_ = segData.WriteByte(byte(numSegments))
			_, _ = segData.Write(iccProfile[offset : offset+chunkSize])

			_, _ = buf.Write([]byte{0xFF, 0xE2})
			_ = binary.Write(&buf, binary.BigEndian, uint16(segData.Len()+2))
			_, _ = buf.Write(segData.Bytes())

			offset += chunkSize
		}
	}

	// Minimal DQT marker
	dqt := make([]byte, 65)
	dqt[0] = 0x00
	for i := 1; i < 65; i++ {
		dqt[i] = 16
	}
	_, _ = buf.Write([]byte{0xFF, 0xDB})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(dqt)+2))
	_, _ = buf.Write(dqt)

	// SOF0 marker
	sof0 := []byte{
		0x08, 0x00, 0x08, 0x00, 0x08, 0x01, 0x01, 0x11, 0x00,
	}
	_, _ = buf.Write([]byte{0xFF, 0xC0})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(sof0)+2))
	_, _ = buf.Write(sof0)

	// DHT marker
	dht := []byte{
		0x00, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x00,
	}
	_, _ = buf.Write([]byte{0xFF, 0xC4})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(dht)+2))
	_, _ = buf.Write(dht)

	// SOS marker
	sos := []byte{0x01, 0x01, 0x00, 0x00, 0x3F, 0x00}
	_, _ = buf.Write([]byte{0xFF, 0xDA})
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(sos)+2))
	_, _ = buf.Write(sos)

	_, _ = buf.Write([]byte{0x00})
	_, _ = buf.Write([]byte{0xFF, 0xD9})

	return buf.Bytes()
}

// createMinimalJPEGWithoutICC creates a minimal valid JPEG without ICC.
func createMinimalJPEGWithoutICC() []byte {
	return createMinimalJPEGWithICC(nil)
}

// createICCTestImage creates a simple test image for ICC encoding tests.
func createICCTestImage(width, height int) image.Image {
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

// createSampleICCProfile creates a minimal sRGB ICC profile header for testing.
// This is a simplified profile header structure, not a fully valid ICC profile,
// but sufficient for testing parsing and preservation.
func createSampleICCProfile(size int) []byte {
	profile := make([]byte, size)

	// ICC profile header (128 bytes minimum)
	if size >= 128 {
		// Profile size (big-endian)
		binary.BigEndian.PutUint32(profile[0:4], uint32(size))

		// Preferred CMM type
		copy(profile[4:8], "appl")

		// Profile version (4.0)
		profile[8] = 4
		profile[9] = 0
		profile[10] = 0
		profile[11] = 0

		// Profile/Device class - Display
		copy(profile[12:16], "mntr")

		// Color space - RGB
		copy(profile[16:20], "RGB ")

		// PCS - XYZ
		copy(profile[20:24], "XYZ ")

		// Creation date/time
		binary.BigEndian.PutUint16(profile[24:26], 2024)
		binary.BigEndian.PutUint16(profile[26:28], 1)
		binary.BigEndian.PutUint16(profile[28:30], 1)

		// Profile file signature
		copy(profile[36:40], "acsp")

		// Primary platform
		copy(profile[40:44], "APPL")

		// Fill rest with pattern for verification
		for i := 128; i < size; i++ {
			profile[i] = byte(i % 256)
		}
	}

	return profile
}

// =============================================================================
// Task 7.1: ICC Profile Preservation Tests
// =============================================================================

// TestParseICCProfile_WithValidICC tests parsing ICC profile from source with APP2 marker.
func TestParseICCProfile_WithValidICC(t *testing.T) {
	// Create a sample ICC profile
	testICCProfile := createSampleICCProfile(256)

	jpegData := createMinimalJPEGWithICC(testICCProfile)

	// Parse ICC profile
	iccData, err := ParseICCProfileBytes(jpegData)
	if err != nil {
		t.Fatalf("ParseICCProfileBytes() error = %v, want nil", err)
	}

	// Verify profile matches
	if !bytes.Equal(iccData, testICCProfile) {
		t.Errorf("ICC profile mismatch")
		t.Logf("Expected length: %d, Got length: %d", len(testICCProfile), len(iccData))
		if len(iccData) >= 4 && len(testICCProfile) >= 4 {
			t.Logf("Expected first 4 bytes: %v", testICCProfile[:4])
			t.Logf("Got first 4 bytes: %v", iccData[:4])
		}
	}
}

// TestEncodePreservesICC tests that encoding preserves ICC profile in output.
func TestEncodePreservesICC(t *testing.T) {
	// Create a sample ICC profile
	testICCProfile := createSampleICCProfile(512)

	sourceJPEG := createMinimalJPEGWithICC(testICCProfile)
	img := createICCTestImage(64, 64)

	// Encode with ICC preservation
	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithSourceImageBytes(sourceJPEG))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v", err)
	}

	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// Parse ICC from output
	outputICC, err := ParseICCProfileBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseICCProfileBytes(output) error = %v", err)
	}

	// Verify ICC content matches
	if !bytes.Equal(outputICC, testICCProfile) {
		t.Errorf("ICC profile mismatch in output")
		t.Logf("Expected length: %d, Got length: %d", len(testICCProfile), len(outputICC))
	}
}

// TestParseICCProfile_NoICC tests graceful handling when source has no ICC profile.
func TestParseICCProfile_NoICC(t *testing.T) {
	jpegData := createMinimalJPEGWithoutICC()

	iccData, err := ParseICCProfileBytes(jpegData)

	if err != ErrNoICC {
		t.Errorf("ParseICCProfileBytes() error = %v, want ErrNoICC", err)
	}

	if iccData != nil {
		t.Errorf("ParseICCProfileBytes() returned data when none expected: %d bytes", len(iccData))
	}
}

// TestParseICCProfile_MultiSegment tests handling of multi-segment ICC profiles (>64KB).
func TestParseICCProfile_MultiSegment(t *testing.T) {
	// Create a large ICC profile that requires multiple segments
	// iccMaxSegmentData is ~65519 bytes, so 100KB should create 2 segments
	largeProfileSize := 100 * 1024 // 100KB
	testICCProfile := createSampleICCProfile(largeProfileSize)

	jpegData := createMinimalJPEGWithICC(testICCProfile)

	// Parse ICC profile
	iccData, err := ParseICCProfileBytes(jpegData)
	if err != nil {
		t.Fatalf("ParseICCProfileBytes() error = %v, want nil", err)
	}

	// Verify profile matches
	if !bytes.Equal(iccData, testICCProfile) {
		t.Errorf("Multi-segment ICC profile mismatch")
		t.Logf("Expected length: %d, Got length: %d", len(testICCProfile), len(iccData))
	}

	// Verify the profile was actually split into multiple segments
	expectedSegments := (len(testICCProfile) + iccMaxSegmentData - 1) / iccMaxSegmentData
	if expectedSegments < 2 {
		t.Errorf("Test setup error: profile should require multiple segments, got %d", expectedSegments)
	}

	t.Logf("Successfully parsed %d-byte ICC profile from %d segments", len(iccData), expectedSegments)
}

// TestICCAppearsAfterEXIF tests that ICC (APP2) appears after EXIF (APP1) in output.
func TestICCAppearsAfterEXIF(t *testing.T) {
	// Create test data with both EXIF and ICC
	testEXIFContent := []byte{
		0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	testICCProfile := createSampleICCProfile(256)

	sourceJPEG := createMinimalJPEGWithBothMetadata(testEXIFContent, testICCProfile)
	img := createICCTestImage(64, 64)

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
	app2Pos := -1

	for i := 0; i < len(outputData)-1; i++ {
		if outputData[i] == 0xFF {
			if outputData[i+1] == 0xE0 && app0Pos == -1 {
				app0Pos = i
			}
			if outputData[i+1] == 0xE1 && app1Pos == -1 {
				app1Pos = i
			}
			if outputData[i+1] == 0xE2 && app2Pos == -1 {
				app2Pos = i
			}
		}
	}

	if app0Pos == -1 {
		t.Fatal("APP0 marker not found in output")
	}
	if app1Pos == -1 {
		t.Fatal("APP1 (EXIF) marker not found in output")
	}
	if app2Pos == -1 {
		t.Fatal("APP2 (ICC) marker not found in output")
	}

	// Verify order: APP0 < APP1 < APP2
	if app1Pos <= app0Pos {
		t.Errorf("APP1 (EXIF) at %d should appear after APP0 at %d", app1Pos, app0Pos)
	}
	if app2Pos <= app1Pos {
		t.Errorf("APP2 (ICC) at %d should appear after APP1 (EXIF) at %d", app2Pos, app1Pos)
	}

	t.Logf("Marker order correct: APP0 at %d, APP1 (EXIF) at %d, APP2 (ICC) at %d", app0Pos, app1Pos, app2Pos)
}

// TestEncodeWithoutICCSource tests encoding when source has no ICC (graceful handling).
func TestEncodeWithoutICCSource(t *testing.T) {
	sourceJPEG := createMinimalJPEGWithoutICC()
	img := createICCTestImage(64, 64)

	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, 75, WithSourceImageBytes(sourceJPEG))
	if err != nil {
		t.Fatalf("NewWeeksEncoderWithOptions() error = %v", err)
	}

	// Encoding should succeed even without ICC in source
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

	// Should not have APP2 ICC marker in output
	_, err = ParseICCProfileBytes(buf.Bytes())
	if err != ErrNoICC {
		t.Errorf("Expected no ICC in output when source has none, got err=%v", err)
	}
}

// TestParseICCProfile_InvalidJPEG tests error handling for invalid JPEG data.
func TestParseICCProfile_InvalidJPEG(t *testing.T) {
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
			_, err := ParseICCProfileBytes(tc.data)
			if err != ErrInvalidJPEG {
				t.Errorf("ParseICCProfileBytes(%v) error = %v, want ErrInvalidJPEG", tc.data, err)
			}
		})
	}
}
