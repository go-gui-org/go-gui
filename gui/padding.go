package gui

// Padding constants.
const (
	PadXSmall = 4
	PadSmall  = 6
	PadMedium = 14
	PadLarge  = 22
)

// Predefined paddings.
var (
	// PaddingNone is explicitly zero padding: it is set, so it does NOT
	// fall through to the theme default like Padding{} (unset) does.
	PaddingNone    = Padding{set: true}
	paddingThree   = PadAll(3)
	PaddingTwoFive = NewPadding(2, 5, 2, 5)
	paddingXSmall  = PadAll(PadXSmall)
	PaddingSmall   = PadAll(PadSmall)
	paddingMedium  = PadAll(PadMedium)
	PaddingLarge   = PadAll(PadLarge)

	// paddingButton is the inset of a Button. Deliberately not a ladder
	// rung: a button's label is its whole content, so it gets more
	// horizontal room than a field (5 vertical / 12 horizontal, against
	// paddingField's 5/10), and at body 14 both land at the same ~31px
	// control height — the invariant that keeps a Button, an Input and
	// a Select in one row level (visual-refresh §3.1).
	paddingButton = NewPadding(5, 12, 5, 12)

	// paddingField is the inset a text-bearing form control puts
	// around its text — the seed for ThemeCfg.PaddingField.
	//
	// Six field widgets each picked their own: Input, NumericInput and
	// ColorFields at (2,4), Select, Combobox and InputDate at (5,5).
	// The vertical half is what makes a Select six pixels taller than
	// the Input beside it in the same form (issue #335, audit section
	// 7), so one tier is the fix.
	//
	// Chosen explicitly rather than built from the padding ladder: the
	// ladder's rungs (4/6/14/22) contain no 5 and no 10, and the field
	// inset answers a different question than a gap between things.
	// With body 14, 5 vertical gives the ~31px control height every
	// field shares (visual-refresh §3.1).
	paddingField = NewPadding(5, 10, 5, 10)
)

// Padding is the gap inside the edges of a Shape. Parameter order
// matches CSS: top, right, bottom, left.
//
// Padding self-flags: the unexported set field distinguishes "not set"
// (zero value, theme default applies) from an explicitly-set value,
// including explicitly zero (PaddingNone). Build paddings with
// NewPadding, PadAll, or the predefined vars — never a raw Padding{...}
// literal, which reads as unset. Use IsSet() to check, Or(def) to read.
type Padding struct {
	Top    float32
	Right  float32
	Bottom float32
	Left   float32
	set    bool
}

// NewPadding creates a Padding with the given values.
func NewPadding(top, right, bottom, left float32) Padding {
	return Padding{Top: top, Right: right, Bottom: bottom, Left: left, set: true}
}

// Width returns left + right.
func (p Padding) Width() float32 {
	return p.Left + p.Right
}

// Height returns top + bottom.
func (p Padding) Height() float32 {
	return p.Top + p.Bottom
}

// IsSet reports whether the padding was explicitly set (via a
// constructor or predefined var) as opposed to being the zero value.
func (p Padding) IsSet() bool {
	return p.set
}

// Or returns p if it is set, otherwise def. This is the read-side
// replacement for Padding.Get(def).
func (p Padding) Or(def Padding) Padding {
	if p.set {
		return p
	}
	return def
}

// withSet returns a copy of p stamped as explicitly set. Used where a
// resolved value (a theme default) must not fall through to another
// default on a later IsSet check.
func (p Padding) withSet() Padding {
	p.set = true
	return p
}

// IsNone returns true if all sides are zero.
func (p Padding) isNone() bool {
	return p.Left == 0 && p.Right == 0 && p.Top == 0 && p.Bottom == 0
}

// PadAll creates a Padding with all sides equal.
func PadAll(p float32) Padding {
	return Padding{Top: p, Right: p, Bottom: p, Left: p, set: true}
}

// PadTBLR creates a Padding with top/bottom = tb and left/right = lr.
func padTBLR(tb, lr float32) Padding {
	return Padding{Top: tb, Right: lr, Bottom: tb, Left: lr, set: true}
}
