package gui

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"
)

// svgAnimStaleNs is the heartbeat threshold for detecting an
// animated SVG whose shape has been removed from the layout
// tree. Generous enough to survive a few slow frames (e.g.
// expensive relayout of a large list).
const svgAnimStaleNs = 2 * int64(time.Second)

// SvgCfg configures an SVG view component.
type SvgCfg struct {
	OnClick func(*Layout, *Event, *Window)

	ID       string
	FileName string // SVG file path
	SvgData  string // OR inline SVG string

	// HoveredElementID / FocusedElementID drive CSS :hover / :focus
	// pseudo-class matching. Set non-empty to flag the matching SVG
	// element id as hovered/focused; cascade re-runs and the parsed
	// result is cached separately per (id, state). Apps that want
	// mouse-driven hover should hit-test paths in the cached result
	// and feed the discovered element id back here on the next render.
	HoveredElementID string
	FocusedElementID string

	// Accessibility
	A11YLabel       string
	A11YDescription string
	Padding         Opt[Padding]
	Width           float32 // display width
	Height          float32 // display height

	// FlatnessTolerance, when > 0, overrides the default tessellation
	// tolerance floor (0.15 viewBox units). Higher = coarser triangles
	// = lower vertex count. Cached separately per value.
	FlatnessTolerance float32

	Color     Color // override fill (for monochrome icons)
	Sizing    Sizing
	NoAnimate bool // disable SMIL animation (default: animated)
}

// svgView implements View for SVG rendering.
type svgView struct {
	cfg SvgCfg
}

// Svg creates an SVG view from file or inline data.
func Svg(cfg SvgCfg) View {
	return &svgView{cfg: cfg}
}

func (sv *svgView) Content() []View { return nil }

func (sv *svgView) GenerateLayout(w *Window) Layout {
	c := &sv.cfg
	svgSrc := c.FileName
	if svgSrc == "" {
		svgSrc = c.SvgData
	}

	width := c.Width
	height := c.Height

	if width <= 0 || height <= 0 {
		natW, natH, err := w.GetSvgDimensions(svgSrc)
		if err != nil {
			log.Printf("svg: %v", err)
			return svgErrorLayout(svgSrc, w)
		}
		if width <= 0 {
			width = natW
		}
		if height <= 0 {
			height = natH
		}
	}

	var svgOpts *SvgParseOpts
	if c.FlatnessTolerance > 0 || c.HoveredElementID != "" ||
		c.FocusedElementID != "" {
		svgOpts = &SvgParseOpts{
			FlatnessTolerance: c.FlatnessTolerance,
			HoveredElementID:  c.HoveredElementID,
			FocusedElementID:  c.FocusedElementID,
		}
	}

	var cached *CachedSvg
	var err error
	if svgOpts != nil {
		cached, err = w.LoadSvgWithOpts(svgSrc, width, height, *svgOpts)
	} else {
		cached, err = w.LoadSvg(svgSrc, width, height)
	}
	if err != nil {
		log.Printf("svg: %v", err)
		return svgErrorLayout(svgSrc, w)
	}

	// Register animation loop for animated SVGs.
	if cached.HasAnimations && !c.NoAnimate {
		animHash := cached.AnimHash
		animSeen := StateMap[string, int64](
			w, nsSvgAnimSeen, capImageCache)
		animSeen.Set(animHash, time.Now().UnixNano())
		animID := "svg_anim:" + animHash
		if !w.hasAnimationLocked(animID) {
			w.animationAdd(&Animate{
				AnimID:  animID,
				Delay:   animationCycle,
				Repeat:  true,
				Refresh: AnimationRefreshRenderOnly,
				Callback: func(an *Animate, w *Window) {
					seenMap := StateMap[string, int64](
						w, nsSvgAnimSeen, capImageCache)
					seen, ok := seenMap.Get(animHash)
					if !ok {
						an.stopped = true
						return
					}
					if time.Now().UnixNano()-seen > svgAnimStaleNs {
						an.stopped = true
						return
					}
					w.RequestRenderOnly()
				},
			})
		}
	}

	var events *eventHandlers
	onClick := leftClickOnly(c.OnClick)
	if onClick != nil {
		events = &eventHandlers{
			OnClick: onClick,
		}
	}
	layout := Layout{
		Shape: &Shape{
			shapeType: shapeSVG,
			ID:        c.ID,
			A11YRole:  AccessRoleImage,
			A11Y: makeA11YInfo(
				a11yLabel(c.A11YLabel, c.ID),
				c.A11YDescription,
			),
			Resource: svgSrc,
			Width:    width,
			Height:   height,
			Color:    c.Color,
			Opacity:  1,
			Sizing:   c.Sizing,
			Padding:  c.Padding.Get(Padding{}),
			events:   events,
			SvgOpts:  svgOpts,
		},
	}
	applyFixedSizingConstraints(layout.Shape)
	return layout
}

// svgErrorLayout returns a magenta error text for missing SVGs.
// Uses basename only to avoid leaking filesystem paths.
func svgErrorLayout(src string, w *Window) Layout {
	name := src
	if !strings.HasPrefix(src, "<") {
		name = filepath.Base(src)
	}
	ts := guiTheme.TextStyleDef
	ts.Color = Magenta
	tv := Text(TextCfg{
		Text:      fmt.Sprintf("[missing: %s]", name),
		TextStyle: ts,
	})
	return tv.GenerateLayout(w)
}
