package logs

import (
	"fmt"
	"os"
)

/*
As stdout is used between goreplay and middleware to exchange requests/responses,
All logs should be sent to stderr.
*/

func Debug(args ...interface{}) {
	if os.Getenv("GOR_TEST") == "1" {
		fmt.Fprintln(os.Stderr, args...)
	}
}

func Info(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
}

func Error(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
}

func Fatal(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}