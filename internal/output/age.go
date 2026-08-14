package output

import (
	"fmt"
	"time"
)

// RelativeAge renders how long ago an epoch-milliseconds timestamp was, in
// the largest sensible unit: "45m", "3h", "12d", "5mo", "2y". Returns "-"
// for zero/unknown timestamps.
func RelativeAge(epochMillis int64) string {
	if epochMillis <= 0 {
		return "-"
	}
	return relativeAgeAt(epochMillis, time.Now())
}

func relativeAgeAt(epochMillis int64, now time.Time) string {
	age := now.Sub(time.UnixMilli(epochMillis))
	switch {
	case age < time.Minute:
		return "now"
	case age < time.Hour:
		return fmt.Sprintf("%dm", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh", int(age.Hours()))
	case age < 60*24*time.Hour:
		return fmt.Sprintf("%dd", int(age.Hours()/24))
	case age < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(age.Hours()/(30*24)))
	default:
		return fmt.Sprintf("%dy", int(age.Hours()/(365*24)))
	}
}
