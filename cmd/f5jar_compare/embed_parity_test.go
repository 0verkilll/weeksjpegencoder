package main

// Weeks-path byte-oracle for f5messageembed vs the real f5.jar EMBED.
//
// f5.jar embeds BETWEEN quantization and Huffman, so the only valid byte-level
// comparison runs the embed through the SAME (byte-identical) weeks encoder:
//
//   1. weeks-encode the cover, tapping every quantized block -> flat cover coeffs
//   2. f5messageembed.Embed(cover, pw, msg) -> stego coeffs
//   3. weeks-encode AGAIN, the tap REPLACING each block with the stego coeffs
//      -> Go stego JPEG bytes (weeks Huffman, byte-identical to f5.jar's)
//   4. EmbedDriver runs the real f5.jar embed on the same pixels -> Java stego JPEG
//   5. assert the two stego JPEGs are byte-identical
//
// The block tap is the load-bearing hook (block_tap.go): it sees exactly the
// coefficients that end up in the byte stream, in encode (MCU) order — the same
// layout f5.jar's coeff[] uses. A coefficient-level diagnostic (decode both with
// one decoder, count differing coeffs) reports the residual ≤N-coeff gap.
//
// Run: F5JAR=/path/to/f5.jar go test -run EmbedParity -v   (needs Java + the
// compiled EmbedDriver.class in this dir).

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	mrand "math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0verkilll/f5messageembed"
	"github.com/0verkilll/jpeg"
	weeks "github.com/0verkilll/weeksjpegencoder"
)

func TestEmbedParityVsF5Jar(t *testing.T) {
	// SUPERSEDED by TestF5JarParity (embedparity_test.go). This weeks-tap harness
	// double-transforms the coefficient layout: it de-zigzags the tapped blocks
	// into f5messageembed's natural-order pool, embeds, then re-zigzags and
	// re-Huffman-encodes through the weeks encoder. The round-trip is self-
	// consistent for the COVER (covCoeffDiffs=0) but the de-zigzag + re-encode
	// pair reports spurious STEGO coefficient diffs even when the embed is
	// byte-identical. TestF5JarParity compares Go vs f5.jar entirely in the
	// decoder's coefficient domain (no weeks re-encode) and is the authoritative
	// parity check; this test is kept only as documentation of the pitfall.
	t.Skip("superseded by TestF5JarParity — weeks-tap layout double-transform yields false diffs")
	f5jar := envOr("F5JAR", "f5.jar")
	if _, err := os.Stat(f5jar); err != nil {
		t.Skipf("f5.jar not found at %s (set F5JAR)", f5jar)
	}
	if _, err := os.Stat("EmbedDriver.class"); err != nil {
		t.Skip("EmbedDriver.class missing in cwd — compile it first")
	}
	cp := "." + string(os.PathListSeparator) + f5jar
	tmp := t.TempDir()

	type kase struct {
		side, q, msgLen int
		seed            int64
	}
	sizes := []int{64, 96, 128, 160, 192, 256}
	qs := []int{40, 60, 75, 85, 95}
	msgs := []int{8, 40, 120, 300}
	var cases []kase
	for i := 0; i < 24; i++ {
		cases = append(cases, kase{sizes[i%len(sizes)], qs[i%len(qs)], msgs[i%len(msgs)], int64(i*1000 + 7)})
	}

	total, byteIdentical, worstCoeff := 0, 0, 0
	for ci, c := range cases {
		pw := fmt.Sprintf("pw-%d", c.seed)
		msg := make([]byte, c.msgLen)
		mrand.New(mrand.NewSource(c.seed)).Read(msg)
		img := drawRGBA(c.side, c.side, c.seed)

		// 1) tap-collect cover coefficients. The tap gives ZIGZAG-order blocks;
		// f5messageembed.Embed indexes a NATURAL-order array (it de-zigzags the
		// permutation index via ApplyDeZigZag). So de-zigzag each block on the
		// way in: naturalBlock[ApplyDeZigZag(z)] = tapBlock[z].
		var cover []int16
		tapCollect := func(_ int, _ bool, b *[64]int) {
			var nat [64]int16
			for z := 0; z < 64; z++ {
				nat[f5messageembed.ApplyDeZigZag(z)] = int16(b[z]) //nolint:gosec
			}
			cover = append(cover, nat[:]...)
		}
		if err := weeksEncode(img, c.q, tapCollect); err != nil {
			t.Fatalf("case %d cover encode: %v", ci, err)
		}

		// 1b) COVER parity check: is the no-embed cover byte-identical to f5.jar's
		// JarDriver cover? If NOT, the divergence is the ENCODER, not the embed.
		rawc := filepath.Join(tmp, fmt.Sprintf("cov%d.raw", ci))
		_ = writeRawPixels(rawc, img)
		jcov := filepath.Join(tmp, fmt.Sprintf("cov%d.jpg", ci))
		_ = exec.Command("java", "-cp", cp, "JarDriver", rawc, jcov, fmt.Sprintf("%d", c.q)).Run()
		var covBuf bytes.Buffer
		ce, _ := weeks.NewWeeksEncoder(&covBuf, c.q)
		_ = ce.Encode(img)
		jcovBytes, _ := os.ReadFile(jcov)
		covByteId := bytes.Equal(covBuf.Bytes(), jcovBytes)
		covCD, covFirst, _, _ := coeffDiffDetail(covBuf.Bytes(), jcovBytes)

		// 2) embed.
		stego := append([]int16(nil), cover...)
		res, err := f5messageembed.Embed(stego, pw, msg)
		if err != nil {
			t.Logf("case %2d embed error: %v (skipping)", ci, err)
			continue
		}

		// 3) tap-replace each block with the stego coefficients on re-encode.
		// res.Coefficients is NATURAL order; re-zigzag back for the encoder:
		// tapBlock[z] = naturalStego[ApplyDeZigZag(z)].
		var goBuf bytes.Buffer
		tapReplace := func(idx int, _ bool, b *[64]int) {
			base := idx * 64
			for z := 0; z < 64; z++ {
				b[z] = int(res.Coefficients[base+f5messageembed.ApplyDeZigZag(z)])
			}
		}
		enc, err := weeks.NewWeeksEncoderWithOptions(&goBuf, c.q, weeks.WithBlockTap(tapReplace))
		if err != nil {
			t.Fatalf("case %d stego encoder: %v", ci, err)
		}
		if err := enc.Encode(img); err != nil {
			t.Fatalf("case %d stego encode: %v", ci, err)
		}
		goStego := goBuf.Bytes()

		// 4) real f5.jar embed on the same pixels.
		raw := filepath.Join(tmp, fmt.Sprintf("c%d.raw", ci))
		if err := writeRawPixels(raw, img); err != nil {
			t.Fatal(err)
		}
		msgFile := filepath.Join(tmp, fmt.Sprintf("c%d.msg", ci))
		if err := os.WriteFile(msgFile, msg, 0o644); err != nil {
			t.Fatal(err)
		}
		javaJPG := filepath.Join(tmp, fmt.Sprintf("c%d_java.jpg", ci))
		cmd := exec.Command("java", "--add-opens=java.base/sun.security.provider=ALL-UNNAMED",
			"-cp", cp, "EmbedDriver", raw, javaJPG, fmt.Sprintf("%d", c.q), pw, msgFile)
		jout, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("case %d EmbedDriver: %v\n%s", ci, err, jout)
		}
		javaK := parseJavaK(string(jout))
		if javaK != res.KParameter {
			t.Logf("case %2d  *** k MISMATCH: go=%d java=%d ***", ci, res.KParameter, javaK)
		}
		javaStego, err := os.ReadFile(javaJPG)
		if err != nil {
			t.Fatal(err)
		}

		// 5) compare.
		total++
		identical := bytes.Equal(goStego, javaStego)
		if identical {
			byteIdentical++
		}
		cd, firstIdx, gv, jv := coeffDiffDetail(goStego, javaStego)
		if !identical && cd > worstCoeff {
			worstCoeff = cd
		}
		t.Logf("case %2d %dx%d Q%02d msg=%3dB k=%d -> COVERbyteId=%-5v coverCoeffDiffs=%d(@%d) | STEGObyteId=%-5v stegoCoeffDiffs=%d(@%d go=%d java=%d)",
			ci, c.side, c.side, c.q, c.msgLen, res.KParameter, covByteId, covCD, covFirst, identical, cd, firstIdx, gv, jv)
	}

	t.Logf("=== %d/%d byte-identical to f5.jar; worst coeff diff = %d ===", byteIdentical, total, worstCoeff)
	if byteIdentical != total {
		t.Errorf("NOT byte-identical: %d/%d (worst %d coeffs)", byteIdentical, total, worstCoeff)
	}
}

func weeksEncode(img *image.RGBA, q int, tap weeks.BlockTapFunc) error {
	enc, err := weeks.NewWeeksEncoderWithOptions(io_DiscardWriter{}, q, weeks.WithBlockTap(tap))
	if err != nil {
		return err
	}
	return enc.Encode(img)
}

// io_DiscardWriter avoids importing io just for Discard.
type io_DiscardWriter struct{}

func (io_DiscardWriter) Write(p []byte) (int, error) { return len(p), nil }

// coeffDiff decodes both stego JPEGs with ONE decoder (consistent layout) and
// counts differing coefficients — a meaningful residual-gap metric.
func coeffDiffDetail(a, b []byte) (count, firstIdx, goVal, javaVal int) {
	firstIdx = -1
	dec := jpeg.NewStandardDecoder()
	ca, ea := dec.ExtractCoefficients(a)
	cb, eb := dec.ExtractCoefficients(b)
	if ea != nil || eb != nil {
		return -1, -1, 0, 0
	}
	n := len(ca)
	if len(cb) < n {
		n = len(cb)
	}
	for i := 0; i < n; i++ {
		if ca[i] != cb[i] {
			count++
			if firstIdx < 0 {
				firstIdx, goVal, javaVal = i, ca[i], cb[i]
			}
		}
	}
	if len(ca) > len(cb) {
		count += len(ca) - len(cb)
	} else {
		count += len(cb) - len(ca)
	}
	return count, firstIdx, goVal, javaVal
}

func drawRGBA(w, h int, seed int64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := mrand.New(mrand.NewSource(seed))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x*13 + y*7 + r.Intn(64)) % 256),
				G: uint8((x*5 + y*17 + r.Intn(64)) % 256),
				B: uint8((x*y + x*3 + r.Intn(64)) % 256),
				A: 255,
			})
		}
	}
	return img
}

// parseJavaK reads f5.jar's "using (1, n, k) code" stdout line -> k.
func parseJavaK(s string) int {
	i := strings.Index(s, "using (1, ")
	if i < 0 {
		if strings.Contains(s, "using default code") {
			return 1
		}
		return -1
	}
	rest := s[i+len("using (1, "):]
	// rest = "<n>, <k>) code"
	c := strings.Index(rest, ", ")
	if c < 0 {
		return -1
	}
	rest = rest[c+2:]
	p := strings.Index(rest, ")")
	if p < 0 {
		return -1
	}
	k := 0
	for _, ch := range rest[:p] {
		if ch < '0' || ch > '9' {
			return -1
		}
		k = k*10 + int(ch-'0')
	}
	return k
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
