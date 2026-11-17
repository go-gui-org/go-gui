package gui

// keep_awake.go — keeping the display on (issue #1172).
//
// Each KeepAwake call is a token. gui keeps the tokens per window and
// asks the OS for one request for the whole app, held while some window
// with a token is on screen: IsVisible() && !IsOccluded(). The platform
// hears only the changes of that state, through
// NativePlatform.SetKeepAwake. See docs/specs/keep-awake.md.

import (
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

// keepAwakeMaxChars caps the strings handed to the platform. It is the
// length macOS allows for the name of a power assertion, the shortest
// limit among the platforms.
const keepAwakeMaxChars = 128

// keepAwakeDefaultReason stands in for an empty reason. gnome-session
// refuses an inhibit without one.
const keepAwakeDefaultReason = "Keeping the display awake"

// keepAwakeDefaultApp names an app that has no manifest and no title.
const keepAwakeDefaultApp = "go-gui"

// keepAwakeToken is one KeepAwake call.
type keepAwakeToken struct {
	reason string
	// released is set by the first call of the release function, or
	// by WindowCleanup, so a second call does nothing.
	released atomic.Bool
}

// keepAwakeHolder is a window that holds at least one token.
type keepAwakeHolder struct {
	w      *Window
	tokens []*keepAwakeToken // oldest first
}

// keepAwakeState is the keep-awake state of the whole app.
var keepAwakeState struct {
	// mu guards holders. It is never held across a platform call, so
	// KeepAwake and release never wait for the OS.
	mu sync.Mutex
	// holders lists the windows with tokens, in the order each took
	// its first one.
	holders []keepAwakeHolder

	// applyMu makes one reconcile run at a time, so the platform
	// calls of two never interleave. applied is what the platform was
	// last asked for, and whether it said yes.
	applyMu sync.Mutex
	applied bool
}

// KeepAwake keeps the display on while this window is on screen, for
// example while a video plays. It returns release, which ends the
// request: call it when playback stops. Calling release again does
// nothing.
//
// Each call is its own request. Two players in one window each hold
// one, and the display can sleep again only once both have released
// theirs. The OS gets one request for the whole app, held while some
// window with a request is visible and not occluded (IsVisible,
// IsOccluded). A hidden or minimized window lets the display sleep
// until it is shown again. Closing a window ends its requests. While
// the display stays on, the computer does not go to sleep either.
//
// reason says why, in words for people. The OS shows it where it lists
// what keeps the computer awake (powercfg /requests on Windows). An
// empty reason reads "Keeping the display awake". The OS gets one
// reason: that of the oldest request at the moment the display starts
// being kept on. It does not change while the display stays on, also
// when that request ends and a later one keeps the display on.
//
// Safe from any goroutine, also from AmendLayout under the frame lock:
// the OS is asked on the UI thread, at the start of the next frame. On
// a nil or destroyed window it does nothing and returns a release that
// does nothing.
func (w *Window) KeepAwake(reason string) (release func()) {
	if w == nil {
		return func() {}
	}
	tok := &keepAwakeToken{reason: reason}
	if !keepAwakeAdd(w, tok) {
		return func() {}
	}
	w.QueueCommand(reconcileKeepAwakeCmd)
	return func() {
		if tok.released.Swap(true) {
			return
		}
		if keepAwakeRemove(w, tok) {
			w.QueueCommand(reconcileKeepAwakeCmd)
		}
	}
}

// reconcileKeepAwakeCmd is the queued half of KeepAwake and release. A
// function value, not a closure, so queuing it allocates nothing.
func reconcileKeepAwakeCmd(w *Window) { w.reconcileKeepAwake() }

// keepAwakeAdd records tok on w. It reports false when w is destroyed.
func keepAwakeAdd(w *Window, tok *keepAwakeToken) bool {
	s := &keepAwakeState
	s.mu.Lock()
	defer s.mu.Unlock()
	// Read under mu. WindowCleanup marks the window destroyed before it
	// drops the window's tokens under mu, so a token added here is
	// either dropped by that cleanup or not added at all.
	if w.destroyed.Load() {
		return false
	}
	for i := range s.holders {
		if s.holders[i].w == w {
			s.holders[i].tokens = append(s.holders[i].tokens, tok)
			return true
		}
	}
	s.holders = append(s.holders, keepAwakeHolder{
		w:      w,
		tokens: []*keepAwakeToken{tok},
	})
	return true
}

// keepAwakeRemove drops tok from w. It reports whether w held it.
func keepAwakeRemove(w *Window, tok *keepAwakeToken) bool {
	s := &keepAwakeState
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.holders {
		h := &s.holders[i]
		if h.w != w {
			continue
		}
		j := slices.Index(h.tokens, tok)
		if j < 0 {
			return false
		}
		h.tokens = slices.Delete(h.tokens, j, j+1)
		if len(h.tokens) == 0 {
			s.holders = slices.Delete(s.holders, i, i+1)
		}
		return true
	}
	return false
}

// keepAwakeDropWindow drops every token w holds.
func keepAwakeDropWindow(w *Window) {
	s := &keepAwakeState
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.holders {
		if s.holders[i].w != w {
			continue
		}
		for _, tok := range s.holders[i].tokens {
			tok.released.Store(true)
		}
		s.holders = slices.Delete(s.holders, i, i+1)
		return
	}
}

// keepAwakeOnScreen returns the first window with tokens that is on
// screen, and the reason of its oldest token. nil when there is none:
// then the display may sleep.
func keepAwakeOnScreen() (*Window, string) {
	s := &keepAwakeState
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.holders {
		h := &s.holders[i]
		if h.w.IsVisible() && !h.w.IsOccluded() {
			return h.w, h.tokens[0].reason
		}
	}
	return nil, ""
}

// reconcileKeepAwake asks the platform to begin or end the OS request
// when the state of the whole app differs from what it last asked for.
// UI thread. It runs after a token is taken or released, when a
// window's visibility changes, when a window is destroyed, and when the
// backend attaches. Without a platform (tests, or before the backend
// attaches) it does nothing, and a later run asks.
func (w *Window) reconcileKeepAwake() {
	np := w.nativePlatform
	if np == nil {
		return
	}
	s := &keepAwakeState
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	holder, reason := keepAwakeOnScreen()
	want := holder != nil
	if want == s.applied {
		return
	}
	var app string
	if want {
		app, reason = keepAwakeAppName(holder), keepAwakeReason(reason)
	}
	err := np.SetKeepAwake(want, app, reason)
	// A failed on leaves applied false, so the next change asks again.
	// A failed off leaves it false too: there is nothing more to ask.
	s.applied = want && err == nil
	if err != nil {
		w.debugWarn(debugCheckKeepAwake, err.Error(),
			"window: keep awake: %v", err)
	}
}

// cleanupKeepAwake drops a destroyed window's tokens and, when that
// ends the OS request, ends it now. From WindowCleanup, while the
// platform is still attached.
//
// It reconciles even when the window held no token: a release made
// just before the close queued its reconcile on this window, and the
// queued commands of a destroyed window never run.
func (w *Window) cleanupKeepAwake() {
	keepAwakeDropWindow(w)
	w.reconcileKeepAwake()
}

// keepAwakeAppName names the app to the platform: the manifest's ID,
// which is the Linux .desktop file name GNOME looks the app up by, then
// the manifest's name, then the window title, then "go-gui".
func keepAwakeAppName(w *Window) string {
	for _, s := range [...]string{
		w.Config.AppInfo.ID, w.Config.AppInfo.Name, w.Config.Title,
	} {
		if t := keepAwakeText(s); t != "" {
			return t
		}
	}
	return keepAwakeDefaultApp
}

// keepAwakeReason is the reason handed to the platform, with the
// default for an empty one.
func keepAwakeReason(reason string) string {
	if t := keepAwakeText(reason); t != "" {
		return t
	}
	return keepAwakeDefaultReason
}

// keepAwakeText makes s safe for every platform: valid UTF-8, which
// D-Bus requires; no NUL, which ends a C string early; no space at
// either end; at most keepAwakeMaxChars characters.
func keepAwakeText(s string) string {
	s = strings.ToValidUTF8(s, "�")
	if strings.IndexByte(s, 0) >= 0 {
		s = strings.ReplaceAll(s, "\x00", "")
	}
	s = strings.TrimSpace(s)
	n := 0
	for i := range s {
		if n == keepAwakeMaxChars {
			return strings.TrimSpace(s[:i])
		}
		n++
	}
	return s
}
