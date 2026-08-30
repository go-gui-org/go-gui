VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS  = -X github.com/go-gui-org/go-gui/gui.Version=$(VERSION) \
           -X github.com/go-gui-org/go-gui/gui.Commit=$(COMMIT)

# Repo-local bin for the pinned linter. The pinned VERSION itself lives in
# tools/lint/go.mod -- see the $(LINT_BIN) rule below.
LINT_DIR = $(CURDIR)/.bin
LINT_BIN = $(LINT_DIR)/golangci-lint
LINT_ARGS ?=

.PHONY: build-linux build-windows build-macos build-wasm build-ios build-android build-examples release clean test test-race vet lint lint-bin lint-cross cross-compile coverage-gate prepush check bench bench-gate deps-doc deps-doc-check security gosec govulncheck large-files deadcode generate-check tidy-check workflow-audit cov-report license-check ergonomics-audit ergonomics-audit-fix ergonomics-audit-fix-dry fmt-md fmt-md-check

# Desktop builds are cgo-free since the purego GL bindings (#155): the
# backend/gl uses X11/xgb + purego EGL on Linux and Win32 syscalls on
# Windows. Only macOS (Metal) and the mobile backends need cgo.
build-linux:
	CGO_ENABLED=0 \
	go build -ldflags "$(LDFLAGS)" \
	  -o build/showcase-linux ./examples/showcase/

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
	go build -ldflags "$(LDFLAGS)" \
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
	  -benchmem -count=5 -run='^$$' -timeout=15m ./gui/...

# Remove all build artifacts: the two artifact directories plus the stray
# root-level binaries produced by bare `go build ./examples/<name>/` or
# `go build ./tools/<name>/` before -o conventions were documented. New
# binaries belong in build/ or examples/bin/, never the repo root.
clean:
	rm -rf build/
	rm -rf $(LINT_DIR)
	rm -rf examples/bin/
	rm -f showcase fontviewer listbox get_started command_demo \
	  process_monitor scroll_demo depsdoc exportaudit \
	  ergonomics-audit readme_check

# Run all tests with explicit timeout.
# The gl backend must run cgo-free on Linux with a display: cgo + the
# LockOSThread-in-init pattern + purego EGL calls crash the runtime exit path
# (golang/go#80723). CGO_ENABLED=0 matches the backend's intended cgo-free
# build; it is a no-op on darwin (gl has no cgo files there).
test:
	CGO_ENABLED=0 go test -count=1 -timeout=5m ./gui/backend/gl/
	go test -count=1 -timeout=5m $$(go list ./... | grep -v '/gui/backend/gl')

# Run all tests with race detector enabled.
#
# gui/backend/gl is excluded rather than run cgo-free like the `test` target
# above: -race requires cgo, so CGO_ENABLED=0 and -race cannot both hold and
# the combination fails outright ("-race requires cgo"). Enabling cgo instead
# reintroduces the runtime exit crash that #162 pinned CGO_ENABLED=0 to avoid,
# so the package has no race-testable configuration on Linux at all. Nothing
# is lost that CI covers -- it runs `make lint`/`vet`, never this target.
test-race:
	go test -race -count=1 -timeout=10m $$(go list ./... | grep -v '/gui/backend/gl')

# Run go vet static analysis.
vet:
	go vet ./...
	go run ./tools/requiredid/cmd/requiredid ./...

# Build the pinned linter on demand, into a repo-local bin.
#
# The version lives in tools/lint/go.mod and nowhere else, so local and CI
# cannot drift -- the old scheme pinned v2.13.1 here and v2.13 (floating
# patch) in six CI steps, and checked the pin with a substring grep that
# also accepted 2.13.10.
#
# tools/lint is a SEPARATE module on purpose: a `tool` directive in the root
# go.mod took it from 40 to 246 lines and go.sum from 119 to 997, and every
# downstream sibling (go-charts, go-edit, go-kite, go-term, go-map) would
# inherit that module graph for a linter they do not run.
#
# GOWORK=off because tools/lint sits inside the repo but is deliberately not
# a go.work member; without it go refuses to build a module the workspace
# does not use. The Go build cache makes every run after the first fast.
#
# GOOS/GOARCH/CGO_ENABLED/CC are neutralised for the BUILD of the linter:
# callers set them to pick the target being ANALYSED (lint-cross, and the
# CI ios/android jobs), and inheriting them here would cross-compile the
# linter itself into a binary the runner cannot execute. An empty GOOS or
# GOARCH means "host default" to the go command; CGO_ENABLED=0 makes the
# caller's CC irrelevant, which golangci-lint (pure Go) never needs.
$(LINT_BIN): tools/lint/go.mod tools/lint/go.sum
	GOWORK=off GOOS= GOARCH= CGO_ENABLED=0 GOFLAGS= GOBIN=$(LINT_DIR) \
	  go -C tools/lint install \
	  github.com/golangci/golangci-lint/v2/cmd/golangci-lint

# Named entry point for callers that want the binary without linting.
lint-bin: $(LINT_BIN)

# Run golangci-lint at the pinned version. LINT_ARGS passes extra flags
# through (CI's ios job needs --build-tags ios).
lint: $(LINT_BIN)
	$(LINT_BIN) run $(LINT_ARGS) ./...

# Lint the GOOS-conditional files the default (GOOS=linux or darwin)
# build cannot see: every //go:build windows/js file was unlinted before
# CI added these steps. Linting is a type-check, not an execution, so
# each target analyses fine from any host. On macOS the darwin run is a
# strict superset of CI's: it also covers the cgo metal files, which CI
# cannot compile from a Linux runner. Mirror of the CI vet job's lint
# steps (issue #292).
lint-cross: $(LINT_BIN)
	GOOS=windows $(LINT_BIN) run ./...
	GOOS=js GOARCH=wasm $(LINT_BIN) run ./...
	GOOS=darwin $(LINT_BIN) run ./...

# Cross-compile the whole module with no C toolchain on the host, for
# every desktop target CI guards (issue #292). A new cgo import anywhere
# fails this; gui/audio's default Linux output is the pure-Go PulseAudio
# sink, so the oto/ALSA sink (-tags otoaudio) is built explicitly to
# keep it from bit-rotting. Mirror of the CI vet job's build steps.
cross-compile:
	for target in linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
		echo "==> $$target"; \
		CGO_ENABLED=0 GOOS=$${target%/*} GOARCH=$${target#*/} \
		  go build ./...; \
	done
	go build -tags otoaudio ./gui/audio/...

# Enforce CI's coverage gates locally: 70% total over gui/ plus the
# per-package floors. The thresholds live in scripts/coverage-gate.sh,
# shared with the CI coverage job (issue #292). Scope matches CI's
# ./gui/..., unlike cov-report's ./... report.
coverage-gate:
	go test -count=1 -coverprofile=/tmp/go-gui-coverage-gate.out ./gui/...
	scripts/coverage-gate.sh /tmp/go-gui-coverage-gate.out

# Run non-duplicated validation steps for CI gate.
# test and lint run as separate CI jobs with OS matrices.
check: vet deps-doc-check large-files generate-check tidy-check fmt-md-check

# Run all validation steps: test, vet, lint, and gate checks.
check-all: test lint check

# Recommended full local validation before pushing (issue #292):
# approximates the CI matrix from one host — race tests, linux + cross-
# GOOS lint, cgo-free cross-compiles, coverage gate, export audit. The
# .githooks/pre-push hook runs make check-all; run prepush once per
# branch to cover the rest. Omissions vs CI, by design: OS-matrix runs,
# coverage diff and benchmark gates (need a main baseline), WASM node
# tests, iOS/Android vet+lint (Xcode/NDK), Windows smoke test, release
# packaging. `make check` is the fast gate when only gate checks are
# wanted.
prepush: test-race lint check lint-cross cross-compile coverage-gate export-audit

# Format every tracked Markdown file with Prettier.
#
# The wrap width and prose-wrap mode live in .prettierrc, so they are not
# retyped per invocation -- CHANGELOG.md drifted for exactly that reason.
# Uses npx, so no repo-local node_modules is required.
fmt-md:
	@npx --yes prettier --write $(shell git ls-files '*.md')

# Gate: fail if any tracked Markdown file is unformatted. Part of `check`,
# so the convention is enforced rather than remembered.
fmt-md-check:
	@npx --yes prettier --check $(shell git ls-files '*.md')

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

# Report the API-surface measurements behind
# docs/specs/developer-ergonomics.md, then gate on hand-rolled widget
# IDs (docs/specs/widget-id-scoping.md) and on plain zero-meaningful
# Cfg fields (the Opt[T] rule in CLAUDE.md, mode opt) and on raw
# Padding/Color literals (mode literals: a Padding{...} or Color{...}
# reads as unset and silently takes the theme default). Modes ids, opt
# and literals exit non-zero on any finding, so they run last: the
# reports above still print either way.
ergonomics-audit:
	go run ./tools/ergonomics-audit/ -mode focus -gui . .
	go run ./tools/ergonomics-audit/ -mode callbacks -gui . .
	go run ./tools/ergonomics-audit/ -mode opt .
	go run ./tools/ergonomics-audit/ -mode ids .
	go run ./tools/ergonomics-audit/ -mode literals .
	go run ./tools/ergonomics-audit/ -mode theme .
	go run ./tools/ergonomics-audit/ -mode a11y .
	go run ./tools/ergonomics-audit/ -mode visual .

# Insert a generated ID into every broken literal in this repo's tests
# and examples. Scoped away from gui/ deliberately: go-gui's own widget
# defects get hand-chosen IDs, because a shipped widget's ID is public
# identity rather than scaffolding. Run ergonomics-audit-fix-dry first and read the
# proposed IDs before letting it write.
ergonomics-audit-fix-dry:
	go run ./tools/ergonomics-audit/ -mode focus -gui . \
	  -fix-dry-run -fix-only '_test\.go$$|^examples/' .

ergonomics-audit-fix:
	go run ./tools/ergonomics-audit/ -mode focus -gui . \
	  -fix -fix-only '_test\.go$$|^examples/' .

# Gate the exported API surface (tools/exportaudit): every export in gui/
# must be referenced from outside gui/ — a consumer repo, an example, or
# the external test package — or carry a // exportaudit:keep marker.
# The consumer scan is authoritative: without the sibling repos an export
# used only there would misclassify, so the in-repo run is advisory.
# Only the "siblings missing" case is suppressed; a real gate failure
# (offenders found with siblings present) must exit non-zero.
export-audit:
	go run ./tools/exportaudit/ -mode gate -gui . .
	@if [ -d ../go-charts ] && [ -d ../go-edit ] && [ -d ../go-kite ] && [ -d ../go-term ] && [ -d ../go-map ]; then \
	  go run ./tools/exportaudit/ -mode gate -gui . \
	    ../go-charts ../go-edit ../go-kite ../go-term ../go-map; \
	else \
	  echo "exportaudit: sibling repos not found at ../go-*; run from a checkout with siblings present" >&2; \
	fi
