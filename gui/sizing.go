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
type Sizing struct {
	Width  sizingType
	Height sizingType
}

// Predefined sizing combinations.
var (
	FitFit   = Sizing{sizingFit, sizingFit}
	FitFill  = Sizing{sizingFit, sizingFill}
	FitFixed = Sizing{sizingFit, sizingFixed}

	FixedFit   = Sizing{sizingFixed, sizingFit}
	FixedFill  = Sizing{sizingFixed, sizingFill}
	FixedFixed = Sizing{sizingFixed, sizingFixed}

	FillFit   = Sizing{sizingFill, sizingFit}
	FillFill  = Sizing{sizingFill, sizingFill}
	FillFixed = Sizing{sizingFill, sizingFixed}
)

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
