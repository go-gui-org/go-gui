//go:build ((linux && !android) || windows) && !js

package gl

import (
	"errors"
	"strings"
	"testing"
)

// TestReadRendererEnv pins how GOGUI_RENDERER is spelled (#1265). Only the
// exact values "soft" and "gl" name a renderer; matching is case-sensitive,
// as GOGUI_WAYLAND's is. Unset is auto-pick; anything else reads as gl.
func TestReadRendererEnv(t *testing.T) {
	cases := []struct {
		env  string
		want rendererEnv
	}{
		{"", rendererEnvAuto},
		{"gl", rendererEnvGL},
		{"soft", rendererEnvSoft},
		{"SOFT", rendererEnvGL},
		{"soft ", rendererEnvGL},
		{"llvmpipe", rendererEnvGL},
	}
	for _, c := range cases {
		t.Setenv("GOGUI_RENDERER", c.env)
		if got := readRendererEnv(); got != c.want {
			t.Errorf("GOGUI_RENDERER=%q: readRendererEnv = %v, want %v", c.env, got, c.want)
		}
	}
}

// TestSwizzleBGRA: premultiplied RGBA in, B G R A out. For a depth-24 visual
// the fourth byte is padding the server ignores; for depth 32 it is the
// premultiplied alpha the compositor reads, so it is kept either way.
func TestSwizzleBGRA(t *testing.T) {
	src := []byte{
		10, 20, 30, 255,
		1, 2, 3, 4,
		0, 0, 0, 0,
	}
	dst := make([]byte, len(src))
	swizzleBGRA(dst, src)
	want := []byte{
		30, 20, 10, 255,
		3, 2, 1, 4,
		0, 0, 0, 0,
	}
	for i := range want {
		if dst[i] != want[i] {
			t.Fatalf("byte %d = %d, want %d (got %v)", i, dst[i], want[i], dst)
		}
	}
	// A short dst writes only the whole pixels it can hold.
	short := make([]byte, 6)
	swizzleBGRA(short, src)
	if short[0] != 30 || short[4] != 0 {
		t.Fatalf("short dst = %v, want first pixel only", short)
	}
}

// TestGPUContextErrorNamesSoft: on Linux and Windows the no-GPU message
// points at GOGUI_RENDERER=soft, the one fix that needs no driver or package
// install. It stays a single line (TestGPUContextErrorMessageNamesFix).
func TestGPUContextErrorNamesSoft(t *testing.T) {
	msg := newGPUContextError("init EGL display",
		errors.New("eglInitialize failed")).Error()
	if !strings.Contains(msg, "GOGUI_RENDERER=soft") {
		t.Fatalf("message %q does not name GOGUI_RENDERER=soft", msg)
	}
	if strings.Contains(msg, "\n") {
		t.Fatalf("message must stay one line, got %q", msg)
	}
}
