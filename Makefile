# v-rpc-tap — the `v rpc-tap` domain (durable, scalable RPC-broker tap -> S3).
# Greenfield: no standalone binary yet (verbs land in P3); `build` compiles the
# library packages. When main.go arrives this graduates to the full v-domain
# Makefile (LDFLAGS / install / cross-compile), matching v-rpc-debug.
BIN ?= v-rpc-tap

build:
	go build ./...

test:
	go test -race -cover ./...

lint:
	golangci-lint run ./...

check: lint test build

clean:
	rm -f dist/$(BIN) dist/$(BIN)-* *.test
