package gui

// markdown_anchor_interaction_test.go drives the in-document anchor
// (#slug) resolution added for issue #278 through the real pipeline:
// a markdown document with a link to one of its own headings, wrapped
// in a scroll container, with the stub text measurer the markdown
// selection suite uses. Covers the document-scoped lookup (the heading
// ID is ScopeID(mdID, "h", slug)) for flat and nested documents, the
// bare-ID fallback for non-heading targets, the rtfResolveAnchor
// helper in isolation, and the selection inertness of non-focusable
// documents.

import (
	"testing"
)

// mdAnchorSource is a document with a heading and a link to it. The
// heading must sit at a nonzero Y inside the scroll container: the
// click observably moves the scroll offset, because scrollToView pins
// the target to the container's top.
const mdAnchorSource = "# Heading\n\n[jump](#heading)"

// mdAnchorFindBlock walks the layout tree for an RTF block that
// belongs to markdown document mdID and whose flat text is text. The
// selection suite's block list (nsMdBlocks) only exists for focusable
// documents, so anchor tests resolve blocks from the tree directly.
func mdAnchorFindBlock(layout *Layout, mdID, text string) (*Layout, bool) {
	if layout.Shape != nil && layout.Shape.TC != nil &&
		layout.Shape.TC.markdownID == mdID &&
		layout.Shape.TC.rTFFlatText == text {
		return layout, true
	}
	for i := range layout.Children {
		if l, ok := mdAnchorFindBlock(&layout.Children[i], mdID, text); ok {
			return l, true
		}
	}
	return nil, false
}

// mdAnchorWindow renders the scroll container around an optional
// markdown document: the 100px lead rectangle keeps the document's
// heading away from the container top, the 900px tail keeps the
// container scrollable, and the optional top-level "heading" rectangle
// gives the bare-slug fallback a distinct shape to hit when it wins.
func mdAnchorWindow(
	t *testing.T, source string, nested, focusable, bareHeading bool,
) *Window {
	t.Helper()
	w := NewTestWindow(WindowCfg{Width: 800, Height: 800})
	w.textMeasurer = mdSelectTestMeasurer{}
	w.TestRender(func(win *Window) View {
		md := win.Markdown(MarkdownCfg{
			ID:        "md",
			Source:    source,
			Style:     DefaultMarkdownStyle(),
			Focusable: focusable,
		})
		if nested {
			md = Column(ContainerCfg{
				ID:      "panel",
				Sizing:  FillFit,
				Content: []View{md},
			})
		}
		view := Column(ContainerCfg{
			ID: "view", Scrollable: true, Sizing: FillFill,
			Content: []View{
				Rectangle(RectangleCfg{Sizing: FixedFill,
					Width: 50, Height: 100}),
				md,
				Rectangle(RectangleCfg{Sizing: FixedFill,
					Width: 50, Height: 900}),
			},
		})
		if bareHeading {
			return Column(ContainerCfg{
				Sizing: FillFill,
				Content: []View{
					Rectangle(RectangleCfg{ID: "heading",
						Sizing: FixedFill, Width: 50, Height: 50}),
					view,
				},
			})
		}
		return view
	})
	return w
}

// mdAnchorClick clicks the middle of the "jump" link run — the first
// run of its block, 4 chars at 10px each — and settles the frame.
func mdAnchorClick(t *testing.T, w *Window, docID string) {
	t.Helper()
	block, ok := mdAnchorFindBlock(&w.layout, docID, "jump")
	if !ok {
		t.Fatal("no link block in the window")
	}
	x, y := block.Shape.X+15, block.Shape.Y+10
	e := Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y}
	w.EventFn(&e)
	w.settle()
	if !e.IsHandled {
		t.Error("anchor link click was not consumed")
	}
	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: x, MouseY: y})
	w.settle()
}

// TestMarkdownAnchorLinkScrollsToView asserts a '#' link inside a
// markdown document scrolls the owning scroll container to the
// linked heading. The heading ID is scoped to the document's
// effective ID (md:h:slug, or panel:md:h:slug nested), so the click
// path must resolve the anchor against TC.markdownID rather than the
// bare slug. Runs flat and nested, focusable and not: identity
// stamping must not depend on selection being enabled.
func TestMarkdownAnchorLinkScrollsToView(t *testing.T) {
	for _, tc := range []struct {
		name      string
		nested    bool
		focusable bool
	}{
		{name: "flat"},
		{name: "nested", nested: true},
		{name: "flat_focusable", focusable: true},
		{name: "nested_focusable", nested: true, focusable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := mdAnchorWindow(t, mdAnchorSource,
				tc.nested, tc.focusable, false)
			docID := "view:md"
			if tc.nested {
				docID = "view:panel:md"
			}

			block, ok := mdAnchorFindBlock(&w.layout, docID, "jump")
			if !ok {
				t.Fatal("no link block in the window")
			}
			x, y := block.Shape.X+15, block.Shape.Y+10
			down := Event{Type: EventMouseDown, MouseButton: MouseLeft,
				MouseX: x, MouseY: y}
			w.EventFn(&down)
			w.settle()
			// Selection-enabled documents lock the mouse on click to
			// drive drag-select; plain documents must not.
			if locked := w.mouseIsLocked(); locked != tc.focusable {
				t.Errorf("mouse locked = %v, want %v",
					locked, tc.focusable)
			}
			w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft,
				MouseX: x, MouseY: y})
			w.settle()

			if !down.IsHandled {
				t.Error("anchor link click was not consumed")
			}
			_, sy, _ := w.TestScrollOffset("view")
			if sy == 0 {
				t.Error("anchor link did not scroll the container")
			}
			if got := w.IsFocus(docID); got != tc.focusable {
				t.Errorf("document focus = %v, want %v",
					got, tc.focusable)
			}
		})
	}
}

// TestMarkdownAnchorLinkBareIDFallback asserts a '#' link naming an
// arbitrary absolute ID that is not a heading of the document still
// scrolls: the scoped lookup misses, the bare slug ("view:bottom")
// hits. Non-focusable document, so the plain rtfOnClick path carries
// the resolution.
func TestMarkdownAnchorLinkBareIDFallback(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 800, Height: 800})
	w.textMeasurer = mdSelectTestMeasurer{}
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			ID: "view", Scrollable: true, Sizing: FillFill,
			Content: []View{
				win.Markdown(MarkdownCfg{
					ID:     "md",
					Source: "[jump](#view:bottom)",
					Style:  DefaultMarkdownStyle(),
				}),
				Rectangle(RectangleCfg{Sizing: FixedFill,
					Width: 50, Height: 900}),
				Rectangle(RectangleCfg{ID: "bottom",
					Sizing: FixedFill, Width: 50, Height: 100}),
			},
		})
	})

	mdAnchorClick(t, w, "view:md")

	_, sy, _ := w.TestScrollOffset("view")
	if sy == 0 {
		t.Error("bare anchor link did not scroll the container")
	}
}

// TestRtfResolveAnchor pins the resolution rule in isolation: for a
// markdown block (markdownID non-empty) the document-scoped heading
// spelling wins, the bare slug is the fallback, and a standalone RTF
// (markdownID empty) never tries the scoped lookup. The window also
// carries a top-level rectangle with the bare ID "heading" so a
// regression that skipped the scoped lookup would resolve to it and
// fail the first case.
func TestRtfResolveAnchor(t *testing.T) {
	w := mdAnchorWindow(t, mdAnchorSource, false, false, true)

	for _, tc := range []struct {
		mdID   string
		slug   string
		want   string
		wantOK bool
	}{
		// Scoped lookup wins, even with a bare "heading" shape around.
		{"view:md", "heading", "view:md:h:heading", true},
		// Scoped miss falls back to the bare slug: "view" is the scroll
		// container, not a heading of the document.
		{"view:md", "view", "view", true},
		// Both miss.
		{"view:md", "missing", "", false},
		// Standalone RTF: bare slug only.
		{"", "heading", "heading", true},
		{"", "missing", "", false},
	} {
		got, ok := rtfResolveAnchor(w, tc.mdID, tc.slug)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("rtfResolveAnchor(mdID=%q, slug=%q) = "+
				"(%q, %v), want (%q, %v)",
				tc.mdID, tc.slug, got, ok, tc.want, tc.wantOK)
		}
	}
}
