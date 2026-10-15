package fakegl

import "testing"

func TestN(t *testing.T) {
	if N != 1 {
		t.Fatal("N moved")
	}
}
