package services

import (
	"bytes"
	"encoding/base64"

	"github.com/jung-kurt/gofpdf"

	"github.com/qrapp/backend/internal/models"
)

// PDFExportOptions configures PDF export
type PDFExportOptions struct {
	Title       string
	Description string
	QRSize      float64 // mm
	PageSize    string  // A4, Letter, etc.
	IncludeURL  bool
}

// GenerateQRPDF creates a PDF containing the QR code
func (s *QRService) GenerateQRPDF(content string, size int, custom *models.QRCustomization, options *PDFExportOptions) (string, error) {
	// Set defaults
	if options == nil {
		options = &PDFExportOptions{
			QRSize:   50,
			PageSize: "A4",
		}
	}
	if options.QRSize == 0 {
		options.QRSize = 50
	}
	if options.PageSize == "" {
		options.PageSize = "A4"
	}

	// Generate QR code as PNG first
	qrBase64, err := s.generateQRImage(content, size, custom)
	if err != nil {
		return "", err
	}

	// Decode base64 PNG
	pngData := qrBase64
	if len(pngData) > 22 && pngData[:22] == "data:image/png;base64," {
		pngData = pngData[22:]
	}
	imgBytes, err := base64.StdEncoding.DecodeString(pngData)
	if err != nil {
		return "", err
	}

	// Create PDF
	pdf := gofpdf.New("P", "mm", options.PageSize, "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)

	// Add title if provided
	if options.Title != "" {
		pdf.Cell(0, 10, options.Title)
		pdf.Ln(15)
	}

	// Add description if provided
	if options.Description != "" {
		pdf.SetFont("Arial", "", 12)
		pdf.MultiCell(0, 6, options.Description, "", "", false)
		pdf.Ln(10)
	}

	// Register and add QR image
	imgReader := bytes.NewReader(imgBytes)
	pdf.RegisterImageOptionsReader("qrcode", gofpdf.ImageOptions{ImageType: "PNG"}, imgReader)

	// Center the QR code
	pageW, _ := pdf.GetPageSize()
	x := (pageW - options.QRSize) / 2
	pdf.ImageOptions("qrcode", x, pdf.GetY(), options.QRSize, options.QRSize, false, gofpdf.ImageOptions{}, 0, "")
	pdf.Ln(options.QRSize + 10)

	// Add URL below QR if requested
	if options.IncludeURL {
		pdf.SetFont("Arial", "", 10)
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(0, 6, content, "", 1, "C", false, 0, "")
	}

	// Generate PDF bytes
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return "", err
	}

	// Return as base64
	pdfBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return "data:application/pdf;base64," + pdfBase64, nil
}
