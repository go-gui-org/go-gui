//go:build darwin && arm64 && !ios

package darwinffi

import (
	"math"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// fcallSink keeps the FCaller in the heap, as Call requires.
var fcallSink = new(FCaller)

// TestFCallerLayout keeps the offsets in fcall_arm64.s true.
func TestFCallerLayout(t *testing.T) {
	var c FCaller
	for _, tc := range []struct {
		name      string
		got, want uintptr
	}{
		{"fn", unsafe.Offsetof(c.fn), 0},
		{"X", unsafe.Offsetof(c.X), 8},
		{"D", unsafe.Offsetof(c.D), 72},
		{"R0", unsafe.Offsetof(c.R0), 136},
		{"RD", unsafe.Offsetof(c.RD), 144},
		{"S", unsafe.Offsetof(c.S), 176},
	} {
		if tc.got != tc.want {
			t.Errorf("offset of %s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

// TestFCallerDoubleArgsAndResult calls libm's pow, which takes two
// doubles and returns one: all three go through float registers.
func TestFCallerDoubleArgsAndResult(t *testing.T) {
	pow := Func(Open("/usr/lib/libSystem.B.dylib"), "pow")
	c := fcallSink
	c.D[0], c.D[1] = 2, 10
	c.Call(pow)
	if c.RD[0] != 1024 {
		t.Fatalf("pow(2, 10) = %v, want 1024", c.RD[0])
	}
	if n := testing.AllocsPerRun(100, func() { c.Call(pow) }); n != 0 {
		t.Errorf("Call allocates %v times, want 0", n)
	}
}

// TestFCallerMixedArgsAndStructResult sends a message that takes an
// NSPoint and an object, and returns an NSPoint: -[NSValue pointValue]
// on a value made from a point. The point goes in D0 and D1, the class
// and selector in X0 and X1.
func TestFCallerMixedArgsAndStructResult(t *testing.T) {
	Open(PathAppKit) // NSValue (NSPoint) lives in AppKit's additions
	pool := PoolPush()
	defer PoolPop(pool)

	c := fcallSink
	c.D[0], c.D[1] = 3.5, -7.25
	c.Send(uintptr(Class("NSValue")), uintptr(Sel("valueWithPoint:")))
	v := c.R0
	if v == 0 {
		t.Fatal("valueWithPoint: returned nil")
	}
	c.D[0], c.D[1] = math.NaN(), math.NaN() // the result must replace these
	c.Send(v, uintptr(Sel("pointValue")))
	if c.RD[0] != 3.5 || c.RD[1] != -7.25 {
		t.Fatalf("pointValue = (%v, %v), want (3.5, -7.25)", c.RD[0], c.RD[1])
	}

	// The same answer as objc.Send, which goes through purego's ABI code.
	type point struct{ X, Y float64 }
	if p := objc.Send[point](objc.ID(v), Sel("pointValue")); p.X != 3.5 || p.Y != -7.25 {
		t.Fatalf("objc.Send pointValue = %v", p)
	}
}

// TestFCallerRectResult checks a four-double struct result (an HFA in
// D0-D3): -[NSValue rectValue].
func TestFCallerRectResult(t *testing.T) {
	Open(PathAppKit)
	pool := PoolPush()
	defer PoolPop(pool)

	c := fcallSink
	c.D[0], c.D[1], c.D[2], c.D[3] = 1, 2, 30, 40
	c.Send(uintptr(Class("NSValue")), uintptr(Sel("valueWithRect:")))
	v := c.R0
	c.D = [8]float64{}
	c.Send(v, uintptr(Sel("rectValue")))
	if c.RD != [4]float64{1, 2, 30, 40} {
		t.Fatalf("rectValue = %v, want [1 2 30 40]", c.RD)
	}
}

// TestFCallerStackArgs builds the event the backend posts to wake its
// poll. otherEventWithType:location:modifierFlags:timestamp:windowNumber:
// context:subtype:data1:data2: takes nine integer arguments with the
// class and selector, so data2 goes on the stack.
func TestFCallerStackArgs(t *testing.T) {
	Open(PathAppKit)
	pool := PoolPush()
	defer PoolPop(pool)

	const applicationDefined = 15 // NSEventTypeApplicationDefined
	c := fcallSink
	c.X = [8]uintptr{
		uintptr(Class("NSEvent")),
		uintptr(Sel("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:")),
		applicationDefined, 0, 0, 0, 3, 41,
	}
	c.D = [8]float64{}
	c.S = [2]uintptr{97, 0}
	c.Call(MsgSend())
	ev := objc.ID(c.R0)
	if ev == 0 {
		t.Fatal("otherEventWithType: returned nil")
	}
	if got := objc.Send[int](ev, Sel("data1")); got != 41 {
		t.Errorf("data1 = %d, want 41", got)
	}
	if got := objc.Send[int](ev, Sel("data2")); got != 97 {
		t.Errorf("data2 = %d, want 97 (the stack argument)", got)
	}
	if got := objc.Send[int16](ev, Sel("subtype")); got != 3 {
		t.Errorf("subtype = %d, want 3", got)
	}
	c.S = [2]uintptr{}
}
