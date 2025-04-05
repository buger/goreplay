# GoReplay middleware

GoReplay support protocol for writing middleware in any language, which allows you to implement custom logic like authentication or complex rewriting and filterting. See protocol description here: https://github.com/buger/goreplay/wiki/Middleware, but the basic idea that middleware process receive hex encoded data via STDIN and emits it back via STDOUT. STDERR for logging inside middleware. Yes, that's simple.

To simplify middleware creation we provide packages for NodeJS and Go (upcoming).

If you want to get access to original and replayed responses, do not forget adding `--output-http-track-response` and `--input-raw-track-response` options.

## NodeJS

Before starting, you should install the package via npm: `npm install goreplay_middleware`.
And initialize middleware the following way:
```javascript
var gor = require("goreplay_middleware");
// `init` will initialize STDIN listener
gor.init();
```

Basic idea is that you write callbacks which respond to `request`, `response`, `replay`, or `message` events, which contain request meta information and actual http paylod. Depending on your needs you may compare, override or filter incoming requests and responses.

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
    data.http

    // Meta is an array size of 4, containing:
    //   1. request type - 1, 2 or 3 (which maps to `request`, `response` and `replay`)
    //   2. uuid - request unique identifier. Request responses have the same ID as their request.
    //   3. timestamp of when request was made (for responses it is time of request start too)
    //   4. latency - time difference between request start and finish. For `request` is zero.
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
                // You can write logger that writes to files instead.
                console.error(`${gor.httpPath(req.http)} STATUS NOT MATCH: 'Expected ${gor.httpStatus(resp.http)}' got '${gor.httpStatus(repl.http)}'`)
            }
            return repl;
        })
        return resp;
    })
    return req
})
```

This middleware includes `searchResponses` helper which is used to compare value of original response with the replayed response. If authentication system or xsrf protection returns unique tokens in headers or the response, it will be helpful to rewrite your requests based on them. Because tokens are unique, and the value contained in original and replayed responses will be different. So, you need to extract value from both responses, and rewrite requests based on those mappings.

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
* `httpMethod` - Http method: 'GET', 'POST', etc. `gor.httpMethod(req.http)`. 
* `setHttpPath` - update URL path: `req.http = gor.setHttpPath(req.http, newPath)`
* `httpPathParam` - get param from URL path: `gor.httpPathParam(req.http, queryParam)`
* `setHttpPathParam` - set URL param: `req.http = gor.setHttpPathParam(req.http, queryParam, value)` 
* `httpStatus` - response status code
* `httpHeaders` - get all headers: `gor.httpHeaders(req.http)`
* `httpHeader` - get HTTP header: `gor.httpHeader(req.http, "Content-Length")`
* `setHttpHeader` - Set HTTP header, returns modified payload: `req.http = gor.setHttpHeader(req.http, "X-Replayed", "1")`
* `httpBody` - get HTTP Body: `gor.httpBody(req.http)`
* `setHttpBody` - Set HTTP Body and ensures that `Content-Length` header have proper value. Returns modified payload: `req.http = gor.setHttpBody(req.http, Buffer.from('hello!'))`.
* `httpBodyParam` - get POST body param: `gor.httpBodyParam(req.http, param)`
* `setHttpBodyParam` - set POST body param: `req.http = gor.setHttpBodyParam(req.http, param, value)`
* `httpCookie` - get HTTP cookie: `gor.httpCookie(req.http, "SESSION_ID")`
* `setHttpCookie` - set HTTP cookie, returns modified payload: `req.http = gor.setHttpCookie(req.http, "iam", "cuckoo")`
* `deleteHttpCookie` - delete HTTP cookie, returns modified payload: `req.http = gor.deleteHttpCookie(req.http, "iam")`

Also it is totally legit to use standard `Buffer` functions like `indexOf` for processing the HTTP payload. Just do not forget that if you modify the body, update the `Content-Length` header with a new value. And if you modify any of the headers, line endings should be `\r\n`. Rest is up to your imagination.


## Masking PII Data

This middleware provides functionality to mask Personally Identifiable Information (PII) data in HTTP requests based on specified headers and JSON paths. It allows you to define a configuration object that specifies which headers and JSON fields should be masked and the type of data they represent.

```javascript
const gor = require("goreplay_middleware");
const faker = require("faker");

// Initialize the middleware
gor.init();

// Configuration for masking PII data
const maskConfig = {
  headers: [
    { name: "Authorization", type: "token" },
    { name: "X-API-Key", type: "token" },
    { name: "X-User-Email", type: "email" },
    { name: "X-User-Name", type: "name" },
  ],
  jsonPaths: [
    { path: "$.user.email", type: "email" },
    { path: "$.user.name", type: "name" },
    { path: "$.user.phone", type: "phone" },
    { path: "$.user.address", type: "address" },
  ],
};

// Function to mask a value based on its type
function maskValue(type) {
  switch (type) {
    case "email":
      return faker.internet.email();
    case "name":
      return faker.name.findName();
    case "phone":
      return faker.phone.phoneNumber();
    case "address":
      return faker.address.streetAddress();
    case "token":
      return faker.random.alphaNumeric(32);
    default:
      return "***";
  }
}

// Middleware function to mask PII data
gor.on("message", (data) => {
  // Mask headers
  maskConfig.headers.forEach((header) => {
    const value = gor.httpHeader(data.http, header.name);
    if (value) {
      data.http = gor.setHttpHeader(data.http, header.name, maskValue(header.type));
    }
  });

  // Mask JSON fields
  const body = gor.httpBody(data.http);
  if (body) {
    try {
      const jsonBody = JSON.parse(body.toString());
      maskConfig.jsonPaths.forEach((field) => {
        const value = eval(`jsonBody${field.path.slice(1)}`);
        if (value) {
          eval(`jsonBody${field.path.slice(1)} = maskValue(field.type)`);
        }
      });
      data.http = gor.setHttpBody(data.http, Buffer.from(JSON.stringify(jsonBody)));
    } catch (error) {
      console.error("Error parsing JSON body:", error);
    }
  }

  return data;
});
```

### Configuration

The `maskConfig` object is used to configure the masking behavior. It consists of two properties:

- `headers`: An array of objects representing the headers to be masked. Each object should have the following properties:
  - `name`: The name of the header.
  - `type`: The type of data the header represents (e.g., "email", "name", "token").

- `jsonPaths`: An array of objects representing the JSON paths to be masked. Each object should have the following properties:
  - `path`: The JSON path to the field to be masked (e.g., "$.user.email").
  - `type`: The type of data the field represents (e.g., "email", "name", "phone", "address").

### Masking Function

The `maskValue` function is responsible for generating masked values based on the data type. It uses the Faker library to generate realistic-looking masked data for different types such as email, name, phone, address, and token. You can extend this function to support additional data types or customize the masking behavior.

### Middleware Function

The middleware function is triggered for each HTTP message (request or response) processed by GoReplay. It performs the following steps:

1. Iterate over the specified headers in the `maskConfig` and mask their values using the `maskValue` function based on the associated data type.

2. Parse the JSON body of the request (if present) and iterate over the specified JSON paths in the `maskConfig`. If a value exists at a given path, replace it with a masked value generated by the `maskValue` function based on the associated data type.

3. Update the request body with the masked JSON data.

4. Return the modified HTTP message.

Note: The middleware uses the `eval` function to dynamically access and modify the JSON object based on the provided paths. Exercise caution when using `eval` and ensure that the paths are properly validated to prevent potential security risks.

To use this middleware, make sure to install the required dependencies (`goreplay_middleware` and `faker`), configure the `maskConfig` object according to your needs, and run the middleware with GoReplay.


## Support

Feel free to ask questions here and by sending email to [support@goreplay.org](mailto:support@goreplay.org). Commercial support is available and welcomed 🙈.
