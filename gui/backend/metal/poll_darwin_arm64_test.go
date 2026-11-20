//go:build darwin && cgo && !ios

package metal

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// testEvents makes one synthetic NSEvent of each kind that storeEvent
// reads values from. None has a window, so the y flip uses a height of 0.
func testEvents(t *testing.T) (mouseDown, key, scroll objc.ID) {
	t.Helper()
	loadObjc()
	mouseDown = objs.nsEvent.Send(darwinffi.Sel("mouseEventWithType:location:modifierFlags:"+
		"timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
		uint(nsEventLeftMouseDown), nsPoint{10, 20}, uint(nsEventModifierFlagCommand),
		float64(0), 0, objc.ID(0), 0, 2, float32(1))
	// Not keyEventWithType:...isARepeat:keyCode:. Its last two of ten
	// integer arguments go on the stack, where Apple's arm64 ABI packs
	// them by size (the BOOL at byte 0, the key code at byte 2), and
	// objc.Send puts each in an 8-byte slot: AppKit then reads key code 0.
	cg := darwinffi.Open("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
	cgKey := msgCall(darwinffi.Func(cg, "CGEventCreateKeyboardEvent"), 0, 0x0C, 1)
	if cgKey == 0 {
		t.Fatal("CGEventCreateKeyboardEvent returned NULL")
	}
	t.Cleanup(func() { darwinffi.CFRelease(cgKey) })
	const kCGKeyboardEventAutorepeat = 8
	msgCall(darwinffi.Func(cg, "CGEventSetIntegerValueField"), cgKey, kCGKeyboardEventAutorepeat, 1)
	key = objs.nsEvent.Send(darwinffi.Sel("eventWithCGEvent:"), objc.ID(cgKey))

	// CGEventCreateScrollWheelEvent2 is the form with a fixed argument
	// list. The variadic one cannot be called through purego on arm64,
	// where variadic arguments go on the stack. Units 0 is pixels, which
	// AppKit reports as precise deltas.
	create := darwinffi.Func(cg, "CGEventCreateScrollWheelEvent2")
	// int32 arguments: wheel 1 (y) is -30, wheel 2 (x) is 4. The
	// callee reads only the low 32 bits of each register.
	wheelY := int32(-30)
	cgEvent := msgCall(create, 0, 0, 2, uintptr(uint32(wheelY)), 4, 0)
	if cgEvent == 0 {
		t.Fatal("CGEventCreateScrollWheelEvent2 returned NULL")
	}
	t.Cleanup(func() { darwinffi.CFRelease(cgEvent) })
	scroll = objs.nsEvent.Send(darwinffi.Sel("eventWithCGEvent:"), objc.ID(cgEvent))
	if mouseDown == 0 || key == 0 || scroll == 0 {
		t.Fatal("could not make the synthetic events")
	}
	return mouseDown, key, scroll
}

// TestStoreEventReadsEvents checks the values storeEvent copies, through
// the float-return sends of msg_darwin_arm64.go.
func TestStoreEventReadsEvents(t *testing.T) {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	mouseDown, key, scroll := testEvents(t)
	ev := &pollState.ev

	storeEvent(mouseDown, 7)
	if ev.typ != metalEventMouseDown || ev.windowID != 7 {
		t.Fatalf("mouse-down: typ %d window %d", ev.typ, ev.windowID)
	}
	// No window, so the flipped y is 0 - 20.
	if ev.mouseX != 10 || ev.mouseY != -20 {
		t.Errorf("mouse-down at (%v, %v), want (10, -20)", ev.mouseX, ev.mouseY)
	}
	if ev.clickCount != 2 || ev.mouseButton != 0 {
		t.Errorf("mouse-down clicks %d button %d, want 2 and 0", ev.clickCount, ev.mouseButton)
	}
	if ev.modifiers&nsEventModifierFlagCommand == 0 {
		t.Errorf("mouse-down modifiers %#x, want Command", ev.modifiers)
	}
	if pollState.lastMouseDown != mouseDown {
		t.Error("mouse-down not kept for metalWindowStartDrag")
	}

	storeEvent(key, 0)
	if ev.typ != metalEventKeyDown || ev.keyCode != 0x0C || !ev.keyRepeat {
		t.Errorf("key-down: typ %d code %#x repeat %v", ev.typ, ev.keyCode, ev.keyRepeat)
	}

	storeEvent(scroll, 0)
	wantY := float32(objc.Send[float64](scroll, sels.scrollingDeltaY)) * preciseScrollScale
	if ev.typ != metalEventScrollWheel || !ev.scrollPrecise {
		t.Fatalf("scroll: typ %d precise %v", ev.typ, ev.scrollPrecise)
	}
	if ev.scrollY != wantY || wantY == 0 {
		t.Errorf("scroll y %v, want %v (non-zero)", ev.scrollY, wantY)
	}
}

// TestStoreEventDoesNotAllocate holds the event path to zero heap
// allocations, as the C code was. objc.Send would cost 4 to 9 per value.
func TestStoreEventDoesNotAllocate(t *testing.T) {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	mouseDown, key, scroll := testEvents(t)
	for name, ev := range map[string]objc.ID{"mouse-down": mouseDown, "key": key, "scroll": scroll} {
		if n := testing.AllocsPerRun(100, func() { storeEvent(ev, 1) }); n != 0 {
			t.Errorf("storeEvent(%s) allocates %v times, want 0", name, n)
		}
	}
}

// TestWakeUpDoesNotAllocate: the animation loop calls wakeUp once a frame
// from its own goroutine. The main queue is not served in this test, so
// the wakes stay queued and never run.
func TestWakeUpDoesNotAllocate(t *testing.T) {
	wakeUp() // load the symbols and callbacks first
	if n := testing.AllocsPerRun(100, wakeUp); n != 0 {
		t.Errorf("wakeUp allocates %v times, want 0", n)
	}
}
