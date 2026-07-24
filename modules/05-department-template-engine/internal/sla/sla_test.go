package sla

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	cases := []struct {
		phrase, priority string
		want             time.Duration
		ok               bool
	}{
		{"15m (P1) / 4h (P3)", "P1", 15 * time.Minute, true},
		{"15m (P1) / 4h (P3)", "P3", 4 * time.Hour, true},
		{"15m (P1) / 4h (P3)", "", 15 * time.Minute, true}, // first segment wins
		{"4h (P1) / 2d (P3)", "P3", 48 * time.Hour, true},
		{"4h", "", 4 * time.Hour, true},
		{"30 minutes", "", 30 * time.Minute, true},
		{"1 business day", "", 8 * time.Hour, true},
		{"2 business days", "", 16 * time.Hour, true},
		{"3 days", "", 72 * time.Hour, true},
		{"1 week", "", 7 * 24 * time.Hour, true},
		{"90s", "", 90 * time.Second, true},
		{"24x7", "", 0, false},
		{"best effort", "", 0, false},
		{"", "", 0, false},
	}
	for _, c := range cases {
		got, ok := Parse(c.phrase, c.priority)
		if ok != c.ok || got != c.want {
			t.Errorf("Parse(%q,%q) = %v,%v; want %v,%v", c.phrase, c.priority, got, ok, c.want, c.ok)
		}
	}
}
