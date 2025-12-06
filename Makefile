.PHONY: all server client1 client2 client3 test test-server test-client test-integration test-coverage test-integration-coverage test-all clean

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
	go test -v -tags=integration ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-integration-coverage:
	go test -v -tags=integration -coverprofile=coverage-integration.out ./...
	go tool cover -html=coverage-integration.out -o coverage-integration.html

test-all: test test-integration

clean:
	rm -f ./bin/*
	rm -f coverage.out coverage.html coverage-integration.out coverage-integration.html