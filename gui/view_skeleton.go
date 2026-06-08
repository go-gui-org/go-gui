package gui

import "time"

// SkeletonVariant selects the skeleton shape.
type SkeletonVariant uint8

// SkeletonVariant constants.
const (
	SkeletonRect SkeletonVariant = iota
	SkeletonCircle
)

// SkeletonCfg configures a skeleton loader view.
type SkeletonCfg struct {
	ID              string
	A11YLabel       string
	A11YDescription string
	Radius          Opt[float32]
	Width           float32
	Height          float32
	MinWidth        float32
	MaxWidth        float32
	MinHeight       float32
	MaxHeight       float32
	Color           Color
	ColorHighlight  Color
	Sizing          Sizing
	Variant         SkeletonVariant
	Disabled        bool
	Invisible       bool
}

// Skeleton creates a skeleton shimmer placeholder view.
func Skeleton(cfg SkeletonCfg) View {
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.SkeletonStyle.Color
	}
	if !cfg.ColorHighlight.IsSet() {
		cfg.ColorHighlight = guiTheme.SkeletonStyle.ColorHighlight
	}
	radius := cfg.Radius.Get(guiTheme.SkeletonStyle.Radius)

	label := cfg.A11YLabel
	if label == "" {
		label = "Loading"
	}

	id := cfg.ID
	colorBase := cfg.Color
	colorHL := cfg.ColorHighlight

	ccfg := ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleProgressBar,
		A11YState: AccessStateBusy | AccessStateLive,
		A11Y: &AccessInfo{
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
		Radius:     SomeF(radius),
		SizeBorder: NoBorder,
		Sizing:     cfg.Sizing,
		Padding:    NoPadding,
		AmendLayout: func(layout *Layout, w *Window) {
			skeletonAmendLayout(layout, w, id,
				colorBase, colorHL)
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
	animID := "skeleton_" + id
	if !w.touchViewBoundAnimation(animID) {
		kf := &KeyframeAnimation{
			AnimID:   animID,
			Repeat:   true,
			Duration: 1500 * time.Millisecond,
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

	stops := []GradientStop{
		{Color: colorBase, Pos: 0},
		{Color: colorBase, Pos: float32(f64Clamp(pos-0.15, 0, 1))},
		{Color: colorHL, Pos: float32(f64Clamp(pos, 0, 1))},
		{Color: colorBase, Pos: float32(f64Clamp(pos+0.15, 0, 1))},
		{Color: colorBase, Pos: 1},
	}

	if layout.Shape.fx == nil {
		layout.Shape.fx = &shapeEffects{}
	}
	layout.Shape.fx.Gradient = &GradientDef{
		Stops:     stops,
		Type:      GradientLinear,
		Direction: GradientToRight,
	}
}
