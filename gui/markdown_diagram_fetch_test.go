package gui

// markdown_diagram_fetch_test.go covers the async diagram pipeline in
// markdown_mermaid.go / markdown_math.go: the stale-result guard
// (diagramCacheShouldApplyResult), the decode-and-store completion
// path (finishDiagramFetch) driven through the injected fetcher seam,
// and the two default HTTP fetchers (they hit a fixed hostname
// but setDiagramHTTPClient can point the shared client at a stub
// transport, so no network is touched).

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui/markdown"
)

// testPNGBytes encodes a solid-color RGBA image of the given size,
// giving finishDiagramFetch a real PNG to decode.
func testPNGBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// waitForDiagramCommand polls until at least one command is queued
// (finishDiagramFetch/queueDiagramError run on the fetcher goroutine,
// so the queue fills asynchronously after the fetcher returns) and
// then drains the queue with a frame. It reads the queue through
// pendingCommandCount so the lock discipline stays with the queue.
func waitForDiagramCommand(t *testing.T, w *Window) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if w.pendingCommandCount() > 0 {
			w.FrameFn()
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("diagram command never queued")
		}
		time.Sleep(time.Millisecond)
	}
}

// newDiagramWindow returns a window with an empty diagram cache.
// It runs the production lazy init first so a break there fails
// here too, then installs a small isolated cache to keep tests
// fast and independent.
func newDiagramWindow() *Window {
	w := NewTestWindow(WindowCfg{Width: 100, Height: 100})
	ensureDiagramCache(w)
	w.viewState.diagramCache = newBoundedDiagramCache(10)
	return w
}

// --- diagramCacheShouldApplyResult ---

func TestDiagramCacheShouldApplyResult(t *testing.T) {
	// Nil cache: nothing to apply to.
	if diagramCacheShouldApplyResult(nil, 1, 1) {
		t.Error("nil cache accepted a result")
	}

	cache := newBoundedDiagramCache(10)

	// Missing entry: a fetch that never started must not land.
	if diagramCacheShouldApplyResult(cache, 1, 1) {
		t.Error("missing entry accepted a result")
	}

	// Entry present but already ready: a late duplicate must not
	// overwrite the stored diagram.
	cache.Set(1, DiagramCacheEntry{
		State:     diagramReady,
		RequestID: 1,
		Width:     10,
	})
	if diagramCacheShouldApplyResult(cache, 1, 1) {
		t.Error("ready entry accepted a result")
	}

	// Loading but from a different request: the document re-rendered
	// and re-issued, so this result is stale.
	cache.Set(2, DiagramCacheEntry{
		State:     diagramLoading,
		RequestID: 7,
	})
	if diagramCacheShouldApplyResult(cache, 2, 8) {
		t.Error("stale requestID accepted a result")
	}

	// The one case that must pass: entry still loading with the same
	// request ID.
	if !diagramCacheShouldApplyResult(cache, 2, 7) {
		t.Error("matching loading entry rejected a result")
	}
}

// --- finishDiagramFetch end to end ---

// TestFetchMermaidAsyncStoresReadyEntry runs the full seam: a loading
// cache entry, an injected fetcher returning a real PNG, and the
// queued finishDiagramFetch command. The cache must end up ready with
// the decoded dimensions and a stored temp file.
func TestFetchMermaidAsyncStoresReadyEntry(t *testing.T) {
	w := newDiagramWindow()

	body := testPNGBytes(t, 60, 40)
	hash := diagramCacheHash("graph TD\n  A-->B")
	reqID := nextDiagramRequestID(w)
	w.viewState.diagramCache.Set(hash, DiagramCacheEntry{
		State:     diagramLoading,
		RequestID: reqID,
	})

	var calls atomic.Int32
	done := make(chan struct{})
	fetchMermaidAsync(w, "graph TD\n  A-->B", hash, reqID,
		func(_ context.Context, _ string) ([]byte, error) {
			calls.Add(1)
			close(done)
			return body, nil
		})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("mermaid fetcher was not called")
	}
	waitForDiagramCommand(t, w)

	entry, ok := w.viewState.diagramCache.Get(hash)
	if !ok {
		t.Fatal("diagram hash missing after fetch")
	}
	if entry.State != diagramReady {
		t.Fatalf("entry state = %v, want ready (error: %q)",
			entry.State, entry.Error)
	}
	if entry.Width != 60 || entry.Height != 40 {
		t.Errorf("decoded size = %gx%g, want 60x40",
			entry.Width, entry.Height)
	}
	if entry.RequestID != reqID {
		t.Errorf("requestID = %d, want %d", entry.RequestID, reqID)
	}
	if entry.pNGPath == "" {
		t.Fatal("ready entry has no stored PNG path")
	}
	storedPath := entry.pNGPath
	if runtime.GOOS == "js" {
		// The WASM build stores diagrams as data URLs, not temp
		// files (diagram_store_js.go) — there is no filesystem to
		// stat. Pin the wasm contract instead: the entry's "path" is
		// a base64 data URL of the fetched PNG.
		if !strings.HasPrefix(storedPath,
			"data:image/png;base64,") {
			t.Errorf("entry pNGPath = %q, want a PNG data URL on "+
				"wasm", storedPath)
		}
	} else {
		// Delete the temp file when the test ends. Register
		// before the reads below so a failure cannot leak it.
		t.Cleanup(func() { removeDiagramPNG(storedPath) })
		stored, err := os.ReadFile(storedPath)
		if err != nil {
			t.Fatalf("stored PNG missing: %v", err)
		}
		if !bytes.Equal(stored, body) {
			t.Error("stored PNG differs from the fetched body")
		}
	}
}

// TestFetchMermaidAsyncFetcherErrorQueuesError asserts a fetcher
// failure lands as a diagramError entry (queueDiagramError's apply
// path).
func TestFetchMermaidAsyncFetcherErrorQueuesError(t *testing.T) {
	w := newDiagramWindow()

	hash := diagramCacheHash("graph TD\n  A-->B")
	reqID := nextDiagramRequestID(w)
	w.viewState.diagramCache.Set(hash, DiagramCacheEntry{
		State:     diagramLoading,
		RequestID: reqID,
	})

	done := make(chan struct{})
	fetchMermaidAsync(w, "graph TD\n  A-->B", hash, reqID,
		func(_ context.Context, _ string) ([]byte, error) {
			close(done)
			return nil, errors.New("kroki down")
		})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("mermaid fetcher was not called")
	}
	waitForDiagramCommand(t, w)

	entry, _ := w.viewState.diagramCache.Get(hash)
	if entry.State != diagramError {
		t.Fatalf("entry state = %v, want error", entry.State)
	}
	if entry.Error != "kroki down" {
		t.Errorf("error text = %q, want %q", entry.Error, "kroki down")
	}
}

// TestFetchMermaidAsyncSourceTooLarge asserts the size guard fires
// before the fetcher is consulted.
func TestFetchMermaidAsyncSourceTooLarge(t *testing.T) {
	w := newDiagramWindow()

	hash := diagramCacheHash("x")
	reqID := nextDiagramRequestID(w)
	w.viewState.diagramCache.Set(hash, DiagramCacheEntry{
		State:     diagramLoading,
		RequestID: reqID,
	})

	var fetcherCalled atomic.Bool
	fetchMermaidAsync(w, strings.Repeat("x", markdown.MaxMermaidSourceLen+1),
		hash, reqID,
		func(_ context.Context, _ string) ([]byte, error) {
			fetcherCalled.Store(true)
			return nil, nil
		})

	waitForDiagramCommand(t, w)
	if fetcherCalled.Load() {
		t.Error("fetcher called for an oversized source")
	}
	entry, _ := w.viewState.diagramCache.Get(hash)
	if entry.State != diagramError {
		t.Fatalf("entry state = %v, want error", entry.State)
	}
	if !strings.Contains(entry.Error, "too large") {
		t.Errorf("error text = %q, want it to mention size",
			entry.Error)
	}
}

// TestFinishDiagramFetchBadPNGQueuesError asserts a body that is not a
// PNG fails decode and lands as an error entry.
func TestFinishDiagramFetchBadPNGQueuesError(t *testing.T) {
	w := newDiagramWindow()

	hash := diagramCacheHash("math1")
	reqID := nextDiagramRequestID(w)
	w.viewState.diagramCache.Set(hash, DiagramCacheEntry{
		State:     diagramLoading,
		RequestID: reqID,
	})

	finishDiagramFetch(w, []byte("not a png"), hash, reqID, 150, "math")
	waitForDiagramCommand(t, w)

	entry, _ := w.viewState.diagramCache.Get(hash)
	if entry.State != diagramError {
		t.Fatalf("entry state = %v, want error", entry.State)
	}
	if !strings.Contains(entry.Error, "PNG decode") {
		t.Errorf("error text = %q, want it to mention PNG decode",
			entry.Error)
	}
}

// TestStaleDiagramResultDropped asserts the stale-request guard: when
// the cache entry is replaced by a newer request before the fetch
// lands, the queued completion must not overwrite it.
func TestStaleDiagramResultDropped(t *testing.T) {
	w := newDiagramWindow()

	body := testPNGBytes(t, 10, 10)
	hash := diagramCacheHash("stale")
	oldReq := nextDiagramRequestID(w)
	w.viewState.diagramCache.Set(hash, DiagramCacheEntry{
		State:     diagramLoading,
		RequestID: oldReq,
	})

	done := make(chan struct{})
	fetchMermaidAsync(w, "stale", hash, oldReq,
		func(_ context.Context, _ string) ([]byte, error) {
			close(done)
			return body, nil
		})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("fetcher not called")
	}

	// The document re-rendered and issued a new request for the same
	// diagram before the old one completed.
	newReq := nextDiagramRequestID(w)
	w.viewState.diagramCache.Set(hash, DiagramCacheEntry{
		State:     diagramLoading,
		RequestID: newReq,
	})

	waitForDiagramCommand(t, w)

	entry, ok := w.viewState.diagramCache.Get(hash)
	if !ok {
		t.Fatal("entry missing")
	}
	if entry.State != diagramLoading || entry.RequestID != newReq {
		t.Errorf("stale result overwrote the entry: state=%v "+
			"requestID=%d, want loading/%d",
			entry.State, entry.RequestID, newReq)
	}
	if entry.pNGPath != "" {
		t.Error("stale result left a PNG path in the cache")
	}
}

// TestStaleDiagramErrorDropped is the same guard on the error path:
// queueDiagramError must also refuse a stale requestID.
func TestStaleDiagramErrorDropped(t *testing.T) {
	w := newDiagramWindow()

	hash := diagramCacheHash("stale-err")
	oldReq := nextDiagramRequestID(w)
	w.viewState.diagramCache.Set(hash, DiagramCacheEntry{
		State:     diagramLoading,
		RequestID: oldReq,
	})

	done := make(chan struct{})
	fetchMermaidAsync(w, "stale-err", hash, oldReq,
		func(_ context.Context, _ string) ([]byte, error) {
			close(done)
			return nil, errors.New("boom")
		})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("fetcher not called")
	}

	newReq := nextDiagramRequestID(w)
	w.viewState.diagramCache.Set(hash, DiagramCacheEntry{
		State:     diagramLoading,
		RequestID: newReq,
	})

	waitForDiagramCommand(t, w)

	entry, _ := w.viewState.diagramCache.Get(hash)
	if entry.State != diagramLoading || entry.RequestID != newReq {
		t.Errorf("stale error overwrote the entry: state=%v "+
			"requestID=%d, want loading/%d",
			entry.State, entry.RequestID, newReq)
	}
	if entry.Error != "" {
		t.Errorf("stale error text = %q, want empty", entry.Error)
	}
}

// --- Default HTTP fetchers ---

// stubTransport returns a canned response for every request, recording
// the URL so tests can assert the request the default fetcher builds.
type stubTransport struct {
	status int
	body   []byte
	urls   *[]string
}

func (s stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if s.urls != nil {
		*s.urls = append(*s.urls, req.URL.String())
	}
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(bytes.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

// withStubHTTPClient swaps the shared diagramHTTPClient for one backed
// by stub and restores it when the test ends. The swap goes through
// the mutex-guarded setter so it stays race-clean against fetches on
// background goroutines. Tests in this package must not run in
// parallel while touching diagrams, or one test would observe
// another test's stub.
func withStubHTTPClient(t *testing.T, status int, body []byte) *[]string {
	t.Helper()
	var urls []string
	prev := getDiagramHTTPClient()
	setDiagramHTTPClient(&http.Client{
		Timeout:   diagramFetchTimeout,
		Transport: stubTransport{status: status, body: body, urls: &urls},
	})
	t.Cleanup(func() { setDiagramHTTPClient(prev) })
	return &urls
}

// TestSetDiagramHTTPClientNilKeepsCurrent asserts a nil swap
// leaves the shared client in place instead of breaking later
// fetches with a nil-pointer call.
func TestSetDiagramHTTPClientNilKeepsCurrent(t *testing.T) {
	before := getDiagramHTTPClient()
	setDiagramHTTPClient(nil)
	if getDiagramHTTPClient() != before {
		t.Error("nil swap replaced the shared client")
	}
}

func TestDefaultMermaidFetcherHTTP(t *testing.T) {
	body := testPNGBytes(t, 30, 20)
	urls := withStubHTTPClient(t, 200, body)

	got, err := defaultMermaidFetcher(context.Background(), "graph TD\n  A-->B")
	if err != nil {
		t.Fatalf("defaultMermaidFetcher: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Error("fetcher did not return the response body")
	}
	if len(*urls) != 1 {
		t.Fatalf("requests = %d, want 1", len(*urls))
	}
	if !strings.Contains((*urls)[0], "https://kroki.io/mermaid/png") {
		t.Errorf("request URL = %q, want the kroki mermaid png "+
			"endpoint", (*urls)[0])
	}
}

func TestDefaultMermaidFetcherHTTPError(t *testing.T) {
	withStubHTTPClient(t, 500, []byte("boom"))

	_, err := defaultMermaidFetcher(context.Background(), "graph TD")
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("error = %v, want it to mention HTTP 500", err)
	}
}

// TestDefaultMathFetcherHTTP asserts the URL construction: the DPI
// clamp (10 → 24), the white-foreground color command (luminance > 128
// requests \color{white}), and the body passthrough.
func TestDefaultMathFetcherHTTP(t *testing.T) {
	body := testPNGBytes(t, 25, 25)
	urls := withStubHTTPClient(t, 200, body)

	got, err := defaultMathFetcher(
		context.Background(), "x^2", 10, RGB(255, 255, 255))
	if err != nil {
		t.Fatalf("defaultMathFetcher: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Error("fetcher did not return the response body")
	}
	if len(*urls) != 1 {
		t.Fatalf("requests = %d, want 1", len(*urls))
	}
	u := (*urls)[0]
	if !strings.Contains(u, "https://latex.codecogs.com/png.image?") {
		t.Errorf("request URL = %q, want the codecogs png endpoint", u)
	}
	if !strings.Contains(u, `\dpi{24}`) {
		t.Errorf("request URL lacks the clamped \\dpi{24}: %q", u)
	}
	if !strings.Contains(u, `\color{white}`) {
		t.Errorf("request URL lacks \\color{white} for a bright "+
			"foreground: %q", u)
	}
	if !strings.Contains(u, "x%5E2") {
		t.Errorf("request URL lacks the encoded latex "+
			"payload (want x%%5E2): %q", u)
	}
}

// TestDefaultMathFetcherURLEncodesReservedChars feeds a formula
// with URL-reserved characters and asserts each one arrives
// encoded, so the formula cannot change the request itself.
func TestDefaultMathFetcherURLEncodesReservedChars(t *testing.T) {
	body := testPNGBytes(t, 25, 25)
	urls := withStubHTTPClient(t, 200, body)

	_, err := defaultMathFetcher(
		context.Background(), "a+b%c&d#e?f=g h", 150,
		RGB(0, 0, 0))
	if err != nil {
		t.Fatalf("defaultMathFetcher: %v", err)
	}
	if len(*urls) != 1 {
		t.Fatalf("requests = %d, want 1", len(*urls))
	}
	u := (*urls)[0]
	for _, want := range []string{
		"a%2Bb", "%25", "%26", "%23", "%3F", "%3D", "g{}h",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("request URL = %q, want it to hold %q",
				u, want)
		}
	}
	_, formula, found := strings.Cut(u, "?")
	if !found {
		t.Fatalf("request URL = %q, want a ? separator", u)
	}
	if strings.Contains(formula, "&") {
		t.Errorf("request URL holds a raw &: %q", u)
	}
}

// TestDefaultMathFetcher_SourceTooLarge asserts the size guard
// fires before any URL is built.
func TestDefaultMathFetcher_SourceTooLarge(t *testing.T) {
	longSource := strings.Repeat("x", markdown.MaxLatexSourceLen+1)
	_, err := defaultMathFetcher(
		context.Background(), longSource, 150, RGB(0, 0, 0))
	if err == nil {
		t.Fatal("expected error for oversized source")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error should mention size: %v", err)
	}
}

// TestDefaultMathFetcherHTTPError covers the non-200 path.
func TestDefaultMathFetcherHTTPError(t *testing.T) {
	withStubHTTPClient(t, 503, []byte("unavailable"))

	_, err := defaultMathFetcher(
		context.Background(), "x^2", 150, RGB(0, 0, 0))
	if err == nil {
		t.Fatal("expected error for HTTP 503")
	}
	if !strings.Contains(err.Error(), "HTTP 503") {
		t.Errorf("error = %v, want it to mention HTTP 503", err)
	}
}
