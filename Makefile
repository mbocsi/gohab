.PHONY: all server client1 client2

all: server client1 client2

server:
	go build -o bin/server ./cmd/server

client1:
	go build -o bin/client1 ./cmd/client1

client2:
	go build -o bin/client2 ./cmd/client2

clean:
	rm ./bin/*