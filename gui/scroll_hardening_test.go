package gui

import (
	"math"
	"strings"
	"testing"
)

// Regression tests for the scroll review findings. Each one failed
// before its fix.

// paddedScrollLayout builds a 50x100 scrollable column with the given
// padding over a 400px-tall child. The viewport is 100-2*pad tall.
func paddedScrollLayout(pad float32) (*Layout, *Window) {
	layout, w := makeScrollLayout("s", 50, 100, 50, 400)
	layout.Shape.Padding = PadAll(pad)
	return layout, w
}

// A gutter press at the far end must land on the real end of the
// range. The range is measured against the viewport, which padding
// shrinks, so using the outer size stopped the list short by the
// padding.
func TestOffsetFromMouseYPaddingReachesEnd(t *testing.T) {
	layout, w := paddedScrollLayout(10)
	offsetFromMouseY(&w.layout, layout.Shape.Y+layout.Shape.Height-1, "s", w)
	got, _ := w.scrollY().Get("s")
	if want := scrollMaxOffsetY(layout); got != want {
		t.Fatalf("gutter end offset = %v, want %v", got, want)
	}
}

func TestOffsetFromMouseXPaddingReachesEnd(t *testing.T) {
	layout, w := makeScrollLayout("s", 100, 50, 400, 50)
	layout.Shape.Axis = axisLeftToRight
	layout.Shape.Padding = PadAll(10)
	offsetFromMouseX(&w.layout, layout.Shape.X+layout.Shape.Width-1, "s", w)
	got, _ := w.scrollX().Get("s")
	if want := scrollMaxOffsetX(layout); got != want {
		t.Fatalf("gutter end offset = %v, want %v", got, want)
	}
}

// ScrollbarVisible paints a gutter over content that fits. A press on
// it must not write a positive offset, which pushes the content down.
func TestOffsetFromMouseYContentFitsStaysZero(t *testing.T) {
	layout, w := makeScrollLayout("s", 50, 100, 50, 40)
	offsetFromMouseY(&w.layout, layout.Shape.Y+layout.Shape.Height-1, "s", w)
	if got, _ := w.scrollY().Get("s"); got != 0 {
		t.Fatalf("offset = %v, want 0 when content fits", got)
	}
}

// A NaN or Inf offset given to a public setter used to pass the clamp
// (f32Clamp lets NaN through) and then every child position became
// NaN. Each setter must ignore a non-finite value.
func TestScrollSettersRejectNonFinite(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(-1))
	for _, tc := range []struct {
		name string
		call func(*Window)
	}{
		{"ScrollVerticalTo", func(w *Window) { w.ScrollVerticalTo("s", nan) }},
		{"ScrollHorizontalTo", func(w *Window) { w.ScrollHorizontalTo("s", inf) }},
		{"scrollVerticalBy", func(w *Window) { w.scrollVerticalBy("s", nan) }},
		{"scrollHorizontalBy", func(w *Window) { w.scrollHorizontalBy("s", inf) }},
		{"ScrollVerticalToPct", func(w *Window) { w.ScrollVerticalToPct("s", nan) }},
		{"ScrollHorizontalToPct", func(w *Window) { w.ScrollHorizontalToPct("s", nan) }},
		{"ScrollVerticalTo miss", func(w *Window) { w.ScrollVerticalTo("gone", nan) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, w := makeScrollLayout("s", 100, 100, 400, 400)
			w.scrollX().Set("s", -10)
			w.scrollY().Set("s", -10)
			tc.call(w)
			for _, m := range []*BoundedMap[string, float32]{w.scrollX(), w.scrollY()} {
				for _, id := range []string{"s", "gone"} {
					if v, ok := m.Get(id); ok && !f32IsFinite(v) {
						t.Fatalf("%s stored %v for %q", tc.name, v, id)
					}
				}
			}
		})
	}
}

// A non-finite offset that reached the map by any route must not
// survive the pipeline's clamp: reset it to the start.
func TestLayoutAdjustScrollOffsetsResetsNonFinite(t *testing.T) {
	_, w := makeScrollLayout("s", 100, 100, 400, 400)
	w.scrollY().Set("s", float32(math.NaN()))
	w.scrollX().Set("s", float32(math.Inf(1)))
	layoutAdjustScrollOffsets(&w.layout, w)
	if v, _ := w.scrollY().Get("s"); v != 0 {
		t.Errorf("y offset = %v, want 0", v)
	}
	if v, _ := w.scrollX().Get("s"); v != 0 {
		t.Errorf("x offset = %v, want 0", v)
	}
}

// findScrollbar returns the scroller's bar of the given orientation.
func findScrollbar(t *testing.T, sc *Layout, o ScrollbarOrientation) *Layout {
	t.Helper()
	for i := range sc.Children {
		c := &sc.Children[i]
		if c.Shape.OverDraw && c.Shape.scrollbarOrientation == o {
			return c
		}
	}
	t.Fatal("scrollbar not found")
	return nil
}

// At the end of the range the thumb must touch the end of its track.
// The thumb position was divided by the track length after GapEnd took
// its insets off, not by the viewport, so with a small overflow the
// thumb stopped well short of the end (halfway at 4px of overflow).
func TestScrollbarThumbReachesTrackEnd(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 200})
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			ID:         "scroller",
			Sizing:     FillFill,
			Scrollable: true,
			ScrollMode: ScrollVerticalOnly,
			// No padding: the 5px overflow must not grow by the theme's
			// padding, or the shortfall becomes too small to see.
			Padding: NoPadding,
			ScrollbarCfgY: &ScrollbarCfg{
				Overflow: ScrollbarVisible,
			},
			Content: []View{
				Column(ContainerCfg{Sizing: FillFixed, Height: 205}),
			},
		})
	}
	w.refreshLayout = true
	w.FrameFn()

	overflow, ok := w.ScrollOverflowY("scroller")
	if !ok || overflow <= 0 || overflow > 30 {
		t.Fatalf("overflow = %v, %v, want a small positive overflow", overflow, ok)
	}
	w.ScrollVerticalTo("scroller", -overflow)
	w.refreshLayout = true
	w.FrameFn()

	sc, ok := w.layout.FindByID("scroller")
	if !ok {
		t.Fatal("scroller not found")
	}
	bar := findScrollbar(t, sc, scrollbarVertical)
	thumb := bar.Children[thumbIndex].Shape
	thumbEnd := thumb.Y + thumb.Height
	trackEnd := bar.Shape.Y + bar.Shape.Height
	if math.Abs(float64(thumbEnd-trackEnd)) > 0.01 {
		t.Fatalf("thumb end = %v, track end = %v; thumb must reach the end",
			thumbEnd, trackEnd)
	}
}

// With a MinThumbSize clamp the thumb travels less than its natural
// size would, so one pixel of drag must move the content by the
// content travel over the thumb travel, not by total/view.
func TestOffsetMouseChangeYMinThumb(t *testing.T) {
	layout, w := makeScrollLayout("s", 50, 500, 50, 100000)
	sy := w.scrollY()
	got := offsetMouseChangeY(sy, layout, 1, "s", 20)
	// Track 500, natural thumb 2.5 clamped to 20: travel 480.
	want := -(float32(100000) - 500) / 480
	if math.Abs(float64(got-want)) > 0.01 {
		t.Fatalf("1px drag offset = %v, want %v", got, want)
	}
}

func TestOffsetMouseChangeXMinThumb(t *testing.T) {
	layout, w := makeScrollLayout("s", 500, 50, 100000, 50)
	layout.Shape.Axis = axisLeftToRight
	got := offsetMouseChangeX(w.scrollX(), layout, 1, "s", 20)
	want := -(float32(100000) - 500) / 480
	if math.Abs(float64(got-want)) > 0.01 {
		t.Fatalf("1px drag offset = %v, want %v", got, want)
	}
}

// Every setter that moves a found scrollable fires its OnScroll. The
// percent setters and scrollToView moved it silently.
func TestScrollSettersFireOnScroll(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*Window)
	}{
		{"ScrollVerticalToPct", func(w *Window) { w.ScrollVerticalToPct("s", 0.5) }},
		{"ScrollHorizontalToPct", func(w *Window) { w.ScrollHorizontalToPct("s", 0.5) }},
		{"scrollToView", func(w *Window) { w.scrollToView("target") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layout, w := makeScrollLayout("s", 100, 100, 400, 400)
			layout.Children[0].Shape.ID = "target"
			layout.Children[0].Shape.Y = 150
			layout.Children[0].Parent = layout
			fired := 0
			layout.Shape.events = &eventHandlers{
				onScroll: func(EventCtx) { fired++ },
			}
			tc.call(w)
			if fired != 1 {
				t.Fatalf("OnScroll fired %d times, want 1", fired)
			}
		})
	}
}

// A reveal request eases to the top even when nothing was inserted
// above the anchor, as the ScrollAnchorReveal doc promises.
func TestScrollAnchorRevealWhenAnchorDidNotMove(t *testing.T) {
	posts := []anchorPost{{"A", 50}, {"B", 50}, {"C", 50}}
	w := anchorWindow(-40, posts)

	w.ScrollAnchorReveal(anchorScrollID, "B")
	anchorLayoutPass(w, posts)

	if w.scrollSmooth == nil {
		t.Fatal("no ease was armed")
	}
	e := w.scrollSmooth.findEntry(anchorScrollID, scrollAxisY)
	if e == nil || !e.active || e.target != 0 {
		t.Fatalf("want an active ease to 0, got %+v", e)
	}
}

// The read-only getters must not create the scroll maps as a side
// effect, as ScrollHorizontalOffset already does not.
func TestScrollGettersDoNotAllocateMaps(t *testing.T) {
	_, w := makeScrollLayout("s", 100, 100, 400, 400)
	w.ScrollVerticalOffset("s")
	w.ScrollVerticalPct("s")
	w.ScrollHorizontalPct("s")
	if w.scrollYRead() != nil || w.scrollXRead() != nil {
		t.Fatal("a getter created a scroll map")
	}
}

// The twins report a near miss the same way.
func TestScrollTwinsReportLookupMiss(t *testing.T) {
	for _, tc := range []struct {
		api  string
		call func(*Window)
	}{
		{"ScrollVerticalPct", func(w *Window) { w.ScrollVerticalPct("list") }},
		{"ScrollHorizontalTo", func(w *Window) { w.ScrollHorizontalTo("list", -1) }},
	} {
		t.Run(tc.api, func(t *testing.T) {
			buf := captureDebugMask(t, DebugUnknownLookup)
			resetLookupWarnings(t)
			w := scopedScrollTree()
			tc.call(w)
			if got := buf.String(); !strings.Contains(got, tc.api+`("list") found nothing`) {
				t.Fatalf("want a near-miss finding, got %q", got)
			}
		})
	}
}
