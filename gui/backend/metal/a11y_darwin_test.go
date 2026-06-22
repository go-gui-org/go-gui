//go:build darwin && !ios

package metal

import "testing"

func TestCStringOrNil_Empty(t *testing.T) {
	var buf []cchar
	got := cStringOrNil("", &buf)
	if got != nil {
		t.Fatal("empty string must return nil")
	}
	if len(buf) != 0 {
		t.Fatalf("collector empty: got %d", len(buf))
	}
}

func TestCStringOrNil_NonEmpty(t *testing.T) {
	var buf []cchar
	got := cStringOrNil("hello", &buf)
	if got == nil {
		t.Fatal("non-empty string must return non-nil")
	}
	if len(buf) != 1 {
		t.Fatalf("collector len: got %d, want 1", len(buf))
	}
	if s := cGoString(got); s != "hello" {
		t.Fatalf("round-trip: got %q, want %q", s, "hello")
	}
	cFree(got)
}

func TestCStringOrNil_MultipleStrings(t *testing.T) {
	var buf []cchar
	s1 := cStringOrNil("first", &buf)
	s2 := cStringOrNil("second", &buf)
	s3 := cStringOrNil("", &buf)

	if s1 == nil || s2 == nil {
		t.Fatal("non-empty strings must return non-nil")
	}
	if s3 != nil {
		t.Fatal("empty string must return nil")
	}
	if len(buf) != 2 {
		t.Fatalf("collector len: got %d, want 2", len(buf))
	}
	if cGoString(s1) != "first" {
		t.Errorf("s1: got %q, want %q", cGoString(s1), "first")
	}
	if cGoString(s2) != "second" {
		t.Errorf("s2: got %q, want %q", cGoString(s2), "second")
	}
	for _, cs := range buf {
		cFree(cs)
	}
}

func TestA11ySyncBridge_ZeroCount(t *testing.T) {
	// Zero or negative count must return without panicking.
	a11ySyncBridge(nil, 0, 0, 0)
	a11ySyncBridge(nil, -1, 0, 0)
}

func TestSetA11yCallback(t *testing.T) {
	called := false
	setA11yCallback(func(action, index int) {
		called = true
	})
	goA11yAction(1, 2)
	if !called {
		t.Fatal("callback not invoked via goA11yAction")
	}
	// Nil callback must not panic.
	setA11yCallback(nil)
	goA11yAction(0, 0)
}
