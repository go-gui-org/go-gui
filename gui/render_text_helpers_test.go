package gui

import (
	"strings"
	"testing"
)

func TestPasswordMaskKeepNewlinesBasic(t *testing.T) {
	got := passwordMaskKeepNewlines("abc")
	if got != "•••" {
		t.Errorf("got %q, want %q", got, "•••")
	}
}

func TestPasswordMaskKeepNewlinesEmpty(t *testing.T) {
	got := passwordMaskKeepNewlines("")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestPasswordMaskKeepNewlinesPreservesNewlines(t *testing.T) {
	got := passwordMaskKeepNewlines("ab\ncd\n")
	if got != "••\n••\n" {
		t.Errorf("got %q, want %q", got, "••\\n••\\n")
	}
}

func TestPasswordMaskKeepNewlinesUnicode(t *testing.T) {
	got := passwordMaskKeepNewlines("🔑key")
	if got != "••••" {
		t.Errorf("got %q, want %q", got, "••••")
	}
}

// passwordMaskSlice masks a byte-range substring using a
// pre-computed mask string. Operates on rune indices derived
// from byte offsets.
func passwordMaskSlice(mask, text string, startByte, endByte int) string {
	if len(mask) == 0 || endByte <= startByte {
		return ""
	}
	start := min(max(byteToRuneIndex(text, startByte), 0), len(mask))
	end := min(max(byteToRuneIndex(text, endByte), start), len(mask))
	return mask[start:end]
}

// hashCombineU64 combines two uint64 values using FNV-style
// multiplication.
func hashCombineU64(seed, value uint64) uint64 {
	return (seed ^ value) * 1099511628211
}

func TestPasswordMaskSliceBasic(t *testing.T) {
	mask := "***"
	text := "abc"
	got := passwordMaskSlice(mask, text, 0, 2)
	if got != "**" {
		t.Errorf("got %q, want %q", got, "**")
	}
}

func TestPasswordMaskSliceEmptyMask(t *testing.T) {
	got := passwordMaskSlice("", "abc", 0, 2)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestPasswordMaskSliceInvertedRange(t *testing.T) {
	got := passwordMaskSlice("***", "abc", 2, 1)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestHashCombineU64Deterministic(t *testing.T) {
	a := hashCombineU64(42, 100)
	b := hashCombineU64(42, 100)
	if a != b {
		t.Error("same inputs should produce same output")
	}
}

func TestHashCombineU64DifferentInputs(t *testing.T) {
	a := hashCombineU64(42, 100)
	b := hashCombineU64(42, 101)
	if a == b {
		t.Error("different inputs should produce different outputs")
	}
}

func TestHashCombineU64SeedMatters(t *testing.T) {
	a := hashCombineU64(1, 100)
	b := hashCombineU64(2, 100)
	if a == b {
		t.Error("different seeds should produce different outputs")
	}
}

// TestPasswordMaskKeepNewlinesLong crosses the 64-rune stack-buffer
// boundary: the heap path must produce byte-identical output to the
// stack path, newlines included.
func TestPasswordMaskKeepNewlinesLong(t *testing.T) {
	in := strings.Repeat("ab\n", 40) // 120 runes, 40 newlines
	got := passwordMaskKeepNewlines(in)
	want := strings.Repeat("••\n", 40)
	if got != want {
		t.Errorf("long mask = %q, want %q", got, want)
	}
	// The stack path and the heap path must agree on a shared prefix.
	short := strings.Repeat("ab\n", 20) // 60 runes
	if passwordMaskKeepNewlines(short) != strings.Repeat("••\n", 20) {
		t.Error("stack path disagrees with the heap path")
	}
}

func TestPasswordMaskKeepNewlinesNoAlloc(t *testing.T) {
	in := "secret\npassword"
	if n := testing.AllocsPerRun(100, func() {
		sinkMask = passwordMaskKeepNewlines(in)
	}); n > 1 {
		t.Errorf("allocs = %v, want <= 1 (the result string only)", n)
	}
}

var sinkMask string
