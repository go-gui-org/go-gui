// This example demonstrates a browsable catalog of system fonts in a virtualized grid.
// Font Viewer — browsable system-font catalog.
//
// Virtualized card grid with filter, sample text, size slider,
// and click-to-copy. Follows the get_started pattern: state is
// retrieved via state(w) in every callback — no closure captures.
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

	"flag"
	"log"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// --- State ---

type FontViewerState struct {
	Sample   string   // current sample text (init: random pangram)
	Filter   string   // case-insensitive family-name substring
	FontSize float32  // preview size in px (12–72; init: 28)
	Families []string // all discovered family names, sorted; may be nil
	// LowerFamilies caches strings.ToLower per family, same order as
	// Families. Rebuilt whenever Families is filled, so the per-frame
	// filter reads the cache instead of lowering every name.
	LowerFamilies []string
	Loaded        bool // families have been enumerated (backend was ready)

	// ShapeAll drops virtualization and shapes every family in one
	// frame (the --shape-all stress mode). Off by default.
	ShapeAll bool

	CopiedFam   string  // family whose "Copied" badge is showing ("" = none)
	CopyOpacity float32 // 1→0, written by the fade tween
	HoveredFam  string  // family under the pointer ("" = none); cleared on eviction
}

// state is the typed state accessor for the font-viewer window.
// Every callback repeats gui.State[FontViewerState](...); one
// accessor keeps the type mention and the call site short.
func state(w *gui.Window) *FontViewerState {
	return gui.State[FontViewerState](w)
}

// --- Constants ---

// Widget IDs referenced from more than one place.
const (
	gridID        = "font-grid"
	sampleInputID = "sample-input"
	// toolbarScope owns the toolbar button IDs below.
	toolbarScope = "fontviewer-toolbar"
)

// Initial configuration, shared with tests.
const (
	initialWinW     = 900
	initialWinH     = 700
	initialFontSize = 28
	minFontSize     = float32(12)
	maxFontSize     = float32(72)
)

// Grid geometry (px).
const (
	cardMaxW     = 380 // card width cap; cards shrink when columns squeeze
	gap          = 16  // gutter between cards, both axes
	sidePad      = 24  // page-level left/right padding
	scrollbarW   = 14  // width reserved for the grid scrollbar
	nameRowH     = 28  // family-name row height inside a card
	previewPad   = 24  // card horizontal padding around the preview text
	previewLines = 3   // sample lines a card fits without clipping
	lineFactor   = 1.4 // engine line-height multiplier (see cardHeight)
	headerH      = 72  // fixed header band height
	toolbarH     = 104 // fixed toolbar band height (two rows)
	overscanRows = 4   // rows emitted beyond the viewport for smooth scroll
)

// Toolbar layout (px).
const (
	toolbarLabelW  = 90  // min width of the "Sample Text"/"Filter Fonts" labels
	filterInputW   = 200 // filter text input
	sliderW        = 170 // font-size slider
	sizeLabelW     = 45  // "NN px" readout
	countLabelW    = 120 // "N / M fonts" readout
	toolbarSpacing = 8   // gap between controls within a row
	toolbarEdgePad = 8   // vertical padding on the toolbar's outer edges
	toolbarSeamPad = 4   // vertical padding where the two rows meet
)

// Header / card / misc visuals.
const (
	headerTopPad     = 14 // header band top padding
	spacingTight     = 4  // small gap inside header and cards
	cardRadius       = 8  // card corner radius
	cardVPad         = 8  // card top/bottom padding
	emptyStateTopPad = 60 // offset of the empty-state message
	copyFadeDuration = 1200 * time.Millisecond
)

// Card colors are fixed (not theme-derived): the preview must stay
// near-black-on-light regardless of theme for font legibility.
var (
	colorCardBG      = gui.RGBA(248, 248, 248, 255)
	colorCardHover   = gui.RGBA(200, 220, 255, 255)
	colorPreviewText = gui.RGB(32, 32, 32)
	colorCopiedBadge = gui.RGB(40, 140, 40) // green "Copied" confirmation
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

// filterFontFamiliesCached is the per-frame filter: it reads the
// precomputed lowercase names instead of lowering every family every
// frame. Falls back to filterFontFamilies when the cache is stale
// (e.g. Families set without LowerFamilies, as in tests).
func filterFontFamiliesCached(all, lower []string, filter string) []string {
	if filter == "" {
		return all
	}
	if len(lower) != len(all) {
		return filterFontFamilies(all, filter)
	}
	lf := strings.ToLower(filter)
	var out []string
	for i, f := range all {
		if strings.Contains(lower[i], lf) {
			out = append(out, f)
		}
	}
	return out
}

// lowerFamilyNames precomputes one lowercase name per family, kept in
// the same order so filterFontFamiliesCached can index it directly.
func lowerFamilyNames(all []string) []string {
	lower := make([]string, len(all))
	for i, f := range all {
		lower[i] = strings.ToLower(f)
	}
	return lower
}

// cardID composes a card container ID from a family name. Names come
// from the OS and may contain IDSep, which would make the leaf
// absolute and drop the "card" scope — sanitize it away.
func cardID(name string) string {
	return gui.ScopeID("card", strings.ReplaceAll(name, gui.IDSep, "-"))
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
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	shapeAll := flag.Bool("shape-all", false, "shape every family in one frame (stress mode)")
	flag.Parse()

	state := &FontViewerState{
		FontSize: initialFontSize,
		Sample:   randomPangram(""),
		ShapeAll: *shapeAll,
	}

	gui.SetTheme(gui.ThemeLight)

	w := gui.NewWindow(gui.WindowCfg{
		State:  state,
		Title:  "go-gui font viewer",
		Width:  initialWinW,
		Height: initialWinH,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
			w.SetFocus(sampleInputID)
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

// --- View tree ---

func mainView(w *gui.Window) gui.View {
	s := state(w)

	// One-time enumeration runs as a command, never in the view phase.
	// ListSystemFonts reads a pre-built set (cheap, no shaping) that is
	// nil until the backend is ready, so a not-ready backend just
	// re-queues next frame via !Loaded.
	if !s.Loaded {
		w.QueueCommand(ensureFamilies)
	}

	matches := filterFontFamiliesCached(s.Families, s.LowerFamilies, s.Filter)

	// Zero all inherited chrome (default is PaddingMedium + SpacingMedium
	// + SizeBorderDef 1.5) so listH = winH - headerH - toolbarH is exact.
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    gui.NoPadding,
		Spacing:    gui.NoSpacing,
		SizeBorder: gui.NoBorder,
		Content:    []gui.View{header(), toolbar(w, len(matches)), fontGrid(w, matches)},
	})
}

// ensureFamilies fills Families (and the lowercase cache) once the
// backend is ready. Runs in the command phase, queued from mainView.
func ensureFamilies(w *gui.Window) {
	s := state(w)
	if s.Loaded {
		return
	}
	fams := gui.ListSystemFonts(w)
	if fams == nil {
		return // backend not ready; mainView re-queues next frame
	}
	s.Families = fams
	s.LowerFamilies = lowerFamilyNames(fams)
	s.Loaded = true
	w.InvalidateLayout()
}

// --- Header ---

func header() gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFixed,
		Height:     headerH,
		Padding:    gui.NewPadding(headerTopPad, sidePad, 0, sidePad),
		Spacing:    gui.SomeF(spacingTight),
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "go-gui font viewer", TextStyle: t.TextStyleDisplay.Regular()}),
			gui.Text(gui.TextCfg{Text: "Browse and preview installed system fonts", TextStyle: t.TextStyleTitleSmall}),
		},
	})
}

// --- Toolbar ---

// toolbarRow wraps one toolbar row in the shared shell: half the
// toolbar band tall, page side padding, middle-aligned children.
func toolbarRow(topPad, bottomPad float32, content []gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFixed,
		Height:     toolbarH / 2,
		Padding:    gui.NewPadding(topPad, sidePad, bottomPad, sidePad),
		Spacing:    gui.SomeF(toolbarSpacing),
		VAlign:     gui.VAlignMiddle,
		SizeBorder: gui.NoBorder,
		Content:    content,
	})
}

// flexGap is a width-flexible spacer used to spread toolbar controls.
func flexGap() gui.View {
	return gui.Column(gui.ContainerCfg{Sizing: gui.FillFit})
}

func toolbar(w *gui.Window, matchCount int) gui.View {
	s := state(w)
	t := gui.CurrentTheme()

	row1 := toolbarRow(toolbarEdgePad, toolbarSeamPad, []gui.View{
		gui.Text(gui.TextCfg{Text: "Sample Text", TextStyle: t.TextStyleTitleSmall, MinWidth: toolbarLabelW}),
		gui.Input(gui.InputCfg{
			ID:        sampleInputID,
			Text:      s.Sample,
			TextStyle: t.TextStyleTitleSmall,
			Sizing:    gui.FillFit,
			OnTextChanged: func(text string, ctx gui.EventCtx) {
				state(ctx.Window).Sample = text
			},
		}),
		gui.Button(gui.ButtonCfg{
			ID: gui.ScopeID(toolbarScope, "shuffle"),
			Content: []gui.View{gui.Text(gui.TextCfg{
				Text:      gui.IconSync,
				TextStyle: gui.TextStyle{Family: gui.IconFontName, Size: t.TextStyleIconMedium.Size, Color: t.TextStyleIconXLarge.Color},
			})},
			OnClick: shuffleSample,
		}),
	})

	row2 := toolbarRow(toolbarSeamPad, toolbarEdgePad, toolbarRow2(s, t, matchCount))

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFixed,
		Height:     toolbarH,
		Padding:    gui.NoPadding,
		Spacing:    gui.NoSpacing,
		SizeBorder: gui.NoBorder,
		Content:    []gui.View{row1, row2},
	})
}

// toolbarRow2 builds the filter / size / count controls.
func toolbarRow2(s *FontViewerState, t gui.Theme, matchCount int) []gui.View {
	content := []gui.View{
		gui.Text(gui.TextCfg{Text: "Filter Fonts", TextStyle: t.TextStyleTitleSmall, MinWidth: toolbarLabelW}),
		gui.Input(gui.InputCfg{
			ID:        "filter-input",
			Text:      s.Filter,
			TextStyle: t.TextStyleTitleSmall,
			Width:     filterInputW,
			Sizing:    gui.FixedFit,
			OnTextChanged: func(text string, ctx gui.EventCtx) {
				state(ctx.Window).Filter = text
				ctx.Window.ScrollVerticalTo(gridID, 0)
			},
		}),
	}
	if s.Filter != "" {
		content = append(content, gui.Button(gui.ButtonCfg{
			ID:      gui.ScopeID(toolbarScope, "clear-filter"),
			Content: []gui.View{gui.Text(gui.TextCfg{Text: "×", TextStyle: t.TextStyleTitleSmall})},
			OnClick: func(ctx gui.EventCtx) {
				state(ctx.Window).Filter = ""
				ctx.Window.ScrollVerticalTo(gridID, 0)
			},
		}))
	}

	return append(content,
		flexGap(),
		gui.Text(gui.TextCfg{Text: "Size", TextStyle: t.TextStyleTitleSmall}),
		gui.Slider(gui.SliderCfg{
			ID:     "size-slider",
			Value:  s.FontSize,
			Min:    minFontSize,
			Max:    maxFontSize,
			Step:   1,
			Width:  sliderW,
			Sizing: gui.FixedFit,
			OnChange: func(v float32, ctx gui.EventCtx) {
				state(ctx.Window).FontSize = v
				ctx.Window.ScrollVerticalTo(gridID, 0) // rowH changed → reset offset
				ctx.Consume()
			},
		}),
		gui.Text(gui.TextCfg{
			Text:      fmt.Sprintf("%d px", int(s.FontSize)),
			TextStyle: t.TextStyleTitleSmall,
			MinWidth:  sizeLabelW,
		}),
		flexGap(),
		gui.Text(gui.TextCfg{
			Text:      fmt.Sprintf("%d / %d fonts", matchCount, len(s.Families)),
			TextStyle: t.TextStyleTitleSmall,
			MinWidth:  countLabelW,
		}),
		flexGap(),
	)
}

// shuffleSample replaces the sample text with a fresh pangram. No
// ctx.Consume: no ancestor handles clicks, so there is nothing to stop.
func shuffleSample(ctx gui.EventCtx) {
	s := state(ctx.Window)
	s.Sample = randomPangram(s.Sample)
}

// randomPangram returns a random pangram other than exclude. On a
// collision it steps to the next entry instead of re-rolling — the
// slight bias is irrelevant here and it avoids an unbounded loop.
func randomPangram(exclude string) string {
	i := rand.IntN(len(pangrams))
	if pangrams[i] == exclude {
		i = (i + 1) % len(pangrams)
	}
	return pangrams[i]
}

// --- FontGrid (virtualized) ---

func fontGrid(w *gui.Window, matches []string) gui.View {
	s := state(w)

	if len(matches) == 0 {
		return emptyState(len(s.Families) == 0)
	}

	// Allowlisted viewport use: virtualization needs real numbers —
	// ListVisibleRange takes the viewport height and the column count
	// comes from the width, both before any arrange pass has run.
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

		// Clear a stale hover whose card was evicted by windowing. The
		// view phase must not write state, so queue the clear for the
		// command phase — the card is not emitted this frame anyway,
		// so one stale frame is invisible.
		if s.HoveredFam != "" && !inWindow(matches, s.HoveredFam, first, last, cols) {
			stale := s.HoveredFam
			w.QueueCommand(func(w *gui.Window) {
				gs := state(w)
				if gs.HoveredFam == stale {
					gs.HoveredFam = ""
				}
			})
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
		ID:         gridID,
		Scrollable: true,
		Focusable:  true,
		Sizing:     gui.FixedFixed,
		Width:      outerW,
		Height:     listH,
		Padding:    gui.NewPadding(0, sidePad+scrollbarW, 0, sidePad),
		Spacing:    gui.NoSpacing, // vertical gap lives in rowH, not spacing
		SizeBorder: gui.NoBorder,
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
		Padding: gui.NewPadding(emptyStateTopPad, 0, 0, 0),
		Content: []gui.View{gui.Text(gui.TextCfg{Text: msg, TextStyle: gui.CurrentTheme().TextStyleBody})},
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
		SizeBorder: gui.NoBorder,
		Content:    cards,
	})
}

// --- FontCard ---

func fontCard(w *gui.Window, name string, cardW, cardH float32) gui.View {
	s := state(w)
	t := gui.CurrentTheme()

	bg := colorCardBG
	if s.HoveredFam == name {
		bg = colorCardHover
	}

	return gui.Column(gui.ContainerCfg{
		ID:      cardID(name),
		Width:   cardW,
		Height:  cardH,
		Sizing:  gui.FixedFixed,
		Color:   bg,
		Radius:  gui.SomeF(cardRadius),
		Padding: gui.NewPadding(cardVPad, previewPad, cardVPad, previewPad),
		Spacing: gui.SomeF(spacingTight),
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
	content := []gui.View{gui.Text(gui.TextCfg{Text: name, TextStyle: t.TextStyleTitleSmall})}
	switch {
	case s.CopiedFam == name:
		content = append(content, gui.Text(gui.TextCfg{
			Text:    "Copied",
			Opacity: gui.Some(s.CopyOpacity),
			TextStyle: gui.TextStyle{
				Family: t.TextStyleCaptionSmall.Family, Size: t.TextStyleCaptionSmall.Size,
				Color: colorCopiedBadge,
			},
		}))
	case s.HoveredFam == name:
		content = append(content, gui.Text(gui.TextCfg{Text: "Copy", TextStyle: t.TextStyleCaptionSmall}))
	}
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFixed,
		Height:     nameRowH,
		Clip:       true,
		Spacing:    gui.SomeF(spacingTight),
		VAlign:     gui.VAlignMiddle,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
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
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: sample,
				Mode: gui.TextModeWrap,
				TextStyle: gui.TextStyle{
					Family: name,
					Size:   fontSize,
					Color:  colorPreviewText,
				},
			}),
		},
	})
}

// copyFamily copies the family name and starts the "Copied" fade.
func copyFamily(name string) func(gui.EventCtx) {
	return func(ctx gui.EventCtx) {
		s := state(ctx.Window)
		s.CopiedFam = name
		s.CopyOpacity = 1
		ctx.Window.SetClipboard(name)
		ctx.Window.AnimationAdd(&gui.TweenAnimation{
			// Per-card AnimID: one shared "copied-fade" would let a
			// second card's click replace the first card's running
			// fade mid-tween. CopiedFam/CopyOpacity stay singleton
			// — only one badge shows at a time by construction.
			AnimID:   gui.ScopeID(cardID(name), "copied-fade"),
			Duration: copyFadeDuration,
			Easing:   gui.EaseOutCubic,
			From:     1,
			To:       0,
			OnValue:  func(v float32, w *gui.Window) { state(w).CopyOpacity = v },
			OnDone:   func(w *gui.Window) { state(w).CopiedFam = "" },
		})
		// let event bubble up to column to change focus
	}
}

// hoverFamily tracks the hovered card for the "Copy" affordance and bg.
func hoverFamily(name string) func(gui.EventCtx) {
	return func(ctx gui.EventCtx) {
		s := state(ctx.Window)
		switch ctx.Event.Type {
		case gui.EventMouseEnter:
			s.HoveredFam = name
		case gui.EventMouseLeave:
			if s.HoveredFam == name {
				s.HoveredFam = ""
			}
		}
	}
}
