// Package darwinffi calls macOS system frameworks from Go with purego,
// so the code that uses it needs no cgo (#1304, #1307).
//
// It holds the parts that every ported macOS service needs: framework
// loading, symbol lookup, CoreFoundation strings, Objective-C classes
// and selectors, NSString and autorelease pools. The services that use
// it are listed in docs/specs/cgo-free-backend-feasibility.md § Port
// order.
//
// All other files build only for darwin && !ios. iOS stays on cgo. This
// file has no build tag, so `go vet ./...` on any target finds a package.
package darwinffi
