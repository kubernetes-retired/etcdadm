package pki

import (
	"testing"
	"time"
)

func TestParseHumanDuration(t *testing.T) {
	grid := []struct {
		Value    string
		Expected time.Duration
	}{
		{"10m", 10 * time.Minute},
		{"24h", 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"10d", 10 * 24 * time.Hour},
		{"365d", 365 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
		{"10y", 10 * 365 * 24 * time.Hour},
	}

	for _, g := range grid {
		g := g
		t.Run(g.Value, func(t *testing.T) {
			actual, err := ParseHumanDuration(g.Value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if actual != g.Expected {
				t.Fatalf("unexpected value; expected=%v, actual=%v", g.Expected, actual)
			}
		})
	}
}
