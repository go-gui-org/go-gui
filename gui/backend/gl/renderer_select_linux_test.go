//go:build linux && !js && !android

package gl

import (
	"errors"
	"strings"
	"testing"
)

// TestSoftRequested pins GOGUI_RENDERER parsing (#1265). Only the exact value
// "soft" selects the CPU renderer; "gl", an empty value and anything else keep
// GL. Matching is case-sensitive, as GOGUI_WAYLAND's is. Step 3 of #1265 adds
// auto-pick, which changes only the unset case.
func TestSoftRequested(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"", false},
		{"gl", false},
		{"soft", true},
		{"SOFT", false},
		{"soft ", false},
		{"llvmpipe", false},
	}
	for _, c := range cases {
		t.Setenv("GOGUI_RENDERER", c.env)
		if got := softRequested(); got != c.want {
			t.Errorf("GOGUI_RENDERER=%q: softRequested = %v, want %v", c.env, got, c.want)
		}
	}
}

// TestSoftDeclinesWayland: until the Wayland soft path lands (#1265 step 4),
// GOGUI_RENDERER=soft runs through XWayland, so waylandRequested reports false
// even on a Wayland session and even when GOGUI_WAYLAND=1 forces a try.
func TestSoftDeclinesWayland(t *testing.T) {
	t.Setenv("GOGUI_RENDERER", "soft")
	for _, env := range []string{"", "1"} {
		t.Setenv("GOGUI_WAYLAND", env)
		t.Setenv("WAYLAND_DISPLAY", "wayland-0")
		if waylandRequested() {
			t.Errorf("GOGUI_RENDERER=soft GOGUI_WAYLAND=%q: waylandRequested = true, want false", env)
		}
	}
}

// TestGPUContextErrorNamesSoftOnLinux: on Linux the no-GPU message points at
// GOGUI_RENDERER=soft, the one fix that needs no driver or package install.
// It stays a single line (TestGPUContextErrorMessageNamesFix).
func TestGPUContextErrorNamesSoftOnLinux(t *testing.T) {
	msg := newGPUContextError("init EGL display",
		errors.New("eglInitialize failed")).Error()
	if !strings.Contains(msg, "GOGUI_RENDERER=soft") {
		t.Fatalf("message %q does not name GOGUI_RENDERER=soft", msg)
	}
	if strings.Contains(msg, "\n") {
		t.Fatalf("message must stay one line, got %q", msg)
	}
}
