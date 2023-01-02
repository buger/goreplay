SOURCE = $(shell ls -1 *.go | grep -v _test.go)
PROJECT_NAME := goreplay
SOURCE_PATH = /go/src/github.com/buger/goreplay/
PORT = 8000
FADDR = :8000
DIST_PATH = dist
CONTAINER_AMD=gor-amd64
CONTAINER_ARM=gor-arm64
RUN = docker run --rm -v `pwd`:$(SOURCE_PATH) -e AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) -e AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) -p 0.0.0.0:$(PORT):$(PORT) -t -i $(CONTAINER_AMD)
BENCHMARK = BenchmarkRAWInput
TEST = TestRawListenerBench
BIN_NAME = gor
VERSION := DEV-$(shell date +%s)
LDFLAGS = -ldflags "-X main.VERSION=$(VERSION) -extldflags \"-static\" -X main.DEMO=$(DEMO)"
MAC_LDFLAGS = -ldflags "-X main.VERSION=$(VERSION) -X main.DEMO=$(DEMO)"
DOCKER_FPM_CMD := docker run --rm -t -v `pwd`:/src -w /src fleetdm/fpm

FPM_COMMON= \
    --name $(PROJECT_NAME) \
    --description "GoReplay is an open-source network monitoring tool which can record your live traffic, and use it for shadowing, load testing, monitoring and detailed analysis." \
    -v $(VERSION) \
    --vendor "Leonid Bugaev" \
    -m "<support@goreplay.org>" \
    --url "https://goreplay.org" \
    -s dir

release: clean release-linux-amd64 release-linux-arm64 release-mac-amd64 release-mac-arm64 release-windows

.PHONY: vendor
vendor:
	go mod vendor

release-bin-linux-amd64: vendor
	docker run --platform linux/amd64 --rm -v `pwd`:$(SOURCE_PATH) -t --env GOOS=linux --env GOARCH=amd64 -i $(CONTAINER_AMD) go build -mod=vendor -o $(BIN_NAME) -tags netgo $(LDFLAGS) ./cmd/gor/

release-bin-linux-arm64: vendor
	docker run --platform linux/arm64 --rm -v `pwd`:$(SOURCE_PATH) -t --env GOOS=linux --env GOARCH=arm64 -i $(CONTAINER_ARM) go build -mod=vendor -o $(BIN_NAME) -tags netgo $(LDFLAGS) ./cmd/gor/

release-bin-mac-amd64: vendor
	GOOS=darwin go build -mod=vendor -o $(BIN_NAME) $(MAC_LDFLAGS) ./cmd/gor/

release-bin-mac-arm64: vendor
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -mod=vendor -o $(BIN_NAME) $(MAC_LDFLAGS)

release-bin-windows: vendor
	docker run -it --rm -v `pwd`:$(SOURCE_PATH) -w $(SOURCE_PATH) -e CGO_ENABLED=1 docker.elastic.co/beats-dev/golang-crossbuild:1.19.2-main --build-cmd "make VERSION=$(VERSION) build" -p "windows/amd64" ./cmd/gor/
	mv $(BIN_NAME) "$(BIN_NAME).exe"

release-linux-amd64: dist release-bin-linux-amd64
	tar -czf $(DIST_PATH)/gor_$(VERSION)_linux_amd64.tar.gz $(BIN_NAME)
	$(DOCKER_FPM_CMD) $(FPM_COMMON) -f -t deb -a arm64 -p ./$(DIST_PATH) ./gor=/usr/local/bin
	$(DOCKER_FPM_CMD) $(FPM_COMMON) -f -t rpm -a arm64 -p ./$(DIST_PATH) ./gor=/usr/local/bin
	rm -rf $(BIN_NAME)

release-linux-arm64: dist release-bin-linux-arm64
	tar -czf $(DIST_PATH)/gor_$(VERSION)_linux_arm64.tar.gz $(BIN_NAME)
	$(DOCKER_FPM_CMD) $(FPM_COMMON) -f -t deb -a arm64 -p ./$(DIST_PATH) ./gor=/usr/local/bin
	$(DOCKER_FPM_CMD) $(FPM_COMMON) -f -t rpm -a arm64 -p ./$(DIST_PATH) ./gor=/usr/local/bin
	rm -rf $(BIN_NAME)

release-mac-amd64: dist release-bin-mac-amd64
	tar -czf $(DIST_PATH)/gor_$(VERSION)_darwin_amd64.tar.gz $(BIN_NAME)
	fpm $(FPM_COMMON) -f -t osxpkg -a amd64 -p ./$(DIST_PATH) ./gor=/usr/local/bin
	mv ./$(DIST_PATH)/$(PROJECT_NAME)-$(VERSION).pkg ./$(DIST_PATH)/$(PROJECT_NAME)-$(VERSION)-amd64.pkg
	rm -rf $(BIN_NAME)

release-mac-arm64: dist release-bin-mac-arm64
	tar -czf $(DIST_PATH)/gor_$(VERSION)_darwin_arm64.tar.gz $(BIN_NAME)
	fpm $(FPM_COMMON) -f -t osxpkg -a arm64 -p ./$(DIST_PATH) ./gor=/usr/local/bin
	mv ./$(DIST_PATH)/$(PROJECT_NAME)-$(VERSION).pkg ./$(DIST_PATH)/$(PROJECT_NAME)-$(VERSION)-arm64.pkg
	rm -rf $(BIN_NAME)

release-windows: dist release-bin-windows
	zip $(DIST_PATH)/gor-$(VERSION)_windows.zip "$(BIN_NAME).exe"
	rm -rf "$(BIN_NAME).exe"

clean:
	rm -rf $(DIST_PATH)

build:
	go build -mod=vendor -o $(BIN_NAME) $(LDFLAGS)

install:
	go install $(MAC_LDFLAGS)

build-env: build-amd64-env build-arm64-env

build-amd64-env:
	docker buildx build --platform linux/amd64 -t $(CONTAINER_AMD) -f Dockerfile.dev .

build-arm64-env:
	docker buildx build --platform linux/arm64 -t $(CONTAINER_ARM) -f Dockerfile.dev .

build-docker:
	docker build -t gor-dev -f Dockerfile .

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

dist:
	mkdir -p $(DIST_PATH)