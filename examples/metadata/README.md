# Metadata Preservation Example

Demonstrates preserving EXIF and ICC color profile metadata during re-encoding.

## Features

- Parse EXIF metadata from source JPEG
- Parse ICC color profile from source JPEG
- Preserve metadata when re-encoding
- Multi-segment ICC profile support (>64KB profiles)

## Usage

```bash
go run main.go source.jpg output.jpg
```

## Code Example

```go
// Preserve all metadata from source image
sourceData, _ := os.ReadFile("source.jpg")
enc, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(w, 75,
    weeksjpegencoder.WithSourceImageBytes(sourceData),
)
enc.Encode(img) // Output includes original EXIF and ICC profile

// Or parse and apply metadata separately
exifData, _ := weeksjpegencoder.ParseEXIFBytes(sourceData)
iccData, _ := weeksjpegencoder.ParseICCProfileBytes(sourceData)

enc, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(w, 75,
    weeksjpegencoder.WithEXIF(exifData),
    weeksjpegencoder.WithICCProfile(iccData),
)
```

## Expected Output

```
Parsing source image: photo.jpg
  EXIF data: 4567 bytes
  ICC profile: 2345 bytes

Encoding with metadata preservation...
  Output: output.jpg (quality 75)
  EXIF preserved: yes
  ICC preserved: yes
```

## Notes

- EXIF data is stored in APP1 markers (0xFFE1)
- ICC profiles are stored in APP2 markers (0xFFE2)
- Large ICC profiles (>64KB) are split across multiple APP2 segments
- If source has no metadata, encoding proceeds without errors
