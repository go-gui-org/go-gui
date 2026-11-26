//go:build darwin && (!ios || cgo)

package gui

import (
	"strings"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// localeCF holds the two CFLocale functions, bound on first use.
// LocaleAutoDetect is a startup call, so the typed RegisterFunc form,
// which allocates per call, is fine.
var localeCF struct {
	once          sync.Once
	copyCurrent   func() uintptr
	getIdentifier func(locale uintptr) uintptr
}

// localeDetect returns the BCP 47 locale ID from the OS
// (CFLocaleCopyCurrent), falling back to environment variables. It
// uses purego, so it works with cgo off (#1307).
func localeDetect() string {
	localeCF.once.Do(func() {
		lib := darwinffi.Open(darwinffi.PathCoreFoundation)
		purego.RegisterLibFunc(&localeCF.copyCurrent, lib, "CFLocaleCopyCurrent")
		purego.RegisterLibFunc(&localeCF.getIdentifier, lib, "CFLocaleGetIdentifier")
	})
	locale := localeCF.copyCurrent()
	if locale == 0 {
		return localeFromEnv()
	}
	// The identifier follows the Get rule: locale owns it, so only the
	// locale is released, after the string is copied out.
	id := darwinffi.GoString(localeCF.getIdentifier(locale))
	darwinffi.CFRelease(locale)
	// Normalize en_US → en-US
	id = strings.ReplaceAll(id, "_", "-")
	if id == "" {
		return localeFromEnv()
	}
	return id
}
