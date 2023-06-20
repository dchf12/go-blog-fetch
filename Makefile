.PHONY: build run test bench cover clean help
BINARY_NAME := $(notdir $(shell pwd))
COVERAGE_FILE := coverage.out
.DEFAULT_GOAL := help

tidy:
	@go mod tidy

build:
	@go build -o $(BINARY_NAME) -v

run: build
	@./$(BINARY_NAME)

test: build
	@go test -v -coverprofile=${COVERAGE_FILE} ./...

bench: 
	@go test -bench=. -benchmem ./...

cover: test
	@go tool cover -func ${COVERAGE_FILE}

clean:
	@go clean
	@rm -f $(BINARY_NAME)
	@rm -f $(COVERAGE_FILE)

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  help      Show this help"
	@echo "  all       Build and run the project"
	@echo "  build     Build the project"
	@echo "  run       Run the project"
	@echo "  test      Run tests"
	@echo "  bench     Run benchmarks"
	@echo "  cover     Run tests and show coverage"
	@echo "  clean     Clean up generated files"
