// SPDX-FileCopyrightText: 2025 Shun Sakai
//
// SPDX-License-Identifier: CC0-1.0

package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
)

func generatePNG(outputPath string, cfg config) (err error) {
	img := image.NewNRGBA(image.Rect(0, 0, cfg.width, cfg.height))

	var wg sync.WaitGroup

	sem := make(chan struct{}, maxConcurrency)

	for cy := 0; cy < cfg.rows; cy++ {
		for cx := 0; cx < cfg.cols; cx++ {
			// The color is resolved from the grid position before the
			// goroutine starts, so the scheduling order cannot affect
			// the result.
			c := cfg.colors.at(cx, cy)
			x, y, w, h := cfg.blockRect(cx, cy)

			sem <- struct{}{}
			wg.Add(1)

			go func() {
				defer wg.Done()
				defer func() { <-sem }()

				for by := y; by < y+h; by++ {
					for bx := x; bx < x+w; bx++ {
						img.SetNRGBA(bx, by, c)
					}
				}
			}()
		}
	}

	wg.Wait()

	output, err := os.Create(filepath.Clean(outputPath))
	if err != nil {
		return err
	}

	defer func() {
		e := output.Close()
		if e != nil {
			err = e
		}
	}()

	return png.Encode(output, img)
}
