package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// HTTPModifierConfig holds configuration options for built-in traffic modifier
type HTTPModifierConfig struct {
	urlNegativeRegexp     HTTPUrlRegexp
	urlRegexp             HTTPUrlRegexp
	urlRewrite            UrlRewriteMap
	headerFilters         HTTPHeaderFilters
	headerNegativeFilters HTTPHeaderFilters
	headerHashFilters     HTTPHashFilters
	paramHashFilters      HTTPHashFilters

	params  HTTPParams
	headers HTTPHeaders
	methods HTTPMethods
}

func (config *HTTPModifierConfig) IsEmpty() bool {
	if len(config.urlRegexp) == 0 &&
		len(config.urlNegativeRegexp) == 0 &&
		len(config.urlRewrite) == 0 &&
		len(config.headerFilters) == 0 &&
		len(config.headerNegativeFilters) == 0 &&
		len(config.headerHashFilters) == 0 &&
		len(config.paramHashFilters) == 0 &&
		len(config.params) == 0 &&
		len(config.headers) == 0 &&
		len(config.methods) == 0 {
		return true
	}

	return false
}

//
// Handling of --http-allow-header, --http-disallow-header options
//
type headerFilter struct {
	name   []byte
	regexp *regexp.Regexp
}

// HTTPHeaderFilters holds list of headers and their regexps
type HTTPHeaderFilters []headerFilter

func (h *HTTPHeaderFilters) String() string {
	return fmt.Sprint(*h)
}

func (h *HTTPHeaderFilters) Set(value string) error {
	valArr := strings.SplitN(value, ":", 2)
	if len(valArr) < 2 {
		return errors.New("need both header and value, colon-delimited (ex. user_id:^169$).")
	}
	r, err := regexp.Compile(valArr[1])
	if err != nil {
		return err
	}

	*h = append(*h, headerFilter{name: []byte(valArr[0]), regexp: r})

	return nil
}

//
// Handling of --http-allow-header-hash and --http-allow-param-hash options
//
type hashFilter struct {
	name    []byte
	percent uint32
}

type HTTPHashFilters []hashFilter

func (h *HTTPHashFilters) String() string {
	return fmt.Sprint(*h)
}

func (h *HTTPHashFilters) Set(value string) error {
	valArr := strings.SplitN(value, ":", 2)
	if len(valArr) < 2 {
		return errors.New("need both header and value, colon-delimited (ex. user_id:50%)")
	}

	f := hashFilter{name: []byte(valArr[0])}

	if strings.Contains(valArr[1], "%") {
		p, _ := strconv.ParseInt(valArr[1][:len(valArr[1])-1], 0, 0)
		f.percent = uint32(p)
	} else if strings.Contains(valArr[1], "/") {
		// DEPRECATED format
		var num, den uint64

		fracArr := strings.Split(valArr[1], "/")
		num, _ = strconv.ParseUint(fracArr[0], 10, 64)
		den, _ = strconv.ParseUint(fracArr[1], 10, 64)

		f.percent = uint32((float64(num) / float64(den)) * 100)
	} else {
		return errors.New("Value should be percent and contain '%'")
	}

	*h = append(*h, f)

	return nil
}

//
// Handling of --http-set-header option
//
type HTTPHeaders []HTTPHeader
type HTTPHeader struct {
	Name  string
	Value string
}

func (h *HTTPHeaders) String() string {
	return fmt.Sprint(*h)
}

func (h *HTTPHeaders) Set(value string) error {
	v := strings.SplitN(value, ":", 2)
	if len(v) != 2 {
		return errors.New("Expected `Key: Value`")
	}

	header := HTTPHeader{
		strings.TrimSpace(v[0]),
		strings.TrimSpace(v[1]),
	}

	*h = append(*h, header)
	return nil
}

//
// Handling of --http-set-param option
//
type HTTPParams []HTTPParam
type HTTPParam struct {
	Name  []byte
	Value []byte
}

func (h *HTTPParams) String() string {
	return fmt.Sprint(*h)
}

func (h *HTTPParams) Set(value string) error {
	v := strings.SplitN(value, "=", 2)
	if len(v) != 2 {
		return errors.New("Expected `Key=Value`")
	}

	param := HTTPParam{
		[]byte(strings.TrimSpace(v[0])),
		[]byte(strings.TrimSpace(v[1])),
	}

	*h = append(*h, param)
	return nil
}

//
// Handling of --http-allow-method option
//
type HTTPMethods [][]byte

func (h *HTTPMethods) String() string {
	return fmt.Sprint(*h)
}

func (h *HTTPMethods) Set(value string) error {
	*h = append(*h, []byte(value))
	return nil
}

//
// Handling of --http-rewrite-url option
//
type urlRewrite struct {
	src    *regexp.Regexp
	target []byte
}

type UrlRewriteMap []urlRewrite

func (r *UrlRewriteMap) String() string {
	return fmt.Sprint(*r)
}

func (r *UrlRewriteMap) Set(value string) error {
	valArr := strings.SplitN(value, ":", 2)
	if len(valArr) < 2 {
		return errors.New("need both src and target, colon-delimited (ex. /a:/b)")
	}
	regexp, err := regexp.Compile(valArr[0])
	if err != nil {
		return err
	}
	*r = append(*r, urlRewrite{src: regexp, target: []byte(valArr[1])})
	return nil
}

//
// Handling of --http-allow-url option
//
type urlRegexp struct {
	regexp *regexp.Regexp
}

type HTTPUrlRegexp []urlRegexp

func (r *HTTPUrlRegexp) String() string {
	return fmt.Sprint(*r)
}

func (r *HTTPUrlRegexp) Set(value string) error {
	regexp, err := regexp.Compile(value)

	*r = append(*r, urlRegexp{regexp: regexp})

	return err
}
