package main

import (
	"bytes"
	"io"
	"time"
)

// Start initialize loop for sending data from inputs to outputs
func Start(stop chan int) {
	if Settings.middleware != "" {
		middleware := NewMiddleware(Settings.middleware)

		for _, in := range Plugins.Inputs {
			middleware.ReadFrom(in)
		}

		// We are going only to read responses, so using same ReadFrom method
		for _, out := range Plugins.Outputs {
			if r, ok := out.(io.Reader); ok {
				middleware.ReadFrom(r)
			}
		}

		go CopyMulty(middleware, Plugins.Outputs...)
	} else {
		for _, in := range Plugins.Inputs {
			go CopyMulty(in, Plugins.Outputs...)
		}

		for _, out := range Plugins.Outputs {
			if r, ok := out.(io.Reader); ok {
				go CopyMulty(r, Plugins.Outputs...)
			}
		}
	}

	for {
		select {
		case <-stop:
			finalize()
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// CopyMulty copies from 1 reader to multiple writers
func CopyMulty(src io.Reader, writers ...io.Writer) (err error) {
	buf := make([]byte, 5*1024*1024)
	wIndex := 0
	modifier := NewHTTPModifier(&Settings.modifierConfig)
	filteredRequests := make(map[string]time.Time)
	filteredRequestsLastCleanTime := time.Now()

	i := 0

	for {
		nr, er := src.Read(buf)

		if nr > 0 && len(buf) > nr {
			payload := buf[:nr]
			meta := payloadMeta(payload)
			requestID := string(meta[1])

			_maxN := nr
			if nr > 500 {
				_maxN = 500
			}

			if Settings.debug {
				Debug("[EMITTER] input:", string(payload[0:_maxN]), nr, "from:", src)
			}

			if modifier != nil {
				if isRequestPayload(payload) {
					headSize := bytes.IndexByte(payload, '\n') + 1
					body := payload[headSize:]
					originalBodyLen := len(body)
					body = modifier.Rewrite(body)

					// If modifier tells to skip request
					if len(body) == 0 {
						filteredRequests[requestID] = time.Now()
						continue
					}

					if originalBodyLen != len(body) {
						payload = append(payload[:headSize], body...)
					}

					if Settings.debug {
						Debug("[EMITTER] Rewritten input:", len(payload), "First 500 bytes:", string(payload[0:_maxN]))
					}
				} else {
					if _, ok := filteredRequests[requestID]; ok {
						delete(filteredRequests, requestID)
						continue
					}
				}
			}

			if Settings.prettifyHTTP {
				payload = prettifyHTTP(payload)
			}

			if Settings.splitOutput {
				// Simple round robin
				writers[wIndex].Write(payload)

				wIndex++

				if wIndex >= len(writers) {
					wIndex = 0
				}
			} else {
				for _, dst := range writers {
					dst.Write(payload)
				}
			}

		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}

		// Run GC on each 1000 request
		if i%1000 == 0 {
			// Clean up filtered requests for which we didn't get a response to filter
			now := time.Now()
			if now.Sub(filteredRequestsLastCleanTime) > 60*time.Second {
				for k, v := range filteredRequests {
					if now.Sub(v) > 60*time.Second {
						delete(filteredRequests, k)
					}
				}
				filteredRequestsLastCleanTime = time.Now()
			}
		}

		i++
	}

	return err
}
