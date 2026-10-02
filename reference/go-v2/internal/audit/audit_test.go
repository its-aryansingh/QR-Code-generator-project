package audit_test

import (
	"testing"

	"github.com/its-aryansingh/qrit/services/internal/audit"
)

func TestTruncateIPToPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"192.168.1.150", "192.168.1.0/24"},
		{"10.0.5.42", "10.0.5.0/24"},
		{"2001:0db8:85a3:0000:0000:8a2e:0370:7334", "2001:db8:85a3::/48"},
		{"invalid-ip", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := audit.TruncateIPToPrefix(tt.input)
		if tt.want == "" {
			if got != nil {
				t.Errorf("TruncateIPToPrefix(%q) = %v; want nil", tt.input, *got)
			}
		} else {
			if got == nil || *got != tt.want {
				t.Errorf("TruncateIPToPrefix(%q) = %v; want %q", tt.input, got, tt.want)
			}
		}
	}
}
