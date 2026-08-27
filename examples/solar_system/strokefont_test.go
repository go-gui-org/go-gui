package main

import "testing"

func TestStrokeFontCoversMonthNames(t *testing.T) {
	for _, name := range monthNames {
		for _, ch := range name {
			if _, ok := glyphFor(ch); !ok {
				t.Fatalf("month name %q: rune %q has no glyph", name, ch)
			}
		}
	}
}

func TestStrokeFontFullAlphabet(t *testing.T) {
	for ch := 'A'; ch <= 'Z'; ch++ {
		g, ok := glyphFor(ch)
		if !ok {
			t.Fatalf("letter %q has no glyph", ch)
		}
		if len(g) == 0 {
			t.Fatalf("letter %q glyph is empty", ch)
		}
		for _, poly := range g {
			if len(poly)%2 != 0 {
				t.Fatalf("letter %q poly has odd length %d", ch, len(poly))
			}
			if len(poly) < 4 {
				t.Fatalf("letter %q poly too short %d", ch, len(poly))
			}
			for k := 0; k < len(poly); k += 2 {
				if poly[k] < -0.1 || poly[k] > 1.1 || poly[k+1] < -0.1 || poly[k+1] > 1.1 {
					t.Fatalf("letter %q point %v out of [0,1] range", ch, poly[k:k+2])
				}
			}
		}
		if _, ok := glyphFor(ch + 32); !ok {
			t.Fatalf("lowercase %q has no glyph", ch+32)
		}
	}
}

func TestGlyphForRejectsNonLetter(t *testing.T) {
	for _, ch := range []rune{'0', ' ', '-', '.'} {
		if _, ok := glyphFor(ch); ok {
			t.Fatalf("rune %q should have no glyph", ch)
		}
	}
}
