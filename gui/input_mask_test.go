package gui

import (
	"fmt"
	"testing"
)

func TestInputMaskPresets(t *testing.T) {
	assertEqual(t, len(inputMaskFromPreset(MaskNone)), 0)
	if inputMaskFromPreset(MaskPhoneUS) != "(999) 999-9999" {
		t.Fatal("phone_us preset mismatch")
	}
	if inputMaskFromPreset(MaskCreditCard16) != "9999 9999 9999 9999" {
		t.Fatal("credit_card_16 preset mismatch")
	}
	if inputMaskFromPreset(MaskCreditCardAmex) != "9999 999999 99999" {
		t.Fatal("credit_card_amex preset mismatch")
	}
	if inputMaskFromPreset(MaskExpiryMMYY) != "99/99" {
		t.Fatal("expiry_mm_yy preset mismatch")
	}
	if inputMaskFromPreset(MaskCVC) != "999" {
		t.Fatal("cvc preset mismatch")
	}
}

func TestInputMaskSanitizePastePhone(t *testing.T) {
	compiled, err := compileInputMask(inputMaskFromPreset(MaskPhoneUS), nil)
	if err != nil {
		t.Fatal(err)
	}
	res := inputMaskInsert("", 0, 0, 0, "abc555-123-4567xyz", &compiled)
	if !res.Changed {
		t.Fatal("expected changed")
	}
	if res.Text != "(555) 123-4567" {
		t.Fatalf("got %q, want %q", res.Text, "(555) 123-4567")
	}
	if res.CursorPos != len([]rune(res.Text)) {
		t.Fatalf("cursor %d, want %d", res.CursorPos, len([]rune(res.Text)))
	}
}

func TestInputMaskRejectInvalidChar(t *testing.T) {
	compiled, err := compileInputMask("99", nil)
	if err != nil {
		t.Fatal(err)
	}
	res := inputMaskInsert("", 0, 0, 0, "a", &compiled)
	if res.Changed {
		t.Fatal("expected no change")
	}
	if res.Text != "" {
		t.Fatalf("got %q, want empty", res.Text)
	}
	assertEqual(t, res.CursorPos, 0)
}

func TestInputMaskDeleteSkipsLiterals(t *testing.T) {
	compiled, err := compileInputMask(inputMaskFromPreset(MaskPhoneUS), nil)
	if err != nil {
		t.Fatal(err)
	}
	text := ""
	cursor := 0
	for _, ch := range "5551234" {
		res := inputMaskInsert(text, cursor, 0, 0, string(ch), &compiled)
		text = res.Text
		cursor = res.CursorPos
	}
	if text != "(555) 123-4" {
		t.Fatalf("got %q, want %q", text, "(555) 123-4")
	}
	del := inputMaskDelete(text, 4, 0, 0, &compiled)
	if !del.Changed {
		t.Fatal("expected changed")
	}
	if del.Text != "(555) 234" {
		t.Fatalf("got %q, want %q", del.Text, "(555) 234")
	}
}

func TestInputMaskBackspaceRemovesEditableSlot(t *testing.T) {
	compiled, err := compileInputMask(inputMaskFromPreset(MaskPhoneUS), nil)
	if err != nil {
		t.Fatal(err)
	}
	start := inputMaskInsert("", 0, 0, 0, "5551234", &compiled)
	if start.Text != "(555) 123-4" {
		t.Fatalf("got %q, want %q", start.Text, "(555) 123-4")
	}
	back := inputMaskBackspace(start.Text, start.CursorPos, 0, 0, &compiled)
	if !back.Changed {
		t.Fatal("expected changed")
	}
	if back.Text != "(555) 123" {
		t.Fatalf("got %q, want %q", back.Text, "(555) 123")
	}
}

func TestInputMaskCustomTokenTransform(t *testing.T) {
	isUpperLetter := func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}
	toUpper := func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r - 32
		}
		return r
	}
	custom := []MaskTokenDef{
		{Symbol: 'A', Matcher: isUpperLetter, Transform: toUpper},
	}
	compiled, err := compileInputMask("AA-99", custom)
	if err != nil {
		t.Fatal(err)
	}
	res := inputMaskInsert("", 0, 0, 0, "ab12", &compiled)
	if !res.Changed {
		t.Fatal("expected changed")
	}
	if res.Text != "AB-12" {
		t.Fatalf("got %q, want %q", res.Text, "AB-12")
	}
}

func TestCompileInputMaskNilMatcherRejected(t *testing.T) {
	// A token without a Matcher compiles into a slot no keystroke
	// can ever fill, so the compile fails instead of installing a
	// dead slot.
	if _, err := compileInputMask("A9",
		[]MaskTokenDef{{Symbol: 'A'}}); err == nil {
		t.Fatal("expected error for a token without Matcher")
	}
}

func TestInputMaskTokensReachField(t *testing.T) {
	// InputCfg.MaskTokens must travel from the factory into the
	// compiled mask the keystroke path reads: only 'x' fits the
	// custom token, so 'y' is refused.
	w := newTestWindow()
	last := ""
	v := Input(InputCfg{
		ID:   "mask-tokens-e2e",
		Mask: "AA",
		MaskTokens: []MaskTokenDef{{
			Symbol:  'A',
			Matcher: func(r rune) bool { return r == 'x' },
		}},
		OnTextChanged: func(s string, _ EventCtx) { last = s },
	})
	layout := generateViewLayout(v, w)
	w.SetFocus("mask-tokens-e2e")
	setInputState(w, "mask-tokens-e2e", inputState{CursorPos: 0})
	fire := func(ch uint32) {
		if ev := layout.Shape.events; ev != nil && ev.OnChar != nil {
			ev.OnChar(EventCtx{&layout, &Event{
				Type: EventChar, CharCode: ch,
			}, w})
		}
	}
	fire('x')
	if last != "x" {
		t.Fatalf("typed x, text = %q, want %q", last, "x")
	}
	fire('y')
	if last != "x" {
		t.Fatalf("typed y after x, text = %q, want still %q", last, "x")
	}
}

// --- mask token predicates ---

func TestIsMaskLetter(t *testing.T) {
	cases := []struct {
		r    rune
		want bool
	}{
		{'a', true}, {'Z', true}, {'é', true}, {'中', true},
		{'0', false}, {'9', false}, {'-', false}, {'_', false},
		{' ', false}, {'.', false},
	}
	for _, tc := range cases {
		if got := isMaskLetter(tc.r); got != tc.want {
			t.Errorf("isMaskLetter(%q) = %v, want %v",
				tc.r, got, tc.want)
		}
	}
}

func TestIsMaskAlnum(t *testing.T) {
	cases := []struct {
		r    rune
		want bool
	}{
		{'a', true}, {'Z', true}, {'0', true}, {'9', true},
		{'é', true}, {'中', true}, {'٣', true}, // Arabic-Indic digit: unicode.IsNumber
		{'-', false}, {'_', false}, {' ', false}, {'.', false},
	}
	for _, tc := range cases {
		if got := isMaskAlnum(tc.r); got != tc.want {
			t.Errorf("isMaskAlnum(%q) = %v, want %v",
				tc.r, got, tc.want)
		}
	}
}

// TestCompiledMaskCacheCapped pins the bound on the shared mask cache:
// a dynamic pattern source degrades to recompiling, not to unbounded
// growth. Patterns use a unique prefix to avoid colliding with the
// suite's real masks.
func TestCompiledMaskCacheCapped(t *testing.T) {
	// The cache is process-global. Swap in an empty map and put the
	// original back, so filling it here cannot starve the caching
	// other tests (an alloc gate among them) rely on.
	compiledMaskCache.Lock()
	saved := compiledMaskCache.m
	compiledMaskCache.m = make(map[string]*CompiledInputMask)
	compiledMaskCache.Unlock()
	t.Cleanup(func() {
		compiledMaskCache.Lock()
		compiledMaskCache.m = saved
		compiledMaskCache.Unlock()
	})

	pat := func(i int) string { return fmt.Sprintf("test-cap-9-%d-999", i) }
	for i := range compiledMaskCacheMax + 50 {
		p := pat(i)
		c, err := compileInputMask(p, nil)
		if err != nil {
			t.Fatalf("compile %q: %v", p, err)
		}
		storeCompiledMaskCache(p, &c)
	}
	compiledMaskCache.RLock()
	n := len(compiledMaskCache.m)
	compiledMaskCache.RUnlock()
	if n > compiledMaskCacheMax {
		t.Errorf("cache size = %d, want <= %d", n, compiledMaskCacheMax)
	}
	// Overflow must not lock the newcomer out. compiledMask() runs
	// every frame, so a pattern the cache refuses forever recompiles
	// forever; the last one stored has to be cached.
	last := pat(compiledMaskCacheMax + 49)
	if cachedCompiledMask(last) == nil {
		t.Error("last pattern not cached; overflow must admit the newcomer")
	}
	// Re-storing an admitted pattern keeps the original instance:
	// fields built from one pattern must share it, which is the
	// sharing the per-generation alloc gate counts on.
	orig := cachedCompiledMask(last)
	fresh, err := compileInputMask(last, nil)
	if err != nil {
		t.Fatalf("recompile %q: %v", last, err)
	}
	storeCompiledMaskCache(last, &fresh)
	if cachedCompiledMask(last) != orig {
		t.Error("re-store replaced the cached instance; must keep the first")
	}
}

func TestCompiledMaskPanicsOnNilMatcher(t *testing.T) {
	// An uncompilable mask is a programmer error, like inverted
	// numeric bounds: fail at construction instead of silently
	// running the field unmasked.
	defer func() {
		if recover() == nil {
			t.Error("expected panic for a token without a matcher")
		}
	}()
	hcfg := inputHandlerCfg{
		Mask:       "99",
		maskTokens: []MaskTokenDef{{Symbol: '9'}},
	}
	_ = hcfg.compiledMask()
}

func TestSlotEntryOutOfRangeIsSkipped(t *testing.T) {
	// An index from a stale cursor degrades to the zero entry,
	// whose nil matcher reads as "no slot", instead of panicking.
	compiled, err := compileInputMask("99", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, idx := range []int{-1, 2, 100} {
		if got := compiled.slotEntry(idx); got.matcher != nil {
			t.Errorf("slotEntry(%d) has a matcher, want the zero entry", idx)
		}
	}
}
