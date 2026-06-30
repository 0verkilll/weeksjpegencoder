# Basic Encoding Example

Demonstrates basic JPEG encoding with the weeksjpegencoder package.

## Features

- Encode an image to a JPEG file
- Use the convenience function for byte slice output
- Default Weeks encoder COM marker signature

## Usage

```bash
go run main.go
```

## Code Example

```go
// Encode to file
f, _ := os.Create("output.jpg")
defer f.Close()

enc, _ := weeksjpegencoder.NewWeeksEncoder(f, 75)
enc.Encode(img)

// Or use convenience function for bytes
data, _ := weeksjpegencoder.WeeksEncodeToBytes(img, 75)
```

## Expected Output

```
Created test image: 640x480 pixels
Encoded to file: output.jpg (quality 75)
Encoded to bytes: 12345 bytes (quality 85)
```

## Security Considerations

- Validate image dimensions before encoding
- Check for errors from all encoding operations
- Use appropriate quality levels (75-85 for typical use)
