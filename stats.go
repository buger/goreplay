package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/buger/goreplay/stat"
	"github.com/buger/goreplay/stat/csv"
)

func statsWriteRecord(st *stat.Writer, record []string, plugin string) {
	if st == nil {
		return
	}
	if st.Error() != nil {
		Debug(1, plugin, " stats closed with error: ", st.Error())
		st.Close()
		st = nil
		return
	}
	st.Write(record)
}

func initializeStatWriter(file string, hdr string) *stat.Writer {
	csvWr, err := csv.NewWriter(filepath.Join(os.Getenv("GORDIR"), file))
	if err != nil {
		log.Fatalf("init meta stat writer error: %q", err)
	}
	st := stat.NewWriter(csvWr, "meta")
	if hdr != "" {
		st.WriteHeader("type: GOR_STATS\ntitle: message meta")
	}
	return st
}
