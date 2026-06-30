package weeksjpegencoder_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	"github.com/0verkilll/jpeg"
	"github.com/0verkilll/weeksjpegencoder"
)

// createExampleImage creates a simple test image for examples.
func createExampleImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 255 / width),
				G: uint8(y * 255 / height),
				B: 128,
				A: 255,
			})
		}
	}
	return img
}

// Example demonstrates basic JPEG encoding using default settings.
func Example() {
	// Create a simple test image
	img := createExampleImage(64, 64)

	// Encode using convenience function with quality 75
	data, err := weeksjpegencoder.WeeksEncodeToBytes(img, 75)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// The output is a valid JPEG
	fmt.Printf("Encoded %d bytes\n", len(data))
	fmt.Printf("Valid JPEG: %v\n", data[0] == 0xFF && data[1] == 0xD8)

	// Output:
	// Encoded 1028 bytes
	// Valid JPEG: true
}

// Example_newWeeksEncoder demonstrates creating an encoder and encoding an image.
func Example_newWeeksEncoder() {
	var buf bytes.Buffer
	img := createExampleImage(32, 32)

	// Create encoder with quality 85
	enc, err := weeksjpegencoder.NewWeeksEncoder(&buf, 85)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Encode the image
	if err := enc.Encode(img); err != nil {
		fmt.Println("Encode error:", err)
		return
	}

	fmt.Printf("Encoded successfully: %d bytes\n", buf.Len())

	// Output:
	// Encoded successfully: 815 bytes
}

// Example_newWeeksEncoderWithOptions demonstrates using functional options.
// 4:4:4 is only available in standard mode (f5.jar is hardcoded for 4:2:0),
// so this example pairs WithSubsampling(444) with WithStandardMode().
func Example_newWeeksEncoderWithOptions() {
	var buf bytes.Buffer
	img := createExampleImage(32, 32)

	// Create encoder with custom options:
	// - Quality 90
	// - Custom comment
	// - 4:4:4 subsampling (requires standard mode)
	enc, err := weeksjpegencoder.NewWeeksEncoderWithOptions(&buf, 90,
		weeksjpegencoder.WithComment("Custom F5 Encoder"),
		weeksjpegencoder.WithSubsampling(jpeg.ChromaSubsampling444),
		weeksjpegencoder.WithStandardMode(),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if err := enc.Encode(img); err != nil {
		fmt.Println("Encode error:", err)
		return
	}

	fmt.Printf("High-quality encode: %d bytes\n", buf.Len())

	// Output:
	// High-quality encode: 924 bytes
}

// Example_withQuantizer demonstrates injecting a custom quantizer for testing.
// Note: Custom quantizers are only used in standard mode, not James-compatible mode.
func Example_withQuantizer() {
	var buf bytes.Buffer
	img := createExampleImage(16, 16)

	// Create a mock quantizer for testing
	mockQuant := &weeksjpegencoder.MockQuantizer{
		// Using all 1s produces minimal quantization (highest quality)
		LumTable:   [64]int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		ChromTable: [64]int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	}

	// Inject the mock quantizer with standard mode (required for custom quantizers)
	enc, err := weeksjpegencoder.NewWeeksEncoderWithOptions(&buf, 75,
		weeksjpegencoder.WithQuantizer(mockQuant),
		weeksjpegencoder.WithStandardMode(),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if err := enc.Encode(img); err != nil {
		fmt.Println("Encode error:", err)
		return
	}

	// Verify the quantizer was called (6 blocks for 16x16 with 4:2:0)
	fmt.Printf("Quantizer called: %d times\n", mockQuant.QuantizeCalls)
	fmt.Printf("Encoded: %d bytes\n", buf.Len())

	// Output:
	// Quantizer called: 6 times
	// Encoded: 810 bytes
}

// Example_withSubsampling demonstrates different chroma subsampling modes.
// Uses standard mode to produce Go-decodable output with proper subsampling.
func Example_withSubsampling() {
	img := createExampleImage(32, 32)

	// Encode with 4:2:0 (default, smallest file) using standard mode
	var buf420 bytes.Buffer
	enc420, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(&buf420, 75,
		weeksjpegencoder.WithSubsampling(jpeg.ChromaSubsampling420),
		weeksjpegencoder.WithStandardMode(),
	)
	_ = enc420.Encode(img)

	// Encode with 4:4:4 (largest file, highest quality) using standard mode
	var buf444 bytes.Buffer
	enc444, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(&buf444, 75,
		weeksjpegencoder.WithSubsampling(jpeg.ChromaSubsampling444),
		weeksjpegencoder.WithStandardMode(),
	)
	_ = enc444.Encode(img)

	fmt.Printf("4:2:0 size: %d bytes\n", buf420.Len())
	fmt.Printf("4:4:4 size: %d bytes\n", buf444.Len())
	fmt.Printf("4:4:4 is larger: %v\n", buf444.Len() > buf420.Len())

	// Output:
	// 4:2:0 size: 791 bytes
	// 4:4:4 size: 856 bytes
	// 4:4:4 is larger: true
}

// Example_setSubsampling demonstrates method chaining configuration. 4:2:2 is
// only available outside James-compatible mode (f5.jar itself is hardcoded for
// 4:2:0), so this example pairs SetSubsampling with WithStandardMode.
func Example_setSubsampling() {
	var buf bytes.Buffer
	img := createExampleImage(32, 32)

	enc, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(&buf, 75,
		weeksjpegencoder.WithStandardMode(),
	)
	enc.SetSubsampling(jpeg.ChromaSubsampling422).
		SetComment("Method chaining example")

	_ = enc.Encode(img)
	fmt.Printf("Encoded with 4:2:2: %d bytes\n", buf.Len())

	// Output:
	// Encoded with 4:2:2: 774 bytes
}

// Example_newIJGQuantizer demonstrates creating a standalone quantizer.
func Example_newIJGQuantizer() {
	// Create a quantizer with quality 50 (standard tables)
	q, err := weeksjpegencoder.NewIJGQuantizer(50)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Get the luminance quantization table
	lumTable := q.GetQuantTable(true)

	// The DC coefficient quantization value (position 0)
	fmt.Printf("DC quant value: %d\n", lumTable[0])

	// Quality 50 produces the standard ITU-T T.81 table unchanged
	// The standard DC value is 16
	fmt.Printf("Is standard DC: %v\n", lumTable[0] == 16)

	// Output:
	// DC quant value: 16
	// Is standard DC: true
}
