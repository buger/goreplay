package main

import (
	"io"
	"reflect"
	"strings"
	"time"
)

// Plugins holds all the plugin objects
var Plugins []interface{}
var Middleware []io.ReadWriter

// extractLimitOptions detects if plugin get called with limiter support
// Returns address and limit
func extractLimitOptions(options string) (string, string) {
	split := strings.Split(options, "|")

	if len(split) > 1 {
		return split[0], split[1]
	}

	return split[0], ""
}

// Automatically detects type of plugin and initialize it
//
// See this article if curious about relfect stuff below: http://blog.burntsushi.net/type-parametric-functions-golang
func registerPlugin(constructor interface{}, options ...interface{}) {
	vc := reflect.ValueOf(constructor)

	// Pre-processing options to make it work with reflect
	vo := []reflect.Value{}
	for _, oi := range options {
		vo = append(vo, reflect.ValueOf(oi))
	}

	// Removing limit options from path
	path, limit := extractLimitOptions(vo[0].String())

	// Writing value back without limiter "|" options
	vo[0] = reflect.ValueOf(path)

	// Calling our constructor with list of given options
	plugin := vc.Call(vo)[0].Interface()
	pluginWrapper := plugin

	if limit != "" {
		pluginWrapper = NewLimiter(plugin, limit)
	} else {
		pluginWrapper = plugin
	}

	Plugins = append(Plugins, pluginWrapper)
}

// InitPlugins specify and initialize all available plugins
func InitPlugins() {
	for _, options := range Settings.inputDummy {
		registerPlugin(NewDummyInput, options)
	}

	for _, options := range Settings.outputDummy {
		registerPlugin(NewDummyOutput, options)
	}

	for _, options := range Settings.inputRAW {
		registerPlugin(NewRAWInput, options, time.Duration(0))
	}

	for _, options := range Settings.inputTCP {
		registerPlugin(NewTCPInput, options)
	}

	for _, options := range Settings.outputTCP {
		registerPlugin(NewTCPOutput, options)
	}

	for _, options := range Settings.inputFile {
		registerPlugin(NewFileInput, options)
	}

	for _, options := range Settings.outputFile {
		registerPlugin(NewFileOutput, options)
	}

	for _, options := range Settings.inputHTTP {
		registerPlugin(NewHTTPInput, options)
	}

	// If we explicitly set Host header http output should not rewrite it
	// Fix: https://github.com/buger/gor/issues/174
	for _, header := range Settings.modifierConfig.headers {
		if header.Name == "Host" {
			Settings.outputHTTPConfig.OriginalHost = true
			break
		}
	}

	for _, options := range Settings.outputHTTP {
		registerPlugin(NewHTTPOutput, options, &Settings.outputHTTPConfig)
	}

	if !Settings.modifierConfig.IsEmpty() {
		Middleware = append(Middleware, NewHTTPModifier(&Settings.modifierConfig))
	}

	for _, cmd := range Settings.middleware {
		Middleware = append(Middleware, NewExternalMiddleware(cmd))
	}
}

func testPlugins(plugins ...interface{}) {
	resetPlugins()

	Plugins = plugins
}

func resetPlugins() {
	for _, plugin := range Plugins {
		if c, isC := plugin.(io.Closer); isC {
			c.Close()
		}
	}

	Plugins = make([]interface{}, 0)
}
