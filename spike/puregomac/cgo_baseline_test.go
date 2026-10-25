//go:build darwin && cgo

package main

import "testing"

// Run: CGO_ENABLED=1 go test -run '^$' -bench Cgo -benchmem ./spike/puregomac
func BenchmarkCgoSend(b *testing.B) {
	load()
	obj := sys0(sys0(cls("NSObject"), sel("alloc")), sel("init"))
	s := uintptr(sel("hash"))
	b.ReportAllocs()
	for b.Loop() {
		cgoHash(obj, s)
	}
}
