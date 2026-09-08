package gui

import (
	"testing"
)

// The mask compiles on every Input generation — once per field per
// frame — so its allocation budget matters. These tests pin it: the
// default-token table is shared, and token-less patterns compile once
// and read back from a cache.
func TestCompiledMaskAllocBudget(t *testing.T) {
	hcfg := inputHandlerCfg{Mask: "(999) 999-9999"}
	if got := testing.AllocsPerRun(100, func() {
		_ = hcfg.compiledMask()
	}); got > 3 {
		t.Fatalf("compiledMask allocs = %v, want <= 3", got)
	}
}

func TestCompiledMaskCustomTokensCompileFresh(t *testing.T) {
	digits := []MaskTokenDef{{
		Symbol:  '#',
		matcher: isASCIIDigit,
	}}
	letters := []MaskTokenDef{{
		Symbol:  '#',
		matcher: isMaskLetter,
	}}
	// Same pattern, different token tables: the cache must not
	// serve one for the other.
	if _, err := compileInputMask("##", digits); err != nil {
		t.Fatal(err)
	}
	got, err := compileInputMask("##", letters)
	if err != nil {
		t.Fatal(err)
	}
	res := inputMaskInsert("", 0, 0, 0, "ab", &got)
	if !res.Changed || res.Text != "ab" {
		t.Fatalf("custom letters mask insert = %q, want %q",
			res.Text, "ab")
	}
}

func TestCompiledMaskCacheSharesInstances(t *testing.T) {
	hcfg := inputHandlerCfg{Mask: "99/99"}
	a := hcfg.compiledMask()
	b := hcfg.compiledMask()
	if a != b {
		t.Fatal("expected the pattern cache to share one instance")
	}
}

func TestCompiledMaskCustomTokensSkipCache(t *testing.T) {
	custom := []MaskTokenDef{{
		Symbol:  '#',
		matcher: isASCIIDigit,
	}}
	hcfg := inputHandlerCfg{Mask: "##", maskTokens: custom}
	a := hcfg.compiledMask()
	b := hcfg.compiledMask()
	if a == nil || b == nil {
		t.Fatal("expected custom-token masks to compile")
	}
	// Custom tables make the key unbounded, so each generation
	// compiles fresh rather than sharing.
	if a == b {
		t.Fatal("expected custom-token masks to compile fresh")
	}
}
