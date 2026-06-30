// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements a James R. Weeks-compatible BitWriter that exactly matches
// the bit buffer layout of Huffman.java's bufferIt method.

package weeksjpegencoder

import (
	"io"
)

// JamesBitWriter implements bit-level writing matching f5.jar's Huffman.java.
//
// The Java encoder uses a specific bit buffer layout:
//   - Bits are accumulated starting at bit position 23 and going downward.
//   - Bytes are extracted from bits 16-23 (buffer >> 16).
//   - This differs from typical implementations that extract from bits 24-31.
//
// Two flush quirks of f5.jar that we replicate:
//   - The final partial byte is padded with 0s (not 1s as in standard JPEG).
//   - The final partial byte is NOT subject to 0xFF→0x00 byte stuffing
//     (Huffman.flushBuffer in f5.jar omits the stuffing branch for the
//     trailing-byte write).
type JamesBitWriter struct {
	w         io.Writer
	putBuffer int   // Bit buffer matching Java's bufferPutBuffer
	putBits   int   // Number of valid bits, matching Java's bufferPutBits
	err       error // Sticky error
}

// NewJamesBitWriter creates a new JamesBitWriter.
//
//goland:noinspection GoUnusedExportedFunction
func NewJamesBitWriter(w io.Writer) *JamesBitWriter {
	return &JamesBitWriter{w: w}
}

// WriteBits writes n bits matching Java's Huffman.bufferIt exactly.
// This is a direct port of the Java algorithm to ensure byte-identical output.
func (bw *JamesBitWriter) WriteBits(bits uint32, nBits int) error {
	if bw.err != nil {
		return bw.err
	}

	if nBits <= 0 || nBits > 24 {
		return nil
	}

	// Direct port of Java's bufferIt method:
	//
	// void bufferIt(BufferedOutputStream out, int code, int size) {
	//     int putBuffer = code;
	//     int putBits = bufferPutBits;
	//     putBuffer &= (1 << size) - 1;
	//     putBits += size;
	//     putBuffer <<= 24 - putBits;
	//     putBuffer |= bufferPutBuffer;
	//     while (putBits >= 8) {
	//         int c = ((putBuffer >> 16) & 0xFF);
	//         out.write(c);
	//         if (c == 0xFF) out.write(0);
	//         putBuffer <<= 8;
	//         putBits -= 8;
	//     }
	//     bufferPutBuffer = putBuffer;
	//     bufferPutBits = putBits;
	// }

	putBuffer := int(bits)
	putBits := bw.putBits

	// Mask to only the bits we want
	putBuffer &= (1 << nBits) - 1

	// Add new bit count to existing
	putBits += nBits

	// Shift new bits to proper position (below existing bits)
	putBuffer <<= 24 - putBits

	// Combine with existing buffer
	putBuffer |= bw.putBuffer

	// Write out complete bytes from bits 16-23
	for putBits >= 8 {
		c := byte((putBuffer >> 16) & 0xFF)
		if err := bw.writeByte(c); err != nil {
			bw.err = err
			return err
		}
		putBuffer <<= 8
		putBits -= 8
	}

	bw.putBuffer = putBuffer
	bw.putBits = putBits

	return nil
}

// writeByte writes a byte to the output, inserting 0x00 after 0xFF per JPEG spec.
func (bw *JamesBitWriter) writeByte(b byte) error {
	_, err := bw.w.Write([]byte{b})
	if err != nil {
		return err
	}
	// Byte stuffing: insert 0x00 after 0xFF
	if b == 0xFF {
		_, err = bw.w.Write([]byte{0x00})
	}
	return err
}

// Flush writes any remaining bits.
// This matches Java's Huffman.flushBuffer which does NOT pad with 1s.
//
// Java's flushBuffer:
//
//	while (putBits >= 8) { write byte from bits 16-23; shift << 8; putBits -= 8; }
//	if (putBits > 0) { write byte from bits 16-23; }
func (bw *JamesBitWriter) Flush() error {
	if bw.err != nil {
		return bw.err
	}

	putBuffer := bw.putBuffer
	putBits := bw.putBits

	// Write out any complete bytes
	for putBits >= 8 {
		c := byte((putBuffer >> 16) & 0xFF)
		if err := bw.writeByte(c); err != nil {
			bw.err = err
			return err
		}
		putBuffer <<= 8
		putBits -= 8
	}

	// Write remaining partial byte. f5.jar's Huffman.flushBuffer does NOT
	// pad with 1s and does NOT apply 0xFF byte-stuffing on this final byte
	// (Huffman.java line 174 onward), so we go straight to the underlying
	// writer rather than through writeByte.
	if putBits > 0 {
		c := byte((putBuffer >> 16) & 0xFF)
		if _, err := bw.w.Write([]byte{c}); err != nil {
			bw.err = err
			return err
		}
	}

	bw.putBuffer = 0
	bw.putBits = 0

	return nil
}

// Compile-time interface compliance check
var _ BitWriter = (*JamesBitWriter)(nil)
