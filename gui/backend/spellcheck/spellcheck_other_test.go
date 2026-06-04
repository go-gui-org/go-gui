//go:build !darwin && (!linux || android || !hunspell)

package spellcheck

import "testing"

func TestCheckNoPanic(t *testing.T) {
	t.Parallel()
	r := Check("")
	if r != nil {
		t.Errorf("Check: got %v, want nil", r)
	}
}

func TestSuggestNoPanic(t *testing.T) {
	t.Parallel()
	r := Suggest("", 0, 0)
	if r != nil {
		t.Errorf("Suggest: got %v, want nil", r)
	}
}

func TestLearnNoPanic(t *testing.T) {
	t.Parallel()
	Learn("")
}
