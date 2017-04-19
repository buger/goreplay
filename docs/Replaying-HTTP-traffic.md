Gor can replay HTTP traffic using `--output-http` option:

```bash
sudo ./gor --input-raw :8000 --output-http="http://staging.env"
```

You can [filter](Request filtering), [rate limit](Rate limiting) and [rewrite](Request rewriting) requests on the fly. 

### HTTP output workers
By default Gor creates a dynamic pool of workers: it starts with 10 and creates more HTTP output workers when the HTTP output queue length is greater than 10.  The number of workers created (N) is equal to the queue length at the time which it is checked and found to have a length greater than 10. The queue length is checked every time a message is written to the HTTP output queue.  No more workers will be spawned until that request to spawn N workers is satisfied.  If a dynamic worker cannot process a message at that time, it will sleep for 100 milliseconds. If a dynamic worker cannot process a message for 2 seconds it dies.
You may specify fixed number of workers using  `--output-http-workers=20` option.

### Following redirects
By default Gor will ignore all redirects since they are handled by clients using your app, but in scenarios where your replayed environment introduces new redirects, you can enable them like this: 
```
gor --input-tcp replay.local:28020 --output-http http://staging.com --output-http-redirects 2
```
The given example will follow up to 2 redirects per request.

### HTTP timeouts
By default http timeout for both request and response is 5 seconds. You can override it like this:
```
gor --input-tcp replay.local:28020 --output-http http://staging.com --output-http-timeout 30s
```

### Response buffer
By default, to reduce memory consumption, internal HTTP client will fetch max 200kb of the response body (used if you use middleware), by you can increase limit using `--output-http-response-buffer` option (accepts number of bytes).

### Basic Auth

If your development or staging environment is protected by Basic Authentication then those credentials can be injected in during the replay:

```
gor --input-raw :80 --output-http "http://user:pass@staging.com"
```

Note: This will overwrite any Authorization headers in the original request.


### Multiple domains support

If you app accepts traffic from multiple domains, and you want to keep original headers, there is specific `--http-original-host` with tells Gor do not touch Host header at all.


***
You may also read about [[Saving and Replaying from file]]