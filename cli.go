// SPDX-FileCopyrightText: 2025 Shun Sakai
//
// SPDX-License-Identifier: CC0-1.0

package main

import (
	"flag"
	"fmt"
	"log"
)

type options struct {
	width     uint
	height    uint
	blockSize uint
	seed      string
	colors    string
	svg       bool
	version   bool
}

var opt options

func init() {
	flag.UintVar(&opt.width, "width", 256, "Set the width of pixel art")
	flag.UintVar(&opt.height, "height", 256, "Set the height of pixel art")
	flag.UintVar(&opt.blockSize, "block-size", 1, "Set the block size of pixel art")
	flag.StringVar(&opt.seed, "seed", "", "Set the seed for reproducible colors")
	flag.StringVar(&opt.colors, "colors", "", "Set a comma-separated palette of hex colors")
	flag.BoolVar(&opt.svg, "svg", false, "Generate SVG file instead of PNG file")
	flag.BoolVar(&opt.version, "version", false, "Print version number")

	flag.Usage = func() {
		if _, err := fmt.Fprintf(
			flag.CommandLine.Output(),
			"Usage: pagen [OPTIONS] <FILE>\n",
		); err != nil {
			log.Fatal(err)
		}

		flag.PrintDefaults()
	}
}
