# Chroma Subsampling Example

Demonstrates different chroma subsampling modes and their impact on file size and quality.

## Features

- Compare 4:2:0, 4:2:2, and 4:4:4 subsampling modes
- Visualize file size differences
- Understand quality vs size tradeoffs

## Usage

```bash
go run main.go
```

## Code Example

```go
import "github.com/0verkilll/jpeg"

// 4:2:0 - Default, smallest files
enc.SetSubsampling(jpeg.ChromaSubsampling420)

// 4:2:2 - Horizontal-only subsampling
enc.SetSubsampling(jpeg.ChromaSubsampling422)

// 4:4:4 - No subsampling, highest quality
enc.SetSubsampling(jpeg.ChromaSubsampling444)
```

## Subsampling Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| 4:2:0 | Both horizontal and vertical subsampling | Web images, photos |
| 4:2:2 | Horizontal-only subsampling | Video, broadcast |
| 4:4:4 | No subsampling | High-quality graphics, text |

## Expected Output

```
Encoding with different subsampling modes...
4:2:0 (default): output_420.jpg - 15234 bytes
4:2:2:           output_422.jpg - 18456 bytes
4:4:4:           output_444.jpg - 22789 bytes
```

## Notes

- 4:2:0 is the default and produces the smallest files
- 4:4:4 preserves all color information but produces larger files
- For images with fine color detail or text, 4:4:4 provides better quality
