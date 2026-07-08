//go:build linux && !js && !android

package gl

import (
	"testing"

	"github.com/jezek/xgb/xproto"
)

func TestFocusRealChange(t *testing.T) {
	cases := []struct {
		name         string
		mode, detail byte
		want         bool
	}{
		{"normal ancestor", xproto.NotifyModeNormal, xproto.NotifyDetailAncestor, true},
		{"normal nonlinear", xproto.NotifyModeNormal, xproto.NotifyDetailNonlinear, true},
		{"while-grabbed real", xproto.NotifyModeWhileGrabbed, xproto.NotifyDetailNonlinear, true},
		// Grab/ungrab churn from an interactive WM move/resize: must be ignored
		// so EventResized keeps flowing during a resize drag.
		{"grab", xproto.NotifyModeGrab, xproto.NotifyDetailNonlinear, false},
		{"ungrab", xproto.NotifyModeUngrab, xproto.NotifyDetailNonlinear, false},
		// Pointer-driven pseudo-focus: not a real keyboard-focus change.
		{"pointer detail", xproto.NotifyModeNormal, xproto.NotifyDetailPointer, false},
		{"pointer-root detail", xproto.NotifyModeNormal, xproto.NotifyDetailPointerRoot, false},
		{"none detail", xproto.NotifyModeNormal, xproto.NotifyDetailNone, false},
	}
	for _, c := range cases {
		if got := focusRealChange(c.mode, c.detail); got != c.want {
			t.Errorf("%s: focusRealChange(%d,%d)=%v, want %v",
				c.name, c.mode, c.detail, got, c.want)
		}
	}
}
