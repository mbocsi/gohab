build: 
	@go build -o ./bin/gohab

run: build
	@./bin/gohab

clean:
	rm ./bin/*