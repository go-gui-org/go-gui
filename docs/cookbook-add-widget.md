# Adding a new widget

Step-by-step guide for adding a widget to the `gui` package. Uses
`Toggle` as the running example — a checkbox-style widget with focus,
keyboard handling, accessibility, and theme defaults.

## 1. Create the Cfg struct

Every widget has a `*Cfg` struct. Conventions:

- **Zero-initializable** — all fields have usable zero values. Users
  omit what they don't need: `ToggleCfg{Label: "Accept"}`
- **Opt[T] for optional overrides** — `Opt[Padding]`, `Opt[float32]`
  distinguish "not set" from an explicit zero. Use `cfg.Radius.Get(default)`
  in the factory.
- **Common fields** — every interactive widget includes: `ID string`,
  `IDFocus uint32`, `Disabled bool`, `Invisible bool`. Container-like
  widgets add `Sizing Sizing`, `Float bool`, `FloatAnchor FloatAttach`,
  `FloatTieOff FloatAttach`, `Padding Opt[Padding]`, `Radius Opt[float32]`,
  `SizeBorder Opt[float32]`.
- **Callbacks** — one func field per event. Sig: `func(*Layout, *Event, *Window)`.
  Set `e.IsHandled = true` when consumed.
- **`gui:"required"` tag** — fields that must be non-empty get the tag.
  The `requiredid` vet analyzer enforces this at `go vet` time. Only use
  when the widget cannot function without the value (e.g. `FormCfg.ID`).

Minimal example:

```go
// file: gui/view_toggle.go

// ToggleCfg configures a toggle/checkbox button.
type ToggleCfg struct {
    TextStyle      TextStyle
    TextStyleLabel TextStyle
    OnClick        func(*Layout, *Event, *Window)
    ID             string
    Label          string
    TextSelect     string
    TextUnselect   string

    A11YLabel        string
    A11YDescription  string
    Padding          Opt[Padding]
    SizeBorder       Opt[float32]
    Radius           Opt[float32]
    MinWidth         float32
    IDFocus          uint32
    Color            Color
    ColorFocus       Color
    ColorHover       Color
    ColorClick       Color
    ColorBorder      Color
    ColorBorderFocus Color
    ColorSelect      Color
    Disabled         bool
    Invisible        bool
    Selected         bool
}
```

## 2. Write the factory function

Sig: `func WidgetName(cfg WidgetCfg) View`. The function:

1. **Calls applyDefaults** — fills in theme colors, sizes, text styles
   for any field the user didn't set
2. **Reads Opt[T] values** via `.Get(fallback)` to resolve "not set"
3. **Builds a Layout tree** — returns a `ContainerCfg`-based layout
   (usually `Row`, `Column`, or `Canvas`)
4. **Sets a11y** — role, state, label on the root shape
5. **Wires events** — OnClick, OnHover, OnChar, AmendLayout

```go
func Toggle(cfg ToggleCfg) View {
    applyToggleDefaults(&cfg)

    d := &DefaultToggleStyle
    sizeBorder := cfg.SizeBorder.Get(d.SizeBorder)
    radius := cfg.Radius.Get(d.Radius)

    boxColor := cfg.Color
    if cfg.Selected {
        boxColor = cfg.ColorSelect
    }

    txt := cfg.TextSelect
    txtStyle := cfg.TextStyle
    if !cfg.Selected {
        if cfg.TextUnselect == " " {
            txtStyle.Color = ColorTransparent
        } else {
            txt = cfg.TextUnselect
        }
    }

    content := make([]View, 0, 2)
    content = append(content, Row(ContainerCfg{
        Color:       boxColor,
        ColorBorder: cfg.ColorBorder,
        SizeBorder:  Some(sizeBorder),
        Padding:     cfg.Padding,
        Radius:      Some(radius),
        Disabled:    cfg.Disabled,
        Invisible:   cfg.Invisible,
        HAlign:      HAlignCenter,
        VAlign:      VAlignMiddle,
        Content: []View{
            Text(TextCfg{Text: txt, TextStyle: txtStyle}),
        },
    }))

    if len(cfg.Label) > 0 {
        content = append(content,
            Text(TextCfg{Text: cfg.Label, TextStyle: cfg.TextStyleLabel}))
    }

    a11yState := AccessStateNone
    if cfg.Selected {
        a11yState = AccessStateChecked
    }

    return Row(ContainerCfg{
        ID:              cfg.ID,
        IDFocus:         cfg.IDFocus,
        Disabled:        cfg.Disabled,
        Invisible:       cfg.Invisible,
        SizeBorder:      NoBorder,
        Padding:         NoPadding,
        VAlign:          VAlignMiddle,
        A11YRole:        AccessRoleCheckbox,
        A11YState:       a11yState,
        A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.Label),
        A11YDescription: cfg.A11YDescription,
        OnChar:          spacebarToClick(cfg.OnClick),
        OnClick:         leftClickOnly(cfg.OnClick),
        OnHover:         /* hover highlight */,
        AmendLayout:     /* focus highlight */,
        Content:         content,
    })
}
```

### Key patterns

**OnClick wrapping** — wrap raw callbacks with event filters:

```go
// Only fire on left-click (not right/middle).
OnClick: leftClickOnly(cfg.OnClick),

// Fire on Space/Enter keyboard activation (IDFocus required).
OnChar: spacebarToClick(cfg.OnClick),
```

**Hover/focus feedback** — use `OnHover` for mouse hover, `AmendLayout`
for keyboard focus. `AmendLayout` runs every frame after sizing — use it
to update child colors based on `w.IsFocus(idFocus)`.

**a11yLabel helper** — `a11yLabel(userLabel, fallback)` returns the
user-supplied label if non-empty, otherwise the fallback. Always set
on interactive widgets.

## 3. Theme defaults

Most widgets have a default style struct in `theme_defaults.go`:

```go
// In theme_defaults.go

type toggleStyleDefaults struct {
    Color            Color
    ColorFocus       Color
    ColorHover       Color
    ColorClick       Color
    ColorBorder      Color
    ColorBorderFocus Color
    ColorSelect      Color
    Padding          Padding
    SizeBorder       float32
    Radius           float32
    TextStyleNormal  TextStyle
    TextStyleLabel   TextStyle
}

var DefaultToggleStyle = toggleStyleDefaults{...}
```

And in the theme's `init()` or `buildTheme()` function, populate the
light/dark variants. The factory's `applyDefaults` function fills in
any field the user didn't set:

```go
func applyToggleDefaults(cfg *ToggleCfg) {
    d := &DefaultToggleStyle
    if !cfg.Color.IsSet() {
        cfg.Color = d.Color
    }
    if !cfg.Padding.IsSet() {
        cfg.Padding = Some(d.Padding)
    }
    if cfg.TextStyle == (TextStyle{}) {
        cfg.TextStyle = d.TextStyleNormal
    } else {
        cfg.TextStyle = mergeTextStyle(cfg.TextStyle, d.TextStyleNormal)
    }
    // ... repeat for each style field
}
```

If your widget doesn't need custom theme entries, skip this step.
Simple widgets can use `guiTheme` colors directly.

## 4. Write tests

Test file: `gui/view_toggle_test.go`. Test at minimum:

1. **Layout structure** — assert the generated Layout has expected
   shape, axis, children
2. **Config passthrough** — ID, Disabled, Selected flags propagate
   to the correct Shape
3. **Property rendering** — text content, colors, sizing reflect the
   config
4. **Accessibility** — role, state, label are set correctly on the
   root Shape

Use `generateViewLayout(view, &Window{})` to build a Layout tree
headlessly (no backend needed):

```go
func TestToggleIDPassthrough(t *testing.T) {
    w := &Window{}
    layout := generateViewLayout(
        Toggle(ToggleCfg{ID: "tg1", OnClick: noop}), w)
    if layout.Shape.ID != "tg1" {
        t.Errorf("ID: got %s", layout.Shape.ID)
    }
}

func TestToggleSelectedTextContent(t *testing.T) {
    w := &Window{}
    layout := generateViewLayout(Toggle(ToggleCfg{
        OnClick:      noop,
        Selected:     true,
        TextSelect:   "YES",
        TextUnselect: "NO",
    }), w)
    box := layout.Children[0]
    tc := box.Children[0].Shape.TC
    if tc == nil || tc.Text != "YES" {
        t.Errorf("text: got %v, want YES", tc)
    }
}
```

For interactive widgets, also test event handling with a real Window
and `RenderFrameZero` (see `view_input_test.go` for examples).

## 5. Add to showcase

In `examples/showcase/`, create a demo function and register it:

```go
// examples/showcase/demo_toggle.go
func demoToggle(_ *gui.Window) gui.View {
    return gui.Column(gui.ContainerCfg{
        Padding: gui.Some(gui.PaddingMedium),
        Spacing: gui.SomeF(8),
        Content: []gui.View{
            gui.Toggle(gui.ToggleCfg{
                Label:    "Basic toggle",
                OnClick:  func(_ *gui.Layout, e *gui.Event, _ *gui.Window) {
                    e.IsHandled = true
                },
            }),
            gui.Toggle(gui.ToggleCfg{
                Label:    "Pre-selected",
                Selected: true,
                OnClick:  func(_ *gui.Layout, e *gui.Event, _ *gui.Window) {
                    e.IsHandled = true
                },
            }),
            gui.Toggle(gui.ToggleCfg{
                Label:    "Disabled",
                Disabled: true,
                OnClick:  func(_ *gui.Layout, e *gui.Event, _ *gui.Window) {
                    e.IsHandled = true
                },
            }),
        },
    })
}
```

Register in `examples/showcase/detail.go`:

```go
var componentDemos = map[string]func(*gui.Window) gui.View{
    // ...
    "toggle": demoToggle,
    // ...
}
```

## 6. Verify

```bash
go test ./gui/...          # all tests (including your new ones)
go vet ./gui/...           # static analysis (requiredid tag checks)
golangci-lint run ./gui/... # full lint
go run ./examples/showcase/ # visual check
```

## Checklist

- [ ] `gui/view_<name>.go` — Cfg struct + factory function
- [ ] Cfg zero-initializable, Opt[T] for optional fields
- [ ] `gui:"required"` tag on mandatory fields (if any)
- [ ] a11y role, state, label set on the root shape
- [ ] Callbacks wrapped with `leftClickOnly` / `spacebarToClick` where
      appropriate
- [ ] Theme defaults in `theme_defaults.go` (or skip if not needed)
- [ ] `apply<Name>Defaults` function for theme fallback
- [ ] `gui/view_<name>_test.go` — layout structure, config passthrough,
      a11y, property rendering tests
- [ ] `examples/showcase/demo_<name>.go` — demo function
- [ ] `examples/showcase/detail.go` — registered in `componentDemos`
- [ ] `go test ./gui/...` passes
- [ ] `golangci-lint run ./gui/...` passes
