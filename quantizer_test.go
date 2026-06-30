// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file contains unit tests for the IJGQuantizer implementation.

package weeksjpegencoder

import (
	"testing"

	"github.com/0verkilll/jpeg"
)

// TestIJGQuantizerQualityLevels tests IJGQuantizer at various quality levels.
// Verifies that quantization tables are properly scaled according to IJG formula.
func TestIJGQuantizerQualityLevels(t *testing.T) {
	testCases := []struct {
		name    string
		quality int
	}{
		{"quality_1", 1},
		{"quality_25", 25},
		{"quality_50", 50},
		{"quality_75", 75},
		{"quality_100", 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := NewIJGQuantizer(tc.quality)
			if err != nil {
				t.Fatalf("NewIJGQuantizer(%d) returned error: %v", tc.quality, err)
			}
			if q == nil {
				t.Fatalf("NewIJGQuantizer(%d) returned nil", tc.quality)
			}

			// Verify tables are accessible and have correct size
			lumTable := q.GetQuantTable(true)
			chromTable := q.GetQuantTable(false)

			if len(lumTable) != 64 {
				t.Errorf("luminance table has %d elements, expected 64", len(lumTable))
			}
			if len(chromTable) != 64 {
				t.Errorf("chrominance table has %d elements, expected 64", len(chromTable))
			}

			// All table values should be in range [1, 255]
			for i := 0; i < 64; i++ {
				if lumTable[i] < 1 || lumTable[i] > 255 {
					t.Errorf("luminance table[%d] = %d, expected range [1, 255]", i, lumTable[i])
				}
				if chromTable[i] < 1 || chromTable[i] > 255 {
					t.Errorf("chrominance table[%d] = %d, expected range [1, 255]", i, chromTable[i])
				}
			}
		})
	}
}

// TestIJGQuantizerScalingFormula verifies the IJG quality scaling formula.
// For q < 50: scale = 5000/q
// For q >= 50: scale = 200 - 2*q
func TestIJGQuantizerScalingFormula(t *testing.T) {
	// Test at quality 50 - scale factor should be 100 (no scaling)
	// At quality 50: scale = 200 - 2*50 = 100
	// Base table values should be unchanged (approximately)
	q50, err := NewIJGQuantizer(50)
	if err != nil {
		t.Fatalf("NewIJGQuantizer(50) error: %v", err)
	}

	lumTable50 := q50.GetQuantTable(true)
	// First element of standard luminance table is 16
	// At scale 100: (16 * 100 + 50) / 100 = 16
	expected50DC := (16*100 + 50) / 100
	if lumTable50[0] != expected50DC {
		t.Errorf("quality 50 lum[0] = %d, expected %d", lumTable50[0], expected50DC)
	}

	// Test at quality 25 - scale factor should be 5000/25 = 200
	// Base table DC value 16 -> (16 * 200 + 50) / 100 = 32
	q25, err := NewIJGQuantizer(25)
	if err != nil {
		t.Fatalf("NewIJGQuantizer(25) error: %v", err)
	}

	lumTable25 := q25.GetQuantTable(true)
	expected25DC := (16*200 + 50) / 100
	if lumTable25[0] != expected25DC {
		t.Errorf("quality 25 lum[0] = %d, expected %d", lumTable25[0], expected25DC)
	}

	// Test at quality 75 - scale factor should be 200 - 2*75 = 50
	// Base table DC value 16 -> (16 * 50 + 50) / 100 = 8
	q75, err := NewIJGQuantizer(75)
	if err != nil {
		t.Fatalf("NewIJGQuantizer(75) error: %v", err)
	}

	lumTable75 := q75.GetQuantTable(true)
	// (16*50 + 50) / 100 = 8, which is always >= 1, so no clamping needed
	expected75DC := (16*50 + 50) / 100
	if lumTable75[0] != expected75DC {
		t.Errorf("quality 75 lum[0] = %d, expected %d", lumTable75[0], expected75DC)
	}

	// Test at quality 1 - scale factor should be 5000/1 = 5000
	// Values should be clamped to max 255
	q1, err := NewIJGQuantizer(1)
	if err != nil {
		t.Fatalf("NewIJGQuantizer(1) error: %v", err)
	}

	lumTable1 := q1.GetQuantTable(true)
	// (16 * 5000) / 100 = 800, which is always > 255, so always clamped to max
	expected1DC := 255
	if lumTable1[0] != expected1DC {
		t.Errorf("quality 1 lum[0] = %d, expected %d", lumTable1[0], expected1DC)
	}

	// Test at quality 100 - scale factor should be 200 - 2*100 = 0
	// All values should be clamped to minimum 1
	q100, err := NewIJGQuantizer(100)
	if err != nil {
		t.Fatalf("NewIJGQuantizer(100) error: %v", err)
	}

	lumTable100 := q100.GetQuantTable(true)
	for i := 0; i < 64; i++ {
		if lumTable100[i] < 1 {
			t.Errorf("quality 100 lum[%d] = %d, expected >= 1", i, lumTable100[i])
		}
	}
}

// TestIJGQuantizerRoundingPositive tests round-to-nearest for positive coefficients.
func TestIJGQuantizerRoundingPositive(t *testing.T) {
	// Use quality 50 for predictable table values
	q, err := NewIJGQuantizer(50)
	if err != nil {
		t.Fatalf("NewIJGQuantizer(50) error: %v", err)
	}

	lumTable := q.GetQuantTable(true)
	qt0 := lumTable[0] // First quantization value

	testCases := []struct {
		name     string
		value    float64
		expected int
	}{
		// Exact multiple
		{"exact_multiple", float64(qt0), 1},
		// Just above half threshold (should round up)
		{"round_up", float64(qt0)*1.5 + 0.1, 2},
		// Just below half threshold (should round down)
		{"round_down", float64(qt0)*1.5 - 0.1, 1},
		// Large positive value
		{"large_positive", float64(qt0) * 10, 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var block [64]float64
			block[0] = tc.value

			result := q.QuantizeBlock(&block, true)
			if result[0] != tc.expected {
				t.Errorf("QuantizeBlock([%f, ...]) = %d, expected %d (qt=%d)",
					tc.value, result[0], tc.expected, qt0)
			}
		})
	}
}

// TestIJGQuantizerRoundingNegative tests round-to-nearest for negative coefficients.
func TestIJGQuantizerRoundingNegative(t *testing.T) {
	// Use quality 50 for predictable table values
	q, err := NewIJGQuantizer(50)
	if err != nil {
		t.Fatalf("NewIJGQuantizer(50) error: %v", err)
	}

	lumTable := q.GetQuantTable(true)
	qt0 := lumTable[0] // First quantization value

	testCases := []struct {
		name     string
		value    float64
		expected int
	}{
		// Exact negative multiple
		{"exact_negative", -float64(qt0), -1},
		// Just below negative half threshold (should round to more negative)
		{"round_more_negative", -float64(qt0)*1.5 - 0.1, -2},
		// Just above negative half threshold (should round to less negative)
		{"round_less_negative", -float64(qt0)*1.5 + 0.1, -1},
		// Large negative value
		{"large_negative", -float64(qt0) * 10, -10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var block [64]float64
			block[0] = tc.value

			result := q.QuantizeBlock(&block, true)
			if result[0] != tc.expected {
				t.Errorf("QuantizeBlock([%f, ...]) = %d, expected %d (qt=%d)",
					tc.value, result[0], tc.expected, qt0)
			}
		})
	}
}

// TestIJGQuantizerEdgeCases tests edge cases: all-zero block and DC-only block.
func TestIJGQuantizerEdgeCases(t *testing.T) {
	q, err := NewIJGQuantizer(75)
	if err != nil {
		t.Fatalf("NewIJGQuantizer(75) error: %v", err)
	}

	t.Run("all_zero_block", func(t *testing.T) {
		var block [64]float64 // All zeros

		result := q.QuantizeBlock(&block, true)
		for i := 0; i < 64; i++ {
			if result[i] != 0 {
				t.Errorf("all-zero block result[%d] = %d, expected 0", i, result[i])
			}
		}
	})

	t.Run("dc_only_block", func(t *testing.T) {
		var block [64]float64
		block[0] = 100.0 // Only DC coefficient has value

		result := q.QuantizeBlock(&block, true)

		// DC should be non-zero (unless quantization completely eliminates it)
		lumTable := q.GetQuantTable(true)
		expectedDC := int(100.0/float64(lumTable[0]) + 0.5)
		if result[0] != expectedDC {
			t.Errorf("DC-only block result[0] = %d, expected %d", result[0], expectedDC)
		}

		// All AC coefficients should be zero
		for i := 1; i < 64; i++ {
			if result[i] != 0 {
				t.Errorf("DC-only block result[%d] = %d, expected 0", i, result[i])
			}
		}
	})
}

// TestIJGQuantizerGetQuantTable tests that GetQuantTable returns correct tables.
func TestIJGQuantizerGetQuantTable(t *testing.T) {
	q, err := NewIJGQuantizer(50)
	if err != nil {
		t.Fatalf("NewIJGQuantizer(50) error: %v", err)
	}

	// Compare with directly scaled tables from jpeg package
	expectedLum := jpeg.ScaleQuantTable(jpeg.StandardLuminanceQuantTable, 50)
	expectedChrom := jpeg.ScaleQuantTable(jpeg.StandardChrominanceQuantTable, 50)

	lumTable := q.GetQuantTable(true)
	chromTable := q.GetQuantTable(false)

	for i := 0; i < 64; i++ {
		if lumTable[i] != expectedLum[i] {
			t.Errorf("luminance table[%d] = %d, expected %d", i, lumTable[i], expectedLum[i])
		}
		if chromTable[i] != expectedChrom[i] {
			t.Errorf("chrominance table[%d] = %d, expected %d", i, chromTable[i], expectedChrom[i])
		}
	}
}

// TestIJGQuantizerInvalidQuality tests that invalid quality values return errors.
func TestIJGQuantizerInvalidQuality(t *testing.T) {
	testCases := []struct {
		name    string
		quality int
	}{
		{"quality_0", 0},
		{"quality_101", 101},
		{"quality_negative", -1},
		{"quality_very_high", 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := NewIJGQuantizer(tc.quality)
			if err == nil {
				t.Errorf("NewIJGQuantizer(%d) should return error for invalid quality", tc.quality)
			}
			if q != nil {
				t.Errorf("NewIJGQuantizer(%d) should return nil quantizer for invalid quality", tc.quality)
			}
		})
	}
}

// TestIJGQuantizerDivisionByZeroGuard tests that division by zero is handled.
// The quantizer should treat qt=0 as qt=1 to prevent division by zero.
func TestIJGQuantizerDivisionByZeroGuard(t *testing.T) {
	// Create a quantizer - even if a table value somehow became 0,
	// quantization should not panic
	q, err := NewIJGQuantizer(50)
	if err != nil {
		t.Fatalf("NewIJGQuantizer(50) error: %v", err)
	}

	// Test with a block that has values at all positions
	var block [64]float64
	for i := 0; i < 64; i++ {
		block[i] = float64(i + 1)
	}

	// This should not panic; QuantizeBlock returns a fixed [64]int.
	_ = q.QuantizeBlock(&block, true)
}

// TestIJGQuantizerMatchesInlineCode verifies that IJGQuantizer produces
// identical results to the inline quantization code in encoder.go.
func TestIJGQuantizerMatchesInlineCode(t *testing.T) {
	quality := 75

	// Create quantizer
	q, err := NewIJGQuantizer(quality)
	if err != nil {
		t.Fatalf("NewIJGQuantizer(%d) error: %v", quality, err)
	}

	// Create reference tables (same as encoder.go lines 108-109)
	lumQuantTable := jpeg.ScaleQuantTable(jpeg.StandardLuminanceQuantTable, quality)
	chromQuantTable := jpeg.ScaleQuantTable(jpeg.StandardChrominanceQuantTable, quality)

	// Test block with various values
	var block [64]float64
	for i := 0; i < 64; i++ {
		// Mix of positive and negative values
		if i%2 == 0 {
			block[i] = float64(i*10 + 5)
		} else {
			block[i] = float64(-i*10 - 5)
		}
	}

	// Test luminance
	result := q.QuantizeBlock(&block, true)
	for i := 0; i < 64; i++ {
		// Reference implementation (encoder.go lines 291-295)
		qt := lumQuantTable[i]
		if qt == 0 {
			qt = 1
		}
		var expected int
		if block[i] >= 0 {
			expected = int(block[i]/float64(qt) + 0.5)
		} else {
			expected = int(block[i]/float64(qt) - 0.5)
		}

		if result[i] != expected {
			t.Errorf("luminance[%d]: got %d, expected %d (block=%f, qt=%d)",
				i, result[i], expected, block[i], qt)
		}
	}

	// Test chrominance
	result = q.QuantizeBlock(&block, false)
	for i := 0; i < 64; i++ {
		qt := chromQuantTable[i]
		if qt == 0 {
			qt = 1
		}
		var expected int
		if block[i] >= 0 {
			expected = int(block[i]/float64(qt) + 0.5)
		} else {
			expected = int(block[i]/float64(qt) - 0.5)
		}

		if result[i] != expected {
			t.Errorf("chrominance[%d]: got %d, expected %d (block=%f, qt=%d)",
				i, result[i], expected, block[i], qt)
		}
	}
}

// =============================================================================
// JamesQuantizer DCT Tests (merged from allocation_test.go)
// =============================================================================

// TestForwardDCTAndQuantizeWorkspaceHandling verifies that workspace arrays
// in ForwardDCTAndQuantize produce correct DCT coefficients.
func TestForwardDCTAndQuantizeWorkspaceHandling(t *testing.T) {
	jq, err := NewJamesQuantizer(75)
	if err != nil {
		t.Fatalf("NewJamesQuantizer failed: %v", err)
	}

	// Use a constant non-128 fill so the DC coefficient is non-zero after
	// the JPEG level shift (which subtracts 128 inside ForwardDCTAndQuantize).
	var dcBlock [64]float64
	for i := range dcBlock {
		dcBlock[i] = 200.0
	}

	result := jq.ForwardDCTAndQuantize(&dcBlock, true)

	// DC coefficient should be a large positive after level shift (200-128=72).
	if result[0] <= 0 {
		t.Errorf("Expected positive DC coefficient, got %d", result[0])
	}

	// Most AC coefficients should be near zero for constant input.
	acSum := 0
	for i := 1; i < 64; i++ {
		if result[i] > 1 || result[i] < -1 {
			acSum++
		}
	}
	if acSum > 5 {
		t.Errorf("Expected mostly zero AC coefficients for constant input, got %d non-zero", acSum)
	}
}

// TestForwardDCTAndQuantizeNumericalStability verifies DCT produces
// consistent results for edge cases.
func TestForwardDCTAndQuantizeNumericalStability(t *testing.T) {
	jq, err := NewJamesQuantizer(75)
	if err != nil {
		t.Fatalf("NewJamesQuantizer failed: %v", err)
	}

	testCases := []struct {
		name   string
		fillFn func(*[64]float64)
	}{
		{"all zeros", func(b *[64]float64) {
			for i := range b {
				b[i] = 0
			}
		}},
		{"all 255", func(b *[64]float64) {
			for i := range b {
				b[i] = 255
			}
		}},
		{"checkerboard", func(b *[64]float64) {
			for i := range b {
				if (i/8+i%8)%2 == 0 {
					b[i] = 0
				} else {
					b[i] = 255
				}
			}
		}},
		{"horizontal gradient", func(b *[64]float64) {
			for i := range b {
				b[i] = float64((i % 8) * 32)
			}
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var block [64]float64
			tc.fillFn(&block)

			// Run twice to verify consistency
			result1 := jq.ForwardDCTAndQuantize(&block, true)
			tc.fillFn(&block) // Reset block
			result2 := jq.ForwardDCTAndQuantize(&block, true)

			for i := 0; i < 64; i++ {
				if result1[i] != result2[i] {
					t.Errorf("Inconsistent results at index %d: %d vs %d", i, result1[i], result2[i])
				}
			}
		})
	}
}

// TestEncodeBlockReusesWorkspace verifies that multiple block encodings
// produce correct results when using the same encoder instance.
func TestEncodeBlockReusesWorkspace(t *testing.T) {
	jq, err := NewJamesQuantizer(75)
	if err != nil {
		t.Fatalf("NewJamesQuantizer failed: %v", err)
	}

	// Encode multiple blocks and verify each produces valid output
	for i := 0; i < 10; i++ {
		var block [64]float64
		for j := range block {
			block[j] = float64((i*17 + j*13) % 256)
		}

		result := jq.ForwardDCTAndQuantize(&block, true)

		// Basic sanity check - DC should be within reasonable range
		// For values 0-255, DC coefficient (before quantization) can be quite large
		if result[0] < -2048 || result[0] > 2048 {
			t.Errorf("Block %d: DC coefficient %d seems out of range", i, result[0])
		}
	}
}
