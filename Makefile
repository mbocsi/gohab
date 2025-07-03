.PHONY: all server client1 client2 client3

all: server client1 client2 client3

server:
	go build -o bin/server ./cmd/server

client1:
	go build -o bin/client1 ./cmd/client1

client2:
	go build -o bin/client2 ./cmd/client2

client3:
	go build -o bin/client3 ./cmd/client3

clean:
	rm ./bin/*