package main

import (
	"bytes"
	"encoding/base64"
	"hash/fnv"
	"strings"

	"github.com/buger/goreplay/proto"
)

type HTTPModifier struct {
	config *HTTPModifierConfig
}

func NewHTTPModifier(config *HTTPModifierConfig) *HTTPModifier {
	// Optimization to skip modifier completely if we do not need it
	if len(config.URLRegexp) == 0 &&
		len(config.URLNegativeRegexp) == 0 &&
		len(config.URLRewrite) == 0 &&
		len(config.HeaderRewrite) == 0 &&
		len(config.HeaderFilters) == 0 &&
		len(config.HeaderNegativeFilters) == 0 &&
		len(config.HeaderBasicAuthFilters) == 0 &&
		len(config.HeaderHashFilters) == 0 &&
		len(config.ParamHashFilters) == 0 &&
		len(config.Params) == 0 &&
		len(config.Headers) == 0 &&
		len(config.Methods) == 0 {
		return nil
	}

	return &HTTPModifier{config: config}
}

func (m *HTTPModifier) Rewrite(payload []byte) (response []byte) {
	if !proto.HasRequestTitle(payload) {
		return payload
	}

	if len(m.config.Methods) > 0 {
		method := proto.Method(payload)

		matched := false

		for _, m := range m.config.Methods {
			if bytes.Equal(method, m) {
				matched = true
				break
			}
		}

		if !matched {
			return
		}
	}

	if len(m.config.Headers) > 0 {
		for _, header := range m.config.Headers {
			payload = proto.SetHeader(payload, []byte(header.Name), []byte(header.Value))
		}
	}

	if len(m.config.Params) > 0 {
		for _, param := range m.config.Params {
			payload = proto.SetPathParam(payload, param.Name, param.Value)
		}
	}

	if len(m.config.URLRegexp) > 0 {
		path := proto.Path(payload)

		matched := false

		for _, f := range m.config.URLRegexp {
			if f.regexp.Match(path) {
				matched = true
				break
			}
		}

		if !matched {
			return
		}
	}

	if len(m.config.URLNegativeRegexp) > 0 {
		path := proto.Path(payload)

		for _, f := range m.config.URLNegativeRegexp {
			if f.regexp.Match(path) {
				return
			}
		}
	}

	if len(m.config.HeaderFilters) > 0 {
		for _, f := range m.config.HeaderFilters {
			value := proto.Header(payload, f.name)

			if len(value) == 0 {
				return
			}

			if !f.regexp.Match(value) {
				return
			}
		}
	}

	if len(m.config.HeaderNegativeFilters) > 0 {
		for _, f := range m.config.HeaderNegativeFilters {
			value := proto.Header(payload, f.name)

			if len(value) > 0 && f.regexp.Match(value) {
				return
			}
		}
	}

	if len(m.config.HeaderBasicAuthFilters) > 0 {
		for _, f := range m.config.HeaderBasicAuthFilters {
			value := proto.Header(payload, []byte("Authorization"))

			if len(value) > 0 {
				valueString := string(value)
				trimmedBasicAuthEncoded := strings.TrimPrefix(valueString, "Basic ")
				if strings.Compare(valueString, trimmedBasicAuthEncoded) != 0 {
					decodedAuth, _ := base64.StdEncoding.DecodeString(trimmedBasicAuthEncoded)
					if !f.regexp.Match(decodedAuth) {
						return
					}
				}
			}
		}
	}

	if len(m.config.HeaderHashFilters) > 0 {
		for _, f := range m.config.HeaderHashFilters {
			value := proto.Header(payload, f.name)

			if len(value) > 0 {
				hasher := fnv.New32a()
				hasher.Write(value)

				if (hasher.Sum32() % 100) >= f.percent {
					return
				}
			}
		}
	}

	if len(m.config.ParamHashFilters) > 0 {
		for _, f := range m.config.ParamHashFilters {
			value, s, _ := proto.PathParam(payload, f.name)

			if s != -1 {
				hasher := fnv.New32a()
				hasher.Write(value)

				if (hasher.Sum32() % 100) >= f.percent {
					return
				}
			}
		}
	}

	if len(m.config.URLRewrite) > 0 {
		path := proto.Path(payload)

		for _, f := range m.config.URLRewrite {
			if f.src.Match(path) {
				path = f.src.ReplaceAll(path, f.target)
				payload = proto.SetPath(payload, path)

				break
			}
		}
	}

	if len(m.config.HeaderRewrite) > 0 {
		for _, f := range m.config.HeaderRewrite {
			value := proto.Header(payload, f.header)
			if len(value) == 0 {
				continue
			}

			if f.src.Match(value) {
				newValue := f.src.ReplaceAll(value, f.target)
				payload = proto.SetHeader(payload, f.header, newValue)
			}
		}
	}

	return payload
}
