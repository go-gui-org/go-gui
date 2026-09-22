package gui

import (
	"reflect"
	"sync"
	"time"
	"unsafe"
)

// Animated theme switch (issue #753).
//
// A window with a transition set fades between themes instead of
// switching in one frame. Only colors blend. Sizes, radii, fonts and
// ext values snap to the target on the first frame, because blending
// them would re-run layout every frame and move things around while
// the fade runs. Colors are what a light/dark switch actually shows.
//
// Three rules keep the fade free of per-frame heap allocation:
//
//   - Every Color in Theme is found once, by a reflect walk, and kept
//     as a table of byte offsets. A frame blends by walking that
//     table with unsafe pointer math. A new Color field in any style
//     struct joins the table on its own, so nothing drifts.
//   - The blended theme lives in one scratch Theme per window, made
//     on the first fade and reused. It is never published through
//     w.theme: themeRef promises the pointed-to value is never
//     written through, and the scratch is rewritten each frame.
//   - Each blended frame takes a fresh theme id. installTheme needs it
//     to re-apply, and the caches keyed on Theme.id (markdown, RTF
//     layout, list heights) need it so they do not keep colors baked
//     under an earlier frame of the fade. Those caches rebuild each
//     fade frame; that cost lasts only as long as the fade.
//
// Colors behind a pointer (the *BoxShadow elevation and focus-ring
// values) are not in the table and snap with the other non-color
// fields. So do Themed subtrees, which install their own theme.

// themeFadeAnimationID names the tween that drives a fade.
const themeFadeAnimationID = "gui.theme.fade"

// themeFade is one window's fade state. Frame-thread only, like every
// other writer of the installed theme.
type themeFade struct {
	// from holds the colors on screen when the fade started. Copied,
	// not referenced: a fade that restarts mid-way starts from the
	// scratch, and the scratch is about to be overwritten.
	from Theme
	// scratch is the installed theme while the fade runs: the
	// target's non-color fields plus the blended colors.
	scratch Theme
	// to is the target, the value pinTheme published to w.theme.
	// A pointer is enough: published values are never written.
	to *Theme
	// t is the eased progress, 0 to 1.
	t float32
	// gen counts fades. The tween's callbacks run later through the
	// command queue, so a callback from a replaced or cancelled fade
	// can still arrive; it carries its own gen and is dropped.
	gen    uint64
	active bool
	// dirty is set when t moved and the scratch must be re-blended.
	dirty bool
}

var (
	themeColorOffsetsOnce sync.Once
	themeColorOffsets     []uintptr
)

// themeColorOffsetTable returns the byte offset, inside Theme, of
// every Color stored by value. Built once; read-only afterwards.
func themeColorOffsetTable() []uintptr {
	themeColorOffsetsOnce.Do(func() {
		themeColorOffsets = appendColorOffsets(
			nil, reflect.TypeFor[Theme](), 0)
	})
	return themeColorOffsets
}

// appendColorOffsets walks typ and appends base plus the offset of
// each Color it holds by value. Pointers, slices, maps, interfaces
// and strings are skipped: what they point at is shared with other
// themes, and writing through it would change those themes too.
func appendColorOffsets(out []uintptr, typ reflect.Type, base uintptr) []uintptr {
	if typ == reflect.TypeFor[Color]() {
		return append(out, base)
	}
	switch typ.Kind() {
	case reflect.Struct:
		for f := range typ.Fields() {
			out = appendColorOffsets(out, f.Type, base+f.Offset)
		}
	case reflect.Array:
		elem := typ.Elem()
		for i := range typ.Len() {
			out = appendColorOffsets(out, elem, base+uintptr(i)*elem.Size())
		}
	}
	return out
}

// blendThemeColors writes into dst every Color of from blended
// toward to by t. Only the colors are written; the caller fills the
// rest of dst (the fade copies the target in once, at the start).
//
// A color unset on either side has no value to blend from, so dst
// takes to's value unchanged.
func blendThemeColors(dst, from, to *Theme, t float32) {
	dp := unsafe.Pointer(dst)
	fp := unsafe.Pointer(from)
	tp := unsafe.Pointer(to)
	for _, off := range themeColorOffsetTable() {
		a := (*Color)(unsafe.Add(fp, off))
		b := (*Color)(unsafe.Add(tp, off))
		d := (*Color)(unsafe.Add(dp, off))
		if !a.set || !b.set {
			*d = *b
			continue
		}
		*d = RGBA(
			lerpU8(a.R, b.R, t), lerpU8(a.G, b.G, t),
			lerpU8(a.B, b.B, t), lerpU8(a.A, b.A, t),
		)
	}
}

// SetThemeTransition sets how long later theme changes on this window
// take to fade. It covers SetTheme and the changes
// FollowSystemAppearance applies. Zero, the default, switches in one
// frame, as before.
//
// Only colors fade. Sizes, radii, fonts and theme ext values take the
// new theme's values on the first frame. Window.Theme returns the new
// theme at once; only what is drawn travels.
//
// The fade is skipped on a window that has not drawn a frame yet,
// where there is nothing on screen to fade from, and when the backend
// reports reduced motion through PrefersReducedMotion.
//
// Frame-thread only, like SetTheme: call from main before Run or from
// an event handler.
//
// exportaudit:keep — documented app API (issue #753); siblings adopt post-release.
func (w *Window) SetThemeTransition(d time.Duration) {
	w.themeTransition = max(d, 0)
}

// prefersReducedMotion reports the backend's reduced-motion setting.
// Backends opt in by implementing PrefersReducedMotion; without it
// the answer is false (no preference), as for the SVG path.
func (w *Window) prefersReducedMotion() bool {
	rm, ok := w.nativePlatform.(interface{ PrefersReducedMotion() bool })
	return ok && rm.PrefersReducedMotion()
}

// startThemeFade begins a fade from what is on screen now to target,
// which pinTheme has just published. Reports false when the change
// must snap instead; the caller then installs target directly.
func (w *Window) startThemeFade(prev, target *Theme) bool {
	d := w.themeTransition
	if d <= 0 || w.frameCount == 0 || prev == nil || w.prefersReducedMotion() {
		w.cancelThemeFade()
		return false
	}
	f := w.themeFade
	// Re-pinning the same theme (an OS appearance event that did not
	// change light/dark) changes no color. A fade would still mint a
	// fresh theme id every frame and drop every cache keyed on it for
	// the whole transition. A running fade already heading there keeps
	// going; otherwise the caller installs target, a no-op re-install.
	// id 0 marks a theme built outside ThemeMaker, which can not be
	// compared this way.
	if target.id != 0 {
		if f != nil && f.active {
			if f.to.id == target.id {
				f.to = target
				return true
			}
		} else if prev.id == target.id {
			return false
		}
	}
	if f == nil {
		f = &themeFade{}
		w.themeFade = f
	}
	// Start from what the window shows: the current blend when a
	// fade is already running, otherwise the previous theme.
	if f.active {
		f.from = f.scratch
	} else {
		f.from = *prev
	}
	f.scratch = *target
	f.to = target
	f.t = 0
	f.gen++
	f.active = true
	f.dirty = true

	gen := f.gen
	w.AnimationAdd(&TweenAnimation{
		AnimID:   themeFadeAnimationID,
		Duration: d,
		Easing:   EaseInOutCubic,
		From:     0,
		To:       1,
		OnValue:  func(v float32, win *Window) { win.themeFadeStep(gen, v) },
		OnDone:   func(win *Window) { win.themeFadeDone(gen) },
	})
	w.installTheme()
	return true
}

// cancelThemeFade drops a running fade. The caller installs whatever
// theme comes next.
func (w *Window) cancelThemeFade() {
	f := w.themeFade
	if f == nil || !f.active {
		return
	}
	f.active = false
	f.to = nil
	w.AnimationRemove(themeFadeAnimationID)
}

// themeFadeStep records the tween's progress. Runs on the frame
// thread through the command queue; the tween also marks a layout
// refresh, so the next frame re-blends and regenerates the view.
func (w *Window) themeFadeStep(gen uint64, v float32) {
	f := w.themeFade
	if f == nil || !f.active || f.gen != gen {
		return
	}
	f.t = v
	f.dirty = true
}

// themeFadeDone ends the fade and installs the target itself, so the
// steady state carries the target's own theme id again.
func (w *Window) themeFadeDone(gen uint64) {
	f := w.themeFade
	if f == nil || !f.active || f.gen != gen {
		return
	}
	target := f.to
	f.active = false
	f.to = nil
	if needsInstall(target) {
		applyTheme(target)
	}
	w.InvalidateLayout()
}

// installFadeTheme installs the blended scratch when a fade is
// running and reports whether it did. A frame whose progress did not
// move re-applies the same scratch (same id) only when another window
// installed its theme in between.
func (w *Window) installFadeTheme() bool {
	f := w.themeFade
	if f == nil || !f.active {
		return false
	}
	if f.dirty {
		blendThemeColors(&f.scratch, &f.from, f.to, f.t)
		f.scratch.id = nextThemeID()
		f.dirty = false
	}
	if needsInstall(&f.scratch) {
		applyTheme(&f.scratch)
	}
	return true
}
