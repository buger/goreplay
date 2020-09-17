FROM alpine:latest as builder
RUN apk add --no-cache ca-certificates openssl
RUN wget https://github.com/buger/goreplay/releases/download/v1.2.0/gor_v1.2.0_x64.tar.gz -O gor.tar.gz
RUN tar xzf gor.tar.gz

FROM scratch
COPY --from=builder /gor .
ENTRYPOINT ["./gor"]
