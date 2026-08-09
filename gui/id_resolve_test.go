package gui

import (
	"strconv"
	"strings"
	"testing"
)

// Per-scope uniqueness (docs/specs/widget-id-per-scope-uniqueness.md).
//
// The claim under test is composability: the same leaf ID may be used
// in two places as long as the places are told apart by explicit
// ancestor IDs. Everything else here guards the edges of that rule —
// what does *not* open a scope, what is left alone, and that the two
// resolution paths (the layout pass and the generation-time scope)
// agree on the string.

func TestResolveLeafRules(t *testing.T) {
	cases := []struct {
		name  string
		scope string
		leaf  string
		want  string
	}{
		{"joins under a scope", "settings", "name", "settings:name"},
		{"no scope leaves the leaf", "", "name", "name"},
		{"empty leaf has no identity", "settings", "", ""},
		{"absolute leaf skips the join", "settings", "a:b", "a:b"},
		{"nested scope is the full path",
			"app:settings", "name", "app:settings:name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveLeaf(c.scope, c.leaf); got != c.want {
				t.Fatalf("resolveLeaf(%q, %q) = %q, want %q",
					c.scope, c.leaf, got, c.want)
			}
		})
	}
}

// The headline case: one leaf, two panels, two identities.
func TestEffIDScopesLeafByAncestor(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	root := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID:      "settings",
					Content: []View{Input(InputCfg{ID: "name"})},
				}),
				Column(ContainerCfg{
					ID:      "profile",
					Content: []View{Input(InputCfg{ID: "name"})},
				}),
			},
		})
	})
	if root == nil {
		t.Fatal("TestRender returned nil")
	}

	for _, id := range []string{"settings:name", "profile:name"} {
		if _, ok := root.FindByID(id); !ok {
			t.Fatalf("FindByID(%q) = false, want the scoped input", id)
		}
	}
	// The bare leaf addresses neither: under an ID-bearing ancestor a
	// plain leaf always joins. There is no "stay global" mode.
	if _, ok := root.FindByID("name"); ok {
		t.Fatal("FindByID(\"name\") found a widget; the leaf is not an " +
			"identity once an ancestor carries an ID")
	}
	if got := w.TestDuplicateIDs(); len(got) != 0 {
		t.Fatalf("want no duplicate findings, got %q", got)
	}
}

// Focus is keyed on the effective ID end to end: the store holds it,
// tab traversal reports it, and the widget paints from it.
func TestFocusUsesEffectiveID(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID:      "settings",
					Content: []View{Input(InputCfg{ID: "name"})},
				}),
				Column(ContainerCfg{
					ID:      "profile",
					Content: []View{Input(InputCfg{ID: "name"})},
				}),
			},
		})
	})

	if err := w.testFocus("settings:name"); err != nil {
		t.Fatalf("TestFocus(settings:name) = %v, want nil", err)
	}
	if got := w.FocusID(); got != "settings:name" {
		t.Fatalf("FocusID() = %q, want %q", got, "settings:name")
	}
	next, err := w.testTab(tabForward)
	if err != nil {
		t.Fatalf("TestTab = %v, want nil", err)
	}
	if next != "profile:name" {
		t.Fatalf("tab moved to %q, want %q", next, "profile:name")
	}
}

// An ID-less container is not a scope. Two leaves under one add no
// prefix and still collide — scoping joins explicit IDs and nothing
// else, so a reorder or an added wrapper cannot silently re-identify a
// widget.
func TestIDLessAncestorAddsNoScope(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	root := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					Content: []View{Input(InputCfg{ID: "name"})},
				}),
			},
		})
	})
	if _, ok := root.FindByID("name"); !ok {
		t.Fatal("FindByID(\"name\") = false; an ID-less ancestor must " +
			"not add a prefix")
	}
}

// Input's inner text shape references the owner through focusOwner
// rather than repeating its ID. That reference has to resolve to the
// owner's *effective* ID, or the caret and selection would be read
// from a key nothing writes.
func TestFocusOwnerResolvesToOwnerEffID(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	root := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID:      "settings",
					Content: []View{Input(InputCfg{ID: "name"})},
				}),
			},
		})
	})

	txt, ok := root.findShape(func(l Layout) bool {
		return l.Shape != nil && l.Shape.focusOwner != ""
	})
	if !ok {
		t.Fatal("no shape with a focusOwner; the Input's text shape " +
			"should carry one")
	}
	if got := txt.focusKey(); got != "settings:name" {
		t.Fatalf("focusKey() = %q, want %q", got, "settings:name")
	}
}

// Two stateful widgets with one leaf, in two scopes, keep separate
// state. This is what the generation-time scope (w.EffID) buys: the
// combobox reads its open flag while building its own subtree, before
// the resolve pass has run.
func TestStatefulWidgetStateIsPerScope(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	view := func(_ *Window) View {
		panel := func(id string) View {
			return Column(ContainerCfg{
				ID: id,
				Content: []View{
					Combobox(ComboboxCfg{
						ID:      "cb",
						Options: []string{"a", "b"},
						OnSelect: func(string, EventCtx) {
						},
					}),
				},
			})
		}
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{panel("settings"), panel("profile")},
		})
	}
	w.TestRender(view)

	// Click the first one open, through the real dispatch path.
	if err := w.TestClick("settings:cb"); err != nil {
		t.Fatalf("TestClick(settings:cb) = %v, want nil", err)
	}
	// nil: re-run the installed generator. Passing the view again
	// reinstalls it, which resets the per-window state this asserts on.
	w.TestRender(nil)

	if !StateReadOr(w, nsCombobox, "settings:cb", false) {
		t.Fatal("settings:cb should be open")
	}
	if StateReadOr(w, nsCombobox, "profile:cb", false) {
		t.Fatal("profile:cb opened too; the two comboboxes share a " +
			"state slot")
	}
	if got := w.TestDuplicateIDs(); len(got) != 0 {
		t.Fatalf("want no duplicate findings, got %q", got)
	}
}

// A float is written inside the tree, so it keeps the scope it was
// written in. Identity is resolved before the floats are split into
// their own layers, which is what makes that possible.
func TestFloatKeepsAncestorScope(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	root := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID: "panel",
					Content: []View{
						Column(ContainerCfg{
							ID:     "popup",
							Float:  true,
							Width:  50,
							Height: 50,
							Sizing: FixedFixed,
						}),
					},
				}),
			},
		})
	})
	if _, ok := root.FindByID("panel:popup"); !ok {
		t.Fatal("FindByID(\"panel:popup\") = false; a float must keep " +
			"the scope it was written in")
	}
}

// The two paths must produce the same string for the same widget: the
// key a widget computes while generating (w.EffID) and the identity the
// pass stamps on its shape.
func TestGenerationScopeMatchesResolvePass(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	var generated string
	root := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID: "outer",
					Content: []View{
						Column(ContainerCfg{
							ID: "inner",
							Content: []View{
								viewFunc(func(w *Window) View {
									generated = w.EffID("leaf")
									return Column(ContainerCfg{ID: "leaf"})
								}),
							},
						}),
					},
				}),
			},
		})
	})

	want := "outer:inner:leaf"
	if generated != want {
		t.Fatalf("w.EffID(\"leaf\") = %q, want %q", generated, want)
	}
	ly, ok := root.FindByID(want)
	if !ok {
		t.Fatalf("FindByID(%q) = false", want)
	}
	if got := ly.Shape.ID; got != "leaf" {
		t.Fatalf("Shape.ID = %q, want the leaf %q — the pass stamps "+
			"effID and must not rewrite ID", got, "leaf")
	}
}

// The scope is restored on the way out of a subtree: a sibling that
// follows an ID-bearing container must not inherit that container's
// scope.
func TestGenerationScopeRestoredForSiblings(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	var sibling string
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID:      "first",
					Content: []View{Column(ContainerCfg{ID: "child"})},
				}),
				viewFunc(func(w *Window) View {
					sibling = w.EffID("after")
					return Column(ContainerCfg{ID: "after"})
				}),
			},
		})
	})
	if sibling != "after" {
		t.Fatalf("sibling scope leaked: w.EffID(\"after\") = %q, want %q",
			sibling, "after")
	}
}

// EventCtx.EffID is the seam for factories that build eagerly, so it
// has to answer for a leaf naming this shape, a leaf naming an
// ancestor, an absolute leaf, and a leaf naming nothing at all.
func TestEventCtxEffID(t *testing.T) {
	// panel(outer) -> input(name) -> inner(no ID). Handlers hang off the
	// inner shape, which is where Input's OnClick actually runs.
	inner := Layout{Shape: &Shape{}}
	input := Layout{Shape: &Shape{ID: "name"}, Children: []Layout{inner}}
	panel := Layout{Shape: &Shape{ID: "outer"}, Children: []Layout{input}}
	root := Layout{Shape: &Shape{}, Children: []Layout{panel}}
	layoutParents(&root, nil)
	w := &Window{}
	resolveShapeIDs(&root, w)

	innerLy := &root.Children[0].Children[0].Children[0]
	ctx := EventCtx{Layout: innerLy, Window: w}

	tests := []struct {
		name string
		leaf string
		want string
	}{
		{"names an ancestor", "name", "outer:name"},
		{"names a further ancestor", "outer", "outer"},
		{"absolute passes through", "a:b", "a:b"},
		{"empty stays empty", "", ""},
		{"names nothing joins the enclosing scope", "scroll",
			"outer:name:scroll"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ctx.EffID(tc.leaf); got != tc.want {
				t.Fatalf("ctx.EffID(%q) = %q, want %q", tc.leaf, got, tc.want)
			}
		})
	}

	// A context with no layout has nothing to resolve against and must
	// hand the leaf back rather than panic.
	if got := (EventCtx{}).EffID("name"); got != "name" {
		t.Fatalf("EffID on an empty ctx = %q, want %q", got, "name")
	}
}

// The memo must be invisible: same answer as the pure rule, every time,
// including after eviction has thrown entries away.
func TestJoinLeafMemoMatchesPureRule(t *testing.T) {
	w := &Window{}
	cases := [][2]string{
		{"settings", "name"},
		{"", "name"},
		{"settings", ""},
		{"settings", "a:b"},
		{"app:settings", "name"},
	}
	for _, c := range cases {
		want := resolveLeaf(c[0], c[1])
		// Twice: the second call is the memo hit.
		for range 2 {
			if got := w.joinLeaf(c[0], c[1]); got != want {
				t.Fatalf("joinLeaf(%q, %q) = %q, want %q",
					c[0], c[1], got, want)
			}
		}
	}

	// Overflow the cache, then re-ask for the first key: eviction costs a
	// recomputation, never a wrong answer.
	for i := range capIDJoin * 2 {
		w.joinLeaf("scope", "leaf"+strconv.Itoa(i))
	}
	if got := w.joinLeaf("settings", "name"); got != "settings:name" {
		t.Fatalf("after eviction joinLeaf = %q, want %q",
			got, "settings:name")
	}
}

// The join is one allocation, once per distinct identity. A frame that
// re-resolves the identities it already has must allocate nothing, or
// the per-widget-per-frame cost this memo exists to remove is back.
func TestJoinLeafCachedIsAllocationFree(t *testing.T) {
	w := &Window{}
	w.joinLeaf("settings", "name") // warm
	got := testing.AllocsPerRun(100, func() {
		sinkEffID = w.joinLeaf("settings", "name")
	})
	if got != 0 {
		t.Fatalf("cached joinLeaf allocated %v times per run, want 0", got)
	}
}

// sinkEffID keeps the compiler from optimising away the call under
// test.
var sinkEffID string

// focusOwner is resolved in place, so resolving twice — two frames over
// a retained tree, or an extra pass — must not compound the prefix.
func TestResolveShapeIDsIsIdempotent(t *testing.T) {
	txt := Layout{Shape: &Shape{focusOwner: "name"}}
	input := Layout{Shape: &Shape{ID: "name"}, Children: []Layout{txt}}
	panel := Layout{Shape: &Shape{ID: "settings"}, Children: []Layout{input}}
	root := Layout{Shape: &Shape{}, Children: []Layout{panel}}
	w := &Window{}

	resolveShapeIDs(&root, w)
	txtShape := root.Children[0].Children[0].Children[0].Shape
	first := txtShape.focusKey()
	if first != "settings:name" {
		t.Fatalf("focusKey() = %q, want %q", first, "settings:name")
	}

	resolveShapeIDs(&root, w)
	if got := txtShape.focusKey(); got != first {
		t.Fatalf("second resolve changed focusKey to %q, want %q",
			got, first)
	}
	if got := root.Children[0].Children[0].Shape.idKey(); got != "settings:name" {
		t.Fatalf("second resolve changed effID to %q, want %q",
			got, "settings:name")
	}
}

// A view function that escapes generation without restoring the scope
// (a panic recovered upstream) must not prefix every later frame. The
// resolve pass clears it at the start of each arrange.
func TestResolveShapeIDsClearsGenerationScope(t *testing.T) {
	w := &Window{}
	w.viewState.idScope = "stale"
	root := Layout{Shape: &Shape{ID: "root"}}
	resolveShapeIDs(&root, w)

	if w.viewState.idScope != "" {
		t.Fatalf("idScope = %q after resolve, want empty",
			w.viewState.idScope)
	}
	if got := w.EffID("name"); got != "name" {
		t.Fatalf("EffID after clear = %q, want %q", got, "name")
	}
}

// idKey falls back to the leaf on a tree that has never been arranged,
// so hand-built layouts in tests and any pre-pipeline read still work.
func TestIDKeyFallsBackBeforeResolve(t *testing.T) {
	s := &Shape{ID: "name"}
	if got := s.idKey(); got != "name" {
		t.Fatalf("idKey() = %q on an unresolved shape, want %q", got, "name")
	}
	if got := (&Shape{}).idKey(); got != "" {
		t.Fatalf("idKey() = %q on an ID-less shape, want empty", got)
	}
}

// Framework-internal identities that are written from *outside* layout
// generation — an event handler calling SetFocus, a state helper
// keyed by a constant — must be absolute.
//
// A plain leaf is resolved against wherever the framework happens to
// mount the widget, so the shape ends up under one key while the helper
// keeps writing another. Nothing errors: the inspector's wireframe
// tracked the pick while its tree never selected the row, and the
// dialog focused a widget that did not exist. Containing IDSep opts
// these out of joining, which is what makes the constant the identity.
func TestInternalIdentityConstantsAreAbsolute(t *testing.T) {
	constants := map[string]string{
		"inspectorTreeID":      inspectorTreeID,
		"inspectorScrollPanel": inspectorScrollPanel,
		"rtfLinkMenuFocusID":   rtfLinkMenuFocusID,
	}
	for name, id := range constants {
		if !strings.Contains(id, IDSep) {
			t.Errorf("%s = %q has no %q, so it joins with whatever "+
				"ID-bearing ancestor it is mounted under and stops "+
				"matching the helpers that write it", name, id, IDSep)
		}
	}

	// The dialog's focus ID is composed rather than declared, so assert
	// the composition, not the raw constant.
	cfg := DialogCfg{}
	applyDialogDefaults(&cfg)
	if !strings.Contains(cfg.FocusID, IDSep) {
		t.Errorf("dialog FocusID = %q has no %q", cfg.FocusID, IDSep)
	}
}
