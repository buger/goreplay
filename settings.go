package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"time"

	_ "github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"
)

var DEMO string

// MultiOption allows to specify multiple flags with same name and collects all values into array
type MultiOption []string

func (h *MultiOption) String() string {
	return fmt.Sprint(*h)
}

// Set gets called multiple times for each flag with same name
func (h *MultiOption) Set(value string) error {
	*h = append(*h, value)
	return nil
}

type InputRAWConfig struct {
	Engine        string        `json:"input-raw-engine" mapstructure:"input-raw-engine"`
	TrackResponse bool          `json:"input-raw-track-response" mapstructure:"input-raw-track-response"`
	RealIPHeader  string        `json:"input-raw-realip-header" mapstructure:"input-raw-realip-header"`
	Expire        time.Duration `json:"input-raw-expire" mapstructure:"input-raw-expire"`
	Protocol      string        `json:"input-raw-protocol" mapstructure:"input-raw-protocol"`
	BpfFilter     string        `json:"input-raw-bpf-filter" mapstructure:"input-raw-bpf-filter"`
	TimestampType string        `json:"input-raw-timestamp-type" mapstructure:"input-raw-timestamp-type"`

	ImmediateMode   bool `json:"input-raw-immediate-mode" mapstructure:"input-raw-immediate-mode"`
	bufferSize      int64
	OverrideSnapLen bool   `json:"input-raw-override-snaplen" mapstructure:"input-raw-override-snaplen"`
	BufferSizeFlag  string `json:"input-raw-buffer-size" mapstructure:"input-raw-buffer-size"`
}

// AppSettings is the struct of main configuration
type AppSettings struct {
	Verbose   bool          `json:"verbose" mapstructure:"verbose"`
	Debug     bool          `json:"debug" mapstructure:"debug"`
	Stats     bool          `json:"stats" mapstructure:"stats"`
	ExitAfter time.Duration `json:"exit-after" mapstructure:"exit-after"`

	SplitOutput          bool   `json:"split-output" mapstructure:"split-output"`
	RecognizeTCPSessions bool   `json:"recognize-tcp-sessions" mapstructure:"recognize-tcp-sessions"`
	Pprof                string `json:"http-pprof" mapstructure:"http-pprof"`

	InputDummy   MultiOption `json:"input-dummy"`
	OutputDummy  MultiOption `json:"output-dummy" mapstructure:"output-dummy"`
	OutputStdout bool        `json:"output-stdout" mapstructure:"output-stdout"`
	OutputNull   bool        `json:"output-null" mapstructure:"output-null"`

	InputTCP        MultiOption     `json:"input-tcp" mapstructure:"input-tcp"`
	InputTCPConfig  TCPInputConfig  `json:"input-tcp-config" mapstructure:"input-tcp-config"`
	OutputTCP       MultiOption     `json:"output-tcp" mapstructure:"output-tcp"`
	OutputTCPConfig TCPOutputConfig `json:"output-tcp-config" mapstructure:"output-tcp-config"`
	OutputTCPStats  bool            `json:"output-tcp-stats" mapstructure:"output-tcp-stats"`

	InputFile        MultiOption      `json:"input-file" mapstructure:"input-file"`
	InputFileLoop    bool             `json:"input-file-loop" mapstructure:"input-file-loop"`
	OutputFile       MultiOption      `json:"output-file" mapstructure:"output-file"`
	OutputFileConfig FileOutputConfig `json:"output-file-config" mapstructure:"output-file-config"`

	InputRAW       MultiOption    `json:"input_raw" mapstructure:"input_raw"`
	InputRAWConfig InputRAWConfig `json:"input-raw-config" mapstructure:"input-raw-config"`

	copyBufferSize int64

	OutputFileSizeFlag    string `json:"output-file-size-limit" mapstructure:"output-file-size-limit"`
	OutputFileMaxSizeFlag string `json:"output-file-max-size-limit" mapstructure:"output-file-max-size-limit"`
	CopyBufferSizeFlag    string `json:"copy-buffer-size" mapstructure:"copy-buffer-size"`

	Middleware string `json:"middleware" mapstructure:"middleware"`

	InputHTTP    MultiOption `json:"input-http" mapstructure:"input-http"`
	OutputHTTP   MultiOption `json:"output-http" mapstructure:"output-http"`
	PrettifyHTTP bool        `json:"prettify-http" mapstructure:"prettify-http"`

	OutputHTTPConfig HTTPOutputConfig `json:"output-http-config" mapstructure:"output-http-config"`

	OutputBinary       MultiOption        `json:"output-binary" mapstructure:"output-binary"`
	OutputBinaryConfig BinaryOutputConfig `json:"output-binary-config" mapstructure:"output-binary-config"`

	ModifierConfig HTTPModifierConfig `json:"modifier-config" mapstructure:"modifier-config"`

	InputKafkaConfig  InputKafkaConfig  `json:"input-kafka-config" mapstructure:"input-kafka-config"`
	OutputKafkaConfig OutputKafkaConfig `json:"output-kafka-config" mapstructure:"output-kafka-config"`

	ConfigFile          string `json:"config-file" mapstructure:"config-file"`
	ConfigServerAddress string `json:"config-server-address" mapstructure:"config-server-address"`
	RemoteConfigHost    string `json:"config-remote-host" mapstructure:"config-remote-host"`

	sync.RWMutex
}

func (a *AppSettings) verbose() bool {
	a.RLock()
	defer a.RUnlock()
	return a.Verbose
}

func (a *AppSettings) setVerbose(verbose bool) {
	a.Lock()
	defer a.Unlock()
	a.Verbose = verbose
}

func (a *AppSettings) debug() bool {
	a.RLock()
	defer a.RUnlock()
	return a.Debug
}

func (a *AppSettings) setDebug(debug bool) {
	a.Lock()
	defer a.Unlock()
	a.Debug = debug
}

func (a *AppSettings) stats() bool {
	a.RLock()
	defer a.RUnlock()
	return a.Stats
}

func (a *AppSettings) setStats(stats bool) {
	a.Lock()
	defer a.Unlock()
	a.Stats = stats
}

func (a *AppSettings) exitAfter() time.Duration {
	a.RLock()
	defer a.RUnlock()
	return a.ExitAfter
}

func (a *AppSettings) setExitAfter(exitAfter time.Duration) {
	a.Lock()
	defer a.Unlock()
	a.ExitAfter = exitAfter
}

func (a *AppSettings) splitOutput() bool {
	a.RLock()
	defer a.RUnlock()
	return a.SplitOutput
}

func (a *AppSettings) setSplitOutput(splitOutput bool) {
	a.Lock()
	defer a.Unlock()
	a.SplitOutput = splitOutput
}

func (a *AppSettings) recognizeTCPSessions() bool {
	a.RLock()
	defer a.RUnlock()
	return a.RecognizeTCPSessions
}

func (a *AppSettings) setRecognizeTCPSessions(recognizeTCPSessions bool) {
	a.Lock()
	defer a.Unlock()
	a.RecognizeTCPSessions = recognizeTCPSessions
}

func (a *AppSettings) pprof() string {
	a.RLock()
	defer a.RUnlock()
	return a.Pprof
}

func (a *AppSettings) setPprof(pprof string) {
	a.Lock()
	defer a.Unlock()
	a.Pprof = pprof
}

func (a *AppSettings) inputDummy() MultiOption {
	a.RLock()
	defer a.RUnlock()
	return a.InputDummy
}

func (a *AppSettings) setInputDummy(inputDummy MultiOption) {
	a.Lock()
	defer a.Unlock()
	a.InputDummy = inputDummy
}

func (a *AppSettings) outputDummy() MultiOption {
	a.RLock()
	defer a.RUnlock()
	return a.OutputDummy
}

func (a *AppSettings) setOutputDummy(outputDummy MultiOption) {
	a.Lock()
	defer a.Unlock()
	a.OutputDummy = outputDummy
}

func (a *AppSettings) outputStdout() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputStdout
}

func (a *AppSettings) setOutputStdout(outputStdout bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputStdout = outputStdout
}

func (a *AppSettings) outputNull() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputNull
}

func (a *AppSettings) setOutputNull(outputNull bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputNull = outputNull
}

func (a *AppSettings) inputTCP() MultiOption {
	a.RLock()
	defer a.RUnlock()
	return a.InputTCP
}

func (a *AppSettings) setInputTCP(inputTCP MultiOption) {
	a.Lock()
	defer a.Unlock()
	a.InputTCP = inputTCP
}

func (a *AppSettings) inputTCPConfig() TCPInputConfig {
	a.RLock()
	defer a.RUnlock()
	return a.InputTCPConfig
}

func (a *AppSettings) setInputTCPConfig(inputTCPConfig TCPInputConfig) {
	a.Lock()
	defer a.Unlock()
	a.InputTCPConfig = inputTCPConfig
}

func (a *AppSettings) inputTCPConfigSecure() bool {
	a.RLock()
	defer a.RUnlock()
	return a.InputTCPConfig.Secure
}

func (a *AppSettings) setInputTCPConfigSecure(secure bool) {
	a.Lock()
	defer a.Unlock()
	a.InputTCPConfig.Secure = secure
}

func (a *AppSettings) inputTCPConfigCertificatePath() string {
	a.RLock()
	defer a.RUnlock()
	return a.InputTCPConfig.CertificatePath
}

func (a *AppSettings) setInputTCPConfigCertificatePath(certificatePath string) {
	a.Lock()
	defer a.Unlock()
	a.InputTCPConfig.CertificatePath = certificatePath
}

func (a *AppSettings) inputTCPConfigKeyPath() string {
	a.RLock()
	defer a.RUnlock()
	return a.InputTCPConfig.KeyPath
}

func (a *AppSettings) setInputTCPConfigKeyPath(keyPath string) {
	a.Lock()
	defer a.Unlock()
	a.InputTCPConfig.KeyPath = keyPath
}

func (a *AppSettings) outputTCP() MultiOption {
	a.RLock()
	defer a.RUnlock()
	return a.OutputTCP
}

func (a *AppSettings) setOutputTCP(outputTCP MultiOption) {
	a.Lock()
	defer a.Unlock()
	a.OutputTCP = outputTCP
}

func (a *AppSettings) outputTCPConfig() TCPOutputConfig {
	a.RLock()
	defer a.RUnlock()
	return a.OutputTCPConfig
}

func (a *AppSettings) setOutputTCPConfig(outputTCPConfig TCPOutputConfig) {
	a.Lock()
	defer a.Unlock()
	a.OutputTCPConfig = outputTCPConfig
}

func (a *AppSettings) outputTCPConfigSecure() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputTCPConfig.Secure
}

func (a *AppSettings) setOutputTCPConfigSecure(secure bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputTCPConfig.Secure = secure
}

func (a *AppSettings) outputTCPConfigSticky() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputTCPConfig.Sticky
}

func (a *AppSettings) setOutputTCPConfigSticky(sticky bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputTCPConfig.Sticky = sticky
}

func (a *AppSettings) outputTCPStats() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputTCPStats
}

func (a *AppSettings) setOutputTCPStats(outputTCPStats bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputTCPStats = outputTCPStats
}

func (a *AppSettings) inputFile() MultiOption {
	a.RLock()
	defer a.RUnlock()
	return a.InputFile
}

func (a *AppSettings) setInputFile(inputFile MultiOption) {
	a.Lock()
	defer a.Unlock()
	a.InputFile = inputFile
}

func (a *AppSettings) inputFileLoop() bool {
	a.RLock()
	defer a.RUnlock()
	return a.InputFileLoop
}

func (a *AppSettings) setInputFileLoop(inputFileLoop bool) {
	a.Lock()
	defer a.Unlock()
	a.InputFileLoop = inputFileLoop
}

func (a *AppSettings) outputFile() MultiOption {
	a.RLock()
	defer a.RUnlock()
	return a.OutputFile
}

func (a *AppSettings) setOutputFile(outputFile MultiOption) {
	a.Lock()
	defer a.Unlock()
	a.OutputFile = outputFile
}

func (a *AppSettings) outputFileConfigFlushInterval() time.Duration {
	a.RLock()
	defer a.RUnlock()
	return a.OutputFileConfig.FlushInterval
}

func (a *AppSettings) setOutputFileConfigFlushInterval(flushInterval time.Duration) {
	a.Lock()
	defer a.Unlock()
	a.OutputFileConfig.FlushInterval = flushInterval
}

func (a *AppSettings) outputFileConfigQueueLimit() int64 {
	a.RLock()
	defer a.RUnlock()
	return a.OutputFileConfig.QueueLimit
}

func (a *AppSettings) setOutputFileConfigQueueLimit(queueLimit int64) {
	a.Lock()
	defer a.Unlock()
	a.OutputFileConfig.QueueLimit = queueLimit
}

func (a *AppSettings) outputFileConfigAppend() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputFileConfig.Append
}

func (a *AppSettings) setOutputFileConfigAppend(append bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputFileConfig.Append = append
}

func (a *AppSettings) outputFileConfigBufferPath() string {
	a.RLock()
	defer a.RUnlock()
	return a.OutputFileConfig.BufferPath
}

func (a *AppSettings) setOutputFileConfigBufferPath(bufferPath string) {
	a.Lock()
	defer a.Unlock()
	a.OutputFileConfig.BufferPath = bufferPath
}

func (a *AppSettings) inputRAW() MultiOption {
	a.RLock()
	defer a.RUnlock()
	return a.InputRAW
}

func (a *AppSettings) setInputRAW(inputRAW MultiOption) {
	a.Lock()
	defer a.Unlock()
	a.InputRAW = inputRAW
}

func (a *AppSettings) inputRAWConfigEngine() string {
	a.RLock()
	defer a.RUnlock()
	return a.InputRAWConfig.Engine
}

func (a *AppSettings) setInputRAWConfigEngine(engine string) {
	a.Lock()
	defer a.Unlock()
	a.InputRAWConfig.Engine = engine
}

func (a *AppSettings) inputRAWConfigTrackResponse() bool {
	a.RLock()
	defer a.RUnlock()
	return a.InputRAWConfig.TrackResponse
}

func (a *AppSettings) setInputRAWConfigTrackResponse(trackResponse bool) {
	a.Lock()
	defer a.Unlock()
	a.InputRAWConfig.TrackResponse = trackResponse
}

func (a *AppSettings) inputRAWConfigRealIPHeader() string {
	a.RLock()
	defer a.RUnlock()
	return a.InputRAWConfig.RealIPHeader
}

func (a *AppSettings) setInputRAWConfigRealIPHeader(realIPHeader string) {
	a.Lock()
	defer a.Unlock()
	a.InputRAWConfig.RealIPHeader = realIPHeader
}

func (a *AppSettings) inputRAWConfigExpire() time.Duration {
	a.RLock()
	defer a.RUnlock()
	return a.InputRAWConfig.Expire
}

func (a *AppSettings) setInputRAWConfigExpire(expire time.Duration) {
	a.Lock()
	defer a.Unlock()
	a.InputRAWConfig.Expire = expire
}

func (a *AppSettings) inputRAWConfigProtocol() string {
	a.RLock()
	defer a.RUnlock()
	return a.InputRAWConfig.Protocol
}

func (a *AppSettings) setInputRAWConfigProtocol(protocol string) {
	a.Lock()
	defer a.Unlock()
	a.InputRAWConfig.Protocol = protocol
}

func (a *AppSettings) inputRAWConfigBpfFilter() string {
	a.RLock()
	defer a.RUnlock()
	return a.InputRAWConfig.BpfFilter
}

func (a *AppSettings) setInputRAWConfigBpfFilter(bpfFilter string) {
	a.Lock()
	defer a.Unlock()
	a.InputRAWConfig.BpfFilter = bpfFilter
}

func (a *AppSettings) inputRAWConfigTimestampType() string {
	a.RLock()
	defer a.RUnlock()
	return a.InputRAWConfig.TimestampType
}

func (a *AppSettings) setInputRAWConfigTimestampType(timestampType string) {
	a.Lock()
	defer a.Unlock()
	a.InputRAWConfig.TimestampType = timestampType
}

func (a *AppSettings) inputRAWConfigImmediateMode() bool {
	a.RLock()
	defer a.RUnlock()
	return a.InputRAWConfig.ImmediateMode
}

func (a *AppSettings) setInputRAWConfigImmediateMode(immediateMode bool) {
	a.Lock()
	defer a.Unlock()
	a.InputRAWConfig.ImmediateMode = immediateMode
}

func (a *AppSettings) inputRAWConfigOverrideSnapLen() bool {
	a.RLock()
	defer a.RUnlock()
	return a.InputRAWConfig.OverrideSnapLen
}

func (a *AppSettings) setInputRAWConfigOverrideSnapLen(overrideSnapLen bool) {
	a.Lock()
	defer a.Unlock()
	a.InputRAWConfig.OverrideSnapLen = overrideSnapLen
}

func (a *AppSettings) inputRAWConfigBufferSizeFlag() string {
	a.RLock()
	defer a.RUnlock()
	return a.InputRAWConfig.BufferSizeFlag
}

func (a *AppSettings) setInputRAWConfigBufferSizeFlag(bufferSizeFlag string) {
	a.Lock()
	defer a.Unlock()
	a.InputRAWConfig.BufferSizeFlag = bufferSizeFlag
}

func (a *AppSettings) outputFileSizeFlag() string {
	a.RLock()
	defer a.RUnlock()
	return a.OutputFileSizeFlag
}

func (a *AppSettings) setOutputFileSizeFlag(outputFileSizeFlag string) {
	a.Lock()
	defer a.Unlock()
	a.OutputFileSizeFlag = outputFileSizeFlag
}

func (a *AppSettings) outputFileMaxSizeFlag() string {
	a.RLock()
	defer a.RUnlock()
	return a.OutputFileMaxSizeFlag
}

func (a *AppSettings) setOutputFileMaxSizeFlag(outputFileMaxSizeFlag string) {
	a.Lock()
	defer a.Unlock()
	a.OutputFileMaxSizeFlag = outputFileMaxSizeFlag
}

func (a *AppSettings) copyBufferSizeFlag() string {
	a.RLock()
	defer a.RUnlock()
	return a.CopyBufferSizeFlag
}

func (a *AppSettings) setCopyBufferSizeFlag(copyBufferSizeFlag string) {
	a.Lock()
	defer a.Unlock()
	a.CopyBufferSizeFlag = copyBufferSizeFlag
}

func (a *AppSettings) middleware() string {
	a.RLock()
	defer a.RUnlock()
	return a.Middleware
}

func (a *AppSettings) setMiddleware(middleware string) {
	a.Lock()
	defer a.Unlock()
	a.Middleware = middleware
}

func (a *AppSettings) inputHTTP() MultiOption {
	a.RLock()
	defer a.RUnlock()
	return a.InputHTTP
}

func (a *AppSettings) setInputHTTP(inputHTTP MultiOption) {
	a.Lock()
	defer a.Unlock()
	a.InputHTTP = inputHTTP
}

func (a *AppSettings) outputHTTP() MultiOption {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTP
}

func (a *AppSettings) setOutputHTTP(outputHTTP MultiOption) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTP = outputHTTP
}

func (a *AppSettings) prettifyHTTP() bool {
	a.RLock()
	defer a.RUnlock()
	return a.PrettifyHTTP
}

func (a *AppSettings) setPrettifyHTTP(prettifyHTTP bool) {
	a.Lock()
	defer a.Unlock()
	a.PrettifyHTTP = prettifyHTTP
}

func (a *AppSettings) outputHTTPConfigRedirectLimit() int {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.RedirectLimit
}

func (a *AppSettings) setOutputHTTPConfigRedirectLimit(redirectLimit int) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.RedirectLimit = redirectLimit
}

func (a *AppSettings) outputHTTPConfigStats() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.Stats
}

func (a *AppSettings) setOutputHTTPConfigStats(stats bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.Stats = stats
}

func (a *AppSettings) outputHTTPConfigWorkersMin() int {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.WorkersMin
}

func (a *AppSettings) setOutputHTTPConfigWorkersMin(workersMin int) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.WorkersMin = workersMin
}

func (a *AppSettings) outputHTTPConfigWorkersMax() int {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.WorkersMax
}

func (a *AppSettings) setOutputHTTPConfigWorkersMax(workersMax int) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.WorkersMax = workersMax
}

func (a *AppSettings) outputHTTPConfigStatsMs() int {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.StatsMs
}

func (a *AppSettings) setOutputHTTPConfigStatsMs(statsMs int) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.StatsMs = statsMs
}

func (a *AppSettings) outputHTTPConfigQueueLen() int {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.QueueLen
}

func (a *AppSettings) setOutputHTTPConfigQueueLen(queueLen int) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.QueueLen = queueLen
}

func (a *AppSettings) outputHTTPConfigElasticSearch() string {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.ElasticSearch
}

func (a *AppSettings) setOutputHTTPConfigElasticSearch(elasticSearch string) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.ElasticSearch = elasticSearch
}

func (a *AppSettings) outputHTTPConfigTimeout() time.Duration {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.Timeout
}

func (a *AppSettings) setOutputHTTPConfigTimeout(timeout time.Duration) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.Timeout = timeout
}

func (a *AppSettings) outputHTTPConfigOriginalHost() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.OriginalHost
}

func (a *AppSettings) setOutputHTTPConfigOriginalHost(host bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.OriginalHost = host
}

func (a *AppSettings) outputHTTPConfigBufferSize() int {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.BufferSize
}

func (a *AppSettings) setOutputHTTPConfigBufferSize(bufferSize int) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.BufferSize = bufferSize
}

func (a *AppSettings) outputHTTPConfigCompatibilityMode() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.CompatibilityMode
}

func (a *AppSettings) setOutputHTTPConfigCompatibilityMode(compatibilityMode bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.CompatibilityMode = compatibilityMode
}

func (a *AppSettings) outputHTTPConfigDebug() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.Debug
}

func (a *AppSettings) setOutputHTTPConfigDebug(debug bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.Debug = debug
}

func (a *AppSettings) outputHTTPConfigTrackResponses() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputHTTPConfig.TrackResponses
}

func (a *AppSettings) setOutputHTTPConfigTrackResponses(trackResponses bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputHTTPConfig.TrackResponses = trackResponses
}

func (a *AppSettings) outputBinary() MultiOption {
	a.RLock()
	defer a.RUnlock()
	return a.OutputBinary
}

func (a *AppSettings) setOutputBinary(outputBinary MultiOption) {
	a.Lock()
	defer a.Unlock()
	a.OutputBinary = outputBinary
}

func (a *AppSettings) outputBinaryConfigWorkers() int {
	a.RLock()
	defer a.RUnlock()
	return a.OutputBinaryConfig.Workers
}

func (a *AppSettings) setOutputBinaryConfigWorkers(workers int) {
	a.Lock()
	defer a.Unlock()
	a.OutputBinaryConfig.Workers = workers
}

func (a *AppSettings) outputBinaryConfigTimeout() time.Duration {
	a.RLock()
	defer a.RUnlock()
	return a.OutputBinaryConfig.Timeout
}

func (a *AppSettings) setOutputBinaryConfigTimeout(timeout time.Duration) {
	a.Lock()
	defer a.Unlock()
	a.OutputBinaryConfig.Timeout = timeout
}

func (a *AppSettings) outputBinaryConfigBufferSize() int {
	a.RLock()
	defer a.RUnlock()
	return a.OutputBinaryConfig.BufferSize
}

func (a *AppSettings) setOutputBinaryConfigBufferSize(bufferSize int) {
	a.Lock()
	defer a.Unlock()
	a.OutputBinaryConfig.BufferSize = bufferSize
}

func (a *AppSettings) outputBinaryConfigDebug() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputBinaryConfig.Debug
}

func (a *AppSettings) setOutputBinaryConfigDebug(debug bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputBinaryConfig.Debug = debug
}

func (a *AppSettings) outputBinaryConfigTrackResponses() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputBinaryConfig.TrackResponses
}

func (a *AppSettings) setOutputBinaryConfigTrackResponses(trackResponses bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputBinaryConfig.TrackResponses = trackResponses
}

func (a *AppSettings) modifierConfig() HTTPModifierConfig {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig
}

func (a *AppSettings) setModifierConfig(config HTTPModifierConfig) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig = config
}

func (a *AppSettings) modifierConfigUrlNegativeRegexp() HTTPUrlRegexp {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.UrlNegativeRegexp
}

func (a *AppSettings) setModifierConfigUrlNegativeRegexp(UrlNegativeRegexp HTTPUrlRegexp) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.UrlNegativeRegexp = UrlNegativeRegexp
}

func (a *AppSettings) modifierConfigUrlRegexp() HTTPUrlRegexp {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.UrlRegexp
}

func (a *AppSettings) setModifierConfigUrlRegexp(UrlRegexp HTTPUrlRegexp) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.UrlRegexp = UrlRegexp
}

func (a *AppSettings) modifierConfigUrlRewrite() UrlRewriteMap {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.UrlRewrite
}

func (a *AppSettings) setModifierConfigUrlRewrite(UrlRewrite UrlRewriteMap) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.UrlRewrite = UrlRewrite
}

func (a *AppSettings) modifierConfigHeaderRewrite() HeaderRewriteMap {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.HeaderRewrite
}

func (a *AppSettings) setModifierConfigHeaderRewrite(headerRewrite HeaderRewriteMap) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.HeaderRewrite = headerRewrite
}

func (a *AppSettings) modifierConfigHeaderFilters() HTTPHeaderFilters {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.HeaderFilters
}

func (a *AppSettings) setModifierConfigHeaderFilters(HeaderFilters HTTPHeaderFilters) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.HeaderFilters = HeaderFilters
}

func (a *AppSettings) modifierConfigHeaderNegativeFilters() HTTPHeaderFilters {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.HeaderNegativeFilters
}

func (a *AppSettings) setModifierConfigHeaderNegativeFilters(headerNegativeFilters HTTPHeaderFilters) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.HeaderNegativeFilters = headerNegativeFilters
}

func (a *AppSettings) modifierConfigHeaderBasicAuthFilters() HTTPHeaderBasicAuthFilters {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.HeaderBasicAuthFilters
}

func (a *AppSettings) setModifierConfigHeaderBasicAuthFilters(headerBasicAuthFilters HTTPHeaderBasicAuthFilters) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.HeaderBasicAuthFilters = headerBasicAuthFilters
}

func (a *AppSettings) modifierConfigHeaderHashFilters() HTTPHashFilters {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.HeaderHashFilters
}

func (a *AppSettings) setModifierConfigHeaderHashFilters(headerHashFilters HTTPHashFilters) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.HeaderHashFilters = headerHashFilters
}

func (a *AppSettings) modifierConfigParamHashFilters() HTTPHashFilters {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.ParamHashFilters
}

func (a *AppSettings) setModifierConfigParamHashFilters(paramHashFilters HTTPHashFilters) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.ParamHashFilters = paramHashFilters
}

func (a *AppSettings) modifierConfigParams() HTTPParams {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.Params
}

func (a *AppSettings) setModifierConfigParams(params HTTPParams) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.Params = params
}

func (a *AppSettings) modifierConfigHeaders() HTTPHeaders {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.Headers
}

func (a *AppSettings) setModifierConfigHeaders(headers HTTPHeaders) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.Headers = headers
}

func (a *AppSettings) modifierConfigMethods() HTTPMethods {
	a.RLock()
	defer a.RUnlock()
	return a.ModifierConfig.Methods
}

func (a *AppSettings) setModifierConfigMethods(methods HTTPMethods) {
	a.Lock()
	defer a.Unlock()
	a.ModifierConfig.Methods = methods
}

func (a *AppSettings) inputKafkaConfigHost() string {
	a.RLock()
	defer a.RUnlock()
	return a.InputKafkaConfig.Host
}

func (a *AppSettings) setInputKafkaConfigHost(host string) {
	a.Lock()
	defer a.Unlock()
	a.InputKafkaConfig.Host = host
}

func (a *AppSettings) inputKafkaConfigTopic() string {
	a.RLock()
	defer a.RUnlock()
	return a.InputKafkaConfig.Topic
}

func (a *AppSettings) setInputKafkaConfigTopic(topic string) {
	a.Lock()
	defer a.Unlock()
	a.InputKafkaConfig.Topic = topic
}

func (a *AppSettings) inputKafkaConfigUseJSON() bool {
	a.RLock()
	defer a.RUnlock()
	return a.InputKafkaConfig.UseJSON
}

func (a *AppSettings) setInputKafkaConfigUseJSON(configJSON bool) {
	a.Lock()
	defer a.Unlock()
	a.InputKafkaConfig.UseJSON = configJSON
}

func (a *AppSettings) outputKafkaConfigHost() string {
	a.RLock()
	defer a.RUnlock()
	return a.OutputKafkaConfig.Host
}

func (a *AppSettings) setOutputKafkaConfigHost(host string) {
	a.Lock()
	defer a.Unlock()
	a.OutputKafkaConfig.Host = host
}

func (a *AppSettings) outputKafkaConfigTopic() string {
	a.RLock()
	defer a.RUnlock()
	return a.OutputKafkaConfig.Topic
}

func (a *AppSettings) setOutputKafkaConfigTopic(topic string) {
	a.Lock()
	defer a.Unlock()
	a.OutputKafkaConfig.Topic = topic
}

func (a *AppSettings) outputKafkaConfigUseJSON() bool {
	a.RLock()
	defer a.RUnlock()
	return a.OutputKafkaConfig.UseJSON
}

func (a *AppSettings) setOutputKafkaConfigUseJSON(outputKafkaConfigUseJSON bool) {
	a.Lock()
	defer a.Unlock()
	a.OutputKafkaConfig.UseJSON = outputKafkaConfigUseJSON
}

func (a *AppSettings) configFile() string {
	a.RLock()
	defer a.RUnlock()
	return a.ConfigFile
}

func (a *AppSettings) setConfigFile(configFile string) {
	a.Lock()
	defer a.Unlock()
	a.ConfigFile = configFile
}

func (a *AppSettings) configServerAddress() string {
	a.RLock()
	defer a.RUnlock()
	return a.ConfigServerAddress
}

func (a *AppSettings) setConfigServerAddress(configServerAddress string) {
	a.Lock()
	defer a.Unlock()
	a.ConfigServerAddress = configServerAddress
}

func (a *AppSettings) remoteConfigHost() string {
	a.RLock()
	defer a.RUnlock()
	return a.RemoteConfigHost
}

func (a *AppSettings) setRemoteConfigHost(remoteConfigHost string) {
	a.Lock()
	defer a.Unlock()
	a.RemoteConfigHost = remoteConfigHost
}

var nestedPathMap = make(map[string]string)

func mapForAppSettings() {

	nestedPathMap["input-tcp-secure"] = "input-tcp-config.input-tcp-secure"
	nestedPathMap["input-tcp-certificate"] = "input-tcp-config.input-tcp-certificate"
	nestedPathMap["input-tcp-certificate-key"] = "input-tcp-config.input-tcp-certificate-key"

	nestedPathMap["output-tcp-secure"] = "output-tcp-config.output-tcp-secure"
	nestedPathMap["output-tcp-sticky"] = "output-tcp-config.output-tcp-sticky"

	nestedPathMap["output-tcp-secure"] = "output-tcp-config.output-tcp-secure"
	nestedPathMap["output-tcp-sticky"] = "output-tcp-config.output-tcp-sticky"

	nestedPathMap["output-file-flush-interval"] = "output-file-config.output-file-flush-interval"
	nestedPathMap["output-file-queue-limit"] = "output-file-config.output-file-queue-limit"
	nestedPathMap["output-file-append"] = "output-file-config.output-file-append"
	nestedPathMap["output-file-buffer"] = "output-file-config.output-file-buffer"

	nestedPathMap["input-raw-engine"] = "input-raw-config.input-raw-engine"
	nestedPathMap["input-raw-track-response"] = "input-raw-config.input-raw-track-response"
	nestedPathMap["input-raw-realip-header"] = "input-raw-config.input-raw-realip-header"
	nestedPathMap["input-raw-expire"] = "input-raw-config.input-raw-expire"
	nestedPathMap["input-raw-protocol"] = "input-raw-config.input-raw-protocol"
	nestedPathMap["input-raw-bpf-filter"] = "input-raw-config.input-raw-bpf-filter"
	nestedPathMap["input-raw-timestamp-type"] = "input-raw-config.input-raw-timestamp-type"
	nestedPathMap["input-raw-immediate-mode"] = "input-raw-config.input-raw-immediate-mode"
	nestedPathMap["input-raw-override-snaplen"] = "input-raw-config.input-raw-override-snaplen"
	nestedPathMap["input-raw-buffer-size"] = "input-raw-config.input-raw-buffer-size"

	nestedPathMap["output-http-redirect-limit"] = "output-http-config.output-http-redirect-limit"
	nestedPathMap["output-http-stats"] = "output-http-config.output-http-stats"
	nestedPathMap["output-http-workers-min"] = "output-http-config.output-http-workers-min"
	nestedPathMap["output-http-workers"] = "output-http-config.output-http-workers"
	nestedPathMap["output-http-stats-ms"] = "output-http-config.output-http-stats-ms"
	nestedPathMap["output-http-queue-len"] = "output-http-config.output-http-queue-len"
	nestedPathMap["output-http-elasticsearch"] = "output-http-config.output-http-elasticsearch"
	nestedPathMap["output-http-timeout"] = "output-http-config.output-http-timeout"
	nestedPathMap["output-http-original-host"] = "output-http-config.output-http-original-host"
	nestedPathMap["output-http-response-buffer"] = "output-http-config.output-http-response-buffer"
	nestedPathMap["output-http-compatibility-mode"] = "output-http-config.output-http-compatibility-mode"
	nestedPathMap["output-http-debug"] = "output-http-config.output-http-debug"
	nestedPathMap["output-http-track-response"] = "output-http-config.output-http-track-response"

	nestedPathMap["output-binary-workers"] = "output-binary-config.output-binary-workers"
	nestedPathMap["output-binary-timeout"] = "output-binary-config.output-binary-timeout"
	nestedPathMap["output-tcp-response-buffer"] = "output-binary-config.output-tcp-response-buffer"
	nestedPathMap["output-binary-debug"] = "output-binary-config.output-binary-debug"
	nestedPathMap["output-binary-track-response"] = "output-binary-config.output-binary-track-response"

	nestedPathMap["http-disallow-url"] = "modifier-config.http-disallow-url"
	nestedPathMap["http-allow-url"] = "modifier-config.http-allow-url"
	nestedPathMap["http-rewrite-url"] = "modifier-config.http-rewrite-url"
	nestedPathMap["http-rewrite-header"] = "modifier-config.http-rewrite-header"

	nestedPathMap["http-allow-header"] = "modifier-config.http-allow-header"
	nestedPathMap["http-disallow-header"] = "modifier-config.http-disallow-header"
	nestedPathMap["http-basic-auth-filter"] = "modifier-config.http-basic-auth-filter"
	nestedPathMap["http-header-limiter"] = "modifier-config.http-header-limiter"

	nestedPathMap["http-param-limiter"] = "modifier-config.http-param-limiter"
	nestedPathMap["http-set-param"] = "modifier-config.http-set-param"
	nestedPathMap["http-set-header"] = "modifier-config.http-set-header"
	nestedPathMap["http-allow-method"] = "modifier-config.http-allow-method"

	nestedPathMap["input-kafka-host"] = "input-kafka-config.input-kafka-host"
	nestedPathMap["input-kafka-topic"] = "input-kafka-config.input-kafka-topic"
	nestedPathMap["input-kafka-json-format"] = "input-kafka-config.input-kafka-json-format"

	nestedPathMap["output-kafka-host"] = "output-kafka-config.output-kafka-host"
	nestedPathMap["output-kafka-topic"] = "output-kafka-config.output-kafka-topic"
	nestedPathMap["output-kafka-json-format"] = "output-kafka-config.output-kafka-json-format"

}

// Settings holds Gor configuration
var Settings AppSettings

func usage() {
	fmt.Printf("Gor is a simple http traffic replication tool written in Go. Its main goal is to replay traffic from production servers to staging and dev environments.\nProject page: https://github.com/buger/gor\nAuthor: <Leonid Bugaev> leonsbox@gmail.com\nCurrent Version: v%s\n\n", VERSION)
	flag.PrintDefaults()
	os.Exit(2)
}

func readAndUpdateConfig(body io.ReadCloser) error {
	decoder := json.NewDecoder(body)

	var t AppSettings
	err := decoder.Decode(&t)
	if err != nil {
		return err
	}

	Settings = t
	newConfig, err := json.Marshal(Settings)
	if err != nil {
		return err
	}
	err = viper.ReadConfig(bytes.NewBuffer(newConfig))
	if err != nil {
		return err
	}
	return nil
}

func updateConfig(respBody []byte) {
	var t AppSettings
	if err := json.Unmarshal(respBody, &t); err != nil {
		return
	}
	Settings.Lock()
	Settings = t //W
	newConfig, err := json.Marshal(Settings)
	Settings.Unlock()
	if err != nil {
		return
	}
	err = viper.ReadConfig(bytes.NewBuffer(newConfig))
	if err != nil {
		return
	}
}

func flagz(res http.ResponseWriter, req *http.Request) {

	for k, v := range req.URL.Query() {
		if len(v) == 1 {
			value, ok := nestedPathMap[k]
			if ok {
				viper.Set(value, v[0])
			} else {
				viper.Set(k, v[0])
			}
			Settings.Lock()
			viper.Unmarshal(&Settings)
			Settings.Unlock()
		}
	}

	if req.Method == "POST" {
		res.Write([]byte(readAndUpdateConfig(req.Body).Error()))
	}

	for k, v := range req.URL.Query() {
		if len(v) == 1 {
			value, ok := nestedPathMap[k]
			if ok {
				viper.Set(value, v[0])
			} else {
				viper.Set(k, v[0])
			}
			Settings.Lock()
			viper.Unmarshal(&Settings)
			Settings.Unlock()
		}
	}

	if req.Method == "POST" {
		res.Write([]byte(readAndUpdateConfig(req.Body).Error()))
	}
	Settings.RLock()
	data, _ := json.MarshalIndent(Settings, "", " ")
	Settings.RUnlock()
	res.Write(data)

}

func initConfigServer() {
	http.HandleFunc("/flagz", flagz)
	http.ListenAndServe(Settings.ConfigServerAddress, nil)
}

func init() {
	flag.Usage = usage

	flag.StringVar(&Settings.Pprof, "http-pprof", "", "Enable profiling. Starts  http server on specified port, exposing special /debug/pprof endpoint. Example: `:8181`")
	flag.BoolVar(&Settings.Verbose, "verbose", false, "Turn on more verbose output")
	flag.BoolVar(&Settings.Debug, "debug", false, "Turn on debug output, shows all intercepted traffic. Works only when with `verbose` flag")
	flag.BoolVar(&Settings.Stats, "stats", false, "Turn on queue stats output")

	if DEMO == "" {
		flag.DurationVar(&Settings.ExitAfter, "exit-after", 0, "exit after specified duration")
	} else {
		Settings.ExitAfter = 5 * time.Minute
	}

	flag.BoolVar(&Settings.SplitOutput, "split-output", false, "By default each output gets same traffic. If set to `true` it splits traffic equally among all outputs.")

	flag.BoolVar(&Settings.RecognizeTCPSessions, "recognize-tcp-sessions", false, "[PRO] If turned on http output will create separate worker for each TCP session. Splitting output will session based as well.")

	flag.Var(&Settings.InputDummy, "input-dummy", "Used for testing outputs. Emits 'Get /' request every 1s")
	flag.Var(&Settings.OutputDummy, "output-dummy", "DEPRECATED: use --output-stdout instead")

	flag.BoolVar(&Settings.OutputStdout, "output-stdout", false, "Used for testing inputs. Just prints to console data coming from inputs.")

	flag.BoolVar(&Settings.OutputNull, "output-null", false, "Used for testing inputs. Drops all requests.")

	flag.Var(&Settings.InputTCP, "input-tcp", "Used for internal communication between Gor instances. Example: \n\t# Receive requests from other Gor instances on 28020 port, and redirect output to staging\n\tgor --input-tcp :28020 --output-http staging.com")
	flag.BoolVar(&Settings.InputTCPConfig.Secure, "input-tcp-secure", false, "Turn on TLS security. Do not forget to specify certificate and key files.")
	flag.StringVar(&Settings.InputTCPConfig.CertificatePath, "input-tcp-certificate", "", "Path to PEM encoded certificate file. Used when TLS turned on.")
	flag.StringVar(&Settings.InputTCPConfig.KeyPath, "input-tcp-certificate-key", "", "Path to PEM encoded certificate key file. Used when TLS turned on.")

	flag.Var(&Settings.OutputTCP, "output-tcp", "Used for internal communication between Gor instances. Example: \n\t# Listen for requests on 80 port and forward them to other Gor instance on 28020 port\n\tgor --input-raw :80 --output-tcp replay.local:28020")
	flag.BoolVar(&Settings.OutputTCPConfig.Secure, "output-tcp-secure", false, "Use TLS secure connection. --input-file on another end should have TLS turned on as well.")
	flag.BoolVar(&Settings.OutputTCPConfig.Sticky, "output-tcp-sticky", false, "Use Sticky connection. Request/Response with same ID will be sent to the same connection.")
	flag.BoolVar(&Settings.OutputTCPStats, "output-tcp-stats", false, "Report TCP output queue stats to console every 5 seconds.")

	flag.Var(&Settings.InputFile, "input-file", "Read requests from file: \n\tgor --input-file ./requests.gor --output-http staging.com")
	flag.BoolVar(&Settings.InputFileLoop, "input-file-loop", false, "Loop input files, useful for performance testing.")

	flag.Var(&Settings.OutputFile, "output-file", "Write incoming requests to file: \n\tgor --input-raw :80 --output-file ./requests.gor")
	flag.DurationVar(&Settings.OutputFileConfig.FlushInterval, "output-file-flush-interval", time.Second, "Interval for forcing buffer flush to the file, default: 1s.")
	flag.BoolVar(&Settings.OutputFileConfig.Append, "output-file-append", false, "The flushed chunk is appended to existence file or not. ")
	flag.StringVar(&Settings.OutputFileSizeFlag, "output-file-size-limit", "32mb", "Size of each chunk. Default: 32mb")
	flag.Int64Var(&Settings.OutputFileConfig.QueueLimit, "output-file-queue-limit", 256, "The length of the chunk queue. Default: 256")
	flag.StringVar(&Settings.OutputFileMaxSizeFlag, "output-file-max-size-limit", "1TB", "Max size of output file, Default: 1TB")

	flag.StringVar(&Settings.OutputFileConfig.BufferPath, "output-file-buffer", "/tmp", "The path for temporary storing current buffer: \n\tgor --input-raw :80 --output-file s3://mybucket/logs/%Y-%m-%d.gz --output-file-buffer /mnt/logs")

	flag.BoolVar(&Settings.PrettifyHTTP, "prettify-http", false, "If enabled, will automatically decode requests and responses with: Content-Encoding: gzip and Transfer-Encoding: chunked. Useful for debugging, in conjuction with --output-stdout")

	flag.Var(&Settings.InputRAW, "input-raw", "Capture traffic from given port (use RAW sockets and require *sudo* access):\n\t# Capture traffic from 8080 port\n\tgor --input-raw :8080 --output-http staging.com")

	flag.BoolVar(&Settings.InputRAWConfig.TrackResponse, "input-raw-track-response", false, "If turned on Gor will track responses in addition to requests, and they will be available to middleware and file output.")

	flag.StringVar(&Settings.InputRAWConfig.Engine, "input-raw-engine", "libpcap", "Intercept traffic using `libpcap` (default), and `raw_socket`")

	flag.StringVar(&Settings.InputRAWConfig.Protocol, "input-raw-protocol", "http", "Specify application protocol of intercepted traffic. Possible values: http, binary")

	flag.StringVar(&Settings.InputRAWConfig.RealIPHeader, "input-raw-realip-header", "", "If not blank, injects header with given name and real IP value to the request payload. Usually this header should be named: X-Real-IP")

	flag.DurationVar(&Settings.InputRAWConfig.Expire, "input-raw-expire", time.Second*2, "How much it should wait for the last TCP packet, till consider that TCP message complete.")

	flag.StringVar(&Settings.InputRAWConfig.BpfFilter, "input-raw-bpf-filter", "", "BPF filter to write custom expressions. Can be useful in case of non standard network interfaces like tunneling or SPAN port. Example: --input-raw-bpf-filter 'dst port 80'")

	flag.StringVar(&Settings.InputRAWConfig.TimestampType, "input-raw-timestamp-type", "", "Possible values: PCAP_TSTAMP_HOST, PCAP_TSTAMP_HOST_LOWPREC, PCAP_TSTAMP_HOST_HIPREC, PCAP_TSTAMP_ADAPTER, PCAP_TSTAMP_ADAPTER_UNSYNCED. This values not supported on all systems, GoReplay will tell you available values of you put wrong one.")
	flag.StringVar(&Settings.CopyBufferSizeFlag, "copy-buffer-size", "5mb", "Set the buffer size for an individual request (default 5MB)")
	flag.BoolVar(&Settings.InputRAWConfig.OverrideSnapLen, "input-raw-override-snaplen", false, "Override the capture snaplen to be 64k. Required for some Virtualized environments")
	flag.BoolVar(&Settings.InputRAWConfig.ImmediateMode, "input-raw-immediate-mode", false, "Set pcap interface to immediate mode.")
	flag.StringVar(&Settings.InputRAWConfig.BufferSizeFlag, "input-raw-buffer-size", "0", "Controls size of the OS buffer which holds packets until they dispatched. Default value depends by system: in Linux around 2MB. If you see big package drop, increase this value.")

	flag.StringVar(&Settings.Middleware, "middleware", "", "Used for modifying traffic using external command")

	// flag.Var(&Settings.inputHTTP, "input-http", "Read requests from HTTP, should be explicitly sent from your application:\n\t# Listen for http on 9000\n\tgor --input-http :9000 --output-http staging.com")

	flag.Var(&Settings.OutputHTTP, "output-http", "Forwards incoming requests to given http address.\n\t# Redirect all incoming requests to staging.com address \n\tgor --input-raw :80 --output-http http://staging.com")

	/* outputHTTPConfig */
	flag.IntVar(&Settings.OutputHTTPConfig.BufferSize, "output-http-response-buffer", 0, "HTTP response buffer size, all data after this size will be discarded.")
	flag.BoolVar(&Settings.OutputHTTPConfig.CompatibilityMode, "output-http-compatibility-mode", false, "Use standard Go client, instead of built-in implementation. Can be slower, but more compatible.")

	flag.IntVar(&Settings.OutputHTTPConfig.WorkersMin, "output-http-workers-min", 0, "Gor uses dynamic worker scaling. Enter a number to set a minimum number of workers. default = 1.")
	flag.IntVar(&Settings.OutputHTTPConfig.WorkersMax, "output-http-workers", 0, "Gor uses dynamic worker scaling. Enter a number to set a maximum number of workers. default = 0 = unlimited.")
	flag.IntVar(&Settings.OutputHTTPConfig.QueueLen, "output-http-queue-len", 1000, "Number of requests that can be queued for output, if all workers are busy. default = 1000")

	flag.IntVar(&Settings.OutputHTTPConfig.RedirectLimit, "output-http-redirect-limit", 0, "Enable how often redirects should be followed.")
	flag.DurationVar(&Settings.OutputHTTPConfig.Timeout, "output-http-timeout", 5*time.Second, "Specify HTTP request/response timeout. By default 5s. Example: --output-http-timeout 30s")
	flag.BoolVar(&Settings.OutputHTTPConfig.TrackResponses, "output-http-track-response", false, "If turned on, HTTP output responses will be set to all outputs like stdout, file and etc.")

	flag.BoolVar(&Settings.OutputHTTPConfig.Stats, "output-http-stats", false, "Report http output queue stats to console every N milliseconds. See output-http-stats-ms")
	flag.IntVar(&Settings.OutputHTTPConfig.StatsMs, "output-http-stats-ms", 5000, "Report http output queue stats to console every N milliseconds. default: 5000")
	flag.BoolVar(&Settings.OutputHTTPConfig.OriginalHost, "output-http-original-host", false, "Normally gor replaces the Host http header with the Host supplied with --output-http.  This option disables that behavior, preserving the original Host header.")
	flag.BoolVar(&Settings.OutputHTTPConfig.Debug, "output-http-debug", false, "Enables http debug output.")
	flag.StringVar(&Settings.OutputHTTPConfig.ElasticSearch, "output-http-elasticsearch", "", "Send request and response stats to ElasticSearch:\n\tgor --input-raw :8080 --output-http staging.com --output-http-elasticsearch 'es_host:api_port/index_name'")
	/* outputHTTPConfig */

	flag.Var(&Settings.OutputBinary, "output-binary", "Forwards incoming binary payloads to given address.\n\t# Redirect all incoming requests to staging.com address \n\tgor --input-raw :80 --input-raw-protocol binary --output-binary staging.com:80")
	/* outputBinaryConfig */
	flag.IntVar(&Settings.OutputBinaryConfig.BufferSize, "output-tcp-response-buffer", 0, "TCP response buffer size, all data after this size will be discarded.")
	flag.IntVar(&Settings.OutputBinaryConfig.Workers, "output-binary-workers", 0, "Gor uses dynamic worker scaling by default.  Enter a number to run a set number of workers.")
	flag.DurationVar(&Settings.OutputBinaryConfig.Timeout, "output-binary-timeout", 0, "Specify HTTP request/response timeout. By default 5s. Example: --output-binary-timeout 30s")
	flag.BoolVar(&Settings.OutputBinaryConfig.TrackResponses, "output-binary-track-response", false, "If turned on, Binary output responses will be set to all outputs like stdout, file and etc.")

	flag.BoolVar(&Settings.OutputBinaryConfig.Debug, "output-binary-debug", false, "Enables binary debug output.")
	/* outputBinaryConfig */

	flag.StringVar(&Settings.OutputKafkaConfig.Host, "output-kafka-host", "", "Read request and response stats from Kafka:\n\tgor --input-raw :8080 --output-kafka-Host '192.168.0.1:9092,192.168.0.2:9092'")
	flag.StringVar(&Settings.OutputKafkaConfig.Topic, "output-kafka-topic", "", "Read request and response stats from Kafka:\n\tgor --input-raw :8080 --output-kafka-Topic 'kafka-log'")
	flag.BoolVar(&Settings.OutputKafkaConfig.UseJSON, "output-kafka-json-format", false, "If turned on, it will serialize messages from GoReplay text format to JSON.")

	flag.StringVar(&Settings.InputKafkaConfig.Host, "input-kafka-host", "", "Send request and response stats to Kafka:\n\tgor --output-stdout --input-kafka-Host '192.168.0.1:9092,192.168.0.2:9092'")
	flag.StringVar(&Settings.InputKafkaConfig.Topic, "input-kafka-topic", "", "Send request and response stats to Kafka:\n\tgor --output-stdout --input-kafka-Topic 'kafka-log'")
	flag.BoolVar(&Settings.InputKafkaConfig.UseJSON, "input-kafka-json-format", false, "If turned on, it will assume that messages coming in JSON format rather than  GoReplay text format.")

	flag.Var(&Settings.ModifierConfig.Headers, "http-set-header", "Inject additional headers to http reqest:\n\tgor --input-raw :8080 --output-http staging.com --http-set-header 'User-Agent: Gor'")
	flag.Var(&Settings.ModifierConfig.Headers, "output-http-header", "WARNING: `--output-http-header` DEPRECATED, use `--http-set-header` instead")

	flag.Var(&Settings.ModifierConfig.HeaderRewrite, "http-rewrite-header", "Rewrite the request header based on a mapping:\n\tgor --input-raw :8080 --output-http staging.com --http-rewrite-header Host: (.*).example.com,$1.beta.example.com")

	flag.Var(&Settings.ModifierConfig.Params, "http-set-param", "Set request url param, if param already exists it will be overwritten:\n\tgor --input-raw :8080 --output-http staging.com --http-set-param api_key=1")

	flag.Var(&Settings.ModifierConfig.Methods, "http-allow-method", "Whitelist of HTTP methods to replay. Anything else will be dropped:\n\tgor --input-raw :8080 --output-http staging.com --http-allow-method GET --http-allow-method OPTIONS")
	flag.Var(&Settings.ModifierConfig.Methods, "output-http-method", "WARNING: `--output-http-method` DEPRECATED, use `--http-allow-method` instead")

	flag.Var(&Settings.ModifierConfig.UrlRegexp, "http-allow-url", "A regexp to match requests against. Filter get matched against full url with domain. Anything else will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-allow-url ^www.")
	flag.Var(&Settings.ModifierConfig.UrlRegexp, "output-http-url-regexp", "WARNING: `--output-http-url-regexp` DEPRECATED, use `--http-allow-url` instead")

	flag.Var(&Settings.ModifierConfig.UrlNegativeRegexp, "http-disallow-url", "A regexp to match requests against. Filter get matched against full url with domain. Anything else will be forwarded:\n\t gor --input-raw :8080 --output-http staging.com --http-disallow-url ^www.")

	flag.Var(&Settings.ModifierConfig.UrlRewrite, "http-rewrite-url", "Rewrite the request url based on a mapping:\n\tgor --input-raw :8080 --output-http staging.com --http-rewrite-url /v1/user/([^\\/]+)/ping:/v2/user/$1/ping")
	flag.Var(&Settings.ModifierConfig.UrlRewrite, "output-http-rewrite-url", "WARNING: `--output-http-rewrite-url` DEPRECATED, use `--http-rewrite-url` instead")

	flag.Var(&Settings.ModifierConfig.HeaderFilters, "http-allow-header", "A regexp to match a specific header against. Requests with non-matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-allow-header api-version:^v1")
	flag.Var(&Settings.ModifierConfig.HeaderFilters, "output-http-header-filter", "WARNING: `--output-http-header-filter` DEPRECATED, use `--http-allow-header` instead")

	flag.Var(&Settings.ModifierConfig.HeaderNegativeFilters, "http-disallow-header", "A regexp to match a specific header against. Requests with matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-disallow-header \"User-Agent: Replayed by Gor\"")

	flag.Var(&Settings.ModifierConfig.HeaderBasicAuthFilters, "http-basic-auth-filter", "A regexp to match the decoded basic auth string against. Requests with non-matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-basic-auth-filter \"^customer[0-9].*\"")

	flag.Var(&Settings.ModifierConfig.HeaderHashFilters, "http-header-limiter", "Takes a fraction of requests, consistently taking or rejecting a request based on the FNV32-1A hash of a specific header:\n\t gor --input-raw :8080 --output-http staging.com --http-header-limiter user-id:25%")

	flag.Var(&Settings.ModifierConfig.HeaderHashFilters, "output-http-header-hash-filter", "WARNING: `output-http-header-hash-filter` DEPRECATED, use `--http-header-hash-limiter` instead")

	flag.Var(&Settings.ModifierConfig.ParamHashFilters, "http-param-limiter", "Takes a fraction of requests, consistently taking or rejecting a request based on the FNV32-1A hash of a specific GET param:\n\t gor --input-raw :8080 --output-http staging.com --http-param-limiter user_id:25%")

	// default values, using for tests
	Settings.OutputFileConfig.sizeLimit = 33554432
	Settings.OutputFileConfig.outputFileMaxSize = 1099511627776
	Settings.copyBufferSize = 5242880
	Settings.InputRAWConfig.bufferSize = 0

	currrentDir, _ := os.Getwd()
	log.Printf("Locating config in folder: %s", currrentDir)

	flag.StringVar(&Settings.ConfigFile, "config-file", "config.json", "The path to Goreplay config file.")
	viper.SetConfigFile(currrentDir + "/" + Settings.ConfigFile)

	flag.StringVar(&Settings.ConfigServerAddress, "config-server-address", ":9999", "The host address for config API.")
	viper.SetConfigFile(Settings.ConfigFile)

	// config-remote-host : http://localhost:8000

	flag.StringVar(&Settings.RemoteConfigHost, "config-remote-host", "", "The host address for config API.")

	mapForAppSettings()

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	// Searches for config file in given paths and read it
	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Error reading config file, %s", err)
		return
	}
	viper.Unmarshal(&Settings)

	if Settings.remoteConfigHost() != "" {
		log.Printf("Starting to read remote config from: %s", Settings.RemoteConfigHost)
		go pollRemoteConfig()
	}

	if Settings.verbose() {
		fmt.Printf("Using config: %s\n", viper.ConfigFileUsed())
	}

	initConfigServer()

}

func pollRemoteConfig() {
	for {
		req, err := http.NewRequest("GET", Settings.RemoteConfigHost+"/config.json", nil)
		if err != nil {
			log.Printf("Error while getting config from remote server., %s", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error while getting config from remote server., %s", err)
			continue
		}
		defer resp.Body.Close()

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error while getting config from remote server., %s", err)
			continue
		}

		updateConfig(respBody)
		time.Sleep(time.Second)
	}
}

func checkSettings() {
	outputFileSize, err := bufferParser(Settings.OutputFileSizeFlag, "32MB")
	if err != nil {
		log.Fatalf("output-file-size-limit error: %v\n", err)
	}
	Settings.OutputFileConfig.sizeLimit = outputFileSize

	outputFileMaxSize, err := bufferParser(Settings.OutputFileMaxSizeFlag, "1TB")
	if err != nil {
		log.Fatalf("output-file-max-size-limit error: %v\n", err)
	}
	Settings.OutputFileConfig.outputFileMaxSize = outputFileMaxSize

	copyBufferSize, err := bufferParser(Settings.CopyBufferSizeFlag, "5mb")
	if err != nil {
		log.Fatalf("copy-buffer-size error: %v\n", err)
	}
	Settings.copyBufferSize = copyBufferSize

	inputRAWBufferSize, err := bufferParser(Settings.InputRAWConfig.BufferSizeFlag, "0")
	if err != nil {
		log.Fatalf("input-raw-buffer-size error: %v\n", err)
	}
	Settings.InputRAWConfig.bufferSize = inputRAWBufferSize

	// libpcap has bug in mac os x. More info: https://github.com/buger/goreplay/issues/730
	if Settings.InputRAWConfig.Expire == time.Second*2 && runtime.GOOS == "darwin" {
		Settings.InputRAWConfig.Expire = time.Second
	}
}

var previousDebugTime = time.Now()
var debugMutex sync.Mutex
var pID = os.Getpid()

// Debug take an effect only if --verbose flag specified
func Debug(args ...interface{}) {
	if Settings.Verbose {
		debugMutex.Lock()
		defer debugMutex.Unlock()
		now := time.Now()
		diff := now.Sub(previousDebugTime).String()
		previousDebugTime = now
		fmt.Printf("[DEBUG][PID %d][%s][elapsed %s] ", pID, now.Format(time.StampNano), diff)
		fmt.Println(args...)
	}
}

// the following regexes follow Go semantics https://golang.org/ref/spec#Letters_and_digits
var (
	rB   = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+$`)
	rKB  = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+kb$`)
	rMB  = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+mb$`)
	rGB  = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+gb$`)
	rTB  = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+tb$`)
	empt = regexp.MustCompile(`^[\n\t\r 0.\f\a]*$`)
)

// bufferParser parses buffer to bytes from different bases and data units
// size is the buffer in string, rpl act as a replacement for empty buffer.
// e.g: (--output-file-size-limit "") may override default 32mb with empty buffer,
// which can be solved by setting rpl by bufferParser(buffer, "32mb")
func bufferParser(size, rpl string) (buffer int64, err error) {
	const (
		_ = 1 << (iota * 10)
		KB
		MB
		GB
		TB
	)

	var (
		lmt = len(size) - 2
		s   = []byte(size)
	)

	if empt.Match(s) {
		size = rpl
		s = []byte(size)
	}

	// recover, especially when buffer size overflows int64 i.e ~8019PBs
	defer func() {
		if e, ok := recover().(error); ok {
			err = e.(error)
		}
	}()

	switch {
	case rB.Match(s):
		buffer, err = strconv.ParseInt(size, 0, 64)
	case rKB.Match(s):
		buffer, err = strconv.ParseInt(size[:lmt], 0, 64)
		buffer *= KB
	case rMB.Match(s):
		buffer, err = strconv.ParseInt(size[:lmt], 0, 64)
		buffer *= MB
	case rGB.Match(s):
		buffer, err = strconv.ParseInt(size[:lmt], 0, 64)
		buffer *= GB
	case rTB.Match(s):
		buffer, err = strconv.ParseInt(size[:lmt], 0, 64)
		buffer *= TB
	default:
		return 0, fmt.Errorf("invalid buffer %q", size)
	}
	return
}
