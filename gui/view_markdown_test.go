package gui

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestMarkdownViewGeneratesLayout(t *testing.T) {
	w := &Window{}
	v := w.Markdown(MarkdownCfg{
		ID:     "md1",
		Source: "# Hello",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
}

func TestMarkdownViewEmptySource(t *testing.T) {
	w := &Window{}
	v := w.Markdown(MarkdownCfg{
		ID:     "md2",
		Source: "",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape even with empty source")
	}
}

func TestMarkdownViewInvisible(t *testing.T) {
	w := &Window{}
	v := w.Markdown(MarkdownCfg{
		ID:        "md3",
		Source:    "text",
		Invisible: true,
	})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled {
		t.Error("invisible markdown should be disabled")
	}
	if !layout.Shape.OverDraw {
		t.Error("invisible markdown should be overdraw")
	}
}

// --- Custom MathFetcher / MermaidFetcher ---

func TestMarkdownView_CustomMathFetcherCalled(t *testing.T) {
	SetMarkdownExternalAPIsEnabled(true)
	defer SetMarkdownExternalAPIsEnabled(false)

	var calls atomic.Int32
	done := make(chan struct{})
	myFetcher := func(
		_ context.Context, _ string, _ int, _ Color,
	) ([]byte, error) {
		calls.Add(1)
		close(done)
		return nil, fmt.Errorf("test fetcher")
	}

	w := &Window{}
	v := w.Markdown(MarkdownCfg{
		ID:          "md_math",
		Source:      "$$x^2$$",
		MathFetcher: myFetcher,
	})
	_ = generateViewLayout(v, w)

	select {
	case <-done:
		if calls.Load() != 1 {
			t.Errorf("expected 1 call, got %d", calls.Load())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("custom MathFetcher was not called")
	}
}

func TestMarkdownView_CustomMermaidFetcherCalled(t *testing.T) {
	SetMarkdownExternalAPIsEnabled(true)
	defer SetMarkdownExternalAPIsEnabled(false)

	var calls atomic.Int32
	done := make(chan struct{})
	myFetcher := func(
		_ context.Context, _ string,
	) ([]byte, error) {
		calls.Add(1)
		close(done)
		return nil, fmt.Errorf("test fetcher")
	}

	w := &Window{}
	v := w.Markdown(MarkdownCfg{
		ID:             "md_mermaid",
		Source:         "```mermaid\ngraph TD\n  A-->B\n```",
		MermaidFetcher: myFetcher,
	})
	_ = generateViewLayout(v, w)

	select {
	case <-done:
		if calls.Load() != 1 {
			t.Errorf("expected 1 call, got %d", calls.Load())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("custom MermaidFetcher was not called")
	}
}

func TestMarkdownView_FetcherDefaultsToNil(t *testing.T) {
	// Verify zero-value MarkdownCfg has nil fetchers
	// (defaults are applied inside fetchMathAsync/fetchMermaidAsync).
	cfg := MarkdownCfg{}
	if cfg.MathFetcher != nil {
		t.Error("zero MathFetcher should be nil")
	}
	if cfg.MermaidFetcher != nil {
		t.Error("zero MermaidFetcher should be nil")
	}
}
