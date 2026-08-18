//go:build !wasm

//
// Every test here drives RenderToImage, which builds a go-glyph text
// system; on wasm that construction needs a DOM (syscall/js), which the
// node-based CI harness does not provide. draw_test.go covers the pure
// rasterizer paths on every platform.

package soft

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newWin builds a headless window whose root is view.
func newWin(t *testing.T, w, h int, view func(*gui.Window) gui.View) *gui.Window {
	t.Helper()
	win := gui.NewWindow(gui.WindowCfg{
		Width:   w,
		Height:  h,
		BgColor: gui.RGB(0, 0, 0),
		OnInit:  func(win *gui.Window) { win.UpdateView(view) },
	})
	t.Cleanup(func() { Release(win) })
	return win
}

func TestRenderRectPixels(t *testing.T) {
	win := newWin(t, 100, 100, func(w *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing:     gui.FixedFixed,
			Width:      40,
			Height:     40,
			Color:      gui.RGB(255, 0, 0),
			SizeBorder: gui.NoBorder,
		})
	})
	img, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 100, 100) {
		t.Fatalf("bounds = %v, want 100x100", got)
	}
	// Inside the rect: pure red.
	if r, g, b, a := at(img, 20, 20); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("centre = %d,%d,%d,%d, want 255,0,0,255", r, g, b, a)
	}
	// Outside it: the window background.
	if r, g, b, a := at(img, 90, 90); r != 0 || g != 0 || b != 0 || a != 255 {
		t.Errorf("corner = %d,%d,%d,%d, want 0,0,0,255", r, g, b, a)
	}
}

func TestRenderScaleDoublesGeometry(t *testing.T) {
	view := func(w *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing:     gui.FixedFixed,
			Width:      40,
			Height:     40,
			Color:      gui.RGB(255, 0, 0),
			SizeBorder: gui.NoBorder,
		})
	}
	win := newWin(t, 100, 100, view)
	img, err := RenderToImage(win, 2)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 200, 200) {
		t.Fatalf("bounds = %v, want 200x200", got)
	}
	// The 40pt rect covers 80 device px, so (70,70) is inside at
	// scale 2 and outside at scale 1.
	if r, _, _, _ := at(img, 70, 70); r != 255 {
		t.Errorf("(70,70) red = %d, want 255 (rect should span 80px)", r)
	}
	if r, _, _, _ := at(img, 90, 90); r != 0 {
		t.Errorf("(90,90) red = %d, want 0 (past the rect)", r)
	}
}

func TestRenderText(t *testing.T) {
	win := newWin(t, 200, 60, func(w *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing:     gui.FillFill,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Label("HHHH", gui.TextStyle{
					Size:  32,
					Color: gui.RGB(255, 255, 255),
				}),
			},
		})
	})
	img, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	if win.TextMeasurer() == nil {
		t.Fatal("text measurer not installed")
	}
	if lit := countLitPixels(img); lit == 0 {
		t.Fatal("no glyph pixels rendered; the atlas warm pass failed")
	} else {
		t.Logf("lit pixels: %d", lit)
	}
}

// countLitPixels counts pixels with any channel above the antialiasing
// threshold; glyph edges are never exact white, so exact equality would be
// fragile.
func countLitPixels(img *image.RGBA) int {
	var lit int
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			if r, _, _, _ := at(img, x, y); r > 40 {
				lit++
			}
		}
	}
	return lit
}

func TestRenderToPNG(t *testing.T) {
	win := newWin(t, 60, 60, func(w *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing:     gui.FillFill,
			Color:      gui.RGB(0, 128, 255),
			SizeBorder: gui.NoBorder,
		})
	})
	path := filepath.Join(t.TempDir(), "out", "shot.png")
	if err := RenderToPNG(win, 1, path); err != nil {
		t.Fatalf("RenderToPNG: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatal("empty PNG")
	}
}

// redView is a window-filling red column, used by the lifecycle tests.
func redView(w *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Color:      gui.RGB(255, 0, 0),
		SizeBorder: gui.NoBorder,
	})
}

func TestRenderScaleChangeRebuilds(t *testing.T) {
	onInit := 0
	win := gui.NewWindow(gui.WindowCfg{
		Width:   100,
		Height:  100,
		BgColor: gui.RGB(0, 0, 0),
		OnInit: func(w *gui.Window) {
			onInit++
			w.UpdateView(redView)
		},
	})
	t.Cleanup(func() { Release(win) })

	img1, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("RenderToImage(scale 1): %v", err)
	}
	if got := img1.Bounds(); got != image.Rect(0, 0, 100, 100) {
		t.Fatalf("bounds = %v, want 100x100", got)
	}

	// A second render at a different scale frees the scale-1 text
	// system and builds a fresh one; the frame must still paint.
	img2, err := RenderToImage(win, 2)
	if err != nil {
		t.Fatalf("RenderToImage(scale 2): %v", err)
	}
	if got := img2.Bounds(); got != image.Rect(0, 0, 200, 200) {
		t.Fatalf("bounds = %v, want 200x200", got)
	}
	if r, _, _, _ := at(img2, 40, 40); r != 255 {
		t.Errorf("scaled rect red = %d, want 255", r)
	}
	if onInit != 1 {
		t.Fatalf("OnInit ran %d times, want 1", onInit)
	}
}

func TestReleaseThenRenderRebuilds(t *testing.T) {
	win := newWin(t, 60, 60, redView)
	if _, err := RenderToImage(win, 1); err != nil {
		t.Fatalf("first RenderToImage: %v", err)
	}
	Release(win)
	if win.TextMeasurer() != nil {
		t.Fatal("Release left the text measurer installed")
	}
	img, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("render after Release: %v", err)
	}
	if r, _, _, _ := at(img, 30, 30); r != 255 {
		t.Errorf("rect red = %d, want 255", r)
	}
}

func TestRenderToImageNilWindow(t *testing.T) {
	if _, err := RenderToImage(nil, 1); err == nil {
		t.Fatal("nil window: want error")
	}
}

func TestRenderToImageZeroSizeWindow(t *testing.T) {
	win := gui.NewWindow(gui.WindowCfg{})
	t.Cleanup(func() { Release(win) })
	if _, err := RenderToImage(win, 1); err == nil {
		t.Fatal("zero-size window: want error")
	}
}

func TestRenderTextFallbackStyle(t *testing.T) {
	win := newWin(t, 120, 60, func(w *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing:     gui.FillFill,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Label("HHHH", gui.TextStyle{
					Size:  32,
					Color: gui.RGB(255, 255, 255),
				}),
			},
		})
	})
	tm, err := prepare(win, 1)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	// Drive the command stream directly with a fallback-style
	// RenderText (no TextStylePtr) to cover drawText's else branch
	// and the textured-quad blend path.
	root := newBuffer(120, 60)
	r := &renderer{
		buf:       root,
		rootBuf:   root,
		scale:     1,
		textSys:   tm.textSys,
		glyphBack: tm.back,
	}
	cmd := gui.RenderCmd{Kind: gui.RenderText, X: 10, Y: 10,
		Text: "HHHH", FontSize: 32, Color: gui.RGB(255, 255, 255)}

	tm.back.buf = nil
	r.drawAll([]gui.RenderCmd{cmd})
	tm.textSys.Commit()
	tm.back.buf = r.buf
	r.buf.clear(gui.RGB(0, 0, 0))
	r.drawAll([]gui.RenderCmd{cmd})
	tm.textSys.Commit()

	if lit := countLitPixels(r.buf.img); lit == 0 {
		t.Fatal("no glyph pixels via fallback style")
	}
}

// --- Phase 2 kinds, end to end through a real window ---

func TestRenderTermGridPixels(t *testing.T) {
	cells := make([]gui.TermCell, 4)
	for i := range cells {
		cells[i] = gui.TermCell{
			Ch:    'X',
			FG:    gui.RGB(255, 255, 255),
			BG:    gui.RGB(0, 0, 255),
			Width: 1,
		}
	}
	cells[3].BG = gui.RGB(255, 0, 0)

	win := newWin(t, 100, 100, func(w *gui.Window) gui.View {
		return gui.TermGrid(gui.TermGridCfg{
			ID:    "term",
			Cells: cells,
			Cols:  2,
			Rows:  2,
			CellW: 20,
			CellH: 20,
		})
	})
	img, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}

	// Cell (0,0) carries the blue background, cell (1,1) the red one.
	if _, _, b, _ := at(img, 5, 5); b != 255 {
		t.Errorf("cell(0,0) blue = %d, want 255", b)
	}
	if r, _, b, _ := at(img, 35, 35); r != 255 || b != 0 {
		t.Errorf("cell(1,1) = r%d b%d, want the red background", r, b)
	}
}

// TestRenderTermGridSelectionCursorAndUnderline exercises the overlay
// branches the pixel test above leaves untouched: the selection tint,
// the block cursor, reverse video and the underline attribute. Every
// cell is blank so the assertions read the fills, never the glyph ink.
func TestRenderTermGridSelectionCursorAndUnderline(t *testing.T) {
	cells := make([]gui.TermCell, 4)
	for i := range cells {
		cells[i] = gui.TermCell{
			Ch:    ' ',
			FG:    gui.RGB(255, 255, 255),
			BG:    gui.RGB(0, 0, 255),
			Width: 1,
		}
	}
	// (0,1): reverse swaps fg/bg, so the background becomes the FG.
	cells[2].FG = gui.RGB(255, 0, 255)
	cells[2].BG = gui.RGB(255, 255, 255)
	cells[2].Attrs = gui.TermReverse
	// (1,1): underline draws a thin foreground line along the bottom.
	cells[3].Attrs = gui.TermUnderline

	win := newWin(t, 100, 100, func(w *gui.Window) gui.View {
		return gui.TermGrid(gui.TermGridCfg{
			ID:    "term",
			Cells: cells,
			Cols:  2,
			Rows:  2,
			CellW: 20,
			CellH: 20,
			Selection: gui.TermSelRange{
				Start: 0, End: 2, Color: gui.RGB(0, 255, 0),
			},
			Cursor: gui.TermCursor{
				Col: 0, Row: 0, Visible: true,
				Style: gui.TermCursorBlock, Color: gui.RGB(255, 0, 0),
			},
		})
	})
	img, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}

	// Selection tints the whole first row over its backgrounds.
	if _, g, _, _ := at(img, 25, 5); g != 255 {
		t.Errorf("selection cell(1,0) green = %d, want 255", g)
	}
	// Reverse cell: the background is the swapped-in foreground.
	if r, g, b, _ := at(img, 5, 25); r != 255 || g != 0 || b != 255 {
		t.Errorf("reverse cell background = r%d g%d b%d, want magenta",
			r, g, b)
	}
	// Block cursor paints the whole cell clear of any glyph.
	if r, _, _, _ := at(img, 2, 2); r != 255 {
		t.Errorf("cursor corner red = %d, want 255", r)
	}
	// Underline: a thin foreground line along the cell bottom.
	if r, _, _, _ := at(img, 35, 39); r < 200 {
		t.Errorf("underline red = %d, want near 255", r)
	}
}

// TestMultilineTextBoxHasNoTrailingLeading pins the fix for a ListBox
// row reading top-biased: glyph sizes a line box as the
// baseline-to-baseline advance (ascent + descent + leading, floored at
// 1.15em), while rendering puts the baseline at ascent below the top.
// The trailing leading was therefore reserved at the bottom of every
// multiline shape and nothing painted into it, so a one-line multiline
// shape came out taller than the same text in single-line mode.
//
// Measured through the real rasterizer: the coloured Fit container
// around each Text is the shape box, so the two must span the same
// rows.
func TestMultilineTextBoxHasNoTrailingLeading(t *testing.T) {
	bg := gui.RGB(255, 0, 0)
	build := func(txt gui.TextCfg) func(*gui.Window) gui.View {
		return func(w *gui.Window) gui.View {
			return gui.Column(gui.ContainerCfg{
				Color:      bg,
				Padding:    gui.PaddingNone,
				SizeBorder: gui.NoBorder,
				Sizing:     gui.FitFit,
				Content: []gui.View{
					gui.Text(txt),
				},
			})
		}
	}
	// bandHeight counts the rows the red container covers.
	bandHeight := func(txt gui.TextCfg) int {
		win := newWin(t, 200, 80, build(txt))
		img, err := RenderToImage(win, 1)
		if err != nil {
			t.Fatalf("RenderToImage: %v", err)
		}
		rows := 0
		for y := range 80 {
			for x := range 200 {
				r, g, b, _ := at(img, x, y)
				if r > 200 && g < 60 && b < 60 {
					rows++
					break
				}
			}
		}
		return rows
	}
	single := bandHeight(gui.TextCfg{Text: "list item"})
	multi := bandHeight(gui.TextCfg{
		Text: "list item", Mode: gui.TextModeMultiline,
	})
	if single == 0 {
		t.Fatal("single-line container did not render")
	}
	if multi != single {
		t.Errorf("multiline box = %d rows, single-line = %d; "+
			"the difference is trailing leading nothing paints into",
			multi, single)
	}
}

// TestListBoxVirtualizedFillsViewport pins the row-height estimate a
// virtualized ListBox virtualizes with. The visible range is
// listHeight/rowHeight (plus a two-row overscan), the spacers are
// index*rowHeight, and the height model ScrollToIndex reads is
// registered with the same number — so an estimate that over-counts
// builds too few rows to cover the viewport and leaves the bottom of
// a scrolled list blank. The list here is tall enough (800px) that the
// over-count outruns the two-row overscan and the defect shows; at
// 400px the overscan absorbs it and the test would pass either way.
//
// Measured through the real rasterizer, because the defect only
// exists once a text measurer is present: with no measurer the
// estimate and the arranged height agree by construction.
func TestListBoxVirtualizedFillsViewport(t *testing.T) {
	const (
		listH  = 800
		winH   = 840
		winW   = 220
		nItems = 400
	)
	items := make([]gui.ListBoxOption, 0, nItems)
	for i := range nItems {
		id := fmt.Sprintf("%03d", i)
		items = append(items, gui.NewListBoxOption(id, "item "+id, id))
	}
	win := newWin(t, winW, winH, func(w *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Padding:    gui.PaddingNone,
			SizeBorder: gui.NoBorder,
			Sizing:     gui.FillFill,
			Content: []gui.View{
				gui.ListBox(gui.ListBoxCfg{
					ID:         "lb",
					Scrollable: true,
					Height:     listH,
					Data:       items,
				}),
			},
		})
	})
	// Frame 1 builds the layout the scroll API needs to find.
	if _, err := RenderToImage(win, 1); err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	win.ScrollVerticalToPct("lb", 0.5)
	img, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	// hasInk reports whether any row in [y0,y1) carries text.
	hasInk := func(y0, y1 int) bool {
		for y := y0; y < y1; y++ {
			for x := range winW {
				r, g, b, _ := at(img, x, y)
				if r > 150 && g > 150 && b > 150 {
					return true
				}
			}
		}
		return false
	}
	// The last few pixels of the viewport, above the list bottom edge:
	// with the estimate right, a row covers them.
	if !hasInk(listH-14, listH) {
		t.Error("bottom of a mid-scrolled virtualized list is blank: " +
			"the row-height estimate over-counts, so too few rows " +
			"are built to cover the viewport")
	}
}
