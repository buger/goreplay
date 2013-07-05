package stats

import (
	"sync"
)

type RequestStat struct {
	Codes map[string]int // { 200: 10, 404:2, 500:1 }

	Count    int // All requests including unfinished or errors
	Finished int

	AvgLat float64
	MaxLat float64
	MinLat float64

	sync.Mutex
}

// Stats for given period of time in context of URL
type PeriodStats struct {
	Timestamp int64

	PathStats map[string]*RequestStat

	TotalStats *RequestStat
}
