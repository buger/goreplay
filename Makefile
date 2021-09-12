SOURCE = $(shell ls -1 *.go | grep -v _test.go)
SOURCE_PATH = /go/src/github.com/buger/goreplay/
PORT = 8000
FADDR = :8000
CONTAINER=gor
PREFIX=
RUN = docker run --rm -v `pwd`:$(SOURCE_PATH) -e AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) -e AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) -p 0.0.0.0:$(PORT):$(PORT) -t -i $(CONTAINER)
BENCHMARK = BenchmarkRAWInput
TEST = TestRawListenerBench
BIN_NAME = gor
VERSION = DEV-$(shell date +%s)
LDFLAGS = -ldflags "-X main.VERSION=$(VERSION)$(PREFIX) -extldflags \"-static\" -X main.DEMO=$(DEMO)"
MAC_LDFLAGS = -ldflags "-X main.VERSION=$(VERSION)$(PREFIX) -X main.DEMO=$(DEMO)"

FPMCOMMON= \
    --name goreplay \
    --description "GoReplay is an open-source network monitoring tool which can record your live traffic, and use it for shadowing, load testing, monitoring and detailed analysis." \
    -v $(VERSION) \
    --vendor "Leonid Bugaev" \
    -m "<support@goreplay.org>" \
    --url "https://goreplay.org" \
    -s dir \
    -C /tmp/gor-build \

.PHONY: vendor

release: release-x64 release-x86 release-mac release-windows

vendor:
	go mod vendor

release-bin: vendor
	docker run --rm -v `pwd`:$(SOURCE_PATH) -t --env GOOS=linux --env GOARCH=amd64  -i $(CONTAINER) go build -mod=vendor -o $(BIN_NAME) -tags netgo $(LDFLAGS)

release-bin-x86: vendor
	docker run --rm -v `pwd`:$(SOURCE_PATH) -t --env GOOS=linux --env GOARCH=386 -i $(CONTAINER) go build -mod=vendor -o $(BIN_NAME) -tags netgo $(LDFLAGS)

release-bin-mac: vendor
	GOOS=darwin go build -mod=vendor -o $(BIN_NAME) $(MAC_LDFLAGS)

release-bin-windows: vendor
	docker run -it --rm -v `pwd`:$(SOURCE_PATH) -w $(SOURCE_PATH) -e CGO_ENABLED=1 docker.elastic.co/beats-dev/golang-crossbuild:1.16.4-main --build-cmd "make VERSION=$(VERSION) build" -p "windows/amd64"

release-x64: release-bin
	tar -czf gor_$(VERSION)$(PREFIX)_x64.tar.gz $(BIN_NAME)
	mkdir -p /tmp/gor-build
	mv ./$(BIN_NAME) /tmp/gor-build/$(BIN_NAME)
	cd /tmp/gor-build
	rm -f goreplay_$(VERSION)_amd64.deb
	rm -f goreplay-$(VERSION)-1.x86_64.rpm
	fpm $(FPMCOMMON) -a amd64 -t deb ./=/usr/local/bin
	fpm $(FPMCOMMON) -a amd64 -t rpm ./=/usr/local/bin
	rm -rf /tmp/gor-build

release-x86: release-bin-x86
	tar -czf gor_$(VERSION)$(PREFIX)_x86.tar.gz $(BIN_NAME)
	rm $(BIN_NAME)

release-mac: release-bin-mac
	tar -czf gor_$(VERSION)$(PREFIX)_mac.tar.gz $(BIN_NAME)
	mkdir -p /tmp/gor-build
	mv ./$(BIN_NAME) /tmp/gor-build/$(BIN_NAME)
	cd /tmp/gor-build
	rm -f goreplay-$(VERSION).pkg
	fpm $(FPMCOMMON) -a amd64 -t osxpkg ./=/usr/local/bin
	rm -rf /tmp/gor-build

release-windows: release-bin-windows
	mv ./gor ./gor.exe
	zip gor-$(VERSION)$(PREFIX)_windows.zip ./gor.exe
	rm -rf ./gor.exe

build:
	go build -mod=vendor -o $(BIN_NAME) $(LDFLAGS)

install:
	go install $(MAC_LDFLAGS)

build-env:
	docker build -t $(CONTAINER) -f Dockerfile.dev .

profile:
	go build && ./$(BIN_NAME) --output-http="http://localhost:9000" --input-dummy 0 --input-raw :9000 --input-http :9000 --memprofile=./mem.out --cpuprofile=./cpu.out --stats --output-http-stats --output-http-timeout 100ms

lint:
	$(RUN) golint $(PKG)

race:
	$(RUN) go test ./... $(ARGS) -v -race -timeout 15s

test:
	$(RUN) go test ./. -timeout 120s $(LDFLAGS) $(ARGS)  -v

test_all:
	$(RUN) go test ./... -timeout 120s $(LDFLAGS) $(ARGS) -v

testone:
	$(RUN) go test ./. -timeout 60s $(LDFLAGS) -run $(TEST) $(ARGS) -v

cover:
	$(RUN) go test $(ARGS) -race -v -timeout 15s -coverprofile=coverage.out
	go tool cover -html=coverage.out

fmt:
	$(RUN) gofmt -w -s ./..

vet:
	$(RUN) go vet

bench:
	$(RUN) go test $(LDFLAGS) -v -run NOT_EXISTING -bench $(BENCHMARK) -benchtime 5s

profile_test:
	$(RUN) go test $(LDFLAGS) -run $(TEST) ./capture/. $(ARGS) -memprofile mem.mprof -cpuprofile cpu.out
	$(RUN) go test $(LDFLAGS) -run $(TEST) ./capture/. $(ARGS) -c

# Used mainly for debugging, because docker container do not have access to parent machine ports
run:
	$(RUN) go run $(LDFLAGS) $(SOURCE) --input-dummy=0 --output-http="http://localhost:9000" --input-raw-track-response --input-raw 127.0.0.1:9000 --verbose 0 --middleware "./examples/middleware/echo.sh" --output-file requests.gor

run-2:
	$(RUN) go run $(LDFLAGS) $(SOURCE) --input-raw :8000 --input-raw-bpf-filter "dst port 8000" --output-stdout --output-http "http://localhost:8000" --input-dummy=0

run-3:
	sudo -E go run $(SOURCE) --input-tcp :27001 --output-stdout

run-arg:
	sudo -E go run $(SOURCE) $(ARGS)

file-server:
	go run $(SOURCE) file-server $(FADDR)

readpcap:
	go run $(SOURCE) --input-raw $(FILE) --input-raw-track-response --input-raw-engine pcap_file --output-stdout

record:
	$(RUN) go run $(SOURCE) --input-dummy=0 --output-file=requests.gor --verbose --debug

replay:
	$(RUN) go run $(SOURCE) --input-file=requests.bin --output-tcp=:9000 --verbose -h

bash:
	$(RUN) /bin/bash


FPMCOMMON= \
    --name gor \
    --description "GoReplay is an open-source network monitoring tool which can record your live traffic, and use it for shadowing, load testing, monitoring and detailed analysis." \
    -v $(VERSION) \
    --vendor "Leonid Bugaev" \
    -m "<support@goreplay.org>" \
    --url "https://goreplay.org" \
    -s dir \
    -C /tmp/gor-build \

build_packages:
	mkdir -p /tmp/gor-build
	go build -i -o /tmp/gor-build/$(BIN_NAME)
	fpm $(FPMCOMMON) -a amd64 -t deb ./=/usr/local/bin
	fpm $(FPMCOMMON) -a amd64 -t rpm ./=/usr/local/bin
