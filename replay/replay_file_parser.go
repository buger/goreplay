package replay

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"strconv"

	"fmt"
)

type ParsedRequest struct {
	Request   []byte
	Timestamp int64
}

func (self ParsedRequest) String() string {
	return fmt.Sprintf("Request: %v, timestamp: %v", string(self.Request), self.Timestamp)
}

func parseReplayFile() (requests []ParsedRequest, err error) {
	requests, err = readLines(Settings.FileToReplayPath)

	if err != nil {
		log.Fatalf("readLines: %s", err)
	}

	return
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) (requests []ParsedRequest, err error) {
	file, err := os.Open(path)

	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(scanLinesFunc)

	for scanner.Scan() {
		if len(scanner.Text()) > 5 {
			buf := append([]byte(nil), scanner.Bytes()...)
			i := bytes.IndexByte(buf, '\n')
			timestamp, _ := strconv.Atoi(string(buf[:i]))
			pr := ParsedRequest{buf[i+1:], int64(timestamp)}

			requests = append(requests, pr)
		}
	}

	return requests, scanner.Err()
}

// scanner spliting logic
func scanLinesFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	delimiter := []byte{'\r', '\n', '\r', '\n', '\n'}

	// We have a http request end: \r\n\r\n
	if i := bytes.Index(data, delimiter); i >= 0 {
		return (i + len(delimiter)), data[0:(i + len(delimiter))], nil
	}

	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}

	// Request more data.
	return 0, nil, nil
}
