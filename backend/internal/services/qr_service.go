package services

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	xdraw "golang.org/x/image/draw"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
	"gorm.io/datatypes"

	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/services/generators"
)

// QR Service errors
var (
	ErrInvalidContent = errors.New("content cannot be empty")
	ErrInvalidSize    = errors.New("size must be between 64 and 2048")
	ErrInvalidQRType  = errors.New("invalid QR type")
	ErrQRNotFound     = errors.New("QR code not found")
	ErrQRExpired      = errors.New("QR code has expired")
	ErrQRInactive     = errors.New("QR code is inactive")
	ErrDynamicQRLimit = errors.New("dynamic QR limit reached")
)

// QRRepository interface
type QRRepository interface {
	Create(record *models.QRRecord) error
	FindByUserID(userID uuid.UUID, limit, offset int) ([]models.QRRecord, error)
	FindByID(id uuid.UUID) (*models.QRRecord, error)
	FindByShortCode(shortCode string) (*models.QRRecord, error)
	CountByUserID(userID uuid.UUID) (int64, error)
	CountDynamicByUserID(userID uuid.UUID) (int64, error)
	Update(record *models.QRRecord) error
	IncrementScanCount(id uuid.UUID) error
	Delete(id uuid.UUID, userID uuid.UUID) error
}

// QRService handles QR code generation and management
type QRService struct {
	repo QRRepository
}

// NewQRService creates a new QR service
func NewQRService(repo QRRepository) *QRService {
	return &QRService{repo: repo}
}

// UpdateQRRequest contains fields for updating a QR code
type UpdateQRRequest struct {
	Title         *string
	RedirectURL   *string
	IsActive      *bool
	Customization *datatypes.JSON
}

// GenerateRequest contains all parameters for QR generation
type GenerateRequest struct {
	UserID          uuid.UUID               `json:"-"`
	Title           string                  `json:"title,omitempty"`
	Content         string                  `json:"content"`
	QRType          string                  `json:"qr_type"`
	Size            int                     `json:"size"`
	IsDynamic       bool                    `json:"is_dynamic"`
	Metadata        map[string]interface{}  `json:"metadata,omitempty"`
	Customization   *models.QRCustomization `json:"customization,omitempty"`
	ExpiresAt       *time.Time              `json:"expires_at,omitempty"`
	WorkspaceID     *uuid.UUID              `json:"workspace_id,omitempty"`
	FolderID        *uuid.UUID              `json:"folder_id,omitempty"`
	Tags            string                  `json:"tags,omitempty"`
	Password        string                  `json:"password,omitempty"`
	MaxScans        *int                    `json:"max_scans,omitempty"`
	GeoRestrictions string                  `json:"geo_restrictions,omitempty"`
	ScheduledAt     *time.Time              `json:"scheduled_at,omitempty"`
}

// GenerateResult contains the generated QR code
type GenerateResult struct {
	Record   *models.QRRecord `json:"record"`
	QRBase64 string           `json:"qr_base64"`
}

// FrameConfig defines the layout of an image-based frame
type FrameConfig struct {
	ID          string `json:"id"`
	File        string `json:"file"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	QRX         int    `json:"qr_x"`
	QRY         int    `json:"qr_y"`
	QRSize      int    `json:"qr_size"`
	RenderOrder string `json:"render_order,omitempty"` // "over" or "under" (default)
}

func (s *QRService) getFrameConfig(style string) (*FrameConfig, error) {
	// Path to frames.json
	// Assuming running from backend root
	path := filepath.Join("assets", "frames", "frames.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var configs []FrameConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, err
	}

	for _, cfg := range configs {
		if cfg.ID == style {
			return &cfg, nil
		}
	}
	return nil, nil // Not found
}

func (s *QRService) applyImageFrame(qrImg image.Image, config *FrameConfig) (*image.RGBA, error) {
	// Load frame image
	framePath := filepath.Join("assets", "frames", config.File)
	f, err := os.Open(framePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	frameImg, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}

	// Create output image (Clone frame)
	out := image.NewRGBA(frameImg.Bounds())
	draw.Draw(out, out.Bounds(), frameImg, image.Point{}, draw.Src)

	// Resize QR to fit config.QRSize
	// We use x/image/draw for high quality resizing
	qrResized := image.NewRGBA(image.Rect(0, 0, config.QRSize, config.QRSize))
	xdraw.CatmullRom.Scale(qrResized, qrResized.Bounds(), qrImg, qrImg.Bounds(), draw.Over, nil)

	// Place QR under the frame?
	// As discussed, if frame has holes, we should effectively "underlay" it.
	// But `out` already has frame.
	// So we should compose: New Image -> Draw QR -> Draw Frame.

	final := image.NewRGBA(frameImg.Bounds())

	// Draw Sequence
	qrRect := image.Rect(config.QRX, config.QRY, config.QRX+config.QRSize, config.QRY+config.QRSize)
	if config.RenderOrder == "over" {
		// Draw Frame first (Background)
		draw.Draw(final, final.Bounds(), frameImg, image.Point{}, draw.Over)
		// Draw QR second (Foreground)
		draw.Draw(final, qrRect, qrResized, image.Point{}, draw.Over)
	} else {
		// Draw QR first
		draw.Draw(final, qrRect, qrResized, image.Point{}, draw.Over)
		// Draw Frame second (Overlay)
		draw.Draw(final, final.Bounds(), frameImg, image.Point{}, draw.Over)
	}

	return final, nil
}

// Generate creates a new QR code with optional customization
func (s *QRService) Generate(req GenerateRequest) (*GenerateResult, error) {
	log.Printf("[QRService] Starting Generate. Type: %s, Dynamic: %v", req.QRType, req.IsDynamic)
	// Validate size
	if req.Size == 0 {
		req.Size = 256
	}
	if req.Size < 64 || req.Size > 2048 {
		return nil, ErrInvalidSize
	}

	// Default type
	if req.QRType == "" {
		req.QRType = models.TypeURL
	}

	// Generate content from type-specific metadata if provided
	var content string
	if req.Metadata != nil && len(req.Metadata) > 0 {
		metadataJSON, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, err
		}
		content, err = generators.GenerateQRContent(req.QRType, metadataJSON)
		if err != nil {
			return nil, err
		}
	} else if req.Content != "" {
		content = req.Content
	} else {
		return nil, ErrInvalidContent
	}

	// Generate short code (always required for unique index)
	shortCode := generateShortCode()
	var redirectURL string
	if req.IsDynamic {
		redirectURL = content
	}

	// Convert metadata to JSONB
	var metadataJSON datatypes.JSON
	if req.Metadata != nil {
		data, _ := json.Marshal(req.Metadata)
		metadataJSON = datatypes.JSON(data)
	}

	// Convert customization to JSONB
	var customJSON datatypes.JSON
	if req.Customization != nil {
		data, _ := json.Marshal(req.Customization)
		customJSON = datatypes.JSON(data)
	}

	// Hash password if provided
	var passwordHash string
	if req.Password != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		passwordHash = string(h)
	}

	// Create record
	record := &models.QRRecord{
		ID:              uuid.New(),
		UserID:          req.UserID,
		Title:           req.Title,
		Content:         content,
		QRType:          req.QRType,
		QRTypeID:        req.QRType,
		Size:            req.Size,
		ShortCode:       shortCode,
		IsDynamic:       req.IsDynamic,
		RedirectURL:     redirectURL,
		Metadata:        metadataJSON,
		Customization:   customJSON,
		IsActive:        true,
		ExpiresAt:       req.ExpiresAt,
		WorkspaceID:     req.WorkspaceID,
		FolderID:        req.FolderID,
		Tags:            req.Tags,
		Password:        passwordHash,
		MaxScans:        req.MaxScans,
		GeoRestrictions: req.GeoRestrictions,
	}

	// For dynamic QR, the QR content is the redirect URL
	qrContent := content
	if req.IsDynamic && shortCode != "" {
		// The actual QR will point to our redirect endpoint with full URL
		baseURL := os.Getenv("APP_BASE_URL")
		if baseURL == "" {
			port := os.Getenv("PORT")
			if port == "" {
				port = "8080"
			}
			baseURL = "http://localhost:" + port
		}
		qrContent = baseURL + "/r/" + shortCode
	}

	// Generate QR image
	log.Println("[QRService] Generating QR Image...")
	qrBase64, err := s.generateQRImage(qrContent, req.Size, req.Customization)
	if err != nil {
		log.Printf("[QRService] Image Generation Failed: %v", err)
		return nil, err
	}
	log.Println("[QRService] Image Generated.")

	// Save to database
	log.Printf("[QRService] Saving to DB... UserID: %v, Dynamic: %v", req.UserID, req.IsDynamic)
	if s.repo != nil && (req.UserID != uuid.Nil || req.IsDynamic) {
		if err := s.repo.Create(record); err != nil {
			log.Printf("[QRService] DB Save Failed: %v", err)
			return nil, err
		}
	}
	log.Println("[QRService] Save Complete.")

	return &GenerateResult{
		Record:   record,
		QRBase64: qrBase64,
	}, nil
}

// DeleteQR deletes a QR code
func (s *QRService) DeleteQR(id uuid.UUID, userID uuid.UUID) error {
	// Check if exists and belongs to user
	_, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	return s.repo.Delete(id, userID)
}

// generateQRImage creates the QR code image with customization
func (s *QRService) generateQRImage(content string, size int, custom *models.QRCustomization) (string, error) {
	img, err := s.createQRRGBA(content, size, custom)
	if err != nil {
		return "", err
	}

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}

	// Convert to base64
	base64Str := base64.StdEncoding.EncodeToString(buf.Bytes())
	return "data:image/png;base64," + base64Str, nil
}

// createQRRGBA creates the raw RGBA image for the QR code
func (s *QRService) createQRRGBA(content string, size int, custom *models.QRCustomization) (*image.RGBA, error) {
	// Determine error correction level
	recoveryLevel := qrcode.Medium

	// Create QR code
	qr, err := qrcode.New(content, recoveryLevel)
	if err != nil {
		return nil, err
	}

	// Default colors
	var fgColor color.Color = color.Black
	var bgColor color.Color = color.White

	// Apply custom colors if provided
	if custom != nil {
		if custom.ForegroundColor != "" {
			fgColor = parseHexColor(custom.ForegroundColor)
		}
		if custom.BackgroundColor != "" {
			bgColor = parseHexColor(custom.BackgroundColor)
		}
	}

	qr.ForegroundColor = fgColor
	qr.BackgroundColor = bgColor

	// Get the bitmap representation
	bitmap := qr.Bitmap()
	modules := len(bitmap)
	if modules == 0 {
		return nil, errors.New("failed to generate QR bitmap")
	}

	// Check for Image-Based Frame FIRST
	var frameConfig *FrameConfig
	if custom != nil && custom.Frame != nil {
		frameConfig, _ = s.getFrameConfig(custom.Frame.Style)
	}

	// Calculate dimensions with frame space
	frameHeight := 0
	// Only add procedural frame height if NOT using image frame
	if frameConfig == nil && custom != nil && custom.Frame != nil && custom.Frame.Style != "" && custom.Frame.Style != "none" {
		frameHeight = 40 // Space for frame text
	}

	totalHeight := size + frameHeight
	moduleSize := float64(size) / float64(modules)

	// Create image
	img := image.NewRGBA(image.Rect(0, 0, size, totalHeight))

	// Fill background
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// Get dot style
	dotStyle := "square"
	if custom != nil && custom.BodyStyle != "" {
		dotStyle = custom.BodyStyle
	}

	// Get corner style
	cornerStyle := "square"
	if custom != nil && custom.CornerStyle != "" {
		cornerStyle = custom.CornerStyle
	}

	// Draw QR modules
	// If gradient is specified, we need to use masking
	if custom != nil && custom.Gradient != nil {
		// Create a separate image for the QR shapes (mask)
		// We use White on Transparent for the mask?
		// draw.DrawMask uses Alpha channel of mask if it's an Alpha image, or converts.
		// Let's use RGBA where we draw the shapes in any opaque color (e.g. Black)
		// and leave the rest transparent.
		shapeImg := image.NewRGBA(image.Rect(0, 0, size, totalHeight))

		// Draw modules onto shapeImg
		for y, row := range bitmap {
			for x, module := range row {
				if module {
					xPos := int(float64(x) * moduleSize)
					yPos := int(float64(y) * moduleSize)
					modSize := int(moduleSize + 0.5)

					isFinderPattern := isInFinderPattern(x, y, modules)
					if isFinderPattern {
						drawModule(shapeImg, xPos, yPos, modSize, color.Black, cornerStyle)
					} else {
						drawModule(shapeImg, xPos, yPos, modSize, color.Black, dotStyle)
					}
				}
			}
		}

		// Create Gradient Image
		gradientImg := image.NewRGBA(image.Rect(0, 0, size, totalHeight))
		startColor := parseHexColor(custom.Gradient.StartColor)
		endColor := parseHexColor(custom.Gradient.EndColor)

		if custom.Gradient.Type == "radial" {
			drawRadialGradient(gradientImg, startColor, endColor)
		} else {
			// Linear default
			drawLinearGradient(gradientImg, startColor, endColor, custom.Gradient.Rotation)
		}

		// Composite Gradient onto Main Img using Shape Mask
		// draw.DrawMask(dst, r, src, sp, mask, mp, op)
		draw.DrawMask(img, img.Bounds(), gradientImg, image.Point{}, shapeImg, image.Point{}, draw.Over)

	} else {
		// Standard solid color drawing
		for y, row := range bitmap {
			for x, module := range row {
				if module {
					xPos := int(float64(x) * moduleSize)
					yPos := int(float64(y) * moduleSize)
					modSize := int(moduleSize + 0.5)

					isFinderPattern := isInFinderPattern(x, y, modules)

					if isFinderPattern {
						drawModule(img, xPos, yPos, modSize, fgColor, cornerStyle)
					} else {
						drawModule(img, xPos, yPos, modSize, fgColor, dotStyle)
					}
				}
			}
		}
	}

	// Apply Image Frame if Configured
	if frameConfig != nil {
		result, err := s.applyImageFrame(img, frameConfig)
		if err == nil {
			return result, nil
		}
	}

	// Draw frame if specified AND NOT image frame
	if frameConfig == nil && custom != nil && custom.Frame != nil && custom.Frame.Style != "" && custom.Frame.Style != "none" {
		// Pass gradient colors to frame? Or just fgColor?
		// Usually frame matches QR color.
		// If gradient, we might want frame to match StartColor? or mixed?
		// For simplicity, use fgColor (which might be Black).
		// Ideally, we should support Frame Color in Frame struct.
		// Frame struct in models/qr_record.go HAS "Color" field.
		// Let's use it if present.

		frameColor := fgColor
		if custom.Frame.Color != "" {
			frameColor = parseHexColor(custom.Frame.Color)
		} else if custom.Gradient != nil {
			// If gradient, use StartColor as fallback for frame
			frameColor = parseHexColor(custom.Gradient.StartColor)
		}

		drawFrame(img, size, totalHeight, frameHeight, custom.Frame, frameColor, bgColor)
	}

	// Add logo overlay if provided
	if custom != nil && custom.Logo != nil && custom.Logo.URL != "" {
		img = s.addLogoOverlay(img, custom.Logo, size)
	}

	return img, nil
}

// isInFinderPattern checks if coordinates are in QR finder patterns (corners)
func isInFinderPattern(x, y, modules int) bool {
	// Top-left finder (0-6, 0-6)
	if x <= 7 && y <= 7 {
		return true
	}
	// Top-right finder
	if x >= modules-8 && y <= 7 {
		return true
	}
	// Bottom-left finder
	if x <= 7 && y >= modules-8 {
		return true
	}
	return false
}

// drawModule draws a single QR module with the specified style
func drawModule(img *image.RGBA, x, y, size int, c color.Color, style string) {
	switch style {
	case "dots", "circle":
		drawCircle(img, x+size/2, y+size/2, size/2, c)
	case "rounded":
		drawRoundedRect(img, x, y, size, size, size/4, c)
	case "extra-rounded":
		drawRoundedRect(img, x, y, size, size, size/2, c)
	case "classy", "diamond":
		drawDiamond(img, x, y, size, c)
	case "leaf":
		drawLeaf(img, x, y, size, c)
	default: // square
		for dy := 0; dy < size; dy++ {
			for dx := 0; dx < size; dx++ {
				img.Set(x+dx, y+dy, c)
			}
		}
	}
}

// drawFrame draws a frame around the QR code with optional text
func drawFrame(img *image.RGBA, width, height, frameHeight int, frame *models.Frame, fgColor, bgColor color.Color) {
	if frame == nil || frame.Style == "" || frame.Style == "none" {
		return
	}

	// Frame rendering logic
	qrSize := width // Assuming width is QR size
	// Note: totalHeight passed in is size + frameHeight

	switch frame.Style {
	case "simple":
		drawSimpleFrame(img, qrSize, 0, fgColor)

	case "rounded":
		drawRoundedBorder(img, 0, 0, width, width, 20, 10, fgColor)

	case "scan-me", "message":
		drawBubbleFrame(img, frame.Text, qrSize, frameHeight, fgColor, bgColor)

	case "arrow":
		drawArrowFrame(img, frame.Text, qrSize, frameHeight, fgColor, bgColor)

	case "badge":
		drawBadgeFrame(img, frame.Text, qrSize, frameHeight, fgColor, bgColor)

	case "phone":
		drawPhoneFrame(img, frame.Text, qrSize, frameHeight, fgColor, bgColor)

	case "polaroid":
		drawPolaroidFrame(img, frame.Text, qrSize, frameHeight, fgColor, bgColor)

	case "ribbon":
		drawRibbonFrame(img, frame.Text, qrSize, frameHeight, fgColor, bgColor)

	case "sticker":
		drawStickerFrame(img, qrSize, frameHeight, fgColor)

	case "neon":
		drawNeonFrame(img, qrSize, frameHeight, fgColor)

	default:
		drawSimpleFrame(img, qrSize, 0, fgColor)
	}
}

// addLogoOverlay adds a logo to the center of the QR code
func (s *QRService) addLogoOverlay(qrImg *image.RGBA, logo *models.Logo, qrSize int) *image.RGBA {
	if logo.URL == "" {
		return qrImg
	}

	// Parse base64 data URL
	// data:image/png;base64,....
	parts := strings.Split(logo.URL, ",")
	if len(parts) != 2 {
		return qrImg
	}

	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return qrImg
	}

	// Decode image
	logoImg, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return qrImg
	}

	// Calculate logo size (default 20% if not set)
	logoSizeRatio := logo.Size
	if logoSizeRatio <= 0 {
		logoSizeRatio = 0.2
	}
	if logoSizeRatio > 0.4 {
		logoSizeRatio = 0.4 // Max 40% to avoid covering too much
	}

	targetSize := int(float64(qrSize) * logoSizeRatio)

	// Resize logo to target size
	dst := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), logoImg, logoImg.Bounds(), xdraw.Over, nil)

	// Calculate center position
	offset := (qrSize - targetSize) / 2

	// Create a new image to hold the composite
	// We need to draw the QR code, then the logo on top
	// Note: qrImg might have a frame, so its bounds might be taller than qrSize
	// But the QR code itself is at (0,0) to (qrSize, qrSize)

	// Draw logo at offset
	rect := image.Rect(offset, offset, offset+targetSize, offset+targetSize)
	draw.Draw(qrImg, rect, dst, image.Point{}, draw.Over)

	return qrImg
}

// GenerateQRSVG creates QR code as SVG string with customization
func (s *QRService) GenerateQRSVG(content string, size int, custom *models.QRCustomization) (string, error) {
	// Create QR code
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", err
	}

	// Get foreground/background colors
	fgColor := "#000000"
	bgColor := "#FFFFFF"
	if custom != nil {
		if custom.ForegroundColor != "" {
			fgColor = custom.ForegroundColor
		}
		if custom.BackgroundColor != "" {
			bgColor = custom.BackgroundColor
		}
	}

	// Get the bitmap representation
	bitmap := qr.Bitmap()
	modules := len(bitmap)
	if modules == 0 {
		return "", errors.New("failed to generate QR bitmap")
	}

	// Calculate module size
	moduleSize := float64(size) / float64(modules)

	// Build SVG using fmt for proper formatting
	var svg strings.Builder
	svg.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	svg.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" version="1.1" `)
	svg.WriteString(`width="` + intToString(size) + `" height="` + intToString(size) + `" `)
	svg.WriteString(`viewBox="0 0 ` + intToString(size) + ` ` + intToString(size) + `">` + "\n")

	// Background
	svg.WriteString(`<rect width="100%" height="100%" fill="` + bgColor + `"/>` + "\n")

	// Draw QR modules
	for y, row := range bitmap {
		for x, module := range row {
			if module {
				xPos := int(float64(x) * moduleSize)
				yPos := int(float64(y) * moduleSize)
				w := int(moduleSize + 1)
				svg.WriteString(`<rect x="` + intToString(xPos) + `" y="` + intToString(yPos) +
					`" width="` + intToString(w) + `" height="` + intToString(w) +
					`" fill="` + fgColor + `"/>` + "\n")
			}
		}
	}

	svg.WriteString("</svg>")

	return svg.String(), nil
}

// intToString converts int to string without importing strconv
func intToString(n int) string {
	if n == 0 {
		return "0"
	}

	neg := n < 0
	if neg {
		n = -n
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if neg {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}

// parseHexColor converts hex string to color.Color
func parseHexColor(hex string) color.Color {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return color.Black
	}

	var r, g, b uint8
	_, err := parseHexComponent(hex[0:2], &r)
	if err != nil {
		return color.Black
	}
	_, err = parseHexComponent(hex[2:4], &g)
	if err != nil {
		return color.Black
	}
	_, err = parseHexComponent(hex[4:6], &b)
	if err != nil {
		return color.Black
	}

	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func parseHexComponent(s string, n *uint8) (int, error) {
	var val uint64
	for _, c := range s {
		val *= 16
		switch {
		case c >= '0' && c <= '9':
			val += uint64(c - '0')
		case c >= 'a' && c <= 'f':
			val += uint64(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			val += uint64(c - 'A' + 10)
		default:
			return 0, errors.New("invalid hex")
		}
	}
	*n = uint8(val)
	return 2, nil
}

// generateShortCode creates a unique short code for dynamic QR
func generateShortCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	// Use base64 URL-safe encoding and trim padding
	code := base64.URLEncoding.EncodeToString(b)
	code = strings.TrimRight(code, "=")
	return code[:8]
}

// GetByShortCode retrieves a QR record by its short code (for redirect)
func (s *QRService) GetByShortCode(shortCode string) (*models.QRRecord, error) {
	record, err := s.repo.FindByShortCode(shortCode)
	if err != nil {
		return nil, ErrQRNotFound
	}

	// Check if active
	if !record.IsActive {
		return nil, ErrQRInactive
	}

	// Check if expired
	if record.ExpiresAt != nil && record.ExpiresAt.Before(time.Now()) {
		return nil, ErrQRExpired
	}

	return record, nil
}

// UpdateQR updates a QR record
func (s *QRService) UpdateQR(id uuid.UUID, userID uuid.UUID, req UpdateQRRequest) error {
	record, err := s.repo.FindByID(id)
	if err != nil {
		return ErrQRNotFound
	}

	// Verify ownership
	if record.UserID != userID {
		return ErrQRNotFound
	}

	// Update Title
	if req.Title != nil {
		record.Title = *req.Title
	}

	// Update RedirectURL (Dynamic only)
	if req.RedirectURL != nil {
		if !record.IsDynamic {
			return errors.New("cannot update redirect URL of static QR code")
		}
		record.RedirectURL = *req.RedirectURL
		record.Content = *req.RedirectURL // For dynamic QR, content might be same or different depending on implementation.
		// Wait, Generate says: if dynamic, qrContent = baseURL + /r/shortCode.
		// But record.Content stored in DB?
		// In models: Content string `gorm:"type:text;not null" json:"content"`
		// In Generate:
		// record.Content = content (which is destination URL for dynamic).
		// record.RedirectURL = redirectURL (which is destination URL).
		// So yes, updating both is correct for how it's currently stored.
	}

	// Update Status
	if req.IsActive != nil {
		record.IsActive = *req.IsActive
	}

	// Update Customization
	if req.Customization != nil {
		record.Customization = *req.Customization
	}

	return s.repo.Update(record)
}

// GetHistory retrieves paginated QR history for a user
func (s *QRService) GetHistory(userID uuid.UUID, page, pageSize int) (*HistoryResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	records, err := s.repo.FindByUserID(userID, pageSize, offset)
	if err != nil {
		return nil, err
	}

	total, err := s.repo.CountByUserID(userID)
	if err != nil {
		return nil, err
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &HistoryResult{
		Records:    records,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetByID retrieves a QR record by ID (with ownership check)
func (s *QRService) GetByID(id uuid.UUID, userID uuid.UUID) (*models.QRRecord, error) {
	record, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrQRNotFound
	}

	// Verify ownership if userID provided
	if userID != uuid.Nil && record.UserID != userID {
		return nil, ErrQRNotFound
	}

	return record, nil
}

// HistoryResult contains paginated QR history
type HistoryResult struct {
	Records    []models.QRRecord `json:"records"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// Compile-time interface check
var _ image.Image = (*image.RGBA)(nil)
var _ draw.Image = (*image.RGBA)(nil)
