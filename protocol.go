package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// These constants help to indicate the type of payload
const (
	RequestPayload          = '1'
	ResponsePayload         = '2'
	ReplayedResponsePayload = '3'
)

func randByte(len int) []byte {
	b := make([]byte, len/2)
	rand.Read(b)

	h := make([]byte, len)
	hex.Encode(h, b)

	return h
}

func uuid() []byte {
	return randByte(24)
}

var payloadSeparator = "\n🐵🙈🙉\n"

func payloadScanner(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.Index(data, []byte(payloadSeparator)); i >= 0 {
		// We have a full newline-terminated line.
		return i + len([]byte(payloadSeparator)), data[0:i], nil
	}

	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// Timing is request start or round-trip time, depending on payloadType
func payloadHeader(payloadType byte, uuid []byte, timing int64, latency int64) (header []byte) {
	//Example:
	//  3 f45590522cd1838b4a0d5c5aab80b77929dea3b3 13923489726487326 1231\n
	return []byte(fmt.Sprintf("%c %s %d %d\n", payloadType, uuid, timing, latency))
}

func payloadBody(payload []byte) []byte {
	headerSize := bytes.IndexByte(payload, '\n')
	return payload[headerSize+1:]
}

func payloadMeta(payload []byte) [][]byte {
	headerSize := bytes.IndexByte(payload, '\n')
	if headerSize < 0 {
		return nil
	}
	return bytes.Split(payload[:headerSize], []byte{' '})
}

func payloadMetaWithBody(payload []byte) (meta, body []byte) {
	if i := bytes.IndexByte(payload, '\n'); i > 0 && len(payload) > i+1 {
		meta = payload[:i+1]
		body = payload[i+1:]
		return
	}
	// we assume the message did not have meta data
	return nil, payload
}

func payloadID(payload []byte) (id []byte) {
	meta := payloadMeta(payload)

	if len(meta) < 2 {
		return
	}
	return meta[1]
}

func isOriginPayload(payload []byte) bool {
	return payload[0] == RequestPayload || payload[0] == ResponsePayload
}

func isRequestPayload(payload []byte) bool {
	return payload[0] == RequestPayload
}
