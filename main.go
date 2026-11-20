// SPDX-FileCopyrightText: 2025 Shun Sakai
//
// SPDX-License-Identifier: CC0-1.0

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Parse()

	if opt.version {
		fmt.Printf("pagen %v\n", version)
		os.Exit(0)
	}

	n := flag.NArg()

	switch {
	case n == 0:
		fmt.Fprintln(os.Stderr, "pagen: a required argument was not provided")
		flag.Usage()
		os.Exit(1)
	case n > 1:
		fmt.Fprintf(os.Stderr, "pagen: unexpected argument '%v' found\n", flag.Arg(1))
		flag.Usage()
		os.Exit(1)
	}

	outputPath := flag.Arg(0)

	var seedSet, colorsSet bool
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "seed":
			seedSet = true
		case "colors":
			colorsSet = true
		}
	})

	cfg, err := newConfig(opt.width, opt.height, opt.blockSize, opt.seed, seedSet, opt.colors, colorsSet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pagen: %v\n", err)
		os.Exit(1)
	}

	if opt.svg {
		err := generateSVG(outputPath, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pagen: %v\n", err)
			os.Exit(1)
		}
	} else {
		err := generatePNG(outputPath, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pagen: %v\n", err)
			os.Exit(1)
		}
	}
}
