//go:build ignore

// Serve builds web_svg_bench for wasm and serves it, recording each
// result the page POSTs to /result in results.jsonl.
//
//	go run ./examples/web_svg_bench/serve.go [-addr :8080] [-nobuild]
//
// Then open http://localhost:8080/?post=1 (see bench.js for options).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "listen address")
	noBuild := flag.Bool("nobuild", false, "serve without rebuilding")
	flag.Parse()

	_, self, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("serve: cannot locate source directory")
	}
	dir := filepath.Dir(self)

	if !*noBuild {
		if err := build(dir); err != nil {
			log.Fatalf("serve: build: %v", err)
		}
	}

	results := filepath.Join(dir, "results.jsonl")
	http.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil || !json.Valid(body) {
			http.Error(w, "bad result", http.StatusBadRequest)
			return
		}
		var line bytes.Buffer
		if err = json.Compact(&line, body); err != nil {
			http.Error(w, "bad result", http.StatusBadRequest)
			return
		}
		line.WriteByte('\n')
		f, err := os.OpenFile(results, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()
		if _, err = f.Write(line.Bytes()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("result: %s", strings.TrimSpace(line.String()))
	})
	http.HandleFunc("/shot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		png, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
		if err != nil || !bytes.HasPrefix(png, []byte("\x89PNG")) {
			http.Error(w, "bad png", http.StatusBadRequest)
			return
		}
		shots := filepath.Join(dir, "shots")
		if err = os.MkdirAll(shots, 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out := filepath.Join(shots, safeName(r.URL.Query().Get("name"))+".png")
		if err = os.WriteFile(out, png, 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("shot: %s", out)
	})
	http.Handle("/", noCache(http.FileServer(http.Dir(dir))))

	log.Printf("serving %s on http://%s/?post=1", dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

// build compiles the example to main.wasm and copies the JS support
// files next to it, like examples/web_demo/Makefile does.
func build(dir string) error {
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "main.wasm"), ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return err
	}
	root := strings.TrimSpace(string(goroot))
	copies := [][2]string{
		{filepath.Join(root, "lib", "wasm", "wasm_exec.js"), "wasm_exec.js"},
		{filepath.Join(dir, "..", "..", "gui", "backend", "web", "gogui.js"), "gogui.js"},
	}
	for _, c := range copies {
		data, readErr := os.ReadFile(c[0])
		if readErr != nil {
			return readErr
		}
		if err = os.WriteFile(filepath.Join(dir, c[1]), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// safeName keeps a screenshot name to letters, digits, '-' and '_',
// so a label cannot name a path outside shots/.
func safeName(s string) string {
	b := []byte(s)
	for i, c := range b {
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '_') {
			b[i] = '_'
		}
	}
	if len(b) == 0 {
		return "shot"
	}
	return string(b)
}

// noCache stops the browser reusing a stale main.wasm between runs.
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}
