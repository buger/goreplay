package goreplay

import (
	"fmt"
	"github.com/buger/goreplay/internal/byteutils"
	"hash/fnv"
	"io"
	"log"
	"sync"

	"github.com/coocood/freecache"
)

// Emitter represents an abject to manage plugins communication
type Emitter struct {
	sync.WaitGroup
	plugins *InOutPlugins
}

// NewEmitter creates and initializes new Emitter object.
func NewEmitter() *Emitter {
	return &Emitter{}
}

// Start initialize loop for sending data from inputs to outputs
func (e *Emitter) Start(plugins *InOutPlugins, middlewareCmd string) {
	if Settings.CopyBufferSize < 1 {
		Settings.CopyBufferSize = 5 << 20
	}
	e.plugins = plugins

	if middlewareCmd != "" {
		middleware := NewMiddleware(middlewareCmd)

		for _, in := range plugins.Inputs {
			middleware.ReadFrom(in)
		}

		e.plugins.Inputs = append(e.plugins.Inputs, middleware)
		e.plugins.All = append(e.plugins.All, middleware)
		e.Add(1)
		go func() {
			defer e.Done()
			if err := CopyMulty(middleware, plugins.Outputs...); err != nil {
				Debug(2, fmt.Sprintf("[EMITTER] error during copy: %q", err))
			}
		}()
	} else {
		for _, in := range plugins.Inputs {
			e.Add(1)
			go func(in PluginReader) {
				defer e.Done()
				if err := CopyMulty(in, plugins.Outputs...); err != nil {
					Debug(2, fmt.Sprintf("[EMITTER] error during copy: %q", err))
				}
			}(in)
		}
	}
}

// Close closes all the goroutine and waits for it to finish.
func (e *Emitter) Close() {
	for _, p := range e.plugins.All {
		if cp, ok := p.(io.Closer); ok {
			cp.Close()
		}
	}
	if len(e.plugins.All) > 0 {
		// wait for everything to stop
		e.Wait()
	}
	e.plugins.All = nil // avoid Close to make changes again
}

// CopyMulty copies from 1 reader to multiple writers
func CopyMulty(src PluginReader, writers ...PluginWriter) error {
	wIndex := 0
	modifier := NewHTTPModifier(&Settings.ModifierConfig)
	filteredRequests := freecache.NewCache(200 * 1024 * 1024) // 200M

	for {
		msg, err := src.PluginRead()
		if err != nil {
			if err == ErrorStopped || err == io.EOF {
				return nil
			}
			return err
		}
		if msg != nil && len(msg.Data) > 0 {
			if len(msg.Data) > int(Settings.CopyBufferSize) {
				msg.Data = msg.Data[:Settings.CopyBufferSize]
			}
			meta := payloadMeta(msg.Meta)
			if len(meta) < 3 {
				Debug(2, fmt.Sprintf("[EMITTER] Found malformed record %q from %q", msg.Meta, src))
				continue
			}
			requestID := meta[1]
			// start a subroutine only when necessary
			if Settings.Verbose >= 3 {
				Debug(3, "[EMITTER] input: ", byteutils.SliceToString(msg.Meta[:len(msg.Meta)-1]), " from: ", src)
			}
			if modifier != nil {
				Debug(3, "[EMITTER] modifier:", requestID, "from:", src)
				if isRequestPayload(msg.Meta) {
					msg.Data = modifier.Rewrite(msg.Data)
					// If modifier tells to skip request
					if len(msg.Data) == 0 {
						filteredRequests.Set(requestID, []byte{}, 60) //
						continue
					}
					Debug(3, "[EMITTER] Rewritten input:", requestID, "from:", src)

				} else {
					_, err := filteredRequests.Get(requestID)
					if err == nil {
						filteredRequests.Del(requestID)
						continue
					}
				}
			}

			if Settings.PrettifyHTTP {
				msg.Data = prettifyHTTP(msg.Data)
				if len(msg.Data) == 0 {
					continue
				}
			}

			if Settings.SplitOutput {
				if Settings.RecognizeTCPSessions {
					if !PRO {
						log.Fatal("Detailed TCP sessions work only with PRO license")
					}
					hasher := fnv.New32a()
					hasher.Write(meta[1])

					wIndex = int(hasher.Sum32()) % len(writers)
					if _, err := writers[wIndex].PluginWrite(msg); err != nil {
						return err
					}
				} else {
					// Simple round robin
					if _, err := writers[wIndex].PluginWrite(msg); err != nil {
						return err
					}

					wIndex = (wIndex + 1) % len(writers)
				}
			} else {
				for _, dst := range writers {
					if _, err := dst.PluginWrite(msg); err != nil && err != io.ErrClosedPipe {
						return err
					}
				}
			}
		}
	}
}
