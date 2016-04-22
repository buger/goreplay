FROM golang:latest

RUN apt-get update && apt-get install ruby vim-common -y

# Install Java for middleware testing
RUN echo "deb http://ppa.launchpad.net/webupd8team/java/ubuntu trusty main" | tee /etc/apt/sources.list.d/webupd8team-java.list
RUN echo "deb-src http://ppa.launchpad.net/webupd8team/java/ubuntu trusty main" | tee -a /etc/apt/sources.list.d/webupd8team-java.list
RUN apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys EEA14886
RUN apt-get update -y
RUN echo oracle-java7-installer shared/accepted-oracle-license-v1-1 select true | /usr/bin/debconf-set-selections
RUN apt-get install oracle-java8-installer -y

RUN apt-get install flex bison -y
RUN wget http://www.tcpdump.org/release/libpcap-1.7.4.tar.gz && tar xzf libpcap-1.7.4.tar.gz && cd libpcap-1.7.4 && ./configure && make install
RUN go get github.com/google/gopacket
RUN go get -u github.com/golang/lint/golint

WORKDIR /go/src/github.com/buger/gor/
ADD . /go/src/github.com/buger/gor/

RUN javac -cp /tmp/commons-io-2.4/commons-io-2.4.jar ./examples/middleware/echo.java
RUN go get
