/*
Package capture provides traffic sniffier using AF_PACKET, pcap or pcap file.
it allows you to listen for traffic from any port (e.g. sniffing) because they operate on IP level.
Ports is TCP/IP feature, same as flow control, reliable transmission and etc.
Currently this package implements TCP layer: flow control is managed under tcp package.
BPF filters can also be applied.

example:

// for the transport should be "tcp"
listener, err := capture.NewListener(host, port, transport, engine, trackResponse)
if err != nil {
	// handle error
}
listener.SetPcapOptions(opts)
err = listner.Activate()
if err != nil {
	// handle it
}

if err := listener.Listen(context.Background(), handler); err != nil {
	 // handle error
}
// or
errCh := listener.ListenBackground(context.Background(), handler) // runs in the background
select {
case err := <- errCh:
	// handle error
case <-quit:
	//
case <- l.Reading: // if we have started reading
}

*/
package capture // import github.com/buger/goreplay/capture
