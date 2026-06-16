package gui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestContentTypeToExt(t *testing.T) {
	tests := []struct {
		ct  string
		ext string
	}{
		{"image/png", ".png"},
		{"image/jpeg", ".jpg"},
		{"image/svg+xml", ".svg"},
		{"image/unknown", ".png"},
	}
	for _, tt := range tests {
		if got := contentTypeToExt(tt.ct); got != tt.ext {
			t.Errorf("contentTypeToExt(%q) = %q, want %q",
				tt.ct, got, tt.ext)
		}
	}
}

func TestFindCachedImageFound(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "abc123")
	path := base + ".jpg"
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := findCachedImage(base); got != path {
		t.Fatalf("findCachedImage: got %q, want %q", got, path)
	}
}

func TestFindCachedImageNotFound(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "missing")
	if got := findCachedImage(base); got != "" {
		t.Fatalf("findCachedImage: got %q, want \"\"", got)
	}
}

// removeCachedFor removes any cache file ResolveImageSrc would
// produce for url. Call from t.Cleanup to keep tests hermetic.
func removeCachedFor(url string) {
	hash := hashString(url)
	base := filepath.Join(
		os.TempDir(), "gui_cache", "images",
		strconv.FormatUint(hash, 16))
	for _, ext := range []string{
		".png", ".jpg", ".jpeg", ".svg",
	} {
		_ = os.Remove(base + ext)
	}
}

func TestResolveImageSrcEmpty(t *testing.T) {
	w := &Window{}
	if got := ResolveImageSrc(w, ""); got != "" {
		t.Fatalf("empty src: got %q, want %q", got, "")
	}
}

func TestResolveImageSrcDataURL(t *testing.T) {
	w := &Window{}
	src := "data:image/png;base64,AAAA"
	if got := ResolveImageSrc(w, src); got != src {
		t.Fatalf("data URL: got %q, want %q", got, src)
	}
}

func TestResolveImageSrcURLTooLong(t *testing.T) {
	w := &Window{}
	huge := "https://example.test/" + strings.Repeat("a",
		maxImageURLLen+1) + ".png"
	if got := ResolveImageSrc(w, huge); got != "" {
		t.Fatalf("oversized URL: got %q, want \"\"", got)
	}
	// No download should have been scheduled.
	dl := StateMapRead[string, int64](w, nsActiveDownloads)
	if dl != nil && dl.Contains(huge) {
		t.Fatal("oversized URL should not schedule a download")
	}
}

func TestResolveImageSrcLocalPath(t *testing.T) {
	w := &Window{}
	src := "/some/local/path.png"
	if got := ResolveImageSrc(w, src); got != src {
		t.Fatalf("local path: got %q, want %q", got, src)
	}
}

// TestResolveImageSrcResolvedCacheHit verifies the per-window
// resolved-path cache is consulted before any filesystem call.
// Without that cache, every frame would MkdirAll + 4× Stat per tile.
func TestResolveImageSrcResolvedCacheHit(t *testing.T) {
	url := "http://example.test/" + t.Name() + ".png"
	w := &Window{}

	// Seed the resolved-path cache with a sentinel path that does
	// NOT exist on disk. If ResolveImageSrc consulted the filesystem
	// instead of the cache, it would miss and schedule a download.
	sentinel := "/sentinel/" + t.Name() + ".png"
	resolved := StateMap[string, string](
		w, nsImageResolved, capScroll)
	resolved.Set(url, sentinel)

	if got := ResolveImageSrc(w, url); got != sentinel {
		t.Fatalf("got %q, want %q", got, sentinel)
	}
	// No download should have been scheduled.
	dl := StateMapRead[string, int64](w, nsActiveDownloads)
	if dl != nil && dl.Contains(url) {
		t.Fatal("unexpected download scheduled for cached URL")
	}
}

func TestResolveImageSrcHTTPCached(t *testing.T) {
	url := "http://example.test/" + t.Name() + ".png"
	t.Cleanup(func() { removeCachedFor(url) })

	// Pre-populate cache.
	hash := hashString(url)
	cacheDir := filepath.Join(
		os.TempDir(), "gui_cache", "images")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(cacheDir, strconv.FormatUint(hash, 16))
	cached := base + ".png"
	if err := os.WriteFile(cached, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := &Window{}
	got := ResolveImageSrc(w, url)
	if got != cached {
		t.Fatalf("cached: got %q, want %q", got, cached)
	}
}

func TestResolveImageSrcHTTPNotCached(t *testing.T) {
	resetDownloadSem()
	url := "http://example.invalid/" + t.Name() + ".png"
	t.Cleanup(func() { removeCachedFor(url) })

	// Fetcher that never returns so the download stays in-flight.
	blocked := make(chan struct{})
	t.Cleanup(func() { close(blocked) })
	w := &Window{
		Config: WindowCfg{
			ImageFetcher: func(
				ctx context.Context, _ string,
			) (*http.Response, error) {
				select {
				case <-blocked:
				case <-ctx.Done():
				}
				return nil, context.Canceled
			},
		},
	}

	got := ResolveImageSrc(w, url)
	if got != "" {
		t.Fatalf("not cached: got %q, want \"\"", got)
	}

	dl := StateMap[string, int64](
		w, nsActiveDownloads, capScroll)
	if !dl.Contains(url) {
		t.Fatal("expected active download entry")
	}
}

func TestDefaultImageFetcherUserAgent(t *testing.T) {
	var got string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			got = r.UserAgent()
			mu.Unlock()
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("x"))
		}))
	t.Cleanup(srv.Close)

	resp, err := defaultImageFetcher(t.Context(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	if got == "" {
		t.Fatal("User-Agent missing")
	}
	if !strings.Contains(got, "go-gui/") {
		t.Fatalf("User-Agent %q does not start with go-gui/", got)
	}
}

func TestDownloadImageStatusOK(t *testing.T) {
	resetDownloadSem()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("pngdata"))
		}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	base := filepath.Join(dir, "img")
	w := &Window{}
	w.ctx = t.Context()

	downloadImage(t.Context(), srv.URL, base, 0, w, nil)
	drainQueuedCommands(w)

	if _, err := os.Stat(base + ".png"); err != nil {
		t.Fatalf("expected cached png: %v", err)
	}
	resolved := StateMapRead[string, string](w, nsImageResolved)
	if resolved == nil {
		t.Fatal("expected resolved-path cache populated")
	}
	if p, ok := resolved.Get(srv.URL); !ok || p != base+".png" {
		t.Fatalf("resolved cache got (%q, %v), want (%q, true)",
			p, ok, base+".png")
	}
}

func TestDownloadImageStatus429(t *testing.T) {
	resetDownloadSem()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("error body"))
		}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	base := filepath.Join(dir, "img")
	w := &Window{}
	w.ctx = t.Context()

	downloadImage(t.Context(), srv.URL, base, 0, w, nil)
	drainQueuedCommands(w)

	for _, ext := range []string{".png", ".jpg", ".svg"} {
		if _, err := os.Stat(base + ext); err == nil {
			t.Fatalf("expected no cached file for 429, found %s",
				base+ext)
		}
	}
}

func TestDownloadImageRejectsNonImageContentType(t *testing.T) {
	resetDownloadSem()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>not an image</html>"))
		}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	base := filepath.Join(dir, "img")
	w := &Window{}
	w.ctx = t.Context()

	downloadImage(t.Context(), srv.URL, base, 0, w, nil)
	drainQueuedCommands(w)

	for _, ext := range []string{".png", ".jpg", ".svg"} {
		if _, err := os.Stat(base + ext); err == nil {
			t.Fatalf("non-image body was cached at %s", base+ext)
		}
	}
}

func TestDownloadImageContentLengthTooLarge(t *testing.T) {
	resetDownloadSem()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			// Declare a body larger than maxBytes; do not send it.
			w.Header().Set("Content-Length", "9999")
			w.WriteHeader(http.StatusOK)
		}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	base := filepath.Join(dir, "img")
	w := &Window{}
	w.ctx = t.Context()

	downloadImage(t.Context(), srv.URL, base, 10, w, nil)
	drainQueuedCommands(w)

	for _, ext := range []string{".png", ".jpg", ".svg"} {
		if _, err := os.Stat(base + ext); err == nil {
			t.Fatalf("oversize CL was written to %s", base+ext)
		}
	}
}

func TestDownloadImageBodySizeLimits(t *testing.T) {
	resetDownloadSem()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			switch r.URL.Path {
			case "/exact":
				_, _ = w.Write(make([]byte, 10))
			case "/too-large":
				_, _ = w.Write(make([]byte, 11))
			}
		}))
	t.Cleanup(srv.Close)

	t.Run("exact", func(t *testing.T) {
		dir := t.TempDir()
		base := filepath.Join(dir, "img_exact")
		w := &Window{}
		w.ctx = t.Context()

		downloadImage(t.Context(), srv.URL+"/exact", base, 10, w, nil)
		drainQueuedCommands(w)

		if _, err := os.Stat(base + ".png"); err != nil {
			t.Fatalf("expected exact-sized body to be written, but got error: %v", err)
		}
	})

	t.Run("too large", func(t *testing.T) {
		dir := t.TempDir()
		base := filepath.Join(dir, "img_large")
		w := &Window{}
		w.ctx = t.Context()

		downloadImage(t.Context(), srv.URL+"/too-large", base, 10, w, nil)
		drainQueuedCommands(w)

		for _, ext := range []string{".png", ".jpg", ".svg"} {
			if _, err := os.Stat(base + ext); err == nil {
				t.Fatalf("oversized body was cached at %s", base+ext)
			}
		}
	})
}

func TestDownloadImageFetcherError(t *testing.T) {
	resetDownloadSem()
	fetcher := func(
		_ context.Context, _ string,
	) (*http.Response, error) {
		return nil, errors.New("network is unreachable")
	}

	dir := t.TempDir()
	base := filepath.Join(dir, "img")
	w := &Window{Config: WindowCfg{ImageFetcher: fetcher}}
	w.ctx = t.Context()

	downloadImage(
		t.Context(), "http://example.invalid/x.png", base, 0, w, nil)
	drainQueuedCommands(w)

	for _, ext := range []string{".png", ".jpg", ".svg"} {
		if _, err := os.Stat(base + ext); err == nil {
			t.Fatalf("fetcher error wrote %s", base+ext)
		}
	}
	// Active-download entry must be cleared so next frame retries.
	dl := StateMapRead[string, int64](w, nsActiveDownloads)
	if dl != nil && dl.Contains("http://example.invalid/x.png") {
		t.Fatal("active download entry not cleared after error")
	}
}

func TestDownloadImageSVGExtension(t *testing.T) {
	resetDownloadSem()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write([]byte(`<svg/>`))
		}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	base := filepath.Join(dir, "img")
	w := &Window{}
	w.ctx = t.Context()

	downloadImage(t.Context(), srv.URL, base, 0, w, nil)
	drainQueuedCommands(w)

	if _, err := os.Stat(base + ".svg"); err != nil {
		t.Fatalf("expected svg cached: %v", err)
	}
	resolved := StateMapRead[string, string](w, nsImageResolved)
	if p, _ := resolved.Get(srv.URL); p != base+".svg" {
		t.Fatalf("resolved path %q, want %q", p, base+".svg")
	}
}

func TestGetDownloadSemClampsOversizeConfig(t *testing.T) {
	resetDownloadSem()
	t.Cleanup(resetDownloadSem)
	w := &Window{Config: WindowCfg{
		MaxImageDownloads: 10000,
	}}
	sem := getDownloadSem(w)
	if got := cap(sem); got != maxConcurrentImageDownloads {
		t.Fatalf("semaphore cap %d, want %d",
			got, maxConcurrentImageDownloads)
	}
}

func TestDownloadImageUsesConfigFetcher(t *testing.T) {
	resetDownloadSem()
	var calls atomic.Int32
	fetcher := func(
		ctx context.Context, url string,
	) (*http.Response, error) {
		calls.Add(1)
		req, err := http.NewRequestWithContext(
			ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "custom/1")
		return http.DefaultClient.Do(req)
	}

	var seenUA string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			seenUA = r.UserAgent()
			mu.Unlock()
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("x"))
		}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	base := filepath.Join(dir, "img")
	w := &Window{Config: WindowCfg{ImageFetcher: fetcher}}
	w.ctx = t.Context()

	downloadImage(t.Context(), srv.URL, base, 0, w, nil)
	drainQueuedCommands(w)

	if calls.Load() != 1 {
		t.Fatalf("fetcher calls = %d, want 1", calls.Load())
	}
	mu.Lock()
	defer mu.Unlock()
	if seenUA != "custom/1" {
		t.Fatalf("UA = %q, want custom/1", seenUA)
	}
}

func TestDownloadImageSemaphoreCap(t *testing.T) {
	resetDownloadSem()
	t.Cleanup(resetDownloadSem)

	const maxCap = 2
	const starts = 10

	var inflight atomic.Int32
	var peak atomic.Int32
	release := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			cur := inflight.Add(1)
			for {
				p := peak.Load()
				if cur <= p || peak.CompareAndSwap(p, cur) {
					break
				}
			}
			<-release
			inflight.Add(-1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("x"))
		}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	w := &Window{Config: WindowCfg{MaxImageDownloads: maxCap}}
	w.ctx = t.Context()

	var wg sync.WaitGroup
	for i := range starts {
		wg.Go(func() {
			base := filepath.Join(
				dir, fmt.Sprintf("img%d", i))
			downloadImage(t.Context(), srv.URL, base, 0, w, nil)
		})
	}

	// Let goroutines queue; a few should be in flight.
	deadline := time.Now().Add(2 * time.Second)
	for inflight.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	// Release all.
	close(release)
	wg.Wait()
	drainQueuedCommands(w)

	if got := peak.Load(); got > maxCap {
		t.Fatalf("peak concurrency %d exceeds cap %d", got, maxCap)
	}
}

// A non-nil per-call override fetcher must preempt
// WindowCfg.ImageFetcher for that download only. Routes the two
// fetchers with counters and verifies the override receives the
// request while the config fetcher stays idle.
func TestDownloadImage_OverrideFetcherPreemptsConfig(t *testing.T) {
	resetDownloadSem()
	var cfgHits, overrideHits atomic.Int32
	cfgFetcher := func(
		ctx context.Context, url string,
	) (*http.Response, error) {
		cfgHits.Add(1)
		return defaultImageFetcher(ctx, url)
	}
	overrideFetcher := func(
		ctx context.Context, url string,
	) (*http.Response, error) {
		overrideHits.Add(1)
		return defaultImageFetcher(ctx, url)
	}
	srv := httptest.NewServer(http.HandlerFunc(
		func(rw http.ResponseWriter, _ *http.Request) {
			rw.Header().Set("Content-Type", "image/png")
			_, _ = rw.Write([]byte{
				0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
		}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	base := filepath.Join(dir, "img")
	w := &Window{Config: WindowCfg{ImageFetcher: cfgFetcher}}
	w.ctx = t.Context()

	downloadImage(t.Context(), srv.URL, base, 0, w, overrideFetcher)
	drainQueuedCommands(w)

	if got := overrideHits.Load(); got != 1 {
		t.Errorf("override hits = %d, want 1", got)
	}
	if got := cfgHits.Load(); got != 0 {
		t.Errorf("config fetcher hits = %d, want 0 (override wins)", got)
	}
}

// A nil override falls back to WindowCfg.ImageFetcher, preserving
// backward compatibility for ResolveImageSrc callers.
func TestDownloadImage_NilOverrideFallsBackToConfig(t *testing.T) {
	resetDownloadSem()
	var cfgHits atomic.Int32
	cfgFetcher := func(
		ctx context.Context, url string,
	) (*http.Response, error) {
		cfgHits.Add(1)
		return defaultImageFetcher(ctx, url)
	}
	srv := httptest.NewServer(http.HandlerFunc(
		func(rw http.ResponseWriter, _ *http.Request) {
			rw.Header().Set("Content-Type", "image/png")
			_, _ = rw.Write([]byte{
				0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
		}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	base := filepath.Join(dir, "img")
	w := &Window{Config: WindowCfg{ImageFetcher: cfgFetcher}}
	w.ctx = t.Context()

	downloadImage(t.Context(), srv.URL, base, 0, w, nil)
	drainQueuedCommands(w)

	if got := cfgHits.Load(); got != 1 {
		t.Errorf("config fetcher hits = %d, want 1 on nil override", got)
	}
}

// drainQueuedCommands runs any commands the downloader queued so
// the active-downloads map reflects completion.
func drainQueuedCommands(w *Window) {
	w.commandsMu.Lock()
	cmds := w.commands
	w.commands = nil
	w.commandsMu.Unlock()
	for _, c := range cmds {
		if c.windowFn != nil {
			c.windowFn(w)
		}
	}
}
