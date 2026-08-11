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
