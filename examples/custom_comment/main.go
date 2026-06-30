// Package main demonstrates setting custom COM marker comments.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/0verkilll/weeksjpegencoder"
)

func main() {
	// Create a test image
	img := createTestImage(320, 240)

	// Example 1: Encode with default Weeks comment (for F5 compatibility)
	if err := encodeWithDefaultComment(img, "output_default.jpg"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Encoded with default comment: output_default.jpg")
	fmt.Println("  Comment: JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech.")

	// Example 2: Encode with custom comment using builder pattern
	if err := encodeWithCustomComment(img, "output_custom.jpg", "My Custom Encoder v1.0"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Encoded with custom comment: output_custom.jpg")
	fmt.Println("  Comment: My Custom Encoder v1.0")

	// Example 3: Encode with custom comment using functional options
	if err := encodeWithOptions(img, "output_options.jpg", "Created by MyApp"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Encoded with functional options: output_options.jpg")
	fmt.Println("  Comment: Created by MyApp")
}

// encodeWithDefaultComment encodes using the default Weeks COM marker.
func encodeWithDefaultComment(img image.Image, filename string) error {
	f, err := os.Create(filename) // #nosec // G304: Example code with controlled filenames
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	enc, err := weeksjpegencoder.NewWeeksEncoder(f, 75)
	if err != nil {
		return fmt.Errorf("failed to create encoder: %w", err)
	}

	return enc.Encode(img)
}

// encodeWithCustomComment encodes using a custom COM marker via builder pattern.
func encodeWithCustomComment(img image.Image, filename, comment string) error {
	f, err := os.Create(filename) // #nosec // G304: Example code with controlled filenames
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	enc, err := weeksjpegencoder.NewWeeksEncoder(f, 75)
	if err != nil {
		return fmt.Errorf("failed to create encoder: %w", err)
	}

	// Use builder pattern to set custom comment
	enc.SetComment(comment)

	return enc.Encode(img)
}

// encodeWithOptions encodes using functional options.
func encodeWithOptions(img image.Image, filename, comment string) error {
	f, err := os.Create(filename) // #nosec // G304: Example code with controlled filenames
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	enc, err := weeksjpegencoder.NewWeeksEncoderWithOptions(f, 75,
		weeksjpegencoder.WithComment(comment),
	)
	if err != nil {
		return fmt.Errorf("failed to create encoder: %w", err)
	}

	return enc.Encode(img)
}

// createTestImage creates a simple test image.
func createTestImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Guard against division by zero
	if width <= 0 || height <= 0 {
		return img
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8(x * 255 / width)  // #nosec G115 - width > 0 guaranteed by guard
			g := uint8(y * 255 / height) // #nosec G115 - height > 0 guaranteed by guard
			b := uint8(128)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}
