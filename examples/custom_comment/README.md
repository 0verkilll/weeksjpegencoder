# Custom Comment Example

Demonstrates setting a custom COM marker comment in JPEG files.

## Features

- Set custom COM marker text
- Use builder pattern with SetComment()
- Use functional options with WithComment()
- Verify comment in output file

## Usage

```bash
go run main.go
```

## Code Example

```go
// Using builder pattern
enc, _ := weeksjpegencoder.NewWeeksEncoder(f, 75)
enc.SetComment("My Custom Encoder v1.0")
enc.Encode(img)

// Using functional options
enc, _ := weeksjpegencoder.NewWeeksEncoderWithOptions(f, 75,
    weeksjpegencoder.WithComment("Created by MyApp"),
)
enc.Encode(img)
```

## Expected Output

```
Default comment: JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech.
Custom comment: My Custom Encoder v1.0
Encoded with default comment: output_default.jpg
Encoded with custom comment: output_custom.jpg
```

## Notes

- The default COM marker is the James R. Weeks signature for F5 compatibility
- Custom comments can identify your application or add metadata
- COM marker text is limited to printable ASCII characters
