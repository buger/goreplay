package plugin

import (
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// Limiter is a wrapper for input or output plugin which adds rate limiting
type Limiter struct {
	Plugin    interface{}
	Limit     int
	IsPercent bool

	currentRPS  int
	currentTime int64
}

func parseLimitOptions(options string) (limit int, isPercent bool) {
	if n := strings.Index(options, "%"); n > 0 {
		limit, _ = strconv.Atoi(options[:n])
		isPercent = true
	} else {
		limit, _ = strconv.Atoi(options)
		isPercent = false
	}

	return
}

// NewLimiter constructor for Limiter, accepts plugin and options
// `options` allow to sprcify relatve or absolute limiting
func NewLimiter(plugin interface{}, options string) ReadWriter {
	l := new(Limiter)
	l.Limit, l.IsPercent = parseLimitOptions(options)
	l.Plugin = plugin
	l.currentTime = time.Now().UnixNano()

	// FileInput have its own rate limiting. Unlike other inputs we not just dropping requests, we can slow down or speed up request emittion.
	if fi, ok := l.Plugin.(Limited); ok && l.IsPercent {
		fi.SetLimit(float64(l.Limit) / float64(100))
	}

	return l
}

func (l *Limiter) isLimited() bool {
	// File input have its own limiting algorithm
	if _, ok := l.Plugin.(Limited); ok && l.IsPercent {
		return false
	}

	if l.IsPercent {
		return l.Limit <= rand.Intn(100)
	}

	if (time.Now().UnixNano() - l.currentTime) > time.Second.Nanoseconds() {
		l.currentTime = time.Now().UnixNano()
		l.currentRPS = 0
	}

	if l.currentRPS >= l.Limit {
		return true
	}

	l.currentRPS++

	return false
}

// PluginWrite writes message to this plugin
func (l *Limiter) PluginWrite(msg *Message) (n int, err error) {
	if l.isLimited() {
		return 0, nil
	}
	if w, ok := l.Plugin.(Writer); ok {
		return w.PluginWrite(msg)
	}
	// avoid further writing
	return 0, io.ErrClosedPipe
}

// PluginRead reads message from this plugin
func (l *Limiter) PluginRead() (msg *Message, err error) {
	if r, ok := l.Plugin.(Reader); ok {
		msg, err = r.PluginRead()
	} else {
		// avoid further reading
		return nil, io.ErrClosedPipe
	}

	if l.isLimited() {
		return nil, nil
	}

	return
}

func (l *Limiter) String() string {
	return fmt.Sprintf("Limiting %s to: %d (isPercent: %v)", l.Plugin, l.Limit, l.IsPercent)
}

// Close closes the resources.
func (l *Limiter) Close() error {
	if fi, ok := l.Plugin.(io.Closer); ok {
		fi.Close()
	}
	return nil
}
