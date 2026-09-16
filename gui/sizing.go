package gui

// SizingType describes the three sizing modes.
type sizingType uint8

// SizingType constants.
const (
	sizingFit   sizingType = iota // element fits to content
	sizingFill                    // element fills to parent
	sizingFixed                   // element unchanged
)

// Sizing describes how a shape is sized horizontally and vertically.
//
// Sizing self-flags: the unexported set field distinguishes "not set"
// (zero value, widget default applies) from an explicitly-set value.
// The zero value is FitFit — a legitimate combination — so the flag is
// what keeps an explicit `Sizing: FitFit` from being mistaken for
// unset. Build sizings with the predefined vars — never a raw
// Sizing{...} literal, which reads as unset. Use IsSet() to check,
// Or(def) to read. The set field is only consulted at cfg resolution;
// Shape.Sizing is consumed by field value, so it is never read there.
type Sizing struct {
	Width  sizingType
	Height sizingType
	set    bool
}

// Predefined sizing combinations.
var (
	FitFit   = Sizing{sizingFit, sizingFit, true}
	FitFill  = Sizing{sizingFit, sizingFill, true}
	FitFixed = Sizing{sizingFit, sizingFixed, true}

	FixedFit   = Sizing{sizingFixed, sizingFit, true}
	FixedFill  = Sizing{sizingFixed, sizingFill, true}
	FixedFixed = Sizing{sizingFixed, sizingFixed, true}

	FillFit   = Sizing{sizingFill, sizingFit, true}
	FillFill  = Sizing{sizingFill, sizingFill, true}
	FillFixed = Sizing{sizingFill, sizingFixed, true}
)

// IsSet reports whether the sizing was explicitly set (via a predefined
// var) as opposed to being the zero value.
func (s Sizing) IsSet() bool {
	return s.set
}

// Or returns s if it is set, otherwise def. This is the read-side
// replacement for the `s == (Sizing{})` zero-sentinel check.
func (s Sizing) Or(def Sizing) Sizing {
	if s.set {
		return s
	}
	return def
}

// applyFixedSizingConstraints sets min = max = size when sizing is Fixed.
//
// A Fixed axis discards the caller's stated Min and Max on that axis:
// the pin overwrites them, so they never take effect. Warn first with
// warnFixedSizingConflict, which reports a conflicting stated bound
// through the DebugSizing category (issue #635).
func applyFixedSizingConstraints(shape *Shape) {
	if shape.Sizing.Width == sizingFixed && shape.Width > 0 {
		shape.MinWidth = shape.Width
		shape.MaxWidth = shape.Width
	}
	if shape.Sizing.Height == sizingFixed && shape.Height > 0 {
		shape.MinHeight = shape.Height
		shape.MaxHeight = shape.Height
	}
}

// warnFixedSizingConflict reports a Fixed axis that also states Min or
// Max, which applyFixedSizingConstraints overwrites with the size.
//
// Call before applyFixedSizingConstraints, while the stated bounds are
// still visible: after the pin Min == Max == size on every Fixed axis,
// so the two states are indistinguishable. Only a conflicting bound
// reports — one equal to the size is redundant but harmless, and a
// zero or negative bound means unset. A Fixed axis with no positive
// size degrades to content sizing (issue #94) and keeps its bounds,
// so it stays quiet too.
//
// Generation-time check, like the text-truncation warn in view_text.go:
// the pipeline never sees the stated bounds. The mask guard keeps the
// off-state cost at one atomic load, matching layoutOverflow.
func warnFixedSizingConflict(w *Window, s *Shape) {
	if w == nil || s == nil {
		return
	}
	if DebugCategory(debugMask.Load())&DebugSizing == 0 {
		return
	}
	widthConflict := s.Sizing.Width == sizingFixed && s.Width > 0 &&
		((s.MinWidth > 0 && s.MinWidth != s.Width) ||
			(s.MaxWidth > 0 && s.MaxWidth != s.Width))
	heightConflict := s.Sizing.Height == sizingFixed && s.Height > 0 &&
		((s.MinHeight > 0 && s.MinHeight != s.Height) ||
			(s.MaxHeight > 0 && s.MaxHeight != s.Height))
	if !widthConflict && !heightConflict {
		return
	}
	axis := "width"
	if widthConflict && heightConflict {
		axis = "width and height"
	} else if heightConflict {
		axis = "height"
	}
	key := s.idKey()
	w.debugWarn(debugCheckFixedSizing, key,
		"shape %q is Fixed on the %s but also states Min/Max; "+
			"Fixed pins Min = Max = size, so the stated bounds "+
			"are ignored (issue #635)", key, axis)
}
