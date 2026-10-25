//go:build darwin

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
)

// Run (a window opens):
//
//	CGO_ENABLED=0 go run ./spike/puregomac -frames 600 -style sys
//	CGO_ENABLED=0 go run ./spike/puregomac -selftest
//
// -selftest runs the frame loop once per send style, posts one synthetic
// key-down into the view, and prints allocations per frame for each.

// AppKit must run on the main OS thread.
func init() { runtime.LockOSThread() }

// sels caches every selector the frame and event paths send. RegisterName
// takes the runtime lock; the hot paths must not call it.
var sels struct {
	nextDrawable, texture, setTexture, setLoadAction, setStoreAction,
	setClearColor, commandBuffer, renderEncoder, endEncoding,
	presentDrawable, commit, setViewport, sendEvent, nextEvent,
	keyCode, modifierFlags, isARepeat, characters objc.SEL
}

func cacheSels() {
	sels.nextDrawable = sel("nextDrawable")
	sels.texture = sel("texture")
	sels.setTexture = sel("setTexture:")
	sels.setLoadAction = sel("setLoadAction:")
	sels.setStoreAction = sel("setStoreAction:")
	sels.setClearColor = sel("setClearColor:")
	sels.commandBuffer = sel("commandBuffer")
	sels.renderEncoder = sel("renderCommandEncoderWithDescriptor:")
	sels.endEncoding = sel("endEncoding")
	sels.presentDrawable = sel("presentDrawable:")
	sels.commit = sel("commit")
	sels.setViewport = sel("setViewport:")
	sels.sendEvent = sel("sendEvent:")
	sels.nextEvent = sel("nextEventMatchingMask:untilDate:inMode:dequeue:")
	sels.keyCode = sel("keyCode")
	sels.modifierFlags = sel("modifierFlags")
	sels.isARepeat = sel("isARepeat")
	sels.characters = sel("characters")
}

// MTLViewport is six doubles: not a homogeneous float aggregate (that
// stops at four members) and over 16 bytes. send48 sends it on both
// arm64 and amd64. MTLScissorRect (4 x NSUInteger, 32 bytes) is the same
// class of argument.
type MTLViewport struct{ X, Y, W, H, ZNear, ZFar float64 }

const (
	mtlPixelFormatBGRA8Unorm = 80
	mtlLoadActionClear       = 2
	mtlStoreActionStore      = 1
)

// app is the spike's state. One window, one layer, one queue.
type app struct {
	nsApp, window, view, layer id
	device, queue, rpd, color0 id
	distantPast, runLoopMode   id

	viewport MTLViewport // read by Metal through a pointer
	clear    MTLClearColor
	clearSet bool // false: color changed since the last setClearColor:

	quit    bool
	keyHits int
	lastKey gui.Event
}

var theApp app

// --- Classes registered at runtime (objc.RegisterClass, no cgo) ---------

func registerClasses() {
	nsView := objc.GetClass("NSView")
	_, err := objc.RegisterClass("SpikeMetalView", nsView, nil, nil, []objc.MethodDef{
		{Cmd: sel("acceptsFirstResponder"), Fn: func(id, objc.SEL) bool { return true }},
		{Cmd: sel("keyDown:"), Fn: viewKeyDown},
	})
	if err != nil {
		panic(err)
	}
	nsObject := objc.GetClass("NSObject")
	_, err = objc.RegisterClass("SpikeWindowDelegate", nsObject,
		[]*objc.Protocol{objc.GetProtocol("NSWindowDelegate")}, nil, []objc.MethodDef{
			{Cmd: sel("windowShouldClose:"), Fn: func(id, objc.SEL, id) bool {
				theApp.quit = true
				return false
			}},
		})
	if err != nil {
		panic(err)
	}
}

// viewKeyDown is the view's keyDown: IMP. It turns the NSEvent into a
// gui.Event the way gui/backend/metal/events.go does for
// METAL_EVENT_KEY_DOWN, and changes the clear color so a key press shows.
func viewKeyDown(_ id, _ objc.SEL, ev id) {
	e := gui.Event{
		Type:      gui.EventKeyDown,
		KeyCode:   mapMacKeyCode(uint16(sys0(ev, sels.keyCode))),
		Modifiers: mapMetalModifiers(uint32(sys0(ev, sels.modifierFlags))),
		KeyRepeat: sys0(ev, sels.isARepeat)&0xff != 0,
	}
	theApp.lastKey = e
	theApp.keyHits++
	theApp.clear.R, theApp.clear.G = theApp.clear.G, theApp.clear.R
	theApp.clearSet = false
}

// --- Setup (not on the hot path; typed sends are fine here) -------------

func (a *app) init() {
	nsApp := sys0(cls("NSApplication"), sel("sharedApplication"))
	sys1(nsApp, sel("setActivationPolicy:"), 0) // Regular
	sys0(nsApp, sel("finishLaunching"))
	a.nsApp = nsApp

	r := NSRect{Origin: NSPoint{200, 200}, Size: NSSize{640, 400}}
	win := msgInitWin(sys0(cls("NSWindow"), sel("alloc")),
		sel("initWithContentRect:styleMask:backing:defer:"), r, 15, 2, false)
	sys1(win, sel("setTitle:"), uintptr(nsString("go-gui #1160 spike: purego, CGO_ENABLED=0")))
	sys1(win, sel("setReleasedWhenClosed:"), 0)
	del := sys0(sys0(cls("SpikeWindowDelegate"), sel("alloc")), sel("init"))
	sys1(win, sel("setDelegate:"), uintptr(del))
	a.window = win

	view := msgInitRect(sys0(cls("SpikeMetalView"), sel("alloc")), sel("initWithFrame:"),
		NSRect{Size: r.Size})
	a.view = view

	a.device = id(callC0(createDeviceFn))
	if a.device == 0 {
		panic("spike: no Metal device")
	}
	a.queue = sys0(a.device, sel("newCommandQueue"))

	// A plain CAMetalLayer, no subclass: layer-hosting view.
	layer := sys0(cls("CAMetalLayer"), sel("layer"))
	sys0(layer, sel("retain"))
	sys1(layer, sel("setDevice:"), uintptr(a.device))
	sys1(layer, sel("setPixelFormat:"), mtlPixelFormatBGRA8Unorm)
	sys1(layer, sel("setFramebufferOnly:"), 1)
	scale := msgFloat(win, sel("backingScaleFactor"))
	msgSetF(layer, sel("setContentsScale:"), scale)
	msgSize(layer, sel("setDrawableSize:"), NSSize{r.Size.Width * scale, r.Size.Height * scale})
	sys1(view, sel("setLayer:"), uintptr(layer))
	sys1(view, sel("setWantsLayer:"), 1)
	a.layer = layer
	a.viewport = MTLViewport{W: r.Size.Width * scale, H: r.Size.Height * scale, ZFar: 1}

	sys1(win, sel("setContentView:"), uintptr(view))
	sys1(win, sel("makeFirstResponder:"), uintptr(view))
	sys1(win, sel("makeKeyAndOrderFront:"), 0)
	sys1(nsApp, sel("activateIgnoringOtherApps:"), 1)

	// One render pass descriptor, reused: each frame only swaps the texture.
	a.rpd = sys0(cls("MTLRenderPassDescriptor"), sel("renderPassDescriptor"))
	sys0(a.rpd, sel("retain"))
	a.color0 = sys1(sys0(a.rpd, sel("colorAttachments")), sel("objectAtIndexedSubscript:"), 0)
	sys1(a.color0, sels.setLoadAction, mtlLoadActionClear)
	sys1(a.color0, sels.setStoreAction, mtlStoreActionStore)
	a.clear = MTLClearColor{0.15, 0.35, 0.55, 1}

	a.distantPast = sys0(sys0(cls("NSDate"), sel("distantPast")), sel("retain"))
	// NSDefaultRunLoopMode is an exported NSString* variable.
	foundation := mustDlopen("/System/Library/Frameworks/Foundation.framework/Foundation")
	a.runLoopMode = *(*id)(cPtr(mustDlsym(foundation, "NSDefaultRunLoopMode")))
}

// --- Per-frame paths, one per send style --------------------------------

type style int

const (
	styleSend  style = iota // objc.ID.Send, variadic ...any
	styleTyped              // purego.RegisterFunc typed pointers
	styleSys                // purego.SyscallN, buffered arguments
)

var styleNames = [...]string{"ID.Send", "RegisterFunc", "SyscallN"}

// pollEvents drains the queue without blocking. Every argument is an
// integer or a pointer, so all three styles can send it.
func (a *app) pollEvents(st style) {
	// nextEventMatchingMask: returns autoreleased events; drain them here,
	// not in the next frame's pool, or they pile up between frames.
	pool := poolPush()
	defer poolPop(pool)
	for {
		var ev id
		switch st {
		case styleSend:
			ev = a.nsApp.Send(sels.nextEvent, ^uint64(0), a.distantPast, a.runLoopMode, true)
		case styleTyped:
			ev = msgNextEv(a.nsApp, sels.nextEvent, ^uint64(0), a.distantPast, a.runLoopMode, true)
		case styleSys:
			ev = sys4(a.nsApp, sels.nextEvent, ^uintptr(0), uintptr(a.distantPast), uintptr(a.runLoopMode), 1)
		}
		if ev == 0 {
			return
		}
		sendEv(st, a.nsApp, ev)
	}
}

func sendEv(st style, nsApp, ev id) {
	switch st {
	case styleSend:
		nsApp.Send(sels.sendEvent, ev)
	case styleTyped:
		msg1(nsApp, sels.sendEvent, uintptr(ev))
	case styleSys:
		sys1(nsApp, sels.sendEvent, uintptr(ev))
	}
}

// frame clears the layer's next drawable and presents it: the minimum a
// go-gui frame does before any draw call.
func (a *app) frame(st style) {
	pool := poolPush()
	defer poolPop(pool)

	switch st {
	case styleSend:
		d := a.layer.Send(sels.nextDrawable)
		if d == 0 {
			return
		}
		a.color0.Send(sels.setTexture, d.Send(sels.texture))
		if !a.clearSet {
			a.color0.Send(sels.setClearColor, a.clear)
			a.clearSet = true
		}
		cb := a.queue.Send(sels.commandBuffer)
		enc := cb.Send(sels.renderEncoder, a.rpd)
		enc.Send(sels.setViewport, a.viewport)
		enc.Send(sels.endEncoding)
		cb.Send(sels.presentDrawable, d)
		cb.Send(sels.commit)

	case styleTyped:
		d := msg0(a.layer, sels.nextDrawable)
		if d == 0 {
			return
		}
		msg1(a.color0, sels.setTexture, uintptr(msg0(d, sels.texture)))
		if !a.clearSet {
			msgClear(a.color0, sels.setClearColor, a.clear)
			a.clearSet = true
		}
		cb := msg0(a.queue, sels.commandBuffer)
		enc := msg1(cb, sels.renderEncoder, uintptr(a.rpd))
		msgViewport(enc, sels.setViewport, a.viewport)
		msg0(enc, sels.endEncoding)
		msg1(cb, sels.presentDrawable, uintptr(d))
		msg0(cb, sels.commit)

	case styleSys:
		d := sys0(a.layer, sels.nextDrawable)
		if d == 0 {
			return
		}
		sys1(a.color0, sels.setTexture, uintptr(sys0(d, sels.texture)))
		if !a.clearSet {
			// Four doubles go in d0..d3; SyscallN cannot place them. The
			// clear color changes with the theme, not per frame, so the
			// typed send runs only on a change.
			msgClear(a.color0, sels.setClearColor, a.clear)
			a.clearSet = true
		}
		cb := sys0(a.queue, sels.commandBuffer)
		enc := sys1(cb, sels.renderEncoder, uintptr(a.rpd))
		send48(enc, sels.setViewport, (*[6]uintptr)(unsafe.Pointer(&a.viewport)))
		sys0(enc, sels.endEncoding)
		sys1(cb, sels.presentDrawable, uintptr(d))
		sys0(cb, sels.commit)
	}
}

// --- Measurement --------------------------------------------------------

// heapAllocs reads MemStats.Mallocs. runtime/metrics was tried first and
// dropped: its /gc/heap/allocs:objects counts small objects when an mcache
// span is refilled, so a short run reads 0 or a whole span's worth.
// ReadMemStats flushes every mcache first, so its count is exact.
var memStats runtime.MemStats

func heapAllocs() uint64 {
	runtime.ReadMemStats(&memStats)
	return memStats.Mallocs
}

// run renders n frames in style st and returns heap allocations per frame,
// measured after a warm-up so one-time growth is not counted.
func (a *app) run(st style, n int) float64 {
	const warm = 30
	var start uint64
	measured := 0
	for i := 0; i < warm+n && !a.quit; i++ {
		if i == warm {
			start = heapAllocs()
		}
		a.pollEvents(st)
		a.frame(st)
		if i >= warm {
			measured++
		}
	}
	// The window can close before or during the measured frames.
	if measured == 0 {
		return 0
	}
	return float64(heapAllocs()-start) / float64(measured)
}

// postKey queues a synthetic key-down (S, Shift) for the window, the
// way a real key press arrives: through nextEvent and sendEvent:.
func (a *app) postKey() {
	pool := poolPush()
	winNum := int(sys0(a.window, sel("windowNumber")))
	s := nsString("S")
	ev := msgKeyEv(cls("NSEvent"),
		sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
		10 /* NSEventTypeKeyDown */, NSPoint{}, 1<<17 /* Shift */, 0, winNum, 0, s, s, false, 1 /* kVK_ANSI_S */)
	sys2(a.nsApp, sel("postEvent:atStart:"), uintptr(ev), 0)
	poolPop(pool)
}

// keyAllocs measures allocations for one key-down dispatched through
// sendEvent: into the Go IMP, with the typed and SyscallN sends inside it.
func (a *app) keyAllocs() (allocs uint64, ok bool) {
	a.postKey()
	before := a.keyHits
	start := heapAllocs()
	a.pollEvents(styleSys)
	allocs = heapAllocs() - start
	return allocs, a.keyHits == before+1
}

func main() {
	frames := flag.Int("frames", 300, "frames to render per style")
	styleFlag := flag.String("style", "sys", "send style: send, typed, sys")
	selftest := flag.Bool("selftest", false, "measure every style, post a key, exit")
	flag.Parse()

	load()
	cacheSels()
	registerClasses()
	theApp.init()
	a := &theApp

	if *selftest {
		fmt.Printf("go-gui #1160 spike  %s/%s  CGO_ENABLED=0  %s\n", runtime.GOOS, runtime.GOARCH, runtime.Version())
		for st := styleSend; st <= styleSys; st++ {
			a.clearSet = false
			fmt.Printf("  %-13s %6.2f allocs/frame (clear + viewport + present)\n", styleNames[st], a.run(st, *frames))
		}
		// Warm the key path once, then measure a second key-down.
		_, ok1 := a.keyAllocs()
		n, ok2 := a.keyAllocs()
		fmt.Printf("  key-down -> IMP -> gui.Event: delivered=%v allocs=%d\n", ok1 && ok2, n)
		// The key code is nonzero on purpose: kVK_ANSI_A is 0, the value a
		// dropped stack argument would also read as.
		fmt.Printf("  last event: Type=%d KeyCode=%d (KeyS=%d) Modifiers=%#x (ModShift=%#x)\n",
			a.lastKey.Type, a.lastKey.KeyCode, gui.KeyS, a.lastKey.Modifiers, gui.ModShift)
		if !ok1 || !ok2 || a.lastKey.KeyCode != gui.KeyS || a.lastKey.Modifiers != gui.ModShift {
			os.Exit(1)
		}
		return
	}

	st := styleSys
	switch *styleFlag {
	case "send":
		st = styleSend
	case "typed":
		st = styleTyped
	}
	for !a.quit {
		per := a.run(st, *frames)
		fmt.Printf("%s: %.2f allocs/frame, last key %+v\n", styleNames[st], per, a.lastKey)
	}
}
