> **This feature available only in PRO version. See https://goreplay.org/pro.html for details.**

Gor includes basic support for working with binary formats like `thrift` or `protocol-buffers`. To start set `--input-raw-protocol` to 'binary' (by default 'http'). For replaying, you should use `--output-binary`, example:

```
gor --input-raw :80 --input-raw-protocol binary --output-binary staging:8081
```

While working with `--input-raw` you may notice a 2-second delay before messages are emitted to the outputs. This behaviour is expected and happening because for general binary protocol it is impossible to know when TCP message ends, so Gor has to set inactivity timeout. Each protocol has own rules (for example write message length as first bytes), and require individual handling to know message length. We consider improving detailed protocol support for `thrift`, `protocol-buffer` and etc.

Note that you can use all load testing features for binary protocols. For example, the following command will loop and replay recorded payload on 10x speed for 30 seconds:
```
gor --input-file './binary*.gor|1000%' --output-binary staging:9091 --input-file-loop --exit-after 30s
```