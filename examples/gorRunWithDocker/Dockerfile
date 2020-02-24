FROM ubuntu:18.04
ADD goreplay /go/
WORKDIR /go
RUN mkdir logs
RUN mkdir records
USER root
ENV INPUT="--input-raw :5000"
ENV OUTPUT="--output-stdout"
CMD ./goreplay ${INPUT} ${OUTPUT}