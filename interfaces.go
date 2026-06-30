// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file defines the core interfaces that abstract JPEG encoding components,
// enabling dependency injection, testability, and loose coupling per SOLID principles.
//
// The interfaces are designed to be small and focused (Interface Segregation Principle),
// with each interface having 1-3 methods that represent a single responsibility.

package weeksjpegencoder

import (
	"image"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// Wrapper Interfaces for JPEG Package Abstraction
// =============================================================================

// BitWriter abstracts bit-level writing for entropy coding.
//
// This interface wraps the jpeg.EncoderBitWriter functionality, allowing
// injection of mock implementations for testing entropy encoding logic
// without writing actual bit streams.
//
// The interface follows JPEG entropy coding requirements:
//   - Bits are written most-significant bit first
//   - Flush pads partial bytes with 1-bits per ITU-T T.81 spec
//
// Example usage:
//
//	func encodeCoefficient(bw BitWriter, code uint16, size int) error {
//	    return bw.WriteBits(uint32(code), size)
//	}
type BitWriter interface {
	// WriteBits writes n bits from the least significant bits of the value.
	// The bits parameter contains the value, and nBits specifies how many
	// bits to write (1-32). Returns an error if the underlying write fails.
	WriteBits(bits uint32, nBits int) error

	// Flush writes any remaining bits, padding with 1-bits as per JPEG spec.
	// This should be called at the end of entropy-coded data to ensure
	// proper byte alignment.
	Flush() error
}

// HuffmanTable abstracts Huffman code lookup for JPEG encoding.
//
// This interface wraps the jpeg.HuffmanEncoderTable functionality, allowing
// alternative Huffman table implementations and mock tables for testing.
//
// In JPEG encoding, Huffman tables are used for:
//   - DC coefficient categories (0-11 for differential DC values)
//   - AC coefficient run/size symbols (RRRRSSSS format per ITU-T T.81)
//
// Example usage:
//
//	func encodeDC(table HuffmanTable, bw BitWriter, category byte) error {
//	    code, size := table.Encode(category)
//	    return bw.WriteBits(uint32(code), int(size))
//	}
type HuffmanTable interface {
	// Encode returns the Huffman code and size for a given symbol.
	// For DC tables, symbol is the category (0-11).
	// For AC tables, symbol is the run/size byte (RRRRSSSS).
	// Returns the code bits and the number of bits in the code.
	Encode(symbol byte) (code uint16, size uint8)
}

// DCT abstracts Discrete Cosine Transform operations for JPEG encoding.
//
// This interface provides the forward DCT operation needed for JPEG encoding.
// It wraps the jpeg.DCTTransformer interface, allowing custom DCT implementations
// or mock transforms for testing block processing logic.
//
// Per ITU-T T.81, the DCT transforms spatial domain pixels into frequency
// domain coefficients, with the DC coefficient at position 0 representing
// the average value and AC coefficients representing frequency components.
//
// Example usage:
//
//	func processBlock(dct DCT, block *[64]float64) {
//	    // Level shift
//	    for i := range block {
//	        block[i] -= 128.0
//	    }
//	    // Forward DCT
//	    dct.Forward(block)
//	}
type DCT interface {
	// Forward performs forward DCT on an 8x8 block in-place.
	// Input values should be level-shifted (subtracted by 128 for 8-bit).
	// After transform, block[0] contains the DC coefficient and
	// blocks[1-63] contain AC coefficients.
	Forward(block *[64]float64)
}

// =============================================================================
// Component Interfaces for SOLID Refactoring
// =============================================================================

// Quantizer performs DCT coefficient quantization for JPEG encoding.
//
// Implementations determine how DCT coefficients are divided by quantization
// table values to reduce precision and file size. The quantization step is
// lossy and primarily responsible for JPEG compression artifacts.
//
// The IJG (Independent JPEG Group) quality scaling formula is commonly used:
//   - Quality < 50: scale = 5000 / quality
//   - Quality >= 50: scale = 200 - 2*quality
//
// Example usage:
//
//	quantizer, _ := NewIJGQuantizer(75)
//	quantized := quantizer.QuantizeBlock(&dctBlock, true) // luminance
type Quantizer interface {
	// QuantizeBlock quantizes DCT coefficients in a block.
	// isLuminance determines which quantization table to use:
	//   - true: luminance (Y component)
	//   - false: chrominance (Cb/Cr components)
	// Returns zigzag-ordered quantized coefficients suitable for entropy coding.
	QuantizeBlock(block *[64]float64, isLuminance bool) [64]int

	// GetQuantTable returns the quantization table for the specified component.
	// Used for writing DQT markers and for inspection/debugging.
	GetQuantTable(isLuminance bool) [64]int
}

// BlockEncoder performs entropy encoding of quantized DCT blocks.
//
// Implementations handle the conversion of quantized coefficients into
// entropy-coded bit streams using Huffman coding, including:
//   - DC differential encoding (each DC value encoded as difference from previous)
//   - AC run-length encoding (zeros encoded as runs)
//   - Special symbols: EOB (End of Block) and ZRL (Zero Run Length)
//
// Per ITU-T T.81, the encoding process for each block:
//  1. DC coefficient: encode difference from previous block's DC
//  2. AC coefficients: run-length encode in zigzag order
//  3. EOB marker: signal end of non-zero coefficients
//
// Example usage:
//
//	encoder := NewHuffmanBlockEncoder(bitWriter, dcTable, acTable)
//	prevDC := 0
//	for _, block := range blocks {
//	    prevDC, _ = encoder.EncodeBlock(&block, prevDC, isLuminance)
//	}
//	encoder.Flush()
type BlockEncoder interface {
	// EncodeBlock encodes a zigzag-ordered quantized block.
	// prevDC is the previous block's DC value for differential encoding.
	// isLuminance selects the appropriate Huffman tables:
	//   - true: luminance DC/AC tables
	//   - false: chrominance DC/AC tables
	// Returns the new DC value (block[0]) for use as prevDC in next call,
	// and any encoding error.
	EncodeBlock(block *[64]int, prevDC int, isLuminance bool) (newDC int, err error)

	// Flush finalizes the entropy-coded data.
	// Pads any partial byte with 1-bits per JPEG spec and writes to output.
	Flush() error
}

// BlockExtractor extracts pixel blocks from images for DCT processing.
//
// Implementations handle color space conversion and chroma subsampling,
// converting image pixels into 8x8 blocks of float64 values suitable
// for DCT transformation.
//
// For YCbCr color space (standard JPEG):
//   - Component 0: Y (luminance)
//   - Component 1: Cb (blue-difference chroma)
//   - Component 2: Cr (red-difference chroma)
//
// Chroma subsampling modes affect how Cb/Cr blocks are extracted:
//   - 4:4:4: Full resolution for all components
//   - 4:2:2: Horizontal subsampling of chroma (2:1)
//   - 4:2:0: Both horizontal and vertical subsampling (2:1 each)
//
// Example usage:
//
//	extractor := NewYCbCrBlockExtractor(jpeg.ChromaSubsampling420)
//	yBlock := extractor.ExtractBlock(img, 0, blockX, blockY)  // luminance
//	cbBlock := extractor.ExtractBlock(img, 1, blockX, blockY) // chroma
type BlockExtractor interface {
	// ExtractBlock extracts an 8x8 block at the given position.
	// Parameters:
	//   - img: Source image (any image.Image implementation)
	//   - component: 0=Y (luminance), 1=Cb, 2=Cr
	//   - x, y: Top-left position of the block in image coordinates
	// Returns 64 float64 values in row-major order.
	// Values are NOT level-shifted (raw pixel values 0-255).
	ExtractBlock(img image.Image, component, x, y int) [64]float64
}

// =============================================================================
// Adapters for jpeg Package Concrete Types
// =============================================================================

// BitWriterAdapter wraps jpeg.EncoderBitWriter to satisfy the BitWriter interface.
type BitWriterAdapter struct {
	bw *jpeg.EncoderBitWriter
}

// NewBitWriterAdapter creates a new BitWriterAdapter wrapping the given EncoderBitWriter.
//
//goland:noinspection GoUnusedExportedFunction
func NewBitWriterAdapter(bw *jpeg.EncoderBitWriter) *BitWriterAdapter {
	return &BitWriterAdapter{bw: bw}
}

// WriteBits writes n bits from the least significant bits of the value.
func (a *BitWriterAdapter) WriteBits(bits uint32, nBits int) error {
	return a.bw.WriteBits(bits, nBits)
}

// Flush writes any remaining bits, padding with 1-bits.
func (a *BitWriterAdapter) Flush() error {
	return a.bw.Flush()
}

// HuffmanTableAdapter wraps jpeg.HuffmanEncoderTable to satisfy the HuffmanTable interface.
type HuffmanTableAdapter struct {
	table *jpeg.HuffmanEncoderTable
}

// NewHuffmanTableAdapter creates a new HuffmanTableAdapter wrapping the given table.
//
//goland:noinspection GoUnusedExportedFunction
func NewHuffmanTableAdapter(table *jpeg.HuffmanEncoderTable) *HuffmanTableAdapter {
	return &HuffmanTableAdapter{table: table}
}

// Encode returns the Huffman code and size for a given symbol.
func (a *HuffmanTableAdapter) Encode(symbol byte) (code uint16, size uint8) {
	result := a.table.Encode(symbol)
	return result.Code, result.Size
}

// DCTAdapter wraps jpeg.DCTTransformer to satisfy the DCT interface.
// The DCT interface only requires Forward, while jpeg.DCTTransformer has both
// Forward and Inverse. This adapter provides the encoding-focused interface.
type DCTAdapter struct {
	dct jpeg.DCTTransformer
}

// NewDCTAdapter creates a new DCTAdapter wrapping the given DCTTransformer.
//
//goland:noinspection GoUnusedExportedFunction
func NewDCTAdapter(dct jpeg.DCTTransformer) *DCTAdapter {
	return &DCTAdapter{dct: dct}
}

// Forward performs forward DCT on an 8x8 block in-place.
func (a *DCTAdapter) Forward(block *[64]float64) {
	a.dct.Forward(block)
}

// =============================================================================
// Mock Implementations for Testing
// =============================================================================

// MockBitWriter is a mock implementation of BitWriter for testing.
// It records all write operations for verification.
type MockBitWriter struct {
	WrittenBits []struct {
		Bits  uint32
		NBits int
	}
	Flushed  bool
	WriteErr error // Set to simulate write errors
}

// WriteBits records the write operation.
func (m *MockBitWriter) WriteBits(bits uint32, nBits int) error {
	if m.WriteErr != nil {
		return m.WriteErr
	}
	m.WrittenBits = append(m.WrittenBits, struct {
		Bits  uint32
		NBits int
	}{bits, nBits})
	return nil
}

// Flush marks the mock as flushed.
func (m *MockBitWriter) Flush() error {
	if m.WriteErr != nil {
		return m.WriteErr
	}
	m.Flushed = true
	return nil
}

// MockHuffmanTable is a mock implementation of HuffmanTable for testing.
type MockHuffmanTable struct {
	Codes map[byte]struct {
		Code uint16
		Size uint8
	}
}

// Encode returns the mock code for the symbol.
func (m *MockHuffmanTable) Encode(symbol byte) (code uint16, size uint8) {
	if m.Codes == nil {
		return 0, 0
	}
	if entry, ok := m.Codes[symbol]; ok {
		return entry.Code, entry.Size
	}
	return 0, 0
}

// MockDCT is a mock implementation of DCT for testing.
type MockDCT struct {
	ForwardCalls int
	// TransformFunc can be set to provide custom transform behavior
	TransformFunc func(block *[64]float64)
}

// Forward records the call and optionally applies a transform.
func (m *MockDCT) Forward(block *[64]float64) {
	m.ForwardCalls++
	if m.TransformFunc != nil {
		m.TransformFunc(block)
	}
}

// MockQuantizer is a mock implementation of Quantizer for testing.
type MockQuantizer struct {
	LumTable      [64]int
	ChromTable    [64]int
	QuantizeCalls int
}

// QuantizeBlock returns a quantized block using the mock tables.
func (m *MockQuantizer) QuantizeBlock(block *[64]float64, isLuminance bool) [64]int {
	m.QuantizeCalls++
	var result [64]int
	table := m.ChromTable
	if isLuminance {
		table = m.LumTable
	}
	for i := 0; i < 64; i++ {
		qt := table[i]
		if qt == 0 {
			qt = 1
		}
		if block[i] >= 0 {
			result[i] = int(block[i]/float64(qt) + 0.5)
		} else {
			result[i] = int(block[i]/float64(qt) - 0.5)
		}
	}
	return result
}

// GetQuantTable returns the mock quantization table.
func (m *MockQuantizer) GetQuantTable(isLuminance bool) [64]int {
	if isLuminance {
		return m.LumTable
	}
	return m.ChromTable
}

// MockBlockEncoder is a mock implementation of BlockEncoder for testing.
type MockBlockEncoder struct {
	EncodeCalls int
	Flushed     bool
	EncodeErr   error
	FlushErr    error
	// EncodedBlocks records all blocks passed to EncodeBlock
	EncodedBlocks []struct {
		Block       [64]int
		PrevDC      int
		IsLuminance bool
	}
}

// EncodeBlock records the call and returns the DC value.
func (m *MockBlockEncoder) EncodeBlock(block *[64]int, prevDC int, isLuminance bool) (newDC int, err error) {
	m.EncodeCalls++
	if m.EncodeErr != nil {
		return 0, m.EncodeErr
	}
	m.EncodedBlocks = append(m.EncodedBlocks, struct {
		Block       [64]int
		PrevDC      int
		IsLuminance bool
	}{*block, prevDC, isLuminance})
	return block[0], nil
}

// Flush marks the mock as flushed.
func (m *MockBlockEncoder) Flush() error {
	if m.FlushErr != nil {
		return m.FlushErr
	}
	m.Flushed = true
	return nil
}

// MockBlockExtractor is a mock implementation of BlockExtractor for testing.
type MockBlockExtractor struct {
	FixedBlock   [64]float64
	ExtractCalls int
	// ExtractedPositions records all extraction positions
	ExtractedPositions []struct {
		Component int
		X, Y      int
	}
}

// ExtractBlock returns the fixed block and records the call.
//
//goland:noinspection GoUnusedParameter
func (m *MockBlockExtractor) ExtractBlock(img image.Image, component, x, y int) [64]float64 {
	m.ExtractCalls++
	m.ExtractedPositions = append(m.ExtractedPositions, struct {
		Component int
		X, Y      int
	}{component, x, y})
	return m.FixedBlock
}
