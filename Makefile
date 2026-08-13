GO ?= go
DIST ?= dist

.PHONY: all build test vet install uninstall clean

all: build

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o $(DIST)/hiddify-tui ./cmd/hiddify-tui
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o $(DIST)/hiddify-migrate ./cmd/hiddify-migrate

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

install: build
	sudo ./packaging/linux/install.sh "$(CURDIR)/$(DIST)"

uninstall:
	sudo ./packaging/linux/uninstall.sh

clean:
	rm -rf $(DIST)
