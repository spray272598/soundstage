package timeutil

import "time"

// Now returns the current UTC time.
func Now() time.Time {
	return time.Now().UTC()
}

// NowMilli returns the current UTC timestamp in milliseconds.
func NowMilli() int64 {
	return Now().UnixMilli()
}
