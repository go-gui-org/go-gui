//go:build windows && !js

package gl

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/go-gui-org/go-gui/gui"
)

// The process renderer and the first-window fallback on Windows (#1310,
// phase 3).
//
// With GOGUI_RENDERER unset, the first window tries GL. If its pixel format,
// context or make-current step (createContext), or the binding of the GL
// functions, fails, the attempt is discarded with plat.destroy and the window
// reopens on the CPU renderer, and every later window of the process opens
// there with no GL attempt. An opengl32.dll that cannot be loaded falls back
// the same way, before the GL attempt opens a window (#1348). If the first
// window comes up on GL, later windows open on GL, and one whose bring-up
// fails returns the classified error. A failure to
// build the built-in pipelines does not fall back: that is a driver-specific
// shader bug, which a fallback would hide behind a log line.
//
// There is no probe window and no version or renderer-name test: the first
// window's own bring-up is the probe, so a machine whose GL comes up, a Mesa
// beside the exe included, makes the same calls as before the fallback
// existed. GOGUI_RENDERER=gl, or a value that is neither gl nor soft, keeps
// the classified error with no fallback; GOGUI_RENDERER=soft opens every
// window on the CPU renderer with no GL at all. Both are read per window and
// neither reads or sets the process renderer.

// win32Renderer is what a process with GOGUI_RENDERER unset draws with.
type win32Renderer uint8

const (
	// win32RendererUndecided: no window has opened yet with the variable
	// unset. A first window that fails for any other reason than a fallback
	// (no window, a pipeline failure, soft failing too) leaves it undecided.
	win32RendererUndecided win32Renderer = iota
	win32RendererGL
	win32RendererSoft
)

// processRenderer is set by the deciding window: the first window that opens
// with GOGUI_RENDERER unset, or the next one after a first window that failed
// without choosing. mu guards chosen and deciding, and is held only to read
// or write them, never across a window's bring-up, so no bring-up waits on
// another (openAutoWin32). deciding is true while a window whose bring-up
// will set chosen is coming up.
var processRenderer struct {
	mu       sync.Mutex
	chosen   win32Renderer
	deciding bool
}

// openWin32Backend is New after the thread lock: it opens w on the renderer
// that env and the process renderer pick, and then publishes the degradation
// reports of the attempt that opened the window.
func openWin32Backend(w *gui.Window, env rendererEnv) (*Backend, error) {
	var rep windowReports
	var b *Backend
	var err error
	switch env {
	case rendererEnvSoft:
		b, err = newSoftWin32(w, &rep, nil)
	case rendererEnvGL:
		b, _, err = newGLWin32(w, &rep, nil)
	default:
		b, err = openAutoWin32(w, &rep)
	}
	if err != nil {
		return nil, err
	}
	rep.publish(w)
	return b, nil
}

// openAutoWin32 opens w with GOGUI_RENDERER unset: on the renderer the
// process chose, or, while none is chosen, on GL with the fallback to the
// CPU renderer. rep holds the reports of the attempt that returns.
//
// Ordering. processRenderer.mu is held only to read or write the state, so
// a window's bring-up never waits on another, whatever application code the
// bring-up reaches (a font warning through an application's log writer, an
// OnSystemAppearance hook when the window attaches) and whatever that code
// opens. A bad icon's warning is not among them: it is in rep, which
// openWin32Backend publishes after this returns, with the choice made. A
// window whose process renderer is chosen opens there. A
// window that finds none chosen and none deciding marks the state deciding
// and is the deciding window: each of its attempts calls commit just before
// the window attaches to w, once nothing after it can fail, and commit sets
// the choice; the soft one then logs the fallback line. So a window opened
// from the attach (an OnSystemAppearance hook), on this thread or another,
// finds the choice made. The deciding window clears deciding on every
// return, so after a failure without a choice (a plain error, a pipeline
// failure, the soft setup failing too) the next window decides.
//
// A window that finds another bring-up deciding (one opened from code that
// bring-up reached before its commit, or from another thread at the same
// moment) makes its own attempt, GL with the same fallback, and neither
// sets the choice nor waits. In principle it could end on another renderer
// than the deciding window, on a machine whose two attempts differ; no
// machine has shown that.
func openAutoWin32(w *gui.Window, rep *windowReports) (*Backend, error) {
	processRenderer.mu.Lock()
	chosen := processRenderer.chosen
	deciding := chosen == win32RendererUndecided && !processRenderer.deciding
	if deciding {
		processRenderer.deciding = true
	}
	processRenderer.mu.Unlock()
	switch chosen {
	case win32RendererSoft:
		return newSoftWin32(w, rep, nil)
	case win32RendererGL:
		b, _, err := newGLWin32(w, rep, nil)
		return b, err
	}

	if deciding {
		defer func() {
			processRenderer.mu.Lock()
			processRenderer.deciding = false
			processRenderer.mu.Unlock()
		}()
	}
	committed := false
	commit := func(r win32Renderer) {
		committed = true
		if deciding {
			processRenderer.mu.Lock()
			processRenderer.chosen = r
			processRenderer.mu.Unlock()
		}
	}

	b, fallback, err := newGLWin32(w, rep, func() { commit(win32RendererGL) })
	if err == nil {
		return b, nil
	}
	gerr, ok := errors.AsType[*gpuContextError](err)
	// committed: the attempt committed GL, so it cannot fall back. No step
	// after the commit fails today; this keeps a future one from mixing
	// renderers.
	if !fallback || !ok || committed {
		return nil, err
	}
	// The GL attempt's window is gone and published nothing; its reports
	// described that window.
	*rep = windowReports{}
	b, serr := newSoftWin32(w, rep, func() {
		commit(win32RendererSoft)
		log.Print(fallbackLine(gerr))
	})
	if serr != nil {
		// Neither renderer can run. The GPU error comes first, so the error
		// still matches ErrNoGPUContext, and it does not advise
		// GOGUI_RENDERER=soft, which just failed. A failure of the soft
		// window itself stays a plain error, as on GL.
		if sie, isSoft := errors.AsType[*softInitError](serr); isSoft {
			return nil, fmt.Errorf("%w (CPU renderer fallback: %w)",
				newGPUContextErrorNoSoft(gerr.op, gerr.cause), sie.err)
		}
		return nil, serr
	}
	return b, nil
}

// fallbackLine is the one line a fallback logs: the step that failed and its
// cause, which for a binding failure names GL_VERSION and GL_RENDERER (a
// refused context has none to read), how to get the error instead, and what
// the CPU renderer does not draw.
func fallbackLine(gerr *gpuContextError) string {
	return fmt.Sprintf("gl: %s failed (%v); drawing with the CPU renderer "+
		"(GOGUI_RENDERER=gl reports the error instead; custom shaders are not drawn)",
		gerr.op, gerr.cause)
}

// windowReports holds the degradation reports of one window attempt: what
// enableWindowTransparency, reportSoftTransparency, applyWindowOpacity,
// setWindowIcon and applyVSync found before it was known whether the
// attempt's window would be kept.
// openWin32Backend publishes the reports of the attempt that opened the
// window only, so a GL attempt that fell back reports nothing about a window
// that no longer exists.
type windowReports struct {
	transparency, opacity, vsync []string
	icon                         []error
}

func (r *windowReports) addTransparency(reason string) {
	r.transparency = append(r.transparency, reason)
}

func (r *windowReports) addOpacity(reason string) {
	r.opacity = append(r.opacity, reason)
}

func (r *windowReports) addVSync(reason string) {
	r.vsync = append(r.vsync, reason)
}

func (r *windowReports) addIcon(err error) {
	r.icon = append(r.icon, err)
}

// publish hands the reports to w, in the order the window setup made them.
// A bad icon is not a gui.Debug finding: it goes to the log, so it shows
// with debug off. A New that fails publishes nothing, the icon warning
// included: the error is what the app sees, and the icon is reported by the
// first window that opens.
func (r *windowReports) publish(w *gui.Window) {
	for _, reason := range r.transparency {
		w.DebugWindowTransparency(reason)
	}
	for _, reason := range r.opacity {
		w.DebugWindowOpacity(reason)
	}
	for _, err := range r.icon {
		log.Printf("gl: window icon: %v", err)
	}
	for _, reason := range r.vsync {
		w.DebugWindowVSync(reason)
	}
}
