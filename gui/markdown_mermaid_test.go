package gui

import (
	"context"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui/markdown"
)

func TestMarkdownMermaid(t *testing.T) {
	style := DefaultMarkdownStyle()
	blocks := markdownToBlocks(
		"```mermaid\ngraph TD\n  A-->B\n```\n", style)
	if len(blocks) == 0 {
		t.Fatal("expected at least one block")
	}
	found := false
	for _, b := range blocks {
		if b.IsCode && b.CodeLanguage == "mermaid" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mermaid code block")
	}
}

func TestMarkdownMermaidAltFence(t *testing.T) {
	style := DefaultMarkdownStyle()
	blocks := markdownToBlocks(
		"~~~mermaid\ngraph TD\n  A-->B\n~~~\n", style)
	found := false
	for _, b := range blocks {
		if b.IsCode && b.CodeLanguage == "mermaid" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mermaid code block with tilde fence")
	}
}

func TestDiagramCacheBasicOps(t *testing.T) {
	cache := newBoundedDiagramCache(10)
	entry := DiagramCacheEntry{
		State:     diagramLoading,
		RequestID: 42,
	}
	cache.Set(100, entry)
	got, ok := cache.Get(100)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.RequestID != 42 {
		t.Fatalf("requestID: got %d", got.RequestID)
	}
	if cache.LoadingCount() != 1 {
		t.Fatalf("loading count: got %d", cache.LoadingCount())
	}
}

func TestDiagramCacheReplacesOldEntry(t *testing.T) {
	cache := newBoundedDiagramCache(10)
	cache.Set(100, DiagramCacheEntry{
		State:     diagramLoading,
		RequestID: 1,
	})
	cache.Set(100, DiagramCacheEntry{
		State:     diagramReady,
		RequestID: 2,
		Width:     100,
		Height:    50,
	})
	got, ok := cache.Get(100)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.State != diagramReady || got.RequestID != 2 {
		t.Fatalf("expected ready/2, got %v/%d",
			got.State, got.RequestID)
	}
}

func TestDiagramCacheEviction(t *testing.T) {
	cache := newBoundedDiagramCache(3)
	for i := range 5 {
		cache.Set(int64(i), DiagramCacheEntry{
			State:     diagramReady,
			RequestID: uint64(i),
		})
	}
	if cache.Len() > 3 {
		t.Fatalf("cache should not exceed capacity: len=%d",
			cache.Len())
	}
}

func TestDefaultMermaidFetcher_SourceTooLarge(t *testing.T) {
	longSource := strings.Repeat("x", markdown.MaxMermaidSourceLen+1)
	_, err := defaultMermaidFetcher(
		context.Background(), longSource)
	if err == nil {
		t.Fatal("expected error for oversized source")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error should mention size: %v", err)
	}
}

func TestDiagramCacheClear(t *testing.T) {
	cache := newBoundedDiagramCache(10)
	cache.Set(1, DiagramCacheEntry{State: diagramLoading})
	cache.Set(2, DiagramCacheEntry{State: diagramReady})
	cache.Set(3, DiagramCacheEntry{State: diagramLoading})
	cache.Clear()
	if cache.Len() != 0 {
		t.Fatalf("expected empty cache after clear: len=%d",
			cache.Len())
	}
	if cache.LoadingCount() != 0 {
		t.Fatalf("expected zero loading count after clear: got %d",
			cache.LoadingCount())
	}
}
