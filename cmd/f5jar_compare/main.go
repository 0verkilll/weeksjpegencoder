// f5jar_compare encodes a set of synthetic test images with two encoders —
// f5.jar's james.JpegEncoder (via a Java driver) and weeksjpegencoder — and
// reports byte-for-byte diffs between the two outputs.
//
// Run from the cmd/f5jar_compare directory:
//
//	javac -cp classes JarDriver.java   # one-time, see prepare-classes script
//	go run .
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0verkilll/jpeg"
	weeks "github.com/0verkilll/weeksjpegencoder"
)

type pattern struct {
	name string
	draw func(w, h int) *image.RGBA
}

type testCase struct {
	pattern pattern
	width   int
	height  int
	quality int
}

func solid(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := color.RGBA{128, 128, 128, 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func horizontalGradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(x * 255 / max(w-1, 1))
			img.SetRGBA(x, y, color.RGBA{v, 0, 0, 255})
		}
	}
	return img
}

func verticalGradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(y * 255 / max(h-1, 1))
			img.SetRGBA(x, y, color.RGBA{0, v, 0, 255})
		}
	}
	return img
}

func checkerboard(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			on := ((x / 8) + (y / 8)) & 1
			if on == 1 {
				img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
	}
	return img
}

// writeRawPixels writes the binary input format expected by JarDriver:
// int32 width, int32 height, then width*height int32s of 0xAARRGGBB.
func writeRawPixels(path string, img *image.RGBA) error {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, int32(w)); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, int32(h)); err != nil {
		return err
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.RGBAAt(x, y)
			argb := uint32(c.A)<<24 | uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
			if err := binary.Write(buf, binary.BigEndian, argb); err != nil {
				return err
			}
		}
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

type result struct {
	tc         testCase
	javaSize   int
	goSize     int
	identical  bool
	firstDiff  int
	diffCount  int
	javaSample []byte
	goSample   []byte
	err        error
}

func diffBytes(a, b []byte) (firstDiff, count int) {
	firstDiff = -1
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			if firstDiff < 0 {
				firstDiff = i
			}
			count++
		}
	}
	if len(a) != len(b) {
		if firstDiff < 0 {
			firstDiff = minLen
		}
		count += abs(len(a) - len(b))
	}
	return
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func runOne(tc testCase, workdir, javaCP string) result {
	res := result{tc: tc}
	img := tc.pattern.draw(tc.width, tc.height)

	stem := fmt.Sprintf("%s_%dx%d_q%02d", tc.pattern.name, tc.width, tc.height, tc.quality)
	rawPath := filepath.Join(workdir, stem+".raw")
	javaJPG := filepath.Join(workdir, stem+"_java.jpg")

	if err := writeRawPixels(rawPath, img); err != nil {
		res.err = err
		return res
	}

	cmd := exec.Command("java", "-cp", javaCP, "JarDriver", rawPath, javaJPG, fmt.Sprintf("%d", tc.quality))
	cmd.Stderr = os.Stderr
	if out, err := cmd.Output(); err != nil {
		_ = out
		res.err = fmt.Errorf("java: %w", err)
		return res
	}

	javaBytes, err := os.ReadFile(javaJPG)
	if err != nil {
		res.err = err
		return res
	}
	res.javaSize = len(javaBytes)

	var goBuf bytes.Buffer
	enc, err := weeks.NewWeeksEncoder(&goBuf, tc.quality)
	if err != nil {
		res.err = err
		return res
	}
	enc.SetSubsampling(jpeg.ChromaSubsampling420)
	if err := enc.Encode(img); err != nil {
		res.err = err
		return res
	}
	goBytes := goBuf.Bytes()
	res.goSize = len(goBytes)

	res.identical = bytes.Equal(javaBytes, goBytes)
	if !res.identical {
		res.firstDiff, res.diffCount = diffBytes(javaBytes, goBytes)
		s := res.firstDiff
		if s < 0 {
			s = 0
		}
		ctxStart := s - 4
		if ctxStart < 0 {
			ctxStart = 0
		}
		ctxEnd := s + 12
		if ctxEnd > len(javaBytes) {
			ctxEnd = len(javaBytes)
		}
		res.javaSample = append([]byte(nil), javaBytes[ctxStart:ctxEnd]...)
		ctxEnd = s + 12
		if ctxEnd > len(goBytes) {
			ctxEnd = len(goBytes)
		}
		res.goSample = append([]byte(nil), goBytes[ctxStart:ctxEnd]...)
		_ = os.WriteFile(filepath.Join(workdir, stem+"_go.jpg"), goBytes, 0o644)
	}
	return res
}

func main() {
	keep := flag.Bool("keep", false, "keep encoded jpegs in workdir for inspection")
	flag.Parse()

	scriptDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	classes := filepath.Join(scriptDir, "classes")
	if _, statErr := os.Stat(classes); statErr != nil {
		fmt.Fprintln(os.Stderr, "classes/ directory missing — run prepare-classes.sh first")
		os.Exit(1)
	}
	if _, statErr := os.Stat(filepath.Join(scriptDir, "JarDriver.class")); statErr != nil {
		fmt.Fprintln(os.Stderr, "JarDriver.class missing — run prepare-classes.sh first")
		os.Exit(1)
	}
	javaCP := scriptDir + string(os.PathListSeparator) + classes

	workdir, err := os.MkdirTemp("", "f5jar-compare-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !*keep {
		defer os.RemoveAll(workdir)
	} else {
		fmt.Println("workdir:", workdir)
	}

	patterns := []pattern{
		{"solid", solid},
		{"hgrad", horizontalGradient},
		{"vgrad", verticalGradient},
		{"checker", checkerboard},
	}
	sizes := [][2]int{{8, 8}, {16, 16}, {32, 32}, {64, 64}, {33, 33}}
	qualities := []int{50, 75, 80, 90}

	var cases []testCase
	for _, p := range patterns {
		for _, s := range sizes {
			for _, q := range qualities {
				cases = append(cases, testCase{pattern: p, width: s[0], height: s[1], quality: q})
			}
		}
	}

	results := make([]result, 0, len(cases))
	identical := 0
	for _, tc := range cases {
		r := runOne(tc, workdir, javaCP)
		results = append(results, r)
		if r.err != nil {
			fmt.Printf("ERR  %s %dx%d Q%02d: %v\n", tc.pattern.name, tc.width, tc.height, tc.quality, r.err)
			continue
		}
		if r.identical {
			identical++
			fmt.Printf("OK   %s %dx%d Q%02d (%d bytes)\n", tc.pattern.name, tc.width, tc.height, tc.quality, r.javaSize)
		} else {
			fmt.Printf("DIFF %s %dx%d Q%02d  java=%d  go=%d  first=%d  count=%d\n",
				tc.pattern.name, tc.width, tc.height, tc.quality,
				r.javaSize, r.goSize, r.firstDiff, r.diffCount)
		}
	}

	fmt.Println()
	fmt.Printf("Identical: %d / %d (%.1f%%)\n", identical, len(results), 100*float64(identical)/float64(len(results)))

	if identical < len(results) {
		fmt.Println("\nFirst diff byte context (java vs go):")
		sort.Slice(results, func(i, j int) bool {
			if results[i].diffCount != results[j].diffCount {
				return results[i].diffCount < results[j].diffCount
			}
			return results[i].firstDiff < results[j].firstDiff
		})
		shown := 0
		for _, r := range results {
			if r.identical || r.err != nil {
				continue
			}
			fmt.Printf("  %s %dx%d Q%02d @ offset %d\n",
				r.tc.pattern.name, r.tc.width, r.tc.height, r.tc.quality, r.firstDiff)
			fmt.Printf("    java: %s\n", hexish(r.javaSample))
			fmt.Printf("    go:   %s\n", hexish(r.goSample))
			shown++
			if shown >= 5 {
				break
			}
		}
	}
}

func hexish(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("%02x", v)
	}
	return strings.Join(parts, " ")
}
