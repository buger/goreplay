package main

import (
	"bytes"
	"hash/fnv"
	"io"
	"log"
	"sync"
	"time"
)

type emitter struct {
	sync.WaitGroup
	quit chan int
}

// NewEmitter creates and initializes new `emitter` object.
func NewEmitter(quit chan int) *emitter {
	return &emitter{
		quit: quit,
	}
}

// Start initialize loop for sending data from inputs to outputs
func (e *emitter) Start(plugins *InOutPlugins, middlewareCmd string) {
	e.Add(1)
	defer e.Done()

	if middlewareCmd != "" {
		middleware := NewMiddleware(middlewareCmd)

		for _, in := range plugins.Inputs {
			middleware.ReadFrom(in)
		}

		// We are going only to read responses, so using same ReadFrom method
		for _, out := range plugins.Outputs {
			if r, ok := out.(io.Reader); ok {
				middleware.ReadFrom(r)
			}
		}
		e.Add(1)
		go func() {
			defer e.Done()
			if err := CopyMulty(e.quit, middleware, plugins.Outputs...); err != nil {
				log.Println("Error during copy: ", err)
				e.close()
			}
		}()
		go func() {
			for {
				select {
				case <-e.quit:
					middleware.Close()
					return
				}
			}
		}()
	} else {
		for _, in := range plugins.Inputs {
			e.Add(1)
			go func(in io.Reader) {
				defer e.Done()
				if err := CopyMulty(e.quit, in, plugins.Outputs...); err != nil {
					log.Println("Error during copy: ", err)
					e.close()
				}
			}(in)
		}

		for _, out := range plugins.Outputs {
			if r, ok := out.(io.Reader); ok {
				e.Add(1)
				go func(r io.Reader) {
					defer e.Done()
					if err := CopyMulty(e.quit, r, plugins.Outputs...); err != nil {
						log.Println("Error during copy: ", err)
						e.close()
					}
				}(r)
			}
		}
	}

	for {
		select {
		case <-e.quit:
			finalize(plugins)
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (e *emitter) close() {
	select {
	case <-e.quit:
	default:
		close(e.quit)
	}
}

// Close closes all the goroutine and waits for it to finish.
func (e *emitter) Close() {
	e.close()
	e.Wait()
}

// CopyMulty copies from 1 reader to multiple writers
func CopyMulty(stop chan int, src io.Reader, writers ...io.Writer) error {
	buf := make([]byte, Settings.copyBufferSize)
	wIndex := 0
	modifier := NewHTTPModifier(&Settings.ModifierConfig)
	filteredRequests := make(map[string]time.Time)
	filteredRequestsLastCleanTime := time.Now()

	i := 0
	for {
		var nr int
		nr, err := src.Read(buf)

		select {
		case <-stop:
			return nil
		default:
		}

		if err == io.EOF || err == ErrorStopped {
			return nil
		}
		if err != nil {
			return err
		}

		_maxN := nr
		if nr > 500 {
			_maxN = 500
		}
		if nr > 0 && len(buf) > nr {
			payload := buf[:nr]
			meta := payloadMeta(payload)
			if len(meta) < 3 {
				if Settings.debug() {
					Debug("[EMITTER] Found malformed record", string(payload[0:_maxN]), nr, "from:", src)
				}
				continue
			}
			requestID := string(meta[1])

			if nr >= 5*1024*1024 {
				log.Println("INFO: Large packet... We received ", len(payload), " bytes from ", src)
			}

			if Settings.debug() {
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

					if Settings.debug() {
						Debug("[EMITTER] Rewritten input:", len(payload), "First 500 bytes:", string(payload[0:_maxN]))
					}
				} else {
					if _, ok := filteredRequests[requestID]; ok {
						delete(filteredRequests, requestID)
						continue
					}
				}
			}

			if Settings.prettifyHTTP() {
				payload = prettifyHTTP(payload)
				if len(payload) == 0 {
					continue
				}
			}

			if Settings.splitOutput() {
				if Settings.recognizeTCPSessions() {
					if !PRO {
						log.Fatal("Detailed TCP sessions work only with PRO license")
					}
					hasher := fnv.New32a()
					// First 20 bytes contain tcp session
					id := payloadID(payload)
					hasher.Write(id[:20])

					wIndex = int(hasher.Sum32()) % len(writers)
					writers[wIndex].Write(payload)
				} else {
					// Simple round robin
					if _, err := writers[wIndex].Write(payload); err != nil {
						return err
					}

					wIndex++

					if wIndex >= len(writers) {
						wIndex = 0
					}
				}
			} else {
				for _, dst := range writers {
					if _, err := dst.Write(payload); err != nil {
						return err
					}
				}
			}
		} else if nr > 0 {
			log.Println("WARN: Packet", nr, "bytes is too large to process. Consider increasing --copy-buffer-size")
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
}
