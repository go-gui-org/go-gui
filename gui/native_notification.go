package gui

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
	if w.nativePlatform == nil {
		dispatchNotificationDone(w, cfg.OnDone, NativeNotificationResult{
			Status:       NotificationError,
			ErrorCode:    "unsupported",
			ErrorMessage: "no native platform",
		})
		return
	}
	// Notification may block; run in goroutine. QueueCommand is
	// thread-safe (uses commandsMu), so the ctx check + queue
	// pattern is safe even without atomicity.
	ctx := w.Ctx()
	go func() {
		result := w.nativePlatform.SendNotification(cfg.Title, cfg.Body)
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
