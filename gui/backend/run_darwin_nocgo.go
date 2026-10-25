//go:build darwin && !cgo && !ios

package backend

import (
	_ "unsafe" // for go:linkname

	"github.com/go-gui-org/go-gui/gui"
)

// The macOS backend is written in Objective-C and needs cgo. Without cgo
// there is no backend, and before #1159 this build linked and then
// panicked at launch.
//
// Run and RunApp call a function that has no body. Its go:linkname
// target is a symbol that does not exist, so the linker stops any app
// that reaches Run with:
//
//	relocation target .../gui/backend.macOS_requires_cgo__set_CGO_ENABLED_1 not defined
//
// A plain compile error (an undefined name) would also stop the build,
// but it would break type-check-only runs. CI lints darwin with cgo off
// from a Linux runner, and go vet and golangci-lint must still pass
// there. A link error leaves them alone and stops only real app builds.

// Run is not available on darwin without cgo. The build fails at link.
func Run(*gui.Window) { macOSRequiresCgo() }

// RunApp is not available on darwin without cgo. The build fails at link.
func RunApp(*gui.App, ...*gui.Window) { macOSRequiresCgo() }

//go:linkname macOSRequiresCgo github.com/go-gui-org/go-gui/gui/backend.macOS_requires_cgo__set_CGO_ENABLED_1
func macOSRequiresCgo()
