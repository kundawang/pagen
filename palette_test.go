// SPDX-FileCopyrightText: 2025 Shun Sakai
//
// SPDX-License-Identifier: CC0-1.0

package main

import (
	"bytes"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

const (
	testSeed    = "42"
	testPalette = "#ff0000,#00ff00,#0000ff"
)

func testConfig(t *testing.T, width, height, blockSize uint) config {
	t.Helper()

	cfg, err := newConfig(width, height, blockSize, testSeed, true, testPalette, true)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func decodePNG(t *testing.T, path string) (pixels [][]color.NRGBA) {
	t.Helper()

	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	b := img.Bounds()
	pixels = make([][]color.NRGBA, b.Dy())
	for y := 0; y < b.Dy(); y++ {
		pixels[y] = make([]color.NRGBA, b.Dx())
		for x := 0; x < b.Dx(); x++ {
			pixels[y][x] = color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
		}
	}
	return pixels
}

type svgRect struct {
	x, y, w, h int
	c          color.NRGBA
}

var rectRe = regexp.MustCompile(
	`<rect x="(\d+)" y="(\d+)" width="(\d+)" height="(\d+)" style="fill-opacity:([\d.]+); fill:rgb\((\d+),(\d+),(\d+)\)"`)

func parseSVG(t *testing.T, path string) []svgRect {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	matches := rectRe.FindAllSubmatch(data, -1)
	if len(matches) == 0 {
		t.Fatalf("no rects found in %v", path)
	}

	rects := make([]svgRect, 0, len(matches))
	for _, m := range matches {
		num := func(i int) int {
			n, err := strconv.Atoi(string(m[i]))
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
		opacity, err := strconv.ParseFloat(string(m[5]), 64)
		if err != nil {
			t.Fatal(err)
		}
		rects = append(rects, svgRect{
			x: num(1), y: num(2), w: num(3), h: num(4),
			c: color.NRGBA{
				R: uint8(num(6)), G: uint8(num(7)), B: uint8(num(8)),
				A: uint8(opacity*255 + 0.5),
			},
		})
	}
	return rects
}

// TestSeededPNGReproducible generates a PNG repeatedly with a fixed seed and
// a three-color palette and requires the pixels to be identical every time,
// regardless of goroutine scheduling.
func TestSeededPNGReproducible(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t, 97, 53, 8)
	dir := t.TempDir()

	var reference [][]color.NRGBA
	for i := 0; i < 8; i++ {
		out := filepath.Join(dir, "out.png")
		if err := generatePNG(out, cfg); err != nil {
			t.Fatal(err)
		}
		pixels := decodePNG(t, out)
		if reference == nil {
			reference = pixels
			continue
		}
		for y := range reference {
			for x := range reference[y] {
				if pixels[y][x] != reference[y][x] {
					t.Fatalf("run %v differs at (%v, %v): got %v, want %v",
						i, x, y, pixels[y][x], reference[y][x])
				}
			}
		}
	}
}

// TestSeededSVGReproducible requires repeated SVG generation with a fixed
// seed and palette to be byte-identical.
func TestSeededSVGReproducible(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t, 97, 53, 8)
	dir := t.TempDir()

	var reference []byte
	for i := 0; i < 8; i++ {
		out := filepath.Join(dir, "out.svg")
		if err := generateSVG(out, cfg); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Clean(out))
		if err != nil {
			t.Fatal(err)
		}
		if reference == nil {
			reference = data
			continue
		}
		if !bytes.Equal(data, reference) {
			t.Fatalf("run %v produced a different SVG", i)
		}
	}
}

// TestPNGSVGCorrespondence requires the PNG and the SVG generated from the
// same seed and palette to assign the same color to every block, including
// the clipped blocks on the right and bottom edges.
func TestPNGSVGCorrespondence(t *testing.T) {
	t.Parallel()

	// 97x53 with block size 8 leaves partial blocks of 1x8, 8x5 and 1x5.
	cfg := testConfig(t, 97, 53, 8)
	dir := t.TempDir()

	pngPath := filepath.Join(dir, "out.png")
	if err := generatePNG(pngPath, cfg); err != nil {
		t.Fatal(err)
	}
	svgPath := filepath.Join(dir, "out.svg")
	if err := generateSVG(svgPath, cfg); err != nil {
		t.Fatal(err)
	}

	pixels := decodePNG(t, pngPath)
	rects := parseSVG(t, svgPath)

	if want := cfg.cols * cfg.rows; len(rects) != want {
		t.Fatalf("SVG contains %v rects, want %v", len(rects), want)
	}

	seen := make(map[[2]int]bool)
	for _, r := range rects {
		if seen[[2]int{r.x, r.y}] {
			t.Fatalf("block at (%v, %v) is drawn twice", r.x, r.y)
		}
		seen[[2]int{r.x, r.y}] = true

		if r.x%cfg.blockSize != 0 || r.y%cfg.blockSize != 0 {
			t.Fatalf("rect at (%v, %v) is not aligned to the grid", r.x, r.y)
		}
		if r.x+r.w > cfg.width || r.y+r.h > cfg.height {
			t.Fatalf("rect at (%v, %v) of size %vx%v overflows the canvas", r.x, r.y, r.w, r.h)
		}

		// Every pixel of the PNG inside this block must carry the color
		// of the SVG rect, so edge blocks follow the same grid cell.
		for y := r.y; y < r.y+r.h; y++ {
			for x := r.x; x < r.x+r.w; x++ {
				if pixels[y][x] != r.c {
					t.Fatalf("pixel (%v, %v) is %v, want %v from the SVG rect",
						x, y, pixels[y][x], r.c)
				}
			}
		}
	}

	// The palette restricts every block to one of the three colors.
	palette, err := parsePalette(testPalette)
	if err != nil {
		t.Fatal(err)
	}
	allowed := make(map[color.NRGBA]bool)
	for _, c := range palette {
		allowed[c] = true
	}
	for _, r := range rects {
		if !allowed[r.c] {
			t.Fatalf("rect at (%v, %v) has color %v outside the palette", r.x, r.y, r.c)
		}
	}
}

// TestDifferentSeedsDiffer is a sanity check that the seed actually feeds
// into the color assignment.
func TestDifferentSeedsDiffer(t *testing.T) {
	t.Parallel()

	a, err := newConfig(64, 64, 8, "1", true, testPalette, true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := newConfig(64, 64, 8, "2", true, testPalette, true)
	if err != nil {
		t.Fatal(err)
	}

	same := 0
	for cy := 0; cy < a.rows; cy++ {
		for cx := 0; cx < a.cols; cx++ {
			if a.colors.at(cx, cy) == b.colors.at(cx, cy) {
				same++
			}
		}
	}
	if same == a.cols*a.rows {
		t.Fatal("different seeds produced identical palettes")
	}
}

func TestNewConfigErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		width, height, blockSz uint
		seed                   string
		seedSet                bool
		colors                 string
		colorsSet              bool
	}{
		{"zero width", 0, 10, 1, "", false, "", false},
		{"zero height", 10, 0, 1, "", false, "", false},
		{"zero block size", 10, 10, 0, "", false, "", false},
		{"dimension overflow", math.MaxUint32, math.MaxUint32, math.MaxUint32, "", false, "", false},
		{"too many pixels", 1 << 20, 1 << 20, 1 << 20, "", false, "", false},
		{"too many blocks", 1 << 13, 1 << 13, 1, "", false, "", false},
		{"invalid seed", 10, 10, 1, "not-a-number", true, "", false},
		{"empty seed", 10, 10, 1, "", true, "", false},
		{"empty palette", 10, 10, 1, "", false, "", true},
		{"blank palette entry", 10, 10, 1, "", false, "#ff0000,,#00ff00", true},
		{"invalid color", 10, 10, 1, "", false, "#ff00zz", true},
		{"wrong color length", 10, 10, 1, "", false, "#ff00", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := newConfig(tc.width, tc.height, tc.blockSz, tc.seed, tc.seedSet, tc.colors, tc.colorsSet)
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestParsePalette(t *testing.T) {
	t.Parallel()

	palette, err := parsePalette("#f00, #00ff00, #0000ff80")
	if err != nil {
		t.Fatal(err)
	}
	want := []color.NRGBA{
		{R: 0xff, A: 0xff},
		{G: 0xff, A: 0xff},
		{B: 0xff, A: 0x80},
	}
	if len(palette) != len(want) {
		t.Fatalf("got %v colors, want %v", len(palette), len(want))
	}
	for i, c := range want {
		if palette[i] != c {
			t.Fatalf("color %v is %v, want %v", i, palette[i], c)
		}
	}
}

// TestUnseededStillRandom requires the default mode to keep producing
// varying output.
func TestUnseededStillRandom(t *testing.T) {
	t.Parallel()

	cfg, err := newConfig(64, 64, 64, "", false, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.colors.at(0, 0) == cfg.colors.at(0, 0) && cfg.colors.at(0, 0) == cfg.colors.at(0, 0) {
		t.Fatal("unseeded colors look constant")
	}
}
