// Package darwinffi calls macOS system frameworks from Go with purego,
// so the code that uses it needs no cgo (#1304, #1307).
//
// It holds the parts that every ported macOS service needs: framework
// loading, symbol lookup, CoreFoundation strings, Objective-C classes
// and selectors, NSString and autorelease pools. The services that use
// it are listed in docs/specs/cgo-free-backend-feasibility.md § Port
// order.
//
// All other files build only for darwin, which includes iOS. On iOS,
// purego itself needs cgo, so an iOS build keeps CGO_ENABLED=1; the
// Metal renderer, the Keychain and the locale call still go through
// this package there (#1342). This file has no build tag, so
// `go vet ./...` on any target finds a package.
package darwinffi
