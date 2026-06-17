package gui

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// ImageCfg configures an image view.
type ImageCfg struct {
	OnClick     func(*Layout, *Event, *Window)
	ClickButton MouseButton // left-click filter; avoids leftClickOnly closure
	OnHover     func(*Layout, *Event, *Window)
	ID          string
	Src         string

	// Accessibility
	A11YLabel       string
	A11YDescription string

	Opacity   Opt[float32]
	Width     float32
	Height    float32
	MinWidth  float32
	MaxWidth  float32
	MinHeight float32
	MaxHeight float32
	BgColor   Color // opaque fill drawn behind image (e.g. white for mermaid PNGs)

	Invisible bool
}

// imageView implements View for image rendering.
type imageView struct {
	cfg ImageCfg
}

// Image creates a new image view. Supports local paths and remote
// http/https URLs. Remote images are cached locally.
func Image(cfg ImageCfg) View {
	if cfg.Invisible {
		return invisibleContainerView()
	}
	cfg.ClickButton = MouseLeft
	return &imageView{cfg: cfg}
}

func (iv *imageView) Content() []View { return nil }

func (iv *imageView) GenerateLayout(w *Window) Layout {
	c := &iv.cfg
	imagePath := c.Src
	if isHTTPURL(c.Src) {
		imagePath = ResolveImageSrc(w, c.Src)
		if imagePath == "" {
			return downloadingPlaceholder(c)
		}
		if strings.HasSuffix(imagePath, ".svg") {
			sv := &svgView{cfg: SvgCfg{
				FileName: imagePath,
				Width:    c.Width,
				Height:   c.Height,
			}}
			return sv.GenerateLayout(w)
		}
	}

	// Data URLs are passed directly to the backend renderer
	// (used by WASM for embedded image assets).
	if !isDataURL(c.Src) {
		if err := ValidateImagePath(imagePath); err != nil {
			log.Printf("image: %v", err)
			return errorTextLayout(c.Src, w)
		}
		if _, err := os.Stat(imagePath); err != nil {
			log.Printf("image: %v", err)
			return errorTextLayout(c.Src, w)
		}
	}

	width := c.Width
	height := c.Height
	if width <= 0 {
		width = 100
	}
	if height <= 0 {
		height = 100
	}

	var events *eventHandlers
	if c.OnClick != nil || c.OnHover != nil {
		events = &eventHandlers{
			OnClick:     c.OnClick,
			ClickButton: c.ClickButton,
			OnHover:     c.OnHover,
		}
	}
	layout := Layout{
		Shape: &Shape{
			shapeType: shapeImage,
			ID:        c.ID,
			A11YRole:  AccessRoleImage,
			A11Y: makeA11YInfo(
				a11yLabel(c.A11YLabel, c.ID),
				c.A11YDescription,
			),
			Resource:  imagePath,
			Color:     c.BgColor,
			Opacity:   c.Opacity.Get(1.0),
			Width:     width,
			MinWidth:  c.MinWidth,
			MaxWidth:  c.MaxWidth,
			Height:    height,
			MinHeight: c.MinHeight,
			MaxHeight: c.MaxHeight,
			events:    events,
		},
	}
	applyFixedSizingConstraints(layout.Shape)
	return layout
}

// downloadingPlaceholder returns a neutral rectangle shown while a
// remote image download is in flight.
func downloadingPlaceholder(c *ImageCfg) Layout {
	width := c.Width
	if width <= 0 {
		width = 100
	}
	height := c.Height
	if height <= 0 {
		height = 100
	}
	layout := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			ID:        c.ID,
			Width:     width,
			Height:    height,
			Color:     guiTheme.ColorBackground,
			Opacity:   c.Opacity.Get(1.0),
		},
	}
	applyFixedSizingConstraints(layout.Shape)
	return layout
}

// errorTextLayout returns a magenta "[missing: src]" text layout.
func errorTextLayout(src string, w *Window) Layout {
	ts := guiTheme.TextStyleDef
	ts.Color = Magenta
	tv := Text(TextCfg{
		Text:      fmt.Sprintf("[missing: %s]", src),
		TextStyle: ts,
	})
	return tv.GenerateLayout(w)
}
