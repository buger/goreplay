package emitter

import (
	"hash/fnv"
	"io"
	"sync"

	"github.com/buger/goreplay/internal/byteutils"
	"github.com/buger/goreplay/internal/size"
	"github.com/buger/goreplay/pkg/http_modifier"
	"github.com/buger/goreplay/pkg/middleware"
	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/prettify"
	"github.com/buger/goreplay/pkg/pro"
	"github.com/buger/goreplay/pkg/proto"

	"github.com/coocood/freecache"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var logger = log.With().Str("component", "emitter").Logger()

// Emitter represents an abject to manage plugins communication
type Emitter struct {
	sync.WaitGroup
	plugins *plugin.InOutPlugins
	config  *Config
}

// Config represents configuration for Emitter
type Config struct {
	CopyBufferSize       size.Size `json:"copy-buffer-size"`
	Middleware           string    `json:"middleware"`
	ModifierConfig       http_modifier.HTTPModifierConfig
	PrettifyHTTP         bool `json:"prettify-http"`
	SplitOutput          bool `json:"split-output"`
	RecognizeTCPSessions bool `json:"recognize-tcp-sessions"`
}

// New creates and initializes new Emitter object.
func New(configs ...*Config) *Emitter {
	config := &Config{}
	if len(configs) > 0 {
		config = configs[0]
	}

	return &Emitter{
		config: config,
	}
}

// Start initialize loop for sending data from inputs to outputs
func (e *Emitter) Start(plugins *plugin.InOutPlugins) {
	if e.config.CopyBufferSize < 1 {
		e.config.CopyBufferSize = 5 << 20
	}
	e.plugins = plugins

	if e.config.Middleware != "" {
		middleware := middleware.NewMiddleware(e.config.Middleware, nil)

		for _, in := range plugins.Inputs {
			middleware.ReadFrom(in)
		}

		e.plugins.Inputs = append(e.plugins.Inputs, middleware)
		e.plugins.All = append(e.plugins.All, middleware)
		e.Add(1)
		go func() {
			defer e.Done()
			if err := e.CopyMulty(middleware, plugins.Outputs...); err != nil {
				logger.Error().Err(err).Msg("error during copy")
			}
		}()
	} else {
		for _, in := range plugins.Inputs {
			e.Add(1)
			go func(in plugin.Reader) {
				defer e.Done()
				if err := e.CopyMulty(in, plugins.Outputs...); err != nil {
					logger.Error().Err(err).Msg("error during copy")
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
func (e *Emitter) CopyMulty(src plugin.Reader, writers ...plugin.Writer) error {
	wIndex := 0
	modifier := http_modifier.NewHTTPModifier(&e.config.ModifierConfig)
	filteredRequests := freecache.NewCache(200 * 1024 * 1024) // 200M

	for {
		msg, err := src.PluginRead()
		if err != nil {
			if err == plugin.ErrorStopped || err == io.EOF {
				return nil
			}
			return err
		}
		if msg != nil && len(msg.Data) > 0 {
			if len(msg.Data) > int(e.config.CopyBufferSize) {
				msg.Data = msg.Data[:e.config.CopyBufferSize]
			}
			meta := proto.PayloadMeta(msg.Meta)
			if len(meta) < 3 {
				logger.Warn().Msgf("Found malformed record %q from %q", msg.Meta, src)
				continue
			}
			requestID := meta[1]
			// start a subroutine only when necessary
			if log.Logger.GetLevel() == zerolog.DebugLevel {
				logger.Debug().Msgf("input: %s from: %s", byteutils.SliceToString(msg.Meta[:len(msg.Meta)-1]), src)
			}
			if modifier != nil {
				logger.Debug().Msgf("modifier: %s from: %s", requestID, src)
				if proto.IsRequestPayload(msg.Meta) {
					msg.Data = modifier.Rewrite(msg.Data)
					// If modifier tells to skip request
					if len(msg.Data) == 0 {
						filteredRequests.Set(requestID, []byte{}, 60) //
						continue
					}
					logger.Debug().Msgf("Rewritten input: %s from: %s", requestID, src)
				} else {
					_, err := filteredRequests.Get(requestID)
					if err == nil {
						filteredRequests.Del(requestID)
						continue
					}
				}
			}

			if e.config.PrettifyHTTP {
				msg.Data = prettify.PrettifyHTTP(msg.Data)
				if len(msg.Data) == 0 {
					continue
				}
			}

			if e.config.SplitOutput {
				if e.config.RecognizeTCPSessions {
					if !pro.PRO {
						logger.Fatal().Msg("Detailed TCP sessions work only with PRO license")
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
