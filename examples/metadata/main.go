// Package main demonstrates EXIF and ICC profile preservation during JPEG encoding.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/0verkilll/weeksjpegencoder"
)

func main() {
	// Example 1: Create a test image and show metadata functions
	fmt.Println("=== Metadata Preservation Example ===")
	fmt.Println()

	// Create a test image
	img := createTestImage(640, 480)

	// Example 2: Demonstrate parsing functions (using synthetic metadata for demo)
	fmt.Println("Metadata Parsing Functions:")
	fmt.Println("  weeksjpegencoder.ParseEXIF(reader)      - Parse EXIF from io.Reader")
	fmt.Println("  weeksjpegencoder.ParseEXIFBytes(data)   - Parse EXIF from byte slice")
	fmt.Println("  weeksjpegencoder.ParseICCProfile(r)     - Parse ICC from io.Reader")
	fmt.Println("  weeksjpegencoder.ParseICCProfileBytes() - Parse ICC from byte slice")
	fmt.Println()

	// Example 3: Encode without metadata
	if err := encodeWithoutMetadata(img, "output_no_metadata.jpg"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Encoded without metadata: output_no_metadata.jpg")

	// Example 4: Show how to preserve metadata from source
	fmt.Println()
	fmt.Println("To preserve metadata from a source JPEG:")
	fmt.Println()
	fmt.Println("  // Method 1: Using WithSourceImageBytes")
	fmt.Println("  sourceData, _ := os.ReadFile(\"source.jpg\")")
	fmt.Println("  enc, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(w, 75,")
	fmt.Println("      weeksjpegencoder.WithSourceImageBytes(sourceData),")
	fmt.Println("  )")
	fmt.Println()
	fmt.Println("  // Method 2: Using WithSourceImage with reader")
	fmt.Println("  f, _ := os.Open(\"source.jpg\")")
	fmt.Println("  defer f.Close()")
	fmt.Println("  enc, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(w, 75,")
	fmt.Println("      weeksjpegencoder.WithSourceImage(f),")
	fmt.Println("  )")
	fmt.Println()
	fmt.Println("  // Method 3: Parse and apply separately")
	fmt.Println("  exifData, _ := weeksjpegencoder.ParseEXIFBytes(sourceData)")
	fmt.Println("  iccData, _ := weeksjpegencoder.ParseICCProfileBytes(sourceData)")
	fmt.Println("  enc, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(w, 75,")
	fmt.Println("      weeksjpegencoder.WithEXIF(exifData),")
	fmt.Println("      weeksjpegencoder.WithICCProfile(iccData),")
	fmt.Println("  )")

	// Example 5: If command-line args provided, use them
	if len(os.Args) == 3 {
		fmt.Println()
		fmt.Println("=== Processing Real Image ===")
		if err := processSourceImage(os.Args[1], os.Args[2]); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// encodeWithoutMetadata encodes an image without preserving metadata.
func encodeWithoutMetadata(img image.Image, filename string) error {
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

// processSourceImage demonstrates real metadata preservation from a source JPEG.
func processSourceImage(sourceFile, outputFile string) error {
	// Read source JPEG
	sourceData, err := os.ReadFile(sourceFile) // #nosec // G304: Example code with controlled filenames
	if err != nil {
		return fmt.Errorf("failed to read source: %w", err)
	}
	fmt.Printf("Read source: %s (%d bytes)\n", sourceFile, len(sourceData))

	// Parse EXIF
	exifData, exifErr := weeksjpegencoder.ParseEXIFBytes(sourceData)
	switch exifErr {
	case nil:
		fmt.Printf("  EXIF data: %d bytes\n", len(exifData))
	case weeksjpegencoder.ErrNoEXIF:
		fmt.Println("  EXIF data: none")
	default:
		fmt.Printf("  EXIF error: %v\n", exifErr)
	}

	// Parse ICC profile
	iccData, iccErr := weeksjpegencoder.ParseICCProfileBytes(sourceData)
	switch iccErr {
	case nil:
		fmt.Printf("  ICC profile: %d bytes\n", len(iccData))
	case weeksjpegencoder.ErrNoICC:
		fmt.Println("  ICC profile: none")
	default:
		fmt.Printf("  ICC error: %v\n", iccErr)
	}

	// Create test image (in real usage, you'd decode the source image)
	img := createTestImage(640, 480)

	// Create output file
	f, err := os.Create(outputFile) // #nosec // G304: Example code with controlled filenames
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Encode with metadata preservation
	enc, err := weeksjpegencoder.NewWeeksEncoderWithOptions(f, 75,
		weeksjpegencoder.WithSourceImageBytes(sourceData),
	)
	if err != nil {
		return fmt.Errorf("failed to create encoder: %w", err)
	}

	if err := enc.Encode(img); err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	fmt.Printf("Output written: %s\n", outputFile)
	fmt.Printf("  EXIF preserved: %v\n", exifErr == nil)
	fmt.Printf("  ICC preserved: %v\n", iccErr == nil)

	return nil
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
