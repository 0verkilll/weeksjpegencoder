// Package patterns provides test pattern definitions for byte-compatibility testing.
//
// This file contains pattern definitions that match the Java ReferenceGenerator exactly.
// The patterns are used to verify byte-level compatibility between Go and Java encoders.
//
// Pattern Types:
//   - solid: Uniform gray (128, 128, 128) - tests DC coefficient encoding
//   - horizontal_gradient: R varies 0-255 across width - tests low-frequency horizontal content
//   - vertical_gradient: G varies 0-255 across height - tests low-frequency vertical content
//   - diagonal_gradient: B varies 0-255 diagonally - tests diagonal frequency content
//   - checkerboard: 8x8 blocks alternating black/white - tests high-frequency DCT coefficients
//   - quadrant: Four different patterns in each quadrant - tests mixed content
//
// Test Dimensions:
//   - 8x8: Single MCU (minimum complete unit)
//   - 64x64: Standard test size
//   - 256x256: Comprehensive test size
//   - 33x33: Non-multiple of 8 (square)
//   - 100x75: Non-multiple of 8 (rectangular)
//
// Quality Levels:
//   - 1, 10, 25, 50, 75, 90, 95, 100

package patterns

// QualityLevels lists all quality levels tested for compatibility.
//
//nolint:unused
//goland:noinspection GoUnusedGlobalVariable
var QualityLevels = []int{1, 10, 25, 50, 75, 90, 95, 100}

// SubsamplingModes lists all subsampling modes tested.
// Note: Java ReferenceGenerator only supports 4:2:0, but Go encoder supports all three.
//
//nolint:unused
//goland:noinspection GoUnusedGlobalVariable
var SubsamplingModes = []string{"4:2:0", "4:2:2", "4:4:4"}
