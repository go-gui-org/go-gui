package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// demo_virtual_list.go — the VirtualList page. It shows the four things
// the uniform virtualized widgets (ListBox, Table, Tree) cannot do:
// rows of different heights, a jump to a row by index, a pin to the
// bottom while rows are added, and an insert at the top that keeps
// each measured height with its own row.

const (
	// virtualListDemoID is the list's leaf ID. The showcase puts the
	// demo under ID-bearing panels, so the index scroll calls take the
	// effective ID from virtualListDemoEffID, not this leaf.
	virtualListDemoID = "vl-demo"
	// virtualListDemoRows is the start size of the list.
	virtualListDemoRows = 5_000
)

// virtualListDemoWho and virtualListDemoBodies give each row its text.
// The bodies have very different lengths, so the wrapped rows have
// very different heights, and only the layout can know them.
var (
	virtualListDemoWho    = [...]string{"ana", "bo", "cy", "dee"}
	virtualListDemoBodies = [...]string{
		"ack",
		"Rebased onto main and ran the gate again; all green.",
		"The height of a row is known only after arrange, one frame " +
			"after the row is built. The list starts from an estimate " +
			"and corrects it with each row it measures.",
		"Shipping tomorrow.",
		"Rows outside the viewport are never built. Two empty spacers " +
			"hold their space, and the spacer sizes come from a " +
			"prefix-sum tree over the known and estimated heights. Far " +
			"down the list, the spacer above you stands in for thousands " +
			"of rows that do not exist.",
	}
)

// virtualListDemoSeqs returns the sequence numbers 0..n-1.
func virtualListDemoSeqs(n int) []int {
	seqs := make([]int, n)
	for i := range seqs {
		seqs[i] = i
	}
	return seqs
}

// virtualListDemoEffID returns the list's effective ID from the last
// frame. The index scroll API takes an effective ID, and a handler
// runs outside generation, where Window.EffID has no scope to join.
func virtualListDemoEffID(w *gui.Window) string {
	if ids := w.ResolveID(virtualListDemoID); len(ids) > 0 {
		return ids[0]
	}
	return virtualListDemoID
}

func demoVirtualList(w *gui.Window) gui.View {
	app := appState(w)
	t := gui.CurrentTheme()
	// Copy the slice header: ItemView and ItemKey run during this
	// frame's generation, and a handler may replace app.VLSeqs before
	// the next one.
	seqs := app.VLSeqs
	// The keyboard cursor is an index the list keeps; the row paints
	// it. Read once per frame, not once per row.
	focused := w.VirtualListFocusedIndex(virtualListDemoEffID(w))

	content := []gui.View{
		gui.Text(gui.TextCfg{
			Text: "Each row is a card with wrapped text, so the rows have " +
				"different heights. Only the rows in view are built. Jump " +
				"to a row by number, add rows at the end with the list " +
				"pinned to the bottom, or insert rows at the top.",
			TextStyle: t.TextStyleBody,
			Mode:      gui.TextModeWrap,
		}),
		virtualListDemoJumpRow(app),
	}
	if app.VLJumpErr != "" {
		content = append(content, gui.Text(gui.TextCfg{
			Text:      app.VLJumpErr,
			TextStyle: t.TextStyleError,
		}))
	}
	content = append(content,
		virtualListDemoEditRow(app),
		gui.VirtualList(gui.VirtualListCfg{
			ID:        virtualListDemoID,
			A11YCfg:   gui.A11YCfg{A11YLabel: "Message feed"},
			ItemCount: len(seqs),
			// The showcase detail panel scrolls, so it gives the list
			// no height of its own. Height makes the list virtualize
			// from the first frame.
			Height: 360,
			Sizing: gui.FillFixed,
			// The key is the sequence number, not the index. An insert
			// at the top moves every index by one; the key keeps each
			// measured height with its own row.
			ItemKey: func(i int) string {
				return strconv.Itoa(seqs[i])
			},
			// The width argument is ignored on purpose. A row that
			// sets MinWidth from it makes the list wider on each frame.
			ItemView: func(i int, _ float32) gui.View {
				return virtualListDemoCard(seqs[i], i, i == focused)
			},
		}),
		gui.Text(gui.TextCfg{
			Text:      fmt.Sprintf("%d rows", len(seqs)),
			TextStyle: t.TextStyleSecondary,
		}),
	)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Padding: gui.NoPadding,
		Content: content,
	})
}

// virtualListDemoJumpRow is the row-number box and the three index
// scroll calls.
func virtualListDemoJumpRow(app *ShowcaseApp) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		VAlign:  gui.VAlignMiddle,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Input(gui.InputCfg{
				ID:          "vl-jump",
				Text:        app.VLJump,
				Placeholder: "row number",
				Width:       120,
				A11YCfg:     gui.A11YCfg{A11YLabel: "Row number"},
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					a := appState(ctx.Window)
					a.VLJump = s
					a.VLJumpErr = ""
				},
			}),
			// Index addressing: the target row is not built, so there
			// is no view to find by ID. The height model knows where it
			// is.
			virtualListDemoButton("vl-center", "Center", func(ctx gui.EventCtx) {
				if n, ok := virtualListDemoJumpIndex(ctx.Window); ok {
					ctx.Window.ScrollToIndexAt(virtualListDemoEffID(ctx.Window), n, 0.5)
				}
			}),
			virtualListDemoButton("vl-reveal", "Reveal", func(ctx gui.EventCtx) {
				// The least scroll that shows the row; no scroll when it
				// is in view already.
				if n, ok := virtualListDemoJumpIndex(ctx.Window); ok {
					ctx.Window.ScrollIndexIntoView(virtualListDemoEffID(ctx.Window), n)
				}
			}),
			virtualListDemoButton("vl-top", "Top", func(ctx gui.EventCtx) {
				ctx.Window.ScrollToIndex(virtualListDemoEffID(ctx.Window), 0)
			}),
			virtualListDemoButton("vl-end", "End", func(ctx gui.EventCtx) {
				ctx.Window.ScrollToEnd(virtualListDemoEffID(ctx.Window))
			}),
		},
	})
}

// virtualListDemoEditRow adds rows at either end and toggles the pin.
func virtualListDemoEditRow(app *ShowcaseApp) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		VAlign:  gui.VAlignMiddle,
		Padding: gui.NoPadding,
		Content: []gui.View{
			virtualListDemoButton("vl-insert-top", "Insert 10 at top", func(ctx gui.EventCtx) {
				virtualListDemoInsertTop(ctx.Window, 10)
			}),
			virtualListDemoButton("vl-append", "Append 1", func(ctx gui.EventCtx) {
				virtualListDemoAppend(ctx.Window)
			}),
			gui.Toggle(gui.ToggleCfg{
				ID:       "vl-pin",
				Label:    "Pin to bottom",
				Selected: app.VLPinned,
				OnClick: func(ctx gui.EventCtx) {
					a := appState(ctx.Window)
					a.VLPinned = !a.VLPinned
					if a.VLPinned {
						ctx.Window.ScrollToEnd(virtualListDemoEffID(ctx.Window))
					}
					ctx.Consume()
				},
			}),
		},
	})
}

// virtualListDemoButton is a text button with an ID, so tests can
// click it.
func virtualListDemoButton(id, label string, onClick func(gui.EventCtx)) gui.View {
	return gui.Button(gui.ButtonCfg{
		ID: id,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: gui.CurrentTheme().TextStyleBody}),
		},
		OnClick: func(ctx gui.EventCtx) {
			onClick(ctx)
			ctx.Consume()
		},
	})
}

// virtualListDemoJumpIndex parses the row-number box. On failure it
// sets the error text under the box and returns false.
func virtualListDemoJumpIndex(w *gui.Window) (int, bool) {
	a := appState(w)
	// Trim: a pasted number often carries a space, and Atoi rejects it.
	n, err := strconv.Atoi(strings.TrimSpace(a.VLJump))
	if err != nil || n < 0 || n >= len(a.VLSeqs) {
		a.VLJumpErr = fmt.Sprintf("enter a row number from 0 to %d", len(a.VLSeqs)-1)
		return 0, false
	}
	a.VLJumpErr = ""
	return n, true
}

// virtualListDemoInsertTop puts n new rows before row 0. The new rows
// get new keys, and the old rows keep theirs, so their measured
// heights stay valid. Without ItemKey the list would show each old
// row at the height of the row that was at its index before.
func virtualListDemoInsertTop(w *gui.Window, n int) {
	a := appState(w)
	listID := virtualListDemoEffID(w)
	// The keyboard cursor is an index, not a key, so the insert would
	// leave it on the row now n places above the focused one. Move it
	// with its row. SetVirtualListFocusedIndex also scrolls the cursor
	// into view, so do it only when the cursor row is built this
	// frame: a cursor left far away (after a jump by button) must not
	// pull the view back to it.
	cursor := w.VirtualListFocusedIndex(listID)
	cursorBuilt := cursor < len(a.VLSeqs) &&
		len(w.ResolveID(gui.ScopeIDN(virtualListDemoID, "row", a.VLSeqs[cursor]))) > 0
	head := make([]int, n, n+len(a.VLSeqs))
	for i := range head {
		head[i] = a.VLNextSeq + i
	}
	a.VLNextSeq += n
	// A new slice, not an in-place shift: the previous frame's
	// ItemView closures still hold the old one.
	a.VLSeqs = append(head, a.VLSeqs...)
	if !cursorBuilt {
		return
	}
	// SetVirtualListFocusedIndex clamps to the row count the list saw
	// on its last build, which is the count before this insert. A
	// cursor near the end would be cut back to the old last row. Queue
	// it twice: the outer command runs before the next build, the
	// inner one after it, when the list knows the new count.
	w.QueueCommand(func(before *gui.Window) {
		before.QueueCommand(func(after *gui.Window) {
			after.SetVirtualListFocusedIndex(listID, cursor+n)
		})
	})
}

// virtualListDemoAppend adds one row at the end. When the pin is on,
// ScrollToEnd follows it. A percentage scroll would not: the content
// height under virtualization is an estimate, so "100%" moves as rows
// are measured.
func virtualListDemoAppend(w *gui.Window) {
	a := appState(w)
	a.VLSeqs = append(a.VLSeqs, a.VLNextSeq)
	a.VLNextSeq++
	if a.VLPinned {
		w.ScrollToEnd(virtualListDemoEffID(w))
	}
}

// virtualListDemoCard builds one row. Its height comes from the
// wrapped body text, which no scalar row height can describe.
func virtualListDemoCard(seq, index int, focused bool) gui.View {
	t := gui.CurrentTheme()
	bg := t.ColorPanel
	head := t.TextStyleLabel
	// Zero: Text falls back to the default body style.
	var body gui.TextStyle
	if focused {
		// Text over a ColorSelect fill takes ColorTextOnSelect; the
		// page text colors lose contrast on a strong accent.
		bg = t.ColorSelect
		head.Color = t.ColorTextOnSelect
		body = t.TextStyleBody
		body.Color = t.ColorTextOnSelect
	}
	return gui.Column(gui.ContainerCfg{
		// The key in the ID, not the index: the row keeps its identity
		// when rows are inserted above it.
		ID:     gui.ScopeIDN(virtualListDemoID, "row", seq),
		Sizing: gui.FillFit,
		// The list has no spacing between rows, because a gap is
		// height the model does not know. The margin is padding here.
		Padding:    gui.PaddingSmall,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Color:   bg,
				Padding: gui.PaddingSmall,
				Spacing: gui.SpacingTight,
				Radius:  gui.RadiusSmall,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("row %d · #%d · %s", index, seq,
							virtualListDemoWho[seq%len(virtualListDemoWho)]),
						TextStyle: head,
					}),
					gui.Text(gui.TextCfg{
						Text:      virtualListDemoBodies[seq%len(virtualListDemoBodies)],
						TextStyle: body,
						Mode:      gui.TextModeWrap,
						Sizing:    gui.FillFit,
					}),
				},
			}),
		},
	})
}
