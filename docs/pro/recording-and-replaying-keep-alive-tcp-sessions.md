> **This feature available only in PRO version. See https://goreplay.org/pro.html for details.**

By default, GoReplay does not guarantee that when you record keep-alive TCP session, it will be replayed in the same TCP connection as well. This is ok for most of the cases, but it does not give an accurate number of TCP sessions while replaying, also may cause issues if your application state depends on TCP session (do not mess with HTTP session).

[GoReplay PRO](https://goreplay.org/pro.html) extension adds support for accurate recording and replaying of keep-alive TCP sessions. Separate connection to your server is created per original session and it makes benchmarks and tests incredibly accurate. To enable session recognition you just need to pass `--recognize-tcp-sessions` option. 

```
gor --input-raw :80 --recognize-tcp-sessions --output-http http://test.target
```

Note that enabling this option also change algorithm of distributing traffic when using `--split-output`, see [Distributed configuration].