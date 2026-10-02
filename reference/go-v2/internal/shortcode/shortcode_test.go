package shortcode_test

import (
	"testing"

	"github.com/its-aryansingh/qrit/services/internal/shortcode"
)

func TestNormalise(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abc1234", "ABC1234"},
		{"o1l9999", "0119999"},
		{"IL0abcd", "110ABCD"},
		{"code123/", "C0DE123"},
		{"  7k9m2x1/  ", "7K9M2X1"},
	}

	for _, tt := range tests {
		got := shortcode.Normalise(tt.input)
		if got != tt.expected {
			t.Errorf("Normalise(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		code    string
		wantErr bool
	}{
		{"7K9M2X1", false},
		{"0123456", false},
		{"ABCDEF0", false},
		{"TOO_SHORT", true},
		{"WAY_TOO_LONG_FOR_CODE", true},
		{"7K9M2XU", true},      // 'U' is excluded from Crockford Base32
		{"ADMIN01", true},      // Denylisted / reserved
		{"FUCK999", true},      // Denylisted
		{"  7k9m2x1  ", false}, // Normalised
	}

	for _, tt := range tests {
		err := shortcode.Validate(tt.code)
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate(%q) err = %v; wantErr %v", tt.code, err, tt.wantErr)
		}
	}
}

func TestGenerate(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code, err := shortcode.Generate()
		if err != nil {
			t.Fatalf("Generate failed on iteration %d: %v", i, err)
		}
		if err := shortcode.Validate(code); err != nil {
			t.Errorf("Generated invalid code %q: %v", code, err)
		}
		if seen[code] {
			t.Errorf("Collision on generated code %q", code)
		}
		seen[code] = true
	}
}

func TestPayload(t *testing.T) {
	payload := shortcode.Payload("qrit.io", "7k9m2x1")
	expected := "HTTPS://QRIT.IO/7K9M2X1"
	if payload != expected {
		t.Errorf("Payload() = %q; want %q", payload, expected)
	}
}
