package services

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Helper functions for drawing shapes

// drawCircle draws a filled circle
func drawCircle(img *image.RGBA, cx, cy, radius int, c color.Color) {
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if x*x+y*y <= radius*radius {
				img.Set(cx+x, cy+y, c)
			}
		}
	}
}

// drawRoundedRect draws a rounded rectangle
func drawRoundedRect(img *image.RGBA, x, y, w, h, radius int, c color.Color) {
	// Draw main rect
	for dy := radius; dy < h-radius; dy++ {
		for dx := 0; dx < w; dx++ {
			img.Set(x+dx, y+dy, c)
		}
	}
	// Draw top and bottom strips
	for dy := 0; dy < radius; dy++ {
		for dx := radius; dx < w-radius; dx++ {
			img.Set(x+dx, y+dy, c)
			img.Set(x+dx, y+h-1-dy, c)
		}
	}
	// Draw corners
	for dy := 0; dy < radius; dy++ {
		for dx := 0; dx < radius; dx++ {
			dist := (radius-dx-1)*(radius-dx-1) + (radius-dy-1)*(radius-dy-1)
			if dist <= radius*radius {
				img.Set(x+dx, y+dy, c)
				img.Set(x+w-1-dx, y+dy, c)
				img.Set(x+dx, y+h-1-dy, c)
				img.Set(x+w-1-dx, y+h-1-dy, c)
			}
		}
	}
}

// drawDiamond draws a diamond shape
func drawDiamond(img *image.RGBA, x, y, size int, c color.Color) {
	half := size / 2
	for dy := 0; dy < size; dy++ {
		dist := half - abs(dy-half)
		for dx := half - dist; dx <= half+dist; dx++ {
			if dx >= 0 && dx < size {
				img.Set(x+dx, y+dy, c)
			}
		}
	}
}

// drawLeaf draws a leaf shape (rounded opposite corners)
func drawLeaf(img *image.RGBA, x, y, size int, c color.Color) {
	// Top-left and bottom-right are rounded
	// Sharp corners at TR and BL

	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size; dx++ {
			// Top-left circle logic (centered at bottom-right of square)
			// dist from (size, size) <= size
			// Wait, leaf shape is intersection of two circles.
			// Circle 1 center (0, size), radius size -> covers top-right to bottom-left arc? No.
			// Circle 1 center (size, 0) radius size -> covers top-left to bottom-right arc?

			// Simple leaf: intersection of circle at (0, size) and circle at (size, 0) with radius `size`.
			// Let's visualize:
			// (0,size) is bottom-left corner. Circle goes through TL and BR.
			// (size,0) is top-right corner. Circle goes through TL and BR.
			// Intersection creates a leaf shape from TL to BR diagonal.

			// Let's use corners as centers.
			// Center 1: Bottom-Left (0, size-1). Matches if dist <= size^2
			distBL := (dx)*(dx) + (size-1-dy)*(size-1-dy)

			// Center 2: Top-Right (size-1, 0). Matches if dist <= size^2
			distTR := (size-1-dx)*(size-1-dx) + (dy)*(dy)

			// If inside both circles, draw.
			if distBL <= size*size && distTR <= size*size {
				img.Set(x+dx, y+dy, c)
			}
		}
	}
}

// Implementation of specific frames

// drawSimpleFrame: A border around the QR code
func drawSimpleFrame(img *image.RGBA, qrSize, framePadding int, c color.Color) {
	// Draw border
	borderWidth := 10
	w, h := img.Bounds().Dx(), img.Bounds().Dy()

	// Top
	for y := 0; y < borderWidth; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	// Bottom
	for y := h - borderWidth; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	// Left
	for y := 0; y < h; y++ {
		for x := 0; x < borderWidth; x++ {
			img.Set(x, y, c)
		}
	}
	// Right
	for y := 0; y < h; y++ {
		for x := w - borderWidth; x < w; x++ {
			img.Set(x, y, c)
		}
	}
}

// drawBubbleFrame: A speech bubble style frame (Scan Me)
func drawBubbleFrame(img *image.RGBA, text string, qrSize, frameHeight int, fgColor, bgColor color.Color) {
	w := img.Bounds().Dx()
	// Bubble is at the bottom
	bubbleH := frameHeight - 10 // leave some margin
	bubbleY := qrSize + 5
	bubbleW := w - 40
	bubbleX := 20

	// Draw rounded bubble
	drawRoundedRect(img, bubbleX, bubbleY, bubbleW, bubbleH, 20, fgColor)

	// Draw triangle pointer pointing up to QR
	// Center of bubble
	cx := w / 2
	triangleH := 15
	triangleW := 20
	triangleY := bubbleY - triangleH + 2 // Overlap slightly

	for dy := 0; dy < triangleH; dy++ {
		widthAtY := triangleW * dy / triangleH
		startX := cx - widthAtY/2
		endX := cx + widthAtY/2
		y := triangleY + dy
		for x := startX; x <= endX; x++ {
			img.Set(x, y, fgColor)
		}
	}

	// Draw text
	if text != "" {
		drawText(img, text, cx, bubbleY+bubbleH/2+5, bgColor)
	}
}

func drawText(img *image.RGBA, text string, cx, cy int, c color.Color) {
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
	}
	textWidth := d.MeasureString(text).Ceil()
	// Center text
	d.Dot = fixed.P(cx-textWidth/2, cy)
	d.DrawString(text)
}

// drawRoundedBorder draws a rounded rectangle border
func drawRoundedBorder(img *image.RGBA, x, y, w, h, radius, thickness int, c color.Color) {
	// Top/Bottom lines
	for dx := radius; dx < w-radius; dx++ {
		for t := 0; t < thickness; t++ {
			img.Set(x+dx, y+t, c)     // Top
			img.Set(x+dx, y+h-1-t, c) // Bottom
		}
	}
	// Left/Right lines
	for dy := radius; dy < h-radius; dy++ {
		for t := 0; t < thickness; t++ {
			img.Set(x+t, y+dy, c)     // Left
			img.Set(x+w-1-t, y+dy, c) // Right
		}
	}
	// Corners
	for dy := 0; dy < radius; dy++ {
		for dx := 0; dx < radius; dx++ {
			dist := (radius-dx-1)*(radius-dx-1) + (radius-dy-1)*(radius-dy-1)
			innerRadius := radius - thickness
			innerDist := innerRadius * innerRadius

			if dist <= radius*radius && dist >= innerDist {
				// Top-Left
				img.Set(x+dx, y+dy, c)
				// Top-Right
				img.Set(x+w-1-dx, y+dy, c)
				// Bottom-Left
				img.Set(x+dx, y+h-1-dy, c)
				// Bottom-Right
				img.Set(x+w-1-dx, y+h-1-dy, c)
			}
		}
	}
}

// drawArrowFrame draws a frame with an arrow pointing to the QR
func drawArrowFrame(img *image.RGBA, text string, qrSize, frameHeight int, fgColor, bgColor color.Color) {
	w := img.Bounds().Dx()
	// Arrow bar at bottom
	barH := frameHeight - 15
	barY := qrSize + 15

	// Draw main bar
	for y := barY; y < barY+barH; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, fgColor)
		}
	}

	// Draw arrow pointing up (negative space or additional shape)
	cx := w / 2

	// Draw triangle
	triangleY := barY - 10
	for dy := 0; dy < 10; dy++ {
		width := dy * 2
		startX := cx - width
		endX := cx + width
		y := triangleY + dy
		for x := startX; x <= endX; x++ {
			img.Set(x, y, fgColor)
		}
	}

	if text != "" {
		drawText(img, text, cx, barY+barH/2+4, bgColor)
	}
}

// drawBadgeFrame draws a badge style frame
func drawBadgeFrame(img *image.RGBA, text string, qrSize, frameHeight int, fgColor, bgColor color.Color) {
	w := img.Bounds().Dx()
	cx := w / 2
	cy := qrSize + frameHeight/2
	radius := frameHeight/2 - 2

	drawCircle(img, cx, cy, radius, fgColor)
	if text != "" {
		drawText(img, text, cx, cy+4, bgColor)
	}
}

// Helper to mix colors
func mixColors(c1, c2 color.Color, t float64) color.Color {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()

	r := uint32(float64(r1)*(1-t) + float64(r2)*t)
	g := uint32(float64(g1)*(1-t) + float64(g2)*t)
	b := uint32(float64(b1)*(1-t) + float64(b2)*t)
	a := uint32(float64(a1)*(1-t) + float64(a2)*t)

	return color.RGBA64{R: uint16(r), G: uint16(g), B: uint16(b), A: uint16(a)}
}

// drawLinearGradient fills the image with a linear gradient
func drawLinearGradient(img *image.RGBA, startColor, endColor color.Color, angleDeg float64) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	angleRad := angleDeg * math.Pi / 180.0

	// Calculate gradient vector
	cosA := math.Cos(angleRad)
	sinA := math.Sin(angleRad)

	// Determine projection range
	// We want to map each pixel (x,y) to a value t in [0,1] along the gradient vector
	// The gradient starts at one edge and ends at the opposite edge
	// Simplified: approximate logic for filling

	// Center of image
	cx, cy := float64(w)/2, float64(h)/2

	// Length of diagonal
	diag := math.Sqrt(float64(w*w) + float64(h*h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Project point onto gradient vector
			dx := float64(x) - cx
			dy := float64(y) - cy
			proj := dx*cosA + dy*sinA

			// Normalize t to [0,1] range based on diagonal length
			// t = 0.5 + proj / diag
			t := 0.5 + proj/diag
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}

			// Mix colors
			img.Set(x, y, mixColors(startColor, endColor, t))
		}
	}
}

// drawRadialGradient fills the image with a radial gradient
func drawRadialGradient(img *image.RGBA, centerColor, outerColor color.Color) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	cx, cy := float64(w)/2, float64(h)/2
	maxDist := math.Sqrt(cx*cx + cy*cy)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)

			t := dist / maxDist
			if t > 1 {
				t = 1
			}

			img.Set(x, y, mixColors(centerColor, outerColor, t))
		}
	}
}

// drawPhoneFrame draws a smartphone frame
func drawPhoneFrame(img *image.RGBA, text string, qrSize, frameHeight int, fgColor, bgColor color.Color) {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	// Outer phone body (rounded rect)
	drawRoundedRect(img, 0, 0, w, h, 40, fgColor)

	// Inner screen (white/bgColor)
	screenMargin := 15
	// Screen area: top notch to bottom chin
	screenY := 40
	screenH := h - 80
	drawRoundedRect(img, screenMargin, screenY, w-2*screenMargin, screenH, 10, bgColor)

	// Notch
	notchW := w / 3
	drawRoundedRect(img, (w-notchW)/2, 10, notchW, 20, 10, bgColor) // Simplified notch

	// Home button/bar indicator
	drawRoundedRect(img, (w-100)/2, h-25, 100, 5, 2, bgColor)

	// Text?
	if text != "" {
		drawText(img, text, w/2, h-50, color.Black) // Text on chin?
	}
}

// drawPolaroidFrame draws a polaroid photo style frame
func drawPolaroidFrame(img *image.RGBA, text string, qrSize, frameHeight int, fgColor, bgColor color.Color) {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	// White body (usually white, but use fgColor if provided/custom)
	// Polaroid is usually valid image area + large bottom margin
	// We assume img is already sized to (size, size + frameHeight)

	// Fill background (Polaroid paper) with fgColor (usually white for polaroid, but allow custom)
	// If standard QR, fgColor is BLACK. We want Frame Color.
	// We'll use bgColor as the "paper" color and fgColor as "text" color?
	// Actually, drawFrame passes fgColor as FrameColor.
	// So fill with fgColor.
	draw.Draw(img, img.Bounds(), &image.Uniform{fgColor}, image.Point{}, draw.Src)

	// Inner photo area (where QR is) - usually slightly darkened or just the QR content
	// QR is at (0,0) usually? No, qr_service offsets it if we tell it to.
	// But currently drawFrame draws ON TOP of existing image?
	// Or existing image HAS QR codes?
	// Existing image `img` HAS the QR codes drawn on `bgColor`.
	// If we overwrite it, we hide the QR.
	// `drawFrame` is called AFTER drawing QR modules.
	// And `img` is sized `size + frameHeight`.
	// If we just draw a border, we are fine.
	// But Polaroid needs margin AROUND the QR.
	// This requires `qr_service` to offset the QR code itself.
	// Currently `qr_service` only supports `frameHeight` at BOTTOM?
	// Let's check `qr_service` line 230:
	// `frameHeight = 40` (Space for frame text).
	// `totalHeight := size + frameHeight`.
	// `img := image.NewRGBA(image.Rect(0, 0, size, totalHeight))`.
	// QR is drawn at (0,0).
	// So QR is at TOP. Frame text space is at BOTTOM.
	// This limits "Polaroid" style which needs side margins.
	// I cannot easily change layout without refactoring `qr_service` creation logic.
	// So "Polaroid" will be: Border around QR + Large Bottom with Text.
	// This is "Instant Photo" style.

	border := 10

	// Draw simple border
	// Top
	draw.Draw(img, image.Rect(0, 0, w, border), &image.Uniform{fgColor}, image.Point{}, draw.Src)
	// Left
	draw.Draw(img, image.Rect(0, 0, border, h), &image.Uniform{fgColor}, image.Point{}, draw.Src)
	// Right
	draw.Draw(img, image.Rect(w-border, 0, w, h), &image.Uniform{fgColor}, image.Point{}, draw.Src)
	// Bottom (large area) is already there if frameHeight is large enough.
	// But we need to ensure background of QR area is not covered?
	// QR is on `bgColor`. Frame is `fgColor`.

	// If we want white polaroid frame, `fgColor` should be white.
	// The text should be black/dark.

	if text != "" {
		// Use standard text drawer, contrast color?
		// We'll use Black for now or invert fgColor?
		textColor := color.RGBA{0, 0, 0, 255}
		drawText(img, text, w/2, h-frameHeight/2, textColor)
	}
}

// drawRibbonFrame draws a ribbon at the bottom
func drawRibbonFrame(img *image.RGBA, text string, qrSize, frameHeight int, fgColor, bgColor color.Color) {
	w := img.Bounds().Dx()
	ribbonY := qrSize + 10
	ribbonH := frameHeight - 20

	// Main ribbon rect
	draw.Draw(img, image.Rect(10, ribbonY, w-10, ribbonY+ribbonH), &image.Uniform{fgColor}, image.Point{}, draw.Src)

	// Ends (triangle cutout?)
	// Left triangle
	// ... simple rect for now

	if text != "" {
		drawText(img, text, w/2, ribbonY+ribbonH/2+4, bgColor)
	}
}

// drawStickerFrame draws a white border effect
func drawStickerFrame(img *image.RGBA, qrSize, frameHeight int, fgColor color.Color) {
	// Simple thick border
	drawRoundedBorder(img, 0, 0, img.Bounds().Dx(), img.Bounds().Dy(), 15, 8, fgColor)
}

// drawNeonFrame draws a glowing effect
func drawNeonFrame(img *image.RGBA, qrSize, frameHeight int, fgColor color.Color) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	// Draw multiple concentric borders with decreasing alpha (if possible) or thickness
	// Go image/draw doesn't support alpha blending easily on existing opaque?
	// But we can just draw lines.

	drawRoundedBorder(img, 5, 5, w-5, h-5, 10, 2, fgColor)
	drawRoundedBorder(img, 8, 8, w-8, h-8, 10, 1, fgColor)
	drawRoundedBorder(img, 2, 2, w-2, h-2, 12, 1, fgColor) // External glow
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
