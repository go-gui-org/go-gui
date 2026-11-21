//go:build !js && !darwin && !android

package gl

import "testing"

// hintSpy records SetHinting calls so a test sees when glyph is reached.
type hintSpy struct{ calls []bool }

func (s *hintSpy) SetHinting(on bool) { s.calls = append(s.calls, on) }

// SetHinting must not reach glyph at once. glyph purges its atlas on a
// change, and a purge can allocate a texture in whatever GL context is
// current. Each window owns an unshared context, so a call made while
// another window's context is current would put the texture in the wrong
// one (#1334). The change waits for flushHinting, which drawFrame calls
// after it makes the window's own context current.
func TestSetHintingWaitsForFlush(t *testing.T) {
	spy := &hintSpy{}
	tm := &textMeasurer{hintSink: spy}
	tm.SetHinting(false)
	if len(spy.calls) != 0 {
		t.Fatalf("SetHinting reached glyph before flush: %v", spy.calls)
	}
	tm.flushHinting()
	if len(spy.calls) != 1 || spy.calls[0] {
		t.Fatalf("flush calls = %v, want [false]", spy.calls)
	}
	// Nothing is pending now, so a second flush does no work.
	tm.flushHinting()
	if len(spy.calls) != 1 {
		t.Fatalf("second flush calls = %v, want 1 call", spy.calls)
	}
}

// Only the last value counts when the app flips the setting twice between
// frames.
func TestSetHintingKeepsLastValue(t *testing.T) {
	spy := &hintSpy{}
	tm := &textMeasurer{hintSink: spy}
	tm.SetHinting(false)
	tm.SetHinting(true)
	tm.flushHinting()
	if len(spy.calls) != 1 || !spy.calls[0] {
		t.Fatalf("flush calls = %v, want [true]", spy.calls)
	}
}

func TestFlushHintingWithoutPendingIsSafe(t *testing.T) {
	var tm textMeasurer // nil textSys and no sink: must not panic
	tm.flushHinting()
	var nilTM *textMeasurer
	nilTM.flushHinting()
}
