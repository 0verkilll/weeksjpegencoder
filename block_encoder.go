// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements the HuffmanBlockEncoder, which performs entropy encoding
// of quantized DCT blocks using Huffman coding per ITU-T T.81 specification.

package weeksjpegencoder

import (
	"github.com/0verkilll/jpeg"
)

// HuffmanBlockEncoder implements the BlockEncoder interface using Huffman coding.
//
// This encoder handles the conversion of quantized DCT coefficients into
// entropy-coded bit streams, implementing:
//   - DC differential encoding: Each DC value as difference from previous block
//   - AC run-length encoding: Consecutive zeros encoded as runs
//   - ZRL (Zero Run Length): Symbol 0xF0 for runs of 16 zeros
//   - EOB (End of Block): Symbol 0x00 to signal remaining zeros
//
// The encoder requires four Huffman tables for DC/AC luminance/chrominance.
type HuffmanBlockEncoder struct {
	bitWriter    BitWriter
	dcLumTable   HuffmanTable
	dcChromTable HuffmanTable
	acLumTable   HuffmanTable
	acChromTable HuffmanTable
}

// NewHuffmanBlockEncoder creates a new HuffmanBlockEncoder.
//
// Parameters:
//   - bw: BitWriter for writing entropy-coded bits
//   - dcLum: Huffman table for luminance DC coefficients
//   - dcChrom: Huffman table for chrominance DC coefficients
//   - acLum: Huffman table for luminance AC coefficients
//   - acChrom: Huffman table for chrominance AC coefficients
//
//goland:noinspection GoUnusedExportedFunction
func NewHuffmanBlockEncoder(bw BitWriter, dcLum, dcChrom, acLum, acChrom HuffmanTable) *HuffmanBlockEncoder {
	return &HuffmanBlockEncoder{
		bitWriter:    bw,
		dcLumTable:   dcLum,
		dcChromTable: dcChrom,
		acLumTable:   acLum,
		acChromTable: acChrom,
	}
}

// EncodeBlock encodes a zigzag-ordered quantized block using Huffman coding.
//
// The encoding process follows ITU-T T.81 Section F.1.2:
//  1. DC coefficient: Calculate diff from prevDC, encode category, write bits
//  2. AC coefficients (1-63): Run-length encode in order
//  3. Write ZRL (0xF0) for runs of 16 zeros
//  4. Write EOB (0x00) when all remaining coefficients are zero
//
// Returns the current block's DC value (block[0]) for use as prevDC in next call.
func (e *HuffmanBlockEncoder) EncodeBlock(block *[64]int, prevDC int, isLuminance bool) (newDC int, err error) {
	// Select appropriate Huffman tables based on component type
	var dcTable, acTable HuffmanTable
	if isLuminance {
		dcTable = e.dcLumTable
		acTable = e.acLumTable
	} else {
		dcTable = e.dcChromTable
		acTable = e.acChromTable
	}

	// Encode DC coefficient (differential encoding)
	if err = e.encodeDC(block[0], prevDC, dcTable); err != nil {
		return block[0], err
	}

	// Encode AC coefficients (run-length encoding)
	if err = e.encodeAC(block, acTable); err != nil {
		return block[0], err
	}

	return block[0], nil
}

// encodeDC encodes the DC coefficient using differential encoding.
// Per ITU-T T.81 F.1.2.1: Calculate DIFF, determine category, write Huffman code.
func (e *HuffmanBlockEncoder) encodeDC(dc, prevDC int, dcTable HuffmanTable) error {
	diff := dc - prevDC
	category, additionalBits, nBits := jpeg.EncodeDCValue(diff)

	// Write DC Huffman code for the category
	code, size := dcTable.Encode(category)
	if err := e.bitWriter.WriteBits(uint32(code), int(size)); err != nil {
		return err
	}

	// Write additional bits (exact value within category)
	if nBits > 0 {
		if err := e.bitWriter.WriteBits(uint32(additionalBits), nBits); err != nil {
			return err
		}
	}

	return nil
}

// encodeAC encodes AC coefficients using run-length encoding.
// Per ITU-T T.81 F.1.2.2: Encode (run, size) pairs, ZRL for 16 zeros, EOB at end.
func (e *HuffmanBlockEncoder) encodeAC(block *[64]int, acTable HuffmanTable) error {
	runLength := 0

	for i := 1; i < 64; i++ {
		if block[i] == 0 {
			runLength++
			continue
		}

		// Write ZRL symbols for runs > 15 zeros
		for runLength > 15 {
			code, size := acTable.Encode(0xF0) // ZRL symbol
			if err := e.bitWriter.WriteBits(uint32(code), int(size)); err != nil {
				return err
			}
			runLength -= 16
		}

		// Encode the non-zero coefficient
		runSize, additionalBits, nBits := jpeg.EncodeACValue(runLength, block[i])

		// Write AC Huffman code for run/size
		code, size := acTable.Encode(runSize)
		if err := e.bitWriter.WriteBits(uint32(code), int(size)); err != nil {
			return err
		}

		// Write additional bits
		if nBits > 0 {
			if err := e.bitWriter.WriteBits(uint32(additionalBits), nBits); err != nil {
				return err
			}
		}

		runLength = 0
	}

	// Write EOB if we ended with zeros
	if runLength > 0 {
		code, size := acTable.Encode(0x00) // EOB symbol
		if err := e.bitWriter.WriteBits(uint32(code), int(size)); err != nil {
			return err
		}
	}

	return nil
}

// Flush finalizes the entropy-coded data by flushing the underlying BitWriter.
// Per ITU-T T.81, this pads any partial byte with 1-bits.
func (e *HuffmanBlockEncoder) Flush() error {
	return e.bitWriter.Flush()
}

// Compile-time interface compliance check
var _ BlockEncoder = (*HuffmanBlockEncoder)(nil)
