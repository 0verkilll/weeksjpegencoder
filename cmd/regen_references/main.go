// regen_references generates the testdata/reference/4_2_0/ corpus by feeding
// every (pattern, dimension, quality) combination through f5.jar's
// james.JpegEncoder (via cmd/f5jar_compare/JarDriver), then writes a fresh
// manifest.sha256 alongside. Run from the project root after the f5.jar
// driver classes have been built (cmd/f5jar_compare/classes/ + JarDriver.class).
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

type pattern struct {
	name string
	draw func(w, h int) *image.RGBA
}

var qualityLevels = []int{1, 10, 25, 50, 75, 90, 95, 100}
var dimensions = [][2]int{{8, 8}, {64, 64}, {256, 256}, {33, 33}, {100, 75}}

// Pattern generators match Java ReferenceGenerator.java byte-for-byte.

func solidPattern(w, h int) *image.RGBA {
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
	denom := w - 1
	if denom < 1 {
		denom = 1
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := uint8((x * 255) / denom)
			img.SetRGBA(x, y, color.RGBA{r, 128, 128, 255})
		}
	}
	return img
}

func verticalGradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	denom := h - 1
	if denom < 1 {
		denom = 1
	}
	for y := 0; y < h; y++ {
		g := uint8((y * 255) / denom)
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{128, g, 128, 255})
		}
	}
	return img
}

func diagonalGradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	maxDist := w + h - 2
	if maxDist == 0 {
		maxDist = 1
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			b := uint8(((x + y) * 255) / maxDist)
			img.SetRGBA(x, y, color.RGBA{128, 128, b, 255})
		}
	}
	return img
}

func checkerboardPattern(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			isWhite := ((x/8)+(y/8))%2 == 0
			c := color.RGBA{0, 0, 0, 255}
			if isWhite {
				c = color.RGBA{255, 255, 255, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func quadrantPattern(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	midX := w / 2
	midY := h / 2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b int
			left := x < midX
			top := y < midY
			switch {
			case left && top:
				rDen := midX
				if rDen <= 0 {
					rDen = 1
				}
				gDen := midY
				if gDen <= 0 {
					gDen = 1
				}
				r = (x * 255) / rDen
				g = (y * 255) / gDen
				b = 128
			case !left && top:
				if (x+y)%2 == 0 {
					r, g, b = 255, 255, 255
				} else {
					r, g, b = 0, 0, 0
				}
			case left && !top:
				if (x/8)%2 == 0 {
					r, g, b = 200, 200, 200
				} else {
					r, g, b = 55, 55, 55
				}
			default:
				noise := ((x * 7) + (y * 13) + (x * y)) % 256
				base := ((x - midX) + (y - midY)) % 256
				// Java's modulo on negative ints can return negative — replicate it.
				r = (noise + base) / 2
				g = (256 - noise + base) / 2
				b = (noise + 256 - base) / 2
				if r < 0 {
					r = 0
				} else if r > 255 {
					r = 255
				}
				if g < 0 {
					g = 0
				} else if g > 255 {
					g = 255
				}
				if b < 0 {
					b = 0
				} else if b > 255 {
					b = 255
				}
			}
			img.SetRGBA(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
		}
	}
	return img
}

func writeRaw(path string, img *image.RGBA) error {
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

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	driverDir := filepath.Join(root, "cmd", "f5jar_compare")
	classesDir := filepath.Join(driverDir, "classes")
	driverClass := filepath.Join(driverDir, "JarDriver.class")
	if _, statErr := os.Stat(driverClass); statErr != nil {
		fail(fmt.Errorf("missing %s — build the JarDriver first", driverClass))
	}
	if _, statErr := os.Stat(classesDir); statErr != nil {
		fail(fmt.Errorf("missing %s — extract jar classes first", classesDir))
	}
	classpath := driverDir + string(os.PathListSeparator) + classesDir

	outDir := filepath.Join(root, "testdata", "reference", "4_2_0")
	if mkdirErr := os.MkdirAll(outDir, 0o755); mkdirErr != nil {
		fail(mkdirErr)
	}

	workdir, err := os.MkdirTemp("", "regen-refs-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(workdir)

	patterns := []pattern{
		{"solid", solidPattern},
		{"horizontal_gradient", horizontalGradient},
		{"vertical_gradient", verticalGradient},
		{"diagonal_gradient", diagonalGradient},
		{"checkerboard", checkerboardPattern},
		{"quadrant", quadrantPattern},
	}

	type entry struct {
		filename string
		hash     string
	}
	var entries []entry
	total := 0
	for _, p := range patterns {
		for _, dim := range dimensions {
			for _, q := range qualityLevels {
				w, h := dim[0], dim[1]
				img := p.draw(w, h)
				stem := fmt.Sprintf("%s_%dx%d_q%02d_420", p.name, w, h, q)
				raw := filepath.Join(workdir, stem+".raw")
				jpg := filepath.Join(outDir, stem+".jpg")
				if err := writeRaw(raw, img); err != nil {
					fail(fmt.Errorf("%s: %w", stem, err))
				}
				cmd := exec.Command("java", "-cp", classpath, "JarDriver", raw, jpg, fmt.Sprintf("%d", q))
				cmd.Stderr = os.Stderr
				if _, err := cmd.Output(); err != nil {
					fail(fmt.Errorf("java %s: %w", stem, err))
				}
				data, err := os.ReadFile(jpg)
				if err != nil {
					fail(err)
				}
				sum := sha256.Sum256(data)
				entries = append(entries, entry{
					filename: "4_2_0/" + stem + ".jpg",
					hash:     hex.EncodeToString(sum[:]),
				})
				total++
				fmt.Printf("[%3d] %s (%d bytes)\n", total, stem, len(data))
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].filename < entries[j].filename
	})
	manifestPath := filepath.Join(root, "testdata", "reference", "manifest.sha256")
	var mf bytes.Buffer
	fmt.Fprintln(&mf, "# SHA-256 manifest for f5.jar reference images")
	fmt.Fprintln(&mf, "# Generated by cmd/regen_references using docs/james-weeks-encoder-f5.jar")
	fmt.Fprintln(&mf)
	for _, e := range entries {
		fmt.Fprintf(&mf, "%s  %s\n", e.hash, e.filename)
	}
	if err := os.WriteFile(manifestPath, mf.Bytes(), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("\nWrote %d images, manifest at %s\n", total, manifestPath)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "regen_references:", err)
	os.Exit(1)
}
