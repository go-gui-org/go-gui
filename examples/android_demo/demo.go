//go:build android

// This example demonstrates go-gui on Android via OpenGL ES 3.0 (advanced: Android).
// Package androidapp is an Android demo app for go-gui.
// Built with gomobile bind to generate an AAR for Kotlin host.
package androidapp

import (
	_ "embed"
	"errors"
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/appinfo"
	"github.com/go-gui-org/go-gui/gui/backend/android"
)

//go:embed appinfo.toml
var manifest []byte

var info = appinfo.MustParse(manifest)

// App is the demo state struct bound into the AAR for the Kotlin host.
type App struct {
	Clicks       int
	SecretDraft  string
	SecretStatus string
	SecretBusy   bool
}

const tokenKey = "api-token"

// demoWindow holds the window Init created; a repeat Init call
// reuses it so the first window never leaks.
var demoWindow *gui.Window

// Init creates the gui.Window and registers it with the backend.
func Init() {
	if demoWindow != nil {
		return
	}
	w := gui.NewWindow(gui.WindowCfg{
		AppInfo: info,
		State:   &App{},
		OnInit: func(w *gui.Window) {
			gui.State[App](w).SecretStatus = "Reading the secret store..."
			runSecret(w, func() string { return secretStatus(w) })
			w.SetView(view)
		},
	})
	demoWindow = w
	android.SetWindow(w)
}

// SetFilesDir passes Context.getFilesDir() to the backend, where
// gui.SaveSettings keeps the app's settings. Call it before Start.
func SetFilesDir(dir string) { android.SetFilesDir(dir) }

// SecretStore is implemented by the Kotlin host. Its structural method set
// is accepted by android.SetSecretStore without exposing backend types in the
// gomobile-bound API.
type SecretStore interface {
	Load(service, key string) ([]byte, error)
	Save(service, key string, value []byte) error
	Delete(service, key string) error
}

// SetSecretStore registers the Kotlin credential-store implementation.
// Call it before Start.
func SetSecretStore(store SecretStore) { android.SetSecretStore(store) }

// Start initializes the GLES backend.
func Start(width, height int, scale float32) {
	android.Start(width, height, scale)
}

// Render runs one frame.
func Render() {
	android.Render()
}

// TouchInput dispatches a multi-touch event.
// Phase: 0=began, 1=moved, 2=ended, 3=cancelled.
func TouchInput(phase int, identifier int64, x, y float32) {
	android.TouchInput(phase, identifier, x, y)
}

// Resize updates the viewport.
func Resize(width, height int, scale float32) {
	android.Resize(width, height, scale)
}

// CleanUp releases all backend resources.
func CleanUp() {
	android.CleanUp()
}

// --- OpenURI bridge ---

// PendingURI returns and clears the pending URI to open.
func PendingURI() string { return android.PendingURI() }

// --- IME bridge ---

// PendingIMEAction returns and clears the pending IME action.
// 0=none, 1=show keyboard, 2=hide keyboard.
func PendingIMEAction() int32 { return android.PendingIMEAction() }

// PendingIMEKeyboardKind returns the focused field's keyboard kind.
// 0=text, 1=number, 2=decimal, 3=phone, 4=email, 5=url, 6=none.
func PendingIMEKeyboardKind() int32 { return android.PendingIMEKeyboardKind() }

// PendingIMEKeyboardSecure reports whether the focused field is a
// password field.
func PendingIMEKeyboardSecure() bool { return android.PendingIMEKeyboardSecure() }

// SoftKeyboardInset is called from Kotlin with the logical-pixel height
// the soft keyboard covers, 0 when it is down.
func SoftKeyboardInset(h float32) { android.SoftKeyboardInset(h) }

// PendingIMERectX returns the IME cursor rect X.
func PendingIMERectX() int32 { return android.PendingIMERectX() }

// PendingIMERectY returns the IME cursor rect Y.
func PendingIMERectY() int32 { return android.PendingIMERectY() }

// PendingIMERectW returns the IME cursor rect width.
func PendingIMERectW() int32 { return android.PendingIMERectW() }

// PendingIMERectH returns the IME cursor rect height.
func PendingIMERectH() int32 { return android.PendingIMERectH() }

// IMEComposition is called from Kotlin with preedit text.
func IMEComposition(text string, cursor, selLen int64) {
	android.IMEComposition(text, cursor, selLen)
}

// IMECommit is called from Kotlin when text is committed.
func IMECommit(text string) { android.IMECommit(text) }

// IMEDeleteSurrounding is called from Kotlin for an InputConnection
// range deletion. Lengths are UTF-16 code units unless inCodePoints.
func IMEDeleteSurrounding(before, after int64, inCodePoints bool) {
	android.IMEDeleteSurrounding(before, after, inCodePoints)
}

// AppearanceChanged is called from Kotlin with the uiMode night flag
// (issue #752): once at startup with the current mode and again on
// every onConfigurationChanged.
func AppearanceChanged(dark bool) { android.AppearanceChanged(dark) }

// --- Notification bridge ---

// PendingNotificationTitle returns the pending notification title.
func PendingNotificationTitle() string {
	return android.PendingNotificationTitle()
}

// PendingNotificationBody returns the pending notification body.
func PendingNotificationBody() string {
	return android.PendingNotificationBody()
}

// NotificationResult reports notification outcome from Kotlin.
// Status: 0=OK, 1=denied, 2=error.
func NotificationResult(status int64, errCode, errMsg string) {
	android.NotificationResult(status, errCode, errMsg)
}

// runSecret keeps potentially blocking platform storage work off the render
// thread and posts only the result back to the window state.
func runSecret(w *gui.Window, call func() string) {
	gui.State[App](w).SecretBusy = true
	go func() {
		message := call()
		w.QueueCommand(func(w *gui.Window) {
			app := gui.State[App](w)
			app.SecretStatus = message
			app.SecretBusy = false
		})
	}()
}

func secretStatus(w *gui.Window) string {
	value, err := gui.LoadSecret(w, tokenKey)
	switch {
	case err == nil:
		defer clear(value)
		return fmt.Sprintf("A token is saved (%d bytes).", len(value))
	case errors.Is(err, gui.ErrSecretNotFound):
		return "No token is saved."
	case errors.Is(err, gui.ErrSecretsUnsupported):
		return "Secure storage is unavailable: " + err.Error()
	default:
		return "Secret store error: " + err.Error()
	}
}

func saveSecret(w *gui.Window) {
	app := gui.State[App](w)
	value := []byte(app.SecretDraft)
	app.SecretDraft = ""
	runSecret(w, func() string {
		defer clear(value)
		if err := gui.SaveSecret(w, tokenKey, value); err != nil {
			return "Secret store error: " + err.Error()
		}
		return secretStatus(w)
	})
}

func deleteSecret(w *gui.Window) {
	runSecret(w, func() string {
		if err := gui.DeleteSecret(w, tokenKey); err != nil {
			return "Secret store error: " + err.Error()
		}
		return secretStatus(w)
	})
}

// --- Accessibility bridge ---

// A11yNodeCount returns the number of accessibility nodes.
func A11yNodeCount() int32 { return android.A11yNodeCount() }

// A11yNodeRole returns the AccessRole for node at index.
func A11yNodeRole(index int32) int32 { return android.A11yNodeRole(index) }

// A11yNodeLabel returns the label for node at index.
func A11yNodeLabel(index int32) string { return android.A11yNodeLabel(index) }

// A11yNodeValue returns the value for node at index.
func A11yNodeValue(index int32) string { return android.A11yNodeValue(index) }

// A11yNodeDescription returns the description for node at index.
func A11yNodeDescription(index int32) string {
	return android.A11yNodeDescription(index)
}

// A11yNodeBoundsX returns the X coordinate of node bounds.
func A11yNodeBoundsX(index int32) float32 { return android.A11yNodeBoundsX(index) }

// A11yNodeBoundsY returns the Y coordinate of node bounds.
func A11yNodeBoundsY(index int32) float32 { return android.A11yNodeBoundsY(index) }

// A11yNodeBoundsW returns the width of node bounds.
func A11yNodeBoundsW(index int32) float32 { return android.A11yNodeBoundsW(index) }

// A11yNodeBoundsH returns the height of node bounds.
func A11yNodeBoundsH(index int32) float32 { return android.A11yNodeBoundsH(index) }

// A11yNodeState returns the AccessState bitmask.
func A11yNodeState(index int32) int64 { return android.A11yNodeState(index) }

// A11yNodeValueNum returns the numeric value.
func A11yNodeValueNum(index int32) float32 { return android.A11yNodeValueNum(index) }

// A11yNodeValueMin returns the minimum value.
func A11yNodeValueMin(index int32) float32 { return android.A11yNodeValueMin(index) }

// A11yNodeValueMax returns the maximum value.
func A11yNodeValueMax(index int32) float32 { return android.A11yNodeValueMax(index) }

// A11yNodeParent returns the parent index (-1 for root).
func A11yNodeParent(index int32) int32 { return android.A11yNodeParent(index) }

// A11yNodeChildStart returns the children start index.
func A11yNodeChildStart(index int32) int32 { return android.A11yNodeChildStart(index) }

// A11yNodeChildCount returns the number of children.
func A11yNodeChildCount(index int32) int32 { return android.A11yNodeChildCount(index) }

// A11yFocusedIndex returns the index of the focused node.
func A11yFocusedIndex() int32 { return android.A11yFocusedIndex() }

// A11yPerformAction handles accessibility actions from Kotlin.
func A11yPerformAction(index, action int32) { android.A11yPerformAction(index, action) }

// PendingA11yAnnouncement returns pending announcement text.
func PendingA11yAnnouncement() string { return android.PendingA11yAnnouncement() }

func view(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    theme.PaddingLarge,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Go-Gui on Android",
				TextStyle: theme.TextStyleDisplay,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Android Keystore secret",
				TextStyle: theme.TextStyleTitleSmall,
			}),
			gui.Input(gui.InputCfg{
				ID:          "secret_token",
				Text:        app.SecretDraft,
				Placeholder: "Access token",
				IsPassword:  true,
				Sizing:      gui.FillFit,
				OnTextChanged: func(text string, ctx gui.EventCtx) {
					gui.State[App](ctx.Window).SecretDraft = text
					ctx.Consume()
				},
			}),
			gui.Row(gui.ContainerCfg{
				Padding:    gui.PaddingNone,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:       "secret_save",
						Disabled: app.SecretBusy || app.SecretDraft == "",
						Content:  []gui.View{gui.Text(gui.TextCfg{Text: "Save"})},
						OnClick: func(ctx gui.EventCtx) {
							saveSecret(ctx.Window)
							ctx.Consume()
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:       "secret_delete",
						Disabled: app.SecretBusy,
						Content:  []gui.View{gui.Text(gui.TextCfg{Text: "Delete"})},
						OnClick: func(ctx gui.EventCtx) {
							deleteSecret(ctx.Window)
							ctx.Consume()
						},
					}),
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      app.SecretStatus,
				TextStyle: theme.TextStyleSecondary,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Render-thread smoke test",
				TextStyle: theme.TextStyleTitleSmall,
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "android_click",
				A11YCfg: gui.A11YCfg{A11YLabel: "Increment counter"},
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("%d Clicks",
							app.Clicks),
					}),
				},
				OnClick: func(ctx gui.EventCtx) {
					gui.State[App](ctx.Window).Clicks++
				},
			}),
		},
	})
}
