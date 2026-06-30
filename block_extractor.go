// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements the YCbCrBlockExtractor for pixel block extraction with
// color space conversion and chroma subsampling per JPEG specification.

package weeksjpegencoder

import (
	"image"
	"image/color"

	"github.com/0verkilll/jpeg"
)

// YCbCrBlockExtractor implements the BlockExtractor interface for YCbCr color space.
//
// This extractor handles color space conversion from any image.Image to YCbCr,
// chroma subsampling with neighborhood averaging for 4:2:0 and 4:2:2 modes,
// boundary clamping for edge blocks, and pixel replication at image boundaries.
//
// The subsampling mode affects how chroma (Cb/Cr) components are extracted:
//   - 4:4:4: No subsampling, each chroma pixel maps 1:1
//   - 4:2:2: Horizontal 2:1 subsampling, chroma averaged over 2 horizontal pixels
//   - 4:2:0: Both 2:1 subsampling, chroma averaged over 2x2 pixel blocks
type YCbCrBlockExtractor struct {
	subsampling jpeg.ChromaSubsamplingMode
}

// NewYCbCrBlockExtractor creates a new YCbCrBlockExtractor with the specified
// chroma subsampling mode (ChromaSubsampling444, ChromaSubsampling422, or
// ChromaSubsampling420).
//
//goland:noinspection GoUnusedExportedFunction
func NewYCbCrBlockExtractor(subsampling jpeg.ChromaSubsamplingMode) *YCbCrBlockExtractor {
	return &YCbCrBlockExtractor{subsampling: subsampling}
}

// ExtractBlock extracts an 8x8 block at the given position.
// Parameters: img (source image), component (0=Y, 1=Cb, 2=Cr), x, y (top-left position).
// Returns 64 float64 values in row-major order, NOT level-shifted (0-255 range).
// For chroma components with subsampling, averages neighborhood pixels.
func (e *YCbCrBlockExtractor) ExtractBlock(img image.Image, component, x, y int) [64]float64 {
	var block [64]float64
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	scaleX, scaleY := e.getScaleFactors(component)

	for blockY := 0; blockY < 8; blockY++ {
		for blockX := 0; blockX < 8; blockX++ {
			srcX := clampInt(x+blockX*scaleX, 0, width-1)
			srcY := clampInt(y+blockY*scaleY, 0, height-1)

			var value float64
			switch component {
			case 0: // Y (luminance)
				value = e.extractLuminance(img, bounds, srcX, srcY)
			case 1: // Cb
				value = e.extractChroma(img, bounds, srcX, srcY, width, height, scaleX, scaleY, true)
			case 2: // Cr
				value = e.extractChroma(img, bounds, srcX, srcY, width, height, scaleX, scaleY, false)
			default:
				value = e.extractLuminance(img, bounds, srcX, srcY)
			}
			block[blockY*8+blockX] = value
		}
	}
	return block
}

// getScaleFactors returns horizontal and vertical scale factors for a component.
// Luminance (component 0) is always 1:1. Chroma depends on subsampling mode.
func (e *YCbCrBlockExtractor) getScaleFactors(component int) (scaleX, scaleY int) {
	if component == 0 {
		return 1, 1
	}
	switch e.subsampling {
	case jpeg.ChromaSubsampling444:
		return 1, 1
	case jpeg.ChromaSubsampling422:
		return 2, 1
	case jpeg.ChromaSubsampling420:
		return 2, 2
	default:
		return 1, 1
	}
}

// extractLuminance extracts the Y (luminance) component from a single pixel.
func (e *YCbCrBlockExtractor) extractLuminance(img image.Image, bounds image.Rectangle, srcX, srcY int) float64 {
	c := img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY)
	ycbcr := color.YCbCrModel.Convert(c).(color.YCbCr)
	return float64(ycbcr.Y)
}

// extractChroma extracts a chroma component, averaging over the neighborhood if needed.
func (e *YCbCrBlockExtractor) extractChroma(img image.Image, bounds image.Rectangle, srcX, srcY, width, height, scaleX, scaleY int, isCb bool) float64 {
	if scaleX == 1 && scaleY == 1 {
		c := img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY)
		ycbcr := color.YCbCrModel.Convert(c).(color.YCbCr)
		if isCb {
			return float64(ycbcr.Cb)
		}
		return float64(ycbcr.Cr)
	}
	return float64(e.averageChroma(img, bounds, srcX, srcY, width, height, scaleX, scaleY, isCb))
}

// averageChroma averages chroma values over a scaleX x scaleY neighborhood.
// Returns the rounded average using round-to-nearest: (sum + count/2) / count.
func (e *YCbCrBlockExtractor) averageChroma(img image.Image, bounds image.Rectangle, srcX, srcY, width, height, scaleX, scaleY int, isCb bool) uint8 {
	var sum, count int

	for dy := 0; dy < scaleY; dy++ {
		for dx := 0; dx < scaleX; dx++ {
			px := clampInt(srcX+dx, 0, width-1)
			py := clampInt(srcY+dy, 0, height-1)

			c := img.At(bounds.Min.X+px, bounds.Min.Y+py)
			ycbcr := color.YCbCrModel.Convert(c).(color.YCbCr)

			if isCb {
				sum += int(ycbcr.Cb)
			} else {
				sum += int(ycbcr.Cr)
			}
			count++
		}
	}

	if count == 0 {
		return 128 // Neutral chroma value (edge case safety)
	}
	// Result is always in [0,255] range since we're averaging uint8 pixel values
	return uint8((sum + count/2) / count) // #nosec G115 - averaging pixel values always yields valid uint8
}

// clampInt clamps an integer value to the range [min, max].
func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Compile-time interface compliance check
var _ BlockExtractor = (*YCbCrBlockExtractor)(nil)
