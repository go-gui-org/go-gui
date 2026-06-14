VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS  = -X github.com/go-gui-org/go-gui/gui.Version=$(VERSION) \
           -X github.com/go-gui-org/go-gui/gui.Commit=$(COMMIT)

CC_WINDOWS ?= x86_64-w64-mingw32-gcc
STATIC_TAG  = static,audio

.PHONY: build-linux build-windows build-macos build-wasm build-ios build-android build-examples release clean test vet lint check bench bench-gate deps-doc deps-doc-check

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
	bash scripts/bundle-windows-dlls.sh
	cd build && zip go-gui-showcase-$(VERSION)-windows-amd64.zip \
	  showcase-windows.exe dlls/flat/*.dll
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
	  -bench='Benchmark(Layout|GenerateViewLayout|ParseSvg|Tessellate|BuildDefsPathDataCache|RenderLayout|RenderSvg)' \
	  -benchmem -count=5 -run='^$' -timeout=15m ./gui/...

clean:
	rm -rf build/

# Run all tests.
test:
	go test ./...

# Run go vet static analysis.
vet:
	go vet ./...

# Run golangci-lint (requires golangci-lint installed).
lint:
	golangci-lint run ./...

# Run all validation steps: test, vet, lint.
check: test vet lint deps-doc-check

# Regenerate docs/dependencies.md from go.mod.
deps-doc:
	go run ./tools/depsdoc/ -w

# Check that docs/dependencies.md is up to date with go.mod.
deps-doc-check:
	go run ./tools/depsdoc/ > /tmp/deps-generated.md
	diff docs/dependencies.md /tmp/deps-generated.md || \
	  { echo "::error::docs/dependencies.md is out of date. Run 'make deps-doc'." >&2; exit 1; }
	rm -f /tmp/deps-generated.md
