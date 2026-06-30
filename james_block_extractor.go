// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements the JamesBlockExtractor for pixel block extraction that
// matches the exact behavior of JpegInfo.java from the James R. Weeks encoder.

package weeksjpegencoder

import (
	"image"

	"github.com/0verkilll/jpeg"
)

// JamesBlockExtractor implements the BlockExtractor interface matching
// the exact behavior of JpegInfo.java from the James R. Weeks encoder.
//
// Key differences from the standard Go YCbCrBlockExtractor:
//  1. Uses exact BT.601 coefficients as floating point (not Go's fixed-point)
//  2. For 4:2:0 chroma, takes only the top-left pixel of each 2x2 block
//     (no averaging)
//  3. Pre-converts the entire image to YCbCr arrays like Java does
type JamesBlockExtractor struct {
	subsampling jpeg.ChromaSubsamplingMode

	// Pre-converted YCbCr arrays (matching Java's JpegInfo layout)
	Y  [][]float32
	Cb [][]float32
	Cr [][]float32

	// Original image dimensions
	imageWidth  int
	imageHeight int

	// Padded dimensions
	yWidth   int
	yHeight  int
	cbWidth  int
	cbHeight int
}

// NewJamesBlockExtractor creates a new JamesBlockExtractor that pre-converts
// the image to YCbCr using the exact same algorithm as JpegInfo.java.
//
//goland:noinspection GoUnusedExportedFunction
func NewJamesBlockExtractor(img image.Image, subsampling jpeg.ChromaSubsamplingMode) *JamesBlockExtractor {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate padded dimensions (matching Java's JpegInfo)
	maxH := 2 // Max horizontal factor for 4:2:0
	maxV := 2 // Max vertical factor for 4:2:0

	yWidth := ((width + 8*maxH - 1) / (8 * maxH)) * maxH * 8
	yHeight := ((height + 8*maxV - 1) / (8 * maxV)) * maxV * 8

	ext := &JamesBlockExtractor{
		subsampling: subsampling,
		imageWidth:  width,
		imageHeight: height,
		yWidth:      yWidth,
		yHeight:     yHeight,
		cbWidth:     yWidth / 2,
		cbHeight:    yHeight / 2,
	}

	// Allocate arrays
	ext.Y = make([][]float32, yHeight)
	for i := range ext.Y {
		ext.Y[i] = make([]float32, yWidth)
	}
	ext.Cb = make([][]float32, yHeight/2)
	for i := range ext.Cb {
		ext.Cb[i] = make([]float32, yWidth/2)
	}
	ext.Cr = make([][]float32, yHeight/2)
	for i := range ext.Cr {
		ext.Cr[i] = make([]float32, yWidth/2)
	}

	// Compute Y at full resolution and Cb1/Cr1 at full resolution. Java's
	// JpegInfo.getYCCArray stores Cb1/Cr1 at the Y plane size before
	// running DownSample to derive Cb2/Cr2.
	cb1 := make([][]float32, yHeight)
	cr1 := make([][]float32, yHeight)
	for i := 0; i < yHeight; i++ {
		cb1[i] = make([]float32, yWidth)
		cr1[i] = make([]float32, yWidth)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			r, g, b, _ := c.RGBA()
			ri := int(r >> 8)
			gi := int(g >> 8)
			bi := int(b >> 8)

			// BT.601 RGB to YCbCr exactly matching f5.jar's JpegInfo.java:
			//   Y[y][x]  =  (float)(0.299*r + 0.587*g + 0.114*b)
			//   Cb1[y][x] = 128 + (float)(-0.16874*r - 0.33126*g + 0.5*b)
			//   Cr1[y][x] = 128 + (float)(0.5*r - 0.41869*g - 0.08131*b)
			ext.Y[y][x] = float32(0.299*float64(ri) + 0.587*float64(gi) + 0.114*float64(bi))
			cb1[y][x] = float32(-0.16874*float64(ri)-0.33126*float64(gi)+0.5*float64(bi)) + 128
			cr1[y][x] = float32(0.5*float64(ri)-0.41869*float64(gi)-0.08131*float64(bi)) + 128
		}
	}

	// DownSample Cb1/Cr1 → Cb/Cr matching JpegInfo.DownSample (JpegInfo.java
	// lines 101–124). For each 2x2 source block: output = (a+b+c+d+bias)/4,
	// where bias alternates 1, 2 across output columns and resets to 1 at
	// the start of each row.
	for outrow := 0; outrow < ext.cbHeight; outrow++ {
		inrow := 2 * outrow
		bias := float32(1)
		for outcol := 0; outcol < ext.cbWidth; outcol++ {
			incol := 2 * outcol
			cbSum := cb1[inrow][incol] + cb1[inrow][incol+1] + cb1[inrow+1][incol] + cb1[inrow+1][incol+1] + bias
			crSum := cr1[inrow][incol] + cr1[inrow][incol+1] + cr1[inrow+1][incol] + cr1[inrow+1][incol+1] + bias
			ext.Cb[outrow][outcol] = cbSum / 4.0
			ext.Cr[outrow][outcol] = crSum / 4.0
			// Java's `bias ^= 3` flips between 1 and 2.
			if bias == 1 {
				bias = 2
			} else {
				bias = 1
			}
		}
	}

	// Pad Y to block boundaries (matching Java)
	for y := height; y < yHeight; y++ {
		for x := 0; x < yWidth; x++ {
			srcX := x
			if srcX >= width {
				srcX = width - 1
			}
			srcY := height - 1
			ext.Y[y][x] = ext.Y[srcY][srcX]
		}
	}
	for y := 0; y < height; y++ {
		for x := width; x < yWidth; x++ {
			ext.Y[y][x] = ext.Y[y][width-1]
		}
	}

	// No extra chroma padding here: Java leaves padded entries in Cb1/Cr1
	// at 0 and lets DownSample average them, so the downsample loop above
	// already produces the same boundary values Java produces.

	return ext
}

// getY returns the Y value at the given row and column, with bounds checking
// exactly like JpegInfo.java getY() method.
func (e *JamesBlockExtractor) getY(row, col int) float64 {
	if row >= len(e.Y) {
		row = len(e.Y) - 1
	}
	if row < 0 {
		row = 0
	}
	if col >= len(e.Y[0]) {
		col = len(e.Y[0]) - 1
	}
	if col < 0 {
		col = 0
	}
	return float64(e.Y[row][col])
}

// getCb returns the Cb value at the given row and column, with bounds checking
// exactly like JpegInfo.java getCb() method.
func (e *JamesBlockExtractor) getCb(row, col int) float64 {
	if row >= len(e.Cb) {
		row = len(e.Cb) - 1
	}
	if row < 0 {
		row = 0
	}
	if col >= len(e.Cb[0]) {
		col = len(e.Cb[0]) - 1
	}
	if col < 0 {
		col = 0
	}
	return float64(e.Cb[row][col])
}

// getCr returns the Cr value at the given row and column, with bounds checking
// exactly like JpegInfo.java getCr() method.
func (e *JamesBlockExtractor) getCr(row, col int) float64 {
	if row >= len(e.Cr) {
		row = len(e.Cr) - 1
	}
	if row < 0 {
		row = 0
	}
	if col >= len(e.Cr[0]) {
		col = len(e.Cr[0]) - 1
	}
	if col < 0 {
		col = 0
	}
	return float64(e.Cr[row][col])
}

// ExtractBlock extracts an 8x8 block at the given position.
// Parameters: img (ignored, uses pre-converted arrays), component (0=Y, 1=Cb, 2=Cr),
// x, y (top-left position in pixel coordinates).
// Returns 64 float64 values in row-major order, NOT level-shifted (0-255 range).
//
//goland:noinspection GoUnusedParameter
func (e *JamesBlockExtractor) ExtractBlock(img image.Image, component, x, y int) [64]float64 {
	var block [64]float64

	for blockY := 0; blockY < 8; blockY++ {
		for blockX := 0; blockX < 8; blockX++ {
			var value float32
			switch component {
			case 0: // Y (luminance)
				row := y + blockY
				col := x + blockX
				if row >= len(e.Y) {
					row = len(e.Y) - 1
				}
				if col >= len(e.Y[0]) {
					col = len(e.Y[0]) - 1
				}
				value = e.Y[row][col]

			case 1: // Cb
				// Chroma is at half resolution
				row := y/2 + blockY
				col := x/2 + blockX
				if row >= len(e.Cb) {
					row = len(e.Cb) - 1
				}
				if col >= len(e.Cb[0]) {
					col = len(e.Cb[0]) - 1
				}
				value = e.Cb[row][col]

			case 2: // Cr
				// Chroma is at half resolution
				row := y/2 + blockY
				col := x/2 + blockX
				if row >= len(e.Cr) {
					row = len(e.Cr) - 1
				}
				if col >= len(e.Cr[0]) {
					col = len(e.Cr[0]) - 1
				}
				value = e.Cr[row][col]

			default:
				value = 128 // Neutral value
			}
			block[blockY*8+blockX] = float64(value)
		}
	}
	return block
}

// Compile-time interface compliance check
var _ BlockExtractor = (*JamesBlockExtractor)(nil)
