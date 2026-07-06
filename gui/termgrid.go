package gui

import "math"

// TermGrid is a terminal character-grid primitive. It draws a
// fixed-pitch grid of cells straight from a caller-owned backing
// buffer in a single render command — no per-cell Layout node and no
// per-cell RenderText. Intended for terminal emulators (go-term), a
// go-edit terminal pane, and similar consumers that regenerate
// thousands of cells every frame.
//
// go-gui stays palette/escape-sequence agnostic: the caller resolves
// every cell's foreground and background to an RGBA Color before
// handing the buffer over. The grid is laid out like any other widget
// (Fixed default sizing of Cols*CellW x Rows*CellH); the backend draws
// glyphs pinned to exact cell positions so the grid never drifts.

// TermAttr is a per-cell attribute bitset. Combine with bitwise OR.
type TermAttr uint8

// TermAttr flags.
const (
	TermBold TermAttr = 1 << iota
	TermItalic
	TermUnderline
	TermReverse
)

// TermCell is a single grid cell. Colors are pre-resolved RGBA;
// go-gui does not consult any palette. Width distinguishes normal
// cells from wide (CJK/emoji) cells and their continuation slots.
type TermCell struct {
	Ch    rune  // cell rune; 0 or ' ' renders as blank
	FG    Color // resolved foreground (glyph) color
	BG    Color // resolved background color
	Attrs TermAttr
	Width uint8 // 1 normal, 2 wide (occupies this + next cell), 0 continuation of a wide cell
}

// TermCursorStyle selects how the cursor is drawn.
type TermCursorStyle uint8

// TermCursorStyle values.
const (
	TermCursorBlock TermCursorStyle = iota
	TermCursorBar
	TermCursorUnderline
)

// TermCursor positions and styles the text cursor within the grid.
type TermCursor struct {
	Col     int
	Row     int
	Color   Color
	Style   TermCursorStyle
	Visible bool
}

// TermSelRange is a linear (row-major) selection over cell indices,
// half-open [Start, End). Start == End means no selection. Color is
// the selection highlight; unset falls back to a default tint.
type TermSelRange struct {
	Start int
	End   int
	Color Color
}

// TermGridData is the resolved grid buffer carried by a RenderCmd.
// The caller may reuse the Cells slice across frames. Held by pointer
// so no per-cell data is copied into the render command stream.
type TermGridData struct {
	Cells        []TermCell // len == Cols*Rows, row-major
	Style        TextStyle  // font/size for the whole grid
	Cursor       TermCursor
	Selection    TermSelRange
	Cols         int
	Rows         int
	CellW, CellH float32
}

// TermGridCfg configures a TermGrid widget.
type TermGridCfg struct {
	OnKeyDown     func(*Layout, *Event, *Window)
	OnClick       func(*Layout, *Event, *Window)
	OnMouseScroll func(*Layout, *Event, *Window)

	ID              string
	A11YLabel       string
	A11YDescription string

	Cells     []TermCell // len must be >= Cols*Rows; row-major
	Cursor    TermCursor
	Selection TermSelRange
	TextStyle TextStyle

	Cols    int
	Rows    int
	CellW   float32
	CellH   float32
	IDFocus uint32
	Sizing  Sizing
}

// termGridView implements View for a terminal character grid.
type termGridView struct {
	cfg TermGridCfg
}

// TermGrid creates a terminal character-grid widget. Cols, Rows,
// CellW, and CellH define the geometry; Cells is the row-major cell
// buffer (len >= Cols*Rows). Default sizing is Fixed at
// Cols*CellW x Rows*CellH.
func TermGrid(cfg TermGridCfg) View {
	if cfg.Sizing == (Sizing{}) {
		cfg.Sizing = FixedFixed
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = DefaultTextStyle
	}
	if cfg.Cols <= 0 || cfg.Rows <= 0 ||
		cfg.CellW <= 0 || math.IsNaN(float64(cfg.CellW)) || math.IsInf(float64(cfg.CellW), 0) ||
		cfg.CellH <= 0 || math.IsNaN(float64(cfg.CellH)) || math.IsInf(float64(cfg.CellH), 0) {
		panic("TermGrid: Cols, Rows must be positive; CellW, CellH must be positive and finite")
	}
	if len(cfg.Cells) < cfg.Cols*cfg.Rows {
		panic("TermGrid: len(Cells) must be >= Cols*Rows")
	}
	return &termGridView{cfg: cfg}
}

func (tv *termGridView) Content() []View { return nil }

func (tv *termGridView) GenerateLayout(w *Window) Layout {
	c := &tv.cfg

	var events *eventHandlers
	if c.OnKeyDown != nil || c.OnClick != nil || c.OnMouseScroll != nil {
		events = w.allocEventHandlers(eventHandlers{
			OnKeyDown:     c.OnKeyDown,
			OnClick:       c.OnClick,
			ClickButton:   MouseLeft,
			OnMouseScroll: c.OnMouseScroll,
		})
	}

	// Default fixed geometry from cell metrics.
	width := float32(c.Cols) * c.CellW
	height := float32(c.Rows) * c.CellH

	// Focusable grid advertises as a text box to assistive tech.
	a11yRole := AccessRoleImage
	if c.IDFocus > 0 {
		a11yRole = AccessRoleTextArea
	}

	tg := &TermGridData{
		Cells:     c.Cells,
		Style:     c.TextStyle,
		Cursor:    c.Cursor,
		Selection: c.Selection,
		Cols:      c.Cols,
		Rows:      c.Rows,
		CellW:     c.CellW,
		CellH:     c.CellH,
	}

	layout := Layout{
		Shape: w.allocShape(Shape{
			shapeType: shapeTermGrid,
			ID:        c.ID,
			A11YRole:  a11yRole,
			A11Y:      makeA11YInfo(c.A11YLabel, c.A11YDescription),
			Width:     width,
			Height:    height,
			Sizing:    c.Sizing,
			IDFocus:   c.IDFocus,
			events:    events,
			tg:        tg,
		}),
	}
	applyFixedSizingConstraints(layout.Shape)
	return layout
}
