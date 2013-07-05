package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	tm "github.com/buger/goterm"
	"log"
	"os"
	"strings"
	"time"
)

// Enable debug logging only if "--verbose" flag passed
func Debug(v ...interface{}) {
	if Settings.Verbose {
		log.Println(v...)
	}
}

// Because its sub-program, Run acts as `main`
func Run() {
	file, err := os.Open(Settings.StatFile)

	if err != nil {
		log.Fatal(err)
	}

	var allStats []*PeriodStats

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		stat := &PeriodStats{}

		json.Unmarshal(scanner.Bytes(), stat)

		allStats = append(allStats, stat)
	}

	tm.Clear()

	for {
		tm.MoveCursor(1, 1)

		tm.Println("Gor stats tool")
		tm.Println("Current Time:", time.Now().Format(time.RFC1123))
		tm.Println("\n\n")

		_, err = file.Seek(-50000, 2)

		scanner := bufio.NewScanner(file)

		if err != nil {
			file.Seek(0, 0)
		} else {
			scanner.Scan() // Skip first line since it can be incomplete
		}

		for scanner.Scan() {
			stat := &PeriodStats{}

			json.Unmarshal(scanner.Bytes(), stat)

			found := false

			for i := len(allStats) - 1; i > 0; i-- {
				if stat.Timestamp == allStats[i].Timestamp {
					allStats[i] = stat
					found = true
					break
				}
			}

			if found == false {
				allStats = append(allStats, stat)
			}
		}

		started := 0
		finished := 0

		activity := tm.NewTable(0, 10, 5, ' ', 0)

		header := []string{"Time", "Started", "Active", "Finished", "20x", "30x", "40x", "50x", "Avg Lat.", "Max Lat.", "Min Lat.", "\n"}
		fmt.Fprintf(activity, strings.Join(header, "\t"))

		for i := len(allStats) - 1; i >= 0; i-- {
			s20x := 0
			s30x := 0
			s40x := 0
			s50x := 0

			stat := allStats[i].TotalStats

			if stat == nil {
				continue
			}

			// Limit to 10 latest records
			if (len(allStats) - i) > 30 {
				break
			}

			started += stat.Count
			finished += stat.Finished

			for k, v := range stat.Codes {
				codeType := k[0:1]

				switch codeType {
				case "2":
					s20x += v
				case "3":
					s30x += v
				case "4":
					s40x += v
				case "5":
					s50x += v
				}
			}

			humanTime := TimeInWords(time.Now().Unix() - allStats[i].Timestamp)

			fmt.Fprintf(activity, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%.2f\t%.2f\t%.2f\n", humanTime, stat.Count, stat.Count-stat.Finished, stat.Finished, s20x, s30x, s40x, s50x, stat.AvgLat, stat.MaxLat, stat.MinLat)
		}

		tm.Println(tm.Bold("Total stats"))

		totals := tm.NewTable(0, 10, 5, ' ', 0)
		fmt.Fprintf(totals, "Time\tStarted\tActive\tFinished\n")
		fmt.Fprintf(totals, "%s\t%d\t%d\t%d\n", "All", started, started-finished, finished)
		tm.Println(totals)

		tm.Println(tm.Bold("Latest activity"))
		tm.Println(activity)

		err = file.Close()

		if err != nil {
			Debug(err)
		}

		file, err = os.Open(Settings.StatFile)

		if err != nil {
			Debug(err)
		}

		tm.Flush()

		time.Sleep(time.Second)
	}

}
