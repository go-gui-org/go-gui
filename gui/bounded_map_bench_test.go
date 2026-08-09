package gui

import "testing"

func BenchmarkBoundedMapSetUniqueToCapacity(b *testing.B) {
	const capSize = 1024
	m := NewBoundedMap[int, int](capSize)
	b.ReportAllocs()
	b.ResetTimer()
	n := 0
	for b.Loop() {
		if m.Len() == capSize {
			m.Clear()
		}
		m.Set(n, n)
		n++
	}
}

func BenchmarkBoundedMapSetWithEvictions(b *testing.B) {
	const capSize = 1024
	m := NewBoundedMap[int, int](capSize)
	for i := range capSize {
		m.Set(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	n := 0
	for b.Loop() {
		m.Set(capSize+n, n)
		n++
	}
}

func BenchmarkBoundedMapDeleteInsertChurn(b *testing.B) {
	m := NewBoundedMap[int, int](64)
	m.Set(0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	n := 0
	for b.Loop() {
		n++
		m.Set(n, n)
		m.Delete(n)
	}
}

func BenchmarkBoundedMapKeysUnderChurn(b *testing.B) {
	m := NewBoundedMap[int, int](128)
	for i := range 2000 {
		m.Set(i, i)
		if i%2 == 0 {
			m.Delete(i)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = m.Keys()
	}
}

func BenchmarkBoundedMapRangeKeysUnderChurn(b *testing.B) {
	m := NewBoundedMap[int, int](128)
	for i := range 2000 {
		m.Set(i, i)
		if i%2 == 0 {
			m.Delete(i)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		count := 0
		m.rangeKeys(func(int) bool {
			count++
			return true
		})
		if count == 0 {
			b.Fatal("unexpected empty map")
		}
	}
}
