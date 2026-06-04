// Animation stress test: many concurrent tween animations with random
// positions, sizes, colors, shapes, and easing functions.
package main

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const iconStar = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 17.27L18.18 21l-1.64-7.03L22 9.24l-7.19-.61L12 2 9.19 8.63 2 9.24l5.46 4.73L5.82 21z"/></svg>`
const iconHeart = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg>`
const iconFace = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zM12 20c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8zm3.5-9c.83 0 1.5-.67 1.5-1.5S16.33 8 15.5 8 14 8.67 14 9.5s.67 1.5 1.5 1.5zm-7 0c.83 0 1.5-.67 1.5-1.5S9.33 8 8.5 8 7 8.67 7 9.5 7.67 11 8.5 11zm3.5 6.5c2.33 0 4.31-1.46 5.11-3.5H6.89c.8 2.04 2.78 3.5 5.11 3.5z"/></svg>`
const iconBolt = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M11 21h-1l1-7H7.5c-.58 0-.57-.32-.38-.66.19-.34.05-.08.07-.12C8.48 10.94 10.42 7.54 13 3h1l-1 7h3.5c.49 0 .56.33.47.51l-.07.15C12.96 17.55 11 21 11 21z"/></svg>`

type itemKind int

const (
	kindCircle itemKind = iota
	kindRect
	kindSVG
)

type animatedItem struct {
	id    string
	icon  string
	kind  itemKind
	size  float32
	x     float32
	y     float32
	color gui.Color
}

type State struct {
	Items  []animatedItem
	NextID int
}

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &State{},
		Title:  "Animation Stress Test",
		Width:  1000,
		Height: 800,
		OnInit: func(w *gui.Window) {
			addItems(w, 10)
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	state := gui.State[State](w)
	ww, wh := w.WindowSize()

	content := make([]gui.View, len(state.Items))
	for i, item := range state.Items {
		content[i] = renderItem(item)
	}

	return gui.Column(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Spacing:    gui.SomeF(20),
				VAlign:     gui.VAlignMiddle,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      fmt.Sprintf("Active Animations: %d", len(state.Items)),
						TextStyle: gui.CurrentTheme().B1,
					}),
					gui.Button(gui.ButtonCfg{
						Content: []gui.View{gui.Text(gui.TextCfg{Text: "Add 10 Items"})},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							addItems(w, 10)
							e.IsHandled = true
						},
					}),
				},
			}),
			gui.Canvas(gui.ContainerCfg{
				ID:         "canvas",
				Sizing:     gui.FillFill,
				SizeBorder: gui.NoBorder,
				Content:    content,
			}),
		},
	})
}

func renderItem(item animatedItem) gui.View {
	switch item.kind {
	case kindSVG:
		return gui.Column(gui.ContainerCfg{
			ID:         item.id,
			X:          item.x,
			Y:          item.y,
			Width:      item.size,
			Height:     item.size,
			Sizing:     gui.FixedFixed,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Svg(gui.SvgCfg{
					Width:   item.size,
					Height:  item.size,
					Sizing:  gui.FixedFixed,
					SvgData: item.icon,
					Color:   item.color,
				}),
			},
		})
	case kindCircle:
		return gui.Column(gui.ContainerCfg{
			ID:         item.id,
			X:          item.x,
			Y:          item.y,
			Width:      item.size,
			Height:     item.size,
			Sizing:     gui.FixedFixed,
			Color:      item.color,
			Radius:     gui.SomeF(item.size / 2),
			SizeBorder: gui.NoBorder,
		})
	default: // kindRect
		return gui.Column(gui.ContainerCfg{
			ID:         item.id,
			X:          item.x,
			Y:          item.y,
			Width:      item.size,
			Height:     item.size,
			Sizing:     gui.FixedFixed,
			Color:      item.color,
			Radius:     gui.SomeF(item.size / 4),
			SizeBorder: gui.NoBorder,
		})
	}
}

func addItems(w *gui.Window, count int) {
	state := gui.State[State](w)
	ww, wh := w.WindowSize()

	safeW := float32(ww)
	if safeW <= 0 {
		safeW = 800
	}
	safeH := float32(wh)
	if safeH <= 0 {
		safeH = 600
	}

	icons := [4]string{iconStar, iconHeart, iconFace, iconBolt}

	for range count {
		state.NextID++
		id := fmt.Sprintf("item_%d", state.NextID)

		size := rand.Float32()*40 + 20 // 20–60
		x := rand.Float32() * max(safeW-size, 0)
		y := rand.Float32() * max(safeH-100-size, 0)

		kind := itemKind(rand.IntN(3))
		icon := icons[rand.IntN(4)]

		r := uint8(rand.IntN(155) + 100)
		g := uint8(rand.IntN(155) + 100)
		b := uint8(rand.IntN(155) + 100)

		state.Items = append(state.Items, animatedItem{
			id:    id,
			kind:  kind,
			icon:  icon,
			color: gui.RGB(r, g, b),
			size:  size,
			x:     x,
			y:     y,
		})

		startWander(w, id)
	}
}

func startWander(w *gui.Window, id string) {
	state := gui.State[State](w)

	idx := -1
	for i, item := range state.Items {
		if item.id == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}

	currentX := state.Items[idx].x
	currentY := state.Items[idx].y
	size := state.Items[idx].size

	ww, wh := w.WindowSize()
	safeW := float32(ww)
	if safeW <= 0 {
		safeW = 800
	}
	safeH := float32(wh)
	if safeH <= 0 {
		safeH = 600
	}

	destX := rand.Float32() * max(safeW-size, 0)
	destY := rand.Float32() * max(safeH-100-size, 0)

	durationMs := rand.IntN(2000) + 1000 // 1000–3000ms

	easings := [5]gui.EasingFn{
		gui.EaseInOutQuad,
		gui.EaseOutCubic,
		gui.EaseOutBounce,
		gui.EaseOutElastic,
		gui.EaseLinear,
	}
	easing := easings[rand.IntN(5)]

	dur := time.Duration(durationMs) * time.Millisecond

	ax := gui.NewTweenAnimation(id+"_x", currentX, destX,
		func(v float32, w *gui.Window) {
			s := gui.State[State](w)
			for i := range s.Items {
				if s.Items[i].id == id {
					s.Items[i].x = v
					break
				}
			}
		})
	ax.Duration = dur
	ax.Easing = easing
	w.AnimationAdd(ax)

	ay := gui.NewTweenAnimation(id+"_y", currentY, destY,
		func(v float32, w *gui.Window) {
			s := gui.State[State](w)
			for i := range s.Items {
				if s.Items[i].id == id {
					s.Items[i].y = v
					break
				}
			}
		})
	ay.Duration = dur
	ay.Easing = easing
	ay.OnDone = func(w *gui.Window) {
		startWander(w, id)
	}
	w.AnimationAdd(ay)
}
