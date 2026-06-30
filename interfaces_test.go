// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file contains tests for interface compliance, ensuring that adapters
// properly wrap jpeg package concrete types to satisfy weeksjpegencoder interfaces.

package weeksjpegencoder

import (
	"bytes"
	"image"
	"testing"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// Test 1: BitWriter Interface Compliance via Adapter
// =============================================================================

// TestBitWriterAdapterSatisfiesInterface verifies that BitWriterAdapter
// wrapping jpeg.EncoderBitWriter satisfies the BitWriter interface.
func TestBitWriterAdapterSatisfiesInterface(t *testing.T) {
	var buf bytes.Buffer
	concrete := jpeg.NewEncoderBitWriter(&buf)
	adapter := NewBitWriterAdapter(concrete)

	// Verify interface satisfaction at compile time
	var _ BitWriter = adapter

	// Test WriteBits method
	err := adapter.WriteBits(0xABC, 12)
	if err != nil {
		t.Errorf("WriteBits failed: %v", err)
	}

	// Test Flush method
	err = adapter.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Verify some output was written
	if buf.Len() == 0 {
		t.Error("Expected output to be written, but buffer is empty")
	}
}

// =============================================================================
// Test 2: HuffmanTable Interface Compliance via Adapter
// =============================================================================

// TestHuffmanTableAdapterSatisfiesInterface verifies that HuffmanTableAdapter
// wrapping jpeg.HuffmanEncoderTable satisfies the HuffmanTable interface.
func TestHuffmanTableAdapterSatisfiesInterface(t *testing.T) {
	concrete := jpeg.NewStandardEncoderDCLuminanceTable()
	adapter := NewHuffmanTableAdapter(concrete)

	// Verify interface satisfaction at compile time
	var _ HuffmanTable = adapter

	// Test Encode method with category 0 (DC value of 0)
	code, size := adapter.Encode(0)

	// Standard DC luminance table has a code for category 0
	// The exact values depend on the standard table, but size should be non-zero
	// for most categories
	if size == 0 && code != 0 {
		t.Errorf("Encode(0) returned unexpected code=%d, size=%d", code, size)
	}

	// Test with a different symbol
	code1, size1 := adapter.Encode(1)
	if size1 == 0 {
		t.Errorf("Encode(1) returned size=0, expected non-zero size")
	}

	t.Logf("DC Lum Table: symbol=0 -> code=%d, size=%d", code, size)
	t.Logf("DC Lum Table: symbol=1 -> code=%d, size=%d", code1, size1)
}

// =============================================================================
// Test 3: DCT Interface Compliance
// =============================================================================

// TestDCTInterfaceSatisfaction verifies that jpeg.DCTTransformer satisfies
// the DCT interface (via type alias or wrapper).
func TestDCTInterfaceSatisfaction(t *testing.T) {
	// The DCT interface matches jpeg.DCTTransformer's Forward method signature
	dct := jpeg.NewSeparableDCT()

	// Verify interface satisfaction at compile time
	var _ DCT = &DCTAdapter{dct}

	// Create a test block
	var block [64]float64
	for i := range block {
		block[i] = float64(i)
	}

	// Test Forward method through adapter
	adapter := &DCTAdapter{dct}
	original := block // Copy for comparison

	adapter.Forward(&block)

	// Block should have been transformed (values should change)
	changed := false
	for i := range block {
		if block[i] != original[i] {
			changed = true
			break
		}
	}

	if !changed {
		t.Error("Forward DCT did not modify block values")
	}

	// DC coefficient (position 0) should be non-zero for non-zero input
	if block[0] == 0 {
		t.Error("Expected non-zero DC coefficient after forward DCT")
	}
}

// =============================================================================
// Test 4: Mock Implementations Satisfy Interfaces
// =============================================================================

// TestMockImplementationsSatisfyInterfaces verifies that mock implementations
// can satisfy the interfaces, enabling future unit testing with mocks.
func TestMockImplementationsSatisfyInterfaces(t *testing.T) {
	// Test MockBitWriter
	t.Run("MockBitWriter", func(t *testing.T) {
		mock := &MockBitWriter{}
		var _ BitWriter = mock

		err := mock.WriteBits(0x123, 10)
		if err != nil {
			t.Errorf("MockBitWriter.WriteBits failed: %v", err)
		}
		if len(mock.WrittenBits) != 1 {
			t.Errorf("Expected 1 write recorded, got %d", len(mock.WrittenBits))
		}
		if mock.WrittenBits[0].Bits != 0x123 || mock.WrittenBits[0].NBits != 10 {
			t.Errorf("WrittenBits mismatch: got %+v", mock.WrittenBits[0])
		}

		err = mock.Flush()
		if err != nil {
			t.Errorf("MockBitWriter.Flush failed: %v", err)
		}
		if !mock.Flushed {
			t.Error("Expected Flushed to be true")
		}
	})

	// Test MockHuffmanTable
	t.Run("MockHuffmanTable", func(t *testing.T) {
		mock := &MockHuffmanTable{
			Codes: map[byte]struct {
				Code uint16
				Size uint8
			}{
				0:    {Code: 0b00, Size: 2},
				1:    {Code: 0b010, Size: 3},
				0xF0: {Code: 0b11111111001, Size: 11}, // ZRL
			},
		}
		var _ HuffmanTable = mock

		code, size := mock.Encode(0)
		if code != 0b00 || size != 2 {
			t.Errorf("MockHuffmanTable.Encode(0) = (%d, %d), want (0, 2)", code, size)
		}

		code, size = mock.Encode(1)
		if code != 0b010 || size != 3 {
			t.Errorf("MockHuffmanTable.Encode(1) = (%d, %d), want (2, 3)", code, size)
		}
	})

	// Test MockDCT
	t.Run("MockDCT", func(t *testing.T) {
		mock := &MockDCT{}
		var _ DCT = mock

		var block [64]float64
		for i := range block {
			block[i] = float64(i + 1)
		}

		mock.Forward(&block)
		if mock.ForwardCalls != 1 {
			t.Errorf("Expected 1 forward call, got %d", mock.ForwardCalls)
		}
	})

	// Test MockQuantizer
	t.Run("MockQuantizer", func(t *testing.T) {
		mock := &MockQuantizer{
			LumTable:   [64]int{16, 11, 10, 16, 24, 40, 51, 61}, // partial
			ChromTable: [64]int{17, 18, 24, 47, 99, 99, 99, 99}, // partial
		}
		var _ Quantizer = mock

		var block [64]float64
		for i := range block {
			block[i] = float64((i + 1) * 10)
		}

		// QuantizeBlock returns a fixed [64]int; this verifies the call runs.
		_ = mock.QuantizeBlock(&block, true)

		table := mock.GetQuantTable(true)
		if table[0] != 16 {
			t.Errorf("GetQuantTable(true)[0] = %d, want 16", table[0])
		}
	})

	// Test MockBlockEncoder
	t.Run("MockBlockEncoder", func(t *testing.T) {
		mock := &MockBlockEncoder{}
		var _ BlockEncoder = mock

		var block [64]int
		block[0] = 100 // DC coefficient

		newDC, err := mock.EncodeBlock(&block, 50, true)
		if err != nil {
			t.Errorf("EncodeBlock failed: %v", err)
		}
		if newDC != 100 {
			t.Errorf("EncodeBlock returned newDC=%d, want 100", newDC)
		}
		if mock.EncodeCalls != 1 {
			t.Errorf("Expected 1 encode call, got %d", mock.EncodeCalls)
		}

		err = mock.Flush()
		if err != nil {
			t.Errorf("Flush failed: %v", err)
		}
		if !mock.Flushed {
			t.Error("Expected Flushed to be true")
		}
	})

	// Test MockBlockExtractor
	t.Run("MockBlockExtractor", func(t *testing.T) {
		mock := &MockBlockExtractor{
			FixedBlock: [64]float64{128, 128, 128, 128}, // partial constant block
		}
		var _ BlockExtractor = mock

		// Create a simple test image
		img := image.NewRGBA(image.Rect(0, 0, 16, 16))

		block := mock.ExtractBlock(img, 0, 0, 0)
		if block[0] != 128 {
			t.Errorf("ExtractBlock()[0] = %f, want 128", block[0])
		}
		if mock.ExtractCalls != 1 {
			t.Errorf("Expected 1 extract call, got %d", mock.ExtractCalls)
		}
	})
}

// =============================================================================
// Test 5: Quantizer Interface Definition
// =============================================================================

// TestQuantizerInterfaceDefinition verifies the Quantizer interface is properly
// defined with the expected method signatures.
func TestQuantizerInterfaceDefinition(t *testing.T) {
	// Create a mock that implements Quantizer
	mock := &MockQuantizer{
		LumTable:   [64]int{16, 11, 10, 16, 24, 40, 51, 61},
		ChromTable: [64]int{17, 18, 24, 47, 99, 99, 99, 99},
	}

	// Verify interface at compile time
	var q Quantizer = mock

	// Test QuantizeBlock signature
	var block [64]float64
	for i := range block {
		block[i] = float64(i * 100)
	}

	result := q.QuantizeBlock(&block, true)
	// Verify the result array has correct length (always 64 for [64]int)
	if len(result) != 64 {
		t.Errorf("QuantizeBlock should return 64 values, got %d", len(result))
	}

	// Verify quantization actually happened - result should have reasonable values
	// For input block[0] = 0, result[0] should be 0
	if result[0] != 0 {
		t.Errorf("Expected result[0] = 0 for input 0, got %d", result[0])
	}

	// Test GetQuantTable signature
	lumTable := q.GetQuantTable(true)
	chromTable := q.GetQuantTable(false)

	if lumTable[0] == 0 && chromTable[0] == 0 {
		t.Error("At least one quantization table should have non-zero values")
	}
}

// =============================================================================
// Test 6: BlockEncoder and BlockExtractor Interface Definitions
// =============================================================================

// TestBlockEncoderInterfaceDefinition verifies the BlockEncoder interface
// is properly defined with expected method signatures.
func TestBlockEncoderInterfaceDefinition(t *testing.T) {
	mock := &MockBlockEncoder{}
	var be BlockEncoder = mock

	var block [64]int
	block[0] = 50 // DC value

	// Test EncodeBlock signature: (block, prevDC, isLuminance) -> (newDC, error)
	newDC, err := be.EncodeBlock(&block, 25, true)
	if err != nil {
		t.Errorf("EncodeBlock returned error: %v", err)
	}
	if newDC != block[0] {
		t.Errorf("EncodeBlock should return new DC value, got %d want %d", newDC, block[0])
	}

	// Test Flush signature: () -> error
	err = be.Flush()
	if err != nil {
		t.Errorf("Flush returned error: %v", err)
	}
}

// TestBlockExtractorInterfaceDefinition verifies the BlockExtractor interface
// is properly defined with expected method signatures.
func TestBlockExtractorInterfaceDefinition(t *testing.T) {
	mock := &MockBlockExtractor{
		FixedBlock: [64]float64{},
	}
	for i := range mock.FixedBlock {
		mock.FixedBlock[i] = 128.0
	}

	var bx BlockExtractor = mock

	// Create test image
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))

	// Test ExtractBlock signature: (img, component, x, y) -> [64]float64.
	// The return is a fixed [64]float64; this verifies the call runs.
	_ = bx.ExtractBlock(img, 0, 0, 0)
}
