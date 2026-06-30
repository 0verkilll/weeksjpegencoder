# James R. Weeks JPEG Encoder Reference

This directory contains the original James R. Weeks Java JPEG encoder that this Go package reimplements.

## Contents

- `james-weeks-encoder-f5.jar` - Original Java implementation (F5 steganography variant)

## About the James R. Weeks Encoder

The James R. Weeks JPEG encoder is a pure Java implementation of the JPEG baseline DCT encoding algorithm. It was originally developed in 1998 and became widely used in steganography tools, most notably the F5 steganography algorithm.

### History

| Version | Date | Changes |
|---------|------|---------|
| 0.9 | 1998 | Initial release |
| 1.0 | 1998 | Fixed entropy encoder buffer flush |
| 1.0a | 1998 | Fixed dirty edges on non-8-multiple dimensions |

### Why This Encoder Matters

The James Weeks encoder has distinctive characteristics that make it identifiable in forensic analysis:

1. **COM Marker Signature**: Every JPEG produced contains the comment:
   ```
   JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech.
   ```

2. **F5 Steganography**: The F5 algorithm (by Andreas Westfeld) uses this encoder to create carrier images, making it a forensic indicator.

3. **Consistent Output**: Produces byte-identical output across platforms when given the same input and quality settings.

## Technical Details

### Architecture

The original Java encoder consists of these components:

| Class | Purpose |
|-------|---------|
| `JpegEncoder` | Main encoder orchestrator |
| `JpegInfo` | Image metadata and configuration |
| `DCT` | Discrete Cosine Transform implementation |
| `Huffman` | Huffman entropy coding |
| `Jpeg` | JPEG marker writing |

### Quality Scaling

The encoder uses the IJG (Independent JPEG Group) quality scaling formula:

```
For quality < 50:  scale = 5000 / quality
For quality >= 50: scale = 200 - (2 * quality)
```

This produces quality values compatible with libjpeg and most JPEG tools.

### Quantization Tables

Uses the standard ITU-T T.81 Annex K quantization tables:

**Luminance (Y) Base Table:**
```
16  11  10  16  24  40  51  61
12  12  14  19  26  58  60  55
14  13  16  24  40  57  69  56
14  17  22  29  51  87  80  62
18  22  37  56  68 109 103  77
24  35  55  64  81 104 113  92
49  64  78  87 103 121 120 101
72  92  95  98 112 100 103  99
```

**Chrominance (Cb/Cr) Base Table:**
```
17  18  24  47  99  99  99  99
18  21  26  66  99  99  99  99
24  26  56  99  99  99  99  99
47  66  99  99  99  99  99  99
99  99  99  99  99  99  99  99
99  99  99  99  99  99  99  99
99  99  99  99  99  99  99  99
99  99  99  99  99  99  99  99
```

### Huffman Tables

Uses standard ITU-T T.81 Annex K Huffman tables (not optimized per-image):
- DC Luminance
- DC Chrominance
- AC Luminance
- AC Chrominance

### Color Space

Converts RGB to YCbCr using BT.601 coefficients:
```
Y  =  0.299R + 0.587G + 0.114B
Cb = -0.1687R - 0.3313G + 0.5B + 128
Cr =  0.5R - 0.4187G - 0.0813B + 128
```

### Chroma Subsampling

Supports three modes:
- **4:2:0**: 2x2 subsampling (default, most common)
- **4:2:2**: 2x1 horizontal subsampling
- **4:4:4**: No subsampling

## F5 Steganography Integration

The F5 algorithm (Westfeld, 2001) uses this encoder because:

1. **Matrix Encoding**: F5 embeds data in DCT coefficients using matrix encoding
2. **Permutation**: DCT coefficients are permuted based on a password-derived sequence
3. **Shrinkage**: Only non-zero AC coefficients are modified, causing coefficient shrinkage
4. **Carrier Compatibility**: F5-encoded images must be re-encoded with the same encoder characteristics

### F5 Detection

Images can be identified as potential F5 carriers by:
1. Presence of the James Weeks COM marker
2. Statistical analysis of DCT coefficient distribution

## Running the Original Encoder

To use the original Java encoder:

```bash
# Extract the JAR
unzip james-weeks-encoder-f5.jar -d extracted/

# Compile (if needed)
cd extracted/james
javac *.java

# Run F5 embedding
java -jar james-weeks-encoder-f5.jar e -e secret.txt -p password cover.bmp stego.jpg

# Run F5 extraction
java -jar james-weeks-encoder-f5.jar x -p password -e extracted.txt stego.jpg
```

## License

The original James R. Weeks encoder is distributed under a BSD-style license:

```
Copyright (c) 1998, James R. Weeks and BioElectroMech.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice,
   this list of conditions, all files included with the source code, and
   the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE AUTHOR AND CONTRIBUTORS "AS IS" AND ANY
EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED.
```

This software is based in part on the work of the Independent JPEG Group (IJG).

## References

- [F5 Steganography Algorithm](https://www2.htw-dresden.de/~westfeld/publikationen/F5.pdf) - Andreas Westfeld, 2001
- [ITU-T T.81](https://www.w3.org/Graphics/JPEG/itu-t81.pdf) - JPEG Standard
- [Independent JPEG Group](http://www.ijg.org/) - libjpeg reference implementation
- [F5Android](https://github.com/harlo/F5Android) - Android F5 implementation using James encoder

## Forensic Analysis

For forensic detection of James Weeks-encoded images, check:

1. **COM Marker**: Look for "James R. Weeks" signature at byte offset ~20-80
2. **DQT Tables**: Verify quantization tables match ITU-T T.81 Annex K with IJG scaling
3. **DHT Tables**: Standard Huffman tables (not optimized)
4. **APP0 Marker**: May or may not have JFIF marker depending on configuration

```bash
# Quick check for James signature
strings image.jpg | grep -i "james"

# Detailed marker analysis
exiftool -v3 image.jpg | grep -A5 "COM"
```
