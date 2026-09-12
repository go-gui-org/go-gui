//go:build js && wasm

package gui

import "encoding/base64"

// storeDiagramPNG base64-encodes PNG bytes and returns a
// data URL. WASM implementation. The hash and prefix stay in
// the signature for parity with the native version, which
// uses them for the temp file name.
func storeDiagramPNG(
	pngBytes []byte, _ int64, _ string,
) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	return "data:image/png;base64," + b64, nil
}

// removeDiagramPNG is a no-op in WASM (data URLs need no
// cleanup).
func removeDiagramPNG(_ string) {}
