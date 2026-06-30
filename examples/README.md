# weeksjpegencoder Examples

This folder contains runnable examples demonstrating various features of the weeksjpegencoder package.

## Available Examples

| Example | Description |
|---------|-------------|
| [basic/](basic/) | Simple encoding to file and byte slice |
| [custom_comment/](custom_comment/) | Setting custom COM marker comments |
| [subsampling/](subsampling/) | Chroma subsampling modes (4:2:0, 4:2:2, 4:4:4) |
| [metadata/](metadata/) | EXIF and ICC profile preservation |

## Running Examples

Each example is a standalone Go module. To run an example:

```bash
cd examples/basic
go run main.go
```

## Example Structure

Each example folder contains:
- `go.mod` - Module definition with dependencies
- `main.go` - Example code
- `README.md` - Documentation and expected output

## Quick Links

### Basic Usage
```go
enc, _ := weeksjpegencoder.NewWeeksEncoder(w, 75)
enc.Encode(img)
```

### Convenience Function
```go
data, _ := weeksjpegencoder.WeeksEncodeToBytes(img, 75)
```

### Custom Comment
```go
enc.SetComment("My Custom Encoder")
```

### Chroma Subsampling
```go
enc.SetSubsampling(jpeg.ChromaSubsampling444)
```

### Metadata Preservation
```go
enc, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(w, 75,
    weeksjpegencoder.WithSourceImageBytes(sourceData),
)
```
