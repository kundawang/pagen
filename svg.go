// SPDX-FileCopyrightText: 2025 Shun Sakai
//
// SPDX-License-Identifier: CC0-1.0

package main

import (
	"os"
	"path/filepath"

	svg "github.com/ajstarks/svgo"
)

func generateSVG(outputPath string, cfg config) (err error) {
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

	canvas := svg.New(output)

	canvas.Start(cfg.width, cfg.height)
	defer canvas.End()

	for cy := 0; cy < cfg.rows; cy++ {
		for cx := 0; cx < cfg.cols; cx++ {
			c := cfg.colors.at(cx, cy)
			fill := canvas.RGBA(int(c.R), int(c.G), int(c.B), float64(c.A)/255)

			x, y, w, h := cfg.blockRect(cx, cy)
			canvas.Rect(x, y, w, h, fill)
		}
	}

	return nil
}
