//go:build js && wasm

package web

import (
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"syscall/js"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// nativePlatform implements gui.NativePlatform for wasm.
type nativePlatform struct {
	doc      js.Value
	canvas   js.Value
	a11y     a11yState
	imeInput js.Value
}

// --- URI ---

// maxOpenURILen caps the raw URI length, mirroring nativehost's
// limit for the desktop backends.
const maxOpenURILen = 8192

// validateOpenURI checks that raw is a valid absolute URI whose
// scheme is in the allowlist (http, https, mailto). It mirrors
// nativehost.ValidateOpenURI, which this package cannot import:
// nativehost pulls in os/exec, which does not build for js/wasm.
func validateOpenURI(raw string) error {
	if len(raw) > maxOpenURILen {
		return fmt.Errorf("web: URI too long: %d bytes (max %d)",
			len(raw), maxOpenURILen)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("web: invalid URI: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto":
		return nil
	default:
		return fmt.Errorf("web: blocked URI scheme in %q", raw)
	}
}

func (n *nativePlatform) OpenURI(uri string) error {
	if err := validateOpenURI(uri); err != nil {
		return err
	}
	w := js.Global().Call("open", uri, "_blank")
	if w.IsNull() || w.IsUndefined() {
		return fmt.Errorf("web: popup blocked for %q", uri)
	}
	return nil
}

// --- File dialogs ---

func (n *nativePlatform) ShowOpenDialog(
	_, _ string, extensions []string, allowMultiple bool,
) gui.PlatformDialogResult {
	input := n.doc.Call("createElement", "input")
	input.Set("type", "file")
	if allowMultiple {
		input.Set("multiple", true)
	}
	if len(extensions) > 0 {
		input.Set("accept", dotExtensions(extensions))
	}

	// Hidden in DOM so .click() works in all browsers.
	input.Get("style").Set("display", "none")
	n.doc.Get("body").Call("appendChild", input)

	ch := make(chan gui.PlatformDialogResult, 1)

	// Both sends are non-blocking: "change" and "cancel" can each
	// fire, and a blocking send on the filled buffer would wedge
	// the JS event loop forever.
	changeCb := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		files := input.Get("files")
		count := files.Length()
		if count == 0 {
			select {
			case ch <- gui.PlatformDialogResult{
				Status: gui.DialogCancel,
			}:
			default:
			}
			return nil
		}
		paths := make([]gui.PlatformPath, count)
		for i := range count {
			paths[i] = gui.PlatformPath{
				Path: files.Index(i).Get("name").String(),
			}
		}
		select {
		case ch <- gui.PlatformDialogResult{
			Status: gui.DialogOK,
			Paths:  paths,
		}:
		default:
		}
		return nil
	})

	cancelCb := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		select {
		case ch <- gui.PlatformDialogResult{
			Status: gui.DialogCancel,
		}:
		default:
		}
		return nil
	})

	input.Call("addEventListener", "change", changeCb)
	input.Call("addEventListener", "cancel", cancelCb)
	// One defer, in this order: the receive below blocks until the
	// user picks or dismisses, and a JS exception in between must
	// not leak the element or the funcs. The element is detached
	// first — calling a released js.Func panics the wasm instance,
	// so no listener may outlive its func.
	defer func() {
		input.Call("remove")
		changeCb.Release()
		cancelCb.Release()
	}()
	input.Call("click")

	return <-ch
}

func (n *nativePlatform) ShowSaveDialog(
	title, _, defaultName, defaultExt string,
	extensions []string, _ bool,
) gui.PlatformDialogResult {
	// Try File System Access API.
	picker := js.Global().Get("showSaveFilePicker")
	if !picker.IsUndefined() {
		return n.saveFilePicker(
			title, defaultName, defaultExt, extensions)
	}

	// Fallback: return the suggested filename.
	name := defaultName
	if name == "" {
		name = "download"
	}
	if defaultExt != "" {
		name += "." + defaultExt
	}
	return gui.PlatformDialogResult{
		Status: gui.DialogOK,
		Paths:  []gui.PlatformPath{{Path: name}},
	}
}

func (n *nativePlatform) saveFilePicker(
	title, defaultName, defaultExt string,
	extensions []string,
) (result gui.PlatformDialogResult) {
	// showSaveFilePicker throws synchronously when called without a
	// user gesture. Convert the JS exception into a result instead
	// of panicking the wasm instance.
	defer func() {
		if r := recover(); r != nil {
			result = gui.PlatformDialogResult{
				Status:       gui.DialogError,
				ErrorCode:    "unsupported",
				ErrorMessage: fmt.Sprintf("save picker unavailable: %v", r),
			}
		}
	}()
	opts := jsObject()
	if defaultName != "" {
		suggested := defaultName
		if defaultExt != "" {
			suggested += "." + defaultExt
		}
		opts.Set("suggestedName", suggested)
	}
	if len(extensions) > 0 {
		types := js.Global().Get("Array").New()
		desc := jsObject()
		accept := jsObject()
		exts := js.Global().Get("Array").New()
		for _, ext := range extensions {
			exts.Call("push", "."+ext)
		}
		accept.Set("application/octet-stream", exts)
		desc.Set("accept", accept)
		if title != "" {
			desc.Set("description", title)
		}
		types.Call("push", desc)
		opts.Set("types", types)
	}

	ch := make(chan gui.PlatformDialogResult, 1)
	promise := js.Global().Call("showSaveFilePicker", opts)

	thenCb := js.FuncOf(func(_ js.Value, args []js.Value) any {
		name := args[0].Get("name").String()
		ch <- gui.PlatformDialogResult{
			Status: gui.DialogOK,
			Paths:  []gui.PlatformPath{{Path: name}},
		}
		return nil
	})
	catchCb := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		ch <- gui.PlatformDialogResult{Status: gui.DialogCancel}
		return nil
	})

	promise.Call("then", thenCb).Call("catch", catchCb)
	defer thenCb.Release()
	defer catchCb.Release()
	return <-ch
}

func (n *nativePlatform) ShowFolderDialog(_, _ string) (result gui.PlatformDialogResult) {
	// showDirectoryPicker throws synchronously without a user
	// gesture; convert the JS exception into a result instead of
	// panicking the wasm instance.
	defer func() {
		if r := recover(); r != nil {
			result = gui.PlatformDialogResult{
				Status:       gui.DialogError,
				ErrorCode:    "unsupported",
				ErrorMessage: fmt.Sprintf("folder picker unavailable: %v", r),
			}
		}
	}()
	picker := js.Global().Get("showDirectoryPicker")
	if picker.IsUndefined() {
		return gui.PlatformDialogResult{
			Status:       gui.DialogError,
			ErrorCode:    "unsupported",
			ErrorMessage: "folder picker not available in this browser",
		}
	}

	ch := make(chan gui.PlatformDialogResult, 1)
	promise := js.Global().Call("showDirectoryPicker")

	thenCb := js.FuncOf(func(_ js.Value, args []js.Value) any {
		name := args[0].Get("name").String()
		ch <- gui.PlatformDialogResult{
			Status: gui.DialogOK,
			Paths:  []gui.PlatformPath{{Path: name}},
		}
		return nil
	})
	catchCb := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		ch <- gui.PlatformDialogResult{Status: gui.DialogCancel}
		return nil
	})

	promise.Call("then", thenCb).Call("catch", catchCb)
	defer thenCb.Release()
	defer catchCb.Release()
	result = <-ch
	return result
}

// --- Alert dialogs ---

func (n *nativePlatform) ShowMessageDialog(
	title, body string, _ gui.NativeAlertLevel,
) gui.NativeAlertResult {
	msg := title
	if body != "" {
		msg += "\n\n" + body
	}
	js.Global().Call("alert", msg)
	return gui.NativeAlertResult{Status: gui.DialogOK}
}

func (n *nativePlatform) ShowConfirmDialog(
	title, body string, _ gui.NativeAlertLevel,
) gui.NativeAlertResult {
	msg := title
	if body != "" {
		msg += "\n\n" + body
	}
	if js.Global().Call("confirm", msg).Bool() {
		return gui.NativeAlertResult{Status: gui.DialogOK}
	}
	return gui.NativeAlertResult{Status: gui.DialogCancel}
}

func (n *nativePlatform) ShowSaveDiscardDialog(
	_, _ string, _ gui.NativeAlertLevel,
) gui.NativeAlertResult {
	return gui.NativeAlertResult{
		Status:       gui.DialogError,
		ErrorCode:    "unsupported",
		ErrorMessage: "3-button save dialog not available on web",
	}
}

// --- Notifications ---

func (n *nativePlatform) SendNotification(
	title, body string,
) gui.NativeNotificationResult {
	notifClass := js.Global().Get("Notification")
	if notifClass.IsUndefined() {
		return gui.NativeNotificationResult{
			Status:       gui.NotificationError,
			ErrorCode:    "unsupported",
			ErrorMessage: "Notification API not available",
		}
	}

	perm := notifClass.Get("permission").String()
	if perm == "default" {
		ch := make(chan string, 1)
		thenCb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			ch <- args[0].String()
			return nil
		})
		catchCb := js.FuncOf(func(_ js.Value, _ []js.Value) any {
			ch <- "denied"
			return nil
		})
		notifClass.Call("requestPermission").
			Call("then", thenCb).Call("catch", catchCb)
		perm = <-ch
		thenCb.Release()
		catchCb.Release()
	}

	switch perm {
	case "granted":
		opts := jsObject()
		opts.Set("body", body)
		notifClass.New(title, opts)
		return gui.NativeNotificationResult{
			Status: gui.NotificationOK,
		}
	case "denied":
		return gui.NativeNotificationResult{
			Status:    gui.NotificationDenied,
			ErrorCode: "denied",
		}
	default:
		return gui.NativeNotificationResult{
			Status:       gui.NotificationError,
			ErrorCode:    "unknown",
			ErrorMessage: "permission: " + perm,
		}
	}
}

// --- Print ---

func (n *nativePlatform) ShowPrintDialog(
	_ gui.NativePrintParams,
) gui.PrintRunResult {
	// toDataURL throws when the canvas is tainted (cross-origin
	// content without CORS). Snapshot first so a tainted canvas
	// reports an error instead of panicking the wasm instance.
	dataURL, err := canvasDataURL(n.canvas)
	if err != nil {
		return gui.PrintRunResult{
			Status:       gui.PrintRunError,
			ErrorCode:    "render_error",
			ErrorMessage: err.Error(),
		}
	}
	// Render canvas to an offscreen iframe so the host page
	// chrome is excluded from the print output.
	iframe := n.doc.Call("createElement", "iframe")
	st := iframe.Get("style")
	st.Set("position", "fixed")
	st.Set("width", "0")
	st.Set("height", "0")
	st.Set("border", "0")
	n.doc.Get("body").Call("appendChild", iframe)
	defer iframe.Call("remove")

	iframeDoc := iframe.Get("contentWindow").Get("document")
	body := iframeDoc.Get("body")
	body.Get("style").Set("margin", "0")

	// Wait for the image to decode before printing: printing
	// immediately after setting src prints a blank page while the
	// decode is still in flight.
	loaded := make(chan struct{}, 1)
	onLoad := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		select {
		case loaded <- struct{}{}:
		default:
		}
		return nil
	})

	img := iframeDoc.Call("createElement", "img")
	// Detach the handlers before releasing the func: on the timeout
	// path the decode is still in flight, and a late event calling a
	// released js.Func panics the wasm instance.
	defer func() {
		img.Set("onload", js.Null())
		img.Set("onerror", js.Null())
		onLoad.Release()
	}()
	img.Set("onload", onLoad)
	img.Set("onerror", onLoad)
	img.Set("src", dataURL)
	imgSt := img.Get("style")
	imgSt.Set("width", "100%")
	imgSt.Set("maxWidth", "100%")
	body.Call("appendChild", img)

	select {
	case <-loaded:
	case <-time.After(5 * time.Second):
	}

	iframe.Get("contentWindow").Call("print")
	return gui.PrintRunResult{Status: gui.PrintRunOK}
}

// canvasDataURL snapshots a canvas to a PNG data URL, converting a
// JS SecurityError on a tainted canvas into a Go error.
func canvasDataURL(canvas js.Value) (url string, err error) {
	defer func() {
		if r := recover(); r != nil {
			url, err = "", fmt.Errorf("canvas snapshot failed: %v", r)
		}
	}()
	return canvas.Call("toDataURL", "image/png").String(), nil
}

// --- Bookmarks (no-op on web) ---

func (n *nativePlatform) BookmarkLoadAll(_ string) []gui.BookmarkEntry { return nil }
func (n *nativePlatform) BookmarkPersist(_, _ string, _ []byte)        {}
func (n *nativePlatform) BookmarkStopAccess(_ []byte)                  {}

// --- Accessibility ---

func (n *nativePlatform) A11yInit(callback func(action, index int)) {
	n.a11y.init(n.doc, callback)
}

func (n *nativePlatform) A11ySync(
	nodes []gui.A11yNode, count, focusedIdx int,
) {
	n.a11y.sync(nodes, count, focusedIdx)
}

func (n *nativePlatform) A11yDestroy() {
	n.a11y.destroy()
}

func (n *nativePlatform) A11yAnnounce(text string) {
	n.a11y.announce(text)
}

// --- IME ---

func (n *nativePlatform) IMEStart() {
	if n.imeInput.Truthy() {
		return
	}
	input := n.doc.Call("createElement", "input")
	input.Set("type", "text")
	st := input.Get("style")
	st.Set("position", "absolute")
	st.Set("opacity", "0")
	st.Set("width", "1px")
	st.Set("height", "1px")
	st.Set("pointerEvents", "none")
	st.Set("left", "0px")
	st.Set("top", "0px")
	n.doc.Get("body").Call("appendChild", input)
	n.imeInput = input
	input.Call("focus")
}

func (n *nativePlatform) IMEStop() {
	if !n.imeInput.Truthy() {
		return
	}
	n.imeInput.Call("remove")
	n.imeInput = js.Value{}
	n.canvas.Call("focus")
}

func (n *nativePlatform) IMESetRect(x, y, _, _ int32) {
	if !n.imeInput.Truthy() {
		return
	}
	st := n.imeInput.Get("style")
	st.Set("left", itoa(int(x))+"px")
	st.Set("top", itoa(int(y))+"px")
}

// --- Window appearance (no-op on web) ---

func (n *nativePlatform) TitlebarDark(_ bool) {}

func (n *nativePlatform) SetWindowVibrancy(_ gui.VibrancyMaterial) {}

func (n *nativePlatform) SetWindowOpacity(_ float32) {}

// No window manager to hand a move or resize gesture to.
func (n *nativePlatform) StartWindowDrag()                   {}
func (n *nativePlatform) StartWindowResize(_ gui.WindowEdge) {}

// --- Spell check (no browser JS API exposes spell results) ---

func (n *nativePlatform) SpellCheck(_ string) []gui.SpellRange     { return nil }
func (n *nativePlatform) SpellSuggest(_ string, _, _ int) []string { return nil }
func (n *nativePlatform) SpellLearn(_ string)                      {}

// --- Native menubar (no-op on web) ---

func (n *nativePlatform) SetNativeMenubar(_ gui.NativeMenubarCfg, _ func(string)) {}
func (n *nativePlatform) ClearNativeMenubar()                                     {}

// webTrayIDs hands out unique positive tray IDs. The web stub
// reports success, so handles must stay distinct for App bookkeeping.
var webTrayIDs atomic.Int64

// --- System tray (no-op on web) ---

func (n *nativePlatform) CreateSystemTray(
	_ gui.SystemTrayCfg, _ func(string),
) (int, error) {
	return int(webTrayIDs.Add(1)), nil
}
func (n *nativePlatform) UpdateSystemTray(_ int, _ gui.SystemTrayCfg) {}
func (n *nativePlatform) RemoveSystemTray(_ int)                      {}

// --- helpers ---

// dotExtensions formats ["png","jpg"] as ".png,.jpg".
func dotExtensions(exts []string) string {
	var b strings.Builder
	for i, ext := range exts {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('.')
		b.WriteString(ext)
	}
	return b.String()
}

// jsObject creates a new empty JS object.
func jsObject() js.Value {
	return js.Global().Get("Object").New()
}

// --- Sound ---

// Beep is a no-op: this platform routes alerts through its own
// notification framework rather than an app-triggered system sound.
func (n *nativePlatform) Beep() {}

func (n *nativePlatform) BeepAvailable() bool { return false }
