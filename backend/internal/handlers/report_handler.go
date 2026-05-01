package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qrapp/backend/internal/repository"
)

// ReportHandler handles PDF/CSV export and report generation
type ReportHandler struct {
	wsRepo *repository.WorkspaceRepository
}

func NewReportHandler(wsRepo *repository.WorkspaceRepository) *ReportHandler {
	return &ReportHandler{wsRepo: wsRepo}
}

// ExportAnalyticsCSV exports workspace analytics as CSV
func (h *ReportHandler) ExportAnalyticsCSV(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	scansByDate, err := h.wsRepo.GetWorkspaceScansByDate(wsID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch data"})
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=analytics_%s_%dd.csv", wsID.String()[:8], days))

	w := csv.NewWriter(c.Writer)
	w.Write([]string{"Date", "Scans"})

	for _, row := range scansByDate {
		date := fmt.Sprintf("%v", row["date"])
		count := fmt.Sprintf("%v", row["count"])
		w.Write([]string{date, count})
	}
	w.Flush()
}

// ExportQRCodesCSV exports all QR codes in a workspace as CSV
func (h *ReportHandler) ExportQRCodesCSV(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))

	records, _, err := h.wsRepo.GetWorkspaceQRCodes(wsID, nil, "", 1000, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch QR codes"})
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=qrcodes_%s.csv", wsID.String()[:8]))

	w := csv.NewWriter(c.Writer)
	w.Write([]string{"Title", "Content", "Type", "Mode", "Scans", "Active", "Created At"})

	for _, qr := range records {
		mode := "Static"
		if qr.IsDynamic {
			mode = "Dynamic"
		}
		active := "Yes"
		if !qr.IsActive {
			active = "No"
		}
		w.Write([]string{
			qr.Title,
			qr.Content,
			qr.QRType,
			mode,
			strconv.Itoa(qr.ScanCount),
			active,
			qr.CreatedAt.Format(time.RFC3339),
		})
	}
	w.Flush()
}

// ExportLeadsCSV exports all captured leads as CSV
func (h *ReportHandler) ExportLeadsCSV(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))
	pageID := c.Query("page_id")

	var pageUUID *uuid.UUID
	if pageID != "" {
		id, _ := uuid.Parse(pageID)
		pageUUID = &id
	}

	leads, _, err := h.wsRepo.GetLeads(wsID, pageUUID, 10000, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch leads"})
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=leads_export.csv")

	w := csv.NewWriter(c.Writer)
	w.Write([]string{"Email", "Data", "Source", "Opted In", "IP Address", "Submitted At"})

	for _, lead := range leads {
		optedIn := "No"
		if lead.OptedIn {
			optedIn = "Yes"
		}
		w.Write([]string{
			lead.Email,
			lead.Data,
			lead.Source,
			optedIn,
			lead.IPAddress,
			lead.CreatedAt.Format(time.RFC3339),
		})
	}
	w.Flush()
}

// GenerateReport returns a JSON summary report for downloading
func (h *ReportHandler) GenerateReport(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	qrCount, scanCount, _ := h.wsRepo.GetWorkspaceStats(wsID)
	scansByDate, _ := h.wsRepo.GetWorkspaceScansByDate(wsID, days)
	devices, _ := h.wsRepo.GetWorkspaceDeviceBreakdown(wsID)
	countries, _ := h.wsRepo.GetWorkspaceCountryBreakdown(wsID)
	browsers, _ := h.wsRepo.GetWorkspaceBrowserBreakdown(wsID)
	topQR, _ := h.wsRepo.GetTopPerformingQR(wsID, 10)

	workspace, _ := h.wsRepo.GetByID(wsID)
	memberCount, _ := h.wsRepo.CountMembers(wsID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"report_generated": time.Now().UTC().Format(time.RFC3339),
			"period_days":      days,
			"workspace": gin.H{
				"name":    workspace.Name,
				"plan":    workspace.Plan,
				"members": memberCount,
			},
			"summary": gin.H{
				"total_qr_codes": qrCount,
				"total_scans":    scanCount,
			},
			"scans_by_date":     scansByDate,
			"device_breakdown":  devices,
			"country_breakdown": countries,
			"browser_breakdown": browsers,
			"top_qr_codes":      topQR,
		},
	})
}
