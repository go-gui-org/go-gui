//go:build linux && !android && (amd64 || arm64)

package decor

import (
	"testing"
	"unsafe"
)

type fakeHandler struct {
	configured, closed, committed int
	panicOn                       string
}

func (f *fakeHandler) Configure(Configuration) {
	f.configured++
	if f.panicOn == "configure" {
		panic("boom")
	}
}
func (f *fakeHandler) Close()  { f.closed++ }
func (f *fakeHandler) Commit() { f.committed++ }

func TestCallbacksRouteByID(t *testing.T) {
	a, b := &fakeHandler{}, &fakeHandler{}
	frames[101], frames[102] = a, b
	defer func() { delete(frames, 101); delete(frames, 102) }()

	onConfigure(0, 0, 101)
	onClose(0, 102)
	onCommit(0, 101)
	onCommit(0, 999) // unknown frame: ignored
	if a.configured != 1 || a.committed != 1 || a.closed != 0 || b.closed != 1 || b.configured != 0 {
		t.Fatalf("a %+v, b %+v", a, b)
	}
}

func TestCallbackPanicHeldForRethrow(t *testing.T) {
	h := &fakeHandler{panicOn: "configure"}
	frames[103] = h
	defer delete(frames, 103)
	// The callback returns normally, as it must into C...
	onConfigure(0, 0, 103)
	// ...and the panic comes out of Rethrow.
	defer func() {
		if r := recover(); r != "boom" {
			t.Fatalf("Rethrow raised %v, want boom", r)
		}
		Rethrow() // cleared: no second panic
	}()
	Rethrow()
	t.Fatal("Rethrow did not panic")
}

func TestOnErrorMessage(t *testing.T) {
	var gotCode int
	var gotMsg string
	old := ErrorFunc
	ErrorFunc = func(code int, msg string) { gotCode, gotMsg = code, msg }
	defer func() { ErrorFunc = old }()
	msg := []byte("compositor incompatible\x00")
	onError(0, 1, uintptr(unsafe.Pointer(&msg[0])))
	if gotCode != 1 || gotMsg != "compositor incompatible" {
		t.Fatalf("got %d %q", gotCode, gotMsg)
	}
	onError(0, 2, 0)
	if gotMsg != "" {
		t.Fatalf("NULL message gave %q", gotMsg)
	}
}

func TestCString(t *testing.T) {
	for in, want := range map[string]string{"": "\x00", "abc": "abc\x00", "a\x00b": "a\x00"} {
		if got := string(cString(in)); got != want {
			t.Errorf("cString(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoad(t *testing.T) {
	if err := Load(); err != nil {
		t.Skip(err)
	}
	if ctxIface[0] == 0 || frameIface[0] == 0 || frameIface[3] == 0 {
		t.Fatal("callbacks not installed")
	}
	for i := 4; i < len(frameIface); i++ {
		if frameIface[i] != 0 {
			t.Fatalf("reserved slot %d is not NULL", i)
		}
	}
}
