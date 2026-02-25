package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"path/filepath"

	"github.com/fogleman/gg"
	"golang.org/x/image/font/basicfont"
)

func main() {
	outDir := filepath.Join("backend", "assets", "frames")
	// Ensure dir exists
	// os.MkdirAll handled by user manually or already done

	generatePhoneFrame(filepath.Join(outDir, "phone.png"))
	generatePolaroidFrame(filepath.Join(outDir, "polaroid.png"))
	generateRibbonFrame(filepath.Join(outDir, "ribbon.png"))
	generateNeonFrame(filepath.Join(outDir, "neon.png"))
	generateScanMeFrame(filepath.Join(outDir, "scan_me.png"))
	generateRoundFrame(filepath.Join(outDir, "round.png"))
	generateBadgeFrame(filepath.Join(outDir, "badge.png"))
	generateArrowFrame(filepath.Join(outDir, "arrow.png"))
	generateDotsFrame(filepath.Join(outDir, "dots.png"))
}

func generatePhoneFrame(path string) {
	w, h := 500, 800
	dc := gg.NewContext(w, h)

	// Draw Phone Body (Rounded Rect)
	dc.DrawRoundedRectangle(0, 0, float64(w), float64(h), 40)
	dc.SetColor(color.Black)
	dc.Fill()

	// Screen (Transparent)
	// Clear center
	// Can't "clear" easily with GG if context is opaque?
	// GG starts transparent.
	// But we filled black.
	// We can set CompositeMode to Clear? Or just draw screen area with Clear.
	// Actually, easier to draw border instead of fill?
	// No, phone body has thickness.
	// Let's create new context, fill black, then Clear rect.

	dc.Clear() // Clear everything
	dc.DrawRoundedRectangle(0, 0, float64(w), float64(h), 40)
	dc.SetColor(color.Black)
	dc.Fill()

	// Cut out screen
	// Use draw.Draw on underlying image for transparency
	if rgba, ok := dc.Image().(*image.RGBA); ok {
		// Screen Rect: 30, 80, 470, 680
		// image.Rect(x0, y0, x1, y1) -> 30, 80, 30+440=470, 80+600=680
		rect := image.Rect(30, 80, 470, 680)
		draw.Draw(rgba, rect, &image.Uniform{color.Transparent}, image.Point{}, draw.Src)
	}

	dc.SavePNG(path)
}

func generatePolaroidFrame(path string) {
	w, h := 600, 750
	dc := gg.NewContext(w, h)

	// White Card
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	dc.SetColor(color.White)
	dc.Fill()

	// Cut Photo Area
	// Use draw.Draw on underlying image for transparency
	if rgba, ok := dc.Image().(*image.RGBA); ok {
		// Rect 40,40 to 560,560
		rect := image.Rect(40, 40, 560, 560)
		draw.Draw(rgba, rect, &image.Uniform{color.Transparent}, image.Point{}, draw.Src)
	}

	// Add Text?
	// dc.SetFontFace(basicfont.Face7x13)
	// dc.SetColor(color.Black)
	// dc.DrawStringAnchored("Memories", float64(w)/2, 650, 0.5, 0.5)

	dc.SavePNG(path)
}

func generateRibbonFrame(path string) {
	w, h := 600, 700
	dc := gg.NewContext(w, h) // Transparent

	// Draw Ribbon Shape at bottom
	// Curve
	dc.MoveTo(50, 600)
	dc.LineTo(550, 600)
	dc.LineTo(550, 680)
	dc.LineTo(300, 660) // Notch?
	dc.LineTo(50, 680)
	dc.ClosePath()

	dc.SetColor(color.RGBA{220, 20, 60, 255}) // Crimson
	dc.Fill()

	dc.SavePNG(path)
}

func generateNeonFrame(path string) {
	w, h := 600, 600
	dc := gg.NewContext(w, h)

	// Glow effect = multiple strokes with decreasing opacity?
	// Simple neon: Thick stroke with bright color
	c := color.RGBA{255, 20, 147, 255} // Pink

	dc.DrawRoundedRectangle(20, 20, 560, 560, 30)
	dc.SetColor(c)
	dc.SetLineWidth(15)
	dc.Stroke()

	// Inner white line
	dc.DrawRoundedRectangle(20, 20, 560, 560, 30)
	dc.SetColor(color.White)
	dc.SetLineWidth(3)
	dc.Stroke()

	dc.SavePNG(path)
}

func generateScanMeFrame(path string) {
	w, h := 600, 700
	dc := gg.NewContext(w, h)

	// Bubble
	dc.DrawRoundedRectangle(150, 20, 300, 60, 20)
	dc.SetColor(color.Black)
	dc.Fill()

	// Text
	dc.SetFontFace(basicfont.Face7x13)
	dc.SetColor(color.White)
	// Scale text?
	// No scale available easily on basic font face?
	// basicfont is fixed.
	// We can manually scale context?
	dc.Push()
	dc.Scale(2, 2) // Make text bigger
	// Position: Center is 300, 50. Scaled: 150, 25.
	dc.DrawStringAnchored("SCAN ME", 150, 25, 0.5, 0.4)
	dc.Pop()

	// Border
	dc.DrawRoundedRectangle(50, 100, 500, 500, 40)
	dc.SetColor(color.Black)
	dc.SetLineWidth(12)
	dc.Stroke()

	dc.SavePNG(path)
}

func generateRoundFrame(path string) {
	w, h := 600, 600
	dc := gg.NewContext(w, h)

	cx, cy := 300.0, 300.0
	r := 280.0

	// Draw Circle Border
	dc.DrawCircle(cx, cy, r)
	dc.SetColor(color.Black)
	dc.SetLineWidth(20)
	dc.Stroke()

	dc.SavePNG(path)
}

func generateBadgeFrame(path string) {
	w, h := 600, 600
	dc := gg.NewContext(w, h)

	cx, cy := 300.0, 300.0

	// Star/Badge shape
	points := 16
	outerR := 290.0
	innerR := 260.0

	dc.MoveTo(cx+outerR, cy)
	for i := 1; i <= points*2; i++ {
		angle := float64(i) * math.Pi / float64(points)
		r := innerR
		if i%2 == 0 {
			r = outerR
		}
		dc.LineTo(cx+math.Cos(angle)*r, cy+math.Sin(angle)*r)
	}
	dc.ClosePath()

	// Fill Gold
	dc.SetColor(color.RGBA{255, 215, 0, 255})
	dc.FillPreserve()
	// Stroke Blue
	dc.SetColor(color.RGBA{0, 0, 139, 255})
	dc.SetLineWidth(10)
	dc.Stroke()

	// Inner transparent hole for QR?
	// Actually typical badge frames are solid with QR ON TOP.
	// My `qr_service` draws QR *under* frame if transparent, or overlays if frame is opaque?
	// Wait. My `qr_service.go` logic: `draw.Draw(final, qrRect, qrResized, Over); draw.Draw(final, frameImg, Over);`
	// Frame is drawn OVER QR.
	// So frame MUST have transparent center.

	// Cut hole
	if rgba, ok := dc.Image().(*image.RGBA); ok {
		// DrawCircle uses vector path. To clear a CIRCLE using draw.Draw is hard (it supports rects).
		// We can use a MASK.
		// Or we can manually iterate pixels?
		// Or since we are in GG, we can just set color transparent and fill?
		// GG doesn't support "Draw with Transparent Color replaces pixel" unless we set Composite Mode.
		// BUT standard draw.Src mode DOES replace.
		// GG doesn't expose mode easily.
		// We can Iterate pixels.
		// Center 300,300. R=220.
		cx, cy, r := 300.0, 300.0, 220.0
		bounds := rgba.Bounds()
		for y := 0; y < bounds.Max.Y; y++ {
			for x := 0; x < bounds.Max.X; x++ {
				dx := float64(x) - cx
				dy := float64(y) - cy
				if dx*dx+dy*dy <= r*r {
					rgba.Set(x, y, color.Transparent)
				}
			}
		}
	}

	dc.SavePNG(path)
}

func generateArrowFrame(path string) {
	w, h := 700, 600
	dc := gg.NewContext(w, h)

	// QR Frame
	dc.DrawRoundedRectangle(150, 50, 500, 500, 20)
	dc.SetColor(color.Black)
	dc.SetLineWidth(10)
	dc.Stroke()

	// Arrow on Left
	// Shaft
	dc.DrawRectangle(20, 290, 130, 20)
	dc.SetColor(color.RGBA{220, 20, 60, 255}) // Crimson
	dc.Fill()

	// Head
	dc.MoveTo(150, 300) // Tip
	dc.LineTo(120, 270)
	dc.LineTo(120, 330)
	dc.ClosePath()
	dc.Fill()

	dc.SavePNG(path)
}

func generateDotsFrame(path string) {
	w, h := 600, 600
	dc := gg.NewContext(w, h)

	// Draw dots along border
	// Rect 20,20 to 580,580
	dc.SetColor(color.Black)

	step := 30.0
	r := 6.0

	// Top & Bottom
	for x := 20.0; x <= 580.0; x += step {
		dc.DrawCircle(x, 20, r)
		dc.DrawCircle(x, 580, r)
	}
	// Left & Right
	for y := 20.0; y <= 580.0; y += step {
		dc.DrawCircle(20, y, r)
		dc.DrawCircle(580, y, r)
	}
	dc.Fill()

	dc.SavePNG(path)
}
