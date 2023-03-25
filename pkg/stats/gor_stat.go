package stats

import (
	"runtime"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// Stats is a struct that holds the stats for a given statName
type Stats struct {
	statName string
	rateMs   int
	latest   int
	mean     int
	max      int
	count    int
}

// New creates a new Stats struct
func New(statName string, rateMs int) (s *Stats) {
	s = new(Stats)
	s.statName = statName
	s.rateMs = rateMs
	s.latest = 0
	s.mean = 0
	s.max = 0
	s.count = 0

	go s.reportStats()

	return
}

func (s *Stats) Write(latest int) {
	if latest > s.max {
		s.max = latest
	}
	if latest != 0 {
		s.mean = ((s.mean * s.count) + latest) / (s.count + 1)
	}
	s.latest = latest
	s.count = s.count + 1
}

// Reset resets the stats
func (s *Stats) Reset() {
	s.latest = 0
	s.max = 0
	s.mean = 0
	s.count = 0
}

func (s *Stats) String() string {
	return s.statName + ":" + strconv.Itoa(s.latest) + "," + strconv.Itoa(s.mean) + "," + strconv.Itoa(s.max) + "," + strconv.Itoa(s.count) + "," + strconv.Itoa(s.count/(s.rateMs/1000.0)) + "," + strconv.Itoa(runtime.NumGoroutine())
}

func (s *Stats) reportStats() {
	log.Info().Msg(s.statName + ":latest,mean,max,count,count/second,gcount")
	for {
		log.Info().Msg(s.String())
		s.Reset()
		time.Sleep(time.Duration(s.rateMs) * time.Millisecond)
	}
}
