BINARY ?= yunxiao
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
# Injected via -ldflags; falls back to package default when unset at link time.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.16.19)
LDFLAGS := -X github.com/yunxiao-cli/yunxiao/internal/version.Version=$(VERSION)

.PHONY: build test install clean tidy ci

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

install: build
	mkdir -p $(BINDIR)
	install -m 755 $(BINARY) $(BINDIR)/$(BINARY)

clean:
	rm -f $(BINARY)

tidy:
	go mod tidy

ci:
	go build -ldflags "$(LDFLAGS)" ./... && go test ./... && go vet ./...
