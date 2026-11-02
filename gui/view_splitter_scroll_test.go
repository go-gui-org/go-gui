package gui

import (
	"fmt"
	"testing"
)

// Issue #1214: a scrollable in a Splitter pane got a scroll range sized
// before the pane had its size. The main pipeline's scroll passes ran
// over the pane while it was still unmeasured, and the clamp, anchor and
// index-scroll results they stored outlived the pane's later layout.

// splitterScrollRows builds the issue's rows: a Fill-width Row holding
// a Fit Column of two texts. Plain Text rows hide the bug, because a
// Text has its height from generation while a Fit container only gets
// one from the measuring passes. extra rows are inserted above "r0".
func splitterScrollRows(extra int) []View {
	rows := make([]View, 0, 30+extra)
	for i := range 30 + extra {
		rows = append(rows, Row(ContainerCfg{
			ID:     ScopeIDN("r", "", i-extra),
			Sizing: FillFit,
			Content: []View{Column(ContainerCfg{Sizing: FitFit, Content: []View{
				Text(TextCfg{Text: "a"}), Text(TextCfg{Text: "b"}),
			}})},
		}))
	}
	return rows
}

func splitterScrollList(extra int) View {
	return Column(ContainerCfg{
		ID: "list", Sizing: FillFill,
		Scrollable: true, ScrollMode: ScrollVerticalOnly,
		Content: splitterScrollRows(extra),
	})
}

// splitterScrollHosts places a view where the bug lived — the second
// pane of a Splitter in either orientation, a Splitter nested in
// another Splitter's pane, a DockLayout panel — and, as the control,
// in a plain Row.
var splitterScrollHosts = []struct {
	name string
	host func(View) View
}{
	{"row", func(v View) View {
		return Row(ContainerCfg{Sizing: FillFill, Content: []View{
			Column(ContainerCfg{Width: 240, Sizing: FixedFill}), v,
		}})
	}},
	{"splitter", func(v View) View { return splitterScrollHost(SplitterHorizontal, v) }},
	{"splitter-vertical", func(v View) View { return splitterScrollHost(SplitterVertical, v) }},
	{"splitter-nested", func(v View) View {
		return Splitter(SplitterCfg{
			ID: "outer", Sizing: FillFill,
			First:  SplitterPaneCfg{Content: []View{Text(TextCfg{Text: "left"})}},
			Second: SplitterPaneCfg{Content: []View{splitterScrollHost(SplitterVertical, v)}},
		})
	}},
	{"dock", func(v View) View {
		return DockLayout(DockLayoutCfg{
			ID: "dock",
			Root: DockSplit("s1", DockSplitHorizontal, 0.3,
				DockPanelGroup("g1", []string{"p1"}, "p1"),
				DockPanelGroup("g2", []string{"p2"}, "p2")),
			Panels: []DockPanelDef{
				{ID: "p1", Label: "P1", Content: []View{Text(TextCfg{Text: "left"})}},
				{ID: "p2", Label: "P2", Content: []View{v}},
			},
		})
	}},
}

func splitterScrollHost(o splitterOrientation, v View) View {
	return Splitter(SplitterCfg{
		ID: "split", Orientation: o, Sizing: FillFill,
		First:  SplitterPaneCfg{Content: []View{Text(TextCfg{Text: "left"})}},
		Second: SplitterPaneCfg{Content: []View{v}},
	})
}

// splitterScrollWindow renders view in a fresh window and returns it
// with the list's effective ID.
func splitterScrollWindow(t *testing.T, view func(*Window) View) (*Window, string) {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 800, Height: 600})
	w.TestRender(view)
	ids := w.ResolveID("list")
	if len(ids) != 1 {
		t.Fatalf("ResolveID(list) = %v, want one ID", ids)
	}
	return w, ids[0]
}

// The issue's repro: wheeling down as far as it goes must reach the
// end of the scroll range and show the last row whole.
func TestSplitterPaneWheelReachesEnd(t *testing.T) {
	for _, h := range splitterScrollHosts {
		t.Run(h.name, func(t *testing.T) {
			w, id := splitterScrollWindow(t, func(*Window) View {
				return h.host(splitterScrollList(0))
			})
			if err := w.TestScroll(id, 0, -100000); err != nil {
				t.Fatalf("TestScroll: %v", err)
			}
			l := w.TestRender(nil)
			list, _ := l.FindByID(id)
			_, off, _ := w.TestScrollOffset(id)
			want := scrollMaxOffsetY(list)
			if want > -1000 {
				t.Fatalf("max offset %.1f: the list must overflow its viewport", want)
			}
			nearF(t, "offset", off, want, 0.5)
			last := list.Children[len(list.Children)-1].Shape
			if below := last.Y + last.Height - (list.Shape.Y + list.Shape.Height); below > 0 {
				t.Errorf("last row ends %.1f px below the viewport", below)
			}
		})
	}
}

// ScrollVerticalTo stores an in-range offset that the next frame must
// keep.
func TestSplitterPaneScrollVerticalToKeepsOffset(t *testing.T) {
	for _, h := range splitterScrollHosts {
		t.Run(h.name, func(t *testing.T) {
			w, id := splitterScrollWindow(t, func(*Window) View {
				return h.host(splitterScrollList(0))
			})
			w.ScrollVerticalTo(id, -2000)
			w.TestRender(nil)
			_, off, _ := w.TestScrollOffset(id)
			nearF(t, "offset", off, -2000, 0.01)
		})
	}
}

// ScrollAnchor keeps the anchor row where it was when rows are
// inserted above it: the offset moves by the inserted height.
func TestSplitterPaneScrollAnchorCorrects(t *testing.T) {
	for _, h := range splitterScrollHosts {
		t.Run(h.name, func(t *testing.T) {
			extra := 0
			w, id := splitterScrollWindow(t, func(*Window) View {
				return h.host(splitterScrollList(extra))
			})
			w.ScrollVerticalTo(id, -300)
			l := w.TestRender(nil)
			list, _ := l.FindByID(id)
			rowStep := list.Children[1].Shape.Y - list.Children[0].Shape.Y

			w.ScrollAnchor(id, w.ResolveID("r:5")[0])
			extra = 2
			w.TestRender(nil)
			_, off, _ := w.TestScrollOffset(id)
			nearF(t, "offset", off, -300-2*rowStep, 0.5)
		})
	}
}

// ScrollToEnd on a Fill-sized ListBox lands on its end. Its
// in-pipeline pass converges the request and must see the real
// viewport, not the unmeasured pane's zero height.
func TestSplitterPaneListBoxScrollToEnd(t *testing.T) {
	items := make([]string, 200)
	for i := range items {
		items[i] = fmt.Sprint("item ", i)
	}
	for _, h := range splitterScrollHosts {
		t.Run(h.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{Width: 800, Height: 600})
			w.TestRender(func(*Window) View {
				return h.host(ListBox(ListBoxCfg{ID: "lb", Items: items, Sizing: FillFill}))
			})
			w.TestRender(nil)
			id := w.ResolveID("lb")[0]
			w.ScrollToEnd(id)
			l := w.TestRender(nil)
			lb, _ := l.FindByID(id)
			_, off, _ := w.TestScrollOffset(id)
			nearF(t, "offset", off, scrollMaxOffsetY(lb), 0.5)
		})
	}
}
