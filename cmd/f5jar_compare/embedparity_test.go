package main

// Coefficient-domain embed/extract parity vs the REAL f5.jar — the AUTHORITATIVE
// f5.jar parity test.
//
// It works entirely in the DCT coefficient domain — the same representation
// f5.jar embeds into and that jpeg.StandardDecoder.ExtractCoefficients /
// f5messageembed.Embed operate on in production — so it isolates the F5 algorithm
// from any JPEG re-encode noise:
//
//  1. JarDriver encodes the cover pixels to a JPEG with f5.jar's own encoder.
//  2. EmbedDriver F5-embeds the SAME pixels with f5.jar (forced to SHA1PRNG so
//     the result is deterministic; see EmbedDriver.forceSha1PrngDefault).
//  3. Decode both JPEGs to coefficients; Go-embed the same message into the
//     cover coefficients and assert the Go stego coefficients are byte-identical
//     to f5.jar's stego coefficients.
//  4. Independently, Go-EXTRACT the message back out of f5.jar's stego.
//
// Requires Java, f5.jar, and the compiled JarDriver.class + EmbedDriver.class in
// this directory. The f5.jar<->Go parity depends on the SHA-1(password) seed fix
// in f5messageembed/f5messageextract: Java F5Random seeds the SecureRandom with
// md.digest(password), NOT the raw password bytes.
//
// EmbedDriver MUST run with --add-opens=java.base/sun.security.provider=ALL-UNNAMED:
// on Java 9+ the default `new SecureRandom(seed)` is NativePRNG, which ignores the
// seed and is non-deterministic; EmbedDriver reinstalls SHA1PRNG as the default so
// f5.jar reproduces the permutation the algorithm was designed for.

import (
	"bytes"
	stdsha1 "crypto/sha1" //nolint:gosec // SHA-1 is the F5 PRNG seed hash, not a security control
	"fmt"
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

type parityHasher struct{}

func (parityHasher) Sum(d []byte) []byte { h := stdsha1.Sum(d); return h[:] }

func TestF5JarParity(t *testing.T) {
	f5jar := envOr("F5JAR", "f5.jar")
	if _, err := os.Stat(f5jar); err != nil {
		t.Skipf("f5.jar not found at %s (set F5JAR)", f5jar)
	}
	for _, cls := range []string{"JarDriver.class", "EmbedDriver.class"} {
		if _, err := os.Stat(cls); err != nil {
			t.Skipf("%s missing — compile JarDriver.java + EmbedDriver.java first", cls)
		}
	}
	cp := "." + string(os.PathListSeparator) + f5jar
	tmp := t.TempDir()

	type kase struct {
		side, q, msgLen int
		seed            int64
	}
	sizes := []int{64, 96, 128, 160, 256}
	qs := []int{40, 60, 75, 85, 95}
	msgs := []int{1, 8, 40, 120}
	var cases []kase
	for i := 0; i < 20; i++ {
		cases = append(cases, kase{sizes[i%len(sizes)], qs[i%len(qs)], msgs[i%len(msgs)], int64(i*1000 + 7)})
	}

	ext := f5messageextract.NewExtractor(parityHasher{}, fisheryates.NewFisherYates(),
		f5messageextract.WithPRNGFactory(f5prng.NewDefaultFactory()))

	embedOK, extractOK, total := 0, 0, 0
	for ci, c := range cases {
		pw := fmt.Sprintf("pw-%d", c.seed)
		msg := make([]byte, c.msgLen)
		mrand.New(mrand.NewSource(c.seed)).Read(msg)
		img := drawRGBA(c.side, c.side, c.seed)

		raw := filepath.Join(tmp, fmt.Sprintf("c%d.raw", ci))
		if err := writeRawPixels(raw, img); err != nil {
			t.Fatal(err)
		}
		msgFile := filepath.Join(tmp, fmt.Sprintf("c%d.msg", ci))
		if err := os.WriteFile(msgFile, msg, 0o644); err != nil {
			t.Fatal(err)
		}
		covJPG := filepath.Join(tmp, fmt.Sprintf("c%d_cov.jpg", ci))
		stgJPG := filepath.Join(tmp, fmt.Sprintf("c%d_stg.jpg", ci))

		if out, err := exec.Command("java", "-cp", cp, "JarDriver", raw, covJPG, fmt.Sprintf("%d", c.q)).CombinedOutput(); err != nil {
			t.Fatalf("case %d JarDriver: %v\n%s", ci, err, out)
		}
		eout, err := exec.Command("java", "--add-opens=java.base/sun.security.provider=ALL-UNNAMED",
			"-cp", cp, "EmbedDriver", raw, stgJPG, fmt.Sprintf("%d", c.q), pw, msgFile).CombinedOutput()
		if err != nil {
			t.Fatalf("case %d EmbedDriver: %v\n%s", ci, err, eout)
		}
		javaK := parseJavaK(string(eout))

		dec := jpeg.NewStandardDecoder()
		covBytes, _ := os.ReadFile(covJPG)
		stgBytes, _ := os.ReadFile(stgJPG)
		covC, e1 := dec.ExtractCoefficients(covBytes)
		javaC, e2 := dec.ExtractCoefficients(stgBytes)
		if e1 != nil || e2 != nil || len(covC) != len(javaC) {
			t.Fatalf("case %d decode/len: %v %v (%d vs %d)", ci, e1, e2, len(covC), len(javaC))
		}

		// Go embed into the f5.jar cover coefficients.
		cover := make([]int16, len(covC))
		for i, v := range covC {
			cover[i] = int16(v) //nolint:gosec
		}
		res, embErr := f5messageembed.Embed(cover, pw, msg)
		total++
		if embErr != nil {
			t.Errorf("case %d Go embed: %v", ci, embErr)
			continue
		}

		// (a) Embed parity: Go stego coeffs == f5.jar stego coeffs, byte for byte.
		diffs, first := 0, -1
		for i := range cover {
			if int(cover[i]) != javaC[i] {
				diffs++
				if first < 0 {
					first = i
				}
			}
		}
		kMatch := res.KParameter == javaK
		if diffs == 0 && kMatch {
			embedOK++
		}

		// (b) Cross-extract: Go recovers f5.jar's message from f5.jar's stego.
		exRes, exErr := ext.Extract(int16sFrom(javaC), pw)
		exMatch := exErr == nil && bytes.Equal(exRes.Data, msg)
		if exMatch {
			extractOK++
		}

		t.Logf("case %2d %dx%d Q%02d msg=%3dB: embed go==java=%v (diffs=%d@%d, k go=%d java=%d) | extract f5.jar->Go=%v",
			ci, c.side, c.side, c.q, c.msgLen, diffs == 0 && kMatch, diffs, first, res.KParameter, javaK, exMatch)
	}

	t.Logf("=== embed byte-identical: %d/%d ; f5.jar->Go extract: %d/%d ===", embedOK, total, extractOK, total)
	if embedOK != total {
		t.Errorf("embed NOT byte-identical to f5.jar in %d/%d cases", total-embedOK, total)
	}
	if extractOK != total {
		t.Errorf("Go failed to extract f5.jar message in %d/%d cases", total-extractOK, total)
	}
}

func int16sFrom(in []int) []int16 {
	out := make([]int16, len(in))
	for i, v := range in {
		out[i] = int16(v) //nolint:gosec
	}
	return out
}
