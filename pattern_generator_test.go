// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements deterministic test pattern generation for byte-level
// compatibility testing with the Java ReferenceGenerator. All patterns are
// designed to produce pixel-identical images to the Java implementation.
//
// Pattern Types:
//   - Solid: Uniform gray (128, 128, 128)
//   - Horizontal Gradient: R varies 0-255 across width
//   - Vertical Gradient: G varies 0-255 across height
//   - Diagonal Gradient: B varies 0-255 diagonally
//   - Checkerboard: 8x8 blocks alternating black/white
//   - Quadrant: Four different patterns in each quadrant

package weeksjpegencoder

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"testing"
)

// =============================================================================
// Pattern Type Constants
// =============================================================================

// PatternType represents the type of test pattern to generate.
type PatternType int

const (
	// PatternSolid generates a uniform gray (128, 128, 128) pattern.
	// Tests DC coefficient encoding with uniform blocks.
	PatternSolid PatternType = iota

	// PatternHorizontalGradient varies R from 0 to 255 across width.
	// Tests low-frequency horizontal content.
	PatternHorizontalGradient

	// PatternVerticalGradient varies G from 0 to 255 across height.
	// Tests low-frequency vertical content.
	PatternVerticalGradient

	// PatternDiagonalGradient varies B from 0 to 255 diagonally.
	// Tests diagonal frequency content.
	PatternDiagonalGradient

	// PatternCheckerboard creates 8x8 blocks alternating black/white.
	// Tests high-frequency DCT coefficients.
	PatternCheckerboard

	// PatternQuadrant creates four different patterns in each quadrant.
	// Tests mixed content: gradient, checkerboard, stripes, and noise-like.
	PatternQuadrant
)

// PatternName returns the string name of the pattern type.
// These names match the Java ReferenceGenerator exactly.
func (p PatternType) PatternName() string {
	switch p {
	case PatternSolid:
		return "solid"
	case PatternHorizontalGradient:
		return "horizontal_gradient"
	case PatternVerticalGradient:
		return "vertical_gradient"
	case PatternDiagonalGradient:
		return "diagonal_gradient"
	case PatternCheckerboard:
		return "checkerboard"
	case PatternQuadrant:
		return "quadrant"
	default:
		return "unknown"
	}
}

// AllPatternTypes returns all pattern types for iteration.
func AllPatternTypes() []PatternType {
	return []PatternType{
		PatternSolid,
		PatternHorizontalGradient,
		PatternVerticalGradient,
		PatternDiagonalGradient,
		PatternCheckerboard,
		PatternQuadrant,
	}
}

// =============================================================================
// Test Dimensions
// =============================================================================

// TestDimension represents a width/height pair for test images.
type TestDimension struct {
	Width  int
	Height int
}

// String returns the dimension as "WxH".
func (d TestDimension) String() string {
	return fmt.Sprintf("%dx%d", d.Width, d.Height)
}

// AllTestDimensions returns all test dimensions matching Java ReferenceGenerator.
func AllTestDimensions() []TestDimension {
	return []TestDimension{
		{8, 8},     // Single MCU
		{64, 64},   // Standard
		{256, 256}, // Comprehensive
		{33, 33},   // Non-multiple of 8 (square)
		{100, 75},  // Non-multiple of 8 (rectangular)
	}
}

// =============================================================================
// Pattern Generator Functions
// =============================================================================

// GeneratePattern creates a test pattern image.
// This function produces pixel-identical output to Java ReferenceGenerator.generatePattern().
func GeneratePattern(patternType PatternType, width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	switch patternType {
	case PatternSolid:
		generateSolidPattern(img, width, height)
	case PatternHorizontalGradient:
		generateHorizontalGradient(img, width, height)
	case PatternVerticalGradient:
		generateVerticalGradient(img, width, height)
	case PatternDiagonalGradient:
		generateDiagonalGradient(img, width, height)
	case PatternCheckerboard:
		generateCheckerboard(img, width, height)
	case PatternQuadrant:
		generateQuadrant(img, width, height)
	}

	return img
}

// generateSolidPattern creates a uniform gray (128, 128, 128) pattern.
// This matches Java ReferenceGenerator.generateSolidPattern() exactly.
// Tests DC coefficient encoding with uniform blocks.
func generateSolidPattern(img *image.RGBA, width, height int) {
	gray := color.RGBA{R: 128, G: 128, B: 128, A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, gray)
		}
	}
}

// generateHorizontalGradient creates a pattern where R varies from 0 to 255 across width.
// This matches Java ReferenceGenerator.generateHorizontalGradient() exactly.
// G and B are fixed at 128.
// Tests low-frequency horizontal content.
func generateHorizontalGradient(img *image.RGBA, width, height int) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Java: int r = (x * 255) / (width > 1 ? width - 1 : 1);
			divisor := width - 1
			if divisor < 1 {
				divisor = 1
			}
			r := (x * 255) / divisor
			img.Set(x, y, color.RGBA{R: uint8(r), G: 128, B: 128, A: 255})
		}
	}
}

// generateVerticalGradient creates a pattern where G varies from 0 to 255 across height.
// This matches Java ReferenceGenerator.generateVerticalGradient() exactly.
// R and B are fixed at 128.
// Tests low-frequency vertical content.
func generateVerticalGradient(img *image.RGBA, width, height int) {
	for y := 0; y < height; y++ {
		// Java: int g = (y * 255) / (height > 1 ? height - 1 : 1);
		divisor := height - 1
		if divisor < 1 {
			divisor = 1
		}
		g := (y * 255) / divisor

		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 128, G: uint8(g), B: 128, A: 255})
		}
	}
}

// generateDiagonalGradient creates a pattern where B varies from 0 to 255 diagonally.
// This matches Java ReferenceGenerator.generateDiagonalGradient() exactly.
// R and G are fixed at 128.
// Tests diagonal frequency content.
func generateDiagonalGradient(img *image.RGBA, width, height int) {
	// Java: int maxDist = width + height - 2; if (maxDist == 0) maxDist = 1;
	maxDist := width + height - 2
	if maxDist == 0 {
		maxDist = 1
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Java: int dist = x + y; int b = (dist * 255) / maxDist;
			dist := x + y
			b := (dist * 255) / maxDist
			img.Set(x, y, color.RGBA{R: 128, G: 128, B: uint8(b), A: 255})
		}
	}
}

// generateCheckerboard creates an 8x8 block checkerboard pattern.
// This matches Java ReferenceGenerator.generateCheckerboard() exactly.
// Tests high-frequency DCT coefficients.
func generateCheckerboard(img *image.RGBA, width, height int) {
	blockSize := 8
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Java: int blockX = x / blockSize; int blockY = y / blockSize;
			// boolean isWhite = ((blockX + blockY) % 2) == 0;
			blockX := x / blockSize
			blockY := y / blockSize
			isWhite := ((blockX + blockY) % 2) == 0

			if isWhite {
				img.Set(x, y, white)
			} else {
				img.Set(x, y, black)
			}
		}
	}
}

// generateQuadrant creates four different patterns in each quadrant.
// This matches Java ReferenceGenerator.generateQuadrant() exactly.
// - Top-left: smooth gradient
// - Top-right: high-frequency 2x2 checkerboard
// - Bottom-left: vertical stripes
// - Bottom-right: deterministic noise-like pattern
func generateQuadrant(img *image.RGBA, width, height int) {
	midX := width / 2
	midY := height / 2

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var r, g, b int

			left := x < midX
			top := y < midY

			if left && top {
				// Top-left: smooth gradient
				// Java: r = (x * 255) / (midX > 0 ? midX : 1);
				// Java: g = (y * 255) / (midY > 0 ? midY : 1);
				// Java: b = 128;
				divX := midX
				if divX < 1 {
					divX = 1
				}
				divY := midY
				if divY < 1 {
					divY = 1
				}
				r = (x * 255) / divX
				g = (y * 255) / divY
				b = 128
			} else if !left && top {
				// Top-right: high-frequency checkerboard (2x2 pixels)
				// Java: boolean isWhite = ((x + y) % 2) == 0;
				isWhite := ((x + y) % 2) == 0
				if isWhite {
					r, g, b = 255, 255, 255
				} else {
					r, g, b = 0, 0, 0
				}
			} else if left {
				// Bottom-left: vertical stripes (left && !top, but !top is implicit here)
				// Java: int stripeWidth = 8;
				// Java: boolean isLight = ((x / stripeWidth) % 2) == 0;
				stripeWidth := 8
				isLight := ((x / stripeWidth) % 2) == 0
				if isLight {
					r, g, b = 200, 200, 200
				} else {
					r, g, b = 55, 55, 55
				}
			} else {
				// Bottom-right: noise-like pattern (deterministic)
				// Java: int noise = ((x * 7) + (y * 13) + (x * y)) % 256;
				// Java: int base = ((x - midX) + (y - midY)) % 256;
				// Java: r = (noise + base) / 2;
				// Java: g = (256 - noise + base) / 2;
				// Java: b = (noise + 256 - base) / 2;
				noise := ((x * 7) + (y * 13) + (x * y)) % 256
				base := ((x - midX) + (y - midY)) % 256
				r = (noise + base) / 2
				g = (256 - noise + base) / 2
				b = (noise + 256 - base) / 2

				// Clamp values to [0, 255]
				r = clamp(r, 0, 255)
				g = clamp(g, 0, 255)
				b = clamp(b, 0, 255)
			}

			img.Set(x, y, color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255})
		}
	}
}

// clamp restricts a value to the range [min, max].
func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// =============================================================================
// Tests for Pattern Generator
// =============================================================================

// TestPatternGeneratorSolid tests solid color pattern generation.
func TestPatternGeneratorSolid(t *testing.T) {
	for _, dim := range AllTestDimensions() {
		t.Run(dim.String(), func(t *testing.T) {
			img := GeneratePattern(PatternSolid, dim.Width, dim.Height)

			// Verify all pixels are (128, 128, 128)
			for y := 0; y < dim.Height; y++ {
				for x := 0; x < dim.Width; x++ {
					r, g, b, a := img.At(x, y).RGBA()
					// Convert from 16-bit to 8-bit
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

					if r8 != 128 || g8 != 128 || b8 != 128 {
						t.Errorf("pixel (%d, %d) = (%d, %d, %d), expected (128, 128, 128)",
							x, y, r8, g8, b8)
					}
					if uint8(a>>8) != 255 {
						t.Errorf("pixel (%d, %d) alpha = %d, expected 255", x, y, uint8(a>>8))
					}
				}
			}
		})
	}
}

// TestPatternGeneratorHorizontalGradient tests horizontal gradient pattern generation.
func TestPatternGeneratorHorizontalGradient(t *testing.T) {
	for _, dim := range AllTestDimensions() {
		t.Run(dim.String(), func(t *testing.T) {
			img := GeneratePattern(PatternHorizontalGradient, dim.Width, dim.Height)

			// Verify R varies from 0 to 255 across width, G and B are 128
			for y := 0; y < dim.Height; y++ {
				for x := 0; x < dim.Width; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

					// Calculate expected R value (matching Java algorithm)
					divisor := dim.Width - 1
					if divisor < 1 {
						divisor = 1
					}
					expectedR := (x * 255) / divisor

					if int(r8) != expectedR {
						t.Errorf("pixel (%d, %d) R = %d, expected %d", x, y, r8, expectedR)
					}
					if g8 != 128 {
						t.Errorf("pixel (%d, %d) G = %d, expected 128", x, y, g8)
					}
					if b8 != 128 {
						t.Errorf("pixel (%d, %d) B = %d, expected 128", x, y, b8)
					}
				}
			}

			// Verify corners for quick sanity check
			r0, _, _, _ := img.At(0, 0).RGBA()
			rEnd, _, _, _ := img.At(dim.Width-1, 0).RGBA()
			if uint8(r0>>8) != 0 {
				t.Errorf("top-left R should be 0, got %d", uint8(r0>>8))
			}
			if uint8(rEnd>>8) != 255 {
				t.Errorf("top-right R should be 255, got %d", uint8(rEnd>>8))
			}
		})
	}
}

// TestPatternGeneratorVerticalGradient tests vertical gradient pattern generation.
func TestPatternGeneratorVerticalGradient(t *testing.T) {
	for _, dim := range AllTestDimensions() {
		t.Run(dim.String(), func(t *testing.T) {
			img := GeneratePattern(PatternVerticalGradient, dim.Width, dim.Height)

			// Verify G varies from 0 to 255 across height, R and B are 128
			for y := 0; y < dim.Height; y++ {
				// Calculate expected G value (matching Java algorithm)
				divisor := dim.Height - 1
				if divisor < 1 {
					divisor = 1
				}
				expectedG := (y * 255) / divisor

				for x := 0; x < dim.Width; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

					if r8 != 128 {
						t.Errorf("pixel (%d, %d) R = %d, expected 128", x, y, r8)
					}
					if int(g8) != expectedG {
						t.Errorf("pixel (%d, %d) G = %d, expected %d", x, y, g8, expectedG)
					}
					if b8 != 128 {
						t.Errorf("pixel (%d, %d) B = %d, expected 128", x, y, b8)
					}
				}
			}

			// Verify corners for quick sanity check
			_, g0, _, _ := img.At(0, 0).RGBA()
			_, gEnd, _, _ := img.At(0, dim.Height-1).RGBA()
			if uint8(g0>>8) != 0 {
				t.Errorf("top-left G should be 0, got %d", uint8(g0>>8))
			}
			if uint8(gEnd>>8) != 255 {
				t.Errorf("bottom-left G should be 255, got %d", uint8(gEnd>>8))
			}
		})
	}
}

// TestPatternGeneratorDiagonalGradient tests diagonal gradient pattern generation.
func TestPatternGeneratorDiagonalGradient(t *testing.T) {
	for _, dim := range AllTestDimensions() {
		t.Run(dim.String(), func(t *testing.T) {
			img := GeneratePattern(PatternDiagonalGradient, dim.Width, dim.Height)

			// Calculate maxDist (matching Java algorithm)
			maxDist := dim.Width + dim.Height - 2
			if maxDist == 0 {
				maxDist = 1
			}

			// Verify B varies from 0 to 255 diagonally, R and G are 128
			for y := 0; y < dim.Height; y++ {
				for x := 0; x < dim.Width; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

					// Calculate expected B value
					dist := x + y
					expectedB := (dist * 255) / maxDist

					if r8 != 128 {
						t.Errorf("pixel (%d, %d) R = %d, expected 128", x, y, r8)
					}
					if g8 != 128 {
						t.Errorf("pixel (%d, %d) G = %d, expected 128", x, y, g8)
					}
					if int(b8) != expectedB {
						t.Errorf("pixel (%d, %d) B = %d, expected %d", x, y, b8, expectedB)
					}
				}
			}

			// Verify corners for quick sanity check
			_, _, b0, _ := img.At(0, 0).RGBA()
			_, _, bEnd, _ := img.At(dim.Width-1, dim.Height-1).RGBA()
			if uint8(b0>>8) != 0 {
				t.Errorf("top-left B should be 0, got %d", uint8(b0>>8))
			}
			if uint8(bEnd>>8) != 255 {
				t.Errorf("bottom-right B should be 255, got %d", uint8(bEnd>>8))
			}
		})
	}
}

// TestPatternGeneratorCheckerboard tests checkerboard pattern generation.
func TestPatternGeneratorCheckerboard(t *testing.T) {
	for _, dim := range AllTestDimensions() {
		t.Run(dim.String(), func(t *testing.T) {
			img := GeneratePattern(PatternCheckerboard, dim.Width, dim.Height)

			blockSize := 8

			// Verify 8x8 checkerboard pattern
			for y := 0; y < dim.Height; y++ {
				for x := 0; x < dim.Width; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

					blockX := x / blockSize
					blockY := y / blockSize
					isWhite := ((blockX + blockY) % 2) == 0

					if isWhite {
						if r8 != 255 || g8 != 255 || b8 != 255 {
							t.Errorf("pixel (%d, %d) should be white (255,255,255), got (%d,%d,%d)",
								x, y, r8, g8, b8)
						}
					} else {
						if r8 != 0 || g8 != 0 || b8 != 0 {
							t.Errorf("pixel (%d, %d) should be black (0,0,0), got (%d,%d,%d)",
								x, y, r8, g8, b8)
						}
					}
				}
			}
		})
	}
}

// TestPatternGeneratorQuadrant tests quadrant pattern generation.
func TestPatternGeneratorQuadrant(t *testing.T) {
	for _, dim := range AllTestDimensions() {
		t.Run(dim.String(), func(t *testing.T) {
			img := GeneratePattern(PatternQuadrant, dim.Width, dim.Height)

			midX := dim.Width / 2
			midY := dim.Height / 2

			// Verify each quadrant has the correct pattern characteristics
			// Top-left: gradient, top-right: checkerboard
			// Bottom-left: stripes, bottom-right: noise-like

			// Sample from each quadrant
			// Top-left corner should be (0, 0, 128) for gradient starting point
			if midX > 0 && midY > 0 {
				r, g, b, _ := img.At(0, 0).RGBA()
				r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)
				if r8 != 0 || g8 != 0 || b8 != 128 {
					t.Errorf("top-left corner should be (0, 0, 128), got (%d, %d, %d)",
						r8, g8, b8)
				}
			}

			// Top-right: verify checkerboard pattern (alternating black/white)
			if midX < dim.Width && midY > 0 {
				r1, g1, b1, _ := img.At(midX, 0).RGBA()
				r2, g2, b2, _ := img.At(midX+1, 0).RGBA()
				r1Val := uint8(r1 >> 8)
				r2Val := uint8(r2 >> 8)

				// Adjacent pixels should be different (black or white)
				if midX+1 < dim.Width {
					if r1Val == r2Val && uint8(g1>>8) == uint8(g2>>8) && uint8(b1>>8) == uint8(b2>>8) {
						t.Errorf("top-right checkerboard: adjacent pixels should be different")
					}
				}
			}

			// Bottom-left: verify vertical stripes
			if midX > 0 && midY < dim.Height {
				// Pixels in same stripe column should be same
				r1, g1, b1, _ := img.At(0, midY).RGBA()
				r2, g2, b2, _ := img.At(0, midY+1).RGBA()
				if midY+1 < dim.Height {
					if uint8(r1>>8) != uint8(r2>>8) || uint8(g1>>8) != uint8(g2>>8) || uint8(b1>>8) != uint8(b2>>8) {
						t.Errorf("bottom-left stripes: vertical pixels should be same color")
					}
				}
			}
		})
	}
}

// TestPatternGeneratorAllPatternsEncodable verifies all patterns can be encoded to valid JPEG.
func TestPatternGeneratorAllPatternsEncodable(t *testing.T) {
	for _, patternType := range AllPatternTypes() {
		for _, dim := range AllTestDimensions() {
			testName := fmt.Sprintf("%s_%s", patternType.PatternName(), dim.String())
			t.Run(testName, func(t *testing.T) {
				// Generate pattern
				img := GeneratePattern(patternType, dim.Width, dim.Height)

				// Encode with F5 encoder
				data, err := WeeksEncodeToBytesStandard(img, 75)
				if err != nil {
					t.Fatalf("WeeksEncodeToBytes failed: %v", err)
				}

				// Verify JPEG structure
				if len(data) < 4 {
					t.Fatal("encoded data too short")
				}
				if data[0] != 0xFF || data[1] != 0xD8 {
					t.Error("missing SOI marker")
				}
				if data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
					t.Error("missing EOI marker")
				}

				// Verify decodable
				decoded, err := stdjpeg.Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatalf("decoded failed: %v", err)
				}

				// Verify dimensions
				if decoded.Bounds().Dx() != dim.Width || decoded.Bounds().Dy() != dim.Height {
					t.Errorf("dimension mismatch: expected %dx%d, got %dx%d",
						dim.Width, dim.Height, decoded.Bounds().Dx(), decoded.Bounds().Dy())
				}
			})
		}
	}
}

// TestPatternGeneratorDimensionsCoverage verifies all required dimensions are supported.
func TestPatternGeneratorDimensionsCoverage(t *testing.T) {
	requiredDimensions := []TestDimension{
		{8, 8},     // Single MCU
		{64, 64},   // Standard
		{256, 256}, // Comprehensive
		{33, 33},   // Non-multiple of 8 (square)
		{100, 75},  // Non-multiple of 8 (rectangular)
	}

	dims := AllTestDimensions()
	for _, req := range requiredDimensions {
		found := false
		for _, dim := range dims {
			if dim.Width == req.Width && dim.Height == req.Height {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("required dimension %s not found in AllTestDimensions()", req.String())
		}
	}
}

// TestPatternGeneratorPatternsCoverage verifies all required patterns are supported.
func TestPatternGeneratorPatternsCoverage(t *testing.T) {
	requiredPatterns := []string{
		"solid",
		"horizontal_gradient",
		"vertical_gradient",
		"diagonal_gradient",
		"checkerboard",
		"quadrant",
	}

	patterns := AllPatternTypes()
	for _, reqName := range requiredPatterns {
		found := false
		for _, pattern := range patterns {
			if pattern.PatternName() == reqName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("required pattern '%s' not found in AllPatternTypes()", reqName)
		}
	}
}

// TestPatternGeneratorDeterminism verifies patterns are deterministic.
func TestPatternGeneratorDeterminism(t *testing.T) {
	for _, patternType := range AllPatternTypes() {
		t.Run(patternType.PatternName(), func(t *testing.T) {
			// Generate the same pattern twice
			img1 := GeneratePattern(patternType, 64, 64)
			img2 := GeneratePattern(patternType, 64, 64)

			// Compare pixel-by-pixel
			for y := 0; y < 64; y++ {
				for x := 0; x < 64; x++ {
					r1, g1, b1, a1 := img1.At(x, y).RGBA()
					r2, g2, b2, a2 := img2.At(x, y).RGBA()

					if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
						t.Errorf("pixel (%d, %d) differs between generations", x, y)
					}
				}
			}
		})
	}
}

// =============================================================================
// Pixel-by-Pixel Verification Tests (for Java compatibility)
// =============================================================================

// TestPatternGeneratorMatchesJavaSolid verifies solid pattern matches Java exactly.
func TestPatternGeneratorMatchesJavaSolid(t *testing.T) {
	// Java generates: new Color(128, 128, 128).getRGB() for all pixels
	img := GeneratePattern(PatternSolid, 64, 64)

	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

			if r8 != 128 || g8 != 128 || b8 != 128 {
				t.Errorf("Java solid pattern mismatch at (%d, %d): got (%d, %d, %d), expected (128, 128, 128)",
					x, y, r8, g8, b8)
			}
		}
	}
}

// TestPatternGeneratorMatchesJavaHorizontalGradient verifies horizontal gradient matches Java.
func TestPatternGeneratorMatchesJavaHorizontalGradient(t *testing.T) {
	// Test various dimensions to ensure formula matches Java
	for _, dim := range AllTestDimensions() {
		t.Run(dim.String(), func(t *testing.T) {
			img := GeneratePattern(PatternHorizontalGradient, dim.Width, dim.Height)

			divisor := dim.Width - 1
			if divisor < 1 {
				divisor = 1
			}

			for y := 0; y < dim.Height; y++ {
				for x := 0; x < dim.Width; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

					// Java: int r = (x * 255) / (width > 1 ? width - 1 : 1);
					expectedR := (x * 255) / divisor

					if int(r8) != expectedR || g8 != 128 || b8 != 128 {
						t.Errorf("Java horizontal gradient mismatch at (%d, %d): got (%d, %d, %d), expected (%d, 128, 128)",
							x, y, r8, g8, b8, expectedR)
					}
				}
			}
		})
	}
}

// TestPatternGeneratorMatchesJavaVerticalGradient verifies vertical gradient matches Java.
func TestPatternGeneratorMatchesJavaVerticalGradient(t *testing.T) {
	for _, dim := range AllTestDimensions() {
		t.Run(dim.String(), func(t *testing.T) {
			img := GeneratePattern(PatternVerticalGradient, dim.Width, dim.Height)

			divisor := dim.Height - 1
			if divisor < 1 {
				divisor = 1
			}

			for y := 0; y < dim.Height; y++ {
				// Java: int g = (y * 255) / (height > 1 ? height - 1 : 1);
				expectedG := (y * 255) / divisor

				for x := 0; x < dim.Width; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

					if r8 != 128 || int(g8) != expectedG || b8 != 128 {
						t.Errorf("Java vertical gradient mismatch at (%d, %d): got (%d, %d, %d), expected (128, %d, 128)",
							x, y, r8, g8, b8, expectedG)
					}
				}
			}
		})
	}
}

// TestPatternGeneratorMatchesJavaDiagonalGradient verifies diagonal gradient matches Java.
func TestPatternGeneratorMatchesJavaDiagonalGradient(t *testing.T) {
	for _, dim := range AllTestDimensions() {
		t.Run(dim.String(), func(t *testing.T) {
			img := GeneratePattern(PatternDiagonalGradient, dim.Width, dim.Height)

			// Java: int maxDist = width + height - 2; if (maxDist == 0) maxDist = 1;
			maxDist := dim.Width + dim.Height - 2
			if maxDist == 0 {
				maxDist = 1
			}

			for y := 0; y < dim.Height; y++ {
				for x := 0; x < dim.Width; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

					// Java: int dist = x + y; int b = (dist * 255) / maxDist;
					dist := x + y
					expectedB := (dist * 255) / maxDist

					if r8 != 128 || g8 != 128 || int(b8) != expectedB {
						t.Errorf("Java diagonal gradient mismatch at (%d, %d): got (%d, %d, %d), expected (128, 128, %d)",
							x, y, r8, g8, b8, expectedB)
					}
				}
			}
		})
	}
}

// TestPatternGeneratorMatchesJavaCheckerboard verifies checkerboard matches Java.
func TestPatternGeneratorMatchesJavaCheckerboard(t *testing.T) {
	for _, dim := range AllTestDimensions() {
		t.Run(dim.String(), func(t *testing.T) {
			img := GeneratePattern(PatternCheckerboard, dim.Width, dim.Height)
			blockSize := 8

			for y := 0; y < dim.Height; y++ {
				for x := 0; x < dim.Width; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

					// Java: int blockX = x / blockSize; int blockY = y / blockSize;
					// boolean isWhite = ((blockX + blockY) % 2) == 0;
					blockX := x / blockSize
					blockY := y / blockSize
					isWhite := ((blockX + blockY) % 2) == 0

					if isWhite {
						if r8 != 255 || g8 != 255 || b8 != 255 {
							t.Errorf("Java checkerboard mismatch at (%d, %d): got (%d, %d, %d), expected (255, 255, 255)",
								x, y, r8, g8, b8)
						}
					} else {
						if r8 != 0 || g8 != 0 || b8 != 0 {
							t.Errorf("Java checkerboard mismatch at (%d, %d): got (%d, %d, %d), expected (0, 0, 0)",
								x, y, r8, g8, b8)
						}
					}
				}
			}
		})
	}
}

// TestPatternGeneratorMatchesJavaQuadrant verifies quadrant pattern matches Java.
func TestPatternGeneratorMatchesJavaQuadrant(t *testing.T) {
	for _, dim := range AllTestDimensions() {
		t.Run(dim.String(), func(t *testing.T) {
			img := GeneratePattern(PatternQuadrant, dim.Width, dim.Height)

			midX := dim.Width / 2
			midY := dim.Height / 2

			for y := 0; y < dim.Height; y++ {
				for x := 0; x < dim.Width; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

					var expectedR, expectedG, expectedB int
					left := x < midX
					top := y < midY

					if left && top {
						// Top-left: smooth gradient
						divX := midX
						if divX < 1 {
							divX = 1
						}
						divY := midY
						if divY < 1 {
							divY = 1
						}
						expectedR = (x * 255) / divX
						expectedG = (y * 255) / divY
						expectedB = 128
					} else if !left && top {
						// Top-right: high-frequency checkerboard
						isWhite := ((x + y) % 2) == 0
						if isWhite {
							expectedR, expectedG, expectedB = 255, 255, 255
						} else {
							expectedR, expectedG, expectedB = 0, 0, 0
						}
					} else if left && !top {
						// Bottom-left: vertical stripes
						stripeWidth := 8
						isLight := ((x / stripeWidth) % 2) == 0
						if isLight {
							expectedR, expectedG, expectedB = 200, 200, 200
						} else {
							expectedR, expectedG, expectedB = 55, 55, 55
						}
					} else {
						// Bottom-right: noise-like pattern
						noise := ((x * 7) + (y * 13) + (x * y)) % 256
						base := ((x - midX) + (y - midY)) % 256
						expectedR = clamp((noise+base)/2, 0, 255)
						expectedG = clamp((256-noise+base)/2, 0, 255)
						expectedB = clamp((noise+256-base)/2, 0, 255)
					}

					if int(r8) != expectedR || int(g8) != expectedG || int(b8) != expectedB {
						t.Errorf("Java quadrant mismatch at (%d, %d): got (%d, %d, %d), expected (%d, %d, %d)",
							x, y, r8, g8, b8, expectedR, expectedG, expectedB)
					}
				}
			}
		})
	}
}

// TestPatternGeneratorAllQualityLevels tests patterns encode correctly at all quality levels.
func TestPatternGeneratorAllQualityLevels(t *testing.T) {
	// Test quality levels matching Java ReferenceGenerator
	qualityLevels := []int{1, 10, 25, 50, 75, 90, 95, 100}
	testPattern := PatternQuadrant // Most complex pattern
	dim := TestDimension{64, 64}

	img := GeneratePattern(testPattern, dim.Width, dim.Height)

	for _, quality := range qualityLevels {
		t.Run(fmt.Sprintf("Q%d", quality), func(t *testing.T) {
			data, err := WeeksEncodeToBytesStandard(img, quality)
			if err != nil {
				t.Fatalf("WeeksEncodeToBytes at Q%d failed: %v", quality, err)
			}

			// Verify decodable
			decoded, err := stdjpeg.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("decode at Q%d failed: %v", quality, err)
			}

			// Verify dimensions preserved
			if decoded.Bounds().Dx() != dim.Width || decoded.Bounds().Dy() != dim.Height {
				t.Errorf("dimension mismatch at Q%d", quality)
			}

			t.Logf("Q%d: %d bytes", quality, len(data))
		})
	}
}
