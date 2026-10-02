package domains

import (
	"testing"
)

func TestCleanDomain(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"go.brand.com", "go.brand.com", false},
		{"https://go.brand.com/test", "go.brand.com", false},
		{"http://GO.BRAND.COM:8080", "go.brand.com", false},
		{"qrit.io", "", true},
		{"sub.qrit.io", "", true},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		got, err := CleanDomain(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("CleanDomain(%q) err = %v, wantErr = %v", tt.input, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("CleanDomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
