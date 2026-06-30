// Package weeksjpegencoder provides f5.jar-compatible JPEG encoding.
//
// This file replicates f5.jar's COM (comment) marker writer, which has a
// well-known length-field bug we have to mirror for byte-identical output.
//
// In f5.jar's james.JpegEncoder.WriteHeaders:
//
//	length = comment.length();
//	COM[2] = (byte) (length >> 8 & 0xFF);
//	COM[3] = (byte) (length & 0xFF);
//
// then WriteArray writes `length + 2` bytes total (2 marker + 2 length-field +
// (length - 2) bytes of comment data). Per the JPEG spec the length field is
// supposed to be `len(comment) + 2`, but f5.jar stores `len(comment)` instead,
// so the last two characters of any caller-supplied comment are silently
// truncated from the file. We reproduce that quirk so a Go caller passing the
// exact same comment string produces byte-identical bytes to f5.jar.

package weeksjpegencoder

import (
	"encoding/binary"
	"io"
)

// writeJamesCOM writes a COM marker exactly matching f5.jar's buggy length
// encoding. comment shorter than 2 bytes is omitted entirely (Java's branch
// `if (length != 0) { ... }` plus the implicit `length - 2` truncation would
// underflow otherwise).
func writeJamesCOM(w io.Writer, comment string) error {
	commentBytes := []byte(comment)
	n := len(commentBytes)
	if n < 2 {
		return nil
	}
	// JPEG spec caps the length field at 0xFFFF (a 16-bit field). f5.jar has
	// no guard for this so longer comments would wrap, but we refuse to
	// emit anything that decoders couldn't parse.
	if n > 0xFFFF {
		n = 0xFFFF
		commentBytes = commentBytes[:n]
	}
	if _, err := w.Write([]byte{0xFF, 0xFE}); err != nil {
		return err
	}
	// Length field carries Java's pre-bug value: comment.length() (not +2).
	if err := binary.Write(w, binary.BigEndian, uint16(n)); err != nil {
		return err
	}
	// f5.jar's WriteArray emits length+2 bytes total = marker(2) + len(2) +
	// payload(n-2). The trailing two bytes of the caller's comment never
	// reach disk.
	if _, err := w.Write(commentBytes[:n-2]); err != nil {
		return err
	}
	return nil
}
