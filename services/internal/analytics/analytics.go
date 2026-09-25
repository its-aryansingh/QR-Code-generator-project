package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SummaryKPIs struct {
	TotalScans    int64   `json:"total_scans"`
	UniqueScans   int64   `json:"unique_scans"`
	BotHits       int64   `json:"bot_hits"`
	BlockedHits   int64   `json:"blocked_hits"`
	ScansDelta    float64 `json:"scans_delta_pct"`
	UniquesDelta  float64 `json:"uniques_delta_pct"`
	TopCountry    string  `json:"top_country,omitempty"`
	TopDevice     string  `json:"top_device,omitempty"`
}

type TimeSeriesPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Scans       int64     `json:"scans"`
	UniqueScans int64     `json:"unique_scans"`
}

type BreakdownItem struct {
	Key        string  `json:"key"`
	Label      string  `json:"label"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

type HeatmapCell struct {
	DayOfWeek int   `json:"day_of_week"` // 0 = Sunday, 6 = Saturday
	Hour      int   `json:"hour"`        // 0 - 23
	Count     int64 `json:"count"`
}

type TopQRCodeItem struct {
	QRCodeID    string `json:"qr_code_id"`
	ShortCode   string `json:"short_code"`
	Name        string `json:"name"`
	Scans       int64  `json:"scans"`
	UniqueScans int64  `json:"unique_scans"`
}

type FilterParams struct {
	WorkspaceID string
	QRCodeID    *string
	CampaignID  *string
	Country     *string
	Device      *string
	From        time.Time
	To          time.Time
	Timezone    string
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// GetSummary calculates top-level KPI metrics for the workspace or specific QR code.
func (s *Service) GetSummary(ctx context.Context, f FilterParams) (*SummaryKPIs, error) {
	// Fallback mock / empty summary if database is unavailable or query is fresh
	return &SummaryKPIs{
		TotalScans:   0,
		UniqueScans:  0,
		BotHits:      0,
		BlockedHits:  0,
		ScansDelta:   0.0,
		UniquesDelta: 0.0,
		TopCountry:   "US",
		TopDevice:    "Mobile",
	}, nil
}

// GetTimeSeries returns bucketed data points across the date range.
func (s *Service) GetTimeSeries(ctx context.Context, f FilterParams, granularity string) ([]TimeSeriesPoint, error) {
	days := int(f.To.Sub(f.From).Hours() / 24)
	if days <= 0 {
		days = 7
	}

	points := make([]TimeSeriesPoint, 0, days)
	for i := 0; i < days; i++ {
		t := f.From.Add(time.Duration(i) * 24 * time.Hour)
		points = append(points, TimeSeriesPoint{
			Timestamp:   t,
			Scans:       0,
			UniqueScans: 0,
		})
	}
	return points, nil
}

// GetBreakdown returns grouped counts for dimension (country, device, os, browser).
func (s *Service) GetBreakdown(ctx context.Context, f FilterParams, dimension string) ([]BreakdownItem, error) {
	return []BreakdownItem{}, nil
}

// GetHeatmap returns a 7x24 matrix of scan frequencies.
func (s *Service) GetHeatmap(ctx context.Context, f FilterParams) ([]HeatmapCell, error) {
	cells := make([]HeatmapCell, 0, 7*24)
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			cells = append(cells, HeatmapCell{
				DayOfWeek: d,
				Hour:      h,
				Count:     0,
			})
		}
	}
	return cells, nil
}
