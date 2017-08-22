package main

import (
	"io"
	"reflect"
	"strings"
	"sync"
)

// InOutPlugins struct for holding references to plugins
type InOutPlugins struct {
	Inputs  []io.Reader
	Outputs []io.Writer
	All     []interface{}
}

var pluginMu sync.Mutex

// Plugins holds all the plugin objects
var Plugins *InOutPlugins = new(InOutPlugins)

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
	plugin := vc.Call(vo)[0].Interface()
	pluginWrapper := plugin

	if limit != "" {
		pluginWrapper = NewLimiter(plugin, limit)
	} else {
		pluginWrapper = plugin
	}

	_, isR := plugin.(io.Reader)
	_, isW := plugin.(io.Writer)

	// Some of the output can be Readers as well because return responses
	if isR && !isW {
		Plugins.Inputs = append(Plugins.Inputs, pluginWrapper.(io.Reader))
	}

	if isW {
		Plugins.Outputs = append(Plugins.Outputs, pluginWrapper.(io.Writer))
	}

	Plugins.All = append(Plugins.All, plugin)
}

// InitPlugins specify and initialize all available plugins
func InitPlugins() {
	pluginMu.Lock()
	defer pluginMu.Unlock()

	for _, options := range Settings.inputDummy {
		registerPlugin(NewDummyInput, options)
	}

	for range Settings.outputDummy {
		registerPlugin(NewDummyOutput)
	}

	if Settings.outputStdout {
		registerPlugin(NewDummyOutput)
	}

	if Settings.outputNull {
		registerPlugin(NewNullOutput)
	}

	engine := EnginePcap
	if Settings.inputRAWEngine == "raw_socket" {
		engine = EngineRawSocket
	} else if Settings.inputRAWEngine == "pcap_file" {
		engine = EnginePcapFile
	}

	for _, options := range Settings.inputRAW {
		registerPlugin(NewRAWInput, options, engine, Settings.inputRAWTrackResponse, Settings.inputRAWExpire, Settings.inputRAWRealIPHeader, Settings.inputRAWBpfFilter)
	}

	for _, options := range Settings.inputTCP {
		registerPlugin(NewTCPInput, options, &Settings.inputTCPConfig)
	}

	for _, options := range Settings.outputTCP {
		registerPlugin(NewTCPOutput, options, &Settings.outputTCPConfig)
	}

	for _, options := range Settings.inputFile {
		registerPlugin(NewFileInput, options, Settings.inputFileLoop)
	}

	for _, options := range Settings.outputFile {
		registerPlugin(NewFileOutput, options, &Settings.outputFileConfig)
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

	if Settings.outputKafkaConfig.host != "" && Settings.outputKafkaConfig.topic != "" {
		registerPlugin(NewKafkaOutput, "", &Settings.outputKafkaConfig)
	}

	if Settings.inputKafkaConfig.host != "" && Settings.inputKafkaConfig.topic != "" {
		registerPlugin(NewKafkaInput, "", &Settings.inputKafkaConfig)
	}
}
