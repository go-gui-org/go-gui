package gui

// markdown_select.go implements cross-block text selection for the Markdown widget.
// Selection state is keyed by MarkdownCfg.IDFocus and spans all RTF blocks
// (paragraphs, headings, list items, blockquotes). Non-RTF blocks (tables,
// images, code, math, HR) are skipped; their content is not selectable.

import (
	"sort"
	"strings"
	"time"

	"github.com/go-gui-org/go-glyph"
)

// mdSelState is the cross-block selection state for one markdown widget.
type mdSelState struct {
	SelBeg        uint32
	SelEnd        uint32
	LastClickTime int64
}

// mdBlockInfo describes one selectable RTF block within a markdown widget,
// populated each frame by markdownContainerAmendLayout.
// Layout is a shallow copy (not a pointer) so the block list can be used safely
// in drag callbacks that outlive the current frame's shape tree.
type mdBlockInfo struct {
	H         float32
	StartRune uint32
	RuneLen   uint32
	Layout    glyph.Layout
	FlatText  string
	ShapeX    float32
	ShapeY    float32
}

// mdBlockCtx carries the markdown widget ID and the current cumulative rune
// offset so render functions can stamp each RTF block with its position within
// the markdown's virtual flat text.
type mdBlockCtx struct {
	ID    uint32
	Start uint32
}

// markdownBlockAmendSel is called from rtfMarkdownAmendLayout for each RTF
// block that belongs to a markdown widget. It computes which portion of this
// block is covered by the markdown selection and writes TextSelBeg/TextSelEnd.
func markdownBlockAmendSel(l *Layout, w *Window) {
	tc := l.Shape.TC
	if tc == nil || tc.MarkdownID == 0 {
		return
	}
	mdID := tc.MarkdownID
	st := StateReadOr(w, nsMdSel, mdID, mdSelState{})
	beg, end := u32Sort(st.SelBeg, st.SelEnd)
	blockStart := tc.MarkdownBlockStart
	blockEnd := blockStart + tc.MarkdownRuneLen
	if end <= blockStart || beg >= blockEnd {
		tc.TextSelBeg = 0
		tc.TextSelEnd = 0
		return
	}
	localBeg := max(beg, blockStart) - blockStart
	localEnd := min(end, blockEnd) - blockStart
	tc.TextSelBeg = localBeg
	tc.TextSelEnd = localEnd
}

// markdownContainerAmendLayout is the AmendLayout hook on the markdown Column.
// It walks all RTF descendants belonging to this markdown widget, rebuilds the
// block-position list in the StateMap, and triggers per-block selection update.
func markdownContainerAmendLayout(l *Layout, w *Window) {
	mdID := l.Shape.IDFocus
	if mdID == 0 {
		return
	}

	// Collect all RTF blocks in this markdown by walking descendants.
	var blocks []mdBlockInfo
	mdWalkBlocks(l, mdID, &blocks)

	// Sort by Y position (should already be ordered, but make it robust).
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].ShapeY < blocks[j].ShapeY
	})

	// Persist for drag callbacks.
	bm := StateMap[uint32, []mdBlockInfo](w, nsMdBlocks, capMany)
	bm.Set(mdID, blocks)
}

// mdWalkBlocks recursively walks the layout tree to collect RTF blocks
// belonging to the given markdown widget.
func mdWalkBlocks(l *Layout, mdID uint32, out *[]mdBlockInfo) {
	if l.Shape != nil && l.Shape.TC != nil &&
		l.Shape.TC.MarkdownID == mdID &&
		l.Shape.HasRtfLayout() {
		tc := l.Shape.TC
		*out = append(*out, mdBlockInfo{
			H:         l.Shape.Height,
			StartRune: tc.MarkdownBlockStart,
			RuneLen:   tc.MarkdownRuneLen,
			Layout:    *tc.RtfLayout, // shallow copy — safe for Items slice from cache
			FlatText:  tc.RtfFlatText,
			ShapeX:    l.Shape.X,
			ShapeY:    l.Shape.Y,
		})
	}
	for i := range l.Children {
		mdWalkBlocks(&l.Children[i], mdID, out)
	}
}

// markdownBlockOnClick is the OnClick handler for RTF blocks inside a
// markdown widget with cross-block selection enabled.
func markdownBlockOnClick(l *Layout, e *Event, w *Window) {
	rtfOnClick(l, e, w)
	if e.MouseButton == MouseRight {
		return
	}

	shape := l.Shape
	if shape.TC == nil || !shape.HasRtfLayout() {
		return
	}
	mdID := shape.TC.MarkdownID
	if mdID == 0 {
		return
	}
	w.SetIDFocus(mdID)

	// Compute abs rune position within the markdown flat text.
	gl := shape.TC.RtfLayout
	flatText := shape.TC.RtfFlatText
	byteIdx := gl.GetClosestOffset(e.MouseX, e.MouseY)
	localRune := byteToRuneIndex(flatText, byteIdx)
	absRune := uint32(localRune) + shape.TC.MarkdownBlockStart

	imap := StateMap[uint32, mdSelState](w, nsMdSel, capMany)
	st, _ := imap.Get(mdID)

	now := time.Now().UnixMilli()
	doubleClick := st.LastClickTime > 0 &&
		now-st.LastClickTime <= doubleClickThresholdMs
	st.LastClickTime = now

	if doubleClick {
		bBeg, bEnd := gl.GetWordAtIndex(byteIdx)
		wb := uint32(byteToRuneIndex(flatText, bBeg)) + shape.TC.MarkdownBlockStart
		we := uint32(byteToRuneIndex(flatText, bEnd)) + shape.TC.MarkdownBlockStart
		st.SelBeg = wb
		st.SelEnd = we
	} else {
		st.SelBeg = absRune
		st.SelEnd = absRune
	}
	imap.Set(mdID, st)
	e.IsHandled = true

	// Capture drag state. Copy layout value so it's safe across frames.
	anchorBeg := st.SelBeg
	anchorEnd := st.SelEnd
	dragMdID := mdID
	isDouble := doubleClick
	dragGl := *gl
	dragFlatText := flatText
	dragBlockStart := shape.TC.MarkdownBlockStart

	w.MouseLock(MouseLockCfg{
		MouseMove: func(_ *Layout, e *Event, w *Window) {
			bm := StateMap[uint32, []mdBlockInfo](w, nsMdBlocks, capMany)
			blocks, _ := bm.Get(dragMdID)
			absPos := mdHitAbsRune(e.MouseX, e.MouseY,
				blocks, dragGl, dragFlatText, dragBlockStart)

			dim := StateMap[uint32, mdSelState](w, nsMdSel, capMany)
			dst, _ := dim.Get(dragMdID)
			if isDouble {
				// Extend word-by-word.
				if absPos < anchorBeg {
					dst.SelBeg = anchorEnd
					dst.SelEnd = absPos
				} else {
					dst.SelBeg = anchorBeg
					dst.SelEnd = absPos
				}
			} else {
				dst.SelBeg = anchorBeg
				dst.SelEnd = absPos
			}
			dim.Set(dragMdID, dst)
		},
		MouseUp: func(_ *Layout, _ *Event, w *Window) {
			w.MouseUnlock()
		},
	})
}

// mdHitAbsRune finds the absolute rune position (in the markdown flat text)
// for a window-absolute mouse position by scanning the block list.
func mdHitAbsRune(
	mx, my float32,
	blocks []mdBlockInfo,
	fallbackGL glyph.Layout,
	fallbackText string,
	fallbackStart uint32,
) uint32 {
	if len(blocks) == 0 {
		bi := fallbackGL.GetClosestOffset(mx, my)
		return fallbackStart + uint32(byteToRuneIndex(fallbackText, bi))
	}
	// Find the block the mouse is over, defaulting to the last block above.
	best := &blocks[0]
	for i := range blocks {
		b := &blocks[i]
		if my >= b.ShapeY && my < b.ShapeY+b.H {
			best = b
			break
		}
		if my >= b.ShapeY+b.H {
			best = b
		}
	}
	relX := mx - best.ShapeX
	relY := my - best.ShapeY
	bi := best.Layout.GetClosestOffset(relX, relY)
	localRune := uint32(byteToRuneIndex(best.FlatText, bi))
	return best.StartRune + localRune
}

// markdownContainerOnKeyDown handles keyboard events for the markdown container.
// Supports Ctrl+A (select all) and Ctrl+C (copy).
func markdownContainerOnKeyDown(l *Layout, e *Event, w *Window) {
	mdID := l.Shape.IDFocus
	if mdID == 0 || !w.IsFocus(mdID) {
		return
	}
	bm := StateMap[uint32, []mdBlockInfo](w, nsMdBlocks, capMany)
	blocks, _ := bm.Get(mdID)
	if len(blocks) == 0 {
		return
	}

	handled := true
	switch e.KeyCode {
	case KeyA:
		if e.Modifiers.HasAny(ModCtrl, ModSuper) {
			totalRunes := uint32(0)
			for _, b := range blocks {
				totalRunes += b.RuneLen
			}
			imap := StateMap[uint32, mdSelState](w, nsMdSel, capMany)
			st, _ := imap.Get(mdID)
			st.SelBeg = 0
			st.SelEnd = totalRunes
			imap.Set(mdID, st)
		} else {
			handled = false
		}
	case KeyC:
		if e.Modifiers.HasAny(ModCtrl, ModSuper) {
			imap := StateMap[uint32, mdSelState](w, nsMdSel, capMany)
			st, _ := imap.Get(mdID)
			if st.SelBeg != st.SelEnd {
				beg, end := u32Sort(st.SelBeg, st.SelEnd)
				w.SetClipboard(mdExtractText(blocks, beg, end))
			}
		} else {
			handled = false
		}
	default:
		handled = false
	}

	if handled {
		e.IsHandled = true
	}
}

// mdExtractText extracts the text covered by [beg, end) rune range from the
// block list, joining blocks with newlines.
func mdExtractText(blocks []mdBlockInfo, beg, end uint32) string {
	var sb strings.Builder
	first := true
	for _, b := range blocks {
		blockEnd := b.StartRune + b.RuneLen
		if end <= b.StartRune || beg >= blockEnd {
			continue
		}
		if !first {
			sb.WriteByte('\n')
		}
		first = false
		localBeg := int(max(beg, b.StartRune) - b.StartRune)
		localEnd := int(min(end, blockEnd) - b.StartRune)
		runeCount := utf8RuneCount(b.FlatText)
		if localEnd > runeCount {
			localEnd = runeCount
		}
		if localBeg < localEnd {
			sb.WriteString(b.FlatText[runeToByteIndex(b.FlatText, localBeg):runeToByteIndex(b.FlatText, localEnd)])
		}
	}
	return sb.String()
}
