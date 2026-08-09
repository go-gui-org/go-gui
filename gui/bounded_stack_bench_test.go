package gui

import "testing"

func BenchmarkBoundedStackPushBelowCapacity(b *testing.B) {
	s := newBoundedStack[int](1024)
	b.ReportAllocs()
	b.ResetTimer()
	n := 0
	for b.Loop() {
		if s.Len() == 1024 {
			s.Clear()
		}
		s.Push(n)
		n++
	}
}

func BenchmarkBoundedStackPushOverflow(b *testing.B) {
	s := newBoundedStack[int](1024)
	for i := range 1024 {
		s.Push(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	n := 0
	for b.Loop() {
		s.Push(n)
		n++
	}
}

func BenchmarkBoundedStackPushPopCycle(b *testing.B) {
	s := newBoundedStack[int](1024)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		s.Push(0)
		s.Pop()
	}
}
