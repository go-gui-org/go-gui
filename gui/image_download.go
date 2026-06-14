package gui

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ImageFetcher is the signature of a function that fetches a remote
// image. Implementations typically set a descriptive User-Agent, add
// auth headers, or route through a shared http.Client. Must be safe
// for concurrent use. Return a non-nil *http.Response whose Body the
// caller will close.
type ImageFetcher func(
	ctx context.Context, url string,
) (*http.Response, error)

// defaultMaxImageDownloads caps concurrent HTTP image downloads
// when WindowCfg.MaxImageDownloads is zero. Six matches OSM's
// informal guidance for well-behaved map clients.
const defaultMaxImageDownloads = 6

// maxConcurrentImageDownloads caps the user-configurable
// MaxImageDownloads so a bad WindowCfg cannot create a channel of
// unbounded capacity.
const maxConcurrentImageDownloads = 64

// defaultMaxDownloadBytes is the download size limit when
// WindowCfg.MaxImageBytes is not set.
const defaultMaxDownloadBytes = int64(16 * 1024 * 1024)

// maxImageURLLen bounds the length of a remote image URL accepted
// by ResolveImageSrc. Real-world HTTP clients and servers already
// reject URLs beyond this length; enforcing it up front prevents
// an attacker-controlled caller from forcing the hash/Sprintf
// allocation path to spend O(n) on a gigantic string.
const maxImageURLLen = 4096

// imageDownloadSem bounds concurrent HTTP image downloads across
// the process. Initialized on first use from the Window's
// MaxImageDownloads. First-writer wins — subsequent windows do not
// resize. Rationale: the limit targets outbound IP rate (OSM
// policy), which is a process-wide concern.
var (
	imageDownloadSemMu sync.Mutex
	imageDownloadSem   chan struct{}
)

// imageCacheDir paths the remote image cache directory. Kept as a
// var rather than a const because it depends on runtime os.TempDir().
var imageCacheDir = filepath.Join(
	os.TempDir(), "gui_cache", "images")

// imageCacheExts lists the extensions findCachedImage probes.
// Package-level to avoid a per-call slice alloc on miss.
var imageCacheExts = []string{".png", ".jpg", ".jpeg", ".svg"}

// imageCacheDirOnce gates MkdirAll on imageCacheDir so the syscall
// runs once per process instead of once per frame per tile.
var (
	imageCacheDirOnce sync.Once
	imageCacheDirErr  error
)

func ensureImageCacheDir() error {
	imageCacheDirOnce.Do(func() {
		imageCacheDirErr = os.MkdirAll(imageCacheDir, 0o750)
	})
	return imageCacheDirErr
}

// isHTTPURL reports whether src is an http:// or https:// URL.
func isHTTPURL(src string) bool {
	return strings.HasPrefix(src, "http://") ||
		strings.HasPrefix(src, "https://")
}

// isDataURL reports whether src is an inline data URL.
func isDataURL(src string) bool {
	return strings.HasPrefix(src, "data:")
}

// urlDigest returns a short hash prefix for a URL, suitable for log
// correlation without leaking the full URL (which may contain query-
// string tokens, signed parameters, or private resource paths).
func urlDigest(url string) string {
	h := hashString(url)
	return "url=" + strconv.FormatUint(h, 16)[:8]
}

// getDownloadSem returns the lazily-initialized download semaphore.
// The first caller's WindowCfg sizes the channel. Values outside
// [1, maxConcurrentImageDownloads] are clamped.
func getDownloadSem(w *Window) chan struct{} {
	imageDownloadSemMu.Lock()
	defer imageDownloadSemMu.Unlock()
	if imageDownloadSem == nil {
		n := w.Config.MaxImageDownloads
		if n <= 0 {
			n = defaultMaxImageDownloads
		}
		if n > maxConcurrentImageDownloads {
			n = maxConcurrentImageDownloads
		}
		imageDownloadSem = make(chan struct{}, n)
	}
	return imageDownloadSem
}

// resetDownloadSem clears the process-wide semaphore so the next
// getDownloadSem call re-initializes from config. Test-only.
func resetDownloadSem() {
	imageDownloadSemMu.Lock()
	defer imageDownloadSemMu.Unlock()
	imageDownloadSem = nil
}

// defaultImageFetcher issues the GET used when
// WindowCfg.ImageFetcher is nil. It sets a descriptive User-Agent
// so tile/image providers can identify and rate-limit go-gui
// traffic per their policies (e.g. OSM requires a specific UA).
func defaultImageFetcher(
	ctx context.Context, url string,
) (*http.Response, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "go-gui/"+Version)
	return http.DefaultClient.Do(req)
}

// ResolveImageSrc returns a local filesystem path for src. Rules:
//   - empty → ""
//   - "data:" URL → returned unchanged (backend handles it)
//   - non-http path → returned unchanged (treated as local path)
//   - http/https already cached on disk → cache path
//   - http/https not cached → async download scheduled; returns ""
//
// When "" is returned for an http URL the download is in flight;
// callers should skip the emit for this frame. The window is
// redrawn automatically when the download completes.
//
// Remote downloads land in filepath.Join(os.TempDir(),
// "gui_cache", "images"). If WindowCfg.AllowedImageRoots is set,
// that path must be included or the backend will reject the
// resolved cache file.
//
// Remote fetches use WindowCfg.ImageFetcher; use
// ResolveImageSrcWithFetcher to override per call.
func ResolveImageSrc(w *Window, src string) string {
	return ResolveImageSrcWithFetcher(w, src, nil)
}

// ResolveImageSrcWithFetcher is ResolveImageSrc with a per-call
// fetcher override. nil fetcher falls back to WindowCfg.ImageFetcher,
// matching ResolveImageSrc exactly. The fetcher is consulted only on
// a cold download; cache hits and already-in-flight URLs ignore it
// (see DrawCanvasImageEntry.Fetcher on the URL-keyed dedup limit).
func ResolveImageSrcWithFetcher(
	w *Window, src string, fetcher ImageFetcher,
) string {
	if src == "" {
		return ""
	}
	if isDataURL(src) {
		return src
	}
	if !isHTTPURL(src) {
		return src
	}
	if len(src) > maxImageURLLen {
		log.Printf("image: URL exceeds %d bytes, skipping",
			maxImageURLLen)
		return ""
	}

	// Hot path: per-window URL→resolved-path cache. Avoids
	// MkdirAll + Stat every frame once the tile is known.
	resolved := StateMap[string, string](
		w, nsImageResolved, capImageCache)
	if p, ok := resolved.Get(src); ok {
		return p
	}

	if err := ensureImageCacheDir(); err != nil {
		log.Printf("image: mkdir failed: %v", err)
		return ""
	}
	basePath := filepath.Join(
		imageCacheDir,
		strconv.FormatUint(hashString(src), 16))
	if p := findCachedImage(basePath); p != "" {
		resolved.Set(src, p)
		return p
	}
	downloads := StateMap[string, int64](
		w, nsActiveDownloads, capScroll)
	if !downloads.Contains(src) {
		downloads.Set(src, time.Now().Unix())
		go downloadImage(
			w.Ctx(), src, basePath,
			w.Config.MaxImageBytes, w, fetcher)
	}
	return ""
}

// downloadImage fetches a remote image to a local cache path.
// wCtx cancellation stops the download. maxBytes caps the body
// size (0 selects defaultMaxDownloadBytes). override, when non-nil,
// preempts WindowCfg.ImageFetcher for this download only; nil
// selects the window default, then defaultImageFetcher.
func downloadImage(
	wCtx context.Context, url, basePath string,
	maxBytes int64, w *Window, override ImageFetcher,
) {
	maxSize := maxBytes
	if maxSize <= 0 {
		maxSize = defaultMaxDownloadBytes
	}

	sem := getDownloadSem(w)
	select {
	case sem <- struct{}{}:
	case <-wCtx.Done():
		removeDownload(url, w)
		return
	}
	defer func() { <-sem }()

	ctx, cancel := context.WithTimeout(wCtx, 30*time.Second)
	defer cancel()

	fetcher := override
	if fetcher == nil {
		fetcher = w.Config.ImageFetcher
	}
	if fetcher == nil {
		fetcher = defaultImageFetcher
	}

	resp, err := fetcher(ctx, url)
	if err != nil {
		log.Printf("image download: %v", err)
		removeDownload(url, w)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Printf("image download: %s: HTTP %d",
			urlDigest(url), resp.StatusCode)
		removeDownload(url, w)
		return
	}

	if resp.ContentLength > maxSize {
		log.Printf("image too large (%d bytes): %s",
			resp.ContentLength, urlDigest(url))
		removeDownload(url, w)
		return
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		log.Printf("invalid content type for image: %s",
			urlDigest(url))
		removeDownload(url, w)
		return
	}

	ext := contentTypeToExt(ct)
	path := basePath + ext

	f, err := os.Create(path)
	if err != nil {
		log.Printf("image download create: %v", err)
		removeDownload(url, w)
		return
	}
	written, err := io.Copy(f, io.LimitReader(resp.Body, maxSize+1))
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(path)
		if closeErr != nil {
			log.Printf("image download write: %v; close: %v",
				err, closeErr)
		} else {
			log.Printf("image download write: %v", err)
		}
		removeDownload(url, w)
		return
	}
	if closeErr != nil {
		_ = os.Remove(path)
		log.Printf("image download close: %v", closeErr)
		removeDownload(url, w)
		return
	}
	if written > maxSize {
		_ = os.Remove(path)
		log.Printf("image download body exceeds limit: %s",
			urlDigest(url))
		removeDownload(url, w)
		return
	}

	w.QueueCommand(func(w *Window) {
		dl := StateMap[string, int64](
			w, nsActiveDownloads, capScroll)
		dl.Delete(url)
		resolved := StateMap[string, string](
			w, nsImageResolved, capImageCache)
		resolved.Set(url, path)
		w.UpdateWindow()
	})
}

func removeDownload(url string, w *Window) {
	w.QueueCommand(func(w *Window) {
		dl := StateMap[string, int64](
			w, nsActiveDownloads, capScroll)
		dl.Delete(url)
	})
}

func contentTypeToExt(ct string) string {
	switch {
	case strings.HasPrefix(ct, "image/svg+xml"):
		return ".svg"
	case strings.HasPrefix(ct, "image/png"):
		return ".png"
	case strings.HasPrefix(ct, "image/jpeg"):
		return ".jpg"
	default:
		return ".png"
	}
}

func findCachedImage(basePath string) string {
	for _, ext := range imageCacheExts {
		candidate := basePath + ext
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
