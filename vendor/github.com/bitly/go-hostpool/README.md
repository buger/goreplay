go-hostpool
===========

[![Build Status](https://secure.travis-ci.org/bitly/go-hostpool.png?branch=master)](http://travis-ci.org/bitly/go-hostpool) [![GoDoc](https://godoc.org/github.com/bitly/go-hostpool?status.svg)](https://godoc.org/github.com/bitly/go-hostpool) [![GitHub release](https://img.shields.io/github/release/bitly/go-hostpool.svg)](https://github.com/bitly/go-hostpool/releases/latest)


A Go package to intelligently and flexibly pool among multiple hosts from your Go application.
Host selection can operate in round robin or epsilon greedy mode, and unresponsive hosts are
avoided.
Usage example:

```go
hp := hostpool.NewEpsilonGreedy([]string{"a", "b"}, 0, &hostpool.LinearEpsilonValueCalculator{})
hostResponse := hp.Get()
hostname := hostResponse.Host()
err := _ // (make a request with hostname)
hostResponse.Mark(err)
```

View more detailed documentation on [godoc.org](http://godoc.org/github.com/bitly/go-hostpool)
