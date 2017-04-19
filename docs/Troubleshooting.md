Gor can report stats on the `output-tcp` and `output-http` request queues. Stats are reported to the console every 5 seconds in the form `latest,mean,max,count,count/second` by using the `--output-http-stats` and `--output-tcp-stats` options.

Examples:

```
2014/04/23 21:17:50 output_tcp:latest,mean,max,count,count/second
2014/04/23 21:17:50 output_tcp:0,0,0,0,0
2014/04/23 21:17:55 output_tcp:1,1,2,68,13
2014/04/23 21:18:00 output_tcp:1,1,2,92,18
2014/04/23 21:18:05 output_tcp:1,1,2,119,23
```

```
2014/04/23 21:19:46 output_http:latest,mean,max,count,count/second
2014/04/23 21:19:46 output_http:0,0,0,0,0
2014/04/23 21:19:51 output_http:0,0,0,0,0
2014/04/23 21:19:56 output_http:0,0,0,0,0
2014/04/23 21:20:01 output_http:1,0,1,50,10
2014/04/23 21:20:06 output_http:1,1,4,72,14
2014/04/23 21:20:11 output_http:1,0,1,179,35
2014/04/23 21:20:16 output_http:1,0,1,148,29
2014/04/23 21:20:21 output_http:1,1,2,91,18
2014/04/23 21:20:26 output_http:1,1,2,150,30
2014/04/23 21:18:15 output_http:100,99,100,70,14
2014/04/23 21:18:21 output_http:100,99,100,55,11
```

### How can I tell if I have bottlenecks?
Key areas that sometimes experience bottlenecks are the output-tcp and output-http functions which have internal queues for requests. Each queue has an upper limit of 100. Enable stats reporting to see if any queues are experiencing bottleneck behavior.
 
#### Output HTTP bottlenecks
When running a Gor replay the output-http feature may bottleneck if:

  * the replay has inadequate bandwidth. If the replay is receiving or sending more messages than its network adapter can handle the output-http-stats  may report that the output-http queue is filling up. See if there is a way to upgrade the replay's bandwidth.
  * with `--output-http-workers` set to anything other than `-1` the `-output-http` target is unable to respond to messages in a timely manner. The http output workers which take messages off the output-http queue, process the request, and ensure that the request did not result in an error may not be able to keep up with the number of incoming requests. If the replay is not using dynamic worker scaling (`--output-http-workers=-1`)  The optimal number of output-http-workers can be determined with the formula `output-workers = (Average number of requests per second)/(Average target response time per second)`.

#### Output TCP bottlenecks
When using the Gor listener the output-tcp feature may bottleneck if:

  * the replay is unable to accept and process more requests than the listener is able generate. Prior to troubleshooting the output-tcp bottleneck, ensure that the replay target is not experiencing any bottlenecks. 
  * the replay target has inadequate bandwidth to handle all its incoming requests.  If a replay target's incoming bandwidth is maxed out the output-tcp-stats may report that the output-tcp queue is filling up. See if there is a way to upgrade the replay's bandwidth.


#### Tuning

To achieve the top most performance you should tune the source server system limits:

    net.ipv4.tcp_max_tw_buckets = 65536
    net.ipv4.tcp_tw_recycle = 1
    net.ipv4.tcp_tw_reuse = 0
    net.ipv4.tcp_max_syn_backlog = 131072
    net.ipv4.tcp_syn_retries = 3
    net.ipv4.tcp_synack_retries = 3
    net.ipv4.tcp_retries1 = 3
    net.ipv4.tcp_retries2 = 8
    net.ipv4.tcp_rmem = 16384 174760 349520
    net.ipv4.tcp_wmem = 16384 131072 262144
    net.ipv4.tcp_mem = 262144 524288 1048576
    net.ipv4.tcp_max_orphans = 65536
    net.ipv4.tcp_fin_timeout = 10
    net.ipv4.tcp_low_latency = 1
    net.ipv4.tcp_syncookies = 0
***

### Gor is crashing with following stacktrace
```
fatal error: unexpected signal during runtime execution
[signal 0xb code=0x1 addr=0x63 pc=0x7ffcdfdf8b2c]

runtime stack:
runtime.throw(0xad8380, 0x2a)
	/usr/local/go/src/runtime/panic.go:547 +0x90
runtime.sigpanic()
	/usr/local/go/src/runtime/sigpanic_unix.go:12 +0x5a

goroutine 103 [syscall, locked to thread]:
runtime.cgocall(0x7b35a0, 0xc82121f1e8, 0x0)
	/usr/local/go/src/runtime/cgocall.go:123 +0x11b fp=0xc82121f188 sp=0xc82121f158
net._C2func_getaddrinfo(0x7ffcec0008c0, 0x0, 0xc821b221e0, 0xc8217b2b18, 0x0, 0x0, 0x0)
	??:0 +0x55 fp=0xc82121f1e8 sp=0xc82121f188
net.cgoLookupIPCNAME(0x7fffb17208ab, 0x12, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xb17200)
```

There is a chance that you hit Go bug. The crash comes from the CGO version of DNS resolver.
By default Go based version used, but ins some cases [it switches to CGO based](https://golang.org/pkg/net/#hdr-Name_Resolution). It is possible to force Go based DNS resolver using GODEBUG environment variable:
`sudo GODEBUG="netdns=go" ./gor --input-raw :80 --output-http staging.env`



Also, see [[FAQ]]