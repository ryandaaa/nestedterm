# NestedTerm Makefile

BINARY_NAME := nestedterm
CMD_DIR := ./cmd/nestedterm
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.PHONY: all build test test-race vet fmt clean install uninstall npm-build npm-check npm-pack

all: build

build:
	go build -ldflags="-s -w" -o $(BINARY_NAME) $(CMD_DIR)

test:
	go test -v ./...

test-race:
	go test -v -race ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

clean:
	rm -f $(BINARY_NAME)

install: build
	mkdir -p $(BINDIR)
	install -m 0755 $(BINARY_NAME) $(BINDIR)/$(BINARY_NAME)
	@echo "NestedTerm installed to $(BINDIR)/$(BINARY_NAME)"
	@echo "Ensure $(BINDIR) is in your PATH."

uninstall:
	rm -f $(BINDIR)/$(BINARY_NAME)
	@echo "NestedTerm uninstalled from $(BINDIR)/$(BINARY_NAME)"

# --- npm package (prebuilt Linux binaries bundled with a Node launcher) ---

npm-check:
	@v_go=$$(sed -n 's/.*Version *= *"\(.*\)".*/\1/p' internal/config/defaults.go); \
	v_npm=$$(sed -n 's/.*"version": *"\(.*\)".*/\1/p' npm/package.json); \
	if [ "$$v_go" != "$$v_npm" ]; then \
		echo "version mismatch: Go=$$v_go npm=$$v_npm"; exit 1; \
	fi; echo "version $$v_go ok"

npm-build: npm-check
	rm -rf npm/dist
	mkdir -p npm/dist/linux-x64 npm/dist/linux-arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o npm/dist/linux-x64/$(BINARY_NAME) $(CMD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o npm/dist/linux-arm64/$(BINARY_NAME) $(CMD_DIR)
	cp README.md LICENSE npm/

npm-pack: npm-build
	cd npm && npm pack
