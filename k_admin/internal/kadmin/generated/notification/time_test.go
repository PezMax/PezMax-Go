package notification

import (
	"testing"
	"time"
)

func TestTimeTextPreservesTimestampWallClock(t *testing.T) {
	previousLocation := time.Local
	time.Local = time.FixedZone("CST", 8*60*60)
	t.Cleanup(func() { time.Local = previousLocation })
	value := time.Date(2026, 10, 15, 14, 30, 0, 0, time.UTC)
	const want = "2026-10-15 14:30:00"
	for _, input := range []interface{}{value, "2026-10-15 14:30:00", "2026-10-15T14:30:00Z"} {
		if got := timeText(input); got != want {
			t.Errorf("timeText(%v) = %q, want %q", input, got, want)
		}
	}
}
