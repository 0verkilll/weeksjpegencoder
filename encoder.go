// Package weeksjpegencoder provides James R. Weeks-compatible JPEG encoding functionality.
//
// This file implements the WeeksEncoder which produces baseline JPEG files (SOF0)
// with the characteristic COM marker signature used by F5Android steganography tools.
//
// The encoder uses:
// - IJG quality scaling formula: scale = 5000/q for q<50, 200-2q for q>=50
// - Standard ITU-T T.81 Annex K base quantization tables
// - Standard Huffman tables from ITU-T T.81 Annex K
// - Baseline DCT encoding (SOF0, 8-bit precision)
//
// COM marker signature: "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech."
//
// This package depends on github.com/0verkilll/jpeg for the underlying
// JPEG encoding infrastructure.

package weeksjpegencoder

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"runtime"

	"github.com/0verkilll/jpeg"
)

// =============================================================================
// Weeks Encoder Constants
// =============================================================================

// weeksDefaultComment is the canonical James R. Weeks COM signature exactly
// as used by f5.jar (net/f5/Embed.java line 30). Note the TWO trailing spaces
// — they're part of the literal f5.jar compares against when deciding whether
// to emit JFIF version 1.00 vs 1.01, and they're consumed by f5.jar's COM
// length quirk (see writeJamesCOM) so they don't appear in the output file.
const weeksDefaultComment = "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech.  "

// =============================================================================
// WeeksEncoder - James R. Weeks-Compatible JPEG Encoder
// =============================================================================

// WeeksEncoder is a baseline JPEG encoder compatible with the James R. Weeks encoder.
// It produces standard baseline JPEG files (SOF0) with the James/BioElectroMech COM marker
// signature, making the output compatible with F5 steganography tools.
//
// Usage:
//
//	var buf bytes.Buffer
//	enc, err := NewWeeksEncoder(&buf, 75)
//	if err != nil {
//	    return err
//	}
//	err = enc.Encode(img)
type WeeksEncoder struct {
	writer      io.Writer
	quality     int
	comment     string
	subsampling jpeg.ChromaSubsamplingMode

	// Quantizer component (extracted per SOLID principles)
	quantizer Quantizer

	// James-compatible quantizer for byte-identical output
	jamesQuantizer *JamesQuantizer

	// BlockEncoder component (extracted per SOLID principles)
	// This can be injected via WithBlockEncoder option for testing
	blockEncoder BlockEncoder

	// blockTap, if set, is invoked for every quantized block before entropy
	// coding. The tap may mutate the block in place. Set via WithBlockTap.
	blockTap BlockTapFunc

	// BlockExtractor component (extracted per SOLID principles)
	// Note: For byte-identical output, this is overridden with JamesBlockExtractor
	blockExtractor BlockExtractor

	// DCT interface for dependency injection (used by WithDCT option)
	dctInterface DCT

	// Huffman encoder tables (stored for creating BlockEncoder)
	dcLumEncoder   *jpeg.HuffmanEncoderTable
	dcChromEncoder *jpeg.HuffmanEncoderTable
	acLumEncoder   *jpeg.HuffmanEncoderTable
	acChromEncoder *jpeg.HuffmanEncoderTable

	// Custom Huffman tables storage
	// Indexed as [tableClass][tableNum] where:
	//   tableClass: 0 = DC, 1 = AC
	//   tableNum: 0 = luminance, 1 = chrominance
	customHuffmanTables [2][2]*CustomHuffmanTable

	// lastHuffmanTableError stores the last error from SetHuffmanTable for debugging
	lastHuffmanTableError error

	// DCT transformer (concrete type for normal operation)
	dct jpeg.DCTTransformer

	// Color converter
	colorConv jpeg.ColorConverter

	// useJamesCompatibleMode enables byte-identical output with the James encoder
	useJamesCompatibleMode bool

	// Custom quantization tables (nil means use default)
	// Index 0 = luminance, Index 1 = chrominance
	customQuantTables [2]*[64]int

	// Error from SetQuantizationTable validation (checked during Encode)
	quantTableErr error

	// EXIF metadata preservation
	// exifData stores raw EXIF segment data (including "Exif\x00\x00" signature)
	exifData []byte
	// hasEXIF indicates whether EXIF data is present
	hasEXIF bool

	// ICC color profile preservation
	// iccProfile stores raw ICC profile data (without APP2 marker overhead)
	iccProfile []byte
	// hasICC indicates whether ICC profile data is present
	hasICC bool

	// parallelEncoding enables multi-core DCT+quantization for James-compatible
	// mode. The entropy (Huffman) stage stays strictly sequential in scan order,
	// so output is byte-identical regardless of worker count. Defaults to true.
	parallelEncoding bool

	// maxWorkers caps the number of goroutines used for the parallel DCT+quantize
	// phase. 0 means use runtime.GOMAXPROCS(0).
	maxWorkers int
}

// NewWeeksEncoderWithOptions creates a new James R. Weeks-compatible JPEG encoder with options.
//
// Parameters:
//   - w: Output writer for the JPEG data
//   - quality: Quality level from 1 to 100 (same as libjpeg)
//   - opts: Functional options to customize the encoder
//
// The encoder is initialized with:
//   - Default James R. Weeks COM marker signature
//   - 4:2:0 chroma subsampling (standard)
//   - Scaled quantization tables based on quality
//   - Standard Huffman tables from ITU-T T.81 Annex K
//
// Options can override these defaults. See WithQuantizer, WithBlockEncoder,
// WithBlockExtractor, WithDCT, WithComment, and WithSubsampling.
//
// Returns an error if quality is outside the valid range [1, 100].
func NewWeeksEncoderWithOptions(w io.Writer, quality int, opts ...Option) (*WeeksEncoder, error) {
	// Create James quantizer for byte-identical output
	jamesQuantizer, err := NewJamesQuantizer(quality)
	if err != nil {
		return nil, err
	}

	// Create default DCT (only used if jamesQuantizer is overridden).
	defaultDCT := jpeg.NewSeparableDCT()

	enc := &WeeksEncoder{
		writer:                 w,
		quality:                quality,
		comment:                weeksDefaultComment,
		subsampling:            jpeg.ChromaSubsampling420,
		jamesQuantizer:         jamesQuantizer,
		quantizer:              jamesQuantizer, // JamesQuantizer implements Quantizer
		blockExtractor:         NewYCbCrBlockExtractor(jpeg.ChromaSubsampling420),
		dct:                    defaultDCT,
		dctInterface:           NewDCTAdapter(defaultDCT),
		colorConv:              jpeg.NewBT601Converter(),
		useJamesCompatibleMode: true, // Default to James-compatible mode
		hasEXIF:                false,
		hasICC:                 false,
		parallelEncoding:       true, // Multi-core DCT+quantize; byte-identical output
		maxWorkers:             0,    // 0 = runtime.GOMAXPROCS(0)
	}

	// Initialize standard Huffman tables
	enc.dcLumEncoder = jpeg.NewStandardEncoderDCLuminanceTable()
	enc.dcChromEncoder = jpeg.NewStandardEncoderDCChrominanceTable()
	enc.acLumEncoder = jpeg.NewStandardEncoderACLuminanceTable()
	enc.acChromEncoder = jpeg.NewStandardEncoderACChrominanceTable()

	// Apply options
	for _, opt := range opts {
		opt(enc)
	}

	return enc, nil
}

// NewWeeksEncoder creates a new James R. Weeks-compatible JPEG encoder.
//
// Parameters:
//   - w: Output writer for the JPEG data
//   - quality: Quality level from 1 to 100 (same as libjpeg)
//
// The encoder is initialized with:
//   - Default James R. Weeks COM marker signature
//   - 4:2:0 chroma subsampling (standard)
//   - Scaled quantization tables based on quality
//   - Standard Huffman tables from ITU-T T.81 Annex K
//
// Returns an error if quality is outside the valid range [1, 100].
func NewWeeksEncoder(w io.Writer, quality int) (*WeeksEncoder, error) {
	return NewWeeksEncoderWithOptions(w, quality)
}

// SetComment sets a custom COM marker comment.
// Returns the encoder for method chaining.
//
// If not called, the default James R. Weeks signature is used:
// "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech."
func (e *WeeksEncoder) SetComment(comment string) *WeeksEncoder {
	e.comment = comment
	return e
}

// SetSubsampling sets the chroma subsampling mode.
// Returns the encoder for method chaining.
//
// Supported modes:
//   - jpeg.ChromaSubsampling420: 4:2:0 (default, most common)
//   - jpeg.ChromaSubsampling422: 4:2:2 (horizontal-only subsampling)
//   - jpeg.ChromaSubsampling444: 4:4:4 (no subsampling, highest quality)
func (e *WeeksEncoder) SetSubsampling(mode jpeg.ChromaSubsamplingMode) *WeeksEncoder {
	e.subsampling = mode
	// Update block extractor to use the new subsampling mode
	e.blockExtractor = NewYCbCrBlockExtractor(mode)
	return e
}

// SetQuantizationTable sets a custom quantization table for the specified component.
// Returns the encoder for method chaining.
//
// Parameters:
//   - tableNum: 0 for luminance (Y component), 1 for chrominance (Cb/Cr components)
//   - table: Array of exactly 64 quantization values in row-major order
//
// Table values must be in the range 1-255 per ITU-T T.81 specification.
// Custom tables override the standard ITU-T T.81 Annex K base tables that are
// normally scaled by the quality parameter.
//
// Any validation errors are stored internally and will cause Encode() to fail
// with a descriptive error message.
//
// Example:
//
//	// Set a flat luminance table (minimal quantization)
//	var lumTable [64]int
//	for i := range lumTable {
//	    lumTable[i] = 1
//	}
//	enc.SetQuantizationTable(0, lumTable)
func (e *WeeksEncoder) SetQuantizationTable(tableNum int, table [64]int) *WeeksEncoder {
	// Validate tableNum
	if tableNum < 0 || tableNum > 1 {
		e.quantTableErr = fmt.Errorf("invalid table number %d: must be 0 (luminance) or 1 (chrominance)", tableNum)
		return e
	}

	// Validate table values are in range 1-255
	for i, v := range table {
		if v < 1 || v > 255 {
			e.quantTableErr = fmt.Errorf("invalid quantization value %d at index %d: must be in range 1-255", v, i)
			return e
		}
	}

	// Store the custom table
	tableCopy := table // Make a copy to avoid external modification
	e.customQuantTables[tableNum] = &tableCopy

	// Update the quantizer with the custom table
	e.applyCustomQuantTable(tableNum, &tableCopy)

	return e
}

// applyCustomQuantTable applies a custom quantization table to the underlying quantizer.
// This updates both the stored quantization table and recalculates divisors for the
// James quantizer's integrated DCT+quantization.
func (e *WeeksEncoder) applyCustomQuantTable(tableNum int, table *[64]int) {
	if e.jamesQuantizer != nil {
		// Update the JamesQuantizer's internal tables
		for i := 0; i < 64; i++ {
			e.jamesQuantizer.quantTables[tableNum][i] = table[i]
		}

		// Recalculate divisors for AAN DCT (matches DCT.java exactly)
		// divisor = 1.0 / (quantTable * aanScale[row] * aanScale[col] * 8.0)
		for i := 0; i < 64; i++ {
			row := i / 8
			col := i % 8
			e.jamesQuantizer.divisors[tableNum][i] = 1.0 / (float64(table[i]) * aanScaleFactor[row] * aanScaleFactor[col] * 8.0)
		}
	}
}

// SetHuffmanTable sets a custom Huffman table for encoding.
// Returns the encoder for method chaining.
//
// Parameters:
//   - tableClass: 0 for DC, 1 for AC
//   - tableNum: 0 for luminance, 1 for chrominance
//   - bits: Array of 16 bytes specifying code lengths (bits[i] = count of codes with length i+1)
//   - values: Symbol values in order of increasing code length
//
// Validation:
//   - tableClass must be 0 or 1
//   - tableNum must be 0 or 1
//   - values length must equal sum of bits array
//
// If validation fails, the table is not set and the error is stored in lastHuffmanTableError.
// Custom tables override the standard ITU-T T.81 Huffman tables when encoding.
//
// Example:
//
//	// Set custom DC luminance table
//	bits := [16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
//	values := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
//	enc.SetHuffmanTable(0, 0, bits, values)
func (e *WeeksEncoder) SetHuffmanTable(tableClass, tableNum int, bits [16]byte, values []byte) *WeeksEncoder {
	// Clear previous error
	e.lastHuffmanTableError = nil

	// Create and validate the custom table
	customTable, err := NewCustomHuffmanTable(tableClass, tableNum, bits, values)
	if err != nil {
		e.lastHuffmanTableError = err
		return e
	}

	// Store the custom table
	e.customHuffmanTables[tableClass][tableNum] = customTable

	// Update the corresponding encoder table
	encoderTable := customTable.GetEncoderTable()
	switch {
	case tableClass == 0 && tableNum == 0:
		e.dcLumEncoder = encoderTable
	case tableClass == 0 && tableNum == 1:
		e.dcChromEncoder = encoderTable
	case tableClass == 1 && tableNum == 0:
		e.acLumEncoder = encoderTable
	case tableClass == 1 && tableNum == 1:
		e.acChromEncoder = encoderTable
	}

	return e
}

// GetHuffmanTableError returns the last error from SetHuffmanTable, if any.
// Returns nil if the last SetHuffmanTable call succeeded.
func (e *WeeksEncoder) GetHuffmanTableError() error {
	return e.lastHuffmanTableError
}

// HasCustomHuffmanTable returns true if a custom Huffman table is set for the
// specified class and number.
func (e *WeeksEncoder) HasCustomHuffmanTable(tableClass, tableNum int) bool {
	if tableClass < 0 || tableClass > 1 || tableNum < 0 || tableNum > 1 {
		return false
	}
	return e.customHuffmanTables[tableClass][tableNum] != nil
}

// hasAnyCustomHuffmanTable returns true if any custom Huffman table is set.
func (e *WeeksEncoder) hasAnyCustomHuffmanTable() bool {
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			if e.customHuffmanTables[i][j] != nil {
				return true
			}
		}
	}
	return false
}

// Encode encodes an image to JPEG format and writes it to the output writer.
//
// The encoding process:
//  1. Convert image to YCbCr color space
//  2. Apply chroma subsampling based on configured mode
//  3. Write JPEG structure: SOI, APP0, APP1 (EXIF if present), APP2 (ICC if present), COM, DQT, SOF0, DHT, SOS
//  4. Process image in MCUs (8x8 blocks, interleaved)
//  5. For each block: forward DCT, quantize, zigzag, entropy encode
//  6. Write EOI marker
//
// Note: The James encoder does NOT level shift before DCT. This differs from
// standard JPEG implementations but is required for byte-identical output.
//
// Returns an error if the image is nil or encoding fails.
func (e *WeeksEncoder) Encode(img image.Image) error {
	// Check for validation errors from SetQuantizationTable
	if e.quantTableErr != nil {
		return fmt.Errorf("quantization table error: %w", e.quantTableErr)
	}

	// Check for validation errors from SetHuffmanTable
	if e.lastHuffmanTableError != nil {
		return fmt.Errorf("huffman table error: %w", e.lastHuffmanTableError)
	}

	if img == nil {
		return errors.New("image cannot be nil")
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width <= 0 || height <= 0 {
		return errors.New("image dimensions must be positive")
	}

	// James-compatible mode mirrors f5.jar exactly, and f5.jar hardcodes
	// HsampFactor/VsampFactor to {2,1,1} — i.e. 4:2:0 chroma only. Refuse
	// 4:2:2 or 4:4:4 here rather than silently emit a frame header that
	// disagrees with the entropy data.
	if e.useJamesCompatibleMode && e.subsampling != jpeg.ChromaSubsampling420 {
		return fmt.Errorf("james-compatible mode only supports 4:2:0 chroma subsampling; use WithStandardMode for %v", e.subsampling)
	}

	// Get component specifications based on subsampling mode
	components := e.getComponentSpecs()
	scanComponents := e.getScanComponents()

	// Calculate MCU dimensions
	mcuWidth, mcuHeight, _ := jpeg.MCUDimensions(components)
	mcuCols := (width + mcuWidth - 1) / mcuWidth
	mcuRows := (height + mcuHeight - 1) / mcuHeight

	// Write JPEG structure
	if err := jpeg.WriteSOI(e.writer); err != nil {
		return err
	}

	// f5.jar writes JFIF v1.00 when the comment is the canonical James R. Weeks
	// signature, v1.01 otherwise. Mirror that for byte-identical output.
	jfifMinor := byte(0x01)
	if e.comment == weeksDefaultComment {
		jfifMinor = 0x00
	}
	if err := jpeg.WriteAPP0Version(e.writer, 0x01, jfifMinor); err != nil {
		return err
	}

	// Write APP1 (EXIF) marker if EXIF data is present
	if e.hasEXIF && len(e.exifData) > 0 {
		if err := writeAPP1EXIF(e.writer, e.exifData); err != nil {
			return err
		}
	}

	// Write APP2 (ICC) marker(s) if ICC profile data is present
	if e.hasICC && len(e.iccProfile) > 0 {
		if err := writeAPP2ICC(e.writer, e.iccProfile); err != nil {
			return err
		}
	}

	if err := writeJamesCOM(e.writer, e.comment); err != nil {
		return err
	}

	// Write quantization tables. In James-compatible mode the encoder body
	// (encodeImageDataJames) calls jamesQuantizer.ForwardDCTAndQuantize
	// directly, so the DQT marker MUST describe the same tables; using
	// e.quantizer here would diverge if a caller injected WithQuantizer.
	tableSource := e.quantizer
	if e.useJamesCompatibleMode {
		tableSource = e.jamesQuantizer
	}
	quantTables := map[int][64]int{
		0: tableSource.GetQuantTable(true),  // luminance
		1: tableSource.GetQuantTable(false), // chrominance
	}
	if err := jpeg.WriteDQT(e.writer, quantTables); err != nil {
		return err
	}

	// Write frame header
	if err := jpeg.WriteSOF0(e.writer, width, height, components); err != nil {
		return err
	}

	// Write Huffman tables
	// Use custom tables if any are set, otherwise use James-compatible format
	if e.hasAnyCustomHuffmanTable() {
		if err := e.writeCustomDHT(); err != nil {
			return err
		}
	} else if e.useJamesCompatibleMode {
		// Use James R. Weeks-compatible format for byte-identical output
		if err := writeJamesDHT(e.writer); err != nil {
			return err
		}
	} else {
		// Use standard DHT format for Go-decodable output
		if err := e.writeStandardDHT(); err != nil {
			return err
		}
	}

	// Write scan header
	if err := jpeg.WriteSOS(e.writer, scanComponents); err != nil {
		return err
	}

	// Encode image data using the appropriate iteration method
	if e.useJamesCompatibleMode {
		if err := e.encodeImageDataJames(img, components); err != nil {
			return err
		}
	} else {
		if err := e.encodeImageData(img, mcuCols, mcuRows, components); err != nil {
			return err
		}
	}

	// Write EOI
	if err := jpeg.WriteEOI(e.writer); err != nil {
		return err
	}

	return nil
}

// writeCustomDHT writes DHT markers for custom and/or standard Huffman tables.
// This is called when at least one custom Huffman table is set.
func (e *WeeksEncoder) writeCustomDHT() error {
	// Get the tables to write (custom if set, otherwise standard)
	specs := e.getHuffmanTableSpecs()

	// Write DHT marker using the jpeg package
	return jpeg.WriteDHT(e.writer, specs)
}

// writeStandardDHT writes DHT markers using standard JPEG format.
// This produces output that is decodable by Go's standard image/jpeg decoder.
//
// Unlike writeJamesDHT which writes all header bytes as 0x00 (a quirk of the
// original Java encoder), this method writes proper class/ID encoding:
//   - DC tables: class=0, id=0 (luminance) or id=1 (chrominance)
//   - AC tables: class=1, id=0 (luminance) or id=1 (chrominance)
func (e *WeeksEncoder) writeStandardDHT() error {
	specs := GetJamesStyleHuffmanSpecs()
	return jpeg.WriteDHT(e.writer, specs)
}

// getHuffmanTableSpecs returns Huffman table specs for DHT writing.
// Uses custom tables where set, standard tables otherwise.
func (e *WeeksEncoder) getHuffmanTableSpecs() []jpeg.HuffmanSpec {
	specs := make([]jpeg.HuffmanSpec, 4)

	// DC Luminance (class=0, id=0)
	if e.customHuffmanTables[0][0] != nil {
		t := e.customHuffmanTables[0][0]
		bits := [16]int{}
		for i, b := range t.bits {
			bits[i] = int(b)
		}
		specs[0] = jpeg.HuffmanSpec{Class: 0, ID: 0, Bits: bits, Values: t.GetValues()}
	} else {
		specs[0] = jpeg.HuffmanSpec{Class: 0, ID: 0, Bits: jpeg.StdEncoderDCLuminanceBits, Values: copyByteSlice(jpeg.StdEncoderDCLuminanceValues)}
	}

	// AC Luminance (class=1, id=0)
	if e.customHuffmanTables[1][0] != nil {
		t := e.customHuffmanTables[1][0]
		bits := [16]int{}
		for i, b := range t.bits {
			bits[i] = int(b)
		}
		specs[1] = jpeg.HuffmanSpec{Class: 1, ID: 0, Bits: bits, Values: t.GetValues()}
	} else {
		specs[1] = jpeg.HuffmanSpec{Class: 1, ID: 0, Bits: jpeg.StdEncoderACLuminanceBits, Values: copyByteSlice(jpeg.StdEncoderACLuminanceValues)}
	}

	// DC Chrominance (class=0, id=1)
	if e.customHuffmanTables[0][1] != nil {
		t := e.customHuffmanTables[0][1]
		bits := [16]int{}
		for i, b := range t.bits {
			bits[i] = int(b)
		}
		specs[2] = jpeg.HuffmanSpec{Class: 0, ID: 1, Bits: bits, Values: t.GetValues()}
	} else {
		specs[2] = jpeg.HuffmanSpec{Class: 0, ID: 1, Bits: jpeg.StdEncoderDCChrominanceBits, Values: copyByteSlice(jpeg.StdEncoderDCChrominanceValues)}
	}

	// AC Chrominance (class=1, id=1)
	if e.customHuffmanTables[1][1] != nil {
		t := e.customHuffmanTables[1][1]
		bits := [16]int{}
		for i, b := range t.bits {
			bits[i] = int(b)
		}
		specs[3] = jpeg.HuffmanSpec{Class: 1, ID: 1, Bits: bits, Values: t.GetValues()}
	} else {
		specs[3] = jpeg.HuffmanSpec{Class: 1, ID: 1, Bits: jpeg.StdEncoderACChrominanceBits, Values: copyByteSlice(jpeg.StdEncoderACChrominanceValues)}
	}

	return specs
}

// encodeImageDataJames encodes image data using James R. Weeks iteration pattern.
// This is necessary for byte-identical output with the original Java encoder.
//
// The Java encoder uses MinBlockWidth/MinBlockHeight iteration which is different
// from standard MCU-based iteration. It iterates in chroma block units (8 pixels)
// rather than MCU units (16 pixels for 4:2:0).
//
// f5.jar's DCT.forwardDCT level-shifts (subtracts 128) internally; the shift
// is applied inside ForwardDCTAndQuantize so the pixel values handed to it are
// the raw [0, 255] values out of the BT.601 conversion.
//
// Memory optimization: Uses pooled buffers for zigzagBlock to enable buffer reuse
// across multiple encoding operations. The pool is thread-safe for future parallel
// encoding support.
//
//goland:noinspection GoUnusedParameter
func (e *WeeksEncoder) encodeImageDataJames(img image.Image, components []jpeg.EncoderComponentSpec) error {
	bounds := img.Bounds()
	imageWidth := bounds.Dx()
	imageHeight := bounds.Dy()

	// Use injected blockEncoder if provided, otherwise create a new one
	var blockEncoder BlockEncoder
	if e.blockEncoder != nil {
		blockEncoder = e.blockEncoder
	} else {
		// Create James-compatible bit writer (pads with 0s, not 1s)
		bitWriter := NewJamesBitWriter(e.writer)

		// Create BlockEncoder using the James BitWriter and Huffman tables
		blockEncoder = NewHuffmanBlockEncoder(
			bitWriter,
			NewHuffmanTableAdapter(e.dcLumEncoder),
			NewHuffmanTableAdapter(e.dcChromEncoder),
			NewHuffmanTableAdapter(e.acLumEncoder),
			NewHuffmanTableAdapter(e.acChromEncoder),
		)
	}
	if e.blockTap != nil {
		blockEncoder = NewTapBlockEncoder(blockEncoder, e.blockTap)
	}

	// Create James-compatible block extractor
	blockExtractor := NewJamesBlockExtractor(img, e.subsampling)

	// DC predictors for each component
	dcPred := [3]int{0, 0, 0}

	// Sampling factors (matching Java's JpegInfo)
	hSampFactor := [3]int{2, 1, 1} // 4:2:0
	vSampFactor := [3]int{2, 1, 1}

	// Calculate MinBlockWidth and MinBlockHeight exactly like JpegEncoder.java
	// Initial value is image size rounded up to 8-pixel boundary
	minBlockWidth := imageWidth
	if imageWidth%8 != 0 {
		minBlockWidth = (int(math.Floor(float64(imageWidth)/8.0)) + 1) * 8
	}
	minBlockHeight := imageHeight
	if imageHeight%8 != 0 {
		minBlockHeight = (int(math.Floor(float64(imageHeight)/8.0)) + 1) * 8
	}

	// Calculate BlockWidth/Height for each component, mirroring f5.jar's
	// JpegInfo.getYCCArray():
	//   compWidth[i]  = imageWidth  / MaxHsampFactor * HsampFactor[i]
	//   BlockWidth[i] = ceil(compWidth[i] / 8.0)
	// (and likewise for height). When imageWidth isn't a multiple of 8, Java
	// rounds it up to the next multiple of 8 before downsampling.
	maxHsamp, maxVsamp := 1, 1
	for i := 0; i < 3; i++ {
		if hSampFactor[i] > maxHsamp {
			maxHsamp = hSampFactor[i]
		}
		if vSampFactor[i] > maxVsamp {
			maxVsamp = vSampFactor[i]
		}
	}
	paddedW := imageWidth
	if imageWidth%8 != 0 {
		paddedW = (imageWidth/8 + 1) * 8
	}
	paddedH := imageHeight
	if imageHeight%8 != 0 {
		paddedH = (imageHeight/8 + 1) * 8
	}
	blockWidth := [3]int{}
	blockHeight := [3]int{}
	for i := 0; i < 3; i++ {
		compW := paddedW / maxHsamp * hSampFactor[i]
		compH := paddedH / maxVsamp * vSampFactor[i]
		blockWidth[i] = int(math.Ceil(float64(compW) / 8.0))
		blockHeight[i] = int(math.Ceil(float64(compH) / 8.0))
	}

	// Get minimum (like JpegEncoder.java WriteCompressedData)
	for i := 0; i < 3; i++ {
		if blockWidth[i] < minBlockWidth {
			minBlockWidth = blockWidth[i]
		}
		if blockHeight[i] < minBlockHeight {
			minBlockHeight = blockHeight[i]
		}
	}

	// Enumerate every 8x8 block in the exact f5.jar WriteCompressedData scan
	// order. Each job records where its pixels come from; the slot index is its
	// position in this sequence, which the entropy phase replays verbatim.
	var jobs []jamesBlockJob
	for r := 0; r < minBlockHeight; r++ {
		for c := 0; c < minBlockWidth; c++ {
			xpos := c * 8
			ypos := r * 8
			for comp := 0; comp < 3; comp++ {
				for i := 0; i < vSampFactor[comp]; i++ {
					for j := 0; j < hSampFactor[comp]; j++ {
						jobs = append(jobs, jamesBlockJob{
							comp:         comp,
							xpos:         xpos,
							ypos:         ypos,
							yblockoffset: i * 8,
							xblockoffset: j * 8,
						})
					}
				}
			}
		}
	}

	dims := jamesBlockDims{
		imageWidth:  imageWidth,
		imageHeight: imageHeight,
		hSampFactor: hSampFactor,
		vSampFactor: vSampFactor,
	}

	// Parallelize the DCT+quantize phase when it's worth the coordination cost
	// and the runtime actually has spare cores. The entropy phase below is
	// always sequential, so this never changes the output bytes.
	workers := e.maxWorkers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if e.parallelEncoding && workers > 1 && len(jobs) >= parallelBlockThreshold {
		return e.encodeJamesParallel(blockEncoder, blockExtractor, jobs, dims, dcPred, workers)
	}
	return e.encodeJamesSequential(blockEncoder, blockExtractor, jobs, dims, dcPred)
}

// encodeImageData encodes all MCUs of the image using standard MCU iteration.
//
// Memory optimization: Uses pooled buffers for zigzagBlock to enable buffer reuse
// across multiple encoding operations. The pool is thread-safe for future parallel
// encoding support.
func (e *WeeksEncoder) encodeImageData(img image.Image, mcuCols, mcuRows int, components []jpeg.EncoderComponentSpec) error {
	// Use injected blockEncoder if provided, otherwise create a new one
	var blockEncoder BlockEncoder
	if e.blockEncoder != nil {
		blockEncoder = e.blockEncoder
	} else {
		// Create bit writer for entropy-coded data
		bitWriter := jpeg.NewEncoderBitWriter(e.writer)

		// Create BlockEncoder using the BitWriter and Huffman tables via adapters
		blockEncoder = NewHuffmanBlockEncoder(
			NewBitWriterAdapter(bitWriter),
			NewHuffmanTableAdapter(e.dcLumEncoder),
			NewHuffmanTableAdapter(e.dcChromEncoder),
			NewHuffmanTableAdapter(e.acLumEncoder),
			NewHuffmanTableAdapter(e.acChromEncoder),
		)
	}
	if e.blockTap != nil {
		blockEncoder = NewTapBlockEncoder(blockEncoder, e.blockTap)
	}

	// Create James-compatible block extractor for byte-identical output
	var blockExtractor BlockExtractor
	if e.useJamesCompatibleMode {
		blockExtractor = NewJamesBlockExtractor(img, e.subsampling)
	} else {
		blockExtractor = e.blockExtractor
	}

	// DC predictors for each component
	dcPred := [4]int{0, 0, 0, 0}

	// Get max sampling factors
	maxH, maxV := 1, 1
	for _, comp := range components {
		if comp.HSampling > maxH {
			maxH = comp.HSampling
		}
		if comp.VSampling > maxV {
			maxV = comp.VSampling
		}
	}

	// Get pooled buffer for zigzag block - this allows buffer reuse across multiple
	// encoding operations and is thread-safe for future parallel encoding.
	zigzagBlock := getIntBlockArray()
	defer putIntBlockArray(zigzagBlock)

	// Pre-allocate block array on stack for DCT input (doesn't escape)
	var block [64]float64

	// Process MCUs
	for mcuRow := 0; mcuRow < mcuRows; mcuRow++ {
		for mcuCol := 0; mcuCol < mcuCols; mcuCol++ {
			// Process each component in the MCU
			for compIdx, comp := range components {
				// Number of blocks for this component in the MCU
				blocksH := comp.HSampling
				blocksV := comp.VSampling

				for blockVIdx := 0; blockVIdx < blocksV; blockVIdx++ {
					for blockHIdx := 0; blockHIdx < blocksH; blockHIdx++ {
						// Calculate block position in image
						blockX := mcuCol*maxH*8 + blockHIdx*8*maxH/comp.HSampling
						blockY := mcuRow*maxV*8 + blockVIdx*8*maxV/comp.VSampling

						// Extract block using BlockExtractor interface
						block = blockExtractor.ExtractBlock(img, compIdx, blockX, blockY)

						// Level shift
						for i := 0; i < jpeg.BlockSize2; i++ {
							block[i] -= 128.0
						}

						isLuminance := compIdx == 0
						var quantized [64]int

						// Use James quantizer's integrated DCT+quantization for byte-identical output
						if e.jamesQuantizer != nil && e.useJamesCompatibleMode {
							quantized = e.jamesQuantizer.ForwardDCTAndQuantize(&block, isLuminance)
						} else {
							// Fallback to separate DCT and quantization
							if e.dctInterface != nil {
								e.dctInterface.Forward(&block)
							} else {
								e.dct.Forward(&block)
							}
							quantized = e.quantizer.QuantizeBlock(&block, isLuminance)
						}

						// Zigzag reorder - use pooled zigzagBlock buffer
						for i := 0; i < jpeg.BlockSize2; i++ {
							zigzagBlock[i] = quantized[jpeg.ZigzagOrder[i]]
						}

						// Encode block using BlockEncoder interface
						newDC, err := blockEncoder.EncodeBlock(zigzagBlock, dcPred[compIdx], isLuminance) // #nosec // G602: compIdx is always 0-2
						if err != nil {
							return err
						}
						dcPred[compIdx] = newDC // #nosec // G602: compIdx is always 0-2
					}
				}
			}
		}
	}

	// Flush remaining bits using BlockEncoder interface
	return blockEncoder.Flush()
}

// getComponentSpecs returns component specifications for the current subsampling mode.
func (e *WeeksEncoder) getComponentSpecs() []jpeg.EncoderComponentSpec {
	switch e.subsampling {
	case jpeg.ChromaSubsampling444:
		return jpeg.Get444ComponentSpecs()
	case jpeg.ChromaSubsampling422:
		return jpeg.Get422ComponentSpecs()
	case jpeg.ChromaSubsampling420:
		return jpeg.Get420ComponentSpecs()
	default:
		return jpeg.Get420ComponentSpecs()
	}
}

// getScanComponents returns scan component specifications for the current subsampling mode.
func (e *WeeksEncoder) getScanComponents() []jpeg.ScanComponent {
	switch e.subsampling {
	case jpeg.ChromaSubsampling444:
		return jpeg.Get444ScanComponents()
	case jpeg.ChromaSubsampling422:
		return jpeg.Get422ScanComponents()
	case jpeg.ChromaSubsampling420:
		return jpeg.Get420ScanComponents()
	default:
		return jpeg.Get420ScanComponents()
	}
}

// =============================================================================
// Convenience Functions
// =============================================================================

// WeeksEncodeToBytes encodes an image to JPEG bytes using the James R. Weeks encoder.
//
// This is a convenience function that creates an encoder, encodes the image,
// and returns the result as a byte slice.
//
// NOTE: This function uses James-compatible mode by default, which produces output
// that is byte-identical with the original James R. Weeks Java encoder but is NOT
// decodable by Go's standard image/jpeg decoder. Use WeeksEncodeToBytesStandard
// if you need Go-decodable output.
//
// Parameters:
//   - img: The image to encode
//   - quality: Quality level from 1 to 100
//
// Returns the encoded JPEG data or an error.
//
//goland:noinspection GoUnusedExportedFunction
func WeeksEncodeToBytes(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoder(&buf, quality)
	if err != nil {
		return nil, err
	}

	err = enc.Encode(img)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// WeeksEncodeToBytesStandard encodes an image to JPEG bytes in standard mode.
//
// This is a convenience function that creates an encoder with WithStandardMode(),
// encodes the image, and returns the result as a byte slice.
//
// Unlike WeeksEncodeToBytes, this function produces output that is decodable by
// Go's standard image/jpeg decoder and other standard JPEG decoders, but is NOT
// byte-identical with the original James R. Weeks Java encoder.
//
// Parameters:
//   - img: The image to encode
//   - quality: Quality level from 1 to 100
//
// Returns the encoded JPEG data or an error.
//
//goland:noinspection GoUnusedExportedFunction
func WeeksEncodeToBytesStandard(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	enc, err := NewWeeksEncoderWithOptions(&buf, quality, WithStandardMode())
	if err != nil {
		return nil, err
	}

	err = enc.Encode(img)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
