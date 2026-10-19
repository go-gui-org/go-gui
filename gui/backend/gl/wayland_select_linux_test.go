//go:build linux && !js && !android

package gl

import (
	"os"
	"testing"
)

// TestMain keeps the package's tests on X11 unless GOGUI_WAYLAND is set. An
// empty value counts as unset, as in waylandRequested: scripts/wayland/run.sh
// passes GOGUI_WAYLAND="" into the container when the caller sets none.
// Wayland is now the default on a Wayland session, so on a Linux desktop an
// X11 test that calls New would get a Wayland window it never destroys,
// which would leave the shared connection open for every test after it.
// Each Wayland test sets GOGUI_WAYLAND=1 itself.
func TestMain(m *testing.M) {
	if os.Getenv("GOGUI_WAYLAND") == "" {
		_ = os.Setenv("GOGUI_WAYLAND", "0")
	}
	os.Exit(m.Run())
}

// TestWaylandRequested pins how GOGUI_WAYLAND and WAYLAND_DISPLAY pick the
// backend. Wayland is the default on a Wayland session; GOGUI_WAYLAND=0 opts
// out and GOGUI_WAYLAND=1 forces a try. With no WAYLAND_DISPLAY and no
// GOGUI_WAYLAND=1, X11 is used: libwayland would otherwise connect to
// "wayland-0" and could find a stray socket in an X11 session.
func TestWaylandRequested(t *testing.T) {
	cases := []struct {
		env, display string
		want, forced bool
	}{
		{"", "", false, false},
		{"", "wayland-0", true, false},
		{"0", "wayland-0", false, false},
		{"0", "", false, false},
		{"1", "", true, true},
		{"1", "wayland-0", true, true},
		// An unknown value is the same as no value.
		{"yes", "wayland-0", true, false},
		{"yes", "", false, false},
	}
	for _, c := range cases {
		t.Setenv("GOGUI_WAYLAND", c.env)
		t.Setenv("WAYLAND_DISPLAY", c.display)
		if got := waylandRequested(); got != c.want {
			t.Errorf("GOGUI_WAYLAND=%q WAYLAND_DISPLAY=%q: waylandRequested = %v, want %v",
				c.env, c.display, got, c.want)
		}
		if got := waylandForced(); got != c.forced {
			t.Errorf("GOGUI_WAYLAND=%q: waylandForced = %v, want %v", c.env, got, c.forced)
		}
	}
}
