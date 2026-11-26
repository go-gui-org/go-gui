package gui

import "testing"

// Issue #1358: a Canvas added its horizontal alignment to each child's
// X. Under RTL the default alignment resolves to the right, so every
// child moved right by its own share of the free space, and children
// of different widths moved by different amounts. A Canvas child sits
// at its own X from the left edge, whatever the direction and the
// alignment.
func TestCanvasIgnoresHAlign(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	for _, tc := range []struct {
		name  string
		dir   TextDirection
		align HorizontalAlign
	}{
		{"LTR start", TextDirLTR, HAlignStart},
		{"RTL start", TextDirRTL, HAlignStart},
		{"LTR center", TextDirLTR, HAlignCenter},
		{"LTR end", TextDirLTR, HAlignEnd},
		{"RTL end", TextDirRTL, HAlignEnd},
		{"RTL right", TextDirRTL, HAlignRight},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loc := saved
			loc.TextDir = tc.dir
			SetLocale(loc)
			w := NewTestWindow(t, WindowCfg{Width: 300, Height: 200})
			box := func(id string, x, width float32) View {
				return Column(ContainerCfg{ID: id, X: x, Width: width, Height: 20,
					Sizing: FixedFixed, Padding: PaddingNone, SizeBorder: NoBorder})
			}
			// Asymmetric padding: a column under RTL takes its left inset
			// from Padding.Right. A Canvas uses left-to-right coordinates,
			// so its children start after Padding.Left in either direction.
			w.TestRender(func(*Window) View {
				return Canvas(ContainerCfg{ID: "cv", Sizing: FillFill, HAlign: tc.align,
					Padding: NewPadding(0, 19, 0, 7), SizeBorder: NoBorder,
					Content: []View{box("a", 0, 100), box("b", 0, 50), box("c", 250, 40)}})
			})
			l := w.TestRender(nil)
			for _, want := range []struct {
				id string
				x  float32
			}{{"a", 7}, {"b", 7}, {"c", 257}} {
				s, ok := l.FindByID(w.ResolveID(want.id)[0])
				if !ok {
					t.Fatalf("%s not found", want.id)
				}
				nearF(t, want.id+" x", s.Shape.X, want.x, 0.01)
			}
		})
	}
}
