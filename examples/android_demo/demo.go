//go:build android

// Package androidapp is an Android demo app for go-gui.
// Built with gomobile bind to generate an AAR for Kotlin host.
package androidapp

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/android"
)

type App struct {
	Clicks int
}

// Init creates the gui.Window and registers it with the backend.
func Init() {
	w := gui.NewWindow(gui.WindowCfg{
		State: &App{},
		OnInit: func(w *gui.Window) {
			w.UpdateView(view)
		},
	})
	android.SetWindow(w)
}

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

// TouchBegan maps a touch-down event.
// Deprecated: use TouchInput for multi-touch support.
func TouchBegan(x, y float32) { android.TouchBegan(x, y) }

// TouchMoved maps a touch-move event.
// Deprecated: use TouchInput for multi-touch support.
func TouchMoved(x, y float32) { android.TouchMoved(x, y) }

// TouchEnded maps a touch-up event.
// Deprecated: use TouchInput for multi-touch support.
func TouchEnded(x, y float32) { android.TouchEnded(x, y) }

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
	ww, wh := w.WindowSize()
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Go-Gui on Android",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.Text(gui.TextCfg{
				Text: "Tap the button to increment.",
			}),
			gui.Button(gui.ButtonCfg{
				IDFocus: 1,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("%d Clicks",
							app.Clicks),
					}),
				},
				OnClick: func(_ *gui.Layout,
					e *gui.Event, w *gui.Window) {
					gui.State[App](w).Clicks++
					e.IsHandled = true
				},
			}),
		},
	})
}
