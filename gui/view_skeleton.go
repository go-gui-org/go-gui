package gui

import "time"

// SkeletonVariant selects the skeleton shape.
type skeletonVariant uint8

// SkeletonVariant constants.
const (
	skeletonRect skeletonVariant = iota
	SkeletonCircle
)

// SkeletonCfg configures a skeleton loader view.
type SkeletonCfg struct {
	ID string
	A11YCfg
	Radius         Radius
	Width          float32
	Height         float32
	MinWidth       float32
	MaxWidth       float32
	MinHeight      float32
	MaxHeight      float32
	Color          Color
	ColorHighlight Color
	Sizing         Sizing
	Variant        skeletonVariant
	Disabled       bool
	Invisible      bool
}

// Skeleton creates a skeleton shimmer placeholder view.
func Skeleton(cfg SkeletonCfg) View {
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.skeletonStyle.Colors.Base
	}
	if !cfg.ColorHighlight.IsSet() {
		cfg.ColorHighlight = guiTheme.skeletonStyle.ColorHighlight
	}
	radius := cfg.Radius.Or(guiTheme.skeletonStyle.Radius)

	label := cfg.A11YLabel
	if label == "" {
		label = "Loading"
	}

	colorBase := cfg.Color
	colorHL := cfg.ColorHighlight

	ccfg := ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleProgressBar,
		A11YState: AccessStateBusy | AccessStateLive,
		a11Y: &accessInfo{
			Label:       label,
			Description: cfg.A11YDescription,
		},
		Width:      cfg.Width,
		Height:     cfg.Height,
		MinWidth:   cfg.MinWidth,
		MaxWidth:   cfg.MaxWidth,
		MinHeight:  cfg.MinHeight,
		MaxHeight:  cfg.MaxHeight,
		Disabled:   cfg.Disabled,
		Invisible:  cfg.Invisible,
		Color:      cfg.Color,
		Radius:     RadiusPx(radius),
		SizeBorder: NoBorder,
		Sizing:     cfg.Sizing,
		Padding:    NoPadding,
		// The shimmer only moves gradient stops, so a render-only
		// frame re-runs the hook instead of a layout.
		amendOnRender: true,
		AmendLayout: func(ctx EventCtx) {
			// The shimmer's animation and state are keyed by this
			// shape's effective ID, so two skeletons written with the
			// same leaf under different ID-bearing panels shimmer
			// independently.
			skeletonAmendLayout(ctx.Layout, ctx.Window,
				ctx.Layout.Shape.idKey(), colorBase, colorHL)
		},
	}

	if cfg.Variant == SkeletonCircle {
		return Circle(ccfg)
	}
	return Row(ccfg)
}

func skeletonAmendLayout(
	layout *Layout, w *Window,
	id string, colorBase, colorHL Color,
) {
	// Note: animation duration is sampled once on first render.
	// Use a different widget ID to apply new parameters.
	animID := cachedAnimID(w, animIDSkeleton, id, "skeleton", id)
	if !w.touchViewBoundAnimation(animID) {
		kf := &KeyframeAnimation{
			AnimID:   animID,
			Repeat:   true,
			Duration: 1500 * time.Millisecond,
			refresh:  AnimationRefreshRenderOnly,
			Keyframes: []Keyframe{
				{At: 0, Value: 0},
				{At: 1, Value: 1, Easing: EaseInOutCSS},
			},
			OnValue: func(v float32, w *Window) {
				pm := StateMap[string, float32](
					w, nsSkeleton, capFew)
				pm.Set(id, v)
			},
		}
		w.animationAddViewBound(kf)
	}

	t := StateReadOr(w, nsSkeleton, id, float32(0))

	// Map t to position range [-0.3, 1.3].
	pos := -0.3 + float64(t)*1.6

	stops := [skeletonStops]GradientStop{
		{Color: colorBase, Pos: 0},
		{Color: colorBase, Pos: float32(f64Clamp(pos-0.15, 0, 1))},
		{Color: colorHL, Pos: float32(f64Clamp(pos, 0, 1))},
		{Color: colorBase, Pos: float32(f64Clamp(pos+0.15, 0, 1))},
		{Color: colorBase, Pos: 1},
	}

	if layout.Shape.fx == nil {
		layout.Shape.fx = &shapeEffects{}
	}
	// A render-only frame runs this hook again on the shape the last
	// layout built, whose gradient this hook already owns: rewrite its
	// stops in place, so a shimmer tick allocates nothing. The renderers
	// that pointed at the old stops are rebuilt by the same pass.
	if g := layout.Shape.fx.Gradient; g != nil && len(g.Stops) == skeletonStops {
		copy(g.Stops, stops[:])
		return
	}
	layout.Shape.fx.Gradient = &GradientDef{
		Stops:     append([]GradientStop(nil), stops[:]...),
		Type:      GradientLinear,
		Direction: GradientToRight,
	}
}

// skeletonStops is the shimmer gradient's stop count.
const skeletonStops = 5
