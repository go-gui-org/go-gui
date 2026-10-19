//go:build js

package gui

import "net/http"

// imageHTTPClient on js/wasm is http.DefaultClient. Go's wasm
// transport sends requests through the browser's fetch API only
// while the transport has no custom dialer, so the dial guard of
// the native build (image_fetch_guard.go) cannot apply here. The
// browser does the connect, under its own rules.
var imageHTTPClient = http.DefaultClient
