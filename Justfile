set shell := ["bash", "-lc"]

gocache := justfile_directory() + "/.gocache"

@default: build

@build:
	mkdir -p {{gocache}}
	CGO_ENABLED=1 GOCACHE={{gocache}} go build ./...

@test:
	mkdir -p {{gocache}}
	CGO_ENABLED=1 GOCACHE={{gocache}} go test -race -cover -timeout=30s ./...

@fmt:
	mkdir -p {{gocache}}
	GOCACHE={{gocache}} go fmt ./...

@lint:
	if command -v golangci-lint >/dev/null 2>&1; then \
		GOCACHE={{gocache}} golangci-lint run ./...; \
	else \
		echo "golangci-lint not found; install with 'just tools' or skip"; \
	fi

@tidy:
	GOCACHE={{gocache}} go mod tidy

@check: fmt lint test

@tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

@clean:
	rm -rf {{gocache}}

@help:
	just --list
