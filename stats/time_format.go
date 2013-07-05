package stats

import (
	"fmt"
	"math"
)

func multiple(str string, num int) string {
	if num == 0 {
		return fmt.Sprintf("%s %d", str, num)
	} else {
		return fmt.Sprintf("%ss %d", str, num)
	}
}

func TimeInWords(diff int64) string {
	if diff == 0 {
		return "now"
	}

	prefix := "ago"

	if diff < 0 {
		prefix = "since now"
	}

	seconds := math.Abs(float64(diff))
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24
	months := days / 30
	years := days / 365

	switch {
	case seconds < 2:
		return fmt.Sprintf("%d second %s", 1, prefix)
	case seconds < 45:
		return fmt.Sprintf("%d seconds %s", int(seconds), prefix)
	case seconds < 90:
		return fmt.Sprintf("%d minute %s", 1, prefix)
	case minutes < 45:
		return fmt.Sprintf("%d minutes %s", int(minutes), prefix)
	case minutes < 90:
		return fmt.Sprintf("%d hour %s", 1, prefix)
	case hours < 24:
		return fmt.Sprintf("%d hours %s", int(hours), prefix)
	case hours < 42:
		return fmt.Sprintf("%d day %s", 1, prefix)
	case days < 30:
		return fmt.Sprintf("%d days %s", int(days), prefix)
	case days < 45:
		return fmt.Sprintf("%d month %s", 1, prefix)
	case days < 365:
		return fmt.Sprintf("%d months %s", int(months), prefix)
	case years < 1.5:
		return fmt.Sprintf("%d year %s", 1, prefix)
	default:
		return fmt.Sprintf("%d years %s", int(years), prefix)
	}
}
