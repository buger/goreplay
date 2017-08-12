Think about GoReplay more like a network analyzer or tcpdump on steroids, it is not a proxy and does not affect your app anyhow. You specify application port, and it will capture and replay incoming data.

Simplest setup will be:
```bash
# Run on servers where you want to catch traffic. You can run it on every `web` machine.
sudo goreplay --input-raw :80 --output-http http://staging.com
```
It will record and replay traffic from the same machine. However, it is possible to use [[Aggregator-forwarder setup]], when GoReplay on your web machines forward traffic to GoReplay aggregator instance running on the separate server.

> You may notice that it require `sudo`: to analyze network GoReplay need permissions which available only to root users. However, it is possible to configure GoReplay [beign run for non-root users](Running-as-non-root-user.md).

### Forwarding to multiple addresses

You can forward traffic to multiple endpoints.
```
goreplay --input-tcp :28020 --output-http "http://staging.com"  --output-http "http://dev.com"
```

### Splitting traffic

By default, it will send same traffic to all outputs, but you have options to equally split it (round-robin) using  `--split-output` option.

```
goreplay --input-raw :80 --output-http "http://staging.com"  --output-http "http://dev.com" --split-output true
```

### Tracking responses

By default `input-raw` does not intercept responses, only requests. You can turn response tracking using `--input-raw-track-response` option. When enable you will be able to access response information in middleware and `output-file`.

### Traffic interception engine

By default, GoReplay will use `libpcap` for intercepting traffic, it should work in most cases. If you have any troubles with it, you may try alternative engine: `raw_socket`.

```
sudo goreplay --input-raw :80 --input-raw-engine "raw_socket" --output-http "http://staging.com"
```

You can read more about [Replaying HTTP traffic](Replaying-HTTP-traffic.md).

### Tracking original IP addresses

You can use `--input-raw-realip-header` option to specify header name: If not blank, injects header with given name and real IP value to the request payload. Usually, this header should be named: `X-Real-IP`, but you can specify any name.

`goreplay --input-raw :80 --input-raw-realip-header "X-Real-IP" ...`

***

Also you may want to know about [Rate limiting](Rate-limiting.md), [Request rewriting](Request-rewriting.md) and [Request filtering](Request-filtering.md).
