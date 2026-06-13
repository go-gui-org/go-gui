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

func TestCheckWithText(t *testing.T) {
	t.Parallel()
	r := Check("hello world")
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

func TestSuggestWithRange(t *testing.T) {
	t.Parallel()
	r := Suggest("speling", 0, 7)
	if r != nil {
		t.Errorf("Suggest: got %v, want nil", r)
	}
}

func TestLearnNoPanic(t *testing.T) {
	t.Parallel()
	Learn("")
}

func TestLearnWithWord(t *testing.T) {
	t.Parallel()
	Learn("onomatopoeia")
	// No panic is the assertion.
}

func TestSpellFunctionsConcurrent(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	go func() {
		defer func() { done <- struct{}{} }()
		_ = Check("text")
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		_ = Suggest("word", 0, 4)
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		Learn("word")
	}()
	for range 3 {
		<-done
	}
}
