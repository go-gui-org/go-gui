// Package main implements the examples/explorer gallery app (issue #438).
package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// ExplorerApp is the window state for the explorer.
type ExplorerApp struct {
	Examples    []ExampleMeta
	Filter      string
	Selected    string
	StatusMsg   string
	SelectedTag string
	Root        string
	Runner      *Runner
}

var globalRunner = NewRunner()

func main() {
	var (
		rootFlag       = flag.String("root", "", "examples root (default: auto-detect)")
		smokeFlag      = flag.Bool("smoke", false, "run discovery and exit 0 if ok")
		screenshotFlag = flag.String("screenshot", "", "write screenshot and exit")
	)
	flag.Parse()

	if *screenshotFlag != "" {
		root := *rootFlag
		if root == "" {
			root = detectExamplesRoot()
		}
		metas, _ := Discover(root)
		selected := ""
		if len(metas) > 0 {
			selected = metas[0].Name
		}
		app := &ExplorerApp{
			Examples: metas,
			Selected: selected,
			Root:     root,
			Runner:   globalRunner,
		}
		w := gui.NewWindow(gui.WindowCfg{
			State:  app,
			Title:  "Examples Explorer",
			Width:  1100,
			Height: 700,
			OnInit: func(w *gui.Window) {
				w.UpdateView(explorerView)
			},
		})
		if err := soft.RenderToPNG(w, 2, *screenshotFlag); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}

	root := *rootFlag
	if root == "" {
		root = detectExamplesRoot()
	}

	// Smoke mode: discovery only, no window.
	if *smokeFlag {
		metas, err := Discover(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "discover: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("discovered %d examples from %s\n", len(metas), root)
		missingTitle := 0
		for _, m := range metas {
			if m.Title == "" {
				missingTitle++
				fmt.Printf("  missing title: %s\n", m.Name)
			}
		}
		if len(metas) < 60 {
			fmt.Fprintf(os.Stderr, "expected ~62 examples, got %d\n", len(metas))
			os.Exit(1)
		}
		if missingTitle > 0 {
			fmt.Fprintf(os.Stderr, "missing titles: %d\n", missingTitle)
			os.Exit(1)
		}
		fmt.Println("smoke ok")
		os.Exit(0)
	}

	gui.SetTheme(gui.ThemeDark)

	metas, err := Discover(root)
	if err != nil {
		// Non-fatal: show empty list with error status.
		metas = nil
		_ = err
	}
	// Default selection is first example.
	selected := ""
	if len(metas) > 0 {
		selected = metas[0].Name
	}

	app := &ExplorerApp{
		Examples: metas,
		Selected: selected,
		Root:     root,
		Runner:   globalRunner,
	}
	if err != nil {
		app.StatusMsg = fmt.Sprintf("discover failed (%s): %v", root, err)
	}

	guiApp := gui.NewApp()
	guiApp.ExitMode = gui.ExitOnMainClose

	w := gui.NewWindow(gui.WindowCfg{
		State:  app,
		Title:  "Examples Explorer",
		Width:  1100,
		Height: 700,
		OnInit: func(w *gui.Window) {
			w.UpdateView(explorerView)
		},
	})
	// Ensure child processes are killed when window closes.
	origOnClose := w.Config.OnCloseRequest
	_ = origOnClose
	backend.RunApp(guiApp, w)
	// Cleanup after backend exits.
	globalRunner.KillAll()
}

func detectExamplesRoot() string {
	candidates := []string{
		"examples",
		"../examples",
		"../../examples",
		filepath.Join(".", "examples"),
	}
	// Also try repo root relative to cwd.
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "examples"),
			filepath.Join(wd, "../examples"),
		)
		// Walk up to 3 levels.
		dir := wd
		for range 3 {
			dir = filepath.Dir(dir)
			candidates = append(candidates, filepath.Join(dir, "examples"))
		}
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			// Verify it contains at least one known example.
			if _, err := os.Stat(filepath.Join(c, "calculator")); err == nil {
				return c
			}
			// Accept any directory with entries.
			return c
		}
	}
	return "examples"
}

func explorerView(w *gui.Window) gui.View {
	app := gui.State[ExplorerApp](w)
	// Refresh Running states lazily each frame via IsRunning; no polling needed
	// beyond the frame itself — filter and selection drive re-render.

	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.NoPadding,
		Spacing: gui.NoSpacing,
		Content: []gui.View{
			leftPane(w, app),
			rightPane(w, app),
		},
	})
}

func leftPane(w *gui.Window, app *ExplorerApp) gui.View {
	t := gui.CurrentTheme()
	filtered := filteredExamples(app)

	// Collect tags for chip filter.
	allTags := allTagsSorted(app.Examples)

	return gui.Column(gui.ContainerCfg{
		ID:      "explorer-left",
		Width:   340,
		Sizing:  gui.FixedFill,
		Color:   t.ColorPanel,
		Padding: gui.NewPadding(12, 12, 12, 12),
		Spacing: gui.SomeF(8),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Examples", TextStyle: t.B3}),
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("%d examples (%d filtered)", len(app.Examples), len(filtered)),
				TextStyle: t.N4,
			}),
			gui.Input(gui.InputCfg{
				ID:          "explorer-filter",
				Sizing:      gui.FillFit,
				Text:        app.Filter,
				Placeholder: "Filter by name or tag…",
				OnTextChanged: func(text string, ctx gui.EventCtx) {
					gui.State[ExplorerApp](ctx.Window).Filter = text
				},
			}),
			tagChips(allTags, app),
			gui.Column(gui.ContainerCfg{
				ID:            "explorer-list",
				Scrollable:    true,
				Sizing:        gui.FillFill,
				Padding:       gui.NewPadding(0, t.ScrollbarStyle.Size+4, 0, 0),
				Spacing:       gui.SomeF(2),
				ScrollbarCfgY: &gui.ScrollbarCfg{GapEdge: gui.SomeF(3)},
				Content:       exampleRows(filtered, app),
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				HAlign:  gui.HAlignRight,
				Spacing: gui.SomeF(8),
				Content: []gui.View{
					gui.TextButton("explorer-refresh", "Refresh", func(ctx gui.EventCtx) {
						a := gui.State[ExplorerApp](ctx.Window)
						metas, err := Discover(a.Root)
						if err != nil {
							a.StatusMsg = fmt.Sprintf("refresh failed: %v", err)
						} else {
							a.Examples = metas
							a.StatusMsg = fmt.Sprintf("refreshed %d examples", len(metas))
						}
						ctx.Window.UpdateWindow()
					}),
					gui.ThemePicker(gui.ThemePickerCfg{
						ID:        "explorer-theme",
						Focusable: true,
					}),
				},
			}),
		},
	})
}

func tagChips(tags []string, app *ExplorerApp) gui.View {
	if len(tags) == 0 {
		return gui.Column(gui.ContainerCfg{Sizing: gui.FillFit})
	}
	// Show All + first 12 tags to avoid overflow.
	limit := 12
	if len(tags) > limit {
		tags = tags[:limit]
	}
	chips := make([]gui.View, 0, len(tags)+1)
	// All chip
	allSelected := app.SelectedTag == ""
	chips = append(chips, chipView("All", allSelected, func(ctx gui.EventCtx) {
		gui.State[ExplorerApp](ctx.Window).SelectedTag = ""
		ctx.Window.UpdateWindow()
	}))
	for _, tag := range tags {
		selected := app.SelectedTag == tag
		chips = append(chips, chipView(tag, selected, func(ctx gui.EventCtx) {
			a := gui.State[ExplorerApp](ctx.Window)
			if a.SelectedTag == tag {
				a.SelectedTag = ""
			} else {
				a.SelectedTag = tag
			}
			ctx.Window.UpdateWindow()
		}))
	}
	return gui.Wrap(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(6),
		Content: chips,
	})
}

func chipView(label string, selected bool, onClick func(gui.EventCtx)) gui.View {
	id := gui.ScopeID("explorer", "chip", label)
	variant := gui.ButtonGhost
	if selected {
		variant = gui.ButtonPrimary
	}
	return gui.TextButtonVariant(id, label, variant, onClick)
}

func exampleRows(examples []ExampleMeta, app *ExplorerApp) []gui.View {
	rows := make([]gui.View, 0, len(examples))
	for _, ex := range examples {
		selected := ex.Name == app.Selected
		running := app.Runner != nil && app.Runner.IsRunning(ex.Name)
		label := ex.Title
		if label == "" {
			label = ex.Name
		}
		suffix := ""
		if running {
			suffix = " ●"
		}
		// Use Button for selectable rows.
		bg := gui.Color{}
		if selected {
			bg = gui.CurrentTheme().ColorSelect
		}
		rows = append(rows, gui.Button(gui.ButtonCfg{
			ID:      gui.ScopeID("explorer", "row", ex.Name),
			Sizing:  gui.FillFit,
			HAlign:  gui.Some(gui.HAlignLeft),
			Padding: gui.NewPadding(6, 8, 6, 8),
			Color:   bg,
			Radius:  gui.SomeF(6),
			OnClick: func(ctx gui.EventCtx) {
				a := gui.State[ExplorerApp](ctx.Window)
				a.Selected = ex.Name
				a.StatusMsg = ""
				ctx.Window.ScrollVerticalTo("explorer-detail", 0)
				ctx.Consume()
				ctx.Window.UpdateWindow()
			},
			Content: []gui.View{
				gui.Column(gui.ContainerCfg{
					Sizing:  gui.FillFit,
					Spacing: gui.SomeF(2),
					Content: []gui.View{
						gui.Text(gui.TextCfg{
							Text:      label + suffix,
							TextStyle: gui.CurrentTheme().N4,
							Mode:      gui.TextModeSingleLine,
						}),
						gui.Text(gui.TextCfg{
							Text:      strings.Join(ex.Tags, ", "),
							TextStyle: gui.CurrentTheme().TextStyleSecondary,
							Mode:      gui.TextModeSingleLine,
						}),
					},
				}),
			},
		}))
	}
	if len(rows) == 0 {
		rows = append(rows, gui.Text(gui.TextCfg{
			Text:      "No examples match.",
			TextStyle: gui.CurrentTheme().TextStyleSecondary,
		}))
	}
	return rows
}

func rightPane(w *gui.Window, app *ExplorerApp) gui.View {
	t := gui.CurrentTheme()
	meta := selectedMeta(app)
	if meta == nil {
		return gui.Column(gui.ContainerCfg{
			ID:      "explorer-detail",
			Sizing:  gui.FillFill,
			Padding: gui.NewPadding(16, 16, 16, 16),
			HAlign:  gui.HAlignCenter,
			VAlign:  gui.VAlignMiddle,
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: "Select an example", TextStyle: t.B2}),
			},
		})
	}

	running := app.Runner != nil && app.Runner.IsRunning(meta.Name)
	var runLabel, runID string
	var runVariant gui.ButtonVariant
	var runDisabled bool
	switch {
	case !meta.Runnable:
		runLabel = "Run (unavailable)"
		runDisabled = true
	case running:
		runLabel = "Running…"
		runDisabled = true
	default:
		runLabel = "Run"
		runVariant = gui.ButtonPrimary
	}
	runID = gui.ScopeID("explorer", "run", meta.Name)

	// Detail content.
	var detailContent []gui.View

	// Title + Run on one row, Run at right edge.
	runRow := []gui.View{
		gui.Button(gui.ButtonCfg{
			ID:       runID,
			Label:    runLabel,
			Variant:  runVariant,
			Disabled: runDisabled,
			OnClick: func(ctx gui.EventCtx) {
				a := gui.State[ExplorerApp](ctx.Window)
				m := selectedMeta(a)
				if m == nil {
					return
				}
				if err := a.Runner.Start(m.Name, m.Runnable); err != nil {
					a.StatusMsg = err.Error()
				} else {
					a.StatusMsg = "Started " + m.Name
				}
				ctx.Consume()
				ctx.Window.UpdateWindow()
			},
		}),
	}

	detailContent = append(detailContent, gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		VAlign:  gui.VAlignMiddle,
		Padding: gui.NewPadding(0, gui.PadMedium, 0, gui.PadMedium),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: meta.Title, TextStyle: t.B2}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				HAlign:  gui.HAlignRight,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.SomeF(8),
				Content: runRow,
			}),
		},
	}))

	// Screenshot — bordered so white/transparent previews read against
	// the dark detail background.
	detailContent = append(detailContent, screenshotView(meta))

	// Full README body below separator (if exists).
	if body := readmeBody(meta); body != "" {
		detailContent = append(detailContent,
			gui.Separator(gui.SeparatorCfg{}),
			w.Markdown(gui.MarkdownCfg{
				Source: body,
				Style:  gui.DefaultMarkdownStyle(),
			}),
		)
	}

	return gui.Column(gui.ContainerCfg{
		ID:            "explorer-detail",
		Scrollable:    true,
		Sizing:        gui.FillFill,
		Padding:       gui.NewPadding(16, 16, 20, 16),
		Spacing:       gui.SomeF(4),
		ScrollbarCfgY: &gui.ScrollbarCfg{GapEdge: gui.SomeF(3)},
		Content:       detailContent,
	})
}

func screenshotView(meta *ExampleMeta) gui.View {
	t := gui.CurrentTheme()
	if meta.HasScreenshot && meta.ScreenshotPath != "" {
		// Prefer absolute path so Image's os.Stat succeeds regardless of cwd.
		p := meta.ScreenshotPath
		if !filepath.IsAbs(p) {
			if abs, err := filepath.Abs(p); err == nil {
				p = abs
			}
		}
		// Verify file still exists before emitting Image (avoids log spam).
		if _, err := os.Stat(p); err == nil {
			// Size the preview to the image's aspect, fitting within a
			// generous detail-pane budget so the screenshot is actually
			// useful (not the 100×100 default). The file is captured at
			// scale 2, so logical size is half the device pixels.
			w, h := previewDisplaySize(p, 700, 560)
			return gui.Row(gui.ContainerCfg{
				Sizing: gui.FillFit,
				HAlign: gui.HAlignCenter,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Sizing:      gui.FitFit,
						Radius:      gui.SomeF(0),
						SizeBorder:  gui.SomeF(1),
						ColorBorder: t.ColorBorder,
						Padding:     gui.PaddingNone,
						Content: []gui.View{
							gui.Image(gui.ImageCfg{
								ID:        gui.ScopeID("explorer", "shot", meta.Name),
								Src:       p,
								Width:     w,
								Height:    h,
								MaxWidth:  w,
								MaxHeight: h,
							}),
						},
					}),
				},
			})
		}
	}
	// Placeholder
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Height:     160,
		Color:      t.ColorPanel,
		Radius:     gui.SomeF(8),
		SizeBorder: gui.SomeF(1),
		HAlign:     gui.HAlignCenter,
		VAlign:     gui.VAlignMiddle,
		Padding:    gui.NewPadding(12, 12, 12, 12),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "No preview yet",
				TextStyle: t.TextStyleSecondary,
			}),
			gui.Text(gui.TextCfg{
				Text:      explainMissing(meta),
				TextStyle: t.TextStyleLabel,
				Mode:      gui.TextModeWrap,
			}),
		},
	})
}

func previewDisplaySize(path string, maxW, maxH float32) (float32, float32) {
	// Default to the budget itself so a missing or corrupt file still
	// shows a large placeholder rather than the 100×100 Image default.
	fallbackW, fallbackH := maxW, maxH
	f, err := os.Open(path) // #nosec G304 -- path from Discover
	if err != nil {
		return fallbackW, fallbackH
	}
	defer func() { _ = f.Close() }()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return fallbackW, fallbackH
	}
	// Captured at scale 2 → logical is half device.
	lw := float32(cfg.Width) / 2
	lh := float32(cfg.Height) / 2
	if lw <= 0 || lh <= 0 {
		return fallbackW, fallbackH
	}
	scale := min(maxW/lw, maxH/lh)
	// Keep at least the logical size; never shrink below 1× for tiny
	// captures, but clamp to the budget so large windows don't overflow.
	if scale < 1 {
		// For large images we shrink to fit; for small we upscale to be useful.
		// No lower clamp — large captures intentionally shrink.
	} else if scale > 3 {
		scale = 3 // cap upscaling to 3× to avoid blowing up tiny icons
	}
	w := lw * scale
	h := lh * scale
	// Clamp to max to handle rounding.
	if w > maxW {
		w = maxW
	}
	if h > maxH {
		h = maxH
	}
	if w < 100 {
		w = 100
	}
	if h < 100 {
		h = 100
	}
	return w, h
}

func explainMissing(meta *ExampleMeta) string {
	if !meta.Runnable {
		return "This example needs a platform toolchain."
	}
	if meta.Name == "custom_shader" {
		return "Custom shaders need a GPU and cannot be captured headlessly."
	}
	return "Run the screenshot capture tool to generate a preview."
}

func readmeBody(meta *ExampleMeta) string {
	if !meta.HasReadme {
		return ""
	}
	data, err := os.ReadFile(meta.ReadmePath) // #nosec G304 -- path from Discover
	if err != nil {
		return ""
	}
	content := string(data)
	// Show body after first "---" if present, else full content.
	if _, after, ok := strings.Cut(content, "\n---"); ok {
		rest := after
		// Skip leading newlines
		rest = strings.TrimLeft(rest, "\n\r ")
		if rest != "" {
			return rest
		}
	}
	return content
}

func selectedMeta(app *ExplorerApp) *ExampleMeta {
	for i := range app.Examples {
		if app.Examples[i].Name == app.Selected {
			return &app.Examples[i]
		}
	}
	if len(app.Examples) > 0 {
		return &app.Examples[0]
	}
	return nil
}

func filteredExamples(app *ExplorerApp) []ExampleMeta {
	q := strings.ToLower(strings.TrimSpace(app.Filter))
	tag := app.SelectedTag
	if q == "" && tag == "" {
		return app.Examples
	}
	var out []ExampleMeta
	for _, ex := range app.Examples {
		if tag != "" && !hasTag(ex.Tags, tag) {
			continue
		}
		if q != "" {
			hay := strings.ToLower(ex.Name + " " + ex.Title + " " + ex.Framework + " " + strings.Join(ex.Tags, " ") + " " + ex.Description)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, ex)
	}
	return out
}

func hasTag(tags []string, needle string) bool {
	return slices.Contains(tags, needle)
}

func allTagsSorted(examples []ExampleMeta) []string {
	set := make(map[string]struct{})
	for _, ex := range examples {
		for _, t := range ex.Tags {
			set[t] = struct{}{}
		}
	}
	tags := make([]string, 0, len(set))
	for t := range set {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}
