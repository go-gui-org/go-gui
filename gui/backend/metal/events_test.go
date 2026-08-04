//go:build darwin && !ios

package metal

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// ─── mapMetalModifiers ────────────────────────────────────────

func TestMapMetalModifiers_Zero(t *testing.T) {
	if m := mapMetalModifiers(0); m != 0 {
		t.Fatalf("zero flags → 0, got %v", m)
	}
}

func TestMapMetalModifiers_Single(t *testing.T) {
	tests := []struct {
		flag uint32
		want gui.Modifier
	}{
		{1 << 17, gui.ModShift}, // NSEventModifierFlagShift
		{1 << 18, gui.ModCtrl},  // NSEventModifierFlagControl
		{1 << 19, gui.ModAlt},   // NSEventModifierFlagOption
		{1 << 20, gui.ModSuper}, // NSEventModifierFlagCommand
	}
	for _, tt := range tests {
		got := mapMetalModifiers(tt.flag)
		if got != tt.want {
			t.Errorf("flag=0x%x: got %v, want %v",
				tt.flag, got, tt.want)
		}
	}
}

func TestMapMetalModifiers_Combos(t *testing.T) {
	// Cmd+Shift (common pasteboard shortcut)
	got := mapMetalModifiers((1 << 20) | (1 << 17))
	want := gui.ModSuper | gui.ModShift
	if got != want {
		t.Fatalf("Cmd+Shift: got %v, want %v", got, want)
	}

	// Ctrl+Alt
	got = mapMetalModifiers((1 << 18) | (1 << 19))
	want = gui.ModCtrl | gui.ModAlt
	if got != want {
		t.Fatalf("Ctrl+Alt: got %v, want %v", got, want)
	}
}

func TestMapMetalModifiers_CapsLockIgnored(t *testing.T) {
	// CapsLock alone → no modifiers.
	if got := mapMetalModifiers(1 << 16); got != 0 {
		t.Fatalf("CapsLock alone: got %v, want 0", got)
	}
	// CapsLock + Shift → Shift only.
	got := mapMetalModifiers((1 << 16) | (1 << 17))
	if got != gui.ModShift {
		t.Fatalf("CapsLock+Shift: got %v, want ModShift", got)
	}
}

// ─── mapMetalMouseButton ──────────────────────────────────────

func TestMapMetalMouseButton(t *testing.T) {
	tests := []struct {
		btn  int
		want gui.MouseButton
	}{
		{0, gui.MouseLeft},
		{1, gui.MouseRight},
		{2, gui.MouseMiddle},
		{3, gui.MouseLeft},  // unknown → default left
		{99, gui.MouseLeft}, // unknown → default left
	}
	for _, tt := range tests {
		got := mapMetalMouseButton(tt.btn)
		if got != tt.want {
			t.Errorf("btn %d: got %v, want %v",
				tt.btn, got, tt.want)
		}
	}
}

// ─── mapMacKeyCode ────────────────────────────────────────────

func TestMapMacKeyCode_KnownKeys(t *testing.T) {
	tests := []struct {
		kc   uint16
		want gui.KeyCode
	}{
		{0x00, gui.KeyA},         // kVK_ANSI_A
		{0x0C, gui.KeyQ},         // kVK_ANSI_Q
		{0x24, gui.KeyEnter},     // kVK_Return
		{0x31, gui.KeySpace},     // kVK_Space
		{0x35, gui.KeyEscape},    // kVK_Escape
		{0x7E, gui.KeyUp},        // kVK_UpArrow
		{0x37, gui.KeyLeftSuper}, // kVK_Command
		{0x30, gui.KeyTab},       // kVK_Tab
		{0x33, gui.KeyBackspace}, // kVK_Delete (backspace on Mac)
		{0x75, gui.KeyDelete},    // kVK_ForwardDelete
		{0x7D, gui.KeyDown},      // kVK_DownArrow
		{0x7B, gui.KeyLeft},      // kVK_LeftArrow
		{0x7C, gui.KeyRight},     // kVK_RightArrow
	}
	for _, tt := range tests {
		got := mapMacKeyCode(tt.kc)
		if got != tt.want {
			t.Errorf("keyCode 0x%x: got %v, want %v",
				tt.kc, got, tt.want)
		}
	}
}

func TestMapMacKeyCode_OutOfBounds(t *testing.T) {
	// Table is 128 entries; anything >= 128 returns KeyInvalid.
	if got := mapMacKeyCode(128); got != gui.KeyInvalid {
		t.Fatalf("kc=128: got %v, want KeyInvalid", got)
	}
	if got := mapMacKeyCode(65535); got != gui.KeyInvalid {
		t.Fatalf("kc=65535: got %v, want KeyInvalid", got)
	}
}

func TestMapMacKeyCode_AllValidIndices(t *testing.T) {
	// Every index in range must return a valid key code or
	// KeyInvalid — must not panic.
	for kc := range 128 {
		_ = mapMacKeyCode(uint16(kc))
	}
}

// ─── keyCodeModFlag ───────────────────────────────────────────

func TestKeyCodeModFlag_ModifierKeys(t *testing.T) {
	tests := []struct {
		kc   gui.KeyCode
		want gui.Modifier
	}{
		{gui.KeyLeftShift, gui.ModShift},
		{gui.KeyRightShift, gui.ModShift},
		{gui.KeyLeftControl, gui.ModCtrl},
		{gui.KeyRightControl, gui.ModCtrl},
		{gui.KeyLeftAlt, gui.ModAlt},
		{gui.KeyRightAlt, gui.ModAlt},
		{gui.KeyLeftSuper, gui.ModSuper},
		{gui.KeyRightSuper, gui.ModSuper},
	}
	for _, tt := range tests {
		got := keyCodeModFlag(tt.kc)
		if got != tt.want {
			t.Errorf("%v: got %v, want %v", tt.kc, got, tt.want)
		}
	}
}

func TestKeyCodeModFlag_NonModifierKeys(t *testing.T) {
	nonModifiers := []gui.KeyCode{
		gui.KeyA, gui.KeySpace, gui.KeyEnter,
		gui.KeyEscape, gui.KeyUp, gui.KeyInvalid,
	}
	for _, kc := range nonModifiers {
		if got := keyCodeModFlag(kc); got != 0 {
			t.Errorf("%v: got %v, want 0", kc, got)
		}
	}
}

func TestKeyCodeModFlag_CapsLock(t *testing.T) {
	// CapsLock has no corresponding modifier flag.
	if got := keyCodeModFlag(gui.KeyCapsLock); got != 0 {
		t.Fatalf("CapsLock: got %v, want 0", got)
	}
}

// ─── cursorSelector ───────────────────────────────────────────

func TestCursorSelector_Known(t *testing.T) {
	// Every defined cursor constant must map to non-empty string.
	cursors := []gui.MouseCursor{
		gui.CursorDefault, gui.CursorArrow, gui.CursorIBeam,
		gui.CursorCrosshair, gui.CursorPointingHand,
		gui.CursorResizeEW, gui.CursorResizeNS,
		gui.CursorResizeNWSE, gui.CursorResizeNESW,
		gui.CursorResizeAll, gui.CursorNotAllowed,
	}
	for _, mc := range cursors {
		if got := cursorSelector(mc); got == "" {
			t.Errorf("cursor %v: empty selector", mc)
		}
	}
}

func TestCursorSelector_Unknown(t *testing.T) {
	if got := cursorSelector(gui.MouseCursor(255)); got != "" {
		t.Fatalf("unknown cursor: got %q, want empty", got)
	}
}

// The cursorCStrings cache hand-lists the cursors it allocates. Guard
// against drift from cursorSelector: every cursor with a selector must
// have a cached C string whose value matches, or updateCursor silently
// no-ops for that cursor.
func TestCursorCStrings_MatchSelector(t *testing.T) {
	cursors := []gui.MouseCursor{
		gui.CursorDefault, gui.CursorArrow, gui.CursorIBeam,
		gui.CursorCrosshair, gui.CursorPointingHand,
		gui.CursorResizeEW, gui.CursorResizeNS,
		gui.CursorResizeNWSE, gui.CursorResizeNESW,
		gui.CursorResizeAll, gui.CursorNotAllowed,
	}
	for _, mc := range cursors {
		sel := cursorSelector(mc)
		if sel == "" {
			continue
		}
		cstr, ok := cursorCStrings[mc]
		if !ok || cstr == nil {
			t.Errorf("cursor %v: selector %q but no cached C string",
				mc, sel)
			continue
		}
		if got := cGoString(cstr); got != sel {
			t.Errorf("cursor %v: cached %q, want %q", mc, got, sel)
		}
	}
}

// An unmapped cursor must return before dereferencing ws.window, so
// updateCursor is safe on a zero-value windowState.
func TestUpdateCursor_UnknownCursorNoOp(t *testing.T) {
	ws := &windowState{}
	ws.updateCursor(gui.MouseCursor(255)) // must not panic or touch C
}

// ─── mapMetalEvent integration (C state injection) ────────────

func TestMapMetalEvent_Quit_ReturnsContFalse(t *testing.T) {
	testInjectQuitEvent()

	_, cont := mapMetalEvent()
	if cont {
		t.Fatalf("METAL_EVENT_QUIT: got cont=true, want false")
	}
}

func TestMapMetalEvent_CmdQ_ReturnsContFalse(t *testing.T) {
	// kVK_ANSI_Q = 0x0C, NSEventModifierFlagCommand = 1<<20
	const (
		kvkQ   = uint16(0x0C)
		modCmd = uint32(1 << 20)
	)
	testInjectKeyDown(kvkQ, modCmd)

	_, cont := mapMetalEvent()
	if cont {
		t.Fatalf("Cmd+Q: got cont=true, want false")
	}
}

func TestMapMetalEvent_PlainKeyDown_ReturnsKeyDown(t *testing.T) {
	// kVK_ANSI_A = 0x00, no modifiers
	testInjectKeyDown(0x00, 0)

	evt, cont := mapMetalEvent()
	if !cont {
		t.Fatal("plain key should continue")
	}
	if evt.Type != gui.EventKeyDown {
		t.Fatalf("plain key: got event type %v, want EventKeyDown",
			evt.Type)
	}
	if evt.KeyCode != gui.KeyA {
		t.Fatalf("plain key: got keycode %v, want KeyA", evt.KeyCode)
	}
}

// ─── IME text-event queue ──────────────────────────────────────

// A single sendEvent: can fire insertText: then setMarkedText:. With
// the old single event slot the commit was overwritten and lost —
// Japanese input produced zero characters (issue #149). Both events
// must survive, in order.
func TestIMEQueue_CommitThenComposition_BothDelivered(t *testing.T) {
	testResetIMEQueue()
	defer testResetIMEQueue()

	testPushIMECommit("日本語")
	testPushIMEComposition("ん", 1, 0)

	if !testPollEvent() {
		t.Fatal("first poll: got no event, want the commit")
	}
	evt, cont := mapMetalEvent()
	if !cont {
		t.Fatal("commit should continue the event loop")
	}
	if evt.Type != gui.EventChar {
		t.Fatalf("first event: got %v, want EventChar", evt.Type)
	}
	if evt.IMEText != "日本語" {
		t.Fatalf("commit text: got %q, want %q", evt.IMEText, "日本語")
	}

	if !testPollEvent() {
		t.Fatal("second poll: got no event, want the composition")
	}
	evt, cont = mapMetalEvent()
	if !cont {
		t.Fatal("composition should continue the event loop")
	}
	if evt.Type != gui.EventIMEComposition {
		t.Fatalf("second event: got %v, want EventIMEComposition", evt.Type)
	}
	if evt.IMEText != "ん" {
		t.Fatalf("composition text: got %q, want %q", evt.IMEText, "ん")
	}
	if evt.IMEStart != 1 {
		t.Fatalf("composition start: got %d, want 1", evt.IMEStart)
	}
}

// A drained queue must not hand the same event back on the next poll.
func TestIMEQueue_DrainedQueueDoesNotRedeliver(t *testing.T) {
	testResetIMEQueue()
	defer testResetIMEQueue()

	testPushIMECommit("a")
	if !testPopTextEvent() {
		t.Fatal("pop: got nothing, want the queued commit")
	}
	if d := testIMEQueueDepth(); d != 0 {
		t.Fatalf("queue depth after drain: got %d, want 0", d)
	}
	if testPopTextEvent() {
		t.Fatal("pop on empty queue: got an event, want none")
	}
}

// Empty marked text is the IME's composition-end signal. It must reach
// Go as an empty composition so the preedit can be cleared.
func TestIMEQueue_CompositionEnd_DeliversEmptyComposition(t *testing.T) {
	testResetIMEQueue()
	defer testResetIMEQueue()

	testPushIMEComposition("", 0, 0)

	if !testPollEvent() {
		t.Fatal("poll: got no event, want the composition end")
	}
	evt, cont := mapMetalEvent()
	if !cont {
		t.Fatal("composition end should continue the event loop")
	}
	if evt.Type != gui.EventIMEComposition {
		t.Fatalf("got %v, want EventIMEComposition", evt.Type)
	}
	if evt.IMEText != "" {
		t.Fatalf("composition end text: got %q, want empty", evt.IMEText)
	}
}

// A queued commit may be delivered a poll or more after the key event
// that produced it, by which point the shared modifier global has moved
// on. The entry must carry its own snapshot.
func TestIMEQueue_ModifiersSurviveDeferredPop(t *testing.T) {
	testResetIMEQueue()
	defer testResetIMEQueue()

	const modCmd = uint32(1 << 20) // NSEventModifierFlagCommand

	// Key-down sets the modifier global; the commit is queued with it.
	testInjectKeyDown(0x00, modCmd)
	testPushIMECommit("a")

	// A later event clobbers the global before the queue is drained.
	testInjectKeyDown(0x00, 0)

	if !testPopTextEvent() {
		t.Fatal("pop: got nothing, want the queued commit")
	}
	evt, _ := mapMetalEvent()
	if evt.Type != gui.EventChar {
		t.Fatalf("got %v, want EventChar", evt.Type)
	}
	if !evt.Modifiers.Has(gui.ModSuper) {
		t.Fatalf("modifiers: got %v, want ModSuper from push time",
			evt.Modifiers)
	}
}

// Overflow must stay bounded rather than grow or corrupt the ring.
func TestIMEQueue_OverflowIsBounded(t *testing.T) {
	testResetIMEQueue()
	defer testResetIMEQueue()

	const queueCap = 32 // TEXT_EVENT_CAP
	for range queueCap + 4 {
		testPushIMECommit("x")
	}
	if d := testIMEQueueDepth(); d != queueCap {
		t.Fatalf("queue depth after overflow: got %d, want %d", d, queueCap)
	}

	// The newest entries are the ones kept.
	for i := range queueCap {
		if !testPopTextEvent() {
			t.Fatalf("pop %d: got nothing, want an entry", i)
		}
	}
	if d := testIMEQueueDepth(); d != 0 {
		t.Fatalf("queue depth after full drain: got %d, want 0", d)
	}
}

// ─── Cursor bounds guard ────────────────────────────────────────

func TestCursorBoundsCheck(t *testing.T) {
	tests := []struct {
		name         string
		x, y, w, h   float32
		wantRejected bool
	}{
		{"Inside", 50, 50, 100, 100, false},
		{"NegX", -1, 50, 100, 100, true},
		{"NegY", 50, -1, 100, 100, true},
		{"ExactEdgeX", 100, 50, 100, 100, true},
		{"ExactEdgeY", 50, 100, 100, 100, true},
		{"NaN_X", float32(math.NaN()), 50, 100, 100, true},
		{"NaN_Y", 50, float32(math.NaN()), 100, 100, true},
		{"InfX", float32(math.Inf(1)), 50, 100, 100, true},
		{"InfY", 50, float32(math.Inf(-1)), 100, 100, true},
		{"Zero_Zero", 0, 0, 100, 100, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := testCursorBoundsCheck(tc.x, tc.y, tc.w, tc.h)
			if got != tc.wantRejected {
				t.Errorf("got rejected=%v, want rejected=%v", got, tc.wantRejected)
			}
		})
	}
}
