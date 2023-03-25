package plugin

import (
	"errors"
	"reflect"
	"strings"
)

// ErrorStopped is the error returned when the go routines reading the input is stopped.
var ErrorStopped = errors.New("reading stopped")

// Message represents data across plugins
type Message struct {
	Meta []byte // metadata
	Data []byte // actual data
}

// Reader is an interface for input plugins
type Reader interface {
	PluginRead() (msg *Message, err error)
}

// Writer is an interface for output plugins
type Writer interface {
	PluginWrite(msg *Message) (n int, err error)
}

// Limited is an interface for plugins that support limiting
type Limited interface {
	Limited() bool
	SetLimit(float64)
}

// ReadWriter is an interface for plugins that support reading and writing
type ReadWriter interface {
	Reader
	Writer
}

// Response is a response from a plugin
type Response struct {
	Payload       []byte
	UUID          []byte
	StartedAt     int64
	RoundTripTime int64
}

// extractLimitOptions detects if plugin get called with limiter support
// Returns address and limit
func extractLimitOptions(options string) (string, string) {
	split := strings.Split(options, "|")

	if len(split) > 1 {
		return split[0], split[1]
	}

	return split[0], ""
}

// InOutPlugins struct for holding references to plugins
type InOutPlugins struct {
	Inputs  []Reader
	Outputs []Writer
	All     []interface{}
}

// RegisterPlugin automatically detects type of plugin and initialize it
//
// See this article if curious about reflect stuff below: http://blog.burntsushi.net/type-parametric-functions-golang
func (plugins *InOutPlugins) RegisterPlugin(constructor interface{}, options ...interface{}) {
	var path, limit string
	vc := reflect.ValueOf(constructor)

	// Pre-processing options to make it work with reflect
	vo := []reflect.Value{}
	for _, oi := range options {
		vo = append(vo, reflect.ValueOf(oi))
	}

	if len(vo) > 0 {
		// Removing limit options from path
		path, limit = extractLimitOptions(vo[0].String())

		// Writing value back without limiter "|" options
		vo[0] = reflect.ValueOf(path)
	}

	// Calling our constructor with list of given options
	p := vc.Call(vo)[0].Interface()

	if limit != "" {
		p = NewLimiter(p, limit)
	}

	// Some of the output can be Readers as well because return responses
	if r, ok := p.(Reader); ok {
		plugins.Inputs = append(plugins.Inputs, r)
	}

	if w, ok := p.(Writer); ok {
		plugins.Outputs = append(plugins.Outputs, w)
	}
	plugins.All = append(plugins.All, p)
}
