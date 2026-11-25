.PHONY: build build-helper test test-go test-frontend check-frontend lint-frontend fmt-frontend test-e2e clean

GO ?= go
BUN ?= bun
GOPROXY ?= direct,https://proxy.golang.org

build: build-helper
	GOPROXY=$(GOPROXY) $(GO) build -o bin/my-vpn ./cmd/app

build-helper:
	GOPROXY=$(GOPROXY) $(GO) build -o bin/core-helper ./cmd/core-helper

test: test-go test-frontend

test-go:
	GOPROXY=$(GOPROXY) $(GO) test -v -race ./...

test-frontend:
	$(BUN) --cwd frontend test

check-frontend:
	$(BUN) --cwd frontend check

lint-frontend:
	$(BUN) --cwd frontend run lint

fmt-frontend:
	$(BUN) --cwd frontend fmt src

test-e2e: build
	bash tests/e2e-linux/run_e2e.sh

clean:
	rm -rf bin build frontend/dist
