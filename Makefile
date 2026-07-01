BINARY := lazyss
PKG := ./cmd/lazyss
BINDIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := build

.PHONY: build
build:
	@mkdir -p $(BINDIR)
	go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY) $(PKG)

.PHONY: run
run:
	go run $(PKG)

.PHONY: doctor
doctor:
	go run $(PKG) doctor

.PHONY: test
test:
	go test -race ./...

.PHONY: script-test
script-test:
	python3 -m unittest discover -s scripts -p '*_test.py'

.PHONY: cover
cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint not installed locally; CI runs golangci-lint v2.12.2"; \
		exit 0; \
	fi; \
	golangci-lint run ./...

.PHONY: vuln
vuln:
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "govulncheck not installed locally; CI runs govulncheck"; \
		exit 0; \
	fi; \
	govulncheck ./...

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: fmt-check
fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "not formatted:"; echo "$$out"; exit 1; fi

.PHONY: check
check: fmt-check vet test script-test build

.PHONY: fast-pr
fast-pr: fmt-check vet test script-test build smoke-local lint vuln

.PHONY: heavy-quality
heavy-quality: cover lint vuln

.PHONY: release-snapshot
release-snapshot:
	@if ! command -v goreleaser >/dev/null 2>&1; then \
		echo "goreleaser is required for local release snapshot; hosted Release Candidate runs the same gate"; \
		exit 1; \
	fi; \
	goreleaser check; \
	goreleaser release --clean --snapshot --skip=publish

.PHONY: smoke-local
smoke-local:
	./scripts/smoke-local.sh

.PHONY: smoke
smoke: smoke-local

.PHONY: homebrew-readiness
homebrew-readiness:
	./scripts/homebrew-readiness.sh

.PHONY: branch-protection-readiness
branch-protection-readiness:
	./scripts/branch-protection-readiness.sh

.PHONY: release-readiness
release-readiness:
	./scripts/release-readiness.sh

.PHONY: release-preflight
release-preflight: release-readiness

.PHONY: live-smoke-evidence-template
live-smoke-evidence-template:
	python3 scripts/live_smoke_evidence.py template \
		--output live-smoke-evidence.json \
		--target-version "$${LAZYSS_RELEASE_VERSION:-v0.1.0}" \
		--commit "$$(git rev-parse HEAD)"

.PHONY: clean
clean:
	rm -rf $(BINDIR) coverage.out coverage.html
