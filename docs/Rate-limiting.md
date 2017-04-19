Rate limiting can be useful if you only want to forward parts of incoming traffic, for example, to not overload your test environment. There are two strategies: dropping random requests or dropping fractions of requests based on Header or URL param value. 

### Dropping random requests
Every input and output support random rate limiting.
There are two limiting algorithms: absolute or percentage based. 

**Absolute**: If for current second it reached specified requests limit - disregard the rest, on next second counter reset.

**Percentage**: For input-file it will slowdown or speedup request execution, for the rest it will use the random generator to decide if request pass or not based on the chance you specified. 

You can specify your desired limit using the "|" operator after the server address, see examples below.

#### Limiting replay using absolute number
```
# staging.server will not get more than ten requests per second
gor --input-tcp :28020 --output-http "http://staging.com|10"
```

#### Limiting listener using percentage based limiter
```
# replay server will not get more than 10% of requests 
# useful for high-load environments
gor --input-raw :80 --output-tcp "replay.local:28020|10%"
```

### Consistent limiting based on Header or URL param value
If you have unique user id (like API key) stored in header or URL you can consistently forward specified percent of traffic only for the fraction of this users. 
Basic formula looks like this: `FNV32-1A_hashing(value) % 100 >= chance`. Examples:
```
# Limit based on header value
gor --input-raw :80 --output-tcp "replay.local:28020|10%" --http-header-limiter "X-API-KEY: 10%"

# Limit based on header value
gor --input-raw :80 --output-tcp "replay.local:28020|10%" --http-param-limiter "api_key: 10%"
```

When limiting based on header or param only percentage based limiting supported.