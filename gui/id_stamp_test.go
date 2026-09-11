package gui

import (
	"strings"
	"testing"
)

// An injected overlay — dialog, toast, inspector — is generated at
// arrange time, long after the main tree pushed and popped its scopes.
// It is its own scope root, and stays one only because generation
// clears the scope whenever it starts from the top. Without that, every
// overlay would inherit whatever prefix the last panel left behind.
func TestInjectedOverlayResolvesFromEmptyScope(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.Dialog(DialogCfg{DialogType: DialogMessage, Title: "hi", Body: "there"})
	root := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			ID:     "panel",
			Content: []View{
				Column(ContainerCfg{ID: "body"}),
			},
		})
	})

	dlg := findShapeByLeaf(root, reservedDialogID)
	if dlg == nil {
		t.Fatal("dialog is not in the frame")
	}
	if got := dlg.idKey(); got != reservedDialogID {
		t.Fatalf("dialog resolved to %q, want the bare %q — the overlay "+
			"inherited a scope from the main tree", got, reservedDialogID)
	}
}

// A float is written inside the tree and stamped where it was written,
// so it keeps the scope of the panel that holds it even though
// extraction later lifts it into a layer of its own. Two panels may
// each hold a popup with the same leaf and get distinct identities.
func TestFloatKeepsTheScopeItWasWrittenIn(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	root := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID: "panel",
					Content: []View{
						Column(ContainerCfg{
							ID:     "pop",
							Float:  true,
							Width:  20,
							Height: 20,
						}),
					},
				}),
			},
		})
	})

	pop := findShapeByLeaf(root, "pop")
	if pop == nil {
		t.Fatal("the float is not in the frame")
	}
	if got := pop.idKey(); got != "panel:pop" {
		t.Fatalf("float resolved to %q, want %q — extraction moved it out "+
			"of the panel it was written in", got, "panel:pop")
	}
}

// A leaf that already spells a path is the whole identity and is never
// joined again, at any depth. That is what lets a composite compose its
// children's IDs itself — gui/datagrid does, and so do the RTF popups,
// whose IDs are framework constants an event handler names directly.
func TestAbsoluteLeafIsNotJoinedUnderAScope(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	root := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID: "panel",
					Content: []View{
						Column(ContainerCfg{
							ID: ScopeID("grid", "header"),
							Content: []View{
								Column(ContainerCfg{ID: rtfLinkMenuFocusID}),
							},
						}),
					},
				}),
			},
		})
	})

	for _, leaf := range []string{
		ScopeID("grid", "header"), rtfLinkMenuFocusID,
	} {
		s := findShapeByLeaf(root, leaf)
		if s == nil {
			t.Fatalf("no shape written as %q is in the frame", leaf)
		}
		if got := s.idKey(); got != leaf {
			t.Errorf("absolute leaf %q resolved to %q, want it unchanged",
				leaf, got)
		}
	}
}

// splicedShapeView appends a hand-built Layout to its children instead
// of generating it, which is the one way a shape can reach the tree
// without an identity now that generation owns the stamp.
type splicedShapeView struct{ id string }

func (v splicedShapeView) GenerateLayout(w *Window) Layout {
	layout := Layout{Shape: w.allocShape(Shape{
		shapeType: shapeRectangle,
		Sizing:    FitFit,
		Opacity:   1,
	})}
	layout.Children = append(layout.Children, Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: v.id, Opacity: 1},
	})
	return layout
}

// splicedParentView appends a hand-built ID-bearing parent holding its
// own hand-built child, so neither carries a stamp. The child's finding
// must name the scope through the unstamped parent, which exercises the
// join fallback in resolveFocusOwnersWalk.
type splicedParentView struct{ parent, child string }

func (v splicedParentView) GenerateLayout(w *Window) Layout {
	layout := Layout{Shape: w.allocShape(Shape{
		shapeType: shapeRectangle,
		Sizing:    FitFit,
		Opacity:   1,
	})}
	layout.Children = append(layout.Children, Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: v.parent, Opacity: 1},
		Children: []Layout{
			{Shape: &Shape{
				shapeType: shapeRectangle,
				ID:        v.child,
				Opacity:   1,
			}},
		},
	})
	return layout
}

// A hand-built parent with an ID is itself drifted, but its children
// still arrange under it. Their findings must name the full scope
// through that parent rather than the grandparent scope.
func TestStampDriftNamesScopeThroughUnstampedParent(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID: "panel",
					Content: []View{
						splicedParentView{parent: "mid", child: "leaf"},
					},
				}),
			},
		})
	})

	found := w.TestFindings(DebugStampDrift)
	var hit string
	for _, f := range found {
		if strings.Contains(f, `"leaf"`) {
			hit = f
			break
		}
	}
	if hit == "" {
		t.Fatalf("want a stamp finding naming the nested shape, got %q",
			found)
	}
	if !strings.Contains(hit, "panel:mid:leaf") {
		t.Errorf("finding does not name the scope through the unstamped "+
			"parent: %q", hit)
	}
}

// The stamp check is what keeps one implementation honest: a shape that
// never went through generation carries no identity, every store keys
// it on its bare leaf, and nothing else about the frame looks wrong.
func TestStampDriftReportsAnUnstampedShape(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID:      "panel",
					Content: []View{splicedShapeView{id: "spliced"}},
				}),
			},
		})
	})

	found := w.TestFindings(DebugStampDrift)
	var hit string
	for _, f := range found {
		if strings.Contains(f, `"spliced"`) {
			hit = f
			break
		}
	}
	if hit == "" {
		t.Fatalf("want a stamp finding naming the spliced shape, got %q",
			found)
	}
	if !strings.Contains(hit, "panel:spliced") {
		t.Errorf("finding does not name the identity the frame arranged "+
			"it under: %q", hit)
	}
}

// The stamp move must not change what the detectors see on a layout
// that is correct as written, including the opt-in reusability
// advisory: every identity here sits under an ID-bearing ancestor.
func TestNestedLayoutHasNoIdentityFindings(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			ID:     "screen",
			Content: []View{
				Column(ContainerCfg{
					ID: "left",
					Content: []View{
						Button(ButtonCfg{ID: "save", Label: "Save"}),
						Input(InputCfg{ID: "name"}),
					},
				}),
				Column(ContainerCfg{
					ID: "right",
					Content: []View{
						Button(ButtonCfg{ID: "save", Label: "Save"}),
						Input(InputCfg{ID: "name"}),
					},
				}),
			},
		})
	})

	if found := w.TestFindings(DebugAll | DebugUnscopedIDs); len(found) != 0 {
		t.Fatalf("want no findings on a scoped layout, got %q", found)
	}
}

// staleStampView returns a shape that already carries an effID, which
// is what a *Shape reused across frames would look like. Generation
// leaves an existing stamp alone — that guard is what keeps the join to
// one lookup per widget — so the wrong identity survives into the frame
// and only the drift check can see it.
type staleStampView struct{ id, stale string }

func (v staleStampView) GenerateLayout(w *Window) Layout {
	s := w.allocShape(Shape{
		shapeType: shapeRectangle,
		Sizing:    FitFit,
		Opacity:   1,
		ID:        v.id,
	})
	s.effID = v.stale
	return Layout{Shape: s}
}

// The second half of the drift check: a stamp that exists but names a
// scope the frame did not arrange the shape under. The shape's state,
// focus and scroll slots are all keyed on the stale string, so nothing
// downstream finds them and nothing else about the frame looks wrong.
func TestStampDriftReportsAStaleStamp(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID: "panel",
					Content: []View{
						staleStampView{id: "widget", stale: "other:widget"},
					},
				}),
			},
		})
	})

	found := w.TestFindings(DebugStampDrift)
	var hit string
	for _, f := range found {
		if strings.Contains(f, `"widget"`) {
			hit = f
			break
		}
	}
	if hit == "" {
		t.Fatalf("want a stamp finding naming the stale shape, got %q",
			found)
	}
	if !strings.Contains(hit, "other:widget") ||
		!strings.Contains(hit, "panel:widget") {
		t.Errorf("finding must name both the stamp it carries and the "+
			"identity its position calls for: %q", hit)
	}
}

// The check is a category like any other: off, it costs nothing and
// reports nothing, so a window with a genuinely drifted shape stays
// silent until the gate asks for it.
func TestStampDriftIsSilentWhenTheCategoryIsOff(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				staleStampView{id: "widget", stale: "other:widget"},
			},
		})
	})

	if found := w.TestFindings(DebugAll &^ DebugStampDrift); len(found) != 0 {
		t.Fatalf("want nothing with the category masked off, got %q", found)
	}
}

// unstampedParentView splices an ID-bearing parent that never went
// through generation, holding a child stamped with the identity its
// position actually calls for. Only the parent is wrong.
type unstampedParentView struct{ parent, childEff string }

func (v unstampedParentView) GenerateLayout(w *Window) Layout {
	child := &Shape{
		shapeType: shapeRectangle, ID: "leaf", Opacity: 1,
	}
	child.effID = v.childEff
	// Spliced one level down: generateViewLayout stamps whatever a view
	// returns, so only a shape below the returned root can reach the
	// frame without an identity.
	return Layout{
		Shape: w.allocShape(Shape{
			shapeType: shapeRectangle,
			Sizing:    FitFit,
			Opacity:   1,
		}),
		Children: []Layout{{
			Shape: &Shape{
				shapeType: shapeRectangle, ID: v.parent, Opacity: 1,
			},
			Children: []Layout{{Shape: child}},
		}},
	}
}

// An unstamped shape has no identity to hand its children, so the walk
// recovers one by joining. Without that the children would be measured
// against an empty scope: a child stamped exactly right gets reported
// as drifted, and the real fault — the parent — is buried under it.
func TestUnstampedParentDoesNotMisplaceItsChildren(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID: "panel",
					Content: []View{unstampedParentView{
						parent:   "box",
						childEff: "panel:box:leaf",
					}},
				}),
			},
		})
	})

	found := w.TestFindings(DebugStampDrift)
	for _, f := range found {
		if strings.Contains(f, `"leaf"`) {
			t.Errorf("the correctly stamped child was reported: %q", f)
		}
	}
	var sawParent bool
	for _, f := range found {
		if strings.Contains(f, `"box"`) {
			sawParent = true
		}
	}
	if !sawParent {
		t.Fatalf("want the unstamped parent reported, got %q", found)
	}
}
