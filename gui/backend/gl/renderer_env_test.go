//go:build ((linux && !android) || windows) && !js

package gl

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
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

// TestSwizzleBGRAMatchesByteLoop: the two-pixel form writes what the
// per-pixel byte loop it replaced wrote (#1327), for every length around
// the two-pixel step: none, one odd pixel, bytes past the last whole pixel,
// a dst shorter or longer than src. Bytes it must not touch keep a guard
// value. The same lengths run in place, as a caller that owns one buffer
// may do.
func TestSwizzleBGRAMatchesByteLoop(t *testing.T) {
	byteLoop := func(dst, src []byte) {
		n := min(len(dst), len(src)) &^ 3
		for i := 0; i < n; i += 4 {
			dst[i], dst[i+1], dst[i+2], dst[i+3] = src[i+2], src[i+1], src[i], src[i+3]
		}
	}
	const guard = 0xa5
	// Seeded noise, the same for every call: a regular ramp ties the
	// channels of a pixel together, and then a wrong mask bit goes unseen.
	fill := func(n int) []byte {
		rng := rand.New(rand.NewPCG(1327, uint64(n)))
		p := make([]byte, n)
		for i := range p {
			p[i] = byte(rng.Uint32())
		}
		return p
	}
	for srcLen := 0; srcLen <= 41; srcLen++ {
		for _, dstLen := range []int{srcLen, srcLen + 5, max(srcLen-3, 0), max(srcLen-8, 0)} {
			src := fill(srcLen)
			got, want := make([]byte, dstLen), make([]byte, dstLen)
			for i := range got {
				got[i], want[i] = guard, guard
			}
			swizzleBGRA(got, src)
			byteLoop(want, src)
			if !bytes.Equal(got, want) {
				t.Fatalf("src %d bytes, dst %d bytes:\n got %v\nwant %v", srcLen, dstLen, got, want)
			}
			if !bytes.Equal(src, fill(srcLen)) {
				t.Fatalf("src %d bytes, dst %d bytes: src was written", srcLen, dstLen)
			}
		}
		inPlace, want := fill(srcLen), fill(srcLen)
		swizzleBGRA(inPlace, inPlace)
		byteLoop(want, fill(srcLen))
		if !bytes.Equal(inPlace, want) {
			t.Fatalf("in place, %d bytes:\n got %v\nwant %v", srcLen, inPlace, want)
		}
	}
}

// BenchmarkSwizzleBGRA times the conversion of one frame, which every
// presented soft frame pays on X11, Wayland and Windows.
func BenchmarkSwizzleBGRA(b *testing.B) {
	for _, sz := range [][2]int{{1920, 1080}, {3840, 2160}} {
		b.Run(fmt.Sprintf("%dx%d", sz[0], sz[1]), func(b *testing.B) {
			src := make([]byte, sz[0]*sz[1]*4)
			for i := range src {
				src[i] = byte(i)
			}
			dst := make([]byte, len(src))
			b.SetBytes(int64(len(src)))
			for b.Loop() {
				swizzleBGRA(dst, src)
			}
		})
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
