//go:build darwin && !ios

package gui

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/ebitengine/purego"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// Issue #1307: localeDetect reads the OS locale through purego, so it
// works with cgo off. Before #1307, a CGO_ENABLED=0 build on macOS used
// the environment-only fallback. The OS setting is the AppleLocale
// default, which CFLocaleCopyCurrent reads, so the two must agree.
func TestLocaleDetectReadsOS(t *testing.T) {
	// LC_ALL wins in the fallback. A value that no Mac uses as its
	// locale shows whether the fallback ran instead of the OS call.
	t.Setenv("LC_ALL", "zz_ZZ")
	out, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output()
	if err != nil {
		t.Skipf("defaults read -g AppleLocale: %v", err)
	}
	want := strings.ReplaceAll(canonicalLocaleID(t, strings.TrimSpace(string(out))), "_", "-")
	if got := localeDetect(); got != want {
		t.Errorf("localeDetect() = %q, want %q (AppleLocale)", got, want)
	}
}

// canonicalLocaleID gives the form of raw that CFLocaleCopyCurrent
// reports. The AppleLocale text can be a legacy alias, and CoreFoundation
// gives the canonical ID for it (iw_IL becomes he_IL). Without this step,
// the test fails on such a Mac while the detection is correct.
func canonicalLocaleID(t *testing.T, raw string) string {
	t.Helper()
	var canon func(allocator, id uintptr) uintptr
	lib := darwinffi.Open(darwinffi.PathCoreFoundation)
	purego.RegisterLibFunc(&canon, lib, "CFLocaleCreateCanonicalLocaleIdentifierFromString")
	s := darwinffi.CFString(raw)
	if s == 0 {
		t.Fatalf("AppleLocale %q is not valid UTF-8", raw)
	}
	defer darwinffi.CFRelease(s)
	// The result follows the Create rule: the test owns it.
	c := canon(0, s)
	if c == 0 {
		t.Fatalf("CFLocaleCreateCanonicalLocaleIdentifierFromString(%q) = NULL", raw)
	}
	defer darwinffi.CFRelease(c)
	return darwinffi.GoString(c)
}
