# GoReplay middleware

GoReplay support protocol for writing middleware in any language, which allows you to implement custom logic like authentification or complex rewriting and filterting. See protocol description here: https://github.com/buger/goreplay/wiki/Middleware, but the basic idea that middleware process receive hex encoded data via STDIN and emits it back via STDOUT. STDERR for loggin inside middleware. Yes, that's simple.

To simplify middleware creation we provide packages for NodeJS and Go (upcoming).

## NodeJS

Before starting, you should install the package via npm: `npm install goreplay_middleware`.
And initialize middleware the following way:
```javascript
var gor = require("goreplay_middleware");
// `init` will initialize STDIN listener
gor.init();
```

Basic idea is that you write callbacks which respond to `request`, `response`, `replay`, or `message` events, which contain request meta information and actuall http paylod. Depending on your needs you may compare, override or filter incoming requests and responses.

You can respond to the incoming events using `on` function, by providing callbacks:
```javascript
// valid events are `request`, `response` (original response), `replay` (replayed response), and `message` (all events)
gor.on('request', function(data) {
    // `data` contains incoming message its meta information.
    data

    // Raw HTTP payload of `Buffer` type
    // Example (hidden character for line endings shown on purpose):
    //   GET / HTTP/1.1\r\n
    //   User-Agent: Golang\r\n
    //   \r\n
    data.html

    // Meta is an array size of 3, containing:
    //   1. request type - 1, 2 or 3 (which maps to `request`, `respose` and `replay`)
    //   2. timestamp of when request was made (for responses it is time of request start too)
    //   3. latency - time difference between request start and finish. For `request` is zero.
    data.meta

    // Unique request ID. It should be same for `request`, `response` and `replay` events of the same request.
    data.ID

    // You should return data at the end of function, even if you not changed request, if you do not want to filter it out.
    // If you just `return` nothing, request will be filtered
    return data
})
```
### Mapping requests and responses
You can provide request ID as additional argument to `on` function, which allow you to map related requests and responses. Below is example of middleware which checks that original and replayed response have same HTTP status code.

```javascript
// Example of very basic way to compare if replayed traffic have no errors
gor.on("request", function(req) {
    gor.on("response", req.ID, function(resp) {
        gor.on("replay", req.ID, function(repl) {
            if (gor.httpStatus(resp.http) != gor.httpStatus(repl.http)) {
                // Note that STDERR is used for logging, and it actually will be send to `Gor` STDOUT.
                // This trick is used because SDTIN and STDOUT already used for process communication.
                // You can write logger that writes to files insead.
                console.error(`${gor.httpPath(req.http)} STATUS NOT MATCH: 'Expected ${gor.httpStatus(resp.http)}' got '${gor.httpStatus(repl.http)}'`)
            }
            return repl;
        })
        return resp;
    })
    return req
})
```

This middleware include `searchResponses` helper used to compare values from original and replayed responses. It may be helpful if auth system or xsrf protection returns unique tokens in headers or response, and you need to rewrite your requests based on them. Because tokens are unique, value contained in original and replayed response will differ, so you need to extract value from both responses, and rewrite requests based on those mappings.

`searchResponses` accepts request id, regexp pattern for searching the compared value (should include capture group), and callback which returns both original and replayed matched value.

Example: 
```javascript
   // Compare HTTP headers for response and replayed response, and map values
let tokMap = {};

gor.on("request", function(req) {
    let tok = gor.httpHeader(req.http, "Auth-Token");
    if (tok && tokMap[tok]) {
        req.http = gor.setHttpHeader(req.http, "Auth-Token", tokMap[tok]) 
    }
    
    gor.searchResponses(req.ID, "X-Set-Token: (\w+)$", function(respTok, replTok) {
        if (respTok && replTok) tokMap[respTok] = replTok;
    })

    return req;
})
```


### API documentation

Package expose following functions to process raw HTTP payloads:
* `init` - initialize middleware object, start reading from STDIN.
* `httpPath` - URL path of the request: `gor.httpPath(req.http)`
* `setHttpPath` - update URL path: `req.http = gor.setHttpPath(req.http, newPath)`
* `httpPathParam` - get param from URL path: `gor.httpPathParam(req.http, queryParam)`
* `setHttpPathParam` - set URL param: `req.http = gor.setHttpPathParam(req.http, queryParam, value)` 
* `httpStatus` - response status code
* `httpHeader` - get HTTP header: `gor.httpHeader(req.http, "Content-Length")`
* `setHttpHeader` - Set HTTP header, returns modified payload: `req.http = gor.setHttpHeader(req.http, "X-Replayed", "1")`
* `httpBody` - get HTTP Body: `gor.httpBody(req.http)`
* `setHttpBody` - Set HTTP Body and ensures that `Content-Length` header have proper value. Returns modified payload: `req.http = gor.setHttpBody(req.http, Buffer.from('hello!'))`.
* `httpBodyParam` - get POST body param: `gor.httpBodyParam(req.http, param)`
* `setHttpBodyParam` - set POST body param: `req.http = gor.setHttpBodyParam(req.http, param, value)`
* `httpCookie` - get HTTP cookie: `gor.httpCookie(req.http, "SESSSION_ID")`
* `setHttpCookie` - set HTTP cookie, returns modified payload: `req.http = gor.setHttpCookie(req.http, "iam", "cuckoo")`

Also it is totally legit to use standard `Buffer` functions like `indexOf` for processing the HTTP payload. Just do not forget that if you modify modify the body, update the `Content-Length` header with new value. And if you modify headers, line endings should be `\r\n`. Rest is up to your imagination.


## Support

Feel free to ask questions here and by sending email to [support@goreplay.org](mailto:support@goreplay.org). Commercial support available and welcomed 🙈.
