package main

import (
	"strconv"
	"strings"
)

var dataUnitMap = map[byte]int64{
	'k': 1024,
	'm': 1024 * 1024,
	'g': 1024 * 1024 * 1024,
}

func parseDataUnit(s string) int64 {
	// Allow kb, mb, gb
	if strings.HasSuffix(s, "b") {
		s = s[:len(s)-1]
	}

	if unit, ok := dataUnitMap[s[len(s)-1]]; ok {
		size, _ := strconv.ParseInt(s[:len(s)-1], 10, 64)
		return unit * size
	} else {
		// If no unit specified use bytes
		size, _ := strconv.ParseInt(s, 10, 64)
		return size
	}
}

type unitSizeVar int64

func (u unitSizeVar) String() string {
	return strconv.Itoa(int(u))
}

func (u *unitSizeVar) Set(s string) error {
	*u = unitSizeVar(parseDataUnit(s))
	return nil
}
