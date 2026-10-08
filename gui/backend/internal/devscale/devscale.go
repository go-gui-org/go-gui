// Package devscale reads GOGUI_DEVICE_SCALE, a developer override for the
// device scale a backend reports (#971). With it, 2x layout and text bugs
// can be seen on a 1x monitor.
//
// It is a separate package so every desktop backend (Metal, GL on X11,
// Wayland and Win32, web) reads one parser, and the gui package gets no
// new exported surface.
package devscale

import (
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Env is the variable that holds the override, for example
// GOGUI_DEVICE_SCALE=2.
const Env = "GOGUI_DEVICE_SCALE"

// minScale and maxScale bound the override. maxScale matches the upper
// bound of the backends' applyDPIScale guard. minScale keeps a window's
// pixel size above 0: at 0.001 a Metal drawable or a Win32 client area
// truncates to 0x0.
const (
	minScale = 0.25
	maxScale = 8
)

var (
	once     sync.Once
	override float32
	set      bool
)

// Override returns the scale from GOGUI_DEVICE_SCALE, and true when the
// variable holds a usable value. It reads the environment once.
func Override() (float32, bool) {
	once.Do(func() { override, set = parse(os.Getenv(Env)) })
	return override, set
}

// Apply returns the override when one is set, otherwise platform: the
// scale the backend got from the system.
func Apply(platform float32) float32 {
	s, ok := Override()
	return apply(platform, s, ok)
}

func apply(platform, s float32, ok bool) float32 {
	if ok {
		return s
	}
	return platform
}

// parse accepts a finite number in [0.25, 8]. Any other value gives no
// override. A wrong value then falls back to the real scale, and does not
// break the window.
func parse(v string) (float32, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 32)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < minScale || f > maxScale {
		return 0, false
	}
	return float32(f), true
}

// Apply120 is Apply for a scale in 120ths, the unit of Wayland's
// fractional-scale-v1. The override is rounded to the nearest 120th, and
// is at least 1 so a buffer never has a size of 0.
//
// whole is for a surface with no viewport. Its scale goes to
// wl_surface.set_buffer_scale, which takes only a whole number of at
// least 1. So the override is rounded to a multiple of 120, at least 120.
// Without this, 0.5 sends a buffer scale of 0, a protocol error, and 1.5
// sizes the buffer at 1.5x while the compositor divides it by 1.
func Apply120(platform int32, whole bool) int32 {
	s, ok := Override()
	return apply120(platform, s, ok, whole)
}

func apply120(platform int32, s float32, ok, whole bool) int32 {
	if !ok {
		return platform
	}
	if whole {
		return max(int32(s+0.5), 1) * 120
	}
	return max(int32(s*120+0.5), 1)
}
