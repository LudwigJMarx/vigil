# Every target here is also a step in .github/workflows/pruefungen.yml. The
# workflow is the binding list; this file exists so the same checks can be run
# before pushing, with one command, on a machine that has Go, Node and Python.

GO      ?= go
NPM     ?= npm
PYTHON  ?= python3
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/LudwigJMarx/vigil/internal/cli.Version=$(VERSION)

.PHONY: help build test check run extension lizenzen clean

help:
	@echo "build      build ./vigil for this machine"
	@echo "test       go test plus the extension's tests"
	@echo "check      everything the CI runs, in the same order"
	@echo "run        build, then serve on 127.0.0.1:8099"
	@echo "extension  compile the browser extension into extension/dist"
	@echo "lizenzen   regenerate THIRD-PARTY-NOTICES.md after a dependency change"
	@echo "clean      remove build output"

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o vigil ./cmd/vigil

extension:
	cd extension && $(NPM) install && $(NPM) run build

test:
	$(GO) test -race ./...
	cd extension && $(NPM) test

check:
	$(PYTHON) scripts/pruefer-verdrahtet.py
	$(PYTHON) scripts/pruefer-verdrahtet.test.py
	$(PYTHON) scripts/pruefer.test.py
	$(PYTHON) scripts/pruefe-keine-hintergrundabfrage.py
	$(PYTHON) scripts/pruefe-doku-befehle.py
	$(PYTHON) scripts/pruefe-lizenzhinweise.py
	gofmt -l ./cmd ./internal | (! grep .) || (echo "run gofmt -w ./cmd ./internal"; exit 1)
	$(GO) vet ./...
	$(GO) test -race ./...
	cd extension && $(NPM) run typecheck && $(NPM) test && $(NPM) run build

run: build
	./vigil serve

lizenzen:
	$(PYTHON) scripts/pruefe-lizenzhinweise.py --schreiben

clean:
	rm -rf vigil extension/dist
