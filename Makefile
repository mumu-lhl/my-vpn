.PHONY: build build-frontend build-helper build-geph5-linux build-geph5-android test test-go test-frontend check-frontend lint-frontend fmt-frontend test-e2e clean

GO ?= go
BUN ?= bun
GOPROXY ?= direct,https://proxy.golang.org
GEPH5_LINUX_TARGET ?= x86_64-unknown-linux-musl

build: build-frontend build-helper
	GOPROXY=$(GOPROXY) $(GO) build -tags looma_embed_frontend -o bin/looma ./cmd/app

build-frontend:
	cd frontend && $(BUN) install --frozen-lockfile && $(BUN) run build

build-helper:
	GOPROXY=$(GOPROXY) $(GO) build -o bin/looma-helper ./cmd/core-helper

build-geph5-linux:
	bash third_party/geph5-build/build.sh linux $(GEPH5_LINUX_TARGET)

build-geph5-android:
	bash third_party/geph5-build/build.sh android

test: test-go test-frontend

test-go:
	GOPROXY=$(GOPROXY) $(GO) test -v -race ./...

test-frontend:
	$(BUN) --cwd frontend test

check-frontend:
	$(BUN) --cwd frontend check

lint-frontend:
	$(BUN) run --cwd frontend lint

fmt-frontend:
	$(BUN) --cwd frontend fmt src

test-e2e: build
	bash tests/e2e-linux/run_e2e.sh

clean:
	rm -rf bin build frontend/dist
