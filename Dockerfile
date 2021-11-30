FROM alpine:3.12
COPY ./gor .
ENTRYPOINT ["./gor"]
