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
	// Fill to capacity, not past it: 100 keys into a cap-50 map evicted
	// "key-Z", so the loop below measured a miss.
	for i := range capModerate {
		sm.Set("key-"+string(rune('A'+i)), i)
	}
	if _, ok := sm.Get("key-Z"); !ok {
		b.Fatal("key-Z must be retained so the loop measures a hit")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = sm.Get("key-Z")
	}
}

// BenchmarkStateMapOverCapacityPass is one view pass that uses more keys
// than a full namespace holds, as a long list does with nsTextBlock: each
// key misses and inserts, so every insert evicts (#1263). Reported per pass.
func BenchmarkStateMapOverCapacityPass(b *testing.B) {
	w := newTestWindow()
	sm := StateMap[int, int](w, "bench-over-cap", capTextBlocks)
	const keysPerPass = 1000
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		w.viewPass++
		for k := range keysPerPass {
			if _, ok := sm.Get(k); !ok {
				sm.Set(k, k)
			}
		}
	}
}

// BenchmarkStateMapEvictAtCapacity inserts a new key into a full namespace
// whose entries were seen in earlier passes: the common eviction (#1263).
func BenchmarkStateMapEvictAtCapacity(b *testing.B) {
	w := newTestWindow()
	sm := StateMap[int, int](w, "bench-evict", capScroll)
	for i := range capScroll {
		sm.Set(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	n := capScroll
	for b.Loop() {
		w.viewPass++
		sm.Set(n, n)
		n++
	}
}

// BenchmarkStateMapEvictBehindLiveKey inserts into a full namespace whose
// oldest entry is read every pass, as a long-lived on-screen widget's state
// is. Each eviction takes the entry behind the live key, so it shifts the
// order slots after it (#1263).
func BenchmarkStateMapEvictBehindLiveKey(b *testing.B) {
	w := newTestWindow()
	sm := StateMap[int, int](w, "bench-live", capTextBlocks)
	for i := range capTextBlocks {
		sm.Set(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	n := capTextBlocks
	for b.Loop() {
		w.viewPass++
		sm.Get(0) // key 0 is the oldest and stays in use
		sm.Set(n, n)
		n++
	}
}
