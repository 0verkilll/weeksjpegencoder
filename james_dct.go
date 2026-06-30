// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements the James R. Weeks-compatible DCT and quantization.
// The original JpegEncoder.java uses the AAN (Arai, Agui, Nakajima) algorithm
// with integrated quantization through divisor tables.

package weeksjpegencoder

import (
	"math"

	"github.com/0verkilll/jpeg"
)

// javaRound implements Java's Math.round() rounding behavior.
// Java's Math.round(x) = (long)Math.floor(x + 0.5) which means:
//   - For positive values at .5 boundary: rounds up (same as Go)
//   - For negative values at .5 boundary: rounds towards positive infinity
//     e.g., Java: round(-2.5) = -2, Go: Round(-2.5) = -3
//
// This difference matters for byte-exact compatibility with JpegEncoder.java.
func javaRound(x float64) int {
	return int(math.Floor(x + 0.5))
}

// =============================================================================
// James R. Weeks DCT Constants
// =============================================================================

// AAN DCT constants matching f5.jar's DCT.java verbatim (truncated to the
// same number of digits Java holds in its source so floating-point rounding
// is identical between the two implementations).
const (
	jamesRot1 = 0.707106781 // cos(PI/4) = 1/sqrt(2)
	jamesRot2 = 0.382683433 // sin(PI/8)
	jamesRot3 = 0.541196100 // 2*cos(3*PI/8)
	jamesRot4 = 1.306562965 // 2*sin(3*PI/8)
)

// AAN scale factors for post-DCT scaling
var aanScaleFactor = [8]float64{
	1.0, 1.387039845, 1.306562965, 1.175875602,
	1.0, 0.785694958, 0.541196100, 0.275899379,
}

// =============================================================================
// JamesQuantizer - James R. Weeks Compatible Quantizer
// =============================================================================

// JamesQuantizer implements the combined DCT+quantization approach
// used by the original James R. Weeks JpegEncoder.java.
//
// The key difference from standard JPEG implementations is that the
// AAN scale factors are incorporated into the divisor tables, which
// means the DCT output is multiplied by the divisor (which includes
// both the quantization value and AAN descaling factors).
//
// The standard JPEG level shift (subtract 128) is applied inside
// ForwardDCTAndQuantize, matching f5.jar's DCT.forwardDCT (DCT.java line 78).
type JamesQuantizer struct {
	// quantTables stores the scaled quantization tables
	quantTables [2][64]int

	// divisors stores 1.0 / (quantTable * aanScale[row] * aanScale[col] * 8.0)
	divisors [2][64]float64
}

// NewJamesQuantizer creates a new James R. Weeks-compatible quantizer.
//
// Parameters:
//   - quality: Quality level from 1 to 100 (same as libjpeg)
//
// This quantizer matches the exact quantization behavior of JpegEncoder.java,
// including the integrated AAN scale factors.
//
//goland:noinspection GoUnusedExportedFunction
func NewJamesQuantizer(quality int) (*JamesQuantizer, error) {
	if quality < 1 || quality > 100 {
		return nil, &jpeg.ValidationError{
			Field:   "quality",
			Value:   quality,
			Message: "quality must be between 1 and 100",
		}
	}

	jq := &JamesQuantizer{}

	// IJG quality scaling formula
	var scale int
	if quality < 50 {
		scale = 5000 / quality
	} else {
		scale = 200 - quality*2
	}

	// Initialize luminance quantization table
	for i := 0; i < 64; i++ {
		temp := (jpeg.StandardLuminanceQuantTable[i]*scale + 50) / 100
		if temp <= 0 {
			temp = 1
		}
		if temp > 255 {
			temp = 255
		}
		jq.quantTables[0][i] = temp
	}

	// Initialize chrominance quantization table
	for i := 0; i < 64; i++ {
		temp := (jpeg.StandardChrominanceQuantTable[i]*scale + 50) / 100
		if temp <= 0 {
			temp = 1
		}
		if temp > 255 {
			temp = 255
		}
		jq.quantTables[1][i] = temp
	}

	// Initialize divisors for AAN DCT (matches DCT.java exactly)
	// divisor = 1.0 / (quantTable * aanScale[row] * aanScale[col] * 8.0)
	for i := 0; i < 64; i++ {
		row := i / 8
		col := i % 8
		jq.divisors[0][i] = 1.0 / (float64(jq.quantTables[0][i]) * aanScaleFactor[row] * aanScaleFactor[col] * 8.0)
		jq.divisors[1][i] = 1.0 / (float64(jq.quantTables[1][i]) * aanScaleFactor[row] * aanScaleFactor[col] * 8.0)
	}

	return jq, nil
}

// ForwardDCTAndQuantize performs the forward DCT using the AAN algorithm
// and quantizes the result in one step, matching f5.jar's DCT.forwardDCT.
//
// Parameters:
//   - input: 64-element block in row-major order (raw pixel values in [0, 255])
//   - isLuminance: true for Y component, false for Cb/Cr components
//
// The function applies the standard JPEG level shift (subtract 128) internally
// before running AAN — matching f5.jar's DCT.java line 78.
//
// Returns 64 quantized integer coefficients in row-major order (NOT zigzag).
//
// Performance Note (DEC-005): CPU profiling shows this function accounts for
// ~13% of total encoding time. All local arrays (output, workspace, in, result)
// are stack-allocated and do NOT escape to heap. Further optimization would
// require SIMD which is out of scope. See decisions.md for full profiling data.
func (jq *JamesQuantizer) ForwardDCTAndQuantize(input *[64]float64, isLuminance bool) [64]int {
	var output [64]float64
	var workspace [8][8]float64

	// Copy input to 2D array, applying the JPEG level shift: subtract 128 so
	// that values land in [-128, 127]. f5.jar's DCT.forwardDCT does this on
	// line 78 of DCT.java; matching it is required for byte-identical output.
	var in [8][8]float64
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			in[i][j] = input[i*8+j] - 128.0
		}
	}

	// Process rows — direct port of f5.jar DCT.forwardDCT (DCT.java:67+).
	// Variable names mirror the Java source (tmp0..tmp7, tmp10..tmp13, z*)
	// so that floating-point evaluation order matches as closely as the
	// language allows.
	for i := 0; i < 8; i++ {
		tmp0 := in[i][0] + in[i][7]
		tmp7 := in[i][0] - in[i][7]
		tmp1 := in[i][1] + in[i][6]
		tmp6 := in[i][1] - in[i][6]
		tmp2 := in[i][2] + in[i][5]
		tmp5 := in[i][2] - in[i][5]
		tmp3 := in[i][3] + in[i][4]
		tmp4 := in[i][3] - in[i][4]

		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2

		workspace[i][0] = tmp10 + tmp11
		workspace[i][4] = tmp10 - tmp11

		z1 := (tmp12 + tmp13) * jamesRot1
		workspace[i][2] = tmp13 + z1
		workspace[i][6] = tmp13 - z1

		tmp10 = tmp4 + tmp5
		tmp11 = tmp5 + tmp6
		tmp12 = tmp6 + tmp7

		z5 := (tmp10 - tmp12) * jamesRot2
		z2 := jamesRot3*tmp10 + z5
		z4 := jamesRot4*tmp12 + z5
		z3 := tmp11 * jamesRot1

		z11 := tmp7 + z3
		z13 := tmp7 - z3

		workspace[i][5] = z13 + z2
		workspace[i][3] = z13 - z2
		workspace[i][1] = z11 + z4
		workspace[i][7] = z11 - z4
	}

	// Process columns — same algorithm transposed.
	for i := 0; i < 8; i++ {
		tmp0 := workspace[0][i] + workspace[7][i]
		tmp7 := workspace[0][i] - workspace[7][i]
		tmp1 := workspace[1][i] + workspace[6][i]
		tmp6 := workspace[1][i] - workspace[6][i]
		tmp2 := workspace[2][i] + workspace[5][i]
		tmp5 := workspace[2][i] - workspace[5][i]
		tmp3 := workspace[3][i] + workspace[4][i]
		tmp4 := workspace[3][i] - workspace[4][i]

		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2

		output[0*8+i] = tmp10 + tmp11
		output[4*8+i] = tmp10 - tmp11

		z1 := (tmp12 + tmp13) * jamesRot1
		output[2*8+i] = tmp13 + z1
		output[6*8+i] = tmp13 - z1

		tmp10 = tmp4 + tmp5
		tmp11 = tmp5 + tmp6
		tmp12 = tmp6 + tmp7

		z5 := (tmp10 - tmp12) * jamesRot2
		z2 := jamesRot3*tmp10 + z5
		z4 := jamesRot4*tmp12 + z5
		z3 := tmp11 * jamesRot1

		z11 := tmp7 + z3
		z13 := tmp7 - z3

		output[5*8+i] = z13 + z2
		output[3*8+i] = z13 - z2
		output[1*8+i] = z11 + z4
		output[7*8+i] = z11 - z4
	}

	// Quantize (matches DCT.java quantizeBlock exactly)
	var result [64]int
	tableNum := 0
	if !isLuminance {
		tableNum = 1
	}

	for i := 0; i < 64; i++ {
		temp := output[i] * jq.divisors[tableNum][i]
		result[i] = javaRound(temp)
	}

	return result
}

// GetQuantTable returns the quantization table for the specified component.
// Values are in row-major order (not zigzag).
func (jq *JamesQuantizer) GetQuantTable(isLuminance bool) [64]int {
	if isLuminance {
		return jq.quantTables[0]
	}
	return jq.quantTables[1]
}

// QuantizeBlock implements the Quantizer interface but should not be used
// directly when byte-identical output is needed. Use ForwardDCTAndQuantize
// instead to get the integrated DCT+quantization behavior.
func (jq *JamesQuantizer) QuantizeBlock(block *[64]float64, isLuminance bool) [64]int {
	// This implementation assumes the input has already been DCT transformed
	// using a standard DCT. For byte-identical output with Java, use
	// ForwardDCTAndQuantize which does both steps together.
	var result [64]int
	tableNum := 0
	if !isLuminance {
		tableNum = 1
	}

	for i := 0; i < 64; i++ {
		// Standard quantization without AAN integration
		qt := jq.quantTables[tableNum][i]
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

// Compile-time interface compliance check
var _ Quantizer = (*JamesQuantizer)(nil)
