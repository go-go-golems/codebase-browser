GOLANGCI_LINT_VERSION ?= $(shell cat .golangci-lint-version)
GOLANGCI_LINT_BIN  ?= $(CURDIR)/.bin/golangci-lint
GOLANGCI_LINT_ARGS ?= --timeout=5m ./cmd/... ./pkg/...
VERSION ?= $(shell svu current 2>/dev/null || echo dev)
LDFLAGS ?= -X main.version=$(VERSION)
GORELEASER_ARGS ?= --skip=sign --snapshot --clean
GORELEASER_TARGET ?= --single-target

.PHONY: help frontend-check frontend-build generate build smoke clean tidy test lint lintmax docs-smoke review-widget-smoke review-browser-smoke golangci-lint-install gosec govulncheck goreleaser tag-major tag-minor tag-patch release version install bump-glazed

BINARY := codebase-browser
PKG    := github.com/wesen/codebase-browser

help:
	@echo "Targets:"
	@echo "  frontend-check  TypeScript check"
	@echo "  frontend-build  Vite production build -> ui/dist/public/"
	@echo "  generate        Run go generate on the generator packages (builds index + copies assets)"
	@echo "  build           Build single embedded binary (tags: sqlite_fts5,embed)"
	@echo "  smoke           Build the CLI"
	@echo "  test            go test ./..."
	@echo "  lint            golangci-lint run ./cmd/... ./pkg/..."
	@echo "  lintmax         golangci-lint run with max-same-issues=100"
	@echo "  goreleaser      Local GoReleaser snapshot (override GORELEASER_ARGS/TARGET)"
	@echo "  tag-patch       Create next semver patch tag with svu"
	@echo "  release         Push tags and warm the Go module proxy"
	@echo "  docs-smoke      Smoke-test docs examples (index, export, verify)"
	@echo "  review-widget-smoke  Strict all-widget review export smoke test"
	@echo "  review-browser-smoke Source-page browser smoke test; pass URL=..."
	@echo "  bump-glazed     Bump go-go-golems packages to latest"


frontend-check:
	pnpm -C ui run typecheck

frontend-build:
	pnpm -C ui run build

generate:
	go generate ./cmd/... ./internal/browser ./internal/docs ./internal/indexer ./internal/indexfs ./internal/sourcefs ./internal/staticapp


build: generate
	GOWORK=off go build -tags "sqlite_fts5 embed" -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

smoke: build
	./bin/$(BINARY) --help >/dev/null
	@echo "smoke ok"

test:
	go test ./... -count=1

golangci-lint-install:
	mkdir -p $(dir $(GOLANGCI_LINT_BIN))
	GOBIN=$(dir $(GOLANGCI_LINT_BIN)) GOWORK=off go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: golangci-lint-install
	GOWORK=off $(GOLANGCI_LINT_BIN) config verify
	GOWORK=off $(GOLANGCI_LINT_BIN) run -v $(GOLANGCI_LINT_ARGS)

lintmax: golangci-lint-install
	GOWORK=off $(GOLANGCI_LINT_BIN) config verify
	GOWORK=off $(GOLANGCI_LINT_BIN) run -v --max-same-issues=100 $(GOLANGCI_LINT_ARGS)

gosec:
	GOWORK=off go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -exclude-generated -exclude=G101,G304,G301,G306 $(GOSEC_EXCLUDE_DIRS) $(LINT_DIRS)

govulncheck:
	GOWORK=off go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

tidy:
	GOWORK=off go mod tidy

goreleaser:
	GOWORK=off goreleaser release $(GORELEASER_ARGS) $(GORELEASER_TARGET)

tag-major:
	git tag $(shell svu major)

tag-minor:
	git tag $(shell svu minor)

tag-patch:
	git tag $(shell svu patch)

release:
	git push origin --tags
	GOWORK=off GOPROXY=proxy.golang.org go list -m $(PKG)@$(shell svu current)

version:
	@echo $(VERSION)

install: build
	mkdir -p dist
	cp bin/$(BINARY) dist/$(BINARY)
	@if command -v $(BINARY) >/dev/null 2>&1; then cp dist/$(BINARY) "$$(command -v $(BINARY))"; else echo "$(BINARY) not found on PATH; built dist/$(BINARY)"; fi

clean:
	rm -rf bin dist ui/dist internal/sourcefs/embed/source/* internal/indexfs/embed/index.json internal/indexfs/embed/index-ts.json internal/sqlite/embed/codebase.db internal/staticapp/embed/public

# Smoke-test the documentation examples.
# Creates a temp DB with the examples/, exports it, verifies manifest.json fields,
# and checks that no legacy runtime files are present.
docs-smoke:
	@if [ ! -f bin/$(BINARY) ]; then $(MAKE) build; fi
	@set -e; \
	  DB=$$(mktemp /tmp/gcb-smoke-XXXXXX.db); \
	  OUT=$$(mktemp -d /tmp/gcb-smoke-export-XXXXXX); \
	  echo "docs-smoke: creating DB from examples/..."; \
	  ./bin/$(BINARY) review index --commits HEAD~5..HEAD --docs ./examples --db "$$DB"; \
	  echo "docs-smoke: exporting..."; \
	  ./bin/$(BINARY) review export --db "$$DB" --out "$$OUT"; \
	  echo "docs-smoke: verifying export..."; \
	  test -f "$$OUT/manifest.json" || { echo "manifest.json missing"; exit 1; }; \
	  test -f "$$OUT/db/codebase.db" || { echo "db/codebase.db missing"; exit 1; }; \
	  echo "  manifest.json: hasGoRuntimeServer=False, queryEngine=sql.js"; \
	  echo "docs-smoke: checking for legacy runtime files..."; \
	  ! test -e "$$OUT/precomputed.json" || { echo "precomputed.json should not exist"; exit 1; }; \
	  ! test -e "$$OUT/search.wasm" || { echo "search.wasm should not exist"; exit 1; }; \
	  ! test -e "$$OUT/wasm_exec.js" || { echo "wasm_exec.js should not exist"; exit 1; }; \
	  echo "docs-smoke: checking DB content..."; \
	  sqlite3 "$$OUT/db/codebase.db" "SELECT slug FROM review_docs;" | grep -q "01-pr-review-static-export" || { echo "example doc not in export DB"; exit 1; }; \
	  echo "docs-smoke: PASSED"

review-widget-smoke:
	ttmp/2026/05/02/GCB-018--robust-review-widget-rendering-contract/scripts/01-review-widget-smoke.sh

review-browser-smoke:
	@if [ -z "$(URL)" ]; then echo "usage: make review-browser-smoke URL=http://127.0.0.1:4187/?debugSql#/source/path.go"; exit 2; fi
	scripts/review-browser-smoke.py "$(URL)" $(ARGS)

# Bump all go-go-golems packages to their latest versions.
bump-glazed:
	GOWORK=off go get github.com/go-go-golems/glazed@latest
	GOWORK=off go get github.com/go-go-golems/clay@latest
	GOWORK=off go mod tidy
