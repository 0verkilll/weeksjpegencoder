// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements the IJGQuantizer, which performs DCT coefficient
// quantization using the Independent JPEG Group (IJG) quality scaling formula.

package weeksjpegencoder

import (
	"github.com/0verkilll/jpeg"
)

// =============================================================================
// IJGQuantizer Implementation
// =============================================================================

// IJGQuantizer implements the Quantizer interface using IJG quality scaling.
//
// The IJG quality formula scales the standard ITU-T T.81 quantization tables:
//   - Quality < 50: scale = 5000 / quality (higher values = more compression)
//   - Quality >= 50: scale = 200 - 2*quality (lower values = less compression)
//
// Quality 50 produces the standard tables unchanged. Quality 100 produces
// the minimum quantization (highest quality), and quality 1 produces
// maximum quantization (lowest quality).
//
// Example usage:
//
//	q, err := NewIJGQuantizer(75)
//	if err != nil {
//	    return err
//	}
//	quantized := q.QuantizeBlock(&dctBlock, true) // luminance
//	table := q.GetQuantTable(true)                // for DQT marker
type IJGQuantizer struct {
	lumTable   [64]int
	chromTable [64]int
}

// NewIJGQuantizer creates a new IJGQuantizer with the specified quality.
//
// Parameters:
//   - quality: Quality level from 1 to 100 (same as libjpeg)
//   - 1: Lowest quality, highest compression
//   - 50: Standard quality (base tables unchanged)
//   - 100: Highest quality, lowest compression
//
// Returns an error if quality is outside the valid range [1, 100].
//
//goland:noinspection GoUnusedExportedFunction
func NewIJGQuantizer(quality int) (*IJGQuantizer, error) {
	if quality < 1 || quality > 100 {
		return nil, &jpeg.ValidationError{
			Field:   "quality",
			Value:   quality,
			Message: "quality must be between 1 and 100",
		}
	}

	return &IJGQuantizer{
		lumTable:   jpeg.ScaleQuantTable(jpeg.StandardLuminanceQuantTable, quality),
		chromTable: jpeg.ScaleQuantTable(jpeg.StandardChrominanceQuantTable, quality),
	}, nil
}

// QuantizeBlock quantizes DCT coefficients in a block.
//
// The quantization process divides each coefficient by its corresponding
// quantization table value and rounds to the nearest integer:
//   - Positive values: int(value/qt + 0.5)
//   - Negative values: int(value/qt - 0.5)
//
// Parameters:
//   - block: Pointer to 64 DCT coefficients (row-major, NOT zigzag order)
//   - isLuminance: true for Y component, false for Cb/Cr components
//
// Returns 64 quantized integer coefficients in the same order as input.
// The result is NOT zigzag reordered - that should be done separately.
func (q *IJGQuantizer) QuantizeBlock(block *[64]float64, isLuminance bool) [64]int {
	var result [64]int
	var table *[64]int

	if isLuminance {
		table = &q.lumTable
	} else {
		table = &q.chromTable
	}

	for i := 0; i < 64; i++ {
		qt := table[i]
		// Guard against division by zero (defensive)
		if qt == 0 {
			qt = 1
		}

		// Round to nearest integer
		if block[i] >= 0 {
			result[i] = int(block[i]/float64(qt) + 0.5)
		} else {
			result[i] = int(block[i]/float64(qt) - 0.5)
		}
	}

	return result
}

// GetQuantTable returns the quantization table for the specified component.
//
// Parameters:
//   - isLuminance: true for luminance (Y) table, false for chrominance (Cb/Cr) table
//
// Returns a copy of the 64-element quantization table. Values are in
// row-major order (not zigzag) and range from 1 to 255.
//
// This method is used for:
//   - Writing DQT (Define Quantization Table) markers
//   - Debugging and inspection
//   - Quality analysis
func (q *IJGQuantizer) GetQuantTable(isLuminance bool) [64]int {
	if isLuminance {
		return q.lumTable
	}
	return q.chromTable
}

// Compile-time interface compliance check
var _ Quantizer = (*IJGQuantizer)(nil)
