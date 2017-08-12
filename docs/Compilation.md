We provide pre-compiled binaries for Mac and Linux, but you are free to compile Gor by yourself.

GoReplay is written using Go, so first you need to download it from here https://golang.org/, use the latest stable version.

The only GoReplay dependency is [libpcap](https://github.com/the-tcpdump-group/libpcap), which is the interface to various kernel packet capture mechanisms, and https://github.com/google/gopacket, which is a Go wrapper around libpcap. Latest libpcap version can be obtained at http://www.tcpdump.org/release/. Libpcap itself depend on `flex` and `bison` packages, many operating systems already have them installed.

```bash
# Fetch libpcap dependencies. Depending on your OS, instead of `apt` you will use `yum` or `rpm`, or `brew` on Mac.
sudo apt-get install flex bison -y

# Download latest stable release, compile and install it
wget http://www.tcpdump.org/release/libpcap-1.7.4.tar.gz && tar xzf libpcap-1.7.4.tar.gz
cd libpcap-1.7.4
./configure && make install

# Lets fetch GoReplay source code
mkdir $HOME/gocode
# See more information about GOPATH https://github.com/golang/go/wiki/GOPATH
export GOPATH=$HOME/gocode
# Fetch code from the Github
go get github.com/buger/goreplay

# Compile from source
cd $HOME/gocode/src/github.com/buger/goreplay
go build LDFLAGS = -ldflags "-extldflags \"-static\""
```

After you finished, you should see `goreplay` binary in current directory.

