# Java Reference Image Generator

This directory contains a standalone Java implementation of the James R. Weeks JpegEncoder, used to generate reference JPEG images for byte-level compatibility testing with the Go f5encoder package.

## Purpose

The f5encoder package aims to produce byte-identical output to the original James R. Weeks Java JPEG encoder used in F5Android. This Java tool generates reference images that can be compared byte-for-byte with Go encoder output.

## Files

| File | Description |
|------|-------------|
| `JpegEncoder.java` | Main JPEG encoder (standalone version of James R. Weeks encoder) |
| `JpegInfo.java` | Image metadata and RGB to YCbCr color conversion |
| `DCT.java` | Discrete Cosine Transform and quantization |
| `Huffman.java` | Huffman entropy coding |
| `ReferenceGenerator.java` | Test pattern generation and batch processing |
| `build.sh` | Build and run script |

## Requirements

- Java Development Kit (JDK) 8 or higher
- No external dependencies (uses only java.awt and java.io)

To check your Java version:
```bash
java -version
javac -version
```

## Building

Compile all Java files:
```bash
./build.sh compile
# or simply:
./build.sh
```

Clean compiled files:
```bash
./build.sh clean
```

## Generating Reference Images

Generate all reference images to the default output directory (`../reference/`):
```bash
./build.sh run
```

Generate to a custom directory:
```bash
./build.sh run /path/to/output
```

## Test Patterns

The generator creates the following test patterns:

| Pattern | Description | Purpose |
|---------|-------------|---------|
| `solid` | Uniform gray (128, 128, 128) | Tests DC coefficient encoding |
| `horizontal_gradient` | R varies 0-255 horizontally | Tests low-frequency horizontal content |
| `vertical_gradient` | G varies 0-255 vertically | Tests low-frequency vertical content |
| `diagonal_gradient` | B varies 0-255 diagonally | Tests diagonal frequency components |
| `checkerboard` | 8x8 alternating black/white blocks | Tests high-frequency DCT coefficients |
| `quadrant` | Four different patterns per quadrant | Tests mixed content handling |

## Dimensions

Reference images are generated at the following dimensions:

| Dimensions | Description |
|------------|-------------|
| 8x8 | Single MCU (Minimum Coded Unit) |
| 64x64 | Standard test size |
| 256x256 | Comprehensive test coverage |
| 33x33 | Non-multiple of 8 (square) |
| 100x75 | Non-multiple of 8 (rectangular) |

## Quality Levels

Images are generated at these JPEG quality levels:

- Q1 (lowest quality, smallest file)
- Q10
- Q25
- Q50 (medium quality)
- Q75 (standard quality)
- Q90
- Q95
- Q100 (highest quality, largest file)

Quality scaling follows the IJG (Independent JPEG Group) formula:
- Q < 50: scale = 5000 / quality
- Q >= 50: scale = 200 - 2*quality

## Output Structure

```
../reference/
├── 4_2_0/
│   ├── solid_8x8_q01_420.jpg
│   ├── solid_8x8_q10_420.jpg
│   ├── ...
│   ├── quadrant_256x256_q100_420.jpg
│   └── ...
└── manifest.sha256
```

## File Naming Convention

```
{pattern}_{width}x{height}_q{quality:02d}_{subsampling}.jpg
```

Example: `horizontal_gradient_64x64_q75_420.jpg`

## Subsampling Modes

Currently, the standalone encoder generates 4:2:0 (default) subsampling mode only. The original F5Android encoder supports:

- 4:2:0 - 2x2 chroma subsampling (default, most common)
- 4:2:2 - Horizontal-only 2x1 subsampling
- 4:4:4 - No subsampling (full chroma resolution)

For complete subsampling coverage, the Go encoder tests use the Go implementation's subsampling modes.

## Manifest File

The `manifest.sha256` file contains SHA-256 checksums of all generated images, used for integrity verification:

```
abc123...  4_2_0/solid_8x8_q01_420.jpg
def456...  4_2_0/solid_8x8_q10_420.jpg
...
```

## Attribution

The JpegEncoder implementation is based on:

> **JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech.**
>
> Based on the Independent JPEG Group's work (Thomas G. Lane's Jpeg 6a library).

The original F5Android source is available at:
https://github.com/guardianproject/F5Android

## Usage in Testing

The generated reference images are used by the Go compatibility tests:

```go
// In compatibility_test.go
func TestByteCompatibility(t *testing.T) {
    // Load Java reference
    javaData, _ := os.ReadFile("testdata/reference/4_2_0/solid_64x64_q75_420.jpg")

    // Generate Go output with same parameters
    goData, _ := generateWithGo("solid", 64, 64, 75)

    // Compare byte-by-byte
    if !bytes.Equal(javaData, goData) {
        t.Error("Byte mismatch detected")
    }
}
```

## Troubleshooting

### "javac not found"
Install JDK:
- macOS: `brew install openjdk`
- Ubuntu: `sudo apt install default-jdk`
- Windows: Download from https://adoptium.net/

### "java.lang.UnsupportedClassVersionError"
Your JDK version is too old. Upgrade to JDK 8 or higher.

### Generated images look different
Ensure you're using the exact same:
1. Test pattern generation algorithm
2. Quality level
3. Subsampling mode
4. Color space conversion (BT.601)
