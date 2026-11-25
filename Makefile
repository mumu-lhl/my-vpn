.PHONY: build build-helper test test-go test-frontend test-e2e clean

GO ?= go
GOPROXY ?= direct,https://proxy.golang.org

build: build-helper
	GOPROXY=$(GOPROXY) $(GO) build -o bin/my-vpn ./cmd/app

build-helper:
	GOPROXY=$(GOPROXY) $(GO) build -o bin/core-helper ./cmd/core-helper

test: test-go test-frontend

test-go:
	GOPROXY=$(GOPROXY) $(GO) test -v -race ./...

test-frontend:
	npm --prefix frontend test

test-e2e: build
	bash tests/e2e-linux/run_e2e.sh

clean:
	rm -rf bin build frontend/dist
