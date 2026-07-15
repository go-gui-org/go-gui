VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS  = -X github.com/go-gui-org/go-gui/gui.Version=$(VERSION) \
           -X github.com/go-gui-org/go-gui/gui.Commit=$(COMMIT)

CC_WINDOWS ?= x86_64-w64-mingw32-gcc
STATIC_TAG  = static,audio
LINT_VERSION = v2.12.2

.PHONY: build-linux build-windows build-macos build-wasm build-ios build-android build-examples release clean test test-race vet lint check bench bench-gate deps-doc deps-doc-check security gosec govulncheck large-files deadcode generate-check tidy-check workflow-audit cov-report license-check

build-linux:
	CGO_ENABLED=1 \
	go build -tags $(STATIC_TAG) -ldflags "$(LDFLAGS)" \
	  -o build/showcase-linux ./examples/showcase/

build-windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$(CC_WINDOWS) \
	go build -tags $(STATIC_TAG) -ldflags "$(LDFLAGS)" \
	  -o build/showcase-windows.exe ./examples/showcase/

build-macos:
	CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries" \
	go build -ldflags "$(LDFLAGS)" \
	  -o build/showcase-macos ./examples/showcase/

build-wasm:
	GOOS=js GOARCH=wasm \
	go build -ldflags "$(LDFLAGS)" \
	  -o build/showcase.wasm ./examples/showcase/
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" build/

build-ios:
	SDK=$$(xcrun --sdk iphoneos --show-sdk-path); \
	CC=$$(xcrun --sdk iphoneos --find clang); \
	cd examples/ios_demo && \
	CGO_ENABLED=1 GOOS=ios GOARCH=arm64 \
	  CC="$$CC" \
	  CGO_CFLAGS="-isysroot $$SDK -arch arm64 -miphoneos-version-min=15.0" \
	  CGO_LDFLAGS="-isysroot $$SDK -arch arm64 -miphoneos-version-min=15.0" \
	  go build -buildmode=c-archive -tags ios -o libgoguiapp.a .

build-android:
	go install golang.org/x/mobile/cmd/gomobile@latest
	gomobile init
	cd examples/android_demo && \
	gomobile bind -target=android/arm64 -androidapi 24 -o gogui.aar .

build-examples:
	@mkdir -p examples/bin; \
	failed=""; \
	EXTRA_FLAGS=""; \
	[ "$$(uname)" = Darwin ] && EXTRA_FLAGS="-Wl,-no_warn_duplicate_libraries"; \
	for dir in examples/*/; do \
		name=$$(basename "$$dir"); \
		case "$$name" in \
			ios_demo|android_demo|bin) continue ;; \
		esac; \
		if ! CGO_LDFLAGS="$$EXTRA_FLAGS" go build -o "examples/bin/$$name" "./$$dir"; then \
			failed="$$failed $$name"; \
		fi; \
	done; \
	if [ -n "$$failed" ]; then \
		echo "ERROR: examples failed to build:$$failed"; \
		exit 1; \
	else \
		echo "All buildable examples compiled to examples/bin/."; \
	fi

release: build-linux build-windows build-macos build-wasm
	tar czf build/go-gui-showcase-$(VERSION)-linux-amd64.tar.gz \
	  -C build showcase-linux
	cd build && zip go-gui-showcase-$(VERSION)-windows-amd64.zip \
	  showcase-windows.exe
	cd build && go run ../cmd/buildapp -version $(VERSION) \
	  -name "Go-Gui Showcase" showcase-macos
	hdiutil create -srcfolder "build/Go-Gui Showcase.app" \
	  -volname "Go-Gui Showcase $(VERSION)" \
	  -format UDZO "build/Go-Gui-Showcase-$(VERSION).dmg"

# Run all benchmarks with allocation reporting (matching CI baseline job).
bench:
	go test -bench=. -benchmem -count=5 -run='^$' -timeout=30m ./gui/...

# Run targeted hot-path benchmarks for regression checking (matching CI gate job).
bench-gate:
	go test \
	  -bench='Benchmark(Layout|GenerateViewLayout|ViewFrame|ParseSvg|Tessellate|BuildDefsPathDataCache|RenderLayout|RenderSvg)' \
	  -benchmem -count=5 -run='^$' -timeout=15m ./gui/...

clean:
	rm -rf build/

# Run all tests with explicit timeout.
test:
	go test -count=1 -timeout=5m ./...

# Run all tests with race detector enabled.
test-race:
	go test -race -count=1 -timeout=10m ./...

# Run go vet static analysis.
vet:
	go vet ./...
	go run ./tools/requiredid/cmd/requiredid ./...

# Run golangci-lint (requires golangci-lint installed, pinned to LINT_VERSION).
lint:
	@golangci-lint --version | grep -q "$(LINT_VERSION)" || \
	  { echo "::error::golangci-lint $(LINT_VERSION) required. Run: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)"; exit 1; }
	golangci-lint run ./...

# Run non-duplicated validation steps for CI gate.
# test and lint run as separate CI jobs with OS matrices.
check: vet deps-doc-check large-files generate-check tidy-check

# Run all validation steps: test, vet, lint, and gate checks.
check-all: test lint check

# Regenerate docs/dependencies.md from go.mod.
deps-doc:
	go run ./tools/depsdoc/ -w

# Check that docs/dependencies.md is up to date with go.mod.
deps-doc-check:
	go run ./tools/depsdoc/ > /tmp/deps-generated.md
	diff docs/dependencies.md /tmp/deps-generated.md || \
	  { echo "::error::docs/dependencies.md is out of date. Run 'make deps-doc'." >&2; exit 1; }
	rm -f /tmp/deps-generated.md

# Report Go source files exceeding 800 lines in gui/. Exit non-zero if any exist.
large-files:
	@scripts/large-files.sh
	@count=$$(find gui -name '*.go' -not -name '*_test.go' \
	  -exec wc -l {} \; | awk '$$1 > 800' | wc -l); \
	if [ "$$count" -gt 0 ]; then \
	  echo "::error::$$count Go source files exceed 800 lines"; \
	  exit 1; \
	fi

# Check that go generate produces no changes to generated files.
generate-check:
	go generate ./...
	@if [ -n "$$(git diff --name-only -- '*_gen.go')" ]; then \
	  echo "::error::go generate produced changes to generated files. Run 'go generate ./...' and commit."; \
	  git diff -- '*_gen.go'; \
	  exit 1; \
	fi

# Report exported-but-unreachable functions (dead code).
deadcode:
	go run golang.org/x/tools/cmd/deadcode@latest \
	  -test \
	  ./...

# Check that go.mod and go.sum are tidy.
tidy-check:
	go mod tidy
	@git diff --exit-code go.mod go.sum || \
	  { echo "::error::go.mod or go.sum is not tidy. Run 'go mod tidy'."; \
	    git diff go.mod go.sum; \
	    exit 1; }

# Run security scans (gosec + govulncheck + license-check).
security: gosec govulncheck license-check

gosec:
	gosec -include=G101,G104,G107,G201,G202,G203,G204,G301,G302,G303,G304,G306,G401,G402,G501,G502,G503,G504,G505 \
	  -conf .gosec.json \
	  ./...

govulncheck:
	govulncheck ./...

# Verify all dependencies have permitted licenses.
license-check:
	go run github.com/google/go-licenses@latest check \
	  --allowed_licenses MIT,BSD-2-Clause,BSD-3-Clause,Apache-2.0,ISC \
	  --include_tests \
	  ./...

# Generate HTML coverage report in browser.
cov-report:
	go test -coverprofile=/tmp/go-gui-coverage.out -timeout=5m ./...
	go tool cover -html=/tmp/go-gui-coverage.out

# Audit workflow files for unpinned actions (excludes setup-go which uses major-version pinning by design).
workflow-audit:
	@grep -n 'uses:.*@v[0-9]' .github/workflows/*.yml | grep -v setup-go || true
	@echo "Lines above use version tags instead of SHAs."
