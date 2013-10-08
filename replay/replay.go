// Replay server receive requests objects from Listeners and forward it to given address.
// Basic usage:
//
//     gor replay -f http://staging.server
//
//
// Rate limiting
//
// It can be useful if you want forward only part of production traffic, not to overload staging environment. You can specify desired request per second using "|" operator after server address:
//
//     # staging.server not get more than 10 requests per second
//     gor replay -f "http://staging.server|10"
//
//
// Forward to multiple addresses
//
// Just separate addresses by coma:
//    gor replay -f "http://staging.server|10,http://dev.server|20"
//
//
//  For more help run:
//
//     gor replay -h
//
package replay

import (
  "bufio"
  "bytes"
  "io"
  "log"
  "net"
  "net/http"
  "time"
)


const bufSize = 4096

type ReplayManager struct {
  reqFactory *RequestFactory
}

func NewReplayManager() (rm *ReplayManager) {
  rm = &ReplayManager{}
  rm.reqFactory = NewRequestFactory()

  return
}

// Debug enables logging only if "--verbose" flag passed
func Debug(v ...interface{}) {
  if Settings.Verbose {
    log.Println(v...)
  }
}

// ParseRequest in []byte returns a http request or an error
func ParseRequest(data []byte) (request *http.Request, err error) {
  buf := bytes.NewBuffer(data)
  reader := bufio.NewReader(buf)

  request, err = http.ReadRequest(reader)

  if err != nil {
    log.Fatal("Can not parse request", string(data), err)
  }

  return
}

// Run acts as `main` function of replay
// Replay server listen to UDP traffic from Listeners
// Each request processed by RequestFactory
func Run() {
  rm := NewReplayManager()

  if Settings.FileToReplyPath != "" {
    rm.RunReplayFromFile()
  } else {
    rm.RunReplayFromNetwork()
  }
}

func (self *ReplayManager) RunReplayFromFile() {
  TotalResponsesCount = 0

  log.Println("Starting file reply")
  requests, err := parseReplyFile()

  if err != nil {
    log.Fatal("Can't parse request: ", err)
  }

  var lastTimestamp int64

  if len(requests) > 0 {
    lastTimestamp = requests[0].Timestamp
  }

  requestsToReplay := 0

	hosts := Settings.ForwardedHosts()
  for _, host := range hosts {
    if host.Limit > 0 {
      requestsToReplay += host.Limit
    } else {
      requestsToReplay += len(requests)
    }
  }

  for _, request := range requests {

    parsedReq, err := ParseRequest(request.Request)

    if err != nil {
      log.Fatal("Can't parse request...:", err)
    }

    time.Sleep(time.Duration(request.Timestamp - lastTimestamp))

    self.sendRequestToReplay(parsedReq)
    lastTimestamp = request.Timestamp
  }

  for requestsToReplay > TotalResponsesCount {
    time.Sleep(time.Second)
  }

}

func (self *ReplayManager) RunReplayFromNetwork() {
  listener, err := net.Listen("tcp", Settings.Address)

  log.Println("Starting replay server at:", Settings.Address)

  if err != nil {
    log.Fatal("Can't start:", err)
  }

  for _, host := range Settings.ForwardedHosts() {
    log.Println("Forwarding requests to:", host.Url, "limit:", host.Limit)
  }

  for {
    conn, err := listener.Accept()

    if err != nil {
      log.Println("Error while Accept()", err)
      continue
    }

    go self.handleConnection(conn)
  }
}

func (self *ReplayManager) handleConnection(conn net.Conn) error {
	defer conn.Close()

	var read = true
	var response []byte
	var buf []byte

	buf = make([]byte, bufSize)

	for read {
		n, err := conn.Read(buf)

		switch err {
		case io.EOF:
			read = false
		case nil:
			response = append(response, buf[:n]...)
			if n < bufSize {
				read = false
			}
		default:
			read = false
		}
	}

	go func() {
		if request, err := ParseRequest(response); err != nil {
			Debug("Error while parsing request", err, response)
		} else {
			Debug("Adding request", request)

      self.sendRequestToReplay(request)
		}
	}()

	return nil
}

func (self *ReplayManager) sendRequestToReplay(req *http.Request) {
  self.reqFactory.Add(req)
}
