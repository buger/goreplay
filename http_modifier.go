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
	if len(config.urlRegexp) == 0 &&
		len(config.urlNegativeRegexp) == 0 &&
		len(config.urlRewrite) == 0 &&
		len(config.headerRewrite) == 0 &&
		len(config.headerFilters) == 0 &&
		len(config.headerNegativeFilters) == 0 &&
		len(config.headerBasicAuthFilters) == 0 &&
		len(config.headerHashFilters) == 0 &&
		len(config.paramHashFilters) == 0 &&
		len(config.params) == 0 &&
		len(config.headers) == 0 &&
		len(config.methods) == 0 {
		return nil
	}

	return &HTTPModifier{config: config}
}

func (m *HTTPModifier) Rewrite(payload []byte) (response []byte) {
	if !proto.IsHTTPPayload(payload) {
		return payload
	}

	if len(m.config.methods) > 0 {
		method := proto.Method(payload)

		matched := false

		for _, m := range m.config.methods {
			if bytes.Equal(method, m) {
				matched = true
				break
			}
		}

		if !matched {
			return
		}
	}

	if len(m.config.headers) > 0 {
		for _, header := range m.config.headers {
			payload = proto.SetHeader(payload, []byte(header.Name), []byte(header.Value))
		}
	}

	if len(m.config.params) > 0 {
		for _, param := range m.config.params {
			payload = proto.SetPathParam(payload, param.Name, param.Value)
		}
	}

	if len(m.config.urlRegexp) > 0 {
		path := proto.Path(payload)

		matched := false

		for _, f := range m.config.urlRegexp {
			if f.regexp.Match(path) {
				matched = true
				break
			}
		}

		if !matched {
			return
		}
	}

	if len(m.config.urlNegativeRegexp) > 0 {
		path := proto.Path(payload)

		for _, f := range m.config.urlNegativeRegexp {
			if f.regexp.Match(path) {
				return
			}
		}
	}

	if len(m.config.headerFilters) > 0 {
		for _, f := range m.config.headerFilters {
			value := proto.Header(payload, f.name)

			if len(value) > 0 && !f.regexp.Match(value) {
				return
			}
		}
	}

	if len(m.config.headerNegativeFilters) > 0 {
		for _, f := range m.config.headerNegativeFilters {
			value := proto.Header(payload, f.name)

			if len(value) > 0 && f.regexp.Match(value) {
				return
			}
		}
	}

	if len(m.config.headerBasicAuthFilters) > 0 {
		for _, f := range m.config.headerBasicAuthFilters {
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

	if len(m.config.headerHashFilters) > 0 {
		for _, f := range m.config.headerHashFilters {
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

	if len(m.config.paramHashFilters) > 0 {
		for _, f := range m.config.paramHashFilters {
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

	if len(m.config.urlRewrite) > 0 {
		path := proto.Path(payload)

		for _, f := range m.config.urlRewrite {
			if f.src.Match(path) {
				path = f.src.ReplaceAll(path, f.target)
				payload = proto.SetPath(payload, path)

				break
			}
		}
	}

	if len(m.config.headerRewrite) > 0 {
		for _, f := range m.config.headerRewrite {
			value := proto.Header(payload, f.header)
			if len(value) == 0 {
				break
			}

			if f.src.Match(value) {
				newValue := f.src.ReplaceAll(value, f.target)
				payload = proto.SetHeader(payload, f.header, newValue)
			}
		}
	}

	return payload
}
