// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements JPEG marker verification tests that compare Go encoder
// output against Java reference images byte-by-byte. The tests verify exact
// byte-level compatibility for each JPEG marker segment.
//
// JPEG Marker Structure:
//   - SOI (0xFFD8): Start of Image
//   - APP0 (0xFFE0): JFIF Application Marker
//   - COM (0xFFFE): Comment Marker
//   - DQT (0xFFDB): Define Quantization Table
//   - SOF0 (0xFFC0): Start of Frame (Baseline)
//   - DHT (0xFFC4): Define Huffman Table
//   - SOS (0xFFDA): Start of Scan
//   - EOI (0xFFD9): End of Image

package weeksjpegencoder

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// JPEG Marker Constants (local to this test file)
// =============================================================================

const (
	// JPEG marker prefix byte
	markerPrefix = 0xFF

	// JPEG marker types (single byte, without 0xFF prefix)
	testMarkerSOI  = 0xD8 // Start of Image
	testMarkerEOI  = 0xD9 // End of Image
	testMarkerAPP0 = 0xE0 // JFIF Application Marker
	testMarkerCOM  = 0xFE // Comment Marker
	testMarkerDQT  = 0xDB // Define Quantization Table
	testMarkerSOF0 = 0xC0 // Start of Frame (Baseline DCT)
	testMarkerDHT  = 0xC4 // Define Huffman Table
	testMarkerSOS  = 0xDA // Start of Scan
)

// JFIF marker identifier string
//
//nolint:unused
var jfifIdentifier = []byte{'J', 'F', 'I', 'F', 0x00}

// F5/James COM marker signature
//
//nolint:unused
const f5COMSignature = "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech."

// =============================================================================
// MarkerSegment - Represents a JPEG marker and its data
// =============================================================================

// MarkerSegment represents a JPEG marker segment.
type MarkerSegment struct {
	Type   byte   // Marker type (without 0xFF prefix)
	Offset int    // Byte offset in the file where marker starts
	Length int    // Length of the segment (including 2-byte length field, 0 for SOI/EOI)
	Data   []byte // Raw segment data (excluding 0xFF and type byte)
}

// String returns a human-readable description of the marker.
func (m *MarkerSegment) String() string {
	return fmt.Sprintf("Marker 0xFF%02X at offset %d, length %d", m.Type, m.Offset, m.Length)
}

// TypeName returns the human-readable name for the marker type.
func (m *MarkerSegment) TypeName() string {
	switch m.Type {
	case testMarkerSOI:
		return "SOI"
	case testMarkerEOI:
		return "EOI"
	case testMarkerAPP0:
		return "APP0"
	case testMarkerCOM:
		return "COM"
	case testMarkerDQT:
		return "DQT"
	case testMarkerSOF0:
		return "SOF0"
	case testMarkerDHT:
		return "DHT"
	case testMarkerSOS:
		return "SOS"
	default:
		return fmt.Sprintf("0x%02X", m.Type)
	}
}

// =============================================================================
// Marker Parsing Utility
// =============================================================================

// ParseJPEGMarkers parses a JPEG file and returns all marker segments.
// This is a comprehensive utility for locating and extracting any JPEG marker segment.
func ParseJPEGMarkers(data []byte) ([]MarkerSegment, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("data too short to be a valid JPEG")
	}

	// Verify SOI marker
	if data[0] != markerPrefix || data[1] != testMarkerSOI {
		return nil, fmt.Errorf("not a valid JPEG: missing SOI marker at offset 0, got 0x%02X%02X", data[0], data[1])
	}

	var markers []MarkerSegment
	offset := 0

	for offset < len(data) {
		// Look for marker prefix
		if data[offset] != markerPrefix {
			offset++
			continue
		}

		// Skip padding bytes (multiple 0xFF)
		for offset+1 < len(data) && data[offset+1] == markerPrefix {
			offset++
		}

		if offset+1 >= len(data) {
			break
		}

		markerType := data[offset+1]

		// Skip stuffed bytes (0xFF00)
		if markerType == 0x00 {
			offset += 2
			continue
		}

		marker := MarkerSegment{
			Type:   markerType,
			Offset: offset,
		}

		// SOI and EOI have no length field
		if markerType == testMarkerSOI || markerType == testMarkerEOI {
			marker.Length = 0
			marker.Data = nil
			markers = append(markers, marker)
			offset += 2
			continue
		}

		// RST markers (0xD0-0xD7) have no length field
		if markerType >= 0xD0 && markerType <= 0xD7 {
			marker.Length = 0
			marker.Data = nil
			markers = append(markers, marker)
			offset += 2
			continue
		}

		// Read length field (big-endian, includes itself)
		if offset+4 > len(data) {
			return markers, fmt.Errorf("truncated marker at offset %d", offset)
		}

		length := int(data[offset+2])<<8 | int(data[offset+3])
		if length < 2 {
			return markers, fmt.Errorf("invalid marker length %d at offset %d", length, offset)
		}

		if offset+2+length > len(data) {
			return markers, fmt.Errorf("marker data exceeds file boundary at offset %d", offset)
		}

		marker.Length = length
		marker.Data = data[offset+2 : offset+2+length] // Includes length field in data

		markers = append(markers, marker)

		// SOS marker is followed by entropy-coded data until next marker
		if markerType == testMarkerSOS {
			// Skip to find next marker (EOI typically)
			offset += 2 + length
			// Scan for next marker in entropy-coded data
			for offset < len(data)-1 {
				if data[offset] == markerPrefix && data[offset+1] != 0x00 {
					// Found next marker, but handle RST markers
					if data[offset+1] >= 0xD0 && data[offset+1] <= 0xD7 {
						offset += 2
						continue
					}
					break
				}
				offset++
			}
		} else {
			offset += 2 + length
		}
	}

	return markers, nil
}

// FindMarker finds the first marker of the specified type.
// Returns nil if not found.
func FindMarker(markers []MarkerSegment, markerType byte) *MarkerSegment {
	for i := range markers {
		if markers[i].Type == markerType {
			return &markers[i]
		}
	}
	return nil
}

// FindAllMarkers finds all markers of the specified type.
func FindAllMarkers(markers []MarkerSegment, markerType byte) []MarkerSegment {
	var result []MarkerSegment
	for _, m := range markers {
		if m.Type == markerType {
			result = append(result, m)
		}
	}
	return result
}

// =============================================================================
// Byte Comparison Utilities
// =============================================================================

// ByteDifference represents a single byte difference between two files.
type ByteDifference struct {
	Offset   int
	Expected byte
	Actual   byte
	Context  string // Marker context
}

// CompareBytes compares two byte slices and returns all differences.
func CompareBytes(expected, actual []byte, context string) []ByteDifference {
	var diffs []ByteDifference

	maxLen := len(expected)
	if len(actual) > maxLen {
		maxLen = len(actual)
	}

	for i := 0; i < maxLen; i++ {
		var exp, act byte
		if i < len(expected) {
			exp = expected[i]
		}
		if i < len(actual) {
			act = actual[i]
		}

		if exp != act || i >= len(expected) || i >= len(actual) {
			diffs = append(diffs, ByteDifference{
				Offset:   i,
				Expected: exp,
				Actual:   act,
				Context:  context,
			})
		}
	}

	return diffs
}

// CompareMarkerSegments compares two marker segments byte-by-byte.
func CompareMarkerSegments(expected, actual *MarkerSegment) []ByteDifference {
	if expected == nil || actual == nil {
		return []ByteDifference{{
			Offset:  0,
			Context: fmt.Sprintf("marker missing: expected=%v, actual=%v", expected != nil, actual != nil),
		}}
	}

	context := expected.TypeName()
	return CompareBytes(expected.Data, actual.Data, context)
}

// ReportDifferences creates a detailed report of byte differences.
func ReportDifferences(t *testing.T, diffs []ByteDifference, markerName string) {
	t.Helper()
	if len(diffs) == 0 {
		return
	}

	t.Errorf("%s: found %d byte differences:", markerName, len(diffs))
	for i, diff := range diffs {
		if i >= 10 {
			t.Errorf("  ... and %d more differences", len(diffs)-10)
			break
		}
		t.Errorf("  offset %d: expected 0x%02X, got 0x%02X (%s)",
			diff.Offset, diff.Expected, diff.Actual, diff.Context)
	}
}

// =============================================================================
// Reference Image Loading
// =============================================================================

// referenceImageDir returns the path to the reference images directory.
func referenceImageDir() string {
	return filepath.Join("testdata", "reference", "4_2_0")
}

// loadReferenceImage loads a Java reference image by name.
func loadReferenceImage(pattern string, width, height, quality int) ([]byte, error) {
	filename := fmt.Sprintf("%s_%dx%d_q%02d_420.jpg", pattern, width, height, quality)
	path := filepath.Join(referenceImageDir(), filename)
	return os.ReadFile(path)
}

// generateGoImage generates a JPEG using the Go encoder with the same parameters.
// Uses James-compatible mode for byte-exact comparison with Java reference images.
func generateGoImage(patternType PatternType, width, height, quality int) ([]byte, error) {
	img := GeneratePattern(patternType, width, height)
	return WeeksEncodeToBytes(img, quality)
}

// generateGoImageStandard generates a JPEG using standard mode.
// Use this for tests that need Go-decodable output or standard DHT format.
func generateGoImageStandard(patternType PatternType, width, height, quality int) ([]byte, error) {
	img := GeneratePattern(patternType, width, height)
	return WeeksEncodeToBytesStandard(img, quality)
}

// =============================================================================
// Test Helper Functions
// =============================================================================

// testMarkerComparison is a helper for comparing markers between Java and Go output.
func testMarkerComparison(t *testing.T, pattern PatternType, width, height, quality int, markerType byte) {
	t.Helper()

	// Load Java reference
	javaData, err := loadReferenceImage(pattern.PatternName(), width, height, quality)
	if err != nil {
		t.Skipf("Reference image not found: %v", err)
		return
	}

	// Generate Go output
	goData, err := generateGoImage(pattern, width, height, quality)
	if err != nil {
		t.Fatalf("Failed to generate Go image: %v", err)
	}

	// Parse markers
	javaMarkers, err := ParseJPEGMarkers(javaData)
	if err != nil {
		t.Fatalf("Failed to parse Java reference: %v", err)
	}

	goMarkers, err := ParseJPEGMarkers(goData)
	if err != nil {
		t.Fatalf("Failed to parse Go output: %v", err)
	}

	// Find and compare specific marker
	javaMarker := FindMarker(javaMarkers, markerType)
	goMarker := FindMarker(goMarkers, markerType)

	if javaMarker == nil {
		t.Fatalf("Marker 0x%02X not found in Java reference", markerType)
	}
	if goMarker == nil {
		t.Fatalf("Marker 0x%02X not found in Go output", markerType)
	}

	diffs := CompareMarkerSegments(javaMarker, goMarker)
	if len(diffs) > 0 {
		ReportDifferences(t, diffs, javaMarker.TypeName())
	}
}

// =============================================================================
// Task 4.2: SOI Marker Verification Test
// =============================================================================

// TestSOIMarkerByteComparison verifies the SOI marker (0xFFD8) matches exactly.
func TestSOIMarkerByteComparison(t *testing.T) {
	testCases := []struct {
		pattern PatternType
		width   int
		height  int
		quality int
	}{
		{PatternSolid, 64, 64, 75},
		{PatternHorizontalGradient, 64, 64, 75},
		{PatternCheckerboard, 64, 64, 75},
		{PatternSolid, 8, 8, 50},
		{PatternQuadrant, 256, 256, 90},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%s_%dx%d_q%d", tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
		t.Run(name, func(t *testing.T) {
			// Load Java reference
			javaData, err := loadReferenceImage(tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
			if err != nil {
				t.Skipf("Reference image not found: %v", err)
				return
			}

			// Generate Go output
			goData, err := generateGoImage(tc.pattern, tc.width, tc.height, tc.quality)
			if err != nil {
				t.Fatalf("Failed to generate Go image: %v", err)
			}

			// Verify SOI marker (first 2 bytes)
			if len(javaData) < 2 || len(goData) < 2 {
				t.Fatal("Data too short")
			}

			// Check Java reference has correct SOI
			if javaData[0] != 0xFF || javaData[1] != 0xD8 {
				t.Errorf("Java reference missing SOI: got 0x%02X%02X, want 0xFFD8", javaData[0], javaData[1])
			}

			// Check Go output has correct SOI
			if goData[0] != 0xFF || goData[1] != 0xD8 {
				t.Errorf("Go output missing SOI: got 0x%02X%02X, want 0xFFD8", goData[0], goData[1])
			}

			// Compare byte-by-byte
			if javaData[0] != goData[0] || javaData[1] != goData[1] {
				t.Errorf("SOI marker mismatch: Java=0x%02X%02X, Go=0x%02X%02X",
					javaData[0], javaData[1], goData[0], goData[1])
			}
		})
	}
}

// =============================================================================
// Task 4.3: APP0/JFIF Marker Verification Test
// =============================================================================

// TestAPP0JFIFMarkerVerification verifies the APP0/JFIF marker structure matches exactly.
func TestAPP0JFIFMarkerVerification(t *testing.T) {
	testCases := []struct {
		pattern PatternType
		width   int
		height  int
		quality int
	}{
		{PatternSolid, 64, 64, 75},
		{PatternHorizontalGradient, 64, 64, 50},
		{PatternCheckerboard, 256, 256, 90},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%s_%dx%d_q%d", tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
		t.Run(name, func(t *testing.T) {
			testMarkerComparison(t, tc.pattern, tc.width, tc.height, tc.quality, testMarkerAPP0)
		})
	}
}

// TestAPP0JFIFStructure verifies the APP0 marker has correct JFIF structure.
func TestAPP0JFIFStructure(t *testing.T) {
	// Generate a test image
	goData, err := generateGoImage(PatternSolid, 64, 64, 75)
	if err != nil {
		t.Fatalf("Failed to generate Go image: %v", err)
	}

	markers, err := ParseJPEGMarkers(goData)
	if err != nil {
		t.Fatalf("Failed to parse markers: %v", err)
	}

	app0 := FindMarker(markers, testMarkerAPP0)
	if app0 == nil {
		t.Fatal("APP0 marker not found")
	}

	// APP0 structure (after length field):
	// - JFIF identifier: "JFIF\0" (5 bytes)
	// - Version: major.minor (2 bytes)
	// - Density units (1 byte)
	// - X density (2 bytes)
	// - Y density (2 bytes)
	// - Thumbnail dimensions (2 bytes)

	if app0.Length < 16 {
		t.Fatalf("APP0 segment too short: %d bytes", app0.Length)
	}

	// Skip 2-byte length field in data
	data := app0.Data[2:]

	// Check JFIF identifier
	if !bytes.Equal(data[0:5], jfifIdentifier) {
		t.Errorf("JFIF identifier mismatch: got %v, want %v", data[0:5], jfifIdentifier)
	}

	// Check version (should be 1.1 or 1.2)
	majorVersion := data[5]
	minorVersion := data[6]
	if majorVersion != 1 {
		t.Errorf("JFIF major version: got %d, want 1", majorVersion)
	}
	if minorVersion > 2 {
		t.Errorf("JFIF minor version: got %d, want 0-2", minorVersion)
	}
}

// =============================================================================
// Task 4.4: COM Marker Byte-Exact Verification Test
// =============================================================================

// TestCOMMarkerByteExact verifies the COM marker matches exactly.
func TestCOMMarkerByteExact(t *testing.T) {
	testCases := []struct {
		pattern PatternType
		width   int
		height  int
		quality int
	}{
		{PatternSolid, 64, 64, 75},
		{PatternHorizontalGradient, 64, 64, 50},
		{PatternVerticalGradient, 64, 64, 25},
		{PatternDiagonalGradient, 64, 64, 90},
		{PatternCheckerboard, 64, 64, 100},
		{PatternQuadrant, 64, 64, 1},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%s_%dx%d_q%d", tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
		t.Run(name, func(t *testing.T) {
			testMarkerComparison(t, tc.pattern, tc.width, tc.height, tc.quality, testMarkerCOM)
		})
	}
}

// TestCOMMarkerSignature verifies the COM marker contains the exact F5 signature.
func TestCOMMarkerSignature(t *testing.T) {
	goData, err := generateGoImage(PatternSolid, 64, 64, 75)
	if err != nil {
		t.Fatalf("Failed to generate Go image: %v", err)
	}

	markers, err := ParseJPEGMarkers(goData)
	if err != nil {
		t.Fatalf("Failed to parse markers: %v", err)
	}

	com := FindMarker(markers, testMarkerCOM)
	if com == nil {
		t.Fatal("COM marker not found")
	}

	// Extract comment text (skip 2-byte length field)
	if len(com.Data) < 2 {
		t.Fatal("COM segment too short")
	}
	comment := string(com.Data[2:])

	expectedSignature := f5COMSignature
	if comment != expectedSignature {
		t.Errorf("COM signature mismatch:\n  got:    %q\n  want:   %q", comment, expectedSignature)
	}
}

// =============================================================================
// Task 4.5: DQT Marker Verification Test
// =============================================================================

// TestDQTMarkerByteComparison verifies the DQT markers match exactly (both tables, zigzag order).
func TestDQTMarkerByteComparison(t *testing.T) {
	testCases := []struct {
		pattern PatternType
		width   int
		height  int
		quality int
	}{
		{PatternSolid, 64, 64, 75},
		{PatternSolid, 64, 64, 50},
		{PatternSolid, 64, 64, 25},
		{PatternSolid, 64, 64, 90},
		{PatternSolid, 64, 64, 100},
		{PatternHorizontalGradient, 64, 64, 1},
		{PatternHorizontalGradient, 64, 64, 10},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%s_%dx%d_q%d", tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
		t.Run(name, func(t *testing.T) {
			testMarkerComparison(t, tc.pattern, tc.width, tc.height, tc.quality, testMarkerDQT)
		})
	}
}

// TestDQTTableStructure verifies the DQT marker contains both luminance and chrominance tables.
func TestDQTTableStructure(t *testing.T) {
	// Test multiple quality levels to ensure scaling works correctly
	qualityLevels := []int{1, 10, 25, 50, 75, 90, 95, 100}

	for _, quality := range qualityLevels {
		t.Run(fmt.Sprintf("Q%d", quality), func(t *testing.T) {
			goData, err := generateGoImage(PatternSolid, 64, 64, quality)
			if err != nil {
				t.Fatalf("Failed to generate Go image: %v", err)
			}

			markers, err := ParseJPEGMarkers(goData)
			if err != nil {
				t.Fatalf("Failed to parse markers: %v", err)
			}

			dqts := FindAllMarkers(markers, testMarkerDQT)
			if len(dqts) == 0 {
				t.Fatal("No DQT markers found")
			}

			// DQT can contain multiple tables in one segment or separate segments
			// Each table is 65 bytes: 1 byte (precision/id) + 64 values
			totalTableBytes := 0
			for _, dqt := range dqts {
				// Subtract 2 for length field
				totalTableBytes += dqt.Length - 2
			}

			// Should have at least 2 tables (luminance + chrominance) = 130 bytes minimum
			expectedMinBytes := 2 * 65 // Two tables
			if totalTableBytes < expectedMinBytes {
				t.Errorf("DQT total size %d bytes, expected at least %d bytes for 2 tables",
					totalTableBytes, expectedMinBytes)
			}
		})
	}
}

// TestDQTQuantizationValues verifies specific quantization values for known quality levels.
func TestDQTQuantizationValues(t *testing.T) {
	// Load Java reference and compare quantization tables
	javaData, err := loadReferenceImage("solid", 64, 64, 75)
	if err != nil {
		t.Skipf("Reference image not found: %v", err)
		return
	}

	goData, err := generateGoImage(PatternSolid, 64, 64, 75)
	if err != nil {
		t.Fatalf("Failed to generate Go image: %v", err)
	}

	javaMarkers, _ := ParseJPEGMarkers(javaData)
	goMarkers, _ := ParseJPEGMarkers(goData)

	javaDQT := FindMarker(javaMarkers, testMarkerDQT)
	goDQT := FindMarker(goMarkers, testMarkerDQT)

	if javaDQT == nil || goDQT == nil {
		t.Fatal("DQT marker not found")
	}

	// Compare raw DQT segment bytes
	diffs := CompareMarkerSegments(javaDQT, goDQT)
	if len(diffs) > 0 {
		t.Errorf("DQT segment differs from Java reference:")
		for i, diff := range diffs {
			if i >= 20 {
				t.Errorf("  ... and %d more differences", len(diffs)-20)
				break
			}
			t.Errorf("  byte %d: expected 0x%02X, got 0x%02X", diff.Offset, diff.Expected, diff.Actual)
		}
	}
}

// =============================================================================
// Task 4.6: SOF0 Marker Verification Test
// =============================================================================

// TestSOF0MarkerByteComparison verifies the SOF0 marker matches exactly.
func TestSOF0MarkerByteComparison(t *testing.T) {
	testCases := []struct {
		pattern PatternType
		width   int
		height  int
		quality int
	}{
		{PatternSolid, 64, 64, 75},
		{PatternSolid, 8, 8, 50},
		{PatternSolid, 256, 256, 90},
		{PatternSolid, 33, 33, 75},
		{PatternSolid, 100, 75, 75},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%s_%dx%d_q%d", tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
		t.Run(name, func(t *testing.T) {
			testMarkerComparison(t, tc.pattern, tc.width, tc.height, tc.quality, testMarkerSOF0)
		})
	}
}

// TestSOF0FrameStructure verifies the SOF0 marker contains correct frame parameters.
func TestSOF0FrameStructure(t *testing.T) {
	testCases := []struct {
		width      int
		height     int
		expectedH  int // horizontal sampling factor for Y
		expectedV  int // vertical sampling factor for Y
		components int // number of components
	}{
		{64, 64, 2, 2, 3},   // 4:2:0 subsampling
		{8, 8, 2, 2, 3},     // Single MCU
		{256, 256, 2, 2, 3}, // Larger image
		{33, 33, 2, 2, 3},   // Non-multiple of 8
		{100, 75, 2, 2, 3},  // Non-square
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%dx%d", tc.width, tc.height)
		t.Run(name, func(t *testing.T) {
			goData, err := generateGoImage(PatternSolid, tc.width, tc.height, 75)
			if err != nil {
				t.Fatalf("Failed to generate Go image: %v", err)
			}

			markers, err := ParseJPEGMarkers(goData)
			if err != nil {
				t.Fatalf("Failed to parse markers: %v", err)
			}

			sof0 := FindMarker(markers, testMarkerSOF0)
			if sof0 == nil {
				t.Fatal("SOF0 marker not found")
			}

			// SOF0 structure (after 2-byte length):
			// - Precision: 1 byte (should be 8)
			// - Height: 2 bytes (big-endian)
			// - Width: 2 bytes (big-endian)
			// - Number of components: 1 byte
			// - Component specs: 3 bytes each (ID, sampling factors, quant table)

			if sof0.Length < 11 {
				t.Fatalf("SOF0 segment too short: %d bytes", sof0.Length)
			}

			data := sof0.Data[2:] // Skip length field

			precision := data[0]
			height := int(data[1])<<8 | int(data[2])
			width := int(data[3])<<8 | int(data[4])
			numComponents := int(data[5])

			if precision != 8 {
				t.Errorf("Precision: got %d, want 8", precision)
			}
			if height != tc.height {
				t.Errorf("Height: got %d, want %d", height, tc.height)
			}
			if width != tc.width {
				t.Errorf("Width: got %d, want %d", width, tc.width)
			}
			if numComponents != tc.components {
				t.Errorf("Number of components: got %d, want %d", numComponents, tc.components)
			}

			// Check Y component sampling factors (first component)
			if len(data) >= 8 {
				yID := data[6]
				ySamplingFactors := data[7]
				yH := int(ySamplingFactors >> 4)
				yV := int(ySamplingFactors & 0x0F)

				if yID != 1 {
					t.Errorf("Y component ID: got %d, want 1", yID)
				}
				if yH != tc.expectedH {
					t.Errorf("Y horizontal sampling: got %d, want %d", yH, tc.expectedH)
				}
				if yV != tc.expectedV {
					t.Errorf("Y vertical sampling: got %d, want %d", yV, tc.expectedV)
				}
			}
		})
	}
}

// =============================================================================
// Task 4.7: DHT Marker Verification Test
// =============================================================================

// TestDHTMarkerByteComparison verifies the DHT markers match exactly (all 4 tables).
func TestDHTMarkerByteComparison(t *testing.T) {
	testCases := []struct {
		pattern PatternType
		width   int
		height  int
		quality int
	}{
		{PatternSolid, 64, 64, 75},
		{PatternHorizontalGradient, 64, 64, 50},
		{PatternCheckerboard, 64, 64, 90},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%s_%dx%d_q%d", tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
		t.Run(name, func(t *testing.T) {
			testMarkerComparison(t, tc.pattern, tc.width, tc.height, tc.quality, testMarkerDHT)
		})
	}
}

// TestDHTTableCount verifies that all 4 Huffman tables are present.
func TestDHTTableCount(t *testing.T) {
	goData, err := generateGoImage(PatternSolid, 64, 64, 75)
	if err != nil {
		t.Fatalf("Failed to generate Go image: %v", err)
	}

	markers, err := ParseJPEGMarkers(goData)
	if err != nil {
		t.Fatalf("Failed to parse markers: %v", err)
	}

	dhts := FindAllMarkers(markers, testMarkerDHT)
	if len(dhts) == 0 {
		t.Fatal("No DHT markers found")
	}

	// Count tables in all DHT segments
	// Each table starts with 1-byte class/ID, then 16 bytes for bit counts, then values
	tableCount := 0
	for _, dht := range dhts {
		data := dht.Data[2:] // Skip length field
		offset := 0
		for offset < len(data) {
			if offset+17 > len(data) {
				break
			}
			// Read 16 bit counts
			numSymbols := 0
			for i := 0; i < 16; i++ {
				numSymbols += int(data[offset+1+i])
			}
			tableCount++
			offset += 17 + numSymbols
		}
	}

	// Should have 4 tables: DC lum, DC chrom, AC lum, AC chrom
	if tableCount != 4 {
		t.Errorf("Expected 4 Huffman tables, found %d", tableCount)
	}
}

// TestDHTTableStructure verifies the DHT marker contains standard Huffman tables.
// Uses standard mode to verify proper class/ID bytes (James mode writes all 0x00 headers).
func TestDHTTableStructure(t *testing.T) {
	goData, err := generateGoImageStandard(PatternSolid, 64, 64, 75)
	if err != nil {
		t.Fatalf("Failed to generate Go image: %v", err)
	}

	markers, err := ParseJPEGMarkers(goData)
	if err != nil {
		t.Fatalf("Failed to parse markers: %v", err)
	}

	dhts := FindAllMarkers(markers, testMarkerDHT)
	if len(dhts) == 0 {
		t.Fatal("No DHT markers found")
	}

	// Verify table class/ID combinations
	expectedTables := map[byte]bool{
		0x00: false, // DC luminance (class=0, ID=0)
		0x01: false, // DC chrominance (class=0, ID=1)
		0x10: false, // AC luminance (class=1, ID=0)
		0x11: false, // AC chrominance (class=1, ID=1)
	}

	for _, dht := range dhts {
		data := dht.Data[2:] // Skip length field
		offset := 0
		for offset < len(data) {
			if offset >= len(data) {
				break
			}
			classID := data[offset]
			if _, exists := expectedTables[classID]; exists {
				expectedTables[classID] = true
			}
			// Skip to next table
			if offset+17 > len(data) {
				break
			}
			numSymbols := 0
			for i := 0; i < 16; i++ {
				numSymbols += int(data[offset+1+i])
			}
			offset += 17 + numSymbols
		}
	}

	for classID, found := range expectedTables {
		if !found {
			class := classID >> 4
			id := classID & 0x0F
			tableType := "DC"
			if class == 1 {
				tableType = "AC"
			}
			tableName := "luminance"
			if id == 1 {
				tableName = "chrominance"
			}
			t.Errorf("Missing Huffman table: %s %s (class=%d, ID=%d)", tableType, tableName, class, id)
		}
	}
}

// =============================================================================
// Task 4.8: SOS Marker Verification Test
// =============================================================================

// TestSOSMarkerByteComparison verifies the SOS marker matches exactly.
func TestSOSMarkerByteComparison(t *testing.T) {
	testCases := []struct {
		pattern PatternType
		width   int
		height  int
		quality int
	}{
		{PatternSolid, 64, 64, 75},
		{PatternHorizontalGradient, 64, 64, 50},
		{PatternCheckerboard, 64, 64, 90},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%s_%dx%d_q%d", tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
		t.Run(name, func(t *testing.T) {
			testMarkerComparison(t, tc.pattern, tc.width, tc.height, tc.quality, testMarkerSOS)
		})
	}
}

// TestSOSScanStructure verifies the SOS marker contains correct scan parameters.
func TestSOSScanStructure(t *testing.T) {
	goData, err := generateGoImage(PatternSolid, 64, 64, 75)
	if err != nil {
		t.Fatalf("Failed to generate Go image: %v", err)
	}

	markers, err := ParseJPEGMarkers(goData)
	if err != nil {
		t.Fatalf("Failed to parse markers: %v", err)
	}

	sos := FindMarker(markers, testMarkerSOS)
	if sos == nil {
		t.Fatal("SOS marker not found")
	}

	// SOS structure (after 2-byte length):
	// - Number of components in scan: 1 byte
	// - For each component:
	//   - Component ID: 1 byte
	//   - Huffman table selectors: 1 byte (DC in high nibble, AC in low nibble)
	// - Spectral selection start (Ss): 1 byte (should be 0 for baseline)
	// - Spectral selection end (Se): 1 byte (should be 63 for baseline)
	// - Successive approximation (Ah, Al): 1 byte (should be 0 for baseline)

	if sos.Length < 8 {
		t.Fatalf("SOS segment too short: %d bytes", sos.Length)
	}

	data := sos.Data[2:] // Skip length field

	numComponents := int(data[0])
	if numComponents != 3 {
		t.Errorf("Number of components in scan: got %d, want 3", numComponents)
	}

	// Check component selectors
	expectedComponents := []struct {
		id      byte
		dcTable byte
		acTable byte
	}{
		{1, 0, 0}, // Y: DC table 0, AC table 0
		{2, 1, 1}, // Cb: DC table 1, AC table 1
		{3, 1, 1}, // Cr: DC table 1, AC table 1
	}

	for i, expected := range expectedComponents {
		if i*2+2 >= len(data) {
			break
		}
		compID := data[1+i*2]
		tables := data[1+i*2+1]
		dcTable := tables >> 4
		acTable := tables & 0x0F

		if compID != expected.id {
			t.Errorf("Component %d ID: got %d, want %d", i, compID, expected.id)
		}
		if dcTable != expected.dcTable {
			t.Errorf("Component %d DC table: got %d, want %d", i, dcTable, expected.dcTable)
		}
		if acTable != expected.acTable {
			t.Errorf("Component %d AC table: got %d, want %d", i, acTable, expected.acTable)
		}
	}

	// Check baseline DCT parameters
	ssOffset := 1 + numComponents*2
	if ssOffset+3 <= len(data) {
		ss := data[ssOffset]   // Spectral selection start
		se := data[ssOffset+1] // Spectral selection end
		ah := data[ssOffset+2] // Successive approximation

		if ss != 0 {
			t.Errorf("Spectral selection start (Ss): got %d, want 0", ss)
		}
		if se != 63 {
			t.Errorf("Spectral selection end (Se): got %d, want 63", se)
		}
		if ah != 0 {
			t.Errorf("Successive approximation: got %d, want 0", ah)
		}
	}
}

// =============================================================================
// Task 4.9: EOI Marker Verification Test
// =============================================================================

// TestEOIMarkerByteComparison verifies the EOI marker (0xFFD9) matches exactly.
func TestEOIMarkerByteComparison(t *testing.T) {
	testCases := []struct {
		pattern PatternType
		width   int
		height  int
		quality int
	}{
		{PatternSolid, 64, 64, 75},
		{PatternHorizontalGradient, 64, 64, 75},
		{PatternCheckerboard, 64, 64, 75},
		{PatternSolid, 8, 8, 50},
		{PatternQuadrant, 256, 256, 90},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%s_%dx%d_q%d", tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
		t.Run(name, func(t *testing.T) {
			// Load Java reference
			javaData, err := loadReferenceImage(tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
			if err != nil {
				t.Skipf("Reference image not found: %v", err)
				return
			}

			// Generate Go output
			goData, err := generateGoImage(tc.pattern, tc.width, tc.height, tc.quality)
			if err != nil {
				t.Fatalf("Failed to generate Go image: %v", err)
			}

			// Verify EOI marker (last 2 bytes)
			if len(javaData) < 2 || len(goData) < 2 {
				t.Fatal("Data too short")
			}

			javaEOI := javaData[len(javaData)-2:]
			goEOI := goData[len(goData)-2:]

			// Check Java reference has correct EOI
			if javaEOI[0] != 0xFF || javaEOI[1] != 0xD9 {
				t.Errorf("Java reference missing EOI: got 0x%02X%02X, want 0xFFD9", javaEOI[0], javaEOI[1])
			}

			// Check Go output has correct EOI
			if goEOI[0] != 0xFF || goEOI[1] != 0xD9 {
				t.Errorf("Go output missing EOI: got 0x%02X%02X, want 0xFFD9", goEOI[0], goEOI[1])
			}

			// Compare byte-by-byte
			if javaEOI[0] != goEOI[0] || javaEOI[1] != goEOI[1] {
				t.Errorf("EOI marker mismatch: Java=0x%02X%02X, Go=0x%02X%02X",
					javaEOI[0], javaEOI[1], goEOI[0], goEOI[1])
			}
		})
	}
}

// =============================================================================
// Task 4.10: Comprehensive Marker Parsing Utility Tests
// =============================================================================

// TestMarkerParsingUtility tests the marker parsing utility comprehensively.
func TestMarkerParsingUtility(t *testing.T) {
	// Generate a test image
	goData, err := generateGoImage(PatternSolid, 64, 64, 75)
	if err != nil {
		t.Fatalf("Failed to generate Go image: %v", err)
	}

	markers, err := ParseJPEGMarkers(goData)
	if err != nil {
		t.Fatalf("ParseJPEGMarkers failed: %v", err)
	}

	// Verify all expected markers are found
	expectedMarkers := []byte{
		testMarkerSOI,
		testMarkerAPP0,
		testMarkerCOM,
		testMarkerDQT,
		testMarkerSOF0,
		testMarkerDHT,
		testMarkerSOS,
		testMarkerEOI,
	}

	for _, expected := range expectedMarkers {
		found := FindMarker(markers, expected)
		if found == nil {
			t.Errorf("Expected marker 0x%02X not found", expected)
		}
	}

	// Verify marker order (SOI first, EOI last)
	if len(markers) < 2 {
		t.Fatal("Too few markers found")
	}

	if markers[0].Type != testMarkerSOI {
		t.Errorf("First marker should be SOI, got 0x%02X", markers[0].Type)
	}

	if markers[len(markers)-1].Type != testMarkerEOI {
		t.Errorf("Last marker should be EOI, got 0x%02X", markers[len(markers)-1].Type)
	}
}

// TestMarkerParsingWithReferenceImage tests parsing a Java reference image.
func TestMarkerParsingWithReferenceImage(t *testing.T) {
	javaData, err := loadReferenceImage("solid", 64, 64, 75)
	if err != nil {
		t.Skipf("Reference image not found: %v", err)
		return
	}

	markers, err := ParseJPEGMarkers(javaData)
	if err != nil {
		t.Fatalf("ParseJPEGMarkers failed on Java reference: %v", err)
	}

	// Log all found markers
	t.Logf("Found %d markers in Java reference:", len(markers))
	for _, m := range markers {
		t.Logf("  %s", m.String())
	}

	// Verify essential markers exist
	if FindMarker(markers, testMarkerSOI) == nil {
		t.Error("SOI marker not found in Java reference")
	}
	if FindMarker(markers, testMarkerEOI) == nil {
		t.Error("EOI marker not found in Java reference")
	}
}

// TestMarkerParsingInvalidInput tests error handling for invalid input.
func TestMarkerParsingInvalidInput(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"too short", []byte{0xFF}},
		{"not jpeg", []byte{0x00, 0x00}},
		{"wrong header", []byte{0xFF, 0x00}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseJPEGMarkers(tc.data)
			if err == nil {
				t.Error("Expected error for invalid input")
			}
		})
	}
}

// TestMarkerOffsetsAreCorrect verifies that marker offsets are accurate.
func TestMarkerOffsetsAreCorrect(t *testing.T) {
	goData, err := generateGoImage(PatternSolid, 64, 64, 75)
	if err != nil {
		t.Fatalf("Failed to generate Go image: %v", err)
	}

	markers, err := ParseJPEGMarkers(goData)
	if err != nil {
		t.Fatalf("ParseJPEGMarkers failed: %v", err)
	}

	for _, m := range markers {
		if m.Offset < 0 || m.Offset >= len(goData) {
			t.Errorf("Marker %s has invalid offset %d", m.TypeName(), m.Offset)
			continue
		}

		// Verify marker bytes at offset
		if goData[m.Offset] != markerPrefix {
			t.Errorf("Marker %s at offset %d: expected 0xFF, got 0x%02X",
				m.TypeName(), m.Offset, goData[m.Offset])
		}
		if m.Offset+1 < len(goData) && goData[m.Offset+1] != m.Type {
			t.Errorf("Marker %s at offset %d: expected type 0x%02X, got 0x%02X",
				m.TypeName(), m.Offset, m.Type, goData[m.Offset+1])
		}
	}
}

// =============================================================================
// Cross-Pattern Marker Verification Tests
// =============================================================================

// TestAllPatternsHaveConsistentMarkerStructure verifies all patterns produce consistent markers.
func TestAllPatternsHaveConsistentMarkerStructure(t *testing.T) {
	patterns := AllPatternTypes()
	quality := 75
	width, height := 64, 64

	for _, pattern := range patterns {
		t.Run(pattern.PatternName(), func(t *testing.T) {
			goData, err := generateGoImage(pattern, width, height, quality)
			if err != nil {
				t.Fatalf("Failed to generate image: %v", err)
			}

			markers, err := ParseJPEGMarkers(goData)
			if err != nil {
				t.Fatalf("Failed to parse markers: %v", err)
			}

			// All images should have the same marker structure
			expectedOrder := []byte{
				testMarkerSOI,
				testMarkerAPP0,
				testMarkerCOM,
				testMarkerDQT,
				testMarkerSOF0,
				testMarkerDHT,
				testMarkerSOS,
				testMarkerEOI,
			}

			markerIndex := 0
			for _, expected := range expectedOrder {
				found := false
				for markerIndex < len(markers) {
					if markers[markerIndex].Type == expected {
						found = true
						markerIndex++
						break
					}
					markerIndex++
				}
				if !found {
					t.Errorf("Marker 0x%02X not found in expected order", expected)
				}
			}
		})
	}
}

// =============================================================================
// Comprehensive Full-File Marker Comparison
// =============================================================================

// TestFullMarkerSequenceComparison compares all marker segments between Java and Go.
func TestFullMarkerSequenceComparison(t *testing.T) {
	testCases := []struct {
		pattern PatternType
		width   int
		height  int
		quality int
	}{
		{PatternSolid, 64, 64, 75},
		{PatternHorizontalGradient, 64, 64, 50},
		{PatternCheckerboard, 64, 64, 90},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%s_%dx%d_q%d", tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
		t.Run(name, func(t *testing.T) {
			javaData, err := loadReferenceImage(tc.pattern.PatternName(), tc.width, tc.height, tc.quality)
			if err != nil {
				t.Skipf("Reference image not found: %v", err)
				return
			}

			goData, err := generateGoImage(tc.pattern, tc.width, tc.height, tc.quality)
			if err != nil {
				t.Fatalf("Failed to generate Go image: %v", err)
			}

			javaMarkers, _ := ParseJPEGMarkers(javaData)
			goMarkers, _ := ParseJPEGMarkers(goData)

			// Compare each marker type
			markerTypes := []byte{testMarkerAPP0, testMarkerCOM, testMarkerDQT, testMarkerSOF0, testMarkerDHT, testMarkerSOS}

			for _, markerType := range markerTypes {
				javaMarker := FindMarker(javaMarkers, markerType)
				goMarker := FindMarker(goMarkers, markerType)

				if javaMarker == nil && goMarker == nil {
					continue
				}

				if javaMarker == nil || goMarker == nil {
					t.Errorf("Marker 0x%02X: present in one but not both (Java=%v, Go=%v)",
						markerType, javaMarker != nil, goMarker != nil)
					continue
				}

				diffs := CompareMarkerSegments(javaMarker, goMarker)
				if len(diffs) > 0 {
					t.Errorf("Marker %s has %d byte differences", javaMarker.TypeName(), len(diffs))
					for i, diff := range diffs {
						if i >= 5 {
							t.Logf("  ... and %d more", len(diffs)-5)
							break
						}
						t.Logf("  byte %d: expected 0x%02X, got 0x%02X", diff.Offset, diff.Expected, diff.Actual)
					}
				}
			}
		})
	}
}

// =============================================================================
// Utility Exports for Other Test Files
// =============================================================================

// ExportedParseJPEGMarkers is an exported wrapper for use in other test files.
//
//goland:noinspection GoUnusedExportedFunction
func ExportedParseJPEGMarkers(data []byte) ([]MarkerSegment, error) {
	return ParseJPEGMarkers(data)
}

// ExportedFindMarker is an exported wrapper for use in other test files.
//
//goland:noinspection GoUnusedExportedFunction
func ExportedFindMarker(markers []MarkerSegment, markerType byte) *MarkerSegment {
	return FindMarker(markers, markerType)
}

// GetEntropyCodedData extracts the entropy-coded data segment (between SOS and EOI).
//
//nolint:unused
//goland:noinspection GoUnusedExportedFunction
func GetEntropyCodedData(data []byte) ([]byte, error) {
	markers, err := ParseJPEGMarkers(data)
	if err != nil {
		return nil, err
	}

	sos := FindMarker(markers, testMarkerSOS)
	if sos == nil {
		return nil, fmt.Errorf("SOS marker not found")
	}

	// Entropy-coded data starts after SOS header
	sosEnd := sos.Offset + 2 + sos.Length

	// Find EOI marker
	eoi := FindMarker(markers, testMarkerEOI)
	if eoi == nil {
		return nil, fmt.Errorf("EOI marker not found")
	}

	// Extract entropy-coded data (excluding EOI)
	if sosEnd >= eoi.Offset {
		return nil, fmt.Errorf("invalid SOS/EOI positions")
	}

	return data[sosEnd:eoi.Offset], nil
}

// GetMarkerRawBytes returns the complete raw bytes for a marker segment including 0xFF marker prefix.
//
//nolint:unused
//goland:noinspection GoUnusedExportedFunction
func GetMarkerRawBytes(data []byte, marker *MarkerSegment) []byte {
	if marker == nil {
		return nil
	}

	// SOI and EOI have no length
	if marker.Type == testMarkerSOI || marker.Type == testMarkerEOI {
		return []byte{markerPrefix, marker.Type}
	}

	// Other markers include 0xFF + type + length field + data
	end := marker.Offset + 2 + marker.Length
	if end > len(data) {
		end = len(data)
	}
	return data[marker.Offset:end]
}

// =============================================================================
// Binary Helpers
// =============================================================================

// readBigEndianUint16 reads a big-endian uint16 from a byte slice.
//
//nolint:unused
//goland:noinspection GoUnusedFunction
func readBigEndianUint16(data []byte) uint16 {
	return binary.BigEndian.Uint16(data)
}
