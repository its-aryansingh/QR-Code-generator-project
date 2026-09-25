package abuse

import (
	"testing"
)

func TestIsDisposable(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"user@mailinator.com", true},
		{"TEST@10MinuteMail.COM", true},
		{"someone@sharklasers.com", true},
		{"temp@subdomain.trashmail.com", true},
		{"valid.user@gmail.com", false},
		{"work@company.co.in", false},
		{"founder@qrit.io", false},
		{"admin@outlook.com", false},
		{"mailinator.com", true},
		{"", false},
	}

	for _, tc := range tests {
		got := IsDisposable(tc.input)
		if got != tc.expected {
			t.Errorf("IsDisposable(%q) = %v; want %v", tc.input, got, tc.expected)
		}
	}
}
