package gui

import (
	"strings"
	"testing"
)

// makeHScrollLayout builds a left-to-right scrollable so the X axis
// carries the overflow. makeScrollLayout stacks top-to-bottom, which
// leaves contentWidth pinned to the widest child and says nothing
// about horizontal reach.
func makeHScrollLayout(idScroll string, width, height float32, contentW, contentH float32) (*Layout, *Window) {
	w := &Window{}
	pinScrollMultiplier(w, 1)
	child := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Width:     contentW,
			Height:    contentH,
		},
	}
	layout := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         idScroll,
			Width:      width,
			Height:     height,
			Axis:       axisLeftToRight,
		},
		Children: []Layout{child},
	}
	w.layout = Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{layout},
	}
	return &w.layout.Children[0], w
}

// TestScrollOverflowReportsHiddenExtent asserts the query answers with
// the hidden extent as a positive magnitude, and that the axis with no
// overflow stays at 0 rather than borrowing the other axis's figure.
func TestScrollOverflowReportsHiddenExtent(t *testing.T) {
	t.Parallel()
	t.Run("vertical", func(t *testing.T) {
		_, w := makeScrollLayout("v", 100, 100, 100, 300)
		got, ok := w.ScrollOverflowY("v")
		if got != 200 || !ok {
			t.Errorf("ScrollOverflowY = %v, %v, want 200, true", got, ok)
		}
		got, ok = w.ScrollOverflowX("v")
		if got != 0 || !ok {
			t.Errorf("ScrollOverflowX = %v, %v, want 0, true (fits)", got, ok)
		}
	})
	t.Run("horizontal", func(t *testing.T) {
		_, w := makeHScrollLayout("h", 100, 50, 400, 50)
		got, ok := w.ScrollOverflowX("h")
		if got != 300 || !ok {
			t.Errorf("ScrollOverflowX = %v, %v, want 300, true", got, ok)
		}
		got, ok = w.ScrollOverflowY("h")
		if got != 0 || !ok {
			t.Errorf("ScrollOverflowY = %v, %v, want 0, true (fits)", got, ok)
		}
	})
}

// TestScrollOverflowZeroWhenContentFits pins the distinction the issue
// asked for: content that fits reads 0, which ScrollVerticalPct cannot
// tell apart from being scrolled to the top.
func TestScrollOverflowZeroWhenContentFits(t *testing.T) {
	t.Parallel()
	_, w := makeScrollLayout("fits", 200, 200, 100, 100)
	// The pair the bool exists to separate: found, but nothing to
	// scroll. A miss reports the same 0 with ok false.
	got, ok := w.ScrollOverflowY("fits")
	if got != 0 || !ok {
		t.Errorf("ScrollOverflowY = %v, %v, want 0, true", got, ok)
	}
	got, ok = w.ScrollOverflowX("fits")
	if got != 0 || !ok {
		t.Errorf("ScrollOverflowX = %v, %v, want 0, true", got, ok)
	}
}

// TestScrollOverflowUnknownID covers the two cold paths: an id nothing
// stamped, and a window whose layout tree was never built.
func TestScrollOverflowUnknownID(t *testing.T) {
	t.Parallel()
	_, w := makeScrollLayout("known", 100, 100, 100, 300)
	got, ok := w.ScrollOverflowY("missing")
	if got != 0 || ok {
		t.Errorf("ScrollOverflowY(missing) = %v, %v, want 0, false", got, ok)
	}
	got, ok = w.ScrollOverflowX("missing")
	if got != 0 || ok {
		t.Errorf("ScrollOverflowX(missing) = %v, %v, want 0, false", got, ok)
	}

	// No frame arranged yet: the routine startup miss, which is why
	// this is a bool and not an error.
	empty := &Window{}
	got, ok = empty.ScrollOverflowY("any")
	if got != 0 || ok {
		t.Errorf("ScrollOverflowY on empty window = %v, %v, want 0, false", got, ok)
	}
	got, ok = empty.ScrollOverflowX("any")
	if got != 0 || ok {
		t.Errorf("ScrollOverflowX on empty window = %v, %v, want 0, false", got, ok)
	}
}

// TestScrollHorizontalOffsetMirrorsVertical asserts the newly exported
// horizontal getter reads back what the setter clamped, matching
// ScrollVerticalOffset's semantics on the other axis.
func TestScrollHorizontalOffsetMirrorsVertical(t *testing.T) {
	t.Parallel()
	_, w := makeHScrollLayout("hx", 100, 50, 400, 50)

	if got := w.ScrollHorizontalOffset("hx"); got != 0 {
		t.Errorf("initial ScrollHorizontalOffset = %v, want 0", got)
	}

	w.ScrollHorizontalTo("hx", -120)
	if got := w.ScrollHorizontalOffset("hx"); got != -120 {
		t.Errorf("ScrollHorizontalOffset = %v, want -120", got)
	}

	// Past the end: the setter clamps to the max offset, so the
	// getter must report the clamped value, not the request.
	w.ScrollHorizontalTo("hx", -9999)
	if got := w.ScrollHorizontalOffset("hx"); got != -300 {
		t.Errorf("clamped ScrollHorizontalOffset = %v, want -300", got)
	}

	if got := w.ScrollHorizontalOffset("missing"); got != 0 {
		t.Errorf("ScrollHorizontalOffset(missing) = %v, want 0", got)
	}
}

// A miss returns the same 0 that "content fits" returns, so a leaf
// spelled without its scope would otherwise read as a confident wrong
// answer. Both axes report the identity the frame stamped.
func TestScrollOverflowUnscopedLeafReports(t *testing.T) {
	for _, tc := range []struct {
		api  string
		call func(*Window, string) (float32, bool)
	}{
		{"ScrollOverflowY", (*Window).ScrollOverflowY},
		{"ScrollOverflowX", (*Window).ScrollOverflowX},
	} {
		t.Run(tc.api, func(t *testing.T) {
			buf := captureDebugMask(t, DebugUnknownLookup)
			resetLookupWarnings(t)
			w := scopedScrollTree()

			if got, ok := tc.call(w, "list"); got != 0 || ok {
				t.Fatalf("%s = %v, %v, want 0, false (bare leaf misses)",
					tc.api, got, ok)
			}

			got := buf.String()
			if !strings.Contains(got, tc.api+`("list") found nothing`) ||
				!strings.Contains(got, `"detail:list"`) {
				t.Fatalf("want a near-miss finding naming detail:list, got %q", got)
			}
		})
	}
}

// The effective ID resolves, so nothing is reported.
func TestScrollOverflowHitStaysSilent(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	w := scopedScrollTree()

	if got, ok := w.ScrollOverflowY("detail:list"); got != 200 || !ok {
		t.Fatalf("ScrollOverflowY = %v, %v, want 200, true", got, ok)
	}
	if got, ok := w.ScrollOverflowX("detail:list"); got != 0 || !ok {
		t.Fatalf("ScrollOverflowX = %v, %v, want 0, true (fits)", got, ok)
	}

	if got := buf.String(); got != "" {
		t.Fatalf("a hit must stay silent, got %q", got)
	}
}

// ScrollHorizontalPct joins the same gate as its vertical setter twin.
func TestScrollHorizontalPctUnscopedLeafReports(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	w := scopedScrollTree()

	w.ScrollHorizontalPct("list")

	got := buf.String()
	if !strings.Contains(got, `ScrollHorizontalPct("list") found nothing`) {
		t.Fatalf("want a near-miss finding for the bare leaf, got %q", got)
	}
}

// TestScrollPctBeforeFirstArrangeDoesNotPanic pins the guard the
// exported percentage getters and ScrollVerticalToPct need: the
// scroll-id walk reads Shape.Scrollable, so a window whose first frame
// has not been arranged has a nil root Shape and faults without it.
// Exporting ScrollHorizontalPct (#546) put that path in reach of an
// application asking about scroll state during startup.
func TestScrollPctBeforeFirstArrangeDoesNotPanic(t *testing.T) {
	t.Parallel()
	w := &Window{}

	if got := w.ScrollHorizontalPct("any"); got != 0 {
		t.Errorf("ScrollHorizontalPct = %v, want 0", got)
	}
	if got := w.ScrollVerticalPct("any"); got != 0 {
		t.Errorf("ScrollVerticalPct = %v, want 0", got)
	}
	// Setters take the same walk; a no-op is the whole assertion.
	w.ScrollVerticalToPct("any", 0.5)
	w.scrollHorizontalToPct("any", 0.5)
}
