// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements EXIF metadata parsing and preservation functionality.
// EXIF data is stored in APP1 markers (0xFFE1) and can be preserved from source
// images during re-encoding.

package weeksjpegencoder

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// exifSignature is the EXIF identifier that appears at the start of APP1 data.
// Per EXIF spec: "Exif\x00\x00" (6 bytes)
var exifSignature = []byte("Exif\x00\x00")

// ErrNoEXIF is returned when no EXIF data is found in the source image.
var ErrNoEXIF = errors.New("no EXIF data found in source image")

// ErrInvalidJPEG is returned when the source data is not valid JPEG.
var ErrInvalidJPEG = errors.New("invalid JPEG data")

// ParseEXIF extracts EXIF data from a JPEG source.
//
// It scans for the APP1 marker (0xFFE1) with EXIF signature ("Exif\x00\x00")
// and returns the raw EXIF segment data (excluding marker and length bytes
// but including the EXIF signature).
//
// Returns ErrNoEXIF if no EXIF data is found.
// Returns ErrInvalidJPEG if the source is not valid JPEG data.
//
// Example:
//
//	f, _ := os.Open("photo.jpg")
//	defer f.Close()
//	exifData, err := ParseEXIF(f)
//	if err == ErrNoEXIF {
//	    // No EXIF in source - this is normal
//	}
//
//goland:noinspection GoUnusedExportedFunction
func ParseEXIF(r io.Reader) ([]byte, error) {
	// Read all data into buffer for easier marker parsing
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return ParseEXIFBytes(data)
}

// ParseEXIFBytes extracts EXIF data from JPEG bytes.
//
// This is a convenience function that works with byte slices instead of readers.
// See ParseEXIF for full documentation.
func ParseEXIFBytes(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, ErrInvalidJPEG
	}

	// Check SOI marker
	if data[0] != 0xFF || data[1] != 0xD8 {
		return nil, ErrInvalidJPEG
	}

	// Scan for APP1 marker with EXIF signature
	offset := 2
	for offset < len(data)-4 {
		// Must be a marker (0xFF followed by non-zero byte)
		if data[offset] != 0xFF {
			return nil, ErrInvalidJPEG
		}

		// Skip padding 0xFF bytes
		for offset < len(data)-1 && data[offset+1] == 0xFF {
			offset++
		}

		if offset >= len(data)-1 {
			return nil, ErrNoEXIF
		}

		markerType := data[offset+1]
		offset += 2

		// Check for markers without length
		if markerType == 0xD8 || markerType == 0xD9 { // SOI or EOI
			continue
		}
		if markerType >= 0xD0 && markerType <= 0xD7 { // RST markers
			continue
		}

		// End of markers - reached SOS
		if markerType == 0xDA { // SOS - Start of Scan
			break
		}

		// Read segment length
		if offset+2 > len(data) {
			return nil, ErrInvalidJPEG
		}
		segmentLength := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		if segmentLength < 2 {
			return nil, ErrInvalidJPEG
		}

		// Check if this is APP1
		if markerType == 0xE1 {
			// Verify the declared segment fits within the buffer before
			// slicing — a truncated or malicious APP1 length would otherwise
			// panic with slice-out-of-range.
			if offset+segmentLength > len(data) {
				return nil, ErrInvalidJPEG
			}

			// Check for EXIF signature
			segmentData := data[offset+2 : offset+segmentLength]
			if len(segmentData) >= len(exifSignature) &&
				bytes.Equal(segmentData[:len(exifSignature)], exifSignature) {
				// Found EXIF - return the data (including signature)
				return segmentData, nil
			}
		}

		// Move to next marker
		offset += segmentLength
	}

	return nil, ErrNoEXIF
}

// writeAPP1EXIF writes an APP1 marker with EXIF data to the writer.
//
// The EXIF data should include the "Exif\x00\x00" signature as obtained
// from ParseEXIF.
//
// Marker format:
//   - 0xFF 0xE1 (APP1 marker)
//   - 2 bytes: length (including length bytes, so data length + 2)
//   - EXIF data (with signature)
//
//goland:noinspection GoUnusedFunction
func writeAPP1EXIF(w io.Writer, exifData []byte) error {
	if len(exifData) == 0 {
		return nil
	}

	// The APP1 length field is a 16-bit value covering the data plus the
	// 2-byte length field itself, so the payload must fit in 65533 bytes.
	if len(exifData) > 0xFFFF-2 {
		return fmt.Errorf("EXIF data too large for a single APP1 segment: %d bytes (max %d)", len(exifData), 0xFFFF-2)
	}

	// Write APP1 marker
	if _, err := w.Write([]byte{0xFF, 0xE1}); err != nil {
		return err
	}

	// Write length (length field includes itself, so +2).
	length := uint16(len(exifData) + 2)
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return err
	}

	// Write EXIF data
	_, err := w.Write(exifData)
	return err
}
