#### Overview
Middleware is a program that accepts request and response payload at STDIN and emits modified requests at STDOUT. You can implement any custom logic like stripping private data, advanced rewriting, support for oAuth and etc. Check examples [included into our repo](https://github.com/buger/gor/tree/master/examples/middleware).


```
                   Original request      +--------------+
+-------------+----------STDIN---------->+              |
|  Gor input  |                          |  Middleware  |
+-------------+----------STDIN---------->+              |
                   Original response (1) +------+---+---+
                                                |   ^
+-------------+    Modified request             v   |
| Gor output  +<---------STDOUT-----------------+   |
+-----+-------+                                     |
      |                                             |
      |            Replayed response                |
      +------------------STDIN----------------->----+
```

(1): Original responses will only be sent to the middleware if the `--input-raw-track-response` option is specified.

Middleware can be written in any language, see `examples/middleware` folder for examples.
Middleware program should accept the fact that all communication with Gor is asynchronous, there is no guarantee that original request and response messages will come one after each other. Your app should take care of the state if logic depends on original or replayed response, see `examples/middleware/token_modifier.go` as example.

Simple bash echo middleware (returns same request) will look like this:
```bash
while read line; do
  echo $line
end
```

Middleware can be enabled using `--middleware` option, by specifying path to executable file:
```
gor --input-raw :80 --middleware "/opt/middleware_executable" --output-http "http://staging.server"
```

#### Communication protocol
All messages should be hex encoded, new line character specifieds the end of the message, eg. new message per line.

Decoded payload consist of 2 parts: header and HTTP payload, separated by new line character.  

Example request payload:

```
1 932079936fa4306fc308d67588178d17d823647c 1439818823587396305
GET /a HTTP/1.1
Host: 127.0.0.1

```

Example response payload (note: you will only receive this if you specify `--input-raw-track-response`)

```
2 8e091765ae902fef8a2b7d9dd960e9d52222bd8c 1439818823587996305 2782013
HTTP/1.1 200 OK
Date: Mon, 17 Aug 2015 13:40:23 GMT
Content-Length: 0
Content-Type: text/plain; charset=utf-8

```

Header contains request meta information separated by spaces. First value is payload type, possible values: `1` - request, `2` - original response, `3` - replayed response.
Next goes request id: unique among all requests (sha1 of time and Ack), but remain same for original and replayed response, so you can create associations between request and responses. The third argument is the time when request/response was initiated/received. Forth argument is populated only for responses and means latency.

HTTP payload is unmodified HTTP requests/responses intercepted from network. You can read more about request format [here](http://www.jmarshall.com/easy/http/), [here](https://en.wikipedia.org/wiki/Hypertext_Transfer_Protocol) and [here](http://www.w3.org/Protocols/rfc2616/rfc2616.html). You can operate with payload as you want, add headers, change path, and etc. Basically you just editing a string, just ensure that it is RCF compliant.

At the end modified (or untouched) request should be emitted back to STDOUT, keeping original header, and hex-encoded. If you want to filter request, just not send it. Emitting responses back is required, even if you did not touch them.

#### Advanced example
Imagine that you have auth system that randomly generate access tokens, which used later for accessing secure content. Since there is no pre-defined token value, naive approach without middleware (or if middleware use only request payloads) will fail, because replayed server have own tokens, not synced with origin. To fix this, our middleware should take in account responses of replayed and origin server, store `originalToken -> replayedToken` aliases and rewrite all requests using this token to use replayed alias. See [examples/middleware/token_modifier.go](https://github.com/buger/gor/tree/master/examples/middleware/token_modifier.go) and [middleware_test.go#TestTokenMiddleware](https://github.com/buger/gor/tree/master/middleware_test.go) as example of described scheme.

***

You may also read about [[Request filtering]], [[Rate limiting]] and [[Request rewriting]].