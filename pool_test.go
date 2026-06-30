// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file contains tests for memory pool functionality to ensure proper
// pooling behavior without affecting encoding output.

package weeksjpegencoder

import (
	"bytes"
	"sync"
	"testing"
)

// =============================================================================
// Fixed-Size Array Pool Tests
// =============================================================================

// TestPoolGetPutLifecycleIntBlockArray verifies that int block array pool
// properly returns and retrieves buffers compatible with BlockEncoder interface.
func TestPoolGetPutLifecycleIntBlockArray(t *testing.T) {
	// Get a buffer from the pool
	block := getIntBlockArray()
	if block == nil {
		t.Fatal("getIntBlockArray returned nil")
	}

	// Fill with test data
	for i := range block {
		block[i] = i * 5
	}

	// Return to pool
	putIntBlockArray(block)

	// Get another buffer
	block2 := getIntBlockArray()
	if block2 == nil {
		t.Fatal("getIntBlockArray returned nil on second call")
	}

	// Should be usable
	block2[0] = 999
	putIntBlockArray(block2)
}

// =============================================================================
// Encoding Integration Tests
// =============================================================================

// TestPoolReuseAcrossMultipleEncodings verifies that pools properly reuse
// buffers across multiple encoding operations without affecting output.
func TestPoolReuseAcrossMultipleEncodings(t *testing.T) {
	// Create a test image
	img := allocCreateGradientImage(32, 32)

	// Encode multiple times and verify consistent output
	var results [][]byte
	for i := 0; i < 5; i++ {
		var buf bytes.Buffer
		enc, err := NewWeeksEncoder(&buf, 75)
		if err != nil {
			t.Fatalf("Iteration %d: NewWeeksEncoder failed: %v", i, err)
		}

		err = enc.Encode(img)
		if err != nil {
			t.Fatalf("Iteration %d: Encode failed: %v", i, err)
		}

		results = append(results, buf.Bytes())
	}

	// All results should be identical (byte-for-byte)
	for i := 1; i < len(results); i++ {
		if !bytes.Equal(results[0], results[i]) {
			t.Errorf("Encoding %d differs from encoding 0", i)
		}
	}
}

// =============================================================================
// Thread Safety Tests
// =============================================================================

// TestPoolThreadSafety verifies that pool operations are safe for concurrent access.
func TestPoolThreadSafety(t *testing.T) {
	const numGoroutines = 10
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines) // 1 pool type (int block array)

	// Test int block array pool concurrency
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				block := getIntBlockArray()
				for k := 0; k < 64; k++ {
					block[k] = k
				}
				putIntBlockArray(block)
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

// =============================================================================
// Defer Cleanup Tests
// =============================================================================

// TestPoolProperCleanupWithDefer verifies that deferred pool returns work correctly.
func TestPoolProperCleanupWithDefer(t *testing.T) {
	// Simulate a function that uses defer for cleanup with array pool
	processBlockArray := func() (result int, err error) {
		block := getIntBlockArray()
		defer putIntBlockArray(block)

		for i := 0; i < 64; i++ {
			block[i] = i * 3
		}

		return block[0] + block[63], nil
	}

	result, err := processBlockArray()
	if err != nil {
		t.Fatalf("processBlockArray returned error: %v", err)
	}

	// 0*3 + 63*3 = 189
	if result != 189 {
		t.Errorf("Expected result 189, got %d", result)
	}

	// Run multiple times to stress the pool
	for i := 0; i < 100; i++ {
		_, err = processBlockArray()
		if err != nil {
			t.Fatalf("Iteration %d (array): processBlock returned error: %v", i, err)
		}
	}
}

// =============================================================================
// Byte-Identical Output Tests
// =============================================================================

// TestPoolEncodingWithPooledBuffers verifies that encoding with pooled buffers
// produces byte-identical output to non-pooled encoding.
func TestPoolEncodingWithPooledBuffers(t *testing.T) {
	img := allocCreateGradientImage(64, 64)

	// First encoding
	var buf1 bytes.Buffer
	enc1, err := NewWeeksEncoder(&buf1, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}
	if err := enc1.Encode(img); err != nil {
		t.Fatalf("First encode failed: %v", err)
	}

	// Second encoding (pools will have been warmed up)
	var buf2 bytes.Buffer
	enc2, err := NewWeeksEncoder(&buf2, 75)
	if err != nil {
		t.Fatalf("NewWeeksEncoder failed: %v", err)
	}
	if err := enc2.Encode(img); err != nil {
		t.Fatalf("Second encode failed: %v", err)
	}

	// Results must be identical
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("Encoding with pooled buffers produced different output")
		t.Logf("First encoding: %d bytes", len(buf1.Bytes()))
		t.Logf("Second encoding: %d bytes", len(buf2.Bytes()))
	}
}

// TestPoolNilHandling verifies that nil inputs are handled gracefully.
func TestPoolNilHandling(t *testing.T) {
	// These should not panic
	putIntBlockArray(nil)
}
