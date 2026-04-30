package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qrapp/backend/internal/repository"
	"github.com/qrapp/backend/internal/services"
)

// BulkHandler handles CSV-based bulk QR code generation
type BulkHandler struct {
	qrService *services.QRService
	wsRepo    *repository.WorkspaceRepository
}

// NewBulkHandler creates a new BulkHandler
func NewBulkHandler(qrService *services.QRService, wsRepo *repository.WorkspaceRepository) *BulkHandler {
	return &BulkHandler{qrService: qrService, wsRepo: wsRepo}
}

type csvRow struct {
	title   string
	content string
	qrType  string
}

type bulkResult struct {
	index   int
	title   string
	success bool
	pngData []byte
	errMsg  string
}

// BulkCSVGenerate accepts a CSV file and streams a ZIP of QR PNGs.
// POST /api/v1/workspaces/:id/bulk
//
// CSV format (header row required):
//
//	title,content,type
//	"My QR","https://example.com","url"
//
// type is optional and defaults to "url".
func (h *BulkHandler) BulkCSVGenerate(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	wsID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid workspace ID"})
		return
	}

	// RBAC check — editor+ can generate QR codes
	member, err := h.wsRepo.GetMember(wsID, userID)
	if err != nil || !member.CanManageQR() {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Insufficient permissions"})
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "CSV file required (field: file)"})
		return
	}
	defer file.Close()

	rows, err := parseCSVRows(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "CSV parse error: " + err.Error()})
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "CSV contains no data rows"})
		return
	}

	// Hard cap per request
	const maxRows = 100
	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}

	// Generate concurrently with bounded parallelism
	results := make([]bulkResult, len(rows))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for i, row := range rows {
		wg.Add(1)
		go func(idx int, r csvRow) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, genErr := h.qrService.Generate(services.GenerateRequest{
				UserID:    userID,
				Title:     r.title,
				Content:   r.content,
				QRType:    r.qrType,
				Size:      512,
				IsDynamic: false,
			})

			if genErr != nil {
				results[idx] = bulkResult{index: idx, title: r.title, success: false, errMsg: genErr.Error()}
				return
			}

			pngBytes, decErr := decodeQRBase64(res.QRBase64)
			if decErr != nil {
				results[idx] = bulkResult{index: idx, title: r.title, success: false, errMsg: "base64 decode failed"}
				return
			}

			results[idx] = bulkResult{index: idx, title: r.title, success: true, pngData: pngBytes}
		}(i, row)
	}
	wg.Wait()

	// Build in-memory ZIP archive
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	successCount := 0
	failCount := 0
	usedNames := map[string]int{}

	for _, res := range results {
		if !res.success {
			failCount++
			continue
		}
		name := sanitizeFilename(res.title)
		if name == "" {
			name = fmt.Sprintf("qr-%d", res.index+1)
		}
		// Deduplicate filenames
		if n, exists := usedNames[name]; exists {
			usedNames[name] = n + 1
			name = fmt.Sprintf("%s-%d", name, n+1)
		} else {
			usedNames[name] = 1
		}

		f, zipErr := zw.Create(name + ".png")
		if zipErr != nil {
			failCount++
			continue
		}
		if _, writeErr := f.Write(res.pngData); writeErr != nil {
			failCount++
			continue
		}
		successCount++
	}

	// Add a summary manifest
	manifest := fmt.Sprintf("QRit Bulk Generation Summary\n============================\nTotal rows:   %d\nGenerated:    %d\nFailed:       %d\n\nFailed rows:\n", len(rows), successCount, failCount)
	for _, res := range results {
		if !res.success {
			manifest += fmt.Sprintf("  - Row %d (%s): %s\n", res.index+1, res.title, res.errMsg)
		}
	}

	mf, _ := zw.Create("summary.txt")
	mf.Write([]byte(manifest))

	zw.Close()

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="qrit-bulk-qrcodes.zip"`)
	c.Header("X-Bulk-Total", fmt.Sprintf("%d", len(rows)))
	c.Header("X-Bulk-Success", fmt.Sprintf("%d", successCount))
	c.Header("X-Bulk-Failed", fmt.Sprintf("%d", failCount))
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

// BulkCSVPreview validates a CSV and returns a preview without generating QRs.
// POST /api/v1/workspaces/:id/bulk/preview
func (h *BulkHandler) BulkCSVPreview(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "CSV file required"})
		return
	}
	defer file.Close()

	rows, err := parseCSVRows(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	preview := make([]gin.H, 0, 10)
	for i, r := range rows {
		if i >= 10 {
			break
		}
		preview = append(preview, gin.H{
			"row":     i + 1,
			"title":   r.title,
			"content": r.content,
			"type":    r.qrType,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"total_rows": len(rows),
		"capped_at":  100,
		"preview":    preview,
	})
}

// ==========================================
// HELPERS
// ==========================================

func parseCSVRows(r io.Reader) ([]csvRow, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // allow variable columns

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("cannot read header row: %w", err)
	}

	// Normalize headers
	colIdx := map[string]int{"title": -1, "content": -1, "type": -1, "url": -1, "name": -1}
	for i, h := range headers {
		key := strings.ToLower(strings.TrimSpace(h))
		if _, ok := colIdx[key]; ok {
			colIdx[key] = i
		}
	}

	// Resolve aliases
	titleCol := firstValid(colIdx, "title", "name")
	contentCol := firstValid(colIdx, "content", "url")
	typeCol := colIdx["type"]

	if contentCol == -1 {
		return nil, fmt.Errorf("CSV must have a 'content' or 'url' column")
	}

	var rows []csvRow
	for {
		rec, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			continue // skip malformed rows
		}

		row := csvRow{qrType: "url"}

		if contentCol < len(rec) {
			row.content = strings.TrimSpace(rec[contentCol])
		}
		if titleCol != -1 && titleCol < len(rec) {
			row.title = strings.TrimSpace(rec[titleCol])
		}
		if typeCol != -1 && typeCol < len(rec) {
			t := strings.ToLower(strings.TrimSpace(rec[typeCol]))
			if t != "" {
				row.qrType = t
			}
		}

		if row.content == "" {
			continue // skip blank rows
		}
		if row.title == "" {
			row.title = row.content
			if len(row.title) > 40 {
				row.title = row.title[:40]
			}
		}

		rows = append(rows, row)
	}

	return rows, nil
}

func firstValid(m map[string]int, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != -1 {
			return v
		}
	}
	return -1
}

func decodeQRBase64(b64 string) ([]byte, error) {
	// Strip data URI prefix if present
	if idx := strings.Index(b64, ","); idx != -1 {
		b64 = b64[idx+1:]
	}
	b64 = strings.TrimSpace(b64)
	return base64.StdEncoding.DecodeString(b64)
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == ' ' {
			b.WriteRune(r)
		}
	}
	result := strings.TrimSpace(b.String())
	result = strings.ReplaceAll(result, " ", "-")
	if len(result) > 60 {
		result = result[:60]
	}
	return result
}

