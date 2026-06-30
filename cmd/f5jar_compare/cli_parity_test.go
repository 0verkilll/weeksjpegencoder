package main

// CLI-ONLY f5.jar parity test — uses ONLY the f5.jar command-line `e` mode
// (proven deterministic on Java 25 with --add-exports), NO custom JarDriver/
// EmbedDriver classes (whose Toolkit.createImage async pixel path is the
// suspected Java-25 breakage). This isolates "is the Go port byte-faithful to
// f5.jar?" from "is the custom Java harness broken?".
//
// Method (per case):
//  1. Build cover pixels, encode to a baseline JPEG f5.jar can read (.jpg).
//  2. f5.jar `e -q Q cover.jpg base.jpg`        (no -e => encode only, the cover base)
//  3. f5.jar `e -q Q -p PW -e msg cover.jpg stego.jpg`  (the f5.jar stego)
//  4. Decode base.jpg -> cover coeffs; decode stego.jpg -> f5.jar stego coeffs.
//  5. Go: f5messageembed.Embed(coverCoeffs, PW, msg) -> Go stego coeffs.
//  6. Assert Go stego coeffs == f5.jar stego coeffs (byte-identical), and that
//     Go can EXTRACT f5.jar's message from f5.jar's stego.
//
// Run:
//   F5JAR=f5.jar \
//   go test . -run TestF5JarParity_CLI -count=1 -v

import (
	"bytes"
	stdsha1 "crypto/sha1" //nolint:gosec // F5 PRNG seed hash, not a security control
	"fmt"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	mrand "math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/0verkilll/f5messageembed"
	"github.com/0verkilll/f5messageextract"
	"github.com/0verkilll/f5prng"
	"github.com/0verkilll/fisheryates"
	"github.com/0verkilll/jpeg"
)

type cliHasher struct{}

func (cliHasher) Sum(d []byte) []byte { h := stdsha1.Sum(d); return h[:] }

func cliDrawJPEG(t *testing.T, path string, side int, seed int64) {
	t.Helper()
	r := mrand.New(mrand.NewSource(seed))
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			// smooth gradient + a little noise => realistic DCT spread, not flat
			rr := uint8((x*255/side + int(r.Intn(24))) & 0xff)
			gg := uint8((y*255/side + int(r.Intn(24))) & 0xff)
			bb := uint8(((x + y) * 255 / (2 * side)) & 0xff)
			img.Set(x, y, color.RGBA{rr, gg, bb, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// baseline JPEG f5.jar will decode to pixels; the exact encoding here is
	// irrelevant — f5.jar re-encodes via james from the decoded pixels.
	if err := stdjpeg.Encode(f, img, &stdjpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
}

func TestF5JarParity_CLI(t *testing.T) {
	f5jar := envOr("F5JAR", "f5.jar")
	if _, err := os.Stat(f5jar); err != nil {
		t.Skipf("f5.jar not found at %s (set F5JAR)", f5jar)
	}
	addExports := "--add-exports=java.base/sun.security.provider=ALL-UNNAMED"
	tmp := t.TempDir()

	sizes := []int{64, 96, 128, 160, 256}
	qs := []int{40, 60, 75, 85, 95}
	msgs := []int{1, 8, 40, 120}

	dec := jpeg.NewStandardDecoder()
	ext := f5messageextract.NewExtractor(cliHasher{}, fisheryates.NewFisherYates(),
		f5messageextract.WithPRNGFactory(f5prng.NewDefaultFactory()))

	embedOK, extractOK, total := 0, 0, 0
	for i := 0; i < 20; i++ {
		side, q, msgLen := sizes[i%len(sizes)], qs[i%len(qs)], msgs[i%len(msgs)]
		seed := int64(i*1000 + 7)
		pw := fmt.Sprintf("pw-%d", seed)
		msg := make([]byte, msgLen)
		mrand.New(mrand.NewSource(seed)).Read(msg)

		cover := filepath.Join(tmp, fmt.Sprintf("c%d.jpg", i))
		base := filepath.Join(tmp, fmt.Sprintf("c%d_base.jpg", i))
		stego := filepath.Join(tmp, fmt.Sprintf("c%d_stego.jpg", i))
		msgFile := filepath.Join(tmp, fmt.Sprintf("c%d.msg", i))
		cliDrawJPEG(t, cover, side, seed)
		if err := os.WriteFile(msgFile, msg, 0o644); err != nil {
			t.Fatal(err)
		}

		// 2. f5.jar cover-base encode (no -e): pure james encode of decoded pixels.
		if out, err := exec.Command("java", addExports, "-jar", f5jar,
			"e", "-q", fmt.Sprintf("%d", q), cover, base).CombinedOutput(); err != nil {
			t.Fatalf("case %d base encode: %v\n%s", i, err, out)
		}
		// 3. f5.jar stego embed.
		if out, err := exec.Command("java", addExports, "-jar", f5jar,
			"e", "-q", fmt.Sprintf("%d", q), "-p", pw, "-e", msgFile, cover, stego).CombinedOutput(); err != nil {
			t.Fatalf("case %d stego embed: %v\n%s", i, err, out)
		}

		baseBytes, _ := os.ReadFile(base)
		stegoBytes, _ := os.ReadFile(stego)
		coverC, e1 := dec.ExtractCoefficients(baseBytes)
		javaC, e2 := dec.ExtractCoefficients(stegoBytes)
		if e1 != nil || e2 != nil {
			t.Fatalf("case %d decode: %v %v", i, e1, e2)
		}
		if len(coverC) != len(javaC) {
			t.Fatalf("case %d coeff len mismatch base=%d stego=%d (re-quant differs!)", i, len(coverC), len(javaC))
		}

		goC := make([]int16, len(coverC))
		for j, v := range coverC {
			goC[j] = int16(v) //nolint:gosec
		}
		res, err := f5messageembed.Embed(goC, pw, msg)
		if err != nil {
			// Over-capacity cases (msg won't fit the cover) are not a parity
			// signal — f5.jar truncates and Go refuses; skip them entirely.
			t.Logf("case %2d %dx%d Q%02d msg=%3dB ncoef=%d: SKIP over-capacity (%v)",
				i, side, side, q, msgLen, len(coverC), err)
			continue
		}
		total++
		diffs, first := 0, -1
		for j := range goC {
			if int(goC[j]) != javaC[j] {
				diffs++
				if first < 0 {
					first = j
				}
			}
		}
		if diffs == 0 {
			embedOK++
		}
		jc := make([]int16, len(javaC))
		for j, v := range javaC {
			jc[j] = int16(v) //nolint:gosec
		}
		exRes, exErr := ext.Extract(jc, pw)
		exMatch := exErr == nil && bytes.Equal(exRes.Data, msg)
		if exMatch {
			extractOK++
		}
		t.Logf("case %2d %dx%d Q%02d msg=%3dB ncoef=%d: embed==%v (diffs=%d@%d k=%d) extract=%v",
			i, side, side, q, msgLen, len(coverC), diffs == 0, diffs, first, res.KParameter, exMatch)
	}
	t.Logf("=== CLI parity: embed byte-identical %d/%d ; f5.jar->Go extract %d/%d ===", embedOK, total, extractOK, total)
	if embedOK != total {
		t.Errorf("CLI embed NOT byte-identical in %d/%d", total-embedOK, total)
	}
	if extractOK != total {
		t.Errorf("CLI extract failed in %d/%d", total-extractOK, total)
	}
}
