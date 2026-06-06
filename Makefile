VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS  = -X github.com/go-gui-org/go-gui/gui.Version=$(VERSION) \
           -X github.com/go-gui-org/go-gui/gui.Commit=$(COMMIT)

CC_WINDOWS ?= x86_64-w64-mingw32-gcc
STATIC_TAG  = static

.PHONY: build-linux build-windows build-macos build-wasm release clean

build-linux:
	CGO_ENABLED=1 \
	go build -tags $(STATIC_TAG) -ldflags "$(LDFLAGS)" \
	  -o build/showcase-linux ./examples/showcase/

build-windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$(CC_WINDOWS) \
	go build -tags $(STATIC_TAG) -ldflags "$(LDFLAGS)" \
	  -o build/showcase-windows.exe ./examples/showcase/

build-macos:
	CGO_ENABLED=1 \
	go build -tags $(STATIC_TAG) -ldflags "$(LDFLAGS)" \
	  -o build/showcase-macos ./examples/showcase/

build-wasm:
	GOOS=js GOARCH=wasm \
	go build -ldflags "$(LDFLAGS)" \
	  -o build/showcase.wasm ./examples/showcase/
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" build/

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

clean:
	rm -rf build/
