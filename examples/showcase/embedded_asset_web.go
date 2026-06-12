//go:build js && wasm

package main

import (
	"encoding/base64"
	"path/filepath"
	"strings"
)

var embeddedPathCache = map[string]string{}

// embeddedAssetPath returns a base64 data URL for the named
// embedded asset. In the browser, Image elements load data URLs
// directly — no filesystem needed.
func embeddedAssetPath(name string) string {
	if p, ok := embeddedPathCache[name]; ok {
		return p
	}
	data, err := showcaseFS.ReadFile(name)
	if err != nil {
		return ""
	}
	mime := "application/octet-stream"
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".png":
		mime = "image/png"
	}
	url := "data:" + mime + ";base64," +
		base64.StdEncoding.EncodeToString(data)
	embeddedPathCache[name] = url
	return url
}

// No-op: web build uses data URLs, no temp files to remove.
func cleanupEmbeddedAssets() {}
