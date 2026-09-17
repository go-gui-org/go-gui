package gui

import "unicode/utf8"

// maxNotificationTitleLen and maxNotificationBodyLen cap the title
// and body before they reach a backend, mirroring nativehost's caps
// for the desktop targets. Web and mobile have no caps of their own,
// so the gui layer enforces them for every platform.
const (
	maxNotificationTitleLen = 256
	maxNotificationBodyLen  = 1024
)

// truncateNotificationBytes caps s at max bytes on a rune boundary,
// so the result stays valid UTF-8.
func truncateNotificationBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	end := max
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end]
}

// NativeNotificationStatus reports notification outcome.
type NativeNotificationStatus uint8

// NativeNotificationStatus values.
const (
	NotificationOK     NativeNotificationStatus = iota
	NotificationDenied                          // permission denied
	NotificationError                           // platform error
)

// NativeNotificationResult contains notification delivery data.
type NativeNotificationResult struct {
	ErrorCode    string
	ErrorMessage string
	Status       NativeNotificationStatus
}

// NativeNotificationCfg configures an OS-level notification.
type NativeNotificationCfg struct {
	OnDone func(NativeNotificationResult, *Window)
	Title  string
	Body   string
}

// NativeNotification posts an OS-level notification.
func (w *Window) NativeNotification(cfg NativeNotificationCfg) {
	if cfg.Title == "" {
		dispatchNotificationDone(w, cfg.OnDone, NativeNotificationResult{
			Status:       NotificationError,
			ErrorCode:    "invalid_cfg",
			ErrorMessage: "title is required",
		})
		return
	}
	w.QueueCommand(func(w *Window) {
		nativeNotificationImpl(w, cfg)
	})
}

func nativeNotificationImpl(w *Window, cfg NativeNotificationCfg) {
	np := w.nativePlatform
	if np == nil {
		dispatchNotificationDone(w, cfg.OnDone, NativeNotificationResult{
			Status:       NotificationError,
			ErrorCode:    "unsupported",
			ErrorMessage: "no native platform",
		})
		return
	}
	title := truncateNotificationBytes(cfg.Title, maxNotificationTitleLen)
	body := truncateNotificationBytes(cfg.Body, maxNotificationBodyLen)
	// Notification may block; run in goroutine. QueueCommand is
	// thread-safe (uses commandsMu), so the ctx check + queue
	// pattern is safe even without atomicity.
	ctx := w.Ctx()
	go func() {
		result := np.SendNotification(title, body)
		if ctx.Err() != nil {
			return
		}
		w.QueueCommand(func(w *Window) {
			dispatchNotificationDone(w, cfg.OnDone, result)
		})
	}()
}

func dispatchNotificationDone(w *Window, onDone func(NativeNotificationResult, *Window), result NativeNotificationResult) {
	if onDone != nil {
		onDone(result, w)
	}
}
