package proto

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

// RandByte generates a random byte array of length len
func RandByte(len int) []byte {
	b := make([]byte, len/2)
	rand.Read(b)

	h := make([]byte, len)
	hex.Encode(h, b)

	return h
}

// UUID generates a random 24 byte string
func UUID() []byte {
	return RandByte(24)
}

// PayloadSeparator is used to separate payloads
var PayloadSeparator = "\n🐵🙈🙉\n"

// PayloadSeparatorAsBytes is used to separate payloads
var PayloadSeparatorAsBytes = []byte(PayloadSeparator)

// PayloadScanner is used to split payloads
func PayloadScanner(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.Index(data, []byte(PayloadSeparator)); i >= 0 {
		// We have a full newline-terminated line.
		return i + len([]byte(PayloadSeparator)), data[0:i], nil
	}

	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// Timing is request start or round-trip time, depending on payloadType
func PayloadHeader(payloadType byte, uuid []byte, timing int64, latency int64) (header []byte) {
	//Example:
	//  3 f45590522cd1838b4a0d5c5aab80b77929dea3b3 13923489726487326 1231\n
	return []byte(fmt.Sprintf("%c %s %d %d\n", payloadType, uuid, timing, latency))
}

// PayloadBody returns the body of the payload
func PayloadBody(payload []byte) []byte {
	headerSize := bytes.IndexByte(payload, '\n')
	return payload[headerSize+1:]
}

// PayloadMeta returns the meta data of the payload
func PayloadMeta(payload []byte) [][]byte {
	headerSize := bytes.IndexByte(payload, '\n')
	if headerSize < 0 {
		return nil
	}
	return bytes.Split(payload[:headerSize], []byte{' '})
}

// PayloadMetaWithBody returns the meta data and body of the payload
func PayloadMetaWithBody(payload []byte) (meta, body []byte) {
	if i := bytes.IndexByte(payload, '\n'); i > 0 && len(payload) > i+1 {
		meta = payload[:i+1]
		body = payload[i+1:]
		return
	}
	// we assume the message did not have meta data
	return nil, payload
}

// PayloadID returns the ID of the payload
func PayloadID(payload []byte) (id []byte) {
	meta := PayloadMeta(payload)

	if len(meta) < 2 {
		return
	}
	return meta[1]
}

// IsOriginPayload returns true if the payload is a request or response payload
func IsOriginPayload(payload []byte) bool {
	return payload[0] == RequestPayload || payload[0] == ResponsePayload
}

// IsRequestPayload returns true if the payload is a request payload
func IsRequestPayload(payload []byte) bool {
	return payload[0] == RequestPayload
}
