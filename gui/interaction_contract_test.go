package gui

import (
	"strings"
	"testing"
)

// Interaction-contract matrix (issue #691): every held interaction
// state through every way a widget can lose the right to hold it.
// Each cell asserts one expected result — cleared, kept, or moved —
// derived from the principles below. Where the code disagrees with a
// principle, the cell fails: either the principle was wrong (record
// the decision in the row note) or the code is (fix it separately,
// with the cell as the regression test).
//
// P1 Focus follows eligibility: focus lives only where canTakeFocus
// holds. Disabled, hidden, removed or replaced focus moves to the
// first tab stop, or clears when none exists. Readonly keeps focus:
// the field is still focusable, it only refuses input.
// P2 Transient physical state dies with its cause: a press, a held
// Space press or a hover exists only while its pointer or key and an
// eligible target exist. A press target that disappears mid-press
// must never fire a click on whatever takes its place.
// P3 Composition belongs to a focused editable: IME clears unless
// its owner is still focused, editable, visible and present, with
// one exception — going readonly keeps an in-flight composition
// (destroying it loses work) while refusing new pre-edit.
// P4 The lock belongs to the gesture, not the widget: a drag lock
// has no owner ID, so widget-side transitions leave it running, but
// window focus loss and a modal dialog end the gesture.
// P5 Blur is a focus change: Tab away moves focus like any other
// focus change, clearing IME and the held Space press with it.

type contractTrans int

const (
	transDisabled contractTrans = iota
	transReadonly
	transHidden
	transRemoved
	transReplaced
	transBlur
	transUnfocus
	transDialog
)

func (t contractTrans) String() string {
	return []string{
		"disabled", "readonly", "hidden", "removed",
		"replaced", "blur", "unfocus", "dialog",
	}[t]
}

type contractState int

const (
	stFocus contractState = iota
	stIME
	stPress
	stSpace
	stDrag
	stHover
	stPopup
)

func (s contractState) String() string {
	return []string{
		"focus", "ime", "press", "space", "drag", "hover", "popup",
	}[s]
}

type contractWant int

const (
	// wantCleared: the state must be gone after the transition.
	wantCleared contractWant = iota
	// wantKept: the state must survive the transition.
	wantKept
	// wantMoved: focus must land on a live tab stop (or clear when
	// none exists), never park on the dead ID.
	wantMoved
	// wantNA: the transition does not apply (readonly on a button,
	// hidden on a widget with no Invisible flag).
	wantNA
)

// contractCtl carries the mutable transition flags one run reads.
// The view generator rebuilds from these every frame, so a
// transition is a flag flip plus a frame.
type contractCtl struct {
	disabled  bool
	readonly  bool
	invisible bool
	removed   bool
	replaced  bool
	clicks    int
}

const (
	contractID     = "c-subject"
	contractReplID = "c-subject2"
	contractNextID = "c-next"
)

// contractReplIDFor maps a begin ID to its post-replacement twin:
// the subject leaf swaps, everything else (dock tab path) follows.
func contractReplIDFor(id string) string {
	return strings.Replace(id, contractID, contractReplID, 1)
}

type contractRow struct {
	widget string
	state  contractState
	build  func(ctl *contractCtl, id string) View
	// beginIDSuffix resolves the interaction target when it is not
	// the subject itself: a dock tab, a listbox row. Empty takes
	// the subject ID. Read back from the frame, never spelled.
	beginIDSuffix string
	// popupNS names the open-state map for stPopup rows.
	popupNS string
	// guardClicks arms the gap-3 guard for stPress rows: after a
	// removed/replaced transition the test releases the pointer and
	// requires no click to have fired. Only makers that count into
	// ctl.clicks set it.
	guardClicks bool
	note        string
	wants       [8]contractWant
}

func (r contractRow) want(tr contractTrans) contractWant {
	return r.wants[tr]
}

// contractWindow builds the two-widget frame every cell drives: the
// subject under test plus a trailing button, so a moved focus has a
// live tab stop to land on and Tab away has somewhere to go.
func contractWindow(
	ctl *contractCtl, build func(ctl *contractCtl, id string) View,
) *Window {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		views := make([]View, 0, 2)
		if !ctl.removed {
			id := contractID
			if ctl.replaced {
				id = contractReplID
			}
			views = append(views, build(ctl, id))
		}
		views = append(views, Button(ButtonCfg{
			ID:      contractNextID,
			OnClick: func(EventCtx) {},
		}))
		return Column(ContainerCfg{Sizing: FillFill, Content: views})
	})
	return w
}

// contractTarget resolves the interaction target for a begin: the
// subject itself, or a descendant read back from the frame.
func contractTarget(
	t *testing.T, w *Window, row contractRow,
) string {
	t.Helper()
	if row.beginIDSuffix == "" {
		return contractID
	}
	for _, id := range w.EffectiveIDs() {
		if strings.HasSuffix(id, row.beginIDSuffix) &&
			strings.HasPrefix(id, contractID) {
			return id
		}
	}
	t.Fatalf("%s: no target with suffix %q in %v",
		row.widget, row.beginIDSuffix, w.EffectiveIDs())
	return ""
}

var contractCompEvent = &Event{
	Type:    EventIMEComposition,
	IMEText: "かん",
}

// contractBegin starts the row's interaction and returns the press
// point for states that take the pointer. It fails the test when the
// setup itself does not engage: a begin that never held the state is
// a harness failure, not a contract result.
func contractBegin(
	t *testing.T, w *Window, row contractRow, id string,
) (px, py float32) {
	t.Helper()
	switch row.state {
	case stFocus:
		w.SetFocus(id)
		w.TestRender(nil)
		if got := w.FocusID(); got != id {
			t.Fatalf("%s: setup focus = %q, want %q", row.widget, got, id)
		}
	case stIME:
		w.SetFocus(id)
		w.TestRender(nil)
		w.imeUpdate(contractCompEvent)
		if !w.IMEComposing() {
			t.Fatalf("%s: setup IME is not composing", row.widget)
		}
	case stPress:
		px, py = hoverOver(t, w, id)
		pressAt(w, MouseLeft, px, py)
		if !w.IsPressed(id) {
			t.Fatalf("%s: setup press not held", row.widget)
		}
	case stSpace:
		w.SetFocus(id)
		w.TestRender(nil)
		e := &Event{Type: EventKeyDown, KeyCode: KeySpace}
		w.handleKeyDownEvent(&w.layout, e)
		if !w.isKeyPressed(id) {
			t.Fatalf("%s: setup Space press not held", row.widget)
		}
	case stDrag:
		px, py = hoverOver(t, w, id)
		pressAt(w, MouseLeft, px, py)
		w.EventFn(&Event{
			Type: EventMouseMove, MouseX: px + 10, MouseY: py + 10,
		})
		w.TestRender(nil)
		if !w.mouseIsLocked() {
			t.Fatalf("%s: setup drag did not lock", row.widget)
		}
	case stHover:
		hoverOver(t, w, id)
		if !w.IsHovered(id) {
			t.Fatalf("%s: setup hover not reported", row.widget)
		}
	case stPopup:
		px, py = hoverOver(t, w, id)
		pressAt(w, MouseLeft, px, py)
		releaseAt(w, MouseLeft, px, py)
		if !StateReadOr(w, row.popupNS, id, false) {
			t.Fatalf("%s: setup popup did not open", row.widget)
		}
	}
	return px, py
}

// contractTransition applies the column: a flag flip plus a frame,
// a Tab key, a window event, or the dialog seam the existing dialog
// tests use.
func contractTransition(t *testing.T, w *Window, ctl *contractCtl, tr contractTrans) {
	t.Helper()
	switch tr {
	case transDisabled:
		ctl.disabled = true
	case transReadonly:
		ctl.readonly = true
	case transHidden:
		ctl.invisible = true
	case transRemoved:
		ctl.removed = true
	case transReplaced:
		ctl.replaced = true
	case transBlur:
		w.EventFn(&Event{Type: EventKeyDown, KeyCode: KeyTab})
	case transUnfocus:
		w.EventFn(&Event{Type: EventUnfocused})
	case transDialog:
		w.dialogCfg.visible = true
	}
	w.refreshLayout = true
	w.TestRender(nil)
}

// contractAssert checks the held state against the want.
// wantMoved additionally requires the focus to be live — a repair
// that parks on a dead ID fails here.
func contractAssert(
	t *testing.T, w *Window, row contractRow, id string,
	want contractWant,
) {
	t.Helper()
	switch row.state {
	case stFocus:
		got := w.FocusID()
		switch want {
		case wantCleared:
			if got != "" {
				t.Errorf("focus after transition = %q, want cleared", got)
			}
		case wantKept:
			if got != id {
				t.Errorf("focus after transition = %q, want %q kept",
					got, id)
			}
		case wantMoved:
			if got == id {
				t.Errorf("focus parked on dead ID %q", id)
			} else if got != "" {
				if _, ok := w.layout.FindByID(got); !ok {
					t.Errorf("focus moved to %q, not in tree", got)
				}
			}
		}
	case stIME:
		if want == wantKept && !w.IMEComposing() {
			t.Errorf("IME cleared, want kept (%s)", row.note)
		}
		if want == wantCleared && w.IMEComposing() {
			t.Errorf("IME kept, want cleared (%s)", row.note)
		}
	case stPress:
		held := w.IsPressed(id) || w.IsPressed(contractReplIDFor(id))
		if want == wantKept && !held {
			t.Errorf("press cleared, want kept (%s)", row.note)
		}
		if want == wantCleared && held {
			t.Errorf("press kept, want cleared (%s)", row.note)
		}
	case stSpace:
		if want == wantKept && !w.isKeyPressed(id) {
			t.Errorf("Space press cleared, want kept (%s)", row.note)
		}
		if want == wantCleared && w.isKeyPressed(id) {
			t.Errorf("Space press kept, want cleared (%s)", row.note)
		}
	case stDrag:
		if want == wantKept && !w.mouseIsLocked() {
			t.Errorf("drag unlocked, want kept (%s)", row.note)
		}
		if want == wantCleared && w.mouseIsLocked() {
			t.Errorf("drag locked, want cleared (%s)", row.note)
		}
	case stHover:
		if want == wantKept && !w.IsHovered(id) {
			t.Errorf("hover cleared, want kept (%s)", row.note)
		}
		if want == wantCleared && w.IsHovered(id) {
			t.Errorf("hover kept, want cleared (%s)", row.note)
		}
	case stPopup:
		open := StateReadOr(w, row.popupNS, id, false)
		if want == wantKept && !open {
			t.Errorf("popup closed, want kept (%s)", row.note)
		}
		if want == wantCleared && open {
			t.Errorf("popup open, want cleared (%s)", row.note)
		}
	}
}

// --- subject makers: each honors the ctl flags it can express ---

func contractInput(ctl *contractCtl, id string) View {
	return Input(InputCfg{
		ID: id, Sizing: FillFit,
		Disabled: ctl.disabled, ReadOnly: ctl.readonly,
		Invisible: ctl.invisible,
	})
}

func contractNumeric(ctl *contractCtl, id string) View {
	return NumericInput(NumericInputCfg{
		ID:       id,
		Disabled: ctl.disabled, ReadOnly: ctl.readonly,
		Invisible: ctl.invisible,
	})
}

func contractTextarea(ctl *contractCtl, id string) View {
	return Input(InputCfg{
		ID: id, Mode: InputMultiline, Height: 80,
		Disabled: ctl.disabled, ReadOnly: ctl.readonly,
		Invisible: ctl.invisible,
	})
}

func contractButton(ctl *contractCtl, id string) View {
	return Button(ButtonCfg{
		ID: id, Label: "ok",
		Disabled: ctl.disabled, Invisible: ctl.invisible,
		OnClick: func(EventCtx) { ctl.clicks++ },
	})
}

func contractToggle(ctl *contractCtl, id string) View {
	return Toggle(ToggleCfg{
		ID: id, Disabled: ctl.disabled, Invisible: ctl.invisible,
		OnClick: func(EventCtx) {},
	})
}

func contractSwitch(ctl *contractCtl, id string) View {
	return Switch(SwitchCfg{
		ID: id, Disabled: ctl.disabled, Invisible: ctl.invisible,
		OnClick: func(EventCtx) {},
	})
}

func contractRadio(ctl *contractCtl, id string) View {
	return Radio(RadioCfg{
		ID: id, Label: "r",
		Disabled: ctl.disabled, Invisible: ctl.invisible,
		OnClick: func(EventCtx) {},
	})
}

func contractSlider(ctl *contractCtl, id string) View {
	return Slider(SliderCfg{
		ID: id, Value: 50, Min: 0, Max: 100,
		Disabled: ctl.disabled, Invisible: ctl.invisible,
	})
}

func contractListbox(ctl *contractCtl, id string) View {
	return ListBox(ListBoxCfg{
		ID: id,
		Data: []ListBoxOption{
			NewListBoxOption("a", "A", "a"),
			NewListBoxOption("b", "B", "b"),
			NewListBoxOption("c", "C", "c"),
		},
		Reorderable: true,
		OnReorder:   func(string, string, EventCtx) {},
		OnSelect:    func([]string, EventCtx) {},
		Disabled:    ctl.disabled, Invisible: ctl.invisible,
	})
}

func contractSplitter(ctl *contractCtl, id string) View {
	pane := func(label string) SplitterPaneCfg {
		return SplitterPaneCfg{
			Content: []View{Text(TextCfg{Text: label})},
		}
	}
	return Splitter(SplitterCfg{
		ID: id, First: pane("one"), Second: pane("two"),
		Disabled: ctl.disabled, Invisible: ctl.invisible,
	})
}

// DockLayoutCfg carries no Disabled or Invisible, so the ctl flags
// ride a scopeless wrapper container: inheritance disables the tabs,
// and an invisible wrapper collapses the subtree.
func contractDock(ctl *contractCtl, id string) View {
	return Column(ContainerCfg{
		Disabled: ctl.disabled, Invisible: ctl.invisible,
		Sizing: FillFill,
		Content: []View{DockLayout(DockLayoutCfg{
			ID:     id,
			Sizing: FillFill,
			Root:   DockPanelGroup("g1", []string{"p1", "p2"}, "p1"),
			Panels: []DockPanelDef{
				{ID: "p1", Label: "Panel 1"},
				{ID: "p2", Label: "Panel 2"},
			},
		})},
	})
}

func contractSelect(ctl *contractCtl, id string) View {
	return Select(SelectCfg{
		ID: id, Options: []string{"a", "b"},
		Disabled: ctl.disabled, Invisible: ctl.invisible,
	})
}

// ComboboxCfg has no Invisible: the hidden column stays wantNA with
// a note, which is itself a finding of this matrix.
func contractCombobox(ctl *contractCtl, id string) View {
	return Combobox(ComboboxCfg{
		ID: id, Options: []string{"a", "b"},
		Disabled: ctl.disabled,
	})
}

// --- the table: one row per widget x held state ---

func contractFocusWants(readonly contractWant) [8]contractWant {
	return [8]contractWant{
		wantMoved, readonly, wantMoved, wantMoved,
		wantMoved, wantMoved, wantKept, wantCleared,
	}
}

const contractFocusNote = "P1: unfocus changes eligibility nowhere, " +
	"so FocusID stays; a modal takes focus (retainDialogFocus), and " +
	"the bare seam dialog offers no tab stop, so focus clears"

var contractRows = []contractRow{
	{widget: "input", state: stFocus, build: contractInput,
		note:  contractFocusNote,
		wants: contractFocusWants(wantKept)},
	{widget: "numeric", state: stFocus, build: contractNumeric,
		note:  contractFocusNote,
		wants: contractFocusWants(wantKept)},
	{widget: "textarea", state: stFocus, build: contractTextarea,
		note:  contractFocusNote,
		wants: contractFocusWants(wantKept)},
	{widget: "button", state: stFocus, build: contractButton,
		note:  contractFocusNote,
		wants: contractFocusWants(wantNA)},
	{widget: "toggle", state: stFocus, build: contractToggle,
		note:  contractFocusNote,
		wants: contractFocusWants(wantNA)},
	{widget: "switch", state: stFocus, build: contractSwitch,
		note:  contractFocusNote,
		wants: contractFocusWants(wantNA)},
	{widget: "radio", state: stFocus, build: contractRadio,
		note:  contractFocusNote,
		wants: contractFocusWants(wantNA)},
	{widget: "slider", state: stFocus, build: contractSlider,
		note:  contractFocusNote,
		wants: contractFocusWants(wantNA)},
	{widget: "listbox", state: stFocus, build: contractListbox,
		note:  contractFocusNote,
		wants: contractFocusWants(wantNA)},
	{widget: "select", state: stFocus, build: contractSelect,
		note:  contractFocusNote,
		wants: contractFocusWants(wantNA)},
	{widget: "combobox", state: stFocus, build: contractCombobox,
		note: contractFocusNote + "; hidden is NA: ComboboxCfg " +
			"has no Invisible, so the flag cannot be expressed",
		wants: [8]contractWant{
			wantMoved, wantNA, wantNA, wantMoved,
			wantMoved, wantMoved, wantKept, wantCleared,
		}},

	{widget: "input", state: stIME, build: contractInput,
		note: "P3: readonly keeps an in-flight composition but " +
			"refuses new pre-edit (dxui parity); dialog steals focus, " +
			"and the focus change clears the composition",
		wants: [8]contractWant{
			wantCleared, wantKept, wantCleared, wantCleared,
			wantCleared, wantCleared, wantCleared, wantCleared,
		}},
	{widget: "textarea", state: stIME, build: contractTextarea,
		note: "P3, as input",
		wants: [8]contractWant{
			wantCleared, wantKept, wantCleared, wantCleared,
			wantCleared, wantCleared, wantCleared, wantCleared,
		}},

	{widget: "input", state: stPress, build: contractInput,
		note: "P2: readonly keeps caret placement; Tab moves no " +
			"pointer, so the press survives blur",
		wants: [8]contractWant{
			wantCleared, wantKept, wantCleared, wantCleared,
			wantCleared, wantKept, wantCleared, wantCleared,
		}},
	{widget: "numeric", state: stPress, build: contractNumeric,
		note: "P2, as input",
		wants: [8]contractWant{
			wantCleared, wantKept, wantCleared, wantCleared,
			wantCleared, wantKept, wantCleared, wantCleared,
		}},
	{widget: "button", state: stPress, build: contractButton,
		guardClicks: true,
		note: "P2; removed/replaced additionally release the " +
			"pointer and require no click (gap 3)",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantKept, wantCleared, wantCleared,
		}},
	{widget: "toggle", state: stPress, build: contractToggle,
		note: "P2, as button",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantKept, wantCleared, wantCleared,
		}},
	{widget: "switch", state: stPress, build: contractSwitch,
		note: "P2, as button",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantKept, wantCleared, wantCleared,
		}},
	{widget: "radio", state: stPress, build: contractRadio,
		note: "P2, as button",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantKept, wantCleared, wantCleared,
		}},
	{widget: "listbox", state: stPress, build: contractListbox,
		note: "P2, as button",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantKept, wantCleared, wantCleared,
		}},
	{widget: "select", state: stPress, build: contractSelect,
		note: "P2, as button",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantKept, wantCleared, wantCleared,
		}},
	{widget: "combobox", state: stPress, build: contractCombobox,
		note: "P2, as button; hidden is NA: ComboboxCfg has no Invisible",
		wants: [8]contractWant{
			wantCleared, wantNA, wantNA, wantCleared,
			wantCleared, wantKept, wantCleared, wantCleared,
		}},
	{widget: "dock", state: stPress, build: contractDock,
		beginIDSuffix: ":tab:g1:p2",
		note:          "P2, as button, on the p2 tab",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantKept, wantCleared, wantCleared,
		}},

	{widget: "button", state: stSpace, build: contractButton,
		note: "P2: eligibility loss clears through focus repair; " +
			"blur and unfocus clear directly",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantCleared, wantCleared, wantCleared,
		}},
	{widget: "toggle", state: stSpace, build: contractToggle,
		note: "P2, as button",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantCleared, wantCleared, wantCleared,
		}},
	{widget: "switch", state: stSpace, build: contractSwitch,
		note: "P2, as button",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantCleared, wantCleared, wantCleared,
		}},
	{widget: "radio", state: stSpace, build: contractRadio,
		note: "P2, as button",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantCleared, wantCleared, wantCleared,
		}},

	{widget: "slider", state: stDrag, build: contractSlider,
		note: "P4: the ownerless lock survives widget-side loss " +
			"(gap 2 design pending); unfocus and dialog end it",
		wants: [8]contractWant{
			wantKept, wantNA, wantKept, wantKept,
			wantKept, wantKept, wantCleared, wantCleared,
		}},
	{widget: "listbox", state: stDrag, build: contractListbox,
		beginIDSuffix: ":item:a",
		note:          "P4, as slider, from the a row",
		wants: [8]contractWant{
			wantKept, wantNA, wantKept, wantKept,
			wantKept, wantKept, wantCleared, wantCleared,
		}},
	{widget: "splitter", state: stDrag, build: contractSplitter,
		note: "P4, as slider, from the handle",
		wants: [8]contractWant{
			wantKept, wantNA, wantKept, wantKept,
			wantKept, wantKept, wantCleared, wantCleared,
		}},
	{widget: "dock", state: stDrag, build: contractDock,
		beginIDSuffix: ":tab:g1:p2",
		note:          "P4, as slider, from the p2 tab",
		wants: [8]contractWant{
			wantKept, wantNA, wantKept, wantKept,
			wantKept, wantKept, wantCleared, wantCleared,
		}},

	{widget: "input", state: stHover, build: contractInput,
		note: "P2: readonly keeps pointer state; the modal layer " +
			"recomputes the target, so dialog clears",
		wants: [8]contractWant{
			wantCleared, wantKept, wantCleared, wantCleared,
			wantCleared, wantKept, wantKept, wantCleared,
		}},
	{widget: "button", state: stHover, build: contractButton,
		note: "P2, as input",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantKept, wantKept, wantCleared,
		}},
	{widget: "slider", state: stHover, build: contractSlider,
		note: "P2, as input",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantKept, wantKept, wantCleared,
		}},
	{widget: "listbox", state: stHover, build: contractListbox,
		note: "P2, as input",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantKept, wantKept, wantCleared,
		}},
	{widget: "select", state: stHover, build: contractSelect,
		note: "P2, as input",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantKept, wantKept, wantCleared,
		}},

	{widget: "select", state: stPopup, build: contractSelect,
		popupNS: nsSelect,
		note: "an open popup belongs to its field: any loss or " +
			"modality change closes it",
		wants: [8]contractWant{
			wantCleared, wantNA, wantCleared, wantCleared,
			wantCleared, wantCleared, wantCleared, wantCleared,
		}},
	{widget: "combobox", state: stPopup, build: contractCombobox,
		popupNS: nsCombobox,
		note:    "as select; hidden is NA: ComboboxCfg has no Invisible",
		wants: [8]contractWant{
			wantCleared, wantNA, wantNA, wantCleared,
			wantCleared, wantCleared, wantCleared, wantCleared,
		}},
}

func TestInteractionContract(t *testing.T) {
	for _, row := range contractRows {
		for tr := transDisabled; tr <= transDialog; tr++ {
			want := row.want(tr)
			if want == wantNA {
				continue
			}
			t.Run(row.widget+"/"+row.state.String()+
				"/"+tr.String(), func(t *testing.T) {
				ctl := &contractCtl{}
				w := contractWindow(ctl, row.build)
				id := contractTarget(t, w, row)
				clicksBefore := ctl.clicks
				px, py := contractBegin(t, w, row, id)
				contractTransition(t, w, ctl, tr)
				contractAssert(t, w, row, id, want)
				if row.guardClicks &&
					(tr == transRemoved || tr == transReplaced) {
					releaseAt(w, MouseLeft, px, py)
					if ctl.clicks != clicksBefore {
						t.Errorf("click fired on release after %s", tr)
					}
				}
			})
		}
	}
}
