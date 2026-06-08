package gui

// Padding constants.
const (
	PadXSmall = 3
	PadSmall  = 5
	PadMedium = 10
	PadLarge  = 15
)

// Predefined paddings.
var (
	PaddingNone    = Padding{}
	paddingThree   = PadAll(3)
	paddingTwoFour = NewPadding(2, 4, 2, 4)
	PaddingTwoFive = NewPadding(2, 5, 2, 5)
	PaddingXSmall  = PadAll(PadXSmall)
	PaddingSmall   = PadAll(PadSmall)
	PaddingMedium  = PadAll(PadMedium)
	PaddingLarge   = PadAll(PadLarge)
	PaddingButton  = PadAll(6)
)

// Padding is the gap inside the edges of a Shape. Parameter order
// matches CSS: top, right, bottom, left.
// Use Opt[Padding] in config structs to distinguish "not set" from
// explicitly zero padding.
type Padding struct {
	Top    float32
	Right  float32
	Bottom float32
	Left   float32
}

// NewPadding creates a Padding with the given values.
func NewPadding(top, right, bottom, left float32) Padding {
	return Padding{Top: top, Right: right, Bottom: bottom, Left: left}
}

// Width returns left + right.
func (p Padding) Width() float32 {
	return p.Left + p.Right
}

// Height returns top + bottom.
func (p Padding) Height() float32 {
	return p.Top + p.Bottom
}

// IsNone returns true if all sides are zero.
func (p Padding) IsNone() bool {
	return p.Left == 0 && p.Right == 0 && p.Top == 0 && p.Bottom == 0
}

// PadAll creates a Padding with all sides equal.
func PadAll(p float32) Padding {
	return Padding{Top: p, Right: p, Bottom: p, Left: p}
}

// PadTBLR creates a Padding with top/bottom = tb and left/right = lr.
func PadTBLR(tb, lr float32) Padding {
	return Padding{Top: tb, Right: lr, Bottom: tb, Left: lr}
}
