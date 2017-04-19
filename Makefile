SOURCE = emitter.go gor.go gor_stat.go input_dummy.go input_file.go input_raw.go input_tcp.go limiter.go output_dummy.go output_null.go output_file.go input_http.go output_http.go output_tcp.go plugins.go settings.go test_input.go elasticsearch.go http_modifier.go http_modifier_settings.go http_client.go middleware.go protocol.go output_file_settings.go output_kafka.go
SOURCE_PATH = /go/src/github.com/buger/gor/
PORT = 8000
FADDR = :8000
RUN = docker run -v `pwd`:$(SOURCE_PATH) -p 0.0.0.0:$(PORT):$(PORT) -i -t gor
BENCHMARK = BenchmarkRAWInput
TEST = TestRawListenerBench
VERSION = DEV-$(shell date +%s)
LDFLAGS = -ldflags "-X main.VERSION=$(VERSION) -extldflags \"-static\""
MAC_LDFLAGS = -ldflags "-X main.VERSION=$(VERSION)"
FADDR = ":8000"

release: release-x64 release-mac

release-bin:
	docker run -v `pwd`:$(SOURCE_PATH) -t --env GOOS=linux --env GOARCH=amd64  -i gor go build -tags netgo $(LDFLAGS)

release-x64:
	docker run -v `pwd`:$(SOURCE_PATH) -t --env GOOS=linux --env GOARCH=amd64  -i gor go build -tags netgo $(LDFLAGS) && tar -czf gor_$(VERSION)_x64.tar.gz gor && rm gor

release-x86:
	docker run -v `pwd`:$(SOURCE_PATH) -t --env GOOS=linux --env GOARCH=386 -i gor go build -tags netgo $(LDFLAGS) && tar -czf gor_$(VERSION)_x86.tar.gz gor && rm gor

release-mac:
	go build $(MAC_LDFLAGS) && tar -czf gor_$(VERSION)_mac.tar.gz gor && rm gor

install:
	go install $(MAC_LDFLAGS)

build:
	docker build -t gor .


profile:
	go build && ./gor --output-http="http://localhost:9000" --input-dummy 0 --input-raw :9000 --input-http :9000 --memprofile=./mem.out --cpuprofile=./cpu.out --stats --output-http-stats --output-http-timeout 100ms

lint:
	$(RUN) golint $(PKG)

race:
	$(RUN) go test ./... $(ARGS) -v -race -timeout 15s

test:
	$(RUN) go test ./. -timeout 60s $(LDFLAGS) $(ARGS)  -v

test_all:
	$(RUN) go test ./... -timeout 60s $(LDFLAGS) $(ARGS) -v

testone:
	$(RUN) go test ./... -timeout 4s $(LDFLAGS) -run $(TEST) $(ARGS) -v

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
	$(RUN) go test $(LDFLAGS) -run $(TEST) ./raw_socket_listener/. $(ARGS) -memprofile mem.mprof -cpuprofile cpu.out
	$(RUN) go test $(LDFLAGS) -run $(TEST) ./raw_socket_listener/. $(ARGS) -c

# Used mainly for debugging, because docker container do not have access to parent machine ports
run:
	$(RUN) go run $(LDFLAGS) $(SOURCE) --input-dummy=0 --output-http="http://localhost:9000" --input-raw-track-response --input-raw 127.0.0.1:9000 --verbose --debug --middleware "./examples/middleware/echo.sh" --output-file requests.gor

run-2:
	sudo -E go run $(SOURCE) --input-dummy="" --output-tcp localhost:27001 --verbose --debug

run-3:
	sudo -E go run $(SOURCE) --input-tcp :27001 --output-stdout

run-arg:
	sudo -E go run $(SOURCE) $(ARGS)

file-server:
	go run $(SOURCE) file-server $(FADDR)

readpcap:
	go run $(SOURCE) --input-raw $(FILE) --input-raw-engine pcap_file --output-null

record:
	$(RUN) go run $(SOURCE) --input-dummy=0 --output-file=requests.gor --verbose --debug

replay:
	$(RUN) go run $(SOURCE) --input-file=requests.bin --output-tcp=:9000 --verbose -h

bash:
	$(RUN) /bin/bash
