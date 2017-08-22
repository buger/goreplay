/*
This middleware made for auth system that randomly generate access tokens, which used later for accessing secure content. Since there is no pre-defined token value, naive approach without middleware (or if middleware use only request payloads) will fail, because replayed server have own tokens, not synced with origin. To fix this, our middleware should take in account responses of replayed and origin server, store `originalToken -> replayedToken` aliases and rewrite all requests using this token to use replayed alias. See `middleware_test.go#TestTokenMiddleware` test for examples of using this middleware.

How middleware works:

                   Original request      +--------------+
+-------------+----------STDIN---------->+              |
|  Gor input  |                          |  Middleware  |
+-------------+----------STDIN---------->+              |
                   Original response     +------+---+---+
                                                |   ^
+-------------+    Modified request             v   |
| Gor output  +<---------STDOUT-----------------+   |
+-----+-------+                                     |
      |                                             |
      |            Replayed response                |
      +------------------STDIN----------------->----+
*/

package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/buger/goreplay/proto"
)

// requestID -> originalToken
var originalTokens map[string][]byte

// originalToken -> replayedToken
var tokenAliases map[string][]byte

func main() {
	originalTokens = make(map[string][]byte)
	tokenAliases = make(map[string][]byte)

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		encoded := scanner.Bytes()
		buf := make([]byte, len(encoded)/2)
		hex.Decode(buf, encoded)

		process(buf)
	}
}

func process(buf []byte) {
	// First byte indicate payload type, possible values:
	//  1 - Request
	//  2 - Response
	//  3 - ReplayedResponse
	payloadType := buf[0]
	headerSize := bytes.IndexByte(buf, '\n') + 1
	header := buf[:headerSize-1]

	// Header contains space separated values of: request type, request id, and request start time (or round-trip time for responses)
	meta := bytes.Split(header, []byte(" "))
	// For each request you should receive 3 payloads (request, response, replayed response) with same request id
	reqID := string(meta[1])
	payload := buf[headerSize:]

	Debug("Received payload:", string(buf))

	switch payloadType {
	case '1': // Request
		if bytes.Equal(proto.Path(payload), []byte("/token")) {
			originalTokens[reqID] = []byte{}
			Debug("Found token request:", reqID)
		} else {
			token, vs, _ := proto.PathParam(payload, []byte("token"))

			if vs != -1 { // If there is GET token param
				if alias, ok := tokenAliases[string(token)]; ok {
					// Rewrite original token to alias
					payload = proto.SetPathParam(payload, []byte("token"), alias)

					// Copy modified payload to our buffer
					buf = append(buf[:headerSize], payload...)
				}
			}
		}

		// Emitting data back
		os.Stdout.Write(encode(buf))
	case '2': // Original response
		if _, ok := originalTokens[reqID]; ok {
			// Token is inside response body
			secureToken := proto.Body(payload)
			originalTokens[reqID] = secureToken
			Debug("Remember origial token:", string(secureToken))
		}
	case '3': // Replayed response
		if originalToken, ok := originalTokens[reqID]; ok {
			delete(originalTokens, reqID)
			secureToken := proto.Body(payload)
			tokenAliases[string(originalToken)] = secureToken

			Debug("Create alias for new token token, was:", string(originalToken), "now:", string(secureToken))
		}
	}
}

func encode(buf []byte) []byte {
	dst := make([]byte, len(buf)*2+1)
	hex.Encode(dst, buf)
	dst[len(dst)-1] = '\n'

	return dst
}

func Debug(args ...interface{}) {
	fmt.Fprint(os.Stderr, "[DEBUG][TOKEN-MOD] ")
	fmt.Fprintln(os.Stderr, args...)
}
