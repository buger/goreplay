package main

import (
	"bytes"
	"hash/fnv"
	"log"

	"github.com/buger/gor/proto"
)

type HTTPModifier struct {
	output chan []byte
	config *HTTPModifierConfig
}

func NewHTTPModifier(config *HTTPModifierConfig) *HTTPModifier {
	m := &HTTPModifier{config: config}

	m.output = make(chan []byte, 1000)

	return m
}

func (m *HTTPModifier) Read(data []byte) (int, error) {
	buf := <-m.output
	copy(data, buf)

	return len(buf), nil
}

func (m *HTTPModifier) Write(payload []byte) (int, error) {
	if !isRequestPayload(payload) {
		m.output <- payload
		return len(payload), nil
	}

	body := payloadBody(payload)

	if !proto.IsHTTPPayload(body) {
		m.output <- payload
		return len(payload), nil
	}

	if len(m.config.methods) > 0 {
		method := proto.Method(body)

		matched := false

		for _, m := range m.config.methods {
			if bytes.Equal(method, m) {
				matched = true
				break
			}
		}

		if !matched {
			return 0, nil
		}
	}

	if len(m.config.urlRegexp) > 0 {
		path := proto.Path(body)

		matched := false

		for _, f := range m.config.urlRegexp {
			if f.regexp.Match(path) {
				matched = true
				break
			}
		}

		if !matched {
			return 0, nil
		}
	}

	if len(m.config.urlNegativeRegexp) > 0 {
		path := proto.Path(body)

		for _, f := range m.config.urlNegativeRegexp {
			if f.regexp.Match(path) {
				return 0, nil
			}
		}
	}

	if len(m.config.headerFilters) > 0 {
		for _, f := range m.config.headerFilters {
			value := proto.Header(body, f.name)

			if len(value) > 0 && !f.regexp.Match(value) {
				return 0, nil
			}
		}
	}

	if len(m.config.headerNegativeFilters) > 0 {
		for _, f := range m.config.headerNegativeFilters {
			value := proto.Header(body, f.name)

			if len(value) > 0 && f.regexp.Match(value) {
				return 0, nil
			}
		}
	}

	if len(m.config.headerHashFilters) > 0 {
		for _, f := range m.config.headerHashFilters {
			value := proto.Header(body, f.name)

			if len(value) > 0 {
				hasher := fnv.New32a()
				hasher.Write(value)

				if (hasher.Sum32() % 100) >= f.percent {
					return 0, nil
				}
			}
		}
	}

	if len(m.config.paramHashFilters) > 0 {
		for _, f := range m.config.paramHashFilters {
			value, s, _ := proto.PathParam(body, f.name)

			if s != -1 {
				hasher := fnv.New32a()
				hasher.Write(value)

				if (hasher.Sum32() % 100) >= f.percent {
					return 0, nil
				}
			}
		}
	}

	isChanged := false

	if len(m.config.headers) > 0 {
		for _, header := range m.config.headers {
			isChanged = true
			body = proto.SetHeader(body, []byte(header.Name), []byte(header.Value))
		}
	}

	if len(m.config.params) > 0 {
		for _, param := range m.config.params {
			isChanged = true
			body = proto.SetPathParam(body, param.Name, param.Value)
		}
	}

	if len(m.config.urlRewrite) > 0 {
		path := proto.Path(body)

		for _, f := range m.config.urlRewrite {
			if f.src.Match(path) {

				log.Println(string(path))
				isChanged = true

				path = f.src.ReplaceAll(path, f.target)
				body = proto.SetPath(body, path)

				break
			}
		}
	}

	if isChanged {
		headerSize := bytes.IndexByte(payload, '\n')
		payload = append(payload[:headerSize+1], body...)
	}

	m.output <- payload
	return len(payload), nil
}

func (m *HTTPModifier) Rewrite(payload []byte) []byte {
	n, _ := m.Write(payload)

	if n == 0 {
		return []byte{}
	}

	return <-m.output
}
