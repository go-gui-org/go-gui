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
	A11YCfg
	// TextPadding pads the % readout, which trails the bar unboxed
	// (visual-refresh §8). Zero takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	TextPadding Padding
	Radius      Opt[float32]
	Percent     float32 // 0.0 to 1.0
	Width       float32
	Height      float32
	MinWidth    float32
	MaxWidth    float32
	MinHeight   float32
	MaxHeight   float32

	Color Color
	// ColorBar paints the filled portion. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorBar Color
	// TextBackground paints behind the % label. Unset takes the
	// theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	TextBackground Color
	Sizing         Sizing
	TextShow       bool
	Disabled       bool
	Invisible      bool
	Indefinite     bool
	// Vertical renders the bar bottom-to-top instead of
	// left-to-right.
	// exportaudit:keep — caller-facing config (issue #372)
	Vertical bool
}

// ProgressBar creates a progress bar view.
func ProgressBar(cfg ProgressBarCfg) View {
	RequireID("ProgressBar", cfg.ID)
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = guiTheme.progressBarStyle.TextStyle
	}
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.progressBarStyle.Color
	}
	if !cfg.ColorBar.IsSet() {
		cfg.ColorBar = guiTheme.progressBarStyle.colorBar
	}
	if !cfg.TextBackground.IsSet() {
		cfg.TextBackground = guiTheme.progressBarStyle.textBackground
	}
	if !cfg.TextPadding.IsSet() {
		cfg.TextPadding = guiTheme.progressBarStyle.textPadding
	}
	radius := cfg.Radius.Get(guiTheme.progressBarStyle.Radius)

	size := guiTheme.progressBarStyle.Size
	w := cfg.Width
	if w == 0 {
		w = size
	}
	h := cfg.Height
	if h == 0 {
		h = size
	}

	// The track: a row carrying the caller's width, track color and
	// radius, with the fill as its only child. The fill Row is the
	// radius-clipped layer (its own radius, so the clip survives the
	// track's), and progressBarAmendLayout sizes it from THIS shape's
	// laid-out box. FillFit makes the track absorb the leftover when
	// the outer row is FillFit; the stated Width is its floor when the
	// outer fits its content.
	//
	// The vertical column's cross-axis fill pass stretches FillFit
	// children to the column's width, which grows to fit the readout:
	// cap the track at its stated width unless the caller asked the
	// widget itself to fill (then the bar fills with it, as before).
	maxW := cfg.MaxWidth
	if cfg.Vertical && cfg.Sizing.Width != sizingFill && maxW == 0 {
		maxW = w
	}
	track := Row(ContainerCfg{
		Sizing:     FillFit,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Radius:     SomeF(radius),
		Color:      cfg.Color,
		Width:      w,
		Height:     h,
		MinWidth:   cfg.MinWidth,
		MaxWidth:   maxW,
		MinHeight:  cfg.MinHeight,
		MaxHeight:  cfg.MaxHeight,
		Content: []View{
			Row(ContainerCfg{
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				Radius:     SomeF(radius),
				Color:      cfg.ColorBar,
			}),
		},
	})

	// The readout trails the bar (visual-refresh §8): the label sits
	// outside the fill, so its contrast never depends on where the
	// fill happens to be. The outer row owns the identity, sizing and
	// a11y; the track above keeps the caller's width semantics.
	content := []View{track}
	if cfg.TextShow && !cfg.Indefinite {
		pct := math.Min(math.Max(float64(cfg.Percent), 0), 1)
		pct = math.Round(pct * 100)
		readout := fmt.Sprintf("%.0f%%", pct)
		content = append(content, Row(ContainerCfg{
			SizeBorder: NoBorder,
			Color:      cfg.TextBackground,
			Padding:    cfg.TextPadding,
			Content: []View{
				Text(TextCfg{
					Text:      readout,
					TextStyle: cfg.TextStyle,
				}),
			},
		}))
	}

	barPercent := cfg.Percent
	vertical := cfg.Vertical
	indefinite := cfg.Indefinite

	a11yState := AccessStateLive
	if cfg.Indefinite {
		a11yState = AccessStateBusy | AccessStateLive
	}

	ccfg := ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleProgressBar,
		A11YState: a11yState,
		a11Y: &accessInfo{
			Label:       a11yLabel(cfg.A11YLabel, cfg.Text),
			Description: cfg.A11YDescription,
			ValueNum:    cfg.Percent,
			ValueMin:    0,
			ValueMax:    1,
		},
		Disabled:   cfg.Disabled,
		Invisible:  cfg.Invisible,
		Spacing:    SomeF(guiTheme.SpacingSmall),
		SizeBorder: NoBorder,
		Sizing:     cfg.Sizing,
		Padding:    NoPadding,
		HAlign:     HAlignCenter,
		VAlign:     VAlignMiddle,
		AmendLayout: func(ctx EventCtx) {
			// Keyed by the effective ID: the indefinite animation and
			// its progress slot belong to this bar, not to every bar
			// that happens to share the leaf.
			progressBarAmendLayout(ctx.Layout, ctx.Window,
				barPercent, vertical, indefinite,
				ctx.Layout.Shape.idKey())
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
	barPercent float32, vertical, indefinite bool,
	id string,
) {
	if len(layout.Children) == 0 {
		return
	}

	percent := f32Clamp(barPercent, 0, 1)
	offset := float32(0)

	// Note: animation duration is sampled once on first render.
	// Changing Indefinite from true→false (or vice versa) after
	// the widget is visible has no effect. Use a different widget
	// ID to apply new parameters.
	if indefinite {
		percent = 0.3
		animID := ScopeID(id, "indefinite")
		if !w.touchViewBoundAnimation(animID) {
			kf := &KeyframeAnimation{
				AnimID:   animID,
				Repeat:   true,
				Duration: 1500 * time.Millisecond,
				Keyframes: []Keyframe{
					{At: 0, Value: 0},
					{At: 0.5, Value: 1, Easing: easeInOutCSS},
					{At: 1, Value: 0, Easing: easeInOutCSS},
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

	// The track is the first child; the readout (when present) trails
	// it as a sibling. The fill lives inside the track and keeps a
	// natural width of 0 (the amend sizes it), so the fill math is
	// relative to the track's own laid-out box — never the outer
	// widget's, which includes the readout.
	bar := &layout.Children[0]
	if len(bar.Children) == 0 {
		return
	}
	fill := &bar.Children[0]

	if vertical {
		h := f32Min(bar.Shape.Height*percent,
			bar.Shape.Height)
		fill.Shape.Y = bar.Shape.Y + bar.Shape.Height*offset
		fill.Shape.Height = h
		fill.Shape.Width = bar.Shape.Width
	} else {
		wd := f32Min(bar.Shape.Width*percent,
			bar.Shape.Width)
		fill.Shape.X = bar.Shape.X + bar.Shape.Width*offset
		fill.Shape.Width = wd
		fill.Shape.Height = bar.Shape.Height
	}
}
