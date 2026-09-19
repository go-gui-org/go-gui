package gui

import "testing"

func TestBuildSvgCacheLookupKey_DimQuantization(t *testing.T) {
	// 32.34 * 10 truncates to 323; 32.39 also truncates to 323.
	// 32.41 → 324. The test pins the int32 truncation contract.
	a := buildSvgCacheLookupKey(0xdead, 0, 32.34, 10, SvgParseOpts{})
	b := buildSvgCacheLookupKey(0xdead, 0, 32.39, 10, SvgParseOpts{})
	c := buildSvgCacheLookupKey(0xdead, 0, 32.41, 10, SvgParseOpts{})
	if a != b {
		t.Errorf("32.34 and 32.39 should hash equal; %+v %+v", a, b)
	}
	if a == c {
		t.Errorf("32.34 and 32.41 should hash distinct; %+v %+v", a, c)
	}
}

func TestBuildSvgCacheLookupKey_ReducedMotionDistinct(t *testing.T) {
	on := buildSvgCacheLookupKey(1, 0, 100, 100,
		SvgParseOpts{PrefersReducedMotion: true})
	off := buildSvgCacheLookupKey(1, 0, 100, 100,
		SvgParseOpts{PrefersReducedMotion: false})
	if on == off {
		t.Fatal("reduced-motion flag must change cache key")
	}
}

func TestBuildSvgCacheLookupKey_SrcHashIsolation(t *testing.T) {
	a := buildSvgCacheLookupKey(1, 0, 100, 100, SvgParseOpts{})
	b := buildSvgCacheLookupKey(2, 0, 100, 100, SvgParseOpts{})
	if a == b {
		t.Fatal("distinct srcHash must produce distinct keys")
	}
}

type stubReducedMotionPlatform struct {
	NativePlatform
	pref bool
}

func (s stubReducedMotionPlatform) PrefersReducedMotion() bool {
	return s.pref
}

func TestSvgParseOpts_AdapterPresentReturnsTrue(t *testing.T) {
	w := &Window{}
	w.nativePlatform = stubReducedMotionPlatform{pref: true}
	if !w.svgParseOpts().PrefersReducedMotion {
		t.Error("expected PrefersReducedMotion true from adapter")
	}
}

func TestSvgParseOpts_AdapterAbsentReturnsZero(t *testing.T) {
	w := &Window{}
	if w.svgParseOpts().PrefersReducedMotion {
		t.Error("nil adapter should yield zero opts")
	}
}
