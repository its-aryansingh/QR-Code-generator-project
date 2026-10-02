package realtime

import (
	"testing"
	"time"
)

func TestFormatMinute(t *testing.T) {
	fixedTime := time.Date(2026, 9, 26, 14, 5, 30, 0, time.UTC)
	got := formatMinute(fixedTime)
	want := "202609261405"
	if got != want {
		t.Errorf("formatMinute(%v) = %s, want %s", fixedTime, got, want)
	}
}
