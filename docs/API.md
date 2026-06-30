# weeksjpegencoder API Documentation

> James R. Weeks-compatible JPEG encoding for Go

This package implements a baseline JPEG encoder that produces files compatible with the F5 steganography algorithm and tools derived from James R. Weeks' original Java JPEG encoder. The encoder generates standard baseline JPEG files (SOF0) with the characteristic COM marker signature used by F5Android and similar steganography tools.

## Overview

The weeksjpegencoder package is a standalone Go module that depends on the jpeg package (`github.com/0verkilll/jpeg`) for low-level JPEG encoding infrastructure including:

- Huffman encoding tables and bit stream writing
- DCT (Discrete Cosine Transform) implementations
- Quantization table scaling
- Marker writing functions
- Color space conversion

## Architecture

The package follows SOLID principles with a component-based architecture that separates concerns into focused, testable units:

```
WeeksEncoder (Orchestrator)
    |
    +-- Quantizer (DCT coefficient quantization)
    |       Implementation: IJGQuantizer
    |
    +-- BlockEncoder (Entropy coding)
    |       Implementation: HuffmanBlockEncoder
    |
    +-- BlockExtractor (Pixel extraction)
            Implementation: YCbCrBlockExtractor
```

Each component is defined by an interface, allowing for dependency injection and easy testing with mock implementations.

## Component Interfaces

The package exports the following interfaces for customization and testing:

| Interface | Purpose |
|-----------|---------|
| `Quantizer` | Performs DCT coefficient quantization using configurable tables |
| `BlockEncoder` | Handles Huffman entropy encoding of quantized blocks |
| `BlockExtractor` | Extracts pixel blocks with color space conversion |
| `BitWriter` | Abstracts bit-level writing for entropy coding |
| `HuffmanTable` | Abstracts Huffman code lookup |
| `DCT` | Abstracts Discrete Cosine Transform operations |

## Interface Abstraction Strategy

The interfaces enable several key capabilities:

1. **Testing**: Inject mock components to test encoding logic in isolation
2. **Extensibility**: Implement custom quantizers, encoders, or extractors
3. **Loose Coupling**: Components depend on abstractions, not concrete types
4. **Single Responsibility**: Each component handles one specific concern

The adapters (`BitWriterAdapter`, `HuffmanTableAdapter`, `DCTAdapter`) allow the concrete jpeg package types to satisfy these interfaces.

## Functional Options

The encoder supports functional options for flexible configuration:

| Option | Description |
|--------|-------------|
| `WithQuantizer` | Inject a custom Quantizer implementation |
| `WithBlockEncoder` | Inject a custom BlockEncoder implementation |
| `WithBlockExtractor` | Inject a custom BlockExtractor implementation |
| `WithDCT` | Inject a custom DCT implementation |
| `WithComment` | Set a custom COM marker comment |
| `WithSubsampling` | Set the chroma subsampling mode |
| `WithStandardMode` | Use standard JPEG encoding (not James-compatible) |
| `WithSourceImage` | Preserve EXIF/ICC metadata from source |
| `WithEXIF` | Set EXIF data directly |
| `WithICCProfile` | Set ICC profile data directly |

## James R. Weeks Encoder Characteristics

The encoder produces JPEGs with the following characteristics:

- **COM marker**: `"JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech."`
- **Quality scaling**: IJG formula (`5000/q` for q<50, `200-2q` for q>=50)
- **Baseline DCT encoding** (SOF0, 8-bit precision)
- **Standard ITU-T T.81 Annex K Huffman tables**
- **Standard ITU-T T.81 Annex K quantization base tables**

## Basic Usage

### Encoding to a file

```go
import (
    "image"
    "os"

    "github.com/0verkilll/weeksjpegencoder"
)

func encodeImage(img image.Image, filename string) error {
    f, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer f.Close()

    enc, err := weeksjpegencoder.NewWeeksEncoder(f, 75) // Quality 75
    if err != nil {
        return err
    }

    return enc.Encode(img)
}
```

### Using the convenience function

```go
data, err := weeksjpegencoder.WeeksEncodeToBytes(img, 75)
if err != nil {
    return err
}
// data contains the JPEG bytes
```

## Advanced Usage with Options

Using functional options for custom configuration:

```go
import (
    "github.com/0verkilll/weeksjpegencoder"
    "github.com/0verkilll/jpeg"
)

// High quality encoding with no chroma subsampling
enc, err := weeksjpegencoder.NewWeeksEncoderWithOptions(w, 95,
    weeksjpegencoder.WithSubsampling(jpeg.ChromaSubsampling444),
    weeksjpegencoder.WithComment("My Custom Encoder"),
)
if err != nil {
    return err
}
return enc.Encode(img)
```

## Testing with Mock Components

The interface-based design enables easy testing:

```go
// Create a mock quantizer that returns predictable values
mockQuant := &weeksjpegencoder.MockQuantizer{
    LumTable:   [64]int{16, 11, 10, /* ... */},
    ChromTable: [64]int{17, 18, 24, /* ... */},
}

// Create encoder with mock quantizer
enc, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(w, 75,
    weeksjpegencoder.WithQuantizer(mockQuant),
)

// Encode and verify behavior
enc.Encode(testImage)

// Assert quantizer was called expected number of times
if mockQuant.QuantizeCalls != expectedCalls {
    t.Errorf("unexpected quantize calls: %d", mockQuant.QuantizeCalls)
}
```

## Custom Component Implementation

### Implementing a custom quantizer

```go
type MyQuantizer struct {
    quality int
}

func (q *MyQuantizer) QuantizeBlock(block *[64]float64, isLuminance bool) [64]int {
    var result [64]int
    // Custom quantization logic
    for i := 0; i < 64; i++ {
        result[i] = int(block[i] / float64(q.quality))
    }
    return result
}

func (q *MyQuantizer) GetQuantTable(isLuminance bool) [64]int {
    // Return quantization table for DQT marker
    var table [64]int
    for i := 0; i < 64; i++ {
        table[i] = q.quality
    }
    return table
}

// Use with encoder
enc, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(w, 75,
    weeksjpegencoder.WithQuantizer(&MyQuantizer{quality: 50}),
)
```

## Chroma Subsampling

The encoder supports three chroma subsampling modes:

| Mode | Description | Use Case |
|------|-------------|----------|
| `ChromaSubsampling420` | 4:2:0 (default) | Smallest files, most common |
| `ChromaSubsampling422` | 4:2:2 | Horizontal-only subsampling |
| `ChromaSubsampling444` | 4:4:4 | No subsampling, highest quality |

### Example

```go
// Using method chaining
enc, _ := weeksjpegencoder.NewWeeksEncoder(w, 75)
enc.SetSubsampling(jpeg.ChromaSubsampling444)
enc.Encode(img)

// Or using functional options
enc, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(w, 75,
    weeksjpegencoder.WithSubsampling(jpeg.ChromaSubsampling444),
)
enc.Encode(img)
```

## Dependencies

This package requires the jpeg package:

```
github.com/0verkilll/jpeg
```

The jpeg package provides all low-level JPEG infrastructure including:

- `EncoderBitWriter` for entropy-coded data
- Standard Huffman tables (`StdEncoderDCLuminanceBits`, etc.)
- DCT implementations (`NewAANDCT`, `NewReferenceDCT`)
- Quantization tables and scaling (`StandardLuminanceQuantTable`, `ScaleQuantTable`)
- Marker writing functions (`WriteSOI`, `WriteSOF0`, `WriteDHT`, etc.)
- Color conversion (`NewBT601Converter`)
- Chroma subsampling modes (`ChromaSubsampling420`, etc.)

## Forensic Detection

Note that James R. Weeks encoder detection capabilities remain in the jpeg package for forensic analysis purposes. Use `jpeg.AnalyzeSignature` to detect Weeks-encoded images by examining COM markers and encoder characteristics.

## ITU-T T.81 Compliance

The encoder produces fully compliant baseline JPEG files per ITU-T T.81. All Huffman tables are from Annex K of the specification.
