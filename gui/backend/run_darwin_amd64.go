//go:build darwin && !arm64 && !ios

package backend

import (
	_ "unsafe" // for go:linkname

	"github.com/go-gui-org/go-gui/gui"
)

// The macOS backend calls AppKit and Metal through purego, and its float
// and struct sends use an arm64 trampoline (gui/internal/darwinffi). It
// runs on Apple silicon only: darwin/amd64 (Intel Macs) was dropped in
// #1323 (decision in #1304).
//
// Run and RunApp call a function that has no body. Its go:linkname
// target is a symbol that does not exist, so the linker stops any app
// that reaches Run with:
//
//	relocation target .../gui/backend.macOS_requires_arm64__Intel_Macs_are_not_supported not defined
//
// A plain compile error (an undefined name) would also stop the build,
// but it would break type-check-only runs. CI lints darwin/amd64 from a
// Linux runner, and go vet and golangci-lint must still pass there. A
// link error leaves them alone and stops only real app builds.

// Run is not available on darwin/amd64. The build fails at link.
func Run(*gui.Window) { macOSRequiresArm64() }

// RunApp is not available on darwin/amd64. The build fails at link.
func RunApp(*gui.App, ...*gui.Window) { macOSRequiresArm64() }

//go:linkname macOSRequiresArm64 github.com/go-gui-org/go-gui/gui/backend.macOS_requires_arm64__Intel_Macs_are_not_supported
func macOSRequiresArm64()
