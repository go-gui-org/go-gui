package gui

// SizingType describes the three sizing modes.
type SizingType uint8

// SizingType constants.
const (
	SizingFit   SizingType = iota // element fits to content
	SizingFill                    // element fills to parent
	SizingFixed                   // element unchanged
)

// Sizing describes how a shape is sized horizontally and vertically.
type Sizing struct {
	Width  SizingType
	Height SizingType
}

// Predefined sizing combinations.
var (
	FitFit   = Sizing{SizingFit, SizingFit}
	FitFill  = Sizing{SizingFit, SizingFill}
	FitFixed = Sizing{SizingFit, SizingFixed}

	FixedFit   = Sizing{SizingFixed, SizingFit}
	FixedFill  = Sizing{SizingFixed, SizingFill}
	FixedFixed = Sizing{SizingFixed, SizingFixed}

	FillFit   = Sizing{SizingFill, SizingFit}
	FillFill  = Sizing{SizingFill, SizingFill}
	FillFixed = Sizing{SizingFill, SizingFixed}
)

// applyFixedSizingConstraints sets min = max = size when sizing is Fixed.
func applyFixedSizingConstraints(shape *Shape) {
	if shape.Sizing.Width == SizingFixed && shape.Width > 0 {
		shape.MinWidth = shape.Width
		shape.MaxWidth = shape.Width
	}
	if shape.Sizing.Height == SizingFixed && shape.Height > 0 {
		shape.MinHeight = shape.Height
		shape.MaxHeight = shape.Height
	}
}
