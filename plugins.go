package main

import (
	"fmt"
	"reflect"
	"strings"
)

// Message represents data accross plugins
type Message struct {
	Meta []byte // metadata
	Data []byte // actual data
}

// PluginReader is an interface for input plugins
type PluginReader interface {
	PluginRead() (msg *Message, err error)
}

// PluginWriter is an interface for output plugins
type PluginWriter interface {
	PluginWrite(msg *Message) (n int, err error)
}

// PluginReadWriter is an interface for plugins that support reading and writing
type PluginReadWriter interface {
	PluginReader
	PluginWriter
}

// InOutPlugins struct for holding references to plugins
type InOutPlugins struct {
	Inputs  []PluginReader
	Outputs []PluginWriter
	All     []interface{}
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

// Automatically detects type of plugin and initialize it
//
// See this article if curious about reflect stuff below: http://blog.burntsushi.net/type-parametric-functions-golang
func (plugins *InOutPlugins) registerPlugin(service string, constructor interface{}, options ...interface{}) {
	if service == "" && Settings.Service != "" {
		service = Settings.Service
	}

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
	fmt.Sprintf("%#v", vc.Call(vo)[0].Interface())
	reflect.ValueOf(plugin).Elem().FieldByName("Service").SetString(service)

	if limit != "" {
		plugin = NewLimiter(plugin, limit)
	}

	// Some of the output can be Readers as well because return responses
	if r, ok := plugin.(PluginReader); ok {
		plugins.Inputs = append(plugins.Inputs, r)
	}

	if w, ok := plugin.(PluginWriter); ok {
		plugins.Outputs = append(plugins.Outputs, w)
	}
	plugins.All = append(plugins.All, plugin)

	fmt.Println("Loaded plugin:", service, plugin)
}

// NewPlugins specify and initialize all available plugins
func NewPlugins(service string, config ServiceSettings, plugins *InOutPlugins) *InOutPlugins {
	if plugins == nil {
		plugins = new(InOutPlugins)
	}

	for _, options := range config.InputDummy {
		plugins.registerPlugin(service, NewDummyInput, options)
	}

	for range config.OutputDummy {
		plugins.registerPlugin(service, NewDummyOutput)
	}

	if config.OutputStdout {
		plugins.registerPlugin(service, NewDummyOutput)
	}

	if config.OutputNull {
		plugins.registerPlugin(service, NewNullOutput)
	}

	for _, options := range config.InputRAW {
		plugins.registerPlugin(service, NewRAWInput, options, config.InputRAWConfig)
	}

	for _, options := range config.InputTCP {
		plugins.registerPlugin(service, NewTCPInput, options, &config.InputTCPConfig)
	}

	for _, options := range config.OutputTCP {
		plugins.registerPlugin(service, NewTCPOutput, options, &config.OutputTCPConfig)
	}

	for _, options := range config.InputFile {
		plugins.registerPlugin(service, NewFileInput, options, config.InputFileLoop)
	}

	for _, path := range config.OutputFile {
		if strings.HasPrefix(path, "s3://") {
			plugins.registerPlugin(service, NewS3Output, path, &config.OutputFileConfig)
		} else {
			plugins.registerPlugin(service, NewFileOutput, path, &config.OutputFileConfig)
		}
	}

	for _, options := range config.InputHTTP {
		plugins.registerPlugin(service, NewHTTPInput, options)
	}

	// If we explicitly set Host header http output should not rewrite it
	// Fix: https://github.com/buger/gor/issues/174
	for _, header := range config.ModifierConfig.Headers {
		if header.Name == "Host" {
			config.OutputHTTPConfig.OriginalHost = true
			break
		}
	}

	for _, options := range config.OutputHTTP {
		plugins.registerPlugin(service, NewHTTPOutput, options, &config.OutputHTTPConfig)
	}

	for _, options := range config.OutputBinary {
		plugins.registerPlugin(service, NewBinaryOutput, options, &config.OutputBinaryConfig)
	}

	if config.OutputKafkaConfig.Host != "" && config.OutputKafkaConfig.Topic != "" {
		plugins.registerPlugin(service, NewKafkaOutput, "", &config.OutputKafkaConfig, &config.KafkaTLSConfig)
	}

	if config.InputKafkaConfig.Host != "" && config.InputKafkaConfig.Topic != "" {
		plugins.registerPlugin(service, NewKafkaInput, "", &config.InputKafkaConfig, &config.KafkaTLSConfig)
	}

	return plugins
}
