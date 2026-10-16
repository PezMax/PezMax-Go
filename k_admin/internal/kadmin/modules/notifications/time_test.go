package notifications

import (
	"testing"
	"time"
)

func TestToTimeStringKeepsTimezone(t *testing.T) {
	value := time.Date(2026, 10, 15, 14, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	const want = "2026-10-15T14:30:00+08:00"
	for _, input := range []interface{}{value, "2026-10-15 14:30:00+08"} {
		if got := toTimeString(input); got != want {
			t.Errorf("toTimeString(%v) = %q, want %q", input, got, want)
		}
	}
}
