package gui

import (
	"slices"
	"strconv"
	"time"
)

// ToastSeverity indicates the visual severity of a toast.
type ToastSeverity uint8

// ToastSeverity constants.
const (
	ToastInfo ToastSeverity = iota
	ToastSuccess
	ToastWarning
	ToastError
)

// ToastCfg configures a toast notification.
type ToastCfg struct {
	OnAction    func(*Window)
	Title       string
	Body        string
	ActionLabel string
	Duration    time.Duration // 0 = default (3s); toastPersistent = no auto-dismiss
	Severity    ToastSeverity
}

// toastNotification is an active toast instance.
type toastNotification struct {
	cfg      ToastCfg
	id       uint64
	animFrac float32 // 0=collapsed, 1=full height
	phase    toastPhase
	hovered  bool
}

// toastPhase tracks toast lifecycle.
type toastPhase uint8

const (
	toastEntering toastPhase = iota
	toastVisible
	toastExiting
)

// toastPersistent disables auto-dismiss when set as Duration.
const toastPersistent = time.Duration(-1)

const (
	toastEnterDuration = 200 * time.Millisecond
	toastExitDuration  = 200 * time.Millisecond
	toastDefaultDelay  = 3 * time.Second
)

// toastContainerView builds the floating column containing all
// visible toasts.
func toastContainerView(w *Window) View {
	if len(w.toasts) == 0 {
		return nil
	}

	// Reset hovered flags each frame; OnHover re-sets for
	// still-hovered toasts.
	for i := range w.toasts {
		w.toasts[i].hovered = false
	}

	style := DefaultToastStyle

	// Map anchor to float attach.
	var anchor, tieOff FloatAttach
	var offsetX, offsetY float32

	switch style.Anchor {
	case ToastTopLeft:
		anchor = FloatTopLeft
		tieOff = FloatTopLeft
		offsetX = style.Margin
		offsetY = style.Margin
	case ToastTopRight:
		anchor = FloatTopRight
		tieOff = FloatTopRight
		offsetX = -style.Margin
		offsetY = style.Margin
	case ToastBottomLeft:
		anchor = FloatBottomLeft
		tieOff = FloatBottomLeft
		offsetX = style.Margin
		offsetY = -style.Margin
	case ToastBottomRight:
		anchor = FloatBottomRight
		tieOff = FloatBottomRight
		offsetX = -style.Margin
		offsetY = -style.Margin
	}

	// Build toast items. Bottom anchors: newest last.
	// Top anchors: newest first (reversed).
	items := make([]View, 0, len(w.toasts))
	isTop := style.Anchor == ToastTopLeft ||
		style.Anchor == ToastTopRight

	if isTop {
		for i := range slices.Backward(w.toasts) {
			items = append(items,
				toastItemView(&w.toasts[i], style))
		}
	} else {
		for i := range w.toasts {
			items = append(items,
				toastItemView(&w.toasts[i], style))
		}
	}

	return Column(ContainerCfg{
		Float:        true,
		FloatAnchor:  anchor,
		FloatTieOff:  tieOff,
		FloatOffsetX: offsetX,
		FloatOffsetY: offsetY,
		Sizing:       FitFit,
		Padding:      NoPadding,
		SizeBorder:   NoBorder,
		Spacing:      Some(style.Spacing),
		Color:        ColorTransparent,
		Content:      items,
	})
}

// toastItemView builds a single toast notification view.
func toastItemView(toast *toastNotification, style ToastStyle) View {
	frac := toast.animFrac
	id := toast.id

	// Accent color based on severity.
	var accentColor Color
	switch toast.cfg.Severity {
	case ToastInfo:
		accentColor = style.ColorInfo
	case ToastSuccess:
		accentColor = style.ColorSuccess
	case ToastWarning:
		accentColor = style.ColorWarning
	case ToastError:
		accentColor = style.ColorError
	}

	// Body column: title + body text.
	var bodyContent []View
	if toast.cfg.Title != "" {
		bodyContent = append(bodyContent, Text(TextCfg{
			Text:      toast.cfg.Title,
			TextStyle: style.TitleStyle,
		}))
	}
	if toast.cfg.Body != "" {
		bodyContent = append(bodyContent, Text(TextCfg{
			Text:      toast.cfg.Body,
			TextStyle: style.TextStyle,
			Mode:      TextModeWrap,
		}))
	}

	// Buttons column: action + dismiss.
	var buttons []View
	if toast.cfg.ActionLabel != "" && toast.cfg.OnAction != nil {
		onAction := toast.cfg.OnAction
		buttons = append(buttons, Button(ButtonCfg{
			Color:   ColorTransparent,
			Content: []View{Text(TextCfg{Text: toast.cfg.ActionLabel, TextStyle: style.TextStyle})},
			OnClick: func(_ *Layout, _ *Event, w *Window) {
				onAction(w)
				toastStartExit(w, id)
			},
		}))
	}
	buttons = append(buttons, Button(ButtonCfg{
		Color:      ColorTransparent,
		SizeBorder: NoBorder,
		Content:    []View{Text(TextCfg{Text: "\u00d7", TextStyle: style.TextStyle})},
		OnClick: func(_ *Layout, _ *Event, w *Window) {
			toastStartExit(w, id)
		},
	}))

	return Row(ContainerCfg{
		Width:       style.Width,
		Sizing:      FixedFit,
		Padding:     NoPadding,
		Color:       style.Color,
		ColorBorder: style.ColorBorder,
		SizeBorder:  Some(style.SizeBorder),
		Radius:      Some(style.Radius),
		Shadow:      style.Shadow,
		Clip:        true,
		Opacity:     SomeF(frac),
		Spacing:     Some(SpacingSmall),
		A11YRole:    AccessRoleGroup,
		A11YState:   AccessStateLive,
		A11YLabel:   toastA11YLabel(toast),
		AmendLayout: func(layout *Layout, _ *Window) {
			if frac < 1.0 {
				layout.Shape.Height *= frac
			}
		},
		OnClick: func(_ *Layout, e *Event, _ *Window) {
			e.IsHandled = true
		},
		OnHover: func(_ *Layout, _ *Event, w *Window) {
			toastSetHovered(w, id, true)
		},
		Content: []View{
			// Accent bar.
			Rectangle(RectangleCfg{
				Color:  accentColor,
				Width:  style.AccentWidth,
				Sizing: FixedFill,
			}),
			// Body.
			Column(ContainerCfg{
				Sizing:  FillFit,
				Padding: Some(style.Padding),
				Content: bodyContent,
			}),
			// Buttons.
			Column(ContainerCfg{
				Sizing:  FitFit,
				Padding: NoPadding,
				VAlign:  VAlignTop,
				Content: buttons,
			}),
		},
	})
}

// toastSetHovered sets the hovered flag on a toast by id.
func toastSetHovered(w *Window, id uint64, hovered bool) {
	for i := range w.toasts {
		if w.toasts[i].id == id {
			w.toasts[i].hovered = hovered
			return
		}
	}
}

// toastStartEnter starts the enter animation for a toast.
func toastStartEnter(w *Window, id uint64) {
	animID := toastAnimID("enter", id)
	w.AnimationAdd(&TweenAnimation{
		AnimID:   animID,
		Duration: toastEnterDuration,
		Easing:   EaseOutCubic,
		From:     0,
		To:       1,
		OnValue: func(val float32, w *Window) {
			for i := range w.toasts {
				if w.toasts[i].id == id {
					w.toasts[i].animFrac = val
					break
				}
			}
		},
		OnDone: func(w *Window) {
			for i := range w.toasts {
				if w.toasts[i].id == id {
					w.toasts[i].phase = toastVisible
					break
				}
			}
			toastStartDismissTimer(w, id)
		},
	})
}

// toastStartDismissTimer starts the auto-dismiss delay.
func toastStartDismissTimer(w *Window, id uint64) {
	dur := toastDuration(w, id)
	if dur == 0 {
		return // no auto-dismiss
	}
	animID := toastAnimID("dismiss", id)
	w.AnimationAdd(&Animate{
		AnimID: animID,
		Delay:  dur,
		Callback: func(_ *Animate, w *Window) {
			// If hovered, reset and wait again.
			for i := range w.toasts {
				if w.toasts[i].id == id && w.toasts[i].hovered {
					w.toasts[i].hovered = false
					toastStartDismissTimer(w, id)
					return
				}
			}
			toastStartExit(w, id)
		},
	})
}

// toastDuration returns the configured duration for a toast.
// Returns 0 for persistent toasts (negative duration).
func toastDuration(w *Window, id uint64) time.Duration {
	for i := range w.toasts {
		if w.toasts[i].id == id {
			d := w.toasts[i].cfg.Duration
			if d < 0 {
				return 0 // persistent
			}
			if d == 0 {
				return toastDefaultDelay
			}
			return d
		}
	}
	return toastDefaultDelay
}

// toastStartExit starts the exit animation. Guards against
// double-exit.
func toastStartExit(w *Window, id uint64) {
	idx := -1
	for i := range w.toasts {
		if w.toasts[i].id == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	if w.toasts[idx].phase == toastExiting {
		return
	}
	w.toasts[idx].phase = toastExiting

	animID := toastAnimID("exit", id)
	w.AnimationAdd(&TweenAnimation{
		AnimID:   animID,
		Duration: toastExitDuration,
		Easing:   EaseInCubic,
		From:     1,
		To:       0,
		OnValue: func(val float32, w *Window) {
			for i := range w.toasts {
				if w.toasts[i].id == id {
					w.toasts[i].animFrac = val
					break
				}
			}
		},
		OnDone: func(w *Window) {
			toastRemove(w, id)
		},
	})
}

// toastRemove deletes a toast from the window's toast slice.
func toastRemove(w *Window, id uint64) {
	for i := range w.toasts {
		if w.toasts[i].id == id {
			w.toasts = append(w.toasts[:i], w.toasts[i+1:]...)
			w.UpdateWindow()
			return
		}
	}
}

// toastEnforceMaxVisible starts exit on oldest non-exiting toasts
// when count exceeds max.
func toastEnforceMaxVisible(w *Window) {
	maxVisible := DefaultToastStyle.MaxVisible
	if maxVisible <= 0 {
		return
	}
	visible := 0
	for i := range w.toasts {
		if w.toasts[i].phase != toastExiting {
			visible++
		}
	}
	for i := 0; visible > maxVisible && i < len(w.toasts); i++ {
		if w.toasts[i].phase != toastExiting {
			toastStartExit(w, w.toasts[i].id)
			visible--
		}
	}
}

// toastA11YLabel returns a label for accessibility, falling back
// to body if title is empty.
func toastA11YLabel(t *toastNotification) string {
	if t.cfg.Title != "" {
		return t.cfg.Title
	}
	return t.cfg.Body
}

// toastAnimID generates a unique animation ID for toast anims.
func toastAnimID(prefix string, id uint64) string {
	return prefix + "_toast_" + strconv.FormatUint(id, 10)
}

// Toast shows a toast notification. Returns the toast id.
func (w *Window) Toast(cfg ToastCfg) uint64 {
	w.toastCounter++
	id := w.toastCounter
	n := toastNotification{
		id:    id,
		cfg:   cfg,
		phase: toastEntering,
	}
	w.toasts = append(w.toasts, n)
	toastStartEnter(w, id)
	toastEnforceMaxVisible(w)
	w.UpdateWindow()
	return id
}

// ToastDismiss starts exit on a specific toast.
func (w *Window) ToastDismiss(id uint64) {
	toastStartExit(w, id)
}

// ToastDismissAll starts exit on all toasts.
func (w *Window) ToastDismissAll() {
	for i := range w.toasts {
		toastStartExit(w, w.toasts[i].id)
	}
}
