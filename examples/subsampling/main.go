// Package main demonstrates chroma subsampling modes.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/0verkilll/jpeg"
	"github.com/0verkilll/weeksjpegencoder"
)

func main() {
	// Create a test image with color gradients (to show subsampling effects)
	img := createColorTestImage(640, 480)

	fmt.Println("Encoding with different subsampling modes...")
	fmt.Println()

	// Encode with 4:2:0 (default)
	size420, err := encodeWithSubsampling(img, "output_420.jpg", jpeg.ChromaSubsampling420)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("4:2:0 (default): output_420.jpg - %d bytes\n", size420)

	// Encode with 4:2:2
	size422, err := encodeWithSubsampling(img, "output_422.jpg", jpeg.ChromaSubsampling422)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("4:2:2:           output_422.jpg - %d bytes\n", size422)

	// Encode with 4:4:4
	size444, err := encodeWithSubsampling(img, "output_444.jpg", jpeg.ChromaSubsampling444)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("4:4:4:           output_444.jpg - %d bytes\n", size444)

	fmt.Println()
	fmt.Println("Size comparison:")
	fmt.Printf("  4:2:2 is %.1f%% larger than 4:2:0\n", float64(size422-size420)/float64(size420)*100)
	fmt.Printf("  4:4:4 is %.1f%% larger than 4:2:0\n", float64(size444-size420)/float64(size420)*100)
}

// encodeWithSubsampling encodes an image with the specified subsampling mode.
func encodeWithSubsampling(img image.Image, filename string, subsampling jpeg.ChromaSubsamplingMode) (int64, error) {
	f, err := os.Create(filename) // #nosec // G304: Example code with controlled filenames
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	enc, err := weeksjpegencoder.NewWeeksEncoderWithOptions(f, 75,
		weeksjpegencoder.WithSubsampling(subsampling),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create encoder: %w", err)
	}

	if encErr := enc.Encode(img); encErr != nil {
		return 0, fmt.Errorf("failed to encode image: %w", encErr)
	}

	// Get file size
	info, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}

	return info.Size(), nil
}

// createColorTestImage creates an image with color patterns to show subsampling effects.
// Note: width and height must be > 0 to avoid division by zero.
func createColorTestImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Guard against division by zero (width/height are always positive in this example)
	if width <= 0 || height <= 0 {
		return img
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create color bands and gradients
			var c color.RGBA

			section := x * 4 / width
			switch section {
			case 0:
				// Red gradient - safe: y*255/height is in [0,255] for valid height
				c = color.RGBA{R: uint8(y * 255 / height), G: 0, B: 0, A: 255} // #nosec // G115: height > 0 guaranteed
			case 1:
				// Green gradient
				c = color.RGBA{R: 0, G: uint8(y * 255 / height), B: 0, A: 255} // #nosec // G115: height > 0 guaranteed
			case 2:
				// Blue gradient
				c = color.RGBA{R: 0, G: 0, B: uint8(y * 255 / height), A: 255} // #nosec // G115: height > 0 guaranteed
			case 3:
				// Rainbow gradient
				r := uint8(x * 255 / width)                  // #nosec // G115: width > 0 guaranteed
				g := uint8(y * 255 / height)                 // #nosec // G115: height > 0 guaranteed
				b := uint8((x + y) * 255 / (width + height)) // #nosec // G115: width+height > 0 guaranteed
				c = color.RGBA{R: r, G: g, B: b, A: 255}
			}

			img.Set(x, y, c)
		}
	}

	return img
}
