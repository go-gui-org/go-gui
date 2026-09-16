package gui

import (
	"math"
	"strings"
	"testing"
)

// collectInvariants runs the shared walk and returns the findings as data,
// so a test asserts on what the checker said rather than on stderr.
func collectInvariants(root *Layout) []string {
	var found []string
	checkLayoutInvariants(root, func(subject, format string, args ...any) {
		found = append(found, subject)
	})
	return found
}

// A clean tree reports nothing. Guards against the checker being inert:
// every other test here would also pass if the walk never ran.
func TestLayoutInvariantsCleanTreeSilent(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			Sizing: FixedFixed, Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
				Width: 40, Height: 10}},
		},
	}
	w := &Window{scratch: newScratchPools(), windowWidth: 200, windowHeight: 200}
	layoutPipeline(root, w)
	if got := collectInvariants(root); len(got) != 0 {
		t.Errorf("clean tree reported %d findings: %v", len(got), got)
	}
}

// Invariant 1: a child outside a non-clipping parent's bounds is
// reported. Hand-positioned after the pipeline so the defect is the one
// under test rather than whatever the sizing pass would have produced.
func TestLayoutInvariantsChildEscapesParent(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			ID: "parent", Sizing: FixedFixed, Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, ID: "child",
				Sizing: FixedFixed, Width: 40, Height: 10}},
		},
	}
	child := root.Children[0].Shape
	child.X = 80 // spans 80..120 inside a parent ending at 100
	child.Width = 40
	found := collectInvariants(root)
	if len(found) == 0 {
		t.Fatal("escaping child reported nothing")
	}
	if found[0] != "child" {
		t.Errorf("finding subject: got %q, want %q", found[0], "child")
	}
}

// A clipping parent is exempt: being sized below its content is what the
// clip is for. Same tree as above, Clip set.
func TestLayoutInvariantsClipParentExempt(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			ID: "parent", Sizing: FixedFixed, Width: 100, Height: 20,
			Clip: true},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, ID: "child",
				Sizing: FixedFixed, Width: 40, Height: 10, X: 80}},
		},
	}
	if got := collectInvariants(root); len(got) != 0 {
		t.Errorf("clipping parent reported %v, want silence", got)
	}
}

// A Float is positioned against something other than this parent's
// content box, so it is exempt from containment.
func TestLayoutInvariantsFloatExempt(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			ID: "parent", Sizing: FixedFixed, Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, ID: "child",
				Sizing: FixedFixed, Width: 40, Height: 10, X: 80,
				Float: true}},
		},
	}
	if got := collectInvariants(root); len(got) != 0 {
		t.Errorf("float child reported %v, want silence", got)
	}
}

// Invariant 2: a minimum left above its maximum means the shape never
// went through clampSize, which every sizing site applies.
func TestLayoutInvariantsMinAboveMax(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "box",
			Sizing: FixedFixed, Width: 50, Height: 10,
			MinWidth: 80, MaxWidth: 40},
	}
	found := collectInvariants(root)
	if len(found) == 0 {
		t.Fatal("min above max reported nothing")
	}
}

// A zero or negative maximum means unset, so it cannot conflict with a
// minimum. Guards the clamp rule's own exemption.
func TestLayoutInvariantsUnsetMaxNoConflict(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "box",
			Sizing: FixedFixed, Width: 50, Height: 10,
			MinWidth: 80, MaxWidth: 0},
	}
	if got := collectInvariants(root); len(got) != 0 {
		t.Errorf("unset max reported %v, want silence", got)
	}
}

// Invariant 3: a non-finite size is reported. A NaN wins every later
// f32Max and poisons the scroll range, so it must not pass silently.
func TestLayoutInvariantsNonFiniteSize(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "box",
			Sizing: FixedFixed, Width: float32(math.NaN()), Height: 10},
	}
	found := collectInvariants(root)
	if len(found) == 0 {
		t.Fatal("NaN width reported nothing")
	}
}

// Invariant 3: a negative size renders as an inverted rect.
func TestLayoutInvariantsNegativeSize(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "box",
			Sizing: FixedFixed, Width: -5, Height: 10},
	}
	found := collectInvariants(root)
	if len(found) == 0 {
		t.Fatal("negative width reported nothing")
	}
}

// textOvershootTree is a text child taller than its parent, the shape an
// unmeasured label takes: fallbackLineHeight sizes it at style.Size*1.4
// while the parent fitted around it at fontHeight's style.Size*1.2.
func textOvershootTree() *Layout {
	return &Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "button",
			Sizing: FixedFixed, Width: 60, Height: 12, Y: 0},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeText, ID: "label",
				Sizing: FixedFixed, Width: 40, Height: 19.6, Y: 0}},
		},
	}
}

// With no TextMeasurer the glyph metrics are an estimate of an estimate,
// so containment has nothing to say about a text child. Pins the
// exemption: without it the four widget tests that caught this fail again.
func TestLayoutInvariantsTextExemptWithoutMeasurer(t *testing.T) {
	var found []string
	checkLayoutInvariantsOpts(textOvershootTree(),
		layoutInvariantOpts{skipTextContainment: true},
		func(subject, format string, args ...any) {
			found = append(found, subject)
		})
	if len(found) != 0 {
		t.Errorf("unmeasured text reported %v, want silence", found)
	}
}

// The exemption is conditional, not permanent: with real metrics a text
// child escaping its parent is still a defect.
func TestLayoutInvariantsTextCheckedWithMeasurer(t *testing.T) {
	var found []string
	checkLayoutInvariantsOpts(textOvershootTree(),
		layoutInvariantOpts{skipTextContainment: false},
		func(subject, format string, args ...any) {
			found = append(found, subject)
		})
	if len(found) == 0 {
		t.Fatal("measured text overshoot reported nothing")
	}
	if found[0] != "label" {
		t.Errorf("finding subject: got %q, want %q", found[0], "label")
	}
}

// The exemption covers containment only. A NaN or a minimum above a
// maximum is a defect whether or not the glyph metrics are real, so it
// must still report on a text shape under a nil measurer.
func TestLayoutInvariantsTextExemptionIsContainmentOnly(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeText, ID: "label",
			Sizing: FixedFixed, Width: 40, Height: 10,
			MinWidth: 80, MaxWidth: 40},
	}
	var found []string
	checkLayoutInvariantsOpts(root,
		layoutInvariantOpts{skipTextContainment: true},
		func(subject, format string, args ...any) {
			found = append(found, subject)
		})
	if len(found) == 0 {
		t.Error("min above max on a text shape reported nothing")
	}
}

// The category is opt-in, like DebugUnscopedIDs: a child escaping its
// parent is sometimes the design — a Slider's thumb is sized past its
// track and placed by AmendLayout — so it is asked for by name rather
// than reported to every app that turns on Debug. The check must still
// map to the category, or debugWarn would report it under another one.
func TestLayoutInvariantsCategoryOptIn(t *testing.T) {
	if DebugAll&DebugLayoutInvariants != 0 {
		t.Error("DebugLayoutInvariants must stay outside DebugAll")
	}
	if got := checkCategory(debugCheckLayoutInvariant); got != DebugLayoutInvariants {
		t.Errorf("checkCategory: got %v, want DebugLayoutInvariants", got)
	}
}

// The gate reads the process-wide mask, so a window only checks when the
// category is on. Asserted through the public switch rather than the
// atomic, so the test breaks if the wiring changes.
func TestLayoutInvariantsGateFollowsCategories(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: new(int)})
	captureDebugMask(t, DebugLayoutInvariants)
	if !w.debugLayoutInvariantsChecked() {
		t.Error("gate off while DebugLayoutInvariants is enabled")
	}
	DebugCategories(DebugDuplicates)
	if w.debugLayoutInvariantsChecked() {
		t.Error("gate on while only DebugDuplicates is enabled")
	}
}

// A violating frame reaches stderr through the real reporting path, with
// a message that names the rule rather than only the shape.
func TestLayoutInvariantsReportsThroughDebugWarn(t *testing.T) {
	out := captureDebugMask(t, DebugLayoutInvariants)
	w := NewTestWindow(WindowCfg{State: new(int)})
	w.debugCheckLayoutInvariants(&Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "box",
			Sizing: FixedFixed, Width: -5, Height: 10},
	})
	if !strings.Contains(out.String(), "layout invariant") {
		t.Errorf("stderr did not name the rule: %q", out.String())
	}
}

// Invariant 1 on the vertical axis: the y check is its own branch, so a
// horizontal-only test would pass with it deleted.
func TestLayoutInvariantsChildEscapesParentVertically(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "parent",
			Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, ID: "child",
				Y: 15, Width: 40, Height: 10}}, // spans 15..25, parent ends at 20
		},
	}
	found := collectInvariants(root)
	if len(found) != 1 || found[0] != "child" {
		t.Errorf("vertical escape: got %v, want [child]", found)
	}
}

// An overshoot inside f32Tolerance is float noise from the position pass,
// not a defect, and stays silent.
func TestLayoutInvariantsContainmentTolerance(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "parent",
			Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, ID: "child",
				X: 60, Width: 40 + f32Tolerance/2, Height: 10}},
		},
	}
	if got := collectInvariants(root); len(got) != 0 {
		t.Errorf("sub-tolerance overshoot reported %v, want silence", got)
	}
}

// Invariant 2 on the vertical axis, a separate branch from the width one.
func TestLayoutInvariantsMinHeightAboveMax(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "box",
			Width: 50, Height: 10, MinHeight: 80, MaxHeight: 40},
	}
	if got := collectInvariants(root); len(got) != 1 {
		t.Errorf("MinHeight above MaxHeight: got %v, want one finding", got)
	}
}

// A non-finite child is one defect and reports once, from the finite
// check, not a second time from containment.
func TestLayoutInvariantsNonFiniteChildReportsOnce(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "parent",
			Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, ID: "child",
				Width: float32(math.Inf(1)), Height: 10}},
		},
	}
	if got := collectInvariants(root); len(got) != 1 {
		t.Errorf("Inf child: got %v, want exactly one finding", got)
	}
}

// An ID-less shape is named by its type, so its findings do not collapse
// into one "" warn-once slot shared with every other anonymous shape.
func TestLayoutInvariantsAnonymousSubject(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: -1, Height: 10},
	}
	found := collectInvariants(root)
	if len(found) != 1 || !strings.HasPrefix(found[0], "shapeType ") {
		t.Errorf("anonymous subject: got %v, want one shapeType finding", found)
	}
}

// Invariant 4: Fill children that leave the parent's content box
// unfilled are reported (issue #638). Containment stays silent here —
// both children sit inside the parent — so without the Fill-sum check
// this non-convergence gap has no signal at all.
func TestLayoutInvariantsFillSumUnderFillReports(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			ID: "row", Sizing: FixedFixed, Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 20, Height: 10}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 20, Height: 10}},
		},
	}
	found := collectInvariants(root)
	if len(found) != 1 || found[0] != "row" {
		t.Errorf("under-filled Fill row: got %v, want [row]", found)
	}
}

// Invariant 4 on the vertical axis: the height branch is its own code,
// so a horizontal-only test would pass with it deleted.
func TestLayoutInvariantsFillSumUnderFillVertical(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisTopToBottom,
			ID: "col", Sizing: FixedFixed, Width: 20, Height: 100},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 10, Height: 20}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 10, Height: 20}},
		},
	}
	found := collectInvariants(root)
	if len(found) != 1 || found[0] != "col" {
		t.Errorf("under-filled Fill column: got %v, want [col]", found)
	}
}

// Invariant 4: an over-constrained row is reported. Both children sit
// at X 0 in this hand-built tree, so containment stays silent and the
// finding below is the Fill-sum check alone.
func TestLayoutInvariantsFillSumOverflowReports(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			ID: "row", Sizing: FixedFixed, Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 60, Height: 10, MinWidth: 60}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 60, Height: 10, MinWidth: 60}},
		},
	}
	found := collectInvariants(root)
	if len(found) != 1 || found[0] != "row" {
		t.Errorf("over-constrained Fill row: got %v, want [row]", found)
	}
}

// A Fill row that exactly fills its parent is the healthy case and
// stays quiet.
func TestLayoutInvariantsFillSumExactQuiet(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			ID: "row", Sizing: FixedFixed, Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 50, Height: 10}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 50, Height: 10}},
		},
	}
	if got := collectInvariants(root); len(got) != 0 {
		t.Errorf("exact Fill sum reported %v, want silence", got)
	}
}

// Without a Fill child the slack is alignment, not a defect: a Fixed
// row narrower than its parent stays quiet.
func TestLayoutInvariantsFillSumNoFillQuiet(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			ID: "row", Sizing: FixedFixed, Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
				Width: 20, Height: 10}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
				Width: 20, Height: 10}},
		},
	}
	if got := collectInvariants(root); len(got) != 0 {
		t.Errorf("gap without Fill reported %v, want silence", got)
	}
}

// Maximum caps are explicit: every Fill child at its maximum leaves a
// gap that is alignment slack, not undistributed space.
func TestLayoutInvariantsFillSumMaxCappedQuiet(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			ID: "row", Sizing: FixedFixed, Width: 500, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 100, Height: 10, MaxWidth: 100}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 100, Height: 10, MaxWidth: 100}},
		},
	}
	if got := collectInvariants(root); len(got) != 0 {
		t.Errorf("max-capped gap reported %v, want silence", got)
	}
}

// A clipping parent is allowed to be sized below its content and a
// viewport gap is fine, so both directions stay quiet under a clip.
func TestLayoutInvariantsFillSumClipExempt(t *testing.T) {
	under := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			ID: "row", Sizing: FixedFixed, Width: 100, Height: 20,
			Clip: true},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 20, Height: 10}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 20, Height: 10}},
		},
	}
	if got := collectInvariants(under); len(got) != 0 {
		t.Errorf("clipped under-fill reported %v, want silence", got)
	}
	over := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			ID: "row", Sizing: FixedFixed, Width: 100, Height: 20,
			Clip: true},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 60, Height: 10, MinWidth: 60}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 60, Height: 10, MinWidth: 60}},
		},
	}
	if got := collectInvariants(over); len(got) != 0 {
		t.Errorf("clipped overflow reported %v, want silence", got)
	}
}

// A Wrap row skips shrinking by design and breaks rows instead, so it
// is exempt from the sum rule.
func TestLayoutInvariantsFillSumWrapExempt(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			ID: "row", Sizing: FixedFixed, Width: 100, Height: 20,
			Wrap: true},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 60, Height: 10, MinWidth: 60}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill,
				Width: 60, Height: 10, MinWidth: 60}},
		},
	}
	if got := collectInvariants(root); len(got) != 0 {
		t.Errorf("wrap row reported %v, want silence", got)
	}
}

// The clean path allocates nothing: the check runs every frame while the
// category is on, and the subject string is built only on a violation.
func TestLayoutInvariantsCleanWalkAllocFree(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40, Height: 10}},
		},
	}
	emit := func(string, string, ...any) {}
	allocs := testing.AllocsPerRun(100, func() {
		checkLayoutInvariantsOpts(root, layoutInvariantOpts{}, emit)
	})
	if allocs != 0 {
		t.Errorf("clean walk allocated %v times per run, want 0", allocs)
	}
}
