package gui

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"
	"unsafe"
)

// Tests for the delta undo history (#1187): each step is the bytes an
// edit replaced at an offset, not a copy of the whole text.

// TestEditUndoRedoRandom drives random edits — typing runs, deletes,
// selection replaces, pastes, caret moves and whole-text sets — through
// the edit core, then undoes every step and redoes every step. Undo must
// walk back through texts the field actually held, newest first, and end
// at the start; redo must walk forward again and end at the last text.
func TestEditUndoRedoRandom(t *testing.T) {
	alphabet := []string{"a", "b", " ", "\n", "é", "日", "\U0001F600", "é"}
	for seed := int64(1); seed <= 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		start := "héllo wörld\nsecond 日本 line"
		text, is := start, inputState{CursorPos: len(start)}
		history := []string{text}
		boundaries := func(s string) []int {
			out := []int{0}
			for i := 0; i < len(s); {
				i = nextGraphemeBoundary(s, i)
				out = append(out, i)
			}
			return out
		}
		pick := func(s string) int {
			b := boundaries(s)
			return b[rng.Intn(len(b))]
		}
		// Fewer edits than the stack holds, so nothing is evicted and
		// undo must reach the start.
		for step := range 30 {
			var next string
			var ok bool
			switch rng.Intn(7) {
			case 0, 1: // typing: a single rune at the caret
				next, is, ok = editInsert(text, alphabet[rng.Intn(5)], is)
			case 2: // backspace or delete
				next, is, ok = editDelete(text, is, rng.Intn(2) == 0)
			case 3: // select a range and replace it
				a, b := pick(text), pick(text)
				is.selectBeg, is.selectEnd = uint32(a), uint32(b)
				is.CursorPos = b
				next, is, ok = editInsert(text, alphabet[rng.Intn(len(alphabet))], is)
			case 4: // paste
				next, is, ok = editInsert(text, "pasted "+alphabet[rng.Intn(len(alphabet))], is)
			case 5: // caret move: breaks the typing run, changes nothing
				is.CursorPos = pick(text)
				is.selectBeg, is.selectEnd = 0, 0
				is.lastEditOp = inputOpNone
				continue
			case 6: // a whole-text set, as PreTextChange's adjusted text does
				adjusted := strings.ToUpper(text)
				is = editSetTextCursorAtEnd(text, adjusted, is)
				next, ok = adjusted, adjusted != text
			}
			if !ok {
				continue
			}
			text = next
			if !utf8.ValidString(text) {
				t.Fatalf("seed %d step %d: edit produced invalid UTF-8 %q", seed, step, text)
			}
			if is.CursorPos < 0 || is.CursorPos > len(text) {
				t.Fatalf("seed %d step %d: caret %d outside %q", seed, step, is.CursorPos, text)
			}
			history = append(history, text)
		}
		final := text

		// Undo: every result is a text from the history, older each time.
		last := len(history) - 1
		for {
			prev, next, ok := editUndo(text, is)
			if !ok {
				break
			}
			at := -1
			for k := last - 1; k >= 0; k-- {
				if history[k] == prev {
					at = k
					break
				}
			}
			if at < 0 {
				t.Fatalf("seed %d: undo gave %q, not an earlier text", seed, prev)
			}
			if next.CursorPos < 0 || next.CursorPos > len(prev) {
				t.Fatalf("seed %d: undo caret %d outside %q", seed, next.CursorPos, prev)
			}
			text, is, last = prev, next, at
		}
		if text != start {
			t.Fatalf("seed %d: undo ended at %q, want the start %q", seed, text, start)
		}
		// Redo: back to the last text.
		for {
			nt, next, ok := editRedo(text, is)
			if !ok {
				break
			}
			text, is = nt, next
		}
		if text != final {
			t.Fatalf("seed %d: redo ended at %q, want %q", seed, text, final)
		}
	}
}

// A typing run is one step whatever its shape: inserting into the
// middle and continuing past the end, each keystroke coalesced.
func TestEditUndoCoalescedRunRestoresExactly(t *testing.T) {
	text, is := "hello world", inputState{CursorPos: 5}
	for _, r := range ", dear" {
		text, is, _ = editInsert(text, string(r), is)
	}
	if text != "hello, dear world" || is.Undo.Len() != 1 {
		t.Fatalf("typing run = %q depth %d, want one step", text, is.Undo.Len())
	}
	// Backspaces form a run of their own.
	for range 5 {
		text, is, _ = editDelete(text, is, false)
	}
	if text != "hello, world" || is.Undo.Len() != 2 {
		t.Fatalf("delete run = %q depth %d, want two steps", text, is.Undo.Len())
	}
	text, is, _ = editUndo(text, is)
	if text != "hello, dear world" || is.CursorPos != 11 {
		t.Fatalf("undo delete run = %q caret %d, want the text before it, caret 11",
			text, is.CursorPos)
	}
	text, is, _ = editUndo(text, is)
	if text != "hello world" || is.CursorPos != 5 {
		t.Fatalf("undo typing run = %q caret %d, want %q caret 5",
			text, is.CursorPos, "hello world")
	}
}

// An app that replaces the field's text under its history leaves steps
// that no longer apply. Undo drops the history instead of splicing stale
// bytes into text it has never seen.
func TestEditUndoAfterAppReplacedText(t *testing.T) {
	text, is := "abc", inputState{CursorPos: 3}
	text, is, _ = editInsert(text, "d", is)
	if text != "abcd" {
		t.Fatalf("insert = %q", text)
	}
	// The app sets an unrelated text; the field's history is stale.
	replaced := "something else"
	got, next, ok := editUndo(replaced, is)
	if ok || got != replaced {
		t.Fatalf("undo over replaced text = (%q, %v), want (%q, false)", got, ok, replaced)
	}
	if !next.Undo.isEmpty() {
		t.Fatal("a stale history must be dropped")
	}
	if _, _, ok = editRedo(replaced, next); ok {
		t.Fatal("redo after a dropped history reported ok")
	}

	// A replacement that still holds the step's bytes at its offset is
	// not detectable and does not need to be: the step applies.
	text, is = "abc", inputState{CursorPos: 3}
	_, is, _ = editInsert(text, "d", is)
	if got, _, ok = editUndo("xbcd", is); !ok || got != "xbc" {
		t.Fatalf("undo over a compatible text = (%q, %v), want (\"xbc\", true)", got, ok)
	}
}

// The history of a large field costs what was edited: fifty single-rune
// steps in a 1 MB text keep a few bytes each, not fifty copies of it.
func TestEditUndoHistoryDoesNotCopyText(t *testing.T) {
	text := strings.Repeat("0123456789abcdef", 1<<16) // 1 MiB
	is := inputState{}
	for i := range undoMaxSize {
		// A caret move between edits breaks the run, so each is a step.
		is.CursorPos = i * 1000
		is.lastEditOp = inputOpNone
		text, is, _ = editInsert(text, "x", is)
	}
	if is.Undo.Len() != undoMaxSize {
		t.Fatalf("depth = %d, want %d", is.Undo.Len(), undoMaxSize)
	}
	total := 0
	for is.Undo.Len() > 0 {
		m, _ := is.Undo.Pop()
		total += len(m.removed) + len(m.inserted)
	}
	if total > undoMaxSize*4 {
		t.Fatalf("history holds %d bytes for %d one-rune edits", total, undoMaxSize)
	}
}

// A small span kept by a memento is copied, so it does not pin the text
// it was cut from; a span that is most of the text is kept as is.
func TestMementoSpanCopiesSmallSpans(t *testing.T) {
	text := strings.Repeat("a", 1000)
	small := mementoSpan(text[10:20], len(text))
	if small != text[10:20] {
		t.Fatal("a small span must keep its content")
	}
	if unsafe.StringData(small) == unsafe.StringData(text[10:]) {
		t.Fatal("a small span shares the text's memory and pins all of it")
	}
	large := mementoSpan(text[:900], len(text))
	if large != text[:900] {
		t.Fatal("a large span must keep its content")
	}
	if unsafe.StringData(large) != unsafe.StringData(text) {
		t.Fatal("a span that is most of the text is kept, not copied")
	}
	if got := mementoSpan("", 10); got != "" {
		t.Fatal("an empty span is empty")
	}
}

// One keystroke in a large field is one copy of the text: no rune
// slices, no per-edit scan of the history.
func BenchmarkEditInsertLargeText(b *testing.B) {
	for _, n := range []int{10_000, 1 << 20} {
		text := strings.Repeat("a", n)
		b.Run(byteSizeName(n), func(b *testing.B) {
			b.ReportAllocs()
			undo := newBoundedStack[inputMemento](undoMaxSize)
			for i := 0; i < b.N; i++ {
				is := inputState{CursorPos: n / 2, Undo: undo}
				_, _, _ = editInsert(text, "x", is)
			}
		})
	}
}

func byteSizeName(n int) string {
	if n >= 1<<20 {
		return "1MiB"
	}
	return "10KB"
}
