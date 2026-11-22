package gui

import "testing"

// A link applies only an offset whose container is in this frame and
// scrolls. An offset left in the store by a container that is gone, or
// that stopped scrolling, must not move the linked container's
// children (#1293).
func TestScrollLinkIgnoresStaleTarget(t *testing.T) {
	for _, c := range []struct {
		name   string
		target func() View
	}{
		{"target gone", func() View { return nil }},
		{"target not scrollable", func() View {
			return Column(ContainerCfg{ID: "body", Sizing: FillFit, Padding: PaddingNone,
				SizeBorder: NoBorder, Content: []View{
					Rectangle(RectangleCfg{Width: 900, Height: 20, Sizing: FixedFixed}),
				}})
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
			w.TestRender(func(*Window) View {
				content := []View{
					Column(ContainerCfg{ID: "band", Sizing: FillFit, Clip: true,
						ScrollLinkX: "body", Padding: PaddingNone, SizeBorder: NoBorder,
						Content: []View{
							Rectangle(RectangleCfg{ID: "cell", Width: 900, Height: 20, Sizing: FixedFixed}),
						}}),
				}
				if v := c.target(); v != nil {
					content = append(content, v)
				}
				return Column(ContainerCfg{Sizing: FillFill, Padding: PaddingNone, Content: content})
			})
			// An offset an earlier frame's scrolling "body" left behind.
			w.scrollX().Set("body", -100)
			l := w.TestRender(nil)
			band, ok := l.FindByID("band")
			if !ok {
				t.Fatal("band not found")
			}
			cell, ok := l.FindByID(ScopeID("band", "cell"))
			if !ok {
				t.Fatal("cell not found")
			}
			if cell.Shape.X != band.Shape.X {
				t.Fatalf("cell x=%.0f, want the band's x=%.0f: a stale offset moved it",
					cell.Shape.X, band.Shape.X)
			}
		})
	}
}

// A live target moves the linked container's children by its offset.
func TestScrollLinkFollowsLiveTarget(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Padding: PaddingNone, Content: []View{
			Column(ContainerCfg{ID: "band", Sizing: FillFit, Clip: true,
				ScrollLinkX: "body", Padding: PaddingNone, SizeBorder: NoBorder,
				Content: []View{
					Rectangle(RectangleCfg{ID: "cell", Width: 900, Height: 20, Sizing: FixedFixed}),
				}}),
			Column(ContainerCfg{ID: "body", Scrollable: true, Sizing: FillFill,
				Padding: PaddingNone, SizeBorder: NoBorder, Content: []View{
					Rectangle(RectangleCfg{Width: 900, Height: 20, Sizing: FixedFixed}),
				}}),
		}})
	})
	w.TestRender(nil)
	w.ScrollHorizontalTo("body", -100)
	l := w.TestRender(nil)
	band, _ := l.FindByID("band")
	cell, _ := l.FindByID(ScopeID("band", "cell"))
	if got := cell.Shape.X - band.Shape.X; got != -100 {
		t.Fatalf("cell offset in the band = %.0f, want -100", got)
	}
}
