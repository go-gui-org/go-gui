// This example demonstrates a scrollable family tree with clickable names and
// orthogonal connector lines (advanced: canvas layout).
//
// The family tree example answers issue #582: how to build a large diagram
// that scrolls and whose labels react to clicks. It combines three parts:
//
//   - gui.Canvas, a container that does not arrange its children. Each child
//     sits at its own X/Y, so the app decides where every name goes.
//   - gui.DrawCanvas, drawn first so it sits under the names. It paints the
//     connector lines.
//   - A scrollable gui.Column around the canvas. It scrolls on both axes.
//
// The names are real buttons, not text painted on the draw canvas. That gives
// hover, focus, keyboard activation and accessibility for free, and no hit
// testing code in the app.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// Geometry of the tree, in logical pixels.
const (
	nodeW      float32 = 120 // width of one name box
	nodeH      float32 = 36  // height of one name box
	coupleGap  float32 = 24  // space between a person and their spouse
	siblingGap float32 = 32  // space between neighbouring subtrees
	rowGap     float32 = 64  // vertical space between generations
	margin     float32 = 24  // empty border around the whole tree
	lineWidth  float32 = 2   // stroke width of the connector lines
)

// Person is one entry in the family. Spouse is optional. Children belong to
// the couple, so the connector starts between the two partners.
type Person struct {
	Name     string
	Spouse   string
	Children []*Person
}

// node is one name box, already placed. The ID and the click handler are
// built once when the tree is laid out, so rendering a frame does not
// allocate a new string or closure per name.
type node struct {
	name    string
	id      string
	x, y    float32
	onClick func(gui.EventCtx)
}

// App holds the laid-out tree and the current selection.
type App struct {
	nodes []node

	// segments holds one polyline per connector, as flat x,y pairs in
	// canvas coordinates. OnDraw only replays them.
	segments [][]float32

	// width and height are the bounds of the whole tree. The canvas must
	// be given this size explicitly: a container that does not arrange
	// its children measures its scroll range from child sizes only and
	// ignores child X/Y. Without it, names past the window edge could not
	// be scrolled into view.
	width, height float32

	// selected is the index into nodes of the last clicked name, or -1.
	selected int

	// lineColor is copied from the theme during view generation. The
	// draw callback runs later, in the render pass, where the theme
	// should not be read directly.
	lineColor gui.Color

	// onDraw is the bound draw method, stored once so each frame does not
	// allocate a new method value.
	onDraw func(*gui.DrawContext)
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  newApp(sampleFamily()),
		Title:  "Family Tree",
		Width:  900,
		Height: 600,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

// sampleFamily returns four generations, wide enough to need horizontal
// scrolling in the default window.
func sampleFamily() *Person {
	return &Person{
		Name: "Arthur", Spouse: "Beatrice",
		Children: []*Person{
			{
				Name: "Charles", Spouse: "Diana",
				Children: []*Person{
					{Name: "Edward", Spouse: "Fiona", Children: []*Person{
						{Name: "George"}, {Name: "Hannah"}, {Name: "Ivy"},
					}},
					{Name: "Julia"},
				},
			},
			{
				Name: "Karen", Spouse: "Louis",
				Children: []*Person{
					{Name: "Martin"},
					{Name: "Nora", Spouse: "Oscar", Children: []*Person{
						{Name: "Peter"}, {Name: "Quinn"},
					}},
					{Name: "Rose", Spouse: "Samuel", Children: []*Person{
						{Name: "Tara"}, {Name: "Umar"}, {Name: "Vera"},
					}},
				},
			},
			{Name: "William", Spouse: "Xena", Children: []*Person{
				{Name: "Yusuf"}, {Name: "Zoe"},
			}},
		},
	}
}

// newApp lays the tree out once. The tree in this example does not change,
// so there is no need to redo the layout every frame.
func newApp(root *Person) *App {
	app := &App{selected: -1}
	app.onDraw = app.drawConnectors
	if root == nil {
		// An empty family draws nothing; the view still renders.
		return app
	}
	treeW := subtreeWidth(root)
	app.place(root, margin, 0)
	app.width = treeW + 2*margin
	app.height = margin + float32(depth(root))*(nodeH+rowGap) - rowGap + margin
	return app
}

// unitWidth is the width of a person's own boxes: one box, or two with a gap
// when there is a spouse.
func unitWidth(p *Person) float32 {
	if p.Spouse == "" {
		return nodeW
	}
	return 2*nodeW + coupleGap
}

// childrenWidth is the total width of all child subtrees placed side by side.
func childrenWidth(p *Person) float32 {
	var total float32
	for i, c := range p.Children {
		if i > 0 {
			total += siblingGap
		}
		total += subtreeWidth(c)
	}
	return total
}

// subtreeWidth is the horizontal space a person and all descendants need.
// A parent wider than its children still reserves its own width.
func subtreeWidth(p *Person) float32 {
	return max(unitWidth(p), childrenWidth(p))
}

// depth counts the generations from p down to the deepest descendant.
func depth(p *Person) int {
	d := 0
	for _, c := range p.Children {
		d = max(d, depth(c))
	}
	return d + 1
}

// place assigns positions to p and its descendants inside the horizontal
// band that starts at left, then records the connector lines. It returns the
// X of the point where a line from the parent should arrive: the top center
// of p's own box (not the spouse's).
func (app *App) place(p *Person, left float32, generation int) float32 {
	sw := subtreeWidth(p)
	y := margin + float32(generation)*(nodeH+rowGap)

	// Center the person (and spouse) over the band.
	ux := left + (sw-unitWidth(p))/2
	app.addNode(p.Name, ux, y)
	if p.Spouse != "" {
		app.addNode(p.Spouse, ux+nodeW+coupleGap, y)
		// Short marriage line between the two partners, at mid height.
		mid := y + nodeH/2
		app.segments = append(app.segments,
			[]float32{ux + nodeW, mid, ux + nodeW + coupleGap, mid})
	}

	if len(p.Children) > 0 {
		// The children's line starts below the couple: from the middle of
		// the marriage line when there is a spouse, otherwise from the
		// bottom of the single box.
		var sx, sy float32
		if p.Spouse != "" {
			sx, sy = ux+nodeW+coupleGap/2, y+nodeH/2
		} else {
			sx, sy = ux+nodeW/2, y+nodeH
		}
		busY := y + nodeH + rowGap/2 // horizontal bus, halfway to the next row
		childY := y + nodeH + rowGap

		// Lay the children out and collect where each line arrives.
		cx := left + (sw-childrenWidth(p))/2
		first, last := float32(0), float32(0)
		for i, c := range p.Children {
			tx := app.place(c, cx, generation+1)
			if i == 0 {
				first = tx
			}
			last = tx
			// Drop from the bus to the child.
			app.segments = append(app.segments, []float32{tx, busY, tx, childY})
			cx += subtreeWidth(c) + siblingGap
		}

		// Drop from the parents to the bus, then the bus itself. The bus
		// spans the children and the parents' drop point, so it stays
		// connected even when the parents are off to one side.
		app.segments = append(app.segments,
			[]float32{sx, sy, sx, busY},
			[]float32{min(first, sx), busY, max(last, sx), busY})
	}
	return ux + nodeW/2
}

// addNode records one name box and builds its ID and click handler.
func (app *App) addNode(name string, x, y float32) {
	i := len(app.nodes)
	app.nodes = append(app.nodes, node{
		name: name,
		// ScopeIDN keeps loop-derived IDs unique without hand-joined
		// strings.
		id: gui.ScopeIDN("person", "", i),
		x:  x,
		y:  y,
		onClick: func(ctx gui.EventCtx) {
			app.selected = i
			ctx.Consume()
		},
	})
}

// drawConnectors paints every connector. Coordinates are relative to the
// draw canvas, which sits at the canvas origin, so they match the X/Y of the
// name boxes exactly.
func (app *App) drawConnectors(dc *gui.DrawContext) {
	for _, s := range app.segments {
		dc.Polyline(s, app.lineColor, lineWidth)
	}
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()
	app.lineColor = theme.ColorBorder

	status := "Click a name."
	if app.selected >= 0 {
		status = "Selected: " + app.nodes[app.selected].name
	}

	// The draw canvas goes first, so the buttons after it paint on top.
	content := make([]gui.View, 0, len(app.nodes)+1)
	content = append(content, gui.DrawCanvas(gui.DrawCanvasCfg{
		Width:  app.width,
		Height: app.height,
		// The lines never change, so one version is enough. A tree that
		// changes would bump this to redraw.
		Version: 1,
		OnDraw:  app.onDraw,
	}))
	for i := range app.nodes {
		n := &app.nodes[i]
		variant := gui.ButtonSecondary
		if i == app.selected {
			variant = gui.ButtonPrimary
		}
		// ButtonCfg has no X/Y, so a plain wrapper carries the position.
		content = append(content, gui.Column(gui.ContainerCfg{
			X:          n.x,
			Y:          n.y,
			Width:      nodeW,
			Height:     nodeH,
			Sizing:     gui.FixedFixed,
			Padding:    gui.PaddingNone,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Button(gui.ButtonCfg{
					ID:      n.id,
					Label:   n.name,
					Variant: variant,
					Sizing:  gui.FillFill,
					OnClick: n.onClick,
				}),
			},
		}))
	}

	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: status}),
			gui.Column(gui.ContainerCfg{
				ID:         "tree",
				Scrollable: true,
				Sizing:     gui.FillFill,
				Padding:    gui.PaddingNone,
				// Clip stops the canvas's fixed width from becoming this
				// column's minimum width. Without it the column grows as
				// wide as the whole tree, and there is nothing left to
				// scroll horizontally.
				Clip: true,
				Content: []gui.View{
					gui.Canvas(gui.ContainerCfg{
						Width:      app.width,
						Height:     app.height,
						Sizing:     gui.FixedFixed,
						Padding:    gui.PaddingNone,
						SizeBorder: gui.NoBorder,
						Content:    content,
					}),
				},
			}),
		},
	})
}
