package gui

// The in-window file browser's view and event handlers (#831). The
// state, navigation and accept logic are in file_browser.go.

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// fileBrowserView builds the dialog body. It runs every frame and reads
// only w.fileBrowser, which handlers change.
func fileBrowserView(w *Window) View {
	st := w.fileBrowser
	if st == nil {
		return Column(ContainerCfg{Padding: NoPadding, SizeBorder: NoBorder})
	}
	loc := activeLocaleShared()
	content := make([]View, 0, 6)

	content = append(content, Input(InputCfg{
		ID:            fileBrowserPathID,
		Text:          st.pathText,
		Sizing:        FillFit,
		A11YCfg:       A11YCfg{A11YLabel: "Folder"},
		OnTextChanged: fileBrowserPathChanged,
		OnEnter:       fileBrowserPathEnter,
	}))

	if len(st.cfg.filters) > 1 {
		opts := make([]SelectOption, len(st.cfg.filters))
		for i, f := range st.cfg.filters {
			opts[i] = SelectOption{Label: fileBrowserFilterLabel(f), Value: strconv.Itoa(i)}
		}
		content = append(content, Select(SelectCfg{
			ID:       fileBrowserFilterID,
			Options:  opts,
			Selected: []string{strconv.Itoa(st.filterIdx)},
			OnSelect: fileBrowserFilterSelected,
		}))
	}

	content = append(content, VirtualList(VirtualListCfg{
		ID:         fileBrowserListID,
		ItemCount:  len(st.entries),
		ItemView:   fileBrowserRowView(w, st),
		ItemKey:    fileBrowserRowKey(st),
		OnKeyDown:  fileBrowserListKeyDown,
		Height:     fileBrowserListHeight,
		Sizing:     FillFixed,
		A11YCfg:    A11YCfg{A11YLabel: st.dir},
		Padding:    NoPadding,
		SizeBorder: BorderThin,
	}))

	if st.cfg.mode == fileBrowserSave {
		content = append(content, Input(InputCfg{
			ID:            fileBrowserNameID,
			Text:          st.name,
			Sizing:        FillFit,
			A11YCfg:       A11YCfg{A11YLabel: "File name"},
			OnTextChanged: fileBrowserNameChanged,
			OnEnter:       fileBrowserNameEnter,
		}))
	}

	switch {
	case st.errText != "":
		content = append(content, Text(TextCfg{Text: st.errText, Mode: TextModeWrap}))
	case st.confirmPath != "":
		content = append(content, Text(TextCfg{
			Text: fmt.Sprintf(fileBrowserReplacePrompt, filepath.Base(st.confirmPath)),
			Mode: TextModeWrap,
		}))
	}

	okLabel := loc.StrOK
	if st.cfg.mode == fileBrowserSave {
		okLabel = loc.StrSave
	}
	content = append(content, Row(ContainerCfg{
		Sizing:     FillFit,
		HAlign:     DefaultDialogStyle.AlignButtons,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Spacing:    SpacingMedium,
		Content: []View{
			Button(ButtonCfg{
				ID:       fileBrowserOKID,
				Disabled: !fileBrowserCanAccept(w),
				Content:  []View{Text(TextCfg{Text: okLabel})},
				OnClick: func(ctx EventCtx) {
					fileBrowserAccept(ctx.Window)
					ctx.Consume()
				},
			}),
			Button(ButtonCfg{
				ID:      fileBrowserCancelID,
				Content: []View{Text(TextCfg{Text: loc.StrCancel})},
				OnClick: func(ctx EventCtx) {
					// DialogDismiss reports the cancel.
					ctx.Window.DialogDismiss()
					ctx.Consume()
				},
			}),
		},
	}))

	return Column(ContainerCfg{
		Sizing:     FillFit,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Spacing:    SpacingMedium,
		Content:    content,
	})
}

// fileBrowserFilterLabel shows a filter as its name plus its patterns,
// "Text (*.txt)", or just the patterns when it has no name.
func fileBrowserFilterLabel(f NativeFileFilter) string {
	pats := make([]string, 0, len(f.Extensions))
	for _, raw := range f.Extensions {
		if ext, err := nativeNormalizeExtension(raw); err == nil && ext != "" {
			pats = append(pats, "*."+ext)
		}
	}
	p := strings.Join(pats, ", ")
	switch {
	case f.Name == "":
		return p
	case p == "":
		return f.Name
	}
	return f.Name + " (" + p + ")"
}

// fileBrowserRowView returns the VirtualList row builder.
func fileBrowserRowView(w *Window, st *fileBrowserState) func(int, float32) View {
	d := &defaultListBoxStyle
	colors := d.Colors
	cursor := w.VirtualListFocusedIndex(fileBrowserListID)
	return func(i int, _ float32) View {
		e := st.entries[i]
		label := e.name
		// "/" marks a folder on every OS. It is a display mark, not a
		// path, and it keeps the goldens the same on Windows.
		if e.isDir && e.name != fileBrowserParent {
			label += "/"
		}
		isCursor := i == cursor
		// The cursor row is the choice unless files are marked, so it
		// takes the selected fill. Folder mode returns the current
		// folder, not a row, so there only the focus ring shows.
		selected := slices.Contains(st.marked, e.name) ||
			(isCursor && len(st.marked) == 0 && st.cfg.mode != fileBrowserFolder)
		bg, bgHover := rowFill(colors, ColorTransparent, selected, false)
		a11yState := AccessStateNone
		if selected {
			a11yState = AccessStateSelected
		}
		row := i
		return Row(ContainerCfg{
			ID:          fileBrowserRowID(i),
			A11YRole:    AccessRoleListItem,
			A11YCfg:     A11YCfg{A11YLabel: label},
			A11YState:   a11yState,
			Color:       bg,
			Padding:     listBoxItemPad,
			SizeBorder:  NoBorder,
			Sizing:      FillFit,
			Content:     []View{Text(TextCfg{Text: label, TextStyle: d.textStyleNormal})},
			AmendLayout: listBoxItemRingAmend(isCursor, fileBrowserListID, colors.BorderFocus),
			OnClick: func(ctx EventCtx) {
				fileBrowserRowClick(ctx, row)
				ctx.Consume()
			},
			OnHover: func(ctx EventCtx) {
				ctx.Window.setMouseCursor(CursorPointingHand)
				ctx.Layout.Shape.Color = bgHover
			},
		})
	}
}

// fileBrowserRowKey keys rows by name, so a measured height follows its
// entry when the filter or the folder changes.
func fileBrowserRowKey(st *fileBrowserState) func(int) string {
	return func(i int) string { return st.entries[i].name }
}

// fileBrowserRowClick moves the cursor to row i. A second click on the
// same row inside the double-tap gap activates it. Ctrl/Cmd-click marks
// a file when AllowMultiple is set.
func fileBrowserRowClick(ctx EventCtx, i int) {
	w := ctx.Window
	st := w.fileBrowser
	if st == nil || i >= len(st.entries) {
		return
	}
	now := w.Now().UnixNano()
	toggle := ctx.Event != nil && isShortcut(ctx.Event.Modifiers)
	// A Ctrl/Cmd-click marks or unmarks; two quick ones on the same file
	// are an unmark, not a double-click that would drop the other marks.
	if !toggle && st.lastClickAt != 0 && st.lastClickRow == i &&
		now-st.lastClickAt < int64(gestureDoubleTapGap) {
		st.lastClickAt = 0
		fileBrowserActivate(w, i)
		return
	}
	st.lastClickRow, st.lastClickAt = i, now
	w.SetVirtualListFocusedIndex(fileBrowserListID, i)
	w.SetFocus(fileBrowserListID)
	e := st.entries[i]
	switch {
	case st.cfg.mode == fileBrowserOpen && st.cfg.allowMultiple && toggle && !e.isDir:
		if j := slices.Index(st.marked, e.name); j >= 0 {
			st.marked = slices.Delete(st.marked, j, j+1)
		} else {
			st.marked = append(st.marked, e.name)
		}
	default:
		st.marked = st.marked[:0]
	}
	if st.cfg.mode == fileBrowserSave && !e.isDir {
		st.name = e.name
		st.confirmPath = ""
	}
}

// fileBrowserListKeyDown handles Enter on the list. The arrows, pages,
// Home and End are VirtualList's own; Escape goes on to the dialog.
func fileBrowserListKeyDown(ctx EventCtx) {
	if ctx.Event == nil || ctx.Event.KeyCode != KeyEnter {
		return
	}
	w := ctx.Window
	if st := w.fileBrowser; st != nil && len(st.marked) > 0 {
		fileBrowserAccept(w)
	} else {
		fileBrowserActivate(w, w.VirtualListFocusedIndex(fileBrowserListID))
	}
	ctx.Consume()
}

func fileBrowserPathChanged(text string, ctx EventCtx) {
	if st := ctx.Window.fileBrowser; st != nil {
		st.pathText = text
	}
}

func fileBrowserPathEnter(ctx EventCtx) {
	// An empty path bar names no folder. Resolving it would give ".",
	// the process working folder, a jump the user did not ask for.
	if st := ctx.Window.fileBrowser; st != nil && strings.TrimSpace(st.pathText) != "" {
		fileBrowserNavigate(ctx.Window, fileBrowserResolve(st.dir, st.pathText))
	}
	ctx.Consume()
}

func fileBrowserNameChanged(text string, ctx EventCtx) {
	if st := ctx.Window.fileBrowser; st != nil {
		st.name = text
		st.confirmPath = ""
		st.errText = ""
	}
}

func fileBrowserNameEnter(ctx EventCtx) {
	fileBrowserAccept(ctx.Window)
	ctx.Consume()
}

func fileBrowserFilterSelected(sel []string, ctx EventCtx) {
	if len(sel) > 0 {
		if i, err := strconv.Atoi(sel[0]); err == nil {
			fileBrowserSetFilter(ctx.Window, i)
		}
	}
	ctx.Consume()
}

// fileBrowserResolve turns typed path-bar text into a folder path. A
// path that starts at a root is taken as given; anything else is
// relative to the current folder. filepath.IsAbs alone is not enough:
// on Windows "\proj" has no drive letter and is not "absolute", but it
// still names a folder from the root of the current drive.
func fileBrowserResolve(cur, typed string) string {
	if typed == "" || cur == "" ||
		filepath.IsAbs(typed) || filepath.VolumeName(typed) != "" ||
		os.IsPathSeparator(typed[0]) {
		return filepath.Clean(typed)
	}
	return filepath.Join(cur, typed)
}
