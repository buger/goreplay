[![Build Status](https://travis-ci.org/buger/gor.png?branch=master)](https://travis-ci.org/buger/gor)

## About

Gor is a simple http traffic replication tool written in Go.
Its main goal is to replay traffic from production servers to staging and dev environments.


Now you can test your code on real user sessions in an automated and repeatable fashion.
**No more falling down in production!**

Gor consists of 2 parts: listener and replay servers.

The listener server catches http traffic from a given port in real-time
and sends it to the replay server or saves to file.
The replay server forwards traffic to a given address.


![Diagram](http://i.imgur.com/9mqj2SK.png)


## Basic example

```bash
# Run on servers where you want to catch traffic. You can run it on each `web` machine.
sudo gor listen -p 80 -r replay.server.local:28020

# Replay server (replay.server.local).
gor replay -f http://staging.server -p 28020
```

## Advanced use

### Rate limiting
Both replay and listener supports rate limiting. It can be useful if you want
forward only part of production traffic and not overload your staging
environment. You can specify your desired requests per second using the
"|" operator after the server address:

```
# staging.server will not get more than 10 requests per second
gor replay -f "http://staging.server|10"
```

```
# replay server will not get more than 10 requests per second
# useful for high-load environments
gor listen -p 8080 -r "replay.server.local:28020|10"
```

### Forward to multiple addresses

You can forward traffic to multiple endpoints. Just separate the addresses by comma.
```
gor replay -f "http://staging.server|10,http://dev.server|5"
```
### Saving requests to file
You can save request to save to file to replay multiple time, or in different network:
```
gor listen -p 8080 -file requests.gor
```

And replaying:
```
gor replay -f "http://staging.server" -file requests.gor
```

## Additional help
```
$ gor listen -h
Usage of ./bin/gor-linux:
  -i="any": By default it try to listen on all network interfaces.To get list of interfaces run `ifconfig`
  -p=80: Specify the http server port whose traffic you want to capture
  -r="localhost:28020": Address of replay server.
```

```
$ gor replay -h
Usage of ./bin/gor-linux:
  -f="http://localhost:8080": http address to forward traffic.
	You can limit requests per second by adding `|#{num}` after address.
	If you have multiple addresses with different limits. For example: http://staging.example.com|100,http://dev.example.com|10
  -ip="0.0.0.0": ip addresses to listen on
  -p=28020: specify port number
```

## Latest releases (including binaries)

https://github.com/buger/gor/releases

## Building from source
1. Setup standard Go environment http://golang.org/doc/code.html and ensure that $GOPATH environment variable properly set.
2. `go get github.com/buger/gor`.
3. `cd $GOPATH/src/github.com/buger/gor`
4. `go build gor.go` to get binary, or `go run gor.go` to build and run (useful for development)

## FAQ

### What OS are supported?
For now only Linux based. *BSD (including MacOS is not supported yet, check https://github.com/buger/gor/issues/22 for details)

### Why does the `gor listener` requires sudo or root access?
Listener works by sniffing traffic from a given port. It's accessible
only by using sudo or root access.

### Do you support all http request types?
Yes. ~~Right now it supports only "GET" requests.~~

## Contributing

1. Fork it
2. Create your feature branch (git checkout -b my-new-feature)
3. Commit your changes (git commit -am 'Added some feature')
4. Push to the branch (git push origin my-new-feature)
5. Create new Pull Request

## Companies using Gor

* http://granify.com
* To add your company drop me a line to github.com/buger or leonsbox@gmail.com
