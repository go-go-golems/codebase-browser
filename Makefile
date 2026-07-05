GOLANGCI_LINT_VERSION ?= $(shell cat .golangci-lint-version)
GOLANGCI_LINT_BIN  ?= $(CURDIR)/.bin/golangci-lint
GOLANGCI_LINT_ARGS ?= --timeout=5m ./cmd/... ./pkg/...

.PHONY: help frontend-check frontend-build generate build smoke clean tidy test lint lintmax docs-smoke demo-solid demo-serve demo-solid-serve demo-smoke golangci-lint-install bump-glazed

BINARY := codebase-browser
PKG    := github.com/wesen/codebase-browser

DEMO_COMMITS ?= 025e4c6..79af1b0
DEMO_DB      ?= /tmp/gcb-solid-demo.db
DEMO_OUT     ?= /tmp/gcb-solid-demo
DEMO_ADDR    ?= :3003
DEMO_PID     ?= /tmp/cbb-live-ui.pid
DEMO_LOG     ?= /tmp/cbb-live-ui.log

help:
	@echo "Targets:"
	@echo "  frontend-check  TypeScript check"
	@echo "  frontend-build  Vite production build -> ui/dist/public/"
	@echo "  generate        Run go generate on the generator packages (builds index + copies assets)"
	@echo "  build           Build single embedded binary (tag: embed)"
	@echo "  smoke           Build the CLI"
	@echo "  test            go test ./..."
	@echo "  lint            golangci-lint run ./cmd/... ./pkg/..."
	@echo "  lintmax         golangci-lint run with max-same-issues=100"
	@echo "  docs-smoke      Smoke-test docs examples (index, export, verify)"
	@echo "  demo-solid      Build the stable 118-commit local demo export"
	@echo "  demo-serve      Restart live server for the existing demo export"
	@echo "  demo-smoke      Smoke-test the running live demo API/artifact"
	@echo "  bump-glazed     Bump go-go-golems packages to latest"


frontend-check:
	pnpm -C ui run typecheck

frontend-build:
	pnpm -C ui run build

generate:
	go generate ./cmd/... ./internal/browser ./internal/docs ./internal/indexer ./internal/indexfs ./internal/sourcefs


build: generate
	go build -o bin/$(BINARY) ./cmd/$(BINARY)

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

tidy:
	go mod tidy

clean:
	rm -rf bin ui/dist internal/sourcefs/embed/source/* internal/indexfs/embed/index.json

# Smoke-test the documentation examples.
# Creates a temp DB with the examples/, exports it, verifies manifest.json fields,
# and checks that no legacy runtime files or rendered review errors are present.
docs-smoke:
	@if [ ! -f bin/$(BINARY) ]; then $(MAKE) build; fi
	@set -e; \
	  DB=$$(mktemp /tmp/gcb-smoke-XXXXXX.db); \
	  OUT=$$(mktemp -d /tmp/gcb-smoke-export-XXXXXX); \
	  trap 'rm -f "$$DB"; rm -rf "$$OUT"' EXIT; \
	  echo "docs-smoke: creating DB from examples over $(DEMO_COMMITS)..."; \
	  ./bin/$(BINARY) review index --commits $(DEMO_COMMITS) --docs ./examples --db "$$DB"; \
	  echo "docs-smoke: exporting..."; \
	  ./bin/$(BINARY) review export --db "$$DB" --out "$$OUT"; \
	  echo "docs-smoke: verifying export..."; \
	  test -f "$$OUT/manifest.json" || { echo "manifest.json missing"; exit 1; }; \
	  test -f "$$OUT/db/codebase.db" || { echo "db/codebase.db missing"; exit 1; }; \
	  echo "docs-smoke: checking for legacy runtime files..."; \
	  ! test -e "$$OUT/precomputed.json" || { echo "precomputed.json should not exist"; exit 1; }; \
	  ! test -e "$$OUT/search.wasm" || { echo "search.wasm should not exist"; exit 1; }; \
	  ! test -e "$$OUT/wasm_exec.js" || { echo "wasm_exec.js should not exist"; exit 1; }; \
	  echo "docs-smoke: checking DB content..."; \
	  sqlite3 "$$OUT/db/codebase.db" "SELECT slug FROM review_docs;" | grep -q "01-pr-review-static-export" || { echo "example doc not in export DB"; exit 1; }; \
	  ERRORS=$$(sqlite3 "$$OUT/db/codebase.db" "SELECT COUNT(*) FROM static_review_rendered_docs WHERE errors_json != '[]';"); \
	  test "$$ERRORS" = "0" || { echo "rendered review docs contain $$ERRORS error row(s)"; exit 1; }; \
	  echo "docs-smoke: PASSED"

# Build the reproducible rich local demo used for manual review.
demo-solid: build
	@set -e; \
	  echo "demo-solid: indexing $(DEMO_COMMITS) -> $(DEMO_DB)"; \
	  rm -f "$(DEMO_DB)"; \
	  ./bin/$(BINARY) review index --commits $(DEMO_COMMITS) --docs ./examples --db "$(DEMO_DB)"; \
	  echo "demo-solid: exporting -> $(DEMO_OUT)"; \
	  rm -rf "$(DEMO_OUT)"; \
	  ./bin/$(BINARY) review export --db "$(DEMO_DB)" --out "$(DEMO_OUT)"; \
	  COMMITS=$$(sqlite3 "$(DEMO_OUT)/db/codebase.db" "SELECT COUNT(*) FROM commits WHERE error = '';"); \
	  ERRORS=$$(sqlite3 "$(DEMO_OUT)/db/codebase.db" "SELECT COUNT(*) FROM static_review_rendered_docs WHERE errors_json != '[]';"); \
	  test "$$ERRORS" = "0" || { echo "rendered review docs contain $$ERRORS error row(s)"; exit 1; }; \
	  echo "demo-solid: PASSED ($$COMMITS commits, 0 rendered review errors)"

# Restart the live Go API/static server for an already exported demo.
demo-serve:
	@if [ ! -f bin/$(BINARY) ]; then $(MAKE) build; fi
	@set -e; \
	  test -f "$(DEMO_OUT)/db/codebase.db" || { echo "missing $(DEMO_OUT)/db/codebase.db; run make demo-solid first"; exit 1; }; \
	  if [ -f "$(DEMO_PID)" ]; then kill $$(cat "$(DEMO_PID)") 2>/dev/null || true; fi; \
	  nohup ./bin/$(BINARY) serve --addr $(DEMO_ADDR) --db "$(DEMO_OUT)/db/codebase.db" --static-dir "$(DEMO_OUT)" > "$(DEMO_LOG)" 2>&1 & echo $$! > "$(DEMO_PID)"; \
	  sleep 1; \
	  curl -fsS "http://127.0.0.1$(DEMO_ADDR)/api/health" >/dev/null; \
	  echo "demo-serve: listening at http://127.0.0.1$(DEMO_ADDR)/"

demo-solid-serve: demo-solid demo-serve

# Smoke-test the running live demo API and exported artifact.
demo-smoke:
	@set -e; \
	  BASE="http://127.0.0.1$(DEMO_ADDR)"; \
	  curl -fsS "$$BASE/api/health" | grep -q 'live-go' || { echo "live health check failed"; exit 1; }; \
	  COMMITS=$$(curl -fsS "$$BASE/api/history/commits" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))'); \
	  test "$$COMMITS" -ge 100 || { echo "expected >=100 commits, got $$COMMITS"; exit 1; }; \
	  MAIN_ROWS=$$(curl -fsS "$$BASE/api/history/symbol?symbol=sym%3Agithub.com%2Fwesen%2Fcodebase-browser%2Fcmd%2Fcodebase-browser.func.main" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))'); \
	  test "$$MAIN_ROWS" -ge 100 || { echo "expected >=100 main history rows, got $$MAIN_ROWS"; exit 1; }; \
	  ERRORS=$$(sqlite3 "$(DEMO_OUT)/db/codebase.db" "SELECT COUNT(*) FROM static_review_rendered_docs WHERE errors_json != '[]';"); \
	  test "$$ERRORS" = "0" || { echo "rendered review docs contain $$ERRORS error row(s)"; exit 1; }; \
	  echo "demo-smoke: PASSED ($$COMMITS commits, $$MAIN_ROWS main history rows, 0 rendered review errors)"

# Bump all go-go-golems packages to their latest versions.
bump-glazed:
	GOWORK=off go get github.com/go-go-golems/glazed@latest
	GOWORK=off go get github.com/go-go-golems/clay@latest
	GOWORK=off go mod tidy
