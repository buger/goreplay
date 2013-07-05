package replay

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"
)

// Stats stores in context of current timestamp
type RequestStat struct {
	Codes map[string]int // { 200: 10, 404:2, 500:1 }

	Count    int // All requests including unfinished or errors
	Finished int

	AvgLat float64
	MaxLat float64
	MinLat float64

	sync.Mutex
}

// Called on request start
func (s *RequestStat) IncReq() {
	s.Lock()
	defer s.Unlock()

	s.Count++
}

// Called after response
func (s *RequestStat) IncResp(resp *HttpResponse) {
	s.Lock()
	defer s.Unlock()

	var statusCode int

	if resp.err != nil {
		statusCode = 509
	} else {
		statusCode = resp.resp.StatusCode
	}

	s.Codes[strconv.Itoa(statusCode)]++
	s.Finished++

	latency := float64(resp.created-resp.req.created) / float64(time.Millisecond)

	if s.MinLat == 0 {
		s.MinLat = latency
	}

	s.MaxLat = math.Max(s.MaxLat, latency)
	s.MinLat = math.Min(s.MinLat, latency)
	s.AvgLat = s.AvgLat + (latency-s.AvgLat)/float64(s.Finished)
}

// Updated stats timestamp to current time and reset to zero all stats values
func (s *RequestStat) Reset() {
	s.Lock()
	defer s.Unlock()

	s.Codes = make(map[string]int)
	s.Count = 0
	s.Finished = 0

	s.AvgLat = 0
	s.MaxLat = 0
	s.MinLat = 0
}

// Stats for given period of time in context of URL
type PeriodStats struct {
	Timestamp int64

	PathStats map[string]*RequestStat

	TotalStats *RequestStat
}

func (s *PeriodStats) URLStat(req *HttpRequest) *RequestStat {
	path := req.req.URL.Path

	stats, ok := s.PathStats[path]

	if !ok {
		s.PathStats[path] = &RequestStat{}
		stats = s.PathStats[path]
		stats.Reset()
	}

	return stats
}

// Called on request start
func (s *PeriodStats) IncReq(req *HttpRequest) {
	s.URLStat(req).IncReq()
	s.TotalStats.IncReq()
}

// Called after response
func (s *PeriodStats) IncResp(resp *HttpResponse) {
	s.URLStat(resp.req).IncResp(resp)
	s.TotalStats.IncResp(resp)
}

func (s *PeriodStats) Encode() []byte {
	var data []byte

	data, err := json.Marshal(s)

	if err != nil {
		fmt.Println("Error while encoding", err)
	}

	data = append(data, '\n')

	return data
}

func (s *PeriodStats) Reset() {
	s.PathStats = make(map[string]*RequestStat)
	s.Timestamp = time.Now().Unix()

	s.TotalStats = &RequestStat{}
	s.TotalStats.Reset()
}

func (s *PeriodStats) Count() int {
	return s.TotalStats.Count
}

func (s *PeriodStats) String() string {
	return fmt.Sprint(s.TotalStats)
}
