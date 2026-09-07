package gui

import (
	"strings"
	"testing"
)

// resetLookupWarnings clears the package-level warn-once memory so one
// test cannot silence the next. The gate's generation moves on every
// off -> on transition, which covers the usual case, but a test that
// reports twice under one mask needs the memory empty to start with.
func resetLookupWarnings(t *testing.T) {
	t.Helper()
	lookupWarnMu.Lock()
	lookupWarned = nil
	lookupWarnMu.Unlock()
}

// scopedLookupTree builds a generated tree holding one widget of the
// given leaf under an ID-bearing ancestor, so the frame stamps
// "<scope>:<leaf>" and the bare leaf addresses nothing.
func scopedLookupTree(t *testing.T, scope, leaf string) (*Window, *Layout) {
	t.Helper()
	w := newTestWindow()
	tree := generateViewLayout(Column(ContainerCfg{
		ID: scope,
		Content: []View{
			Column(ContainerCfg{ID: leaf}),
		},
	}), w)
	w.layout = tree
	layoutParents(&w.layout, nil)
	return w, &w.layout
}

// A leaf spelled without its scope reports the identity the frame did
// stamp, which is the whole point of the category.
func TestFindByIDUnscopedLeafReports(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	_, root := scopedLookupTree(t, "detail", "nav")

	if _, ok := root.FindByID("nav"); ok {
		t.Fatal("bare leaf must not resolve under a scope")
	}

	got := buf.String()
	if !strings.Contains(got, `FindByID("nav") found nothing`) ||
		!strings.Contains(got, `"detail:nav"`) {
		t.Fatalf("want a near-miss finding naming detail:nav, got %q", got)
	}
}

// The category is part of DebugAll, so Debug(true) catches this
// without the caller naming the bit.
func TestFindByIDUnscopedLeafReportsUnderDebugAll(t *testing.T) {
	buf := captureDebugMask(t, DebugAll)
	resetLookupWarnings(t)
	_, root := scopedLookupTree(t, "detail", "nav")

	root.FindByID("nav")

	if got := buf.String(); !strings.Contains(got, `"detail:nav"`) {
		t.Fatalf("DebugAll must cover DebugUnknownLookup, got %q", got)
	}
}

// A lookup for a name the frame stamped nowhere is a probe, not a
// misspelling, and stays silent.
func TestFindByIDUnknownNameStaysSilent(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	_, root := scopedLookupTree(t, "detail", "nav")

	root.FindByID("no-such-widget")

	if got := buf.String(); got != "" {
		t.Fatalf("a lookup with no near miss must stay silent, got %q", got)
	}
}

// The correctly spelled lookup finds its target and reports nothing.
func TestFindByIDHitStaysSilent(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	_, root := scopedLookupTree(t, "detail", "nav")

	if _, ok := root.FindByID("detail:nav"); !ok {
		t.Fatal("effective ID must resolve")
	}

	if got := buf.String(); got != "" {
		t.Fatalf("a hit must stay silent, got %q", got)
	}
}

// Every other category on, this one off: nothing is reported.
func TestFindByIDMissSilentWhenCategoryOff(t *testing.T) {
	buf := captureDebugMask(t, DebugAll&^DebugUnknownLookup)
	resetLookupWarnings(t)
	_, root := scopedLookupTree(t, "detail", "nav")

	root.FindByID("nav")

	if got := buf.String(); got != "" {
		t.Fatalf("category is off, want silence, got %q", got)
	}
}

// findByID is the same lookup without the report: the probe form
// rtfResolveAnchor and the scroll anchor use.
func TestFindByIDProbeFormStaysSilent(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	_, root := scopedLookupTree(t, "detail", "nav")

	if _, ok := root.findByID("nav"); ok {
		t.Fatal("bare leaf must not resolve under a scope")
	}

	if got := buf.String(); got != "" {
		t.Fatalf("the probe form must stay silent, got %q", got)
	}
}

// The same miss at the frame rate reports once.
func TestFindByIDMissWarnsOnce(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	_, root := scopedLookupTree(t, "detail", "nav")

	root.FindByID("nav")
	root.FindByID("nav")
	root.FindByID("nav")

	if n := strings.Count(buf.String(), "found nothing"); n != 1 {
		t.Fatalf("want one report for a repeated miss, got %d", n)
	}
}

// A lookup made against a subtree still names the identity the frame
// stamped: the walk climbs to the root before searching for near
// misses.
func TestFindByIDSubtreeMissClimbsToRoot(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	_, root := scopedLookupTree(t, "detail", "nav")
	sub := &root.Children[0]

	sub.FindByID("nav")

	if got := buf.String(); !strings.Contains(got, `"detail:nav"`) {
		t.Fatalf("want the stamped identity from the frame root, got %q", got)
	}
}

// Two scopes carrying the same leaf are both offered, sorted.
func TestFindByIDMissListsEveryCandidate(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	w := newTestWindow()
	w.layout = generateViewLayout(Column(ContainerCfg{
		Content: []View{
			Column(ContainerCfg{ID: "second", Content: []View{
				Column(ContainerCfg{ID: "nav"}),
			}}),
			Column(ContainerCfg{ID: "first", Content: []View{
				Column(ContainerCfg{ID: "nav"}),
			}}),
		},
	}), w)
	layoutParents(&w.layout, nil)

	w.layout.FindByID("nav")

	if got := buf.String(); !strings.Contains(got, `"first:nav", "second:nav"`) {
		t.Fatalf("want both candidates in sorted order, got %q", got)
	}
}

// scopedScrollTree builds a window whose only scrollable resolves to
// "detail:list", so the bare leaf addresses nothing.
func scopedScrollTree() *Window {
	w := &Window{}
	pinScrollMultiplier(w, 1)
	child := Layout{Shape: &Shape{
		shapeType: shapeRectangle,
		Width:     100,
		Height:    300,
	}}
	w.layout = Layout{
		Shape: &Shape{shapeType: shapeRectangle},
		Children: []Layout{{
			Shape: &Shape{
				shapeType:  shapeRectangle,
				Scrollable: true,
				ID:         "list",
				effID:      "detail:list",
				Width:      100,
				Height:     100,
				Axis:       axisTopToBottom,
			},
			Children: []Layout{child},
		}},
	}
	layoutParents(&w.layout, nil)
	return w
}

// ScrollVerticalTo keeps writing the offset — a set before the widget
// is built is legitimate — and reports the spelling the frame stamped.
func TestScrollVerticalToUnscopedLeafReports(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	w := scopedScrollTree()

	w.ScrollVerticalTo("list", -60)

	got := buf.String()
	if !strings.Contains(got, `ScrollVerticalTo("list") found nothing`) ||
		!strings.Contains(got, `"detail:list"`) {
		t.Fatalf("want a near-miss finding naming detail:list, got %q", got)
	}
	if v, _ := w.scrollY().Get("list"); v != -60 {
		t.Fatalf("offset must still be recorded, got %v", v)
	}
}

func TestScrollVerticalToPctUnscopedLeafReports(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	w := scopedScrollTree()

	w.ScrollVerticalToPct("list", 0.5)

	if got := buf.String(); !strings.Contains(got,
		`ScrollVerticalToPct("list") found nothing`) {
		t.Fatalf("want a near-miss finding, got %q", got)
	}
}

// A scroll offset set before the first frame names a widget that does
// not exist yet, which is legitimate and reports nothing.
func TestScrollVerticalToBeforeFirstFrameStaysSilent(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	w := &Window{}

	w.ScrollVerticalTo("list", -60)

	if got := buf.String(); got != "" {
		t.Fatalf("a pre-frame set must stay silent, got %q", got)
	}
}

// A miss that finds no near miss must not consume the warn-once slot:
// the widget may be built by a later frame, and that frame's miss is
// the one worth reporting.
func TestFindByIDMissReportsOnceTheWidgetExists(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	empty := &Layout{Shape: &Shape{}}

	empty.FindByID("nav")
	if got := buf.String(); got != "" {
		t.Fatalf("an empty frame has no near miss, got %q", got)
	}

	_, root := scopedLookupTree(t, "detail", "nav")
	root.FindByID("nav")

	if got := buf.String(); !strings.Contains(got, `"detail:nav"`) {
		t.Fatalf("want the finding once the frame stamps the leaf, got %q", got)
	}
}

// Turning the gate off and on again discards the warn-once memory, so
// a miss that survived a fix attempt is reported against the new run.
func TestFindByIDMissReportsAgainAfterGateRecycled(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	_, root := scopedLookupTree(t, "detail", "nav")

	root.FindByID("nav")
	// An off -> on transition moves the gate's generation, which is
	// what the package-level memory keys itself against.
	DebugCategories(0)
	DebugCategories(DebugUnknownLookup)
	root.FindByID("nav")

	if n := strings.Count(buf.String(), "found nothing"); n != 2 {
		t.Fatalf("want a report per generation, got %d", n)
	}
}

// A leaf stamped under many scopes names only the first few: the
// message has to stay readable.
func TestFindByIDMissCapsCandidateList(t *testing.T) {
	buf := captureDebugMask(t, DebugUnknownLookup)
	resetLookupWarnings(t)
	w := newTestWindow()
	scopes := []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7"}
	content := make([]View, 0, len(scopes))
	for _, scope := range scopes {
		content = append(content, Column(ContainerCfg{
			ID:      scope,
			Content: []View{Column(ContainerCfg{ID: "nav"})},
		}))
	}
	w.layout = generateViewLayout(Column(ContainerCfg{Content: content}), w)
	layoutParents(&w.layout, nil)

	w.layout.FindByID("nav")

	got := buf.String()
	if !strings.Contains(got, `"s5:nav" (and 2 more)`) {
		t.Fatalf("want the list capped with a remainder count, got %q", got)
	}
	if strings.Contains(got, `"s6:nav"`) {
		t.Fatalf("want candidates past the cap left out, got %q", got)
	}
}
