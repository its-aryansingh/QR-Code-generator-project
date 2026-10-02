package routing

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestRoutingEngineTableCases(t *testing.T) {
	istLoc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		istLoc = time.FixedZone("IST", 5*3600+1800)
	}

	tests := []struct {
		name       string
		version    Version
		facts      RequestFacts
		now        time.Time
		loc        *time.Location
		wantURL    string
		wantRuleID string
	}{
		{
			name: "Default destination when no rules exist",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules:              nil,
			},
			facts:      RequestFacts{Country: "US"},
			now:        time.Now(),
			wantURL:    "https://example.com/default",
			wantRuleID: "",
		},
		{
			name: "Country in matches",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_india",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "country", Op: "in", Value: []string{"IN", "BD"}}}},
						DestinationURL: "https://example.com/india",
					},
				},
			},
			facts:      RequestFacts{Country: "IN"},
			now:        time.Now(),
			wantURL:    "https://example.com/india",
			wantRuleID: "r_india",
		},
		{
			name: "Country in does not match",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_india",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "country", Op: "in", Value: []string{"IN", "BD"}}}},
						DestinationURL: "https://example.com/india",
					},
				},
			},
			facts:      RequestFacts{Country: "US"},
			now:        time.Now(),
			wantURL:    "https://example.com/default",
			wantRuleID: "",
		},
		{
			name: "OS in iOS matches",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_ios",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "os", Op: "in", Value: []string{"iOS", "iPadOS"}}}},
						DestinationURL: "https://apps.apple.com/app",
					},
				},
			},
			facts:      RequestFacts{OS: "iOS"},
			now:        time.Now(),
			wantURL:    "https://apps.apple.com/app",
			wantRuleID: "r_ios",
		},
		{
			name: "Device type in mobile matches",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_mob",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "device_type", Op: "in", Value: []string{"mobile"}}}},
						DestinationURL: "https://m.example.com",
					},
				},
			},
			facts:      RequestFacts{DeviceType: "mobile"},
			now:        time.Now(),
			wantURL:    "https://m.example.com",
			wantRuleID: "r_mob",
		},
		{
			name: "Local time standard window matches",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_lunch",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "local_time", Op: "between", Value: []string{"11:00", "15:00"}}}},
						DestinationURL: "https://example.com/lunch",
					},
				},
			},
			facts:      RequestFacts{},
			now:        time.Date(2026, 9, 26, 12, 30, 0, 0, time.UTC),
			wantURL:    "https://example.com/lunch",
			wantRuleID: "r_lunch",
		},
		{
			name: "Local time standard window misses",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_lunch",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "local_time", Op: "between", Value: []string{"11:00", "15:00"}}}},
						DestinationURL: "https://example.com/lunch",
					},
				},
			},
			facts:      RequestFacts{},
			now:        time.Date(2026, 9, 26, 16, 0, 0, 0, time.UTC),
			wantURL:    "https://example.com/default",
			wantRuleID: "",
		},
		{
			name: "Local time wrap around midnight (22:00 to 04:00) before midnight",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_night",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "local_time", Op: "between", Value: []string{"22:00", "04:00"}}}},
						DestinationURL: "https://example.com/night",
					},
				},
			},
			facts:      RequestFacts{},
			now:        time.Date(2026, 9, 26, 23, 15, 0, 0, time.UTC),
			wantURL:    "https://example.com/night",
			wantRuleID: "r_night",
		},
		{
			name: "Local time wrap around midnight (22:00 to 04:00) after midnight",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_night",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "local_time", Op: "between", Value: []string{"22:00", "04:00"}}}},
						DestinationURL: "https://example.com/night",
					},
				},
			},
			facts:      RequestFacts{},
			now:        time.Date(2026, 9, 26, 2, 45, 0, 0, time.UTC),
			wantURL:    "https://example.com/night",
			wantRuleID: "r_night",
		},
		{
			name: "Local time in IST timezone",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_ist_lunch",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "local_time", Op: "between", Value: []string{"12:00", "14:00"}}}},
						DestinationURL: "https://example.com/ist_lunch",
					},
				},
			},
			facts: RequestFacts{},
			// 07:30 UTC = 13:00 IST
			now:        time.Date(2026, 9, 26, 7, 30, 0, 0, time.UTC),
			loc:        istLoc,
			wantURL:    "https://example.com/ist_lunch",
			wantRuleID: "r_ist_lunch",
		},
		{
			name: "Weekday in range (Saturday = 6)",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_weekend",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "weekday", Op: "in", Value: []int{6, 7}}}},
						DestinationURL: "https://example.com/weekend",
					},
				},
			},
			facts: RequestFacts{},
			// 2026-09-26 is a Saturday (ISO 6)
			now:        time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC),
			wantURL:    "https://example.com/weekend",
			wantRuleID: "r_weekend",
		},
		{
			name: "Weekday out of range",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_weekday",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "weekday", Op: "in", Value: []int{1, 2, 3, 4, 5}}}},
						DestinationURL: "https://example.com/workdays",
					},
				},
			},
			facts: RequestFacts{},
			// 2026-09-26 is a Saturday
			now:        time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC),
			wantURL:    "https://example.com/default",
			wantRuleID: "",
		},
		{
			name: "Scan count limit threshold",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_early_bird",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "scan_count", Op: "lt", Value: 100}}},
						DestinationURL: "https://example.com/early_bird",
					},
				},
			},
			facts:      RequestFacts{ScanCount: 42},
			now:        time.Now(),
			wantURL:    "https://example.com/early_bird",
			wantRuleID: "r_early_bird",
		},
		{
			name: "Scan count limit exceeded",
			version: Version{
				DefaultDestination: "https://example.com/default",
				Rules: []Rule{
					{
						ID:             "r_early_bird",
						Enabled:        true,
						When:           &WhenGroup{All: []Condition{{Field: "scan_count", Op: "lt", Value: 100}}},
						DestinationURL: "https://example.com/early_bird",
					},
				},
			},
			facts:      RequestFacts{ScanCount: 150},
			now:        time.Now(),
			wantURL:    "https://example.com/default",
			wantRuleID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, ruleID := Resolve(tt.version, tt.facts, tt.now, tt.loc)
			if url != tt.wantURL {
				t.Errorf("got URL = %q, want %q", url, tt.wantURL)
			}
			if ruleID != tt.wantRuleID {
				t.Errorf("got ruleID = %q, want %q", ruleID, tt.wantRuleID)
			}
		})
	}
}

func TestRoutingABSplitProperties(t *testing.T) {
	v := Version{
		DefaultDestination: "https://example.com/default",
		Rules: []Rule{
			{
				ID:      "r_ab",
				Enabled: true,
				When:    nil, // always match
				Split: []SplitVariant{
					{Variant: "A", Weight: 50, DestinationURL: "https://example.com/a"},
					{Variant: "B", Weight: 50, DestinationURL: "https://example.com/b"},
				},
			},
		},
	}

	// 1. Stickiness check: same visitor facts always produce exact same variant
	facts1 := RequestFacts{
		QRCodeID:  "018e0000-0000-7000-8000-000000000001",
		IP:        "198.51.100.42",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4)",
	}

	destFirst, ruleFirst := Resolve(v, facts1, time.Now(), nil)
	for i := 0; i < 100; i++ {
		d, r := Resolve(v, facts1, time.Now(), nil)
		if d != destFirst || r != ruleFirst {
			t.Fatalf("stickiness violated at iteration %d: got %s, want %s", i, d, destFirst)
		}
	}

	// 2. Uniform 50/50 split distribution check over 10,000 synthetic visitors (within ±2%)
	counts := map[string]int{"A": 0, "B": 0}
	total := 10000

	for i := 0; i < total; i++ {
		f := RequestFacts{
			QRCodeID:  "018e0000-0000-7000-8000-000000000001",
			IP:        fmt.Sprintf("10.0.%d.%d", i/256, i%256),
			UserAgent: fmt.Sprintf("UA-%d", i),
		}
		_, ruleID := Resolve(v, f, time.Now(), nil)
		if ruleID == "r_ab:A" {
			counts["A"]++
		} else if ruleID == "r_ab:B" {
			counts["B"]++
		}
	}

	pctA := float64(counts["A"]) / float64(total) * 100.0
	pctB := float64(counts["B"]) / float64(total) * 100.0

	if math.Abs(pctA-50.0) > 2.0 || math.Abs(pctB-50.0) > 2.0 {
		t.Fatalf("split distribution out of ±2%% tolerance: A=%.2f%%, B=%.2f%%", pctA, pctB)
	}
}
