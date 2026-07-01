BINARY := lazyss
PKG := ./cmd/lazyss
BINDIR := bin
COVERAGE := coverage.out
COVERAGE_SUMMARY := coverage.txt
COVERAGE_BASELINE := coverage.baseline
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
	go test -race -coverprofile=$(COVERAGE) ./...
	go tool cover -func=$(COVERAGE) | tee $(COVERAGE_SUMMARY) | tail -1
	python3 scripts/coverage_baseline.py verify --summary $(COVERAGE_SUMMARY) --baseline $(COVERAGE_BASELINE)

.PHONY: script-test
script-test:
	python3 -m unittest discover -s scripts -p '*_test.py'

.PHONY: cover
cover: test
	go tool cover -html=$(COVERAGE) -o coverage.html

.PHONY: vet
vet:
	go vet ./...

.PHONY: mod-tidy-check
mod-tidy-check:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

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
check: fmt-check mod-tidy-check vet test script-test build

.PHONY: fast-pr
fast-pr: fmt-check mod-tidy-check vet test script-test build smoke-local lint vuln

.PHONY: heavy-quality
heavy-quality: cover lint vuln

.PHONY: release-snapshot
release-snapshot:
	@if ! command -v goreleaser >/dev/null 2>&1; then \
		echo "goreleaser is required for local release snapshot; hosted Release Candidate runs the same gate"; \
		exit 1; \
	fi; \
	goreleaser check; \
	goreleaser release --clean --snapshot --skip=publish; \
	python3 scripts/release_artifacts.py verify --dist dist

.PHONY: release-artifacts-verify
release-artifacts-verify:
	python3 scripts/release_artifacts.py verify --dist "$${DIST:-dist}"

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
	rm -rf $(BINDIR) coverage.out coverage.html coverage.txt
