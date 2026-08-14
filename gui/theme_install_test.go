package gui

import "testing"

// Theme installation is package-global by design (the installed theme is
// a frame-scoped cache), so these tests must not run in parallel with
// each other or with anything else that installs a theme.

// themeWithPanel returns a theme whose ColorPanel is c, so a rendered
// container's color identifies which theme was installed.
func themeWithPanel(t *testing.T, name string, c Color) Theme {
	t.Helper()
	cfg := themeDarkCfg
	cfg.Name = name
	cfg.ColorPanel = c
	return ThemeMaker(cfg)
}

// panelView renders one container colored from the installed theme.
func panelView(w *Window) View {
	return Column(ContainerCfg{
		ID:    "panel",
		Color: CurrentTheme().ColorPanel,
	})
}

func restoreTheme(t *testing.T) {
	t.Helper()
	prev := currentDefaultTheme()
	t.Cleanup(func() { SetTheme(prev) })
}

func TestWindowThemeIsolated(t *testing.T) {
	restoreTheme(t)
	red := themeWithPanel(t, "red", RGB(200, 0, 0))
	blue := themeWithPanel(t, "blue", RGB(0, 0, 200))

	w1 := NewTestWindow(WindowCfg{})
	w2 := NewTestWindow(WindowCfg{})
	w1.SetTheme(red)
	w2.SetTheme(blue)

	// Frames run one window at a time, exactly as the backend loop does.
	got1 := findByLeafIDTest(t, w1.TestRender(panelView), "panel").Shape.Color
	got2 := findByLeafIDTest(t, w2.TestRender(panelView), "panel").Shape.Color
	if got1 != red.ColorPanel {
		t.Errorf("w1 panel = %v, want %v", got1, red.ColorPanel)
	}
	if got2 != blue.ColorPanel {
		t.Errorf("w2 panel = %v, want %v", got2, blue.ColorPanel)
	}

	// And back again — the install must follow the window, not the
	// most recent SetTheme call.
	if got := findByLeafIDTest(t, w1.TestRender(nil), "panel").Shape.Color; got != red.ColorPanel {
		t.Errorf("w1 panel after w2 frame = %v, want %v",
			got, red.ColorPanel)
	}
}

func TestWindowThemeFollowsDefault(t *testing.T) {
	restoreTheme(t)
	green := themeWithPanel(t, "green", RGB(0, 200, 0))
	pinned := themeWithPanel(t, "pinned", RGB(200, 0, 200))

	follower := NewTestWindow(WindowCfg{})
	pinnedWin := NewTestWindow(WindowCfg{})
	pinnedWin.SetTheme(pinned)

	SetTheme(green)

	if got := follower.Theme().id; got != green.id {
		t.Errorf("follower theme id = %d, want %d", got, green.id)
	}
	if got := pinnedWin.Theme().id; got != pinned.id {
		t.Errorf("pinned theme id = %d, want %d", got, pinned.id)
	}
	if got := findByLeafIDTest(t, follower.TestRender(panelView), "panel").Shape.Color; got != green.ColorPanel {
		t.Errorf("follower panel = %v, want %v", got, green.ColorPanel)
	}
	if got := findByLeafIDTest(t, pinnedWin.TestRender(panelView), "panel").Shape.Color; got != pinned.ColorPanel {
		t.Errorf("pinned panel = %v, want %v", got, pinned.ColorPanel)
	}
}

func TestInstallThemeSkipsWhenUnchanged(t *testing.T) {
	restoreTheme(t)
	th := themeWithPanel(t, "steady", RGB(1, 2, 3))
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(th)
	w.TestRender(panelView)

	// Corrupt one mirror. installTheme must not rewrite it, proving the
	// fast path took over once the theme was already installed.
	saved := defaultButtonStyle.Color
	defaultButtonStyle.Color = RGB(9, 9, 9)
	w.installTheme()
	got := defaultButtonStyle.Color
	defaultButtonStyle.Color = saved
	if got != RGB(9, 9, 9) {
		t.Error("installTheme rewrote mirrors for an already-installed theme")
	}
}

func TestThemedScopesSubtree(t *testing.T) {
	restoreTheme(t)
	outer := themeWithPanel(t, "outer", RGB(10, 0, 0))
	inner := themeWithPanel(t, "inner", RGB(0, 10, 0))

	w := NewTestWindow(WindowCfg{})
	w.SetTheme(outer)

	root := w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			ID: "root",
			Content: []View{
				Themed(inner, func(*Window) View {
					return Column(ContainerCfg{
						ID:    "scoped",
						Color: CurrentTheme().ColorPanel,
					})
				}),
				// Built after Themed's node, so it also proves the
				// theme was restored on the way out.
				viewFunc(func(*Window) View {
					return Column(ContainerCfg{
						ID:    "sibling",
						Color: CurrentTheme().ColorPanel,
					})
				}),
			},
		})
	})

	scoped := findByLeafIDTest(t, root, "scoped")
	sibling := findByLeafIDTest(t, root, "sibling")
	if scoped.Shape.Color != inner.ColorPanel {
		t.Errorf("scoped = %v, want %v",
			scoped.Shape.Color, inner.ColorPanel)
	}
	if sibling.Shape.Color != outer.ColorPanel {
		t.Errorf("sibling = %v, want %v",
			sibling.Shape.Color, outer.ColorPanel)
	}
	if CurrentTheme().id != outer.id {
		t.Error("Themed did not restore the window theme after the frame")
	}
}

func TestThemedNests(t *testing.T) {
	restoreTheme(t)
	a := themeWithPanel(t, "a", RGB(1, 0, 0))
	b := themeWithPanel(t, "b", RGB(0, 1, 0))
	c := themeWithPanel(t, "c", RGB(0, 0, 1))

	w := NewTestWindow(WindowCfg{})
	w.SetTheme(a)

	root := w.TestRender(func(*Window) View {
		return Themed(b, func(*Window) View {
			return Column(ContainerCfg{
				ID:    "b",
				Color: CurrentTheme().ColorPanel,
				Content: []View{
					Themed(c, func(*Window) View {
						return Column(ContainerCfg{
							ID:    "c",
							Color: CurrentTheme().ColorPanel,
						})
					}),
					viewFunc(func(*Window) View {
						return Column(ContainerCfg{
							ID:    "after-c",
							Color: CurrentTheme().ColorPanel,
						})
					}),
				},
			})
		})
	})

	if got := findByLeafIDTest(t, root, "c").Shape.Color; got != c.ColorPanel {
		t.Errorf("inner = %v, want %v", got, c.ColorPanel)
	}
	if got := findByLeafIDTest(t, root, "after-c").Shape.Color; got != b.ColorPanel {
		t.Errorf("after inner = %v, want %v", got, b.ColorPanel)
	}
}

func TestThemedKeepsExplicitOverride(t *testing.T) {
	restoreTheme(t)
	inner := themeWithPanel(t, "inner-ovr", RGB(0, 10, 0))
	want := RGB(123, 45, 67)

	w := NewTestWindow(WindowCfg{})
	root := w.TestRender(func(*Window) View {
		return Themed(inner, func(*Window) View {
			return Column(ContainerCfg{ID: "explicit", Color: want})
		})
	})
	if got := findByLeafIDTest(t, root, "explicit").Shape.Color; got != want {
		t.Errorf("explicit color = %v, want %v", got, want)
	}
}

func TestThemedNilBuild(t *testing.T) {
	restoreTheme(t)
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			ID:      "root",
			Content: []View{Themed(ThemeLight, nil)},
		})
	})
}

func TestThemedZeroIDThemeInstalls(t *testing.T) {
	restoreTheme(t)
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(themeWithPanel(t, "base", RGB(0, 0, 10)))
	// A hand-built Theme has id 0, which must bypass the install fast
	// path rather than masquerade as the already-installed theme.
	raw := Theme{ColorPanel: RGB(1, 2, 3)}

	root := w.TestRender(func(*Window) View {
		return Themed(raw, func(*Window) View {
			return Column(ContainerCfg{
				ID:    "raw",
				Color: CurrentTheme().ColorPanel,
			})
		})
	})
	if got := findByLeafIDTest(t, root, "raw").Shape.Color; got != raw.ColorPanel {
		t.Errorf("raw theme panel = %v, want %v", got, raw.ColorPanel)
	}
	if CurrentTheme().id != w.Theme().id {
		t.Error("Themed did not restore the window theme after the frame")
	}
}

// findByLeafIDTest locates a layout node by its leaf ID, ignoring the
// effective-ID scope the resolve pass stamps. Deliberate: these tests
// build one shape per known leaf and assert its rendered style, and
// spelling the scoped key ("root:scoped") would bake ancestor-scoping
// rules into the expectations. Layout.FindByID is the API for real
// lookups, which key on the effective ID.
func findByLeafIDTest(t *testing.T, root *Layout, id string) *Layout {
	t.Helper()
	var walk func(*Layout) *Layout
	walk = func(l *Layout) *Layout {
		if l == nil {
			return nil
		}
		if l.Shape != nil && l.Shape.ID == id {
			return l
		}
		for i := range l.Children {
			if found := walk(&l.Children[i]); found != nil {
				return found
			}
		}
		return nil
	}
	found := walk(root)
	if found == nil {
		t.Fatalf("layout %q not found", id)
	}
	return found
}
