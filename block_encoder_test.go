// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file contains unit tests for the HuffmanBlockEncoder implementation.
// Tests verify DC encoding, AC run-length encoding, EOB markers, ZRL handling,
// and proper interaction with mock BitWriter and HuffmanTable interfaces.

package weeksjpegencoder

import (
	"testing"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// Test Helper: Create mock Huffman tables with predictable codes
// =============================================================================

// createTestHuffmanTable creates a mock HuffmanTable with predictable codes.
// Each symbol maps to a code equal to the symbol value, with size based on value.
func createTestHuffmanTable() *MockHuffmanTable {
	codes := make(map[byte]struct {
		Code uint16
		Size uint8
	})

	// DC categories 0-12 (standard JPEG supports up to 11, but we add 12 for boundary)
	for i := byte(0); i <= 12; i++ {
		codes[i] = struct {
			Code uint16
			Size uint8
		}{uint16(i), i + 2} // Code is i, size is i+2 bits
	}

	// AC symbols: EOB (0x00), ZRL (0xF0), and common run/size combinations
	codes[0x00] = struct {
		Code uint16
		Size uint8
	}{0x00, 4} // EOB
	codes[0xF0] = struct {
		Code uint16
		Size uint8
	}{0xF0, 11} // ZRL (16 zeros)

	// Add some common AC run/size combinations (run=0, size=1-8)
	for size := byte(1); size <= 8; size++ {
		symbol := size // run=0, size=size
		codes[symbol] = struct {
			Code uint16
			Size uint8
		}{uint16(symbol), size + 3}
	}

	// Add run/size combinations for run 1-15, size 1-4
	for run := byte(1); run <= 15; run++ {
		for size := byte(1); size <= 4; size++ {
			symbol := (run << 4) | size
			codes[symbol] = struct {
				Code uint16
				Size uint8
			}{uint16(symbol), 8}
		}
	}

	return &MockHuffmanTable{Codes: codes}
}

// =============================================================================
// Task 3.1: Tests for BlockEncoder Functionality
// =============================================================================

// TestBlockEncoder_DCEncoding_DiffValues tests DC encoding with various diff values.
// Verifies correct category and additional bits are written for:
// 0, 1, -1, 255, -255, 2047, -2048
func TestBlockEncoder_DCEncoding_DiffValues(t *testing.T) {
	tests := []struct {
		name     string
		dcValue  int
		prevDC   int
		wantDiff int
	}{
		{"diff_0", 0, 0, 0},
		{"diff_1", 1, 0, 1},
		{"diff_-1", -1, 0, -1},
		{"diff_255", 255, 0, 255},
		{"diff_-255", -255, 0, -255},
		{"diff_2047", 2047, 0, 2047},
		{"diff_-2048", -2048, 0, -2048},
		{"diff_from_prev", 100, 50, 50}, // prevDC matters
		{"diff_negative_from_prev", 50, 100, -50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBW := &MockBitWriter{}
			dcTable := createTestHuffmanTable()
			acTable := createTestHuffmanTable()

			encoder := NewHuffmanBlockEncoder(mockBW, dcTable, dcTable, acTable, acTable)

			// Create block with just DC value, rest zeros (will trigger EOB)
			var block [64]int
			block[0] = tt.dcValue

			newDC, err := encoder.EncodeBlock(&block, tt.prevDC, true)
			if err != nil {
				t.Fatalf("EncodeBlock failed: %v", err)
			}

			// Verify returned DC value
			if newDC != tt.dcValue {
				t.Errorf("newDC = %d, want %d", newDC, tt.dcValue)
			}

			// Verify at least one write happened (DC Huffman code)
			if len(mockBW.WrittenBits) == 0 {
				t.Fatal("expected at least one WriteBits call for DC code")
			}

			// Verify the first write uses the correct category from EncodeDCValue
			expectedCat, _, _ := jpeg.EncodeDCValue(tt.wantDiff)
			dcCode, dcSize := dcTable.Encode(expectedCat)

			if mockBW.WrittenBits[0].Bits != uint32(dcCode) || mockBW.WrittenBits[0].NBits != int(dcSize) {
				t.Errorf("DC code written = {%d, %d}, expected {%d, %d} for category %d",
					mockBW.WrittenBits[0].Bits, mockBW.WrittenBits[0].NBits,
					dcCode, dcSize, expectedCat)
			}
		})
	}
}

// TestBlockEncoder_ACRunLength_SingleNonZero tests AC encoding with single non-zero coefficient.
func TestBlockEncoder_ACRunLength_SingleNonZero(t *testing.T) {
	tests := []struct {
		name     string
		position int // Position of the non-zero coefficient (1-63)
		value    int // The AC coefficient value
	}{
		{"pos1_val1", 1, 1},
		{"pos1_val-1", 1, -1},
		{"pos5_val10", 5, 10},
		{"pos10_val-20", 10, -20},
		{"pos63_val127", 63, 127},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBW := &MockBitWriter{}
			dcTable := createTestHuffmanTable()
			acTable := createTestHuffmanTable()

			encoder := NewHuffmanBlockEncoder(mockBW, dcTable, dcTable, acTable, acTable)

			// Create block with DC=0 and one non-zero AC
			var block [64]int
			block[0] = 0 // DC
			block[tt.position] = tt.value

			_, err := encoder.EncodeBlock(&block, 0, true)
			if err != nil {
				t.Fatalf("EncodeBlock failed: %v", err)
			}

			// Verify writes happened:
			// 1. DC Huffman code
			// 2. AC run/size code for the non-zero coefficient
			// 3. AC additional bits
			// 4. EOB if there are trailing zeros
			if len(mockBW.WrittenBits) < 2 {
				t.Errorf("expected at least 2 WriteBits calls, got %d", len(mockBW.WrittenBits))
			}
		})
	}
}

// TestBlockEncoder_ACRunLength_MultipleNonZeros tests AC encoding with multiple non-zero coefficients.
func TestBlockEncoder_ACRunLength_MultipleNonZeros(t *testing.T) {
	mockBW := &MockBitWriter{}
	dcTable := createTestHuffmanTable()
	acTable := createTestHuffmanTable()

	encoder := NewHuffmanBlockEncoder(mockBW, dcTable, dcTable, acTable, acTable)

	// Create block with DC=0 and several non-zero ACs
	var block [64]int
	block[0] = 0  // DC
	block[1] = 10 // First AC
	block[2] = -5 // Second AC (no zeros between)
	block[5] = 20 // Fourth AC (3 zeros before)
	block[10] = 7 // Fifth AC (5 zeros before)
	// Rest are zeros -> EOB

	_, err := encoder.EncodeBlock(&block, 0, true)
	if err != nil {
		t.Fatalf("EncodeBlock failed: %v", err)
	}

	// Should have multiple writes:
	// DC code, AC1 code+bits, AC2 code+bits, AC5 code+bits, AC10 code+bits, EOB
	if len(mockBW.WrittenBits) < 6 {
		t.Errorf("expected at least 6 WriteBits calls for multiple non-zeros, got %d", len(mockBW.WrittenBits))
	}
}

// TestBlockEncoder_EOBMarker tests that EOB marker is written when block ends with zeros.
func TestBlockEncoder_EOBMarker(t *testing.T) {
	tests := []struct {
		name          string
		lastNonZeroAt int  // Position of last non-zero (0 means DC only)
		expectEOB     bool // Whether we expect EOB
	}{
		{"dc_only_zeros", 0, true},     // DC=5, all AC=0 -> EOB
		{"one_ac_then_zeros", 1, true}, // DC=5, AC1=1, rest zeros -> EOB
		{"mid_then_zeros", 30, true},   // DC=5, some AC, rest zeros -> EOB
		{"last_position", 63, false},   // Last position has value -> no EOB needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBW := &MockBitWriter{}
			dcTable := createTestHuffmanTable()
			acTable := createTestHuffmanTable()

			encoder := NewHuffmanBlockEncoder(mockBW, dcTable, dcTable, acTable, acTable)

			var block [64]int
			block[0] = 5 // DC
			if tt.lastNonZeroAt > 0 {
				block[tt.lastNonZeroAt] = 1
			}

			_, err := encoder.EncodeBlock(&block, 0, true)
			if err != nil {
				t.Fatalf("EncodeBlock failed: %v", err)
			}

			// Check if EOB was written (symbol 0x00)
			// We look for a write that corresponds to EOB
			eobFound := false
			eobCode, eobSize := acTable.Encode(0x00)
			for _, w := range mockBW.WrittenBits {
				if w.Bits == uint32(eobCode) && w.NBits == int(eobSize) {
					eobFound = true
					break
				}
			}

			if tt.expectEOB && !eobFound {
				t.Errorf("expected EOB marker to be written, but it wasn't")
			}
			// Note: If last position is 63 with non-zero, no EOB needed
		})
	}
}

// TestBlockEncoder_ZRLMarker tests ZRL (0xF0) written for runs > 15 zeros.
func TestBlockEncoder_ZRLMarker(t *testing.T) {
	tests := []struct {
		name        string
		zerosBefore int // Number of zeros before non-zero coefficient
		expectZRL   int // Expected number of ZRL markers
	}{
		{"15_zeros", 15, 0}, // 15 zeros before AC -> no ZRL
		{"16_zeros", 16, 1}, // 16 zeros -> 1 ZRL
		{"17_zeros", 17, 1}, // 17 zeros -> 1 ZRL, 1 remaining
		{"32_zeros", 32, 2}, // 32 zeros -> 2 ZRLs
		{"48_zeros", 48, 3}, // 48 zeros -> 3 ZRLs
		{"63_zeros", 62, 3}, // 62 zeros (position 63) -> 3 ZRLs + remainder
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBW := &MockBitWriter{}
			dcTable := createTestHuffmanTable()
			acTable := createTestHuffmanTable()

			encoder := NewHuffmanBlockEncoder(mockBW, dcTable, dcTable, acTable, acTable)

			var block [64]int
			block[0] = 0 // DC
			// Position the non-zero at zerosBefore+1 (since position 0 is DC)
			nonZeroPos := tt.zerosBefore + 1
			if nonZeroPos < 64 {
				block[nonZeroPos] = 1
			}

			_, err := encoder.EncodeBlock(&block, 0, true)
			if err != nil {
				t.Fatalf("EncodeBlock failed: %v", err)
			}

			// Count ZRL markers (0xF0)
			zrlCode, zrlSize := acTable.Encode(0xF0)
			zrlCount := 0
			for _, w := range mockBW.WrittenBits {
				if w.Bits == uint32(zrlCode) && w.NBits == int(zrlSize) {
					zrlCount++
				}
			}

			if zrlCount != tt.expectZRL {
				t.Errorf("expected %d ZRL markers, got %d", tt.expectZRL, zrlCount)
			}
		})
	}
}

// TestBlockEncoder_MockBitWriter tests BitWriter receives correct bits.
func TestBlockEncoder_MockBitWriter(t *testing.T) {
	mockBW := &MockBitWriter{}
	dcTable := createTestHuffmanTable()
	acTable := createTestHuffmanTable()

	encoder := NewHuffmanBlockEncoder(mockBW, dcTable, dcTable, acTable, acTable)

	// Simple block: DC=1, first AC=1, rest zeros
	var block [64]int
	block[0] = 1 // DC diff=1 (assuming prevDC=0) -> category 1
	block[1] = 1 // AC position 1, value 1 -> run=0, size=1

	_, err := encoder.EncodeBlock(&block, 0, true)
	if err != nil {
		t.Fatalf("EncodeBlock failed: %v", err)
	}

	// Verify writes occurred
	if len(mockBW.WrittenBits) == 0 {
		t.Fatal("expected WriteBits calls, got none")
	}

	// First write should be DC Huffman code for category 1
	// Category for diff=1 is 1, so we expect table.Encode(1)
	dcCode, dcSize := dcTable.Encode(1)
	if mockBW.WrittenBits[0].Bits != uint32(dcCode) || mockBW.WrittenBits[0].NBits != int(dcSize) {
		t.Errorf("first write = {%d, %d}, expected DC code {%d, %d}",
			mockBW.WrittenBits[0].Bits, mockBW.WrittenBits[0].NBits,
			dcCode, dcSize)
	}
}

// TestBlockEncoder_MockHuffmanTable tests HuffmanTable receives correct symbols.
func TestBlockEncoder_MockHuffmanTable(t *testing.T) {
	// This test verifies the encoder calls Encode with correct symbols
	mockBW := &MockBitWriter{}

	// Create tracking tables
	dcCalls := make([]byte, 0)
	acCalls := make([]byte, 0)

	dcTable := &trackingHuffmanTable{
		calls:            &dcCalls,
		MockHuffmanTable: MockHuffmanTable{Codes: createTestHuffmanTable().Codes},
	}
	acTable := &trackingHuffmanTable{
		calls:            &acCalls,
		MockHuffmanTable: MockHuffmanTable{Codes: createTestHuffmanTable().Codes},
	}

	encoder := NewHuffmanBlockEncoder(mockBW, dcTable, dcTable, acTable, acTable)

	// Block: DC=10 (diff=10, category 4), AC1=5, rest zeros
	var block [64]int
	block[0] = 10 // DC -> category 4
	block[1] = 5  // AC -> run=0, size=3 (since 5 fits in 3 bits)

	_, err := encoder.EncodeBlock(&block, 0, true)
	if err != nil {
		t.Fatalf("EncodeBlock failed: %v", err)
	}

	// Verify DC table was called with correct category
	if len(dcCalls) == 0 {
		t.Fatal("expected DC table Encode to be called")
	}
	expectedCat, _, _ := jpeg.EncodeDCValue(10)
	if dcCalls[0] != expectedCat {
		t.Errorf("DC table called with %d, expected category %d", dcCalls[0], expectedCat)
	}

	// Verify AC table was called (at minimum for the AC value and EOB)
	if len(acCalls) < 2 {
		t.Errorf("expected at least 2 AC table calls (AC + EOB), got %d", len(acCalls))
	}
}

// trackingHuffmanTable wraps MockHuffmanTable to track Encode calls.
type trackingHuffmanTable struct {
	MockHuffmanTable
	calls *[]byte
}

func (t *trackingHuffmanTable) Encode(symbol byte) (code uint16, size uint8) {
	*t.calls = append(*t.calls, symbol)
	return t.MockHuffmanTable.Encode(symbol)
}

// TestBlockEncoder_Flush tests that Flush properly finalizes entropy-coded data.
func TestBlockEncoder_Flush(t *testing.T) {
	mockBW := &MockBitWriter{}
	dcTable := createTestHuffmanTable()
	acTable := createTestHuffmanTable()

	encoder := NewHuffmanBlockEncoder(mockBW, dcTable, dcTable, acTable, acTable)

	// Encode a simple block first
	var block [64]int
	block[0] = 1
	_, err := encoder.EncodeBlock(&block, 0, true)
	if err != nil {
		t.Fatalf("EncodeBlock failed: %v", err)
	}

	// Flush should not have been called yet
	if mockBW.Flushed {
		t.Error("BitWriter.Flush called before encoder.Flush()")
	}

	// Now call Flush
	err = encoder.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Verify BitWriter.Flush was called
	if !mockBW.Flushed {
		t.Error("BitWriter.Flush not called after encoder.Flush()")
	}
}

// TestBlockEncoder_LuminanceVsChrominance tests correct table selection.
func TestBlockEncoder_LuminanceVsChrominance(t *testing.T) {
	tests := []struct {
		name        string
		isLuminance bool
	}{
		{"luminance", true},
		{"chrominance", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBW := &MockBitWriter{}

			// Create distinct tables for luminance and chrominance
			dcLumTable := &MockHuffmanTable{Codes: map[byte]struct {
				Code uint16
				Size uint8
			}{0: {100, 2}, 1: {101, 3}}} // Luminance uses codes 100, 101
			dcChromTable := &MockHuffmanTable{Codes: map[byte]struct {
				Code uint16
				Size uint8
			}{0: {200, 2}, 1: {201, 3}}} // Chrominance uses codes 200, 201
			acLumTable := &MockHuffmanTable{Codes: map[byte]struct {
				Code uint16
				Size uint8
			}{0x00: {110, 4}, 0x01: {111, 4}}}
			acChromTable := &MockHuffmanTable{Codes: map[byte]struct {
				Code uint16
				Size uint8
			}{0x00: {210, 4}, 0x01: {211, 4}}}

			encoder := NewHuffmanBlockEncoder(mockBW, dcLumTable, dcChromTable, acLumTable, acChromTable)

			var block [64]int
			block[0] = 0 // DC diff=0 -> category 0

			_, err := encoder.EncodeBlock(&block, 0, tt.isLuminance)
			if err != nil {
				t.Fatalf("EncodeBlock failed: %v", err)
			}

			// Check which table's code was used
			if len(mockBW.WrittenBits) == 0 {
				t.Fatal("expected WriteBits calls")
			}

			firstCode := mockBW.WrittenBits[0].Bits
			if tt.isLuminance {
				if firstCode != 100 {
					t.Errorf("expected luminance DC code 100, got %d", firstCode)
				}
			} else {
				if firstCode != 200 {
					t.Errorf("expected chrominance DC code 200, got %d", firstCode)
				}
			}
		})
	}
}

// =============================================================================
// Compile-time interface compliance check
// =============================================================================

var _ BlockEncoder = (*HuffmanBlockEncoder)(nil)
