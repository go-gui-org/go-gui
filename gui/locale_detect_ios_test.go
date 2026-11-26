//go:build ios && cgo

package gui

import "testing"

// TestIOSLocaleDetectReadsOS checks that iOS reads the locale through
// CFLocaleCopyCurrent (purego, #1342), not the environment fallback.
// LC_ALL is set to a value no device uses, so getting it back means
// the OS call did not run.
func TestIOSLocaleDetectReadsOS(t *testing.T) {
	t.Setenv("LC_ALL", "zz_ZZ")
	got := localeDetect()
	if got == "" || got == "zz-ZZ" {
		t.Errorf("localeDetect() = %q, want the OS locale", got)
	}
	t.Logf("locale: %s", got)
}
