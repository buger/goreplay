package replay

import (
	"fmt"
	"os"
	"path"
	"sync"
	"time"
)

const STATS_BUF = 61

// We store stats for last 60 seconds.
// Number 60 used because http timeout limit is set to 60 seconds.
// So stats older then 60 seconds is can't be changed.
type SiteStats struct {
	Start  int64
	Update int64

	stats []*PeriodStats

	file *os.File

	fileOffset int64

	mutex sync.Mutex
}

var statsID int = 0

func NewSiteStats() (stats *SiteStats) {
	stats = &SiteStats{}
	stats.Start = time.Now().Unix()
	stats.Update = time.Now().Unix()
	stats.stats = make([]*PeriodStats, STATS_BUF)

	stats.stats[0] = &PeriodStats{}
	stats.stats[0].Reset()

	stats.RotateStats()

	if Settings.StatPath != "" {
		var err error

		fileName := fmt.Sprint("stats-", time.Now().Format("2006-01-02T15:04:05"), ".", statsID, ".gor")
		filePath := path.Join(Settings.StatPath, fileName)

		stats.file, err = os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)

		if err != nil {
			fmt.Println("ERROR: Can't write stats to ", Settings.StatPath, err)
		}

		fmt.Println("Writing stats to:", filePath)
	}

	statsID++

	return
}

func (s *SiteStats) GetCurrent() (rs *PeriodStats, idx int) {
	idx = int(s.Update-s.Start) % STATS_BUF

	if s.stats[idx] == nil {
		s.stats[idx] = &PeriodStats{}
		s.stats[idx].Reset()
	}

	return s.stats[idx], idx
}

func (s *SiteStats) FindByTime(ts int64) *PeriodStats {
	for _, stat := range s.stats {
		if stat != nil && stat.Timestamp == ts {
			return stat
		}
	}

	return nil
}

func (s *SiteStats) RotateStats() {
	go func() {
		for {
			time.Sleep(1 * time.Second)

			s.WriteStats()

			s.UpdateCurrent()
		}
	}()
}

func (s *SiteStats) UpdateCurrent() {
	if s.Update != time.Now().Unix() {
		s.Update = time.Now().Unix()
		rs, _ := s.GetCurrent()
		rs.Reset()
	}
}

func (s *SiteStats) IncReq(req *HttpRequest) {
	s.UpdateCurrent()

	ps, _ := s.GetCurrent()
	ps.IncReq(req)
}

func (s *SiteStats) IncResp(resp *HttpResponse) {
	ps := s.FindByTime(resp.req.created / int64(time.Second))

	ps.IncResp(resp)
}

func (s *SiteStats) Count() int {
	ps, _ := s.GetCurrent()
	return ps.Count()
}

// Every second flush stats to disk
func (s *SiteStats) WriteStats() {
	if s.file == nil {
		return
	}

	// On every write, we rewriting whole stats, except last expired element
	_, err := s.file.Seek(-s.fileOffset, 2)

	// If file is too small (less then STATS_BUF records), just seek to start of the file
	if err != nil {
		s.file.Seek(0, 0)
	}

	_, currIdx := s.GetCurrent()

	s.fileOffset = 0

	for i := (STATS_BUF - 1); i >= 0; i-- {
		idx := currIdx - i

		if idx < 0 {
			idx = STATS_BUF + idx
		}

		ps := s.stats[idx]

		if ps != nil && ps.Count() > 0 {
			bytes := ps.Encode()
			n, _ := s.file.Write(bytes)

			// We need length of stats records without expired element
			if (s.Update - ps.Timestamp) < (STATS_BUF - 1) {
				s.fileOffset += int64(n)
			}
		}
	}
}
