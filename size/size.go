package size

import (
	"fmt"
	"regexp"
	"strconv"
)

// Size represents size that implements flag.Var
type Size int64

// the following regexes follow Go semantics https://golang.org/ref/spec#Letters_and_digits
var (
	rB  = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+$`)
	rKB = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+kb$`)
	rMB = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+mb$`)
	rGB = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+gb$`)
	rTB = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+tb$`)
)

// Set parses size to integer from different bases and data units
func (siz *Size) Set(size string) (err error) {
	if size == "" {
		return
	}
	const (
		_ = 1 << (iota * 10)
		KB
		MB
		GB
		TB
	)

	var (
		lmt = len(size) - 2
		s   = []byte(size)
	)

	var _len int64
	switch {
	case rB.Match(s):
		_len, err = strconv.ParseInt(size, 0, 64)
	case rKB.Match(s):
		_len, err = strconv.ParseInt(size[:lmt], 0, 64)
		_len *= KB
	case rMB.Match(s):
		_len, err = strconv.ParseInt(size[:lmt], 0, 64)
		_len *= MB
	case rGB.Match(s):
		_len, err = strconv.ParseInt(size[:lmt], 0, 64)
		_len *= GB
	case rTB.Match(s):
		_len, err = strconv.ParseInt(size[:lmt], 0, 64)
		_len *= TB
	default:
		return fmt.Errorf("invalid _len %q", size)
	}
	*siz = Size(_len)
	return
}

func (siz *Size) String() string {
	return fmt.Sprintf("%d", *siz)
}
