// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file contains tests for the custom Huffman table API (Task Group 5).

package weeksjpegencoder

import (
	"bytes"
	"image"
	"testing"
)

// =============================================================================
// Task 5.1: Custom Huffman Table API Tests
// =============================================================================

// TestSetHuffmanTable_ValidDCLuminanceTable tests SetHuffmanTable with a valid DC luminance table.
func TestSetHuffmanTable_ValidDCLuminanceTable(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	// Standard DC luminance table bits and values
	bits := [16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
	values := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B}

	// SetHuffmanTable should return the encoder for chaining
	result := enc.SetHuffmanTable(0, 0, bits, values) // DC (0), Luminance (0)
	if result != enc {
		t.Error("SetHuffmanTable should return the encoder for method chaining")
	}

	// Verify the table was stored
	if enc.customHuffmanTables[0][0] == nil {
		t.Error("Custom DC luminance table should be stored")
	}

	// Verify no error was recorded
	if enc.lastHuffmanTableError != nil {
		t.Errorf("Unexpected error: %v", enc.lastHuffmanTableError)
	}
}

// TestSetHuffmanTable_ValidACChrominanceTable tests SetHuffmanTable with a valid AC chrominance table.
func TestSetHuffmanTable_ValidACChrominanceTable(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	// Standard AC chrominance table bits (first 16) and values
	bits := [16]byte{0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 119}

	// Calculate expected values length
	sumBits := 0
	for _, b := range bits {
		sumBits += int(b)
	}
	// Create values slice with correct length
	values := make([]byte, sumBits)
	for i := range values {
		values[i] = byte(i % 256)
	}

	// SetHuffmanTable should return the encoder for chaining
	result := enc.SetHuffmanTable(1, 1, bits, values) // AC (1), Chrominance (1)
	if result != enc {
		t.Error("SetHuffmanTable should return the encoder for method chaining")
	}

	// Verify the table was stored
	if enc.customHuffmanTables[1][1] == nil {
		t.Error("Custom AC chrominance table should be stored")
	}

	// Verify no error was recorded
	if enc.lastHuffmanTableError != nil {
		t.Errorf("Unexpected error: %v", enc.lastHuffmanTableError)
	}
}

// TestSetHuffmanTable_ValidationBitsArray tests validation of bits array (16 elements).
func TestSetHuffmanTable_ValidationBitsArray(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	// The bits array is fixed-size [16]byte, so there's no way to provide wrong length.
	// Test that the array elements work correctly with valid range.
	bits := [16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
	values := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B}

	// Should succeed with valid bits array
	enc.SetHuffmanTable(0, 0, bits, values)

	if enc.customHuffmanTables[0][0] == nil {
		t.Error("Valid bits array should result in table being stored")
	}
}

// TestSetHuffmanTable_ValidationValuesLengthMatchesSumOfBits tests that values length equals sum of bits.
func TestSetHuffmanTable_ValidationValuesLengthMatchesSumOfBits(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	// Standard DC luminance table bits
	bits := [16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
	// Sum of bits = 0+1+5+1+1+1+1+1+1+0+0+0+0+0+0+0 = 12

	// Test with wrong length (too short)
	wrongValues := []byte{0x00, 0x01, 0x02} // Only 3, should be 12

	// Store the error (via the validationError field)
	enc.SetHuffmanTable(0, 0, bits, wrongValues)

	// Table should not be stored when validation fails
	if enc.customHuffmanTables[0][0] != nil {
		t.Error("Custom table should not be stored when values length doesn't match sum of bits")
	}

	// Check that validation error was recorded
	if enc.lastHuffmanTableError == nil {
		t.Error("Validation error should be recorded when values length is wrong")
	}

	// Test with correct length
	correctValues := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B}
	enc.SetHuffmanTable(0, 0, bits, correctValues)

	if enc.customHuffmanTables[0][0] == nil {
		t.Error("Custom table should be stored when values length matches sum of bits")
	}
}

// TestSetHuffmanTable_CustomTablesOverrideStandardTables tests that custom tables override standard tables.
func TestSetHuffmanTable_CustomTablesOverrideStandardTables(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	// Use standard tables bits and values (which should work)
	bits := [16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
	values := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B}

	// Set custom DC luminance table
	enc.SetHuffmanTable(0, 0, bits, values)

	// Create a simple 8x8 test image
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, image.White)
		}
	}

	// Encode should work with custom table
	err = enc.Encode(img)
	if err != nil {
		t.Fatalf("Encode with custom Huffman table failed: %v", err)
	}

	// Verify the output is valid JPEG
	data := buf.Bytes()
	if len(data) < 2 {
		t.Fatal("Output too short")
	}
	if data[0] != 0xFF || data[1] != 0xD8 {
		t.Error("Output should start with SOI marker")
	}
}

// TestSetHuffmanTable_InvalidTableClass tests validation of table class parameter.
func TestSetHuffmanTable_InvalidTableClass(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	bits := [16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
	values := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B}

	// Test with invalid table class (should be 0 or 1)
	enc.SetHuffmanTable(2, 0, bits, values) // Invalid class

	// Should record validation error
	if enc.lastHuffmanTableError == nil {
		t.Error("Invalid table class should record validation error")
	}
}

// TestSetHuffmanTable_InvalidTableNum tests validation of table number parameter.
func TestSetHuffmanTable_InvalidTableNum(t *testing.T) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}

	bits := [16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
	values := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B}

	// Test with invalid table number (should be 0 or 1)
	enc.SetHuffmanTable(0, 2, bits, values) // Invalid table number

	// Should record validation error
	if enc.lastHuffmanTableError == nil {
		t.Error("Invalid table number should record validation error")
	}
}
