package domain

import (
	"fmt"
	"time"
)

// Period identifies a gift-rank window.
type Period string

const (
	// PeriodDay is a calendar-day window.
	PeriodDay Period = "day"
	// PeriodWeek is an ISO-week window.
	PeriodWeek Period = "week"
	// PeriodMonth is a calendar-month window.
	PeriodMonth Period = "month"
)

// Value returns the bucket key for the current time under this period.
func (p Period) Value() string {
	now := time.Now()
	switch p {
	case PeriodDay:
		return now.Format("2006-01-02")
	case PeriodWeek:
		_, week := now.ISOWeek()
		return fmt.Sprintf("%d-W%02d", now.Year(), week)
	case PeriodMonth:
		return now.Format("2006-01")
	default:
		return now.Format("2006-01-02")
	}
}

// AllPeriods lists every rank window we maintain.
func AllPeriods() []Period {
	return []Period{PeriodDay, PeriodWeek, PeriodMonth}
}
