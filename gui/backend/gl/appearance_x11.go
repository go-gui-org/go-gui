//go:build linux && !js && !android

package gl

import (
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// OS light/dark setting on Linux (issue #752). Source is gsettings,
// no D-Bus dependency: the initial value comes from `gsettings get`
// and changes from the shared monitor child (appearance_monitor.go).
// GNOME-family only — KDE and bare window managers report no setting
// (accepted gap, see the #752 spec). A missing binary or schema also
// reads as no setting, and the app keeps its own theme.

// querySystemAppearance runs `gsettings get` once.
func querySystemAppearance() (gui.Appearance, bool) {
	out, err := exec.Command("gsettings", "get", gsettingsSchema, gsettingsKey).Output()
	if err != nil {
		return gui.AppearanceLight, false
	}
	return parseGsettingsColorScheme(string(out))
}

// SystemAppearance implements gui.NativePlatform.
func (n *nativePlatform) SystemAppearance() (gui.Appearance, bool) {
	return querySystemAppearance()
}

// PrefersReducedMotion reports the OS reduce-motion setting
// (issue #757). Source is the GNOME enable-animations key,
// inverted like the Windows SPI flag: animations off means the
// user asked for reduced motion. A missing binary or schema reads
// as no setting, and the app keeps animating, like the color-scheme
// query above. GNOME-family only; other desktops report no setting
// (accepted gap, same as #752).
//
// The reading is cached (issue #892). The render pass asks once per
// visible SVG per frame (LoadSvg → svgParseOpts), and a `gsettings`
// spawn per ask held the frame lock long enough to drop an
// SVG-heavy page from 60 to ~20 fps. The first call reads
// synchronously so the first SVG parse sees the real setting; after
// that a call never waits: it answers from the cache and, once the
// reading is older than reducedMotionTTL, starts one background
// refresh. A setting changed while the app runs shows up within
// about one TTL.
func (n *nativePlatform) PrefersReducedMotion() bool {
	return cachedPrefersReducedMotion()
}

// Cached readings. rmUnread marks a cold cache; the other three
// mirror the (reduced, ok) pair the query returns.
const (
	rmUnread  uint32 = iota
	rmNone           // no setting: binary or schema missing
	rmAnimate        // known: animations on
	rmReduced        // known: animations off
)

// reducedMotionTTL bounds how stale a cached reading may get before a
// call starts a refresh. A var so tests can shorten it.
var reducedMotionTTL = 2 * time.Second

// rmEpoch anchors rmCache.readAt. time.Since(rmEpoch) uses the
// monotonic clock, so a wall-clock jump cannot stall or force
// refreshes.
var rmEpoch = time.Now()

// rmCache is process-wide: gsettings has one value per session, not
// per window. Atomics keep the hot path lock- and allocation-free.
var rmCache struct {
	value      atomic.Uint32 // one of the rm* readings
	readAt     atomic.Int64  // time.Since(rmEpoch) at the last read
	refreshing atomic.Bool   // a background refresh is in flight
}

func cachedPrefersReducedMotion() bool {
	v := rmCache.value.Load()
	if v == rmUnread {
		// Cold: read inline once. Concurrent first callers may both
		// query; each stores the same answer, so that is harmless.
		storeReducedMotion(reducedMotionQuery())
		return rmCache.value.Load() == rmReduced
	}
	age := time.Since(rmEpoch) - time.Duration(rmCache.readAt.Load())
	if age >= reducedMotionTTL && rmCache.refreshing.CompareAndSwap(false, true) {
		// Capture the query before the goroutine starts so a test
		// swapping reducedMotionQuery never races the read.
		query := reducedMotionQuery
		go func() {
			defer rmCache.refreshing.Store(false)
			storeReducedMotion(query())
		}()
	}
	return v == rmReduced
}

// storeReducedMotion records one query result and its time.
func storeReducedMotion(reduced, ok bool) {
	v := rmNone
	switch {
	case ok && reduced:
		v = rmReduced
	case ok:
		v = rmAnimate
	}
	// readAt first: a reader that sees the new value then sees a
	// fresh time and does not start a needless refresh.
	rmCache.readAt.Store(int64(time.Since(rmEpoch)))
	rmCache.value.Store(v)
}

// resetReducedMotionCache empties the cache, waiting out any
// background refresh so it cannot land after the reset. Tests only.
func resetReducedMotionCache() {
	for rmCache.refreshing.Load() {
		time.Sleep(time.Millisecond)
	}
	rmCache.value.Store(rmUnread)
	rmCache.readAt.Store(0)
}

// enableAnimationsSchema and enableAnimationsKey name the gsettings
// key reporting whether desktop animations are enabled.
const (
	enableAnimationsSchema = "org.gnome.desktop.interface"
	enableAnimationsKey    = "enable-animations"
)

// queryPrefersReducedMotion runs `gsettings get` once for the
// enable-animations key.
func queryPrefersReducedMotion() (bool, bool) {
	out, err := exec.Command("gsettings", "get", enableAnimationsSchema, enableAnimationsKey).Output()
	if err != nil {
		return false, false
	}
	return parseGsettingsEnableAnimations(string(out))
}

// reducedMotionQuery reads the OS setting. A var so tests can feed
// a reading without spawning gsettings.
var reducedMotionQuery = queryPrefersReducedMotion

// SetSystemAppearanceCallback implements gui.NativePlatform. The
// shared monitor runs while at least one window is subscribed; a nil
// cb unregisters.
func (n *nativePlatform) SetSystemAppearanceCallback(cb func(gui.Appearance)) {
	setAppearanceCallback(n, cb)
}
