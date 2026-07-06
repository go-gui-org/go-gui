//go:build js && wasm

package web

import (
	"fmt"
	"strings"
	"syscall/js"

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

func (n *nativePlatform) OpenURI(uri string) error {
	if !hasPrefixFold(uri, "http://") &&
		!hasPrefixFold(uri, "https://") &&
		!hasPrefixFold(uri, "mailto:") {
		return fmt.Errorf("web: blocked URI scheme in %q", uri)
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

	changeCb := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		files := input.Get("files")
		count := files.Length()
		if count == 0 {
			ch <- gui.PlatformDialogResult{
				Status: gui.DialogCancel,
			}
			return nil
		}
		paths := make([]gui.PlatformPath, count)
		for i := range count {
			paths[i] = gui.PlatformPath{
				Path: files.Index(i).Get("name").String(),
			}
		}
		ch <- gui.PlatformDialogResult{
			Status: gui.DialogOK,
			Paths:  paths,
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
	input.Call("click")

	result := <-ch
	changeCb.Release()
	cancelCb.Release()
	input.Call("remove")
	return result
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
) gui.PlatformDialogResult {
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
	result := <-ch
	thenCb.Release()
	catchCb.Release()
	return result
}

func (n *nativePlatform) ShowFolderDialog(_, _ string) gui.PlatformDialogResult {
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
	result := <-ch
	thenCb.Release()
	catchCb.Release()
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
	// Render canvas to an offscreen iframe so the host page
	// chrome is excluded from the print output.
	iframe := n.doc.Call("createElement", "iframe")
	st := iframe.Get("style")
	st.Set("position", "fixed")
	st.Set("width", "0")
	st.Set("height", "0")
	st.Set("border", "0")
	n.doc.Get("body").Call("appendChild", iframe)

	iframeDoc := iframe.Get("contentWindow").Get("document")
	body := iframeDoc.Get("body")
	body.Get("style").Set("margin", "0")

	img := iframeDoc.Call("createElement", "img")
	img.Set("src", n.canvas.Call("toDataURL", "image/png").String())
	imgSt := img.Get("style")
	imgSt.Set("width", "100%")
	imgSt.Set("maxWidth", "100%")
	body.Call("appendChild", img)

	iframe.Get("contentWindow").Call("print")
	iframe.Call("remove")
	return gui.PrintRunResult{Status: gui.PrintRunOK}
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

// --- Spell check (no browser JS API exposes spell results) ---

func (n *nativePlatform) SpellCheck(_ string) []gui.SpellRange     { return nil }
func (n *nativePlatform) SpellSuggest(_ string, _, _ int) []string { return nil }
func (n *nativePlatform) SpellLearn(_ string)                      {}

// --- Native menubar (no-op on web) ---

func (n *nativePlatform) SetNativeMenubar(_ gui.NativeMenubarCfg, _ func(string)) {}
func (n *nativePlatform) ClearNativeMenubar()                                     {}

// --- System tray (no-op on web) ---

func (n *nativePlatform) CreateSystemTray(
	_ gui.SystemTrayCfg, _ func(string),
) (int, error) {
	return 0, nil
}
func (n *nativePlatform) UpdateSystemTray(_ int, _ gui.SystemTrayCfg) {}
func (n *nativePlatform) RemoveSystemTray(_ int)                      {}

// --- helpers ---

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) &&
		strings.EqualFold(s[:len(prefix)], prefix)
}

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
