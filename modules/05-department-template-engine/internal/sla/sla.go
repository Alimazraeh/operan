// Package sla parses the human-authored service-level strings used in the
// template service portfolios ("15m (P1) / 4h (P3)", "4h", "1 business day",
// "2 days", "24x7") into durations for SLA clocks.
package sla

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var segment = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(business\s+days?|business\s+hours?|[a-zA-Z]+)`)

// Parse returns the duration for an SLA phrase, choosing the segment matching
// priority (e.g. "P1") when the phrase lists several ("15m (P1) / 4h (P3)").
// A business day counts as 8h, a business hour as 1h. Returns ok=false for
// phrases that carry no duration ("24x7", "best effort", "").
func Parse(phrase, priority string) (time.Duration, bool) {
	phrase = strings.TrimSpace(phrase)
	if phrase == "" {
		return 0, false
	}

	// Multi-tier phrases: pick the alternative mentioning the priority.
	if priority != "" && strings.Contains(phrase, "/") {
		for _, alt := range strings.Split(phrase, "/") {
			if strings.Contains(strings.ToUpper(alt), strings.ToUpper(priority)) {
				if d, ok := parseOne(alt); ok {
					return d, true
				}
			}
		}
	}
	return parseOne(phrase)
}

func parseOne(s string) (time.Duration, bool) {
	m := segment.FindStringSubmatch(strings.ToLower(s))
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	unit := strings.TrimSpace(m[2])
	switch {
	case strings.HasPrefix(unit, "business day"):
		return time.Duration(n * 8 * float64(time.Hour)), true
	case strings.HasPrefix(unit, "business hour"):
		return time.Duration(n * float64(time.Hour)), true
	case strings.HasPrefix(unit, "m"): // m, min, minutes
		return time.Duration(n * float64(time.Minute)), true
	case strings.HasPrefix(unit, "h"): // h, hr, hours
		return time.Duration(n * float64(time.Hour)), true
	case strings.HasPrefix(unit, "d"): // d, day, days
		return time.Duration(n * 24 * float64(time.Hour)), true
	case strings.HasPrefix(unit, "w"): // w, week
		return time.Duration(n * 7 * 24 * float64(time.Hour)), true
	case strings.HasPrefix(unit, "s"): // s, sec
		return time.Duration(n * float64(time.Second)), true
	}
	return 0, false
}
