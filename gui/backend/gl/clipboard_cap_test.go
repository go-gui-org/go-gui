package gl

import (
	"testing"
)

func TestAppendClipChunkUnderCap(t *testing.T) {
	out, ok := appendClipChunk([]byte("ab"), []byte("cd"), 8)
	if !ok || string(out) != "abcd" {
		t.Fatalf("got %q, %v; want %q, true", out, ok, "abcd")
	}
}

func TestAppendClipChunkExactCapPagesOn(t *testing.T) {
	out, ok := appendClipChunk([]byte("ab"), []byte("cd"), 4)
	if !ok || string(out) != "abcd" {
		t.Fatalf("got %q, %v; want %q, true", out, ok, "abcd")
	}
}

func TestAppendClipChunkTruncatesAndStops(t *testing.T) {
	out, ok := appendClipChunk([]byte("ab"), []byte("cdef"), 4)
	if ok || string(out) != "abcd" {
		t.Fatalf("got %q, %v; want %q, false", out, ok, "abcd")
	}
}

func TestAppendClipChunkFullBufferIgnoresPage(t *testing.T) {
	full := []byte("abcd")
	out, ok := appendClipChunk(full, []byte("e"), 4)
	if ok || string(out) != "abcd" {
		t.Fatalf("got %q, %v; want %q, false", out, ok, "abcd")
	}
}

func TestAppendClipChunkZeroLimit(t *testing.T) {
	out, ok := appendClipChunk(nil, []byte("x"), 0)
	if ok || len(out) != 0 {
		t.Fatalf("got %q, %v; want empty, false", out, ok)
	}
}
