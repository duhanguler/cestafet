package mcpserver

import (
	"testing"
	"time"
)

func TestParseExpiry(t *testing.T) {
	ok := map[string]time.Duration{
		"":     0,
		"30m":  30 * time.Minute,
		"2h":   2 * time.Hour,
		"1d":   24 * time.Hour,
		"1d6h": 30 * time.Hour,
		" 3d ": 72 * time.Hour,
	}
	for in, want := range ok {
		if got, err := parseExpiry(in); err != nil || got != want {
			t.Errorf("parseExpiry(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"soon", "-2h", "0h", "2", "1w", "d"} {
		if _, err := parseExpiry(in); err == nil {
			t.Errorf("parseExpiry(%q): expected an error", in)
		}
	}
}
