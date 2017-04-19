### What OS are supported?
Gor will run everywhere where [libpcap](http://www.tcpdump.org/) works, and it works on most of the platforms. However, currently, we test it on Linux and Mac. See more about [[Compilation]].

### Why does the `--input-raw` requires sudo or root access?
Listener works by sniffing traffic from a given port. It's accessible
only by using sudo or root access. But it is possible to [[Running as non root user]].

### How do you deal with user session to replay the traffic correctly?
You can rewrite session related headers/params to match your staging environment. If you require custom logic (e.g random token based auth) follow this discussion: https://github.com/buger/gor/issues/154

### Can I use Gor to intercept SSL traffic?
Basic idea is that SSL was made to protect itself from traffic interception. There 2 options: 
1. Move SSL handling to proxy like Nginx or Amazon ELB. And allow Gor to listen on upstreams. 
2. Use `--input-http` so you can duplicate request payload directly from your app to Gor, but it will require your app modifications.

More can be find here: https://github.com/buger/gor/issues/85

### Is there a limit for size of HTTP request when using output-http?
Due to the fact that Gor can't guarantee interception of all packets, for large payloads > 200kb there is chance of missing some packets and corrupting body. Treat it as a feature and chance to test broken bodies handling :)
The only way to guarantee delivery is using `--input-http`, but you will miss some features.

### I'm getting 'too many open files' error
Typical Linux shell has a small open files soft limit at 1024. You can easily raise that when you do this before starting your gor replay process:
  
  ulimit -n 64000

More about ulimit: http://www.thecodingmachine.com/solving-the-too-many-open-files-exception-in-red5-or-any-other-application/

### The CPU average across my load-balanced targets is higher than the source
If you are replaying traffic from multiple listeners to a load-balanced target and you use sticky sessions, you may observe that the target servers have a higher CPU load than the listener servers. This may be because the sticky session cookie of the original load balancer is not honored by the target load balancer thus resulting in requests that would normally hit the same target server hitting different servers on the backend thus reducing some caching benefits gained via the load balancing.  Try running just one listener against one replay target and see if the CPU utilization comparison is more accurate.

Also see [[Troubleshooting]].