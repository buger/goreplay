Filtering is useful when you need to capture only specific part of traffic, like API requests. It is possible to filter by URL, HTTP header or HTTP method.

#### Allow url regexp
```
# only forward requests being sent to the /api endpoint
gor --input-raw :8080 --output-http staging.com --http-allow-url /api
```

#### Disallow url regexp
```
# only forward requests NOT being sent to the /api... endpoint
gor --input-raw :8080 --output-http staging.com --http-disallow-url /api
```
#### Filter based on regexp of header

```
# only forward requests with an api version of 1.0x
gor --input-raw :8080 --output-http staging.com --http-allow-header api-version:^1\.0\d

# only forward requests NOT containing User-Agent header value "Replayed by Gor"
gor --input-raw :8080 --output-http staging.com --http-disallow-header "User-Agent: Replayed by Gor"
```

#### Filter based on HTTP method
Requests not matching a specified whitelist can be filtered out. For example to strip non-nullipotent requests:

```
gor --input-raw :80 --output-http "http://staging.server" \
    --http-allow-method GET \
    --http-allow-method OPTIONS
```


-----
You may also read about [[Request rewriting]], [[Rate limiting]] and [[Middleware]]