//go:build darwin && !ios

package gui

import (
	"os/exec"
	"strings"
	"testing"
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
	want := strings.ReplaceAll(strings.TrimSpace(string(out)), "_", "-")
	if got := localeDetect(); got != want {
		t.Errorf("localeDetect() = %q, want %q (AppleLocale)", got, want)
	}
}
