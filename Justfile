set shell := ["bash", "-lc"]

gocache := justfile_directory() + "/.gocache"

@default: build

@build:
	mkdir -p {{gocache}}
	CGO_ENABLED=1 GOCACHE={{gocache}} go build ./...

@test:
	mkdir -p {{gocache}}
	CGO_ENABLED=1 GOCACHE={{gocache}} go test -race -cover -timeout=3m ./...

@integration:
	mkdir -p {{gocache}}
	iface="lo"
	if [[ $(uname -s) == "Darwin" ]]; then iface="lo0"; fi
	CGO_ENABLED=1 \
	GOCACHE={{gocache}} \
	FI_SOCKETS_IFACE=${FI_SOCKETS_IFACE:-$iface} \
	FI_PROVIDER=${FI_PROVIDER:-sockets} \
	LIBFABRIC_E2E_PROVIDER=${LIBFABRIC_E2E_PROVIDER:-sockets} \
	LIBFABRIC_E2E_NODE=${LIBFABRIC_E2E_NODE:-127.0.0.1} \
	go test -timeout=5m -tags=integration ./integration/...

@fmt:
	mkdir -p {{gocache}}
	GOCACHE={{gocache}} go fmt ./...

@lint:
	if command -v golangci-lint >/dev/null 2>&1; then \
		GOCACHE={{gocache}} golangci-lint run ./client/... ./fi/... ./internal/...; \
	else \
		echo "golangci-lint not found; install with 'just tools' or skip"; \
	fi

@tidy:
	GOCACHE={{gocache}} go mod tidy

@check: fmt lint test

@tools:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.4.0


@clean:
	rm -rf {{gocache}}

@help:
	just --list
