all:
	go get -d && go build

run:
	go run gor.go

test:
	go build
	go test