// SPDX-FileCopyrightText: 2025 Shun Sakai
//
// SPDX-License-Identifier: CC0-1.0

package main

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"math/rand/v2"
	"runtime"
	"strconv"
	"strings"
)

const (
	// maxBlocks bounds the total number of blocks in a single image so
	// that a tiny block size combined with large dimensions fails fast
	// instead of spawning an unbounded amount of work.
	maxBlocks = 1 << 24

	// maxPixels bounds the total number of pixels so that the image
	// dimensions cannot overflow or exhaust memory when materialized.
	maxPixels = 1 << 28
)

// maxConcurrency bounds the number of goroutines used to fill blocks. The
// result never depends on this value, only the resource usage does.
var maxConcurrency = max(runtime.GOMAXPROCS(0), 1)

// grid describes how an image is divided into uniformly sized blocks. The
// last column and row may be smaller than blockSize when the dimensions are
// not a multiple of it.
type grid struct {
	width, height int
	blockSize     int
	cols, rows    int
}

// newGrid validates the dimensions and computes the block layout.
func newGrid(width, height, blockSize int) (grid, error) {
	if width <= 0 || height <= 0 {
		return grid{}, fmt.Errorf("width and height must be greater than 0: got %vx%v", width, height)
	}
	if blockSize <= 0 {
		return grid{}, fmt.Errorf("block size must be greater than 0: got %v", blockSize)
	}
	if width > maxPixels/height {
		return grid{}, fmt.Errorf(
			"dimensions %vx%v are too large: must contain at most %v pixels",
			width, height, maxPixels,
		)
	}

	// (n-1)/blockSize+1 avoids the overflow of n+blockSize-1.
	cols := (width-1)/blockSize + 1
	rows := (height-1)/blockSize + 1
	if cols > maxBlocks/rows {
		return grid{}, fmt.Errorf(
			"too many blocks: %vx%v blocks of size %v exceeds the limit of %v blocks",
			width, height, blockSize, maxBlocks,
		)
	}

	return grid{width, height, blockSize, cols, rows}, nil
}

// blockRect returns the pixel rectangle covered by the block at grid
// position (cx, cy), clipped to the image bounds.
func (g grid) blockRect(cx, cy int) (x, y, w, h int) {
	x = cx * g.blockSize
	y = cy * g.blockSize
	w = min(g.blockSize, g.width-x)
	h = min(g.blockSize, g.height-y)
	return x, y, w, h
}

// colorSource assigns a color to each block. When seeded, the color depends
// only on the seed, the palette and the grid position of the block, so the
// result is reproducible and independent of the order in which blocks are
// processed. When unseeded, colors are random as before.
type colorSource struct {
	seed    uint64
	seeded  bool
	palette []color.NRGBA
}

// at returns the color of the block at grid position (cx, cy).
func (cs colorSource) at(cx, cy int) color.NRGBA {
	if !cs.seeded {
		if len(cs.palette) == 0 {
			return generateNRGBA()
		}
		return cs.palette[rand.N(len(cs.palette))]
	}

	h := cs.hash(cx, cy)
	if len(cs.palette) == 0 {
		return color.NRGBA{uint8(h), uint8(h >> 8), uint8(h >> 16), math.MaxUint8}
	}
	return cs.palette[h%uint64(len(cs.palette))]
}

// hash mixes the seed with the grid position using the splitmix64 finalizer,
// which is deterministic and platform independent.
func (cs colorSource) hash(cx, cy int) uint64 {
	z := cs.seed + uint64(cx)*0x9e3779b97f4a7c15 + uint64(cy)*0xc2b2ae3d27d4eb4f
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// config is the validated input shared by the PNG and SVG generators.
type config struct {
	grid
	colors colorSource
}

// newConfig validates the command-line input and builds the configuration.
// seedSet and colorsSet report whether the flags were given explicitly, so
// that an explicitly empty value is an error rather than the default.
func newConfig(width, height, blockSize uint, seed string, seedSet bool, colors string, colorsSet bool) (config, error) {
	if width > math.MaxInt || height > math.MaxInt || blockSize > math.MaxInt {
		return config{}, fmt.Errorf(
			"dimensions are too large: got %vx%v with block size %v",
			width, height, blockSize,
		)
	}

	g, err := newGrid(int(width), int(height), int(blockSize))
	if err != nil {
		return config{}, err
	}

	var cs colorSource
	if seedSet {
		n, err := strconv.ParseUint(seed, 0, 64)
		if err != nil {
			return config{}, fmt.Errorf("invalid seed %q: %v", seed, err)
		}
		cs.seed, cs.seeded = n, true
	}
	if colorsSet {
		palette, err := parsePalette(colors)
		if err != nil {
			return config{}, err
		}
		cs.palette = palette
	}

	return config{g, cs}, nil
}

// parsePalette parses a comma-separated list of hex colors.
func parsePalette(list string) ([]color.NRGBA, error) {
	if strings.TrimSpace(list) == "" {
		return nil, errors.New("palette must contain at least one color")
	}

	parts := strings.Split(list, ",")
	palette := make([]color.NRGBA, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("palette contains an empty color: %q", list)
		}
		c, err := parseColor(part)
		if err != nil {
			return nil, err
		}
		palette = append(palette, c)
	}
	return palette, nil
}

// parseColor parses a hex color in "#rgb", "#rrggbb" or "#rrggbbaa" form.
func parseColor(s string) (color.NRGBA, error) {
	hex := strings.TrimPrefix(s, "#")
	if len(hex) != 3 && len(hex) != 6 && len(hex) != 8 {
		return color.NRGBA{}, fmt.Errorf("invalid color %q: must be #rgb, #rrggbb or #rrggbbaa", s)
	}

	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("invalid color %q: %v", s, err)
	}

	switch len(hex) {
	case 3:
		r := uint8(v>>8&0xf) * 0x11
		g := uint8(v>>4&0xf) * 0x11
		b := uint8(v&0xf) * 0x11
		return color.NRGBA{R: r, G: g, B: b, A: math.MaxUint8}, nil
	case 6:
		return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: math.MaxUint8}, nil
	default:
		return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, nil
	}
}
