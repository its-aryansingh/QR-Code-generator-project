package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
)

func main() {
	dir := filepath.Join("backend", "assets", "frames")
	files, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".png" {
			continue
		}

		path := filepath.Join(dir, f.Name())
		file, err := os.Open(path)
		if err != nil {
			fmt.Printf("Error opening %s: %v\n", f.Name(), err)
			continue
		}

		img, _, err := image.Decode(file)
		file.Close()
		if err != nil {
			fmt.Printf("Error decoding %s: %v\n", f.Name(), err)
			continue
		}

		bounds := img.Bounds()
		w, h := bounds.Dx(), bounds.Dy()

		// Find transparent region
		// Simple approach: Find min/max X/Y of transparent pixels
		minX, minY := w, h
		maxX, maxY := 0, 0
		found := false

		// Optimization: Check center first? No, verify all.
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				if a < 1000 { // Transparent (Alpha ~0)
					if x < minX {
						minX = x
					}
					if x > maxX {
						maxX = x
					}
					if y < minY {
						minY = y
					}
					if y > maxY {
						maxY = y
					}
					found = true
				}
			}
		}

		// Debug: Print center pixel
		r, g, b, a := img.At(w/2, h/2).RGBA()
		fmt.Printf("File: %s | Center Pixel (%d,%d): R:%d G:%d B:%d A:%d\n", f.Name(), w/2, h/2, r, g, b, a)

		if found {
			// Center of transparent region
			transW := maxX - minX + 1
			transH := maxY - minY + 1

			// We want a square QR code to fit inside this region.
			// Ideally centered.
			size := transW
			if transH < size {
				size = transH
			}

			// If not square, center the square in the rect
			qrX := minX + (transW-size)/2
			qrY := minY + (transH-size)/2

			fmt.Printf("File: %s | Size: %dx%d | Transparent: %d,%d %dx%d | Sug: x=%d y=%d size=%d\n",
				f.Name(), w, h, minX, minY, transW, transH, qrX, qrY, size)
		} else {
			fmt.Printf("File: %s | Size: %dx%d | No transparent region found.\n", f.Name(), w, h)
		}
	}
}
