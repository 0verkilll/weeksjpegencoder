// Package main demonstrates basic JPEG encoding with weeksjpegencoder.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/0verkilll/weeksjpegencoder"
)

func main() {
	// Create a test image with a gradient pattern
	img := createTestImage(640, 480)
	fmt.Printf("Created test image: %dx%d pixels\n", img.Bounds().Dx(), img.Bounds().Dy())

	// Example 1: Encode to file
	if err := encodeToFile(img, "output.jpg", 75); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error encoding to file: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Encoded to file: output.jpg (quality 75)")

	// Example 2: Encode to bytes
	data, err := weeksjpegencoder.WeeksEncodeToBytes(img, 85)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error encoding to bytes: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Encoded to bytes: %d bytes (quality 85)\n", len(data))
}

// encodeToFile encodes an image to a JPEG file with the specified quality.
func encodeToFile(img image.Image, filename string, quality int) error {
	f, err := os.Create(filename) // #nosec // G304: Example code with controlled filenames
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	enc, err := weeksjpegencoder.NewWeeksEncoder(f, quality)
	if err != nil {
		return fmt.Errorf("failed to create encoder: %w", err)
	}

	if err := enc.Encode(img); err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}

	return nil
}

// createTestImage creates a test image with a gradient pattern.
func createTestImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Guard against division by zero
	if width <= 0 || height <= 0 {
		return img
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8(x * 255 / width)                  // #nosec G115 - width > 0 guaranteed by guard
			g := uint8(y * 255 / height)                 // #nosec G115 - height > 0 guaranteed by guard
			b := uint8((x + y) * 255 / (width + height)) // #nosec G115 - width+height > 0 guaranteed
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}
