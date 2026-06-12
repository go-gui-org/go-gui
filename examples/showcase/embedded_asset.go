//go:build !(js && wasm)

package main

import (
	"os"
	"path/filepath"
)

var embeddedPathCache = map[string]string{}

// embeddedAssetPath extracts an embedded binary asset to a temp
// file and returns the path. Cached so the file is written once.
func embeddedAssetPath(name string) string {
	if p, ok := embeddedPathCache[name]; ok {
		return p
	}
	data, err := showcaseFS.ReadFile(name)
	if err != nil {
		return ""
	}
	f, err := os.CreateTemp("", "showcase-*"+filepath.Ext(name))
	if err != nil {
		return ""
	}
	_, err = f.Write(data)
	_ = f.Close()
	if err != nil {
		_ = os.Remove(f.Name())
		return ""
	}
	embeddedPathCache[name] = f.Name()
	return f.Name()
}

// cleanupEmbeddedAssets removes temp files created by embeddedAssetPath.
func cleanupEmbeddedAssets() {
	for _, path := range embeddedPathCache {
		_ = os.Remove(path)
	}
	clear(embeddedPathCache)
}
