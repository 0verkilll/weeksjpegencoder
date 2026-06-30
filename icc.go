// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements ICC color profile parsing and preservation functionality.
// ICC profiles are stored in APP2 markers (0xFFE2) and can be preserved from source
// images during re-encoding. Large profiles (>64KB) are split across multiple
// APP2 marker segments.

package weeksjpegencoder

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

// ICC profile constants
const (
	// Maximum data per ICC segment (excluding marker, length, signature, and sequence bytes)
	// Segment max = 65535 (max length field) - 2 (length field) - 12 (signature) - 2 (sequence)
	iccMaxSegmentData = 65535 - 2 - 12 - 2 // = 65519 bytes of actual ICC data per segment
)

// iccSignature is the ICC_PROFILE identifier that appears at the start of APP2 data.
// Per ICC spec: "ICC_PROFILE\x00" (12 bytes)
var iccSignature = []byte("ICC_PROFILE\x00")

// ErrNoICC is returned when no ICC profile is found in the source image.
var ErrNoICC = errors.New("no ICC profile found in source image")

// ErrInvalidICCProfile is returned when ICC profile data is malformed.
var ErrInvalidICCProfile = errors.New("invalid ICC profile data")

// ParseICCProfile extracts ICC profile data from a JPEG source.
//
// It scans for APP2 markers (0xFFE2) with ICC_PROFILE signature and reassembles
// multi-segment profiles (profiles >64KB span multiple APP2 markers with sequence
// numbers).
//
// Returns ErrNoICC if no ICC profile is found.
// Returns ErrInvalidJPEG if the source is not valid JPEG data.
// Returns ErrInvalidICCProfile if ICC segments are malformed or incomplete.
//
// Example:
//
//	f, _ := os.Open("photo.jpg")
//	defer f.Close()
//	iccData, err := ParseICCProfile(f)
//	if err == ErrNoICC {
//	    // No ICC profile in source - this is normal
//	}
//
//goland:noinspection GoUnusedExportedFunction
func ParseICCProfile(r io.Reader) ([]byte, error) {
	// Read all data into buffer for easier marker parsing
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return ParseICCProfileBytes(data)
}

// ParseICCProfileBytes extracts ICC profile data from JPEG bytes.
//
// This is a convenience function that works with byte slices instead of readers.
// See ParseICCProfile for full documentation.
func ParseICCProfileBytes(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, ErrInvalidJPEG
	}

	// Check SOI marker
	if data[0] != 0xFF || data[1] != 0xD8 {
		return nil, ErrInvalidJPEG
	}

	// Collect ICC segments
	// Key: sequence number (1-based), Value: segment data
	segments := make(map[int][]byte)
	expectedCount := 0

	// Scan for APP2 markers with ICC signature
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
			break
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

		// Check if this is APP2
		if markerType == 0xE2 {
			// Verify we have enough data
			if offset+segmentLength > len(data) {
				return nil, ErrInvalidJPEG
			}

			segmentData := data[offset+2 : offset+segmentLength]

			// Check for ICC_PROFILE signature
			if len(segmentData) >= len(iccSignature)+2 &&
				bytes.Equal(segmentData[:len(iccSignature)], iccSignature) {

				// Extract sequence number and count
				// Format after signature: sequenceNumber (1 byte), count (1 byte)
				seqNum := int(segmentData[12]) // 1-based sequence number
				count := int(segmentData[13])  // Total number of segments

				if seqNum < 1 || seqNum > count || count < 1 {
					return nil, ErrInvalidICCProfile
				}

				// Store segment data (after signature and sequence bytes)
				iccData := segmentData[14:]
				segments[seqNum] = iccData

				// Track expected count
				if expectedCount == 0 {
					expectedCount = count
				} else if expectedCount != count {
					// Inconsistent count across segments
					return nil, ErrInvalidICCProfile
				}
			}
		}

		// Move to next marker
		offset += segmentLength
	}

	// No ICC segments found
	if len(segments) == 0 {
		return nil, ErrNoICC
	}

	// Verify we have all segments
	if len(segments) != expectedCount {
		return nil, ErrInvalidICCProfile
	}

	// Reassemble profile from segments in order
	var profile bytes.Buffer
	for i := 1; i <= expectedCount; i++ {
		seg, ok := segments[i]
		if !ok {
			return nil, ErrInvalidICCProfile
		}
		profile.Write(seg)
	}

	return profile.Bytes(), nil
}

// writeAPP2ICC writes APP2 marker(s) with ICC profile data to the writer.
//
// Large profiles (>64KB) are split across multiple APP2 segments with proper
// sequence numbering.
//
// Marker format per segment:
//   - 0xFF 0xE2 (APP2 marker)
//   - 2 bytes: length (including length bytes)
//   - 12 bytes: "ICC_PROFILE\x00" signature
//   - 1 byte: sequence number (1-based)
//   - 1 byte: total segment count
//   - ICC profile data chunk
//
//goland:noinspection GoUnusedFunction
func writeAPP2ICC(w io.Writer, iccProfile []byte) error {
	if len(iccProfile) == 0 {
		return nil
	}

	// Calculate number of segments needed
	numSegments := (len(iccProfile) + iccMaxSegmentData - 1) / iccMaxSegmentData
	if numSegments > 255 {
		return errors.New("ICC profile too large: exceeds maximum 255 segments")
	}

	// Write each segment
	offset := 0
	for segNum := 1; segNum <= numSegments; segNum++ {
		// Calculate chunk size for this segment
		chunkSize := iccMaxSegmentData
		remaining := len(iccProfile) - offset
		if remaining < chunkSize {
			chunkSize = remaining
		}

		// Calculate total segment length
		// Length includes: length field (2) + signature (12) + seq/count (2) + data
		segmentLength := 2 + len(iccSignature) + 2 + chunkSize

		// Write APP2 marker
		if _, err := w.Write([]byte{0xFF, 0xE2}); err != nil {
			return err
		}

		// Write length (segmentLength is bounded by maxChunkSize which is well under uint16 max)
		// #nosec G115 - segmentLength bounded by maxChunkSize (65519) which fits in uint16
		if err := binary.Write(w, binary.BigEndian, uint16(segmentLength)); err != nil {
			return err
		}

		// Write ICC_PROFILE signature
		if _, err := w.Write(iccSignature); err != nil {
			return err
		}

		// Write sequence number and count
		if _, err := w.Write([]byte{byte(segNum), byte(numSegments)}); err != nil {
			return err
		}

		// Write ICC data chunk
		if _, err := w.Write(iccProfile[offset : offset+chunkSize]); err != nil {
			return err
		}

		offset += chunkSize
	}

	return nil
}
