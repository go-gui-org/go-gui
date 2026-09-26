build:
	GOOS=js GOARCH=wasm go build -o main.wasm .
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" .
	cp ../../gui/backend/web/gogui.js .

serve: build
	python3 -m http.server 8080

clean:
	rm -f main.wasm wasm_exec.js gogui.js

.PHONY: build serve clean
