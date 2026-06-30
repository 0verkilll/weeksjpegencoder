// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements custom Huffman table support, allowing users to specify
// their own Huffman tables for encoding instead of the standard ITU-T T.81 tables.

package weeksjpegencoder

import (
	"fmt"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// Custom Huffman Table Types
// =============================================================================

// CustomHuffmanTable represents a custom Huffman table for JPEG encoding.
// It stores the bits and values arrays as defined in ITU-T T.81, and
// implements the HuffmanTable interface for encoding.
type CustomHuffmanTable struct {
	// Bits contains the count of codes for each length (1-16 bits).
	// Bits[i] = number of codes with length i+1.
	bits [16]byte

	// Values contains the symbol values in order of increasing code length.
	values []byte

	// TableClass: 0 = DC, 1 = AC
	tableClass int

	// TableNum: 0 = luminance, 1 = chrominance
	tableNum int

	// encoderTable is the underlying encoder table for efficient encoding
	encoderTable *jpeg.HuffmanEncoderTable
}

// NewCustomHuffmanTable creates a new custom Huffman table from bits and values.
//
// Parameters:
//   - tableClass: 0 for DC, 1 for AC
//   - tableNum: 0 for luminance, 1 for chrominance
//   - bits: Array of 16 bytes specifying code lengths (bits[i] = count of codes with length i+1)
//   - values: Symbol values in order of increasing code length
//
// Returns an error if validation fails:
//   - tableClass must be 0 or 1
//   - tableNum must be 0 or 1
//   - values length must equal sum of bits array
//
//goland:noinspection GoUnusedExportedFunction
func NewCustomHuffmanTable(tableClass, tableNum int, bits [16]byte, values []byte) (*CustomHuffmanTable, error) {
	// Validate table class
	if tableClass < 0 || tableClass > 1 {
		return nil, fmt.Errorf("invalid table class %d: must be 0 (DC) or 1 (AC)", tableClass)
	}

	// Validate table number
	if tableNum < 0 || tableNum > 1 {
		return nil, fmt.Errorf("invalid table number %d: must be 0 (luminance) or 1 (chrominance)", tableNum)
	}

	// Calculate expected values length
	expectedLen := 0
	for _, b := range bits {
		expectedLen += int(b)
	}

	// Validate values length
	if len(values) != expectedLen {
		return nil, fmt.Errorf("values length %d does not match sum of bits array %d", len(values), expectedLen)
	}

	// Convert bits array to int array for jpeg package
	bitsInt := [16]int{}
	for i, b := range bits {
		bitsInt[i] = int(b)
	}

	// Create the underlying encoder table
	encoderTable := jpeg.NewHuffmanEncoderTable(bitsInt, values, tableClass, tableNum)

	return &CustomHuffmanTable{
		bits:         bits,
		values:       values,
		tableClass:   tableClass,
		tableNum:     tableNum,
		encoderTable: encoderTable,
	}, nil
}

// Encode returns the Huffman code and size for a given symbol.
// This implements the HuffmanTable interface.
func (t *CustomHuffmanTable) Encode(symbol byte) (code uint16, size uint8) {
	result := t.encoderTable.Encode(symbol)
	return result.Code, result.Size
}

// GetBits returns the bits array (counts of codes for each length).
func (t *CustomHuffmanTable) GetBits() [16]byte {
	return t.bits
}

// GetValues returns a copy of the values array.
func (t *CustomHuffmanTable) GetValues() []byte {
	valuesCopy := make([]byte, len(t.values))
	copy(valuesCopy, t.values)
	return valuesCopy
}

// GetTableClass returns the table class (0 for DC, 1 for AC).
func (t *CustomHuffmanTable) GetTableClass() int {
	return t.tableClass
}

// GetTableNum returns the table number (0 for luminance, 1 for chrominance).
func (t *CustomHuffmanTable) GetTableNum() int {
	return t.tableNum
}

// GetEncoderTable returns the underlying jpeg.HuffmanEncoderTable.
func (t *CustomHuffmanTable) GetEncoderTable() *jpeg.HuffmanEncoderTable {
	return t.encoderTable
}

// Compile-time interface compliance check
var _ HuffmanTable = (*CustomHuffmanTable)(nil)
