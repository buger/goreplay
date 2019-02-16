FROM alpine:latest as builder
RUN apk add --no-cache ca-certificates openssl
RUN wget https://github.com/buger/goreplay/releases/download/v0.16.1/gor_0.16.1_x64.tar.gz -O gor.tar.gz
RUN tar xzf gor.tar.gz

FROM scratch
COPY --from=builder /goreplay .
ENTRYPOINT ["./goreplay"]
