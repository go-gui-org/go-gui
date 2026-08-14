package gui

import "testing"

func TestInputMaskPresets(t *testing.T) {
	assertEqual(t, len(inputMaskFromPreset(maskNone)), 0)
	if inputMaskFromPreset(MaskPhoneUS) != "(999) 999-9999" {
		t.Fatal("phone_us preset mismatch")
	}
	if inputMaskFromPreset(maskCreditCard16) != "9999 9999 9999 9999" {
		t.Fatal("credit_card_16 preset mismatch")
	}
	if inputMaskFromPreset(maskCreditCardAmex) != "9999 999999 99999" {
		t.Fatal("credit_card_amex preset mismatch")
	}
	if inputMaskFromPreset(MaskExpiryMMYY) != "99/99" {
		t.Fatal("expiry_mm_yy preset mismatch")
	}
	if inputMaskFromPreset(maskCVC) != "999" {
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
		{Symbol: 'A', matcher: isUpperLetter, Transform: toUpper},
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
