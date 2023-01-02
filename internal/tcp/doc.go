/*
Package tcp implements TCP transport layer protocol, it is responsible for
parsing, reassembling tcp packets, handling communication with engine listeners(github.com/buger/goreplay/capture),
and reporting errors and statistics of packets.
the packets are parsed by following TCP way(https://en.wikipedia.org/wiki/Transmission_Control_Protocol#TCP_segment_structure).

example:

import "github.com/buger/goreplay/tcp"

messageExpire := time.Second*5
maxSize := 5 << 20

debugger := func(debugLevel int, data ...interface{}){} // debugger can also be nil
messageHandler := func(mssg *tcp.Message){}

mssgPool := tcp.NewMessageParser(maxMessageSize, messageExpire, debugger, messageHandler)
listener.Listen(ctx, mssgPool.Handler)

you can use pool.End or/and pool.Start to set custom session behaviors

debugLevel in debugger function indicates the priority of the logs, the bigger the number the lower
the priority. errors are signified by debug level 4 for errors, 5 for discarded packets, and 6 for received packets.
*/
package tcp // import github.com/buger/goreplay/tcp
