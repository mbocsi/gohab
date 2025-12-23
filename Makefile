.PHONY: all server client1 client2 client3 test test-server test-client test-integration test-coverage test-integration-coverage test-all test-functional test-run test-pattern test-watch clean

all: server client1 client2 client3

server:
	go build -o bin/server ./cmd/server

client1:
	go build -o bin/client1 ./cmd/client1

client2:
	go build -o bin/client2 ./cmd/client2

client3:
	go build -o bin/client3 ./cmd/client3

# Core test commands
test:
	go test ./...

test-server:
	go test ./server/... ./services/... ./web/... ./proto/...

test-client:
	go test ./client/...

test-integration:
	go test -v ./test/integration/...

# Enhanced testing commands
test-functional:
	@echo "Running functional integration tests..."
	go test ./test/integration/functional_*.go ./test/integration/utils.go ./test/integration/stateful_devices.go -v

# Targeted test execution (usage: make test-run TEST=TestName [FILES="path/to/test.go"])
test-run:
	@if [ -z "$(TEST)" ]; then echo "Usage: make test-run TEST=TestName [FILES=\"path/to/test.go\"]"; exit 1; fi
	@if [ -n "$(FILES)" ]; then \
		echo "Running test: $(TEST) in files: $(FILES)"; \
		go test $(FILES) -run $(TEST) -v; \
	else \
		echo "Running test: $(TEST) in all packages"; \
		go test ./... -run $(TEST) -v; \
	fi

# Test with pattern matching (usage: make test-pattern PATTERN="Query.*" [PKG="./server/..."])
test-pattern:
	@if [ -z "$(PATTERN)" ]; then echo "Usage: make test-pattern PATTERN='TestPattern.*' [PKG=\"./server/...\"]"; exit 1; fi
	@if [ -n "$(PKG)" ]; then \
		echo "Running tests matching pattern: $(PATTERN) in package: $(PKG)"; \
		go test $(PKG) -run $(PATTERN) -v; \
	else \
		echo "Running tests matching pattern: $(PATTERN) in all packages"; \
		go test ./... -run $(PATTERN) -v; \
	fi

# Continuous testing (watch mode)
test-watch:
	@echo "Running tests in watch mode (requires 'entr' command)..."
	@echo "Install with: brew install entr (macOS) or apt-get install entr (Ubuntu)"
	find . -name "*.go" | entr -c make test

test-watch-integration:
	@echo "Running integration tests in watch mode..."
	find . -name "*.go" | entr -c make test-functional

# Coverage commands
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-integration-coverage:
	go test -v -coverprofile=coverage-integration.out ./test/integration/...
	go tool cover -html=coverage-integration.out -o coverage-integration.html
	@echo "Integration coverage report generated: coverage-integration.html"

test-coverage-func:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Comprehensive testing
test-all: test test-integration

test-verbose:
	go test -v ./...

# Debugging and troubleshooting
test-race:
	@echo "Running tests with race detection..."
	go test -race ./...

test-short:
	@echo "Running short tests only..."
	go test -short ./...

test-timeout:
	@echo "Running tests with extended timeout..."
	go test -timeout 30s ./...

# Test failure analysis
test-failfast:
	@echo "Running tests with fail-fast mode..."
	go test -failfast ./...

test-count:
	@echo "Running tests multiple times to catch flaky tests..."
	go test -count=3 ./...

# Help command
test-help:
	@echo "GoHab Test Commands:"
	@echo ""
	@echo "Basic Commands:"
	@echo "  make test                    - Run all unit tests"
	@echo "  make test-server            - Run server-side tests only"
	@echo "  make test-client            - Run client-side tests only"
	@echo "  make test-integration       - Run integration tests"
	@echo ""
	@echo "Functional Tests:"
	@echo "  make test-functional        - Run all functional integration tests"
	@echo ""
	@echo "Targeted Testing:"
	@echo "  make test-run TEST=TestQueryMessageHandling"
	@echo "  make test-run TEST=ConcurrentQueries FILES=\"./test/integration/functional_*.go ./test/integration/utils.go\""
	@echo "  make test-pattern PATTERN='.*Query.*'"
	@echo "  make test-pattern PATTERN='.*Concurrent.*' PKG=\"./server/...\""
	@echo ""
	@echo "Coverage:"
	@echo "  make test-coverage          - Generate HTML coverage report"
	@echo "  make test-coverage-func     - Show function-level coverage"
	@echo ""
	@echo "Development:"
	@echo "  make test-watch             - Run tests in watch mode (requires entr)"
	@echo "  make test-race              - Run with race detection"
	@echo "  make test-verbose           - Run with verbose output"
	@echo ""
	@echo "Troubleshooting:"
	@echo "  make test-failfast          - Stop on first failure"
	@echo "  make test-count             - Run tests multiple times"
	@echo "  make test-short             - Run short tests only"

clean:
	rm -f ./bin/*
	rm -f coverage.out coverage.html coverage-integration.out coverage-integration.html