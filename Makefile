BINARY := lazyss
PKG := ./cmd/lazyss
BINDIR := bin
COVERAGE := coverage.out
COVERAGE_SUMMARY := coverage.txt
COVERAGE_BASELINE := coverage.baseline
GOVULNCHECK_VERSION := v1.5.0
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
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

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

.PHONY: workflow-policy
workflow-policy:
	python3 scripts/workflow_policy.py --workflows-dir .github/workflows

.PHONY: build-matrix
build-matrix:
	@set -e; \
	tmpdir="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmpdir"' EXIT; \
	for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do \
		goos="$${target%/*}"; \
		goarch="$${target#*/}"; \
		output="$$tmpdir/lazyss-$$goos-$$goarch"; \
		if [ "$$goos" = "windows" ]; then output="$$output.exe"; fi; \
		echo "build $$goos/$$goarch"; \
		GOOS="$$goos" GOARCH="$$goarch" go build -o "$$output" $(PKG); \
	done

.PHONY: release-snapshot
release-snapshot:
	@if ! command -v goreleaser >/dev/null 2>&1; then \
		echo "goreleaser is required for local release snapshot; hosted Release Candidate runs the same gate"; \
		exit 1; \
	fi; \
	goreleaser check; \
	goreleaser release --clean --snapshot --skip=publish; \
	python3 scripts/release_artifacts.py verify --dist dist

.PHONY: release-candidate-local
release-candidate-local: build-matrix release-snapshot
	@set +e; \
	$(MAKE) homebrew-readiness; \
	rc=$$?; \
	set -e; \
	case "$$rc" in \
		0) echo "homebrew-readiness passed" ;; \
		2) echo "homebrew-readiness has approval/external-state blockers" ;; \
		*) exit "$$rc" ;; \
	esac

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

.PHONY: branch-protection-plan
branch-protection-plan:
	python3 scripts/branch_protection_plan.py \
		--json-output branch-protection.json \
		--markdown-output branch-protection.md

.PHONY: release-approval-plan
release-approval-plan:
	python3 scripts/release_approval_plan.py \
		--markdown-output release-approval.md

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

.PHONY: homebrew-private-evidence-template
homebrew-private-evidence-template:
	python3 scripts/homebrew_private_evidence.py template \
		--output homebrew-private-evidence.json \
		--target-version "$${LAZYSS_RELEASE_VERSION:-v0.1.0}" \
		--commit "$$(git rev-parse HEAD)"

.PHONY: clean
clean:
	rm -rf $(BINDIR) coverage.out coverage.html coverage.txt lazyss lazyss.exe
