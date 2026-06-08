package gui

import "testing"

type benchState struct {
	Counter int
	Name    string
	Items   [10]float32
}

func BenchmarkStateTypeLookup(b *testing.B) {
	w := newTestWindow()
	st := &benchState{Counter: 42, Name: "bench"}
	w.setState(st)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		s := State[benchState](w)
		s.Counter++
	}
}

func BenchmarkStateMapLookup(b *testing.B) {
	w := newTestWindow()
	sm := StateMap[string, int](w, nsCombobox, capModerate)
	for i := range 100 {
		sm.Set("key-"+string(rune('A'+i)), i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = sm.Get("key-Z")
	}
}
