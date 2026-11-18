.PHONY: lint test-unit setup

lint: setup
	golangci-lint run ./...

test-unit: setup
	go test -race ./...

setup:
	@[ "$$(git config --local core.hooksPath)" = .githooks ] || git config core.hooksPath .githooks
