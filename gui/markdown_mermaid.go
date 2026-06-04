package gui

// markdown_mermaid.go implements diagram caching and async
// mermaid diagram fetching via the Kroki API.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"time"

	"github.com/go-gui-org/go-gui/gui/markdown"
)

const (
	maxConcurrentDiagramFetches = 8
	maxDiagramResponseBytes     = 10 * 1024 * 1024
	diagramFetchTimeout         = 30 * time.Second
)

// DiagramState represents the loading state of a diagram.
type DiagramState uint8

// DiagramState constants.
const (
	DiagramLoading DiagramState = iota
	DiagramReady
	DiagramError
)

// DiagramCacheEntry stores cached diagram data.
type DiagramCacheEntry struct {
	PNGPath   string // temp file path
	Error     string
	RequestID uint64
	Width     float32
	Height    float32
	DPI       float32 // DPI used for rendering
	State     DiagramState
}

// BoundedDiagramCache is a FIFO cache for diagram entries.
// Custom cache (not BoundedMap) — needs png_path cleanup
// on evict/overwrite.
type BoundedDiagramCache struct {
	data         map[int64]DiagramCacheEntry
	order        []int64
	maxSize      int
	loadingCount int
}

// NewBoundedDiagramCache creates a diagram cache with the
// given capacity.
func NewBoundedDiagramCache(maxSize int) *BoundedDiagramCache {
	if maxSize < 1 {
		maxSize = 50
	}
	return &BoundedDiagramCache{
		data:    make(map[int64]DiagramCacheEntry),
		maxSize: maxSize,
	}
}

// Get returns a cached diagram entry.
func (c *BoundedDiagramCache) Get(
	key int64,
) (DiagramCacheEntry, bool) {
	e, ok := c.data[key]
	return e, ok
}

// Set adds a diagram entry. Evicts oldest if at capacity.
// Cleans up old temp files on overwrite or eviction.
func (c *BoundedDiagramCache) Set(
	key int64, value DiagramCacheEntry,
) {
	if c.maxSize < 1 {
		return
	}
	existing, exists := c.data[key]
	if exists {
		if existing.State == DiagramLoading {
			c.loadingCount--
		}
		if existing.PNGPath != "" &&
			existing.PNGPath != value.PNGPath {
			removeDiagramPNG(existing.PNGPath)
		}
	} else {
		if len(c.data) >= c.maxSize && len(c.order) > 0 {
			oldest := c.order[0]
			if oe, ok := c.data[oldest]; ok {
				if oe.State == DiagramLoading {
					c.loadingCount--
				}
				if oe.PNGPath != "" {
					removeDiagramPNG(oe.PNGPath)
				}
			}
			delete(c.data, oldest)
			c.order = c.order[1:]
			if len(c.order) < cap(c.order)/2 {
				c.order = append([]int64(nil), c.order...)
			}
		}
		c.order = append(c.order, key)
	}
	if value.State == DiagramLoading {
		c.loadingCount++
	}
	c.data[key] = value
}

// LoadingCount returns entries in loading state.
func (c *BoundedDiagramCache) LoadingCount() int {
	return c.loadingCount
}

// Len returns the number of entries.
func (c *BoundedDiagramCache) Len() int {
	return len(c.data)
}

// Clear removes all entries and deletes temp files.
func (c *BoundedDiagramCache) Clear() {
	for _, e := range c.data {
		if e.PNGPath != "" {
			removeDiagramPNG(e.PNGPath)
		}
	}
	clear(c.data)
	c.order = c.order[:0]
	c.loadingCount = 0
}

// diagramCacheShouldApplyResult checks if a result should
// be applied (entry still loading with same request ID).
func diagramCacheShouldApplyResult(
	cache *BoundedDiagramCache, hash int64, requestID uint64,
) bool {
	if cache == nil {
		return false
	}
	e, ok := cache.Get(hash)
	if !ok {
		return false
	}
	return e.State == DiagramLoading &&
		e.RequestID == requestID
}

// finishDiagramFetch decodes PNG body, stores it, and
// queues a cache update. Shared by math and mermaid fetchers.
func finishDiagramFetch(
	w *Window, body []byte, hash int64,
	requestID uint64, dpi float32, kind string,
) {
	img, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		queueDiagramError(w, hash, requestID,
			"PNG decode: "+err.Error())
		return
	}

	bounds := img.Bounds()
	imgW := float32(bounds.Dx())
	imgH := float32(bounds.Dy())

	ref, err := storeDiagramPNG(body, hash, kind)
	if err != nil {
		queueDiagramError(w, hash, requestID,
			"store PNG: "+err.Error())
		return
	}

	w.QueueCommand(func(w *Window) {
		if !diagramCacheShouldApplyResult(
			w.viewState.diagramCache,
			hash, requestID) {
			removeDiagramPNG(ref)
			return
		}
		w.viewState.diagramCache.Set(hash,
			DiagramCacheEntry{
				State:     DiagramReady,
				PNGPath:   ref,
				Width:     imgW,
				Height:    imgH,
				DPI:       dpi,
				RequestID: requestID,
			})
		w.UpdateWindow()
	})
}

// fetchMermaidAsync fetches a mermaid diagram from Kroki API
// in a background goroutine.
//
// PRIVACY NOTE: Mermaid source is sent to external
// third-party API (kroki.io) for rendering.
func fetchMermaidAsync(
	w *Window, source string, hash int64,
	requestID uint64,
) {
	ctx := w.Ctx()
	go func() {
		if len(source) > markdown.MaxMermaidSourceLen {
			queueDiagramError(w, hash, requestID,
				"Mermaid source too large")
			return
		}

		body, err := mermaidHTTPFetch(ctx, source)
		if err != nil {
			queueDiagramError(w, hash, requestID,
				err.Error())
			return
		}

		finishDiagramFetch(w, body, hash, requestID, 0, "mermaid")
	}()
}

func mermaidHTTPFetch(ctx context.Context, source string) ([]byte, error) {
	payload, err := json.Marshal(map[string]string{
		"diagram_source": source,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://kroki.io/mermaid/png",
		bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: diagramFetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(
		io.LimitReader(resp.Body, maxDiagramResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		preview := truncatePreview(string(body), 200)
		return nil, fmt.Errorf(
			"HTTP %d: %s", resp.StatusCode, preview)
	}
	if len(body) > maxDiagramResponseBytes {
		return nil, fmt.Errorf(
			"response too large (%dMB)",
			len(body)/1024/1024)
	}
	return body, nil
}
