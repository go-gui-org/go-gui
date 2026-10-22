package msl

import (
	"strings"
	"testing"
)

// Decls land at program scope: after the VertexOut struct the
// fragment function reads, before fs_main, outside any function
// (#573).
func TestBuildCustomDeclsBeforeFragment(t *testing.T) {
	decls := "float _helper(float x) { return x * 2.0; }"
	body := "float4 frag_color = float4(_helper(1.0));"
	src := BuildCustomDecls(decls, body)
	at := strings.Index(src, decls)
	if at < 0 {
		t.Fatal("decls missing")
	}
	if s := strings.Index(src, "struct VertexOut"); s < 0 || s > at {
		t.Error("decls precede struct VertexOut")
	}
	if f := strings.Index(src, "fragment float4 fs_main("); f < at {
		t.Error("decls not before fs_main")
	}
	if strings.Index(src, body) < strings.Index(src, "fragment float4 fs_main(") {
		t.Error("body not inside fs_main")
	}
	if BuildCustomDecls("", body) != BuildCustom(body) {
		t.Error("empty decls should match BuildCustom")
	}
}
