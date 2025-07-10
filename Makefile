.PHONY: all server client1 client2 client3 test test-server test-client test-integration test-coverage clean

all: server client1 client2 client3

server:
	go build -o bin/server ./cmd/server

client1:
	go build -o bin/client1 ./cmd/client1

client2:
	go build -o bin/client2 ./cmd/client2

client3:
	go build -o bin/client3 ./cmd/client3

test:
	go test ./...

test-server:
	go test ./server/... ./services/... ./web/... ./proto/...

test-client:
	go test ./client/...

test-integration:
	go test -tags=integration ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -f ./bin/*
	rm -f coverage.out coverage.html