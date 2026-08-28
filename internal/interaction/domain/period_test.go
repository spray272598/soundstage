package domain

import (
	"testing"
	"time"
)

func TestPeriodValue(t *testing.T) {
	// Pin a fixed time so the test is deterministic.
	fixed := time.Date(2026, time.August, 28, 13, 0, 0, 0, time.UTC)

	// Override the clock used by Value via a temporary monkey-free approach:
	// Value() uses time.Now(), so instead we assert the format shape.
	_ = fixed

	if got := PeriodDay.Value(); len(got) != len("2026-08-28") {
		t.Fatalf("day period format wrong: %q", got)
	}
	if got := PeriodMonth.Value(); len(got) != len("2026-08") {
		t.Fatalf("month period format wrong: %q", got)
	}
	week := PeriodWeek.Value()
	// ISO week format: 2026-W35
	if len(week) != len("2026-W35") {
		t.Fatalf("week period format wrong: %q", week)
	}

	for _, p := range AllPeriods() {
		if p.Value() == "" {
			t.Fatalf("empty value for period %s", p)
		}
	}
}
