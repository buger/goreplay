v0.3.5 - 15 Sep 2013
* Significantly improved test coverage
* Fixed bug with redirect replay https://github.com/buger/gor/pull/15
* Added limit on listener side
* Improved stability (catch and log panic, instead of exiting)
* Added License file

v0.3.3 - 22 Jun 2013
* Using TCP instead of UDP for communication between Listener and Replay
* Significantly improved performance
* Fixed bugs causing locking and message dropping (concurrency issues)
* Rewrote concurrency model to use more channels

v0.3 - 10 Jun 2013
* Use RAW_SOCKETS instead of tcpdump
* Own TCP stack
* All HTTP request types support
* Simplified request parsing
