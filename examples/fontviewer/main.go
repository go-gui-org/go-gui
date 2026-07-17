// Font Viewer — browsable system-font catalog.
//
// Virtualized card grid with filter, sample text, size slider,
// and click-to-copy. Follows the get_started pattern: state is
// retrieved via gui.State[T](w) in every callback — no closure
// captures.
//
// Behavioral constraints (see docs/specs/font-viewer.md):
//   - Grid is virtualized (P0) via gui.ListVisibleRange.
//   - Cards are fixed-size → O(visible) per frame.
//   - Mouse-first, Latin-only preview.
package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

// --- State ---

type FontViewerState struct {
	Sample   string   // current sample text (init: random pangram)
	Filter   string   // case-insensitive family-name substring
	FontSize float32  // preview size in px (12–72; init: 28)
	Families []string // all discovered family names, sorted; may be nil
	Loaded   bool     // families have been enumerated (backend was ready)

	// ShapeAll drops virtualization and shapes every family in one
	// frame (the --shape-all stress mode). Off by default.
	ShapeAll bool

	CopiedFam   string  // family whose "Copied" badge is showing ("" = none)
	CopyOpacity float32 // 1→0, written by the fade tween
	HoveredFam  string  // family under the pointer ("" = none); cleared on eviction
}

// --- Constants ---

const (
	gridID       = "font-grid"
	cardMaxW     = 380
	gap          = 16
	sidePad      = 24
	scrollbarW   = 14
	nameRowH     = 28
	previewPad   = 24
	previewLines = 3
	lineFactor   = 1.4
	headerH      = 72
	toolbarH     = 104
	overscanRows = 4
	minFontSize  = float32(12)
	maxFontSize  = float32(72)
)

var pangrams = []string{
	"The quick brown fox jumps over the lazy dog",
	"Sphinx of black quartz, judge my vow",
	"Pack my box with five dozen liquor jugs",
	"How vexingly quick daft zebras jump",
	"Waltz, bad nymph, for quick jigs vex",
	"Jackdaws love my big sphinx of quartz",
}

// --- Helpers ---

// cardHeight is uniform per FontSize, driving both the card box and
// the virtualization rowH. Uses the engine's 1.4× line height so
// three preview lines fit without clipping.
func cardHeight(fontSize float32) float32 {
	return nameRowH + previewPad + previewLines*fontSize*lineFactor
}

func filterFontFamilies(all []string, filter string) []string {
	if filter == "" {
		return all
	}
	lf := strings.ToLower(filter)
	var out []string
	for _, f := range all {
		if strings.Contains(strings.ToLower(f), lf) {
			out = append(out, f)
		}
	}
	return out
}

// inWindow reports whether fam's row in matches lies within the
// emitted [firstRow, lastRow] window — used to clear HoveredFam when a
// card is evicted by virtualization (layoutMouseLeave never visits an
// off-window card, so its OnMouseLeave never fires).
func inWindow(matches []string, fam string, firstRow, lastRow, cols int) bool {
	for r := firstRow; r <= lastRow; r++ {
		start := r * cols
		end := min((r+1)*cols, len(matches))
		for i := start; i < end; i++ {
			if matches[i] == fam {
				return true
			}
		}
	}
	return false
}

// spacerV is a fixed-height gap that pads the virtualized grid above
// the first and below the last emitted row, keeping the scroll range
// equal to the full catalog.
func spacerV(h float32) gui.View {
	return gui.Column(gui.ContainerCfg{Sizing: gui.FillFixed, Height: h})
}

// --- Main ---

func main() {
	state := &FontViewerState{
		FontSize: 28,
		Sample:   pangrams[rand.IntN(len(pangrams))],
		ShapeAll: len(os.Args) > 1 && os.Args[1] == "--shape-all",
	}

	gui.SetTheme(gui.ThemeLight.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  state,
		Title:  "go-gui font viewer",
		Width:  900,
		Height: 700,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			w.SetFocus("sample-input")
		},
	})

	backend.Run(w)
}

// --- View tree ---

func mainView(w *gui.Window) gui.View {
	s := gui.State[FontViewerState](w)

	// Lazy one-time enumeration. ListSystemFonts reads a pre-built set
	// (cheap, no shaping); nil until the backend is ready → retry next
	// frame.
	if !s.Loaded {
		s.Families = gui.ListSystemFonts(w)
		s.Loaded = s.Families != nil
	}

	matches := filterFontFamilies(s.Families, s.Filter)

	// Zero all inherited chrome (default is PaddingMedium + SpacingMedium
	// + SizeBorderDef 1.5) so listH = winH - headerH - toolbarH is exact.
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    gui.NoPadding,
		Spacing:    gui.SomeF(0),
		SizeBorder: gui.Some(float32(0)),
		Content:    []gui.View{header(), toolbar(w, len(matches)), fontGrid(w, matches)},
	})
}

// --- Header ---

func header() gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFixed,
		Height:     headerH,
		Padding:    gui.Some(gui.Padding{Left: sidePad, Top: 14, Right: sidePad, Bottom: 0}),
		Spacing:    gui.SomeF(4),
		SizeBorder: gui.Some(float32(0)),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "go-gui font viewer", TextStyle: t.N1}),
			gui.Text(gui.TextCfg{Text: "Browse and preview installed system fonts", TextStyle: t.B2}),
		},
	})
}

// --- Toolbar ---

func toolbar(w *gui.Window, matchCount int) gui.View {
	s := gui.State[FontViewerState](w)
	t := gui.CurrentTheme()

	row1 := gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFixed,
		Height:     toolbarH / 2,
		Padding:    gui.Some(gui.Padding{Left: sidePad, Right: sidePad, Top: 8, Bottom: 4}),
		Spacing:    gui.SomeF(8),
		VAlign:     gui.VAlignMiddle,
		SizeBorder: gui.Some(float32(0)),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Sample Text", TextStyle: t.B2}),
			gui.Input(gui.InputCfg{
				ID:        "sample-input",
				Text:      s.Sample,
				TextStyle: t.B2,
				Sizing:    gui.FillFit,
				OnTextChanged: func(_ *gui.Layout, text string, w *gui.Window) {
					gui.State[FontViewerState](w).Sample = text
				},
			}),
			gui.Button(gui.ButtonCfg{
				Content: []gui.View{gui.Text(gui.TextCfg{
					Text:      gui.IconSync,
					TextStyle: gui.TextStyle{Family: gui.IconFontName, Size: t.Icon1.Size, Color: t.Icon1.Color},
				})},
				OnClick: shuffleSample,
			}),
		},
	})

	row2 := gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFixed,
		Height:     toolbarH / 2,
		Padding:    gui.Some(gui.Padding{Left: sidePad, Right: sidePad, Top: 4, Bottom: 8}),
		Spacing:    gui.SomeF(8),
		VAlign:     gui.VAlignMiddle,
		SizeBorder: gui.Some(float32(0)),
		Content:    toolbarRow2(s, t, matchCount),
	})

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFixed,
		Height:     toolbarH,
		Padding:    gui.NoPadding,
		Spacing:    gui.SomeF(0),
		SizeBorder: gui.Some(float32(0)),
		Content:    []gui.View{row1, row2},
	})
}

// toolbarRow2 builds the filter / size / count controls.
func toolbarRow2(s *FontViewerState, t gui.Theme, matchCount int) []gui.View {
	content := []gui.View{
		gui.Text(gui.TextCfg{Text: "Filter Fonts", TextStyle: t.B2}),
		gui.Input(gui.InputCfg{
			ID:        "filter-input",
			Text:      s.Filter,
			TextStyle: t.B2,
			Width:     200,
			Sizing:    gui.FixedFit,
			OnTextChanged: func(_ *gui.Layout, text string, w *gui.Window) {
				gui.State[FontViewerState](w).Filter = text
				w.ScrollVerticalTo(gridID, 0)
			},
		}),
	}
	if s.Filter != "" {
		content = append(content, gui.Button(gui.ButtonCfg{
			Content: []gui.View{gui.Text(gui.TextCfg{Text: "×", TextStyle: t.B2})},
			OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
				gui.State[FontViewerState](w).Filter = ""
				w.ScrollVerticalTo(gridID, 0)
				e.IsHandled = true
			},
		}))
	}

	return append(content,
		gui.Column(gui.ContainerCfg{Sizing: gui.FillFit}), // flexible gap
		gui.Text(gui.TextCfg{Text: "Size", TextStyle: t.B2}),
		gui.Slider(gui.SliderCfg{
			ID:     "size-slider",
			Value:  s.FontSize,
			Min:    minFontSize,
			Max:    maxFontSize,
			Step:   1,
			Width:  170,
			Sizing: gui.FixedFit,
			OnChange: func(v float32, e *gui.Event, w *gui.Window) {
				gui.State[FontViewerState](w).FontSize = v
				w.ScrollVerticalTo(gridID, 0) // rowH changed → reset offset
				e.IsHandled = true
			},
		}),
		gui.Text(gui.TextCfg{
			Text:      fmt.Sprintf("%d px", int(s.FontSize)),
			TextStyle: t.B2,
			MinWidth:  45,
		}),
		gui.Text(gui.TextCfg{
			Text:      fmt.Sprintf("%d / %d fonts", matchCount, len(s.Families)),
			TextStyle: t.B2,
			MinWidth:  120,
		}),
	)
}

// shuffleSample picks a new pangram distinct from the current one.
func shuffleSample(_ *gui.Layout, e *gui.Event, w *gui.Window) {
	s := gui.State[FontViewerState](w)
	for {
		p := pangrams[rand.IntN(len(pangrams))]
		if p != s.Sample || len(pangrams) == 1 {
			s.Sample = p
			break
		}
	}
	e.IsHandled = true
}

// --- FontGrid (virtualized) ---

func fontGrid(w *gui.Window, matches []string) gui.View {
	s := gui.State[FontViewerState](w)

	if len(matches) == 0 {
		return emptyState(len(s.Families) == 0)
	}

	winW, winH := w.WindowSize()
	cardH := cardHeight(s.FontSize)
	rowH := cardH + gap
	outerW := float32(winW)
	listH := max(rowH, float32(winH)-headerH-toolbarH) // clamp: never <= 0
	contentW := outerW - 2*sidePad - scrollbarW

	cols := max(1, int((contentW+gap)/(cardMaxW+gap)))
	cardW := min(cardMaxW, (contentW-float32(cols-1)*gap)/float32(cols))
	rows := (len(matches) + cols - 1) / cols

	// Which rows to emit. ShapeAll drops virtualization to stress
	// all-N shaping in one frame; the default path windows to [first,
	// last] read from the previous frame's scroll offset.
	first, last := 0, rows-1
	if !s.ShapeAll {
		scrollY, _ := w.ScrollY().Get(gridID)
		first, last = gui.ListVisibleRange(rows, rowH, listH, scrollY, overscanRows)

		// Clear a stale hover whose card was evicted by windowing.
		if s.HoveredFam != "" && !inWindow(matches, s.HoveredFam, first, last, cols) {
			s.HoveredFam = ""
		}
	}

	children := []gui.View{spacerV(float32(first) * rowH)}
	for r := first; r <= last; r++ {
		children = append(children, gridRow(w, matches, r, cols, cardW, cardH, rowH))
	}
	children = append(children, spacerV(float32(rows-1-last)*rowH))

	// FixedFixed with explicit Width AND Height — the same numbers feed
	// the cols/range math, so neither axis can disagree with arrange.
	return gui.Column(gui.ContainerCfg{
		ID: gridID, Scrollable: true,
		Sizing:     gui.FixedFixed,
		Width:      outerW,
		Height:     listH,
		Padding:    gui.Some(gui.Padding{Left: sidePad, Right: sidePad + scrollbarW}),
		Spacing:    gui.SomeF(0), // vertical gap lives in rowH, not spacing
		SizeBorder: gui.Some(float32(0)),
		Content:    children,
	})
}

// emptyState distinguishes an empty catalog (nil or no fonts) from a
// filter that excludes everything.
func emptyState(noFonts bool) gui.View {
	msg := "No fonts match the filter"
	if noFonts {
		msg = "No system fonts found"
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		Padding: gui.Some(gui.Padding{Top: 60}),
		Content: []gui.View{gui.Text(gui.TextCfg{Text: msg, TextStyle: gui.CurrentTheme().N3})},
	})
}

// gridRow emits one row of up to cols cards. FitFixed: width fits the
// cards (never zero — a zero-width row collapses the descendant clip
// chain), height is exactly rowH so the spacer math stays honest.
func gridRow(w *gui.Window, matches []string, rowIdx, cols int, cardW, cardH, rowH float32) gui.View {
	start := rowIdx * cols
	end := min(start+cols, len(matches))
	cards := make([]gui.View, 0, end-start)
	for i := start; i < end; i++ {
		cards = append(cards, fontCard(w, matches[i], cardW, cardH))
	}
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FitFixed,
		Height:     rowH,
		Spacing:    gui.SomeF(gap), // horizontal gutter between cards
		Padding:    gui.NoPadding,
		SizeBorder: gui.Some(float32(0)),
		Content:    cards,
	})
}

// --- FontCard ---

func fontCard(w *gui.Window, name string, cardW, cardH float32) gui.View {
	s := gui.State[FontViewerState](w)
	t := gui.CurrentTheme()

	bg := gui.Color{R: 248, G: 248, B: 248, A: 255}
	if s.HoveredFam == name {
		bg = gui.Color{R: 200, G: 220, B: 255, A: 255}
	}

	return gui.Column(gui.ContainerCfg{
		ID:      "card:" + name,
		Width:   cardW,
		Height:  cardH,
		Sizing:  gui.FixedFixed,
		Color:   bg,
		Radius:  gui.Some(float32(8)),
		Padding: gui.Some(gui.Padding{Left: previewPad, Right: previewPad, Top: 8, Bottom: 8}),
		Spacing: gui.SomeF(4),
		Content: []gui.View{
			cardNameRow(s, t, name),
			cardPreview(s.Sample, name, s.FontSize),
		},
		OnClick: copyFamily(name),
		OnHover: hoverFamily(name),
	})
}

// cardNameRow renders the family name plus a hover "Copy" / post-click
// "Copied" affordance. Clip truncates over-long names.
func cardNameRow(s *FontViewerState, t gui.Theme, name string) gui.View {
	content := []gui.View{gui.Text(gui.TextCfg{Text: name, TextStyle: t.B3})}
	switch {
	case s.CopiedFam == name:
		content = append(content, gui.Text(gui.TextCfg{
			Text:    "Copied",
			Opacity: gui.Some(s.CopyOpacity),
			TextStyle: gui.TextStyle{
				Family: t.N6.Family, Size: t.N6.Size,
				Color: gui.RGB(40, 140, 40),
			},
		}))
	case s.HoveredFam == name:
		content = append(content, gui.Text(gui.TextCfg{Text: "Copy", TextStyle: t.N6}))
	}
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFixed,
		Height:     nameRowH,
		Clip:       true,
		Spacing:    gui.SomeF(4),
		VAlign:     gui.VAlignMiddle,
		Padding:    gui.NoPadding,
		SizeBorder: gui.Some(float32(0)),
		Content:    content,
	})
}

// cardPreview fills the card space below the name and clips the wrapped
// sample to it. FillFill lets the engine size the box — no manual math.
func cardPreview(sample, name string, fontSize float32) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Clip:       true,
		Padding:    gui.NoPadding,
		SizeBorder: gui.Some(float32(0)),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: sample,
				Mode: gui.TextModeWrap,
				TextStyle: gui.TextStyle{
					Family: name,
					Size:   fontSize,
					Color:  gui.RGB(32, 32, 32),
				},
			}),
		},
	})
}

// copyFamily copies the family name and starts the "Copied" fade.
func copyFamily(name string) func(*gui.Layout, *gui.Event, *gui.Window) {
	return func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
		s := gui.State[FontViewerState](w)
		s.CopiedFam = name
		s.CopyOpacity = 1
		w.SetClipboard(name)
		w.AnimationAdd(&gui.TweenAnimation{
			AnimID:   "copied-fade",
			Duration: 1200 * time.Millisecond,
			Easing:   gui.EaseOutCubic,
			From:     1,
			To:       0,
			OnValue:  func(v float32, _ *gui.Window) { gui.State[FontViewerState](w).CopyOpacity = v },
			OnDone:   func(_ *gui.Window) { gui.State[FontViewerState](w).CopiedFam = "" },
		})
		e.IsHandled = true
	}
}

// hoverFamily tracks the hovered card for the "Copy" affordance and bg.
func hoverFamily(name string) func(*gui.Layout, *gui.Event, *gui.Window) {
	return func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
		s := gui.State[FontViewerState](w)
		switch e.Type {
		case gui.EventMouseEnter:
			s.HoveredFam = name
		case gui.EventMouseLeave:
			if s.HoveredFam == name {
				s.HoveredFam = ""
			}
		}
	}
}
