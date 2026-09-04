package gui

// SizeLimits carries a window's normalized resize bounds in the same
// logical pixels WindowCfg.Width and WindowCfg.Height use. A zero
// field means "unset": no floor, or no ceiling, on that axis. Each
// backend scales these to physical pixels itself, since the logical
// -> physical factor is the backend's own DPI state.
// exportaudit:keep — backend-facing (issue #494)
type SizeLimits struct {
	MinW int
	MinH int
	MaxW int
	MaxH int
}

// None reports whether nothing is constrained, so a backend can skip
// the platform call entirely rather than write a no-op hint.
// exportaudit:keep — backend-facing (issue #494)
func (l SizeLimits) None() bool {
	return l.MinW == 0 && l.MinH == 0 && l.MaxW == 0 && l.MaxH == 0
}

// WindowSizeLimits normalizes the four WindowCfg limit fields into the
// single form every backend consumes. Centralized so the three native
// backends cannot disagree about the edge cases.
//
// Rules, in order:
//
//   - FixedSize collapses both bounds onto Width/Height, so a fixed
//     window is expressed as min == max. This is what lets the X11
//     backend honor FixedSize at all: it has no resizable style bit,
//     only WM_NORMAL_HINTS. A FixedSize window with no usable
//     Width/Height falls through to the ordinary rules rather than
//     pinning the window to zero.
//   - A negative value is not a smaller window, it is a caller
//     mistake; treat it as unset rather than propagating it into a
//     platform call.
//   - A ceiling below its floor is contradictory. The floor is the
//     stronger promise (content stops fitting below it), so raise the
//     ceiling to meet it instead of dropping the floor.
//
// exportaudit:keep — backend-facing (issue #494)
func WindowSizeLimits(cfg WindowCfg) SizeLimits {
	if cfg.FixedSize && cfg.Width > 0 && cfg.Height > 0 {
		return SizeLimits{
			MinW: cfg.Width, MinH: cfg.Height,
			MaxW: cfg.Width, MaxH: cfg.Height,
		}
	}
	l := SizeLimits{
		MinW: clampUnset(cfg.MinWidth),
		MinH: clampUnset(cfg.MinHeight),
		MaxW: clampUnset(cfg.MaxWidth),
		MaxH: clampUnset(cfg.MaxHeight),
	}
	// Only a set ceiling can contradict a floor; an unset one stays
	// unset so the backend omits the max hint.
	if l.MaxW != 0 && l.MaxW < l.MinW {
		l.MaxW = l.MinW
	}
	if l.MaxH != 0 && l.MaxH < l.MinH {
		l.MaxH = l.MinH
	}
	return l
}

// maxWindowExtent is the largest bound any backend is handed, in
// pixels. It is the X11 ceiling -- drawable coordinates are 16-bit
// signed, so a larger hint wraps into nonsense -- and it doubles as the
// overflow guard for the Win32 int32 track sizes. No real display comes
// close, so a caller asking for more gets the ceiling rather than a
// wrapped value.
const maxWindowExtent = 1<<15 - 1

// clampUnset folds a negative size to zero ("unset") and holds a
// nonsense-large one down to what the platforms can express.
func clampUnset(v int) int {
	if v < 0 {
		return 0
	}
	return min(v, maxWindowExtent)
}

// Scaled returns the limits in physical pixels for a backend whose
// logical -> physical factor is s. Used by the X11 and Windows
// backends, which create windows in device pixels; macOS takes points
// and uses the unscaled values directly. A zero field stays zero so
// "unset" survives scaling. This is the only place the conversion
// lives, so the two backends cannot round a bound differently.
// exportaudit:keep — backend-facing (issue #494)
func (l SizeLimits) Scaled(s float32) SizeLimits {
	return SizeLimits{
		MinW: scaleLimit(l.MinW, s),
		MinH: scaleLimit(l.MinH, s),
		MaxW: scaleLimit(l.MaxW, s),
		MaxH: scaleLimit(l.MaxH, s),
	}
}

func scaleLimit(v int, s float32) int {
	if v == 0 {
		return 0
	}
	// Truncated rather than rounded, because that is exactly how the
	// X11 and Windows backends convert Width/Height. Rounding here
	// would let a floor equal to the created size land a pixel above
	// it, and the window would grow by that pixel the moment it opened
	// -- most visibly under FixedSize, which is expressed as a floor at
	// the created size. A sub-pixel scale must still not truncate a
	// real floor away to "unset", hence the 1; a high scale must not
	// carry a bound past what the platforms can express, hence the cap.
	return min(max(int(float32(v)*s), 1), maxWindowExtent)
}
