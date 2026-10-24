# NestedTerm Makefile

BINARY_NAME := nestedterm
CMD_DIR := ./cmd/nestedterm
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.PHONY: all build test test-race vet fmt clean install uninstall

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
