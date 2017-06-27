FROM alpine:latest
RUN apk update && apk add ca-certificates && update-ca-certificates && apk add openssl
RUN wget https://github.com/buger/goreplay/releases/download/v0.16.0.2/gor_0.16.0_x64.tar.gz -O gor.tar.gz
RUN tar xzf gor.tar.gz
ENTRYPOINT ./gor
