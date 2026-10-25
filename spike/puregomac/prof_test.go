//go:build darwin

package main

import "testing"

// Per-send cost of each style, for the spike's numbers table.
// Run: CGO_ENABLED=0 go test -run '^$' -bench . -benchmem ./spike/puregomac
func BenchmarkSend(b *testing.B) {
	load()
	obj := sys0(sys0(cls("NSObject"), sel("alloc")), sel("init"))
	s := sel("hash")
	b.Run("ID.Send", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			obj.Send(s)
		}
	})
	b.Run("RegisterFunc", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			msg0(obj, s)
		}
	})
	b.Run("SyscallN/variadic", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			vsys0(obj, s)
		}
	})
	b.Run("SyscallN/buffered", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sys0(obj, s)
		}
	})
}
