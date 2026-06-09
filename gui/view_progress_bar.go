package gui

import (
	"fmt"
	"math"
	"time"
)

// ProgressBarCfg configures a progress bar view.
type ProgressBarCfg struct {
	TextStyle TextStyle
	ID        string `gui:"required"`
	Text      string

	// Accessibility
	A11YLabel       string
	A11YDescription string
	TextPadding     Opt[Padding]
	Radius          Opt[float32]
	Percent         float32 // 0.0 to 1.0
	Width           float32
	Height          float32
	MinWidth        float32
	MaxWidth        float32
	MinHeight       float32
	MaxHeight       float32

	Color          Color
	ColorBar       Color
	TextBackground Color
	Sizing         Sizing
	TextShow       bool
	Disabled       bool
	Invisible      bool
	Indefinite     bool
	Vertical       bool
}

// ProgressBar creates a progress bar view.
func ProgressBar(cfg ProgressBarCfg) View {
	RequireID("ProgressBar", cfg.ID)
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = guiTheme.ProgressBarStyle.TextStyle
	}
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.ProgressBarStyle.Color
	}
	if !cfg.ColorBar.IsSet() {
		cfg.ColorBar = guiTheme.ProgressBarStyle.ColorBar
	}
	if !cfg.TextBackground.IsSet() {
		cfg.TextBackground = guiTheme.ProgressBarStyle.TextBackground
	}
	if !cfg.TextPadding.IsSet() {
		cfg.TextPadding = Some(guiTheme.ProgressBarStyle.TextPadding)
	}
	radius := cfg.Radius.Get(guiTheme.ProgressBarStyle.Radius)

	content := make([]View, 0, 2)
	content = append(content, Row(ContainerCfg{
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Radius:     SomeF(radius),
		Color:      cfg.ColorBar,
	}))

	if cfg.TextShow && !cfg.Indefinite {
		pct := math.Min(math.Max(float64(cfg.Percent), 0), 1)
		pct = math.Round(pct * 100)
		content = append(content, Row(ContainerCfg{
			SizeBorder: NoBorder,
			Color:      cfg.TextBackground,
			Padding:    cfg.TextPadding,
			Content: []View{
				Text(TextCfg{
					Text:      fmt.Sprintf("%.0f%%", pct),
					TextStyle: cfg.TextStyle,
				}),
			},
		}))
	}

	barPercent := cfg.Percent
	textShow := cfg.TextShow
	vertical := cfg.Vertical
	indefinite := cfg.Indefinite
	id := cfg.ID

	size := guiTheme.ProgressBarStyle.Size

	a11yState := AccessStateLive
	if cfg.Indefinite {
		a11yState = AccessStateBusy | AccessStateLive
	}

	w := cfg.Width
	if w == 0 {
		w = size
	}
	h := cfg.Height
	if h == 0 {
		h = size
	}

	ccfg := ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleProgressBar,
		A11YState: a11yState,
		A11Y: &AccessInfo{
			Label:       a11yLabel(cfg.A11YLabel, cfg.Text),
			Description: cfg.A11YDescription,
			ValueNum:    cfg.Percent,
			ValueMin:    0,
			ValueMax:    1,
		},
		Width:      w,
		Height:     h,
		MinWidth:   cfg.MinWidth,
		MaxWidth:   cfg.MaxWidth,
		MinHeight:  cfg.MinHeight,
		MaxHeight:  cfg.MaxHeight,
		Disabled:   cfg.Disabled,
		Invisible:  cfg.Invisible,
		Color:      cfg.Color,
		Radius:     SomeF(radius),
		SizeBorder: NoBorder,
		Sizing:     cfg.Sizing,
		Padding:    NoPadding,
		HAlign:     HAlignCenter,
		VAlign:     VAlignMiddle,
		AmendLayout: func(layout *Layout, w *Window) {
			progressBarAmendLayout(layout, w,
				barPercent, textShow, vertical,
				indefinite, id)
		},
		Content: content,
	}

	if cfg.Vertical {
		return Column(ccfg)
	}
	return Row(ccfg)
}

func progressBarAmendLayout(
	layout *Layout, w *Window,
	barPercent float32, textShow, vertical, indefinite bool,
	id string,
) {
	if len(layout.Children) == 0 {
		return
	}

	percent := f32Clamp(barPercent, 0, 1)
	offset := float32(0)

	if indefinite {
		percent = 0.3
		animID := id + "_indefinite"
		if !w.touchViewBoundAnimation(animID) {
			kf := &KeyframeAnimation{
				AnimID:   animID,
				Repeat:   true,
				Duration: 1500 * time.Millisecond,
				Keyframes: []Keyframe{
					{At: 0, Value: 0},
					{At: 0.5, Value: 1, Easing: EaseInOutCSS},
					{At: 1, Value: 0, Easing: EaseInOutCSS},
				},
				OnValue: func(v float32, w *Window) {
					pm := StateMap[string, float32](
						w, nsProgress, capModerate)
					pm.Set(id, v)
				},
			}
			w.animationAddViewBound(kf)
		}
		pm := StateMap[string, float32](w, nsProgress, capModerate)
		if progress, ok := pm.Get(id); ok {
			offset = (1 - percent) * progress
		}
	}

	bar := &layout.Children[0]

	if vertical {
		h := f32Min(layout.Shape.Height*percent,
			layout.Shape.Height)
		bar.Shape.X = layout.Shape.X
		bar.Shape.Y = layout.Shape.Y +
			layout.Shape.Height*offset
		bar.Shape.Height = h
		bar.Shape.Width = layout.Shape.Width
	} else {
		wd := f32Min(layout.Shape.Width*percent,
			layout.Shape.Width)
		bar.Shape.X = layout.Shape.X +
			layout.Shape.Width*offset
		bar.Shape.Y = layout.Shape.Y
		bar.Shape.Width = wd
		bar.Shape.Height = layout.Shape.Height
	}

	if textShow && !indefinite && len(layout.Children) > 1 {
		progressBarCenterLabel(layout.Shape, &layout.Children[1])
	}
}

// progressBarCenterLabel centers a label layout within the
// parent shape on both axes, shifting child shapes to match.
func progressBarCenterLabel(parent *Shape, lbl *Layout) {
	centerX := parent.X + parent.Width/2
	oldX := lbl.Shape.X
	lbl.Shape.X = centerX - lbl.Shape.Width/2
	if len(lbl.Children) > 0 {
		lbl.Children[0].Shape.X -= oldX - lbl.Shape.X
	}

	centerY := parent.Y + parent.Height/2
	oldY := lbl.Shape.Y
	lbl.Shape.Y = centerY - lbl.Shape.Height/2
	if len(lbl.Children) > 0 {
		lbl.Children[0].Shape.Y -= oldY - lbl.Shape.Y
	}
}
