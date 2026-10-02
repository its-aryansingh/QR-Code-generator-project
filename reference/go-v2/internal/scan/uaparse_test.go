package scan

import (
	"testing"
)

func TestParseUserAgent(t *testing.T) {
	tests := []struct {
		name        string
		ua          string
		wantDevice  string
		wantOS      string
		wantBrowser string
	}{
		{
			name:        "iPhone Safari",
			ua:          "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
			wantDevice:  "mobile",
			wantOS:      "iOS",
			wantBrowser: "Safari",
		},
		{
			name:        "Android Chrome",
			ua:          "Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.6261.64 Mobile Safari/537.36",
			wantDevice:  "mobile",
			wantOS:      "Android",
			wantBrowser: "Chrome",
		},
		{
			name:        "iPad Safari",
			ua:          "Mozilla/5.0 (iPad; CPU OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1",
			wantDevice:  "tablet",
			wantOS:      "iOS",
			wantBrowser: "Safari",
		},
		{
			name:        "Windows Edge",
			ua:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/122.0.2365.52",
			wantDevice:  "desktop",
			wantOS:      "Windows",
			wantBrowser: "Edge",
		},
		{
			name:        "Samsung Internet",
			ua:          "Mozilla/5.0 (Linux; Android 13; SAMSUNG SM-A536B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/23.0 Chrome/115.0.0.0 Mobile Safari/537.36",
			wantDevice:  "mobile",
			wantOS:      "Android",
			wantBrowser: "Samsung Internet",
		},
		{
			name:        "macOS Firefox",
			ua:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:123.0) Gecko/20100101 Firefox/123.0",
			wantDevice:  "desktop",
			wantOS:      "macOS",
			wantBrowser: "Firefox",
		},
		{
			name:        "Empty UA",
			ua:          "",
			wantDevice:  "unknown",
			wantOS:      "unknown",
			wantBrowser: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ParseUserAgent(tt.ua)
			if res.DeviceType != tt.wantDevice {
				t.Errorf("DeviceType = %s, want %s", res.DeviceType, tt.wantDevice)
			}
			if res.OS != tt.wantOS {
				t.Errorf("OS = %s, want %s", res.OS, tt.wantOS)
			}
			if res.Browser != tt.wantBrowser {
				t.Errorf("Browser = %s, want %s", res.Browser, tt.wantBrowser)
			}
		})
	}
}
