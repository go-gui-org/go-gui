# Developer Experience cheat sheet

go-gui is one of the easiest GUI toolkits to use. A widget is a struct plus a
callback, and most code works the first time. This page lists the few places
where the obvious reading is misleading. Each one is subtle: the code compiles,
the app runs, and one behavior is off. The linked specs explain the reasons, and
the tooling at the bottom finds most of them in your app.

## Focus

A shape is a focus target when it has `Focusable: true` and an `ID` (an empty
auto `ID` already counts: the factory fills a generated leaf before the frame
stamps identity). Tab order also needs `!FocusSkip` and `!Disabled`.

```go
// No ID: takes a generated ID and joins the tab order (#881).
Button{Label: "Save", OnClick: save}

// Inputs are focusable by default. FocusDisabled opts out.
Input{ID: "name", FocusDisabled: true} // not in the tab order
```

Most input controls are focusable by default. They are `Button`,
`ColorChannelSlider`, `ColorPicker`, `ColorPlane`, `ColorWheel`, `Combobox`,
`DatePicker`, `ExpandPanel`, `Input`, `InputDate`, `ListBox`, `NumericInput`,
`RadioButtonGroup`, `Radio`, `Select`, `Slider`, `Switch`, `Toggle`, `Tree`,
`VirtualList`. All of them take a generated ID when `ID` is empty (see "Auto
IDs" below). Everything else opts in with `Focusable: true`, and an opt-in shape
still needs an explicit `ID`. The `requiredid` analyzer and the
`DebugMissingIDs` gate report one without it. See
`docs/specs/focusable-default-input.md`.

## Soft keyboard

A touch platform opens its keyboard when a text field takes focus and closes it
when the field loses focus. The app does not call anything for that. Set the
layout on the field:

```go
gui.Input(gui.InputCfg{ID: "email", Keyboard: gui.KeyboardEmail})
gui.Input(gui.InputCfg{ID: "pin", Keyboard: gui.KeyboardNone}) // app keypad
```

- `Keyboard` is a hint, not a filter. Use `Mask` or `PreTextChange` to limit
  what can be typed.
- `w.HideSoftKeyboard()` keeps focus; `w.ClearFocus()` leaves the field.
- The framework does not move layout for the keyboard. Read
  `w.SoftKeyboardInset()` in the View function if content must stay above it.
- An app keypad uses `FocusDisabled` buttons (a press on them keeps focus) that
  send `EventChar` through `ctx.Window.QueueCommand` + `w.EventFn`. See
  `examples/pin_pad`.

## ID scoping

`Shape.ID` is a leaf. The real identity is the effective ID: the leaf joined
with `:` to the IDs of its ID-bearing ancestors. An ID-less container adds no
scope. A leaf that already contains `:` is absolute. Effective IDs are unique
per window.

```go
// Under Panel{ID:"settings"} this is "settings:name".
// Under Panel{ID:"profile"} it is "profile:name".
Input{ID: "name", ...}
```

Two panels can contain the same leaf ID. Each is a different widget. Compose IDs
with `ScopeID` or `ScopeIDN`, never by hand. Public APIs — `SetFocus`,
`FindByID`, `IsFocus`, `ScrollVerticalTo`, `Test*` — take the effective ID.

Do not spell one by hand. Ask the frame:

```go
ids := w.ResolveID("nav")  // ["detail:nav"] under Panel{ID:"detail"}
w.SetFocus(ids[0])
all := w.EffectiveIDs()    // every identity in the frame, tree order
```

An empty answer means no widget of that name is in the current frame: either the
spelling is wrong or the widget is not rendered. More than one answer means the
leaf is used under more than one scope, which is legal — pick the scope meant.

A part (a row key, a heading slug) must not contain `:`. A composite widget's
inner shape sets `Shape.focusOwner` to the owner's leaf instead of repeating its
`ID`.

Two accessors read the identity, and they answer different questions:

- `shape.idKey()` — the identity of this shape. Read it at every keying site,
  never the bare `Shape.ID`. It returns the resolved `effID`, and falls back to
  the leaf only on a shape that generation never stamped, such as a hand-built
  `Layout` in a test.
- `shape.focusKey()` — the identity whose focus, input and spell-check state
  this shape renders. It returns `focusOwner` when the shape belongs to a
  composite widget, and `idKey()` when it does not. It is empty for a shape that
  participates in neither, which is the signal to draw no caret and no
  selection.

Duplicates are loud: `gui.Debug` reports them, and `(*Window).TestDuplicateIDs`
asserts a clean window. See `docs/specs/widget-id-scoping.md` and
`docs/specs/widget-id-per-scope-uniqueness.md`.

## Auto IDs

A widget whose `ID` is tagged `gui:"auto"` can leave it empty. It then gets a
generated leaf, such as `~input3`, made from three parts:

1. The nearest ancestor with an explicit ID (the scope).
2. The widget kind.
3. The count of earlier widgets of that kind in the scope.

A scrolling or overflowing container without an ID gets one the same way.

```go
// A static form needs no IDs. Tab, click, cursor and scroll all work.
Column(ContainerCfg{Content: []View{
	Input(InputCfg{Label: "Name"}),
	Input(InputCfg{Label: "Email"}),
	Button(ButtonCfg{Label: "Send", OnClick: send}),
}})
```

Set an explicit ID in these cases:

- When code names the widget: `SetFocus`, `FindByID`, `ScrollVerticalTo`, or
  `Test*`. A generated ID is not an address.
- When a widget of the same kind can appear before it at run time: a row
  inserted in a list, or an optional field above a form. The insert moves the
  count, and the key with it. An explicit ID on a container stops this: the
  count restarts inside it.

A widget of another kind does not move the count. An error `Text` shown above
two inputs leaves their keys as they were. An auto ID opens no scope, so an
explicit `ID` inside an ID-less scroll container resolves as if the container
were absent. `Form` counts across the whole window, because its registry keys on
the bare form ID.

`gui.Debug` reports a focused widget whose generated key moved to another widget
(`DebugAutoIDs`), and an app ID that starts with the reserved `~`. See
`docs/specs/auto-widget-identity.md`.

## `Opt[T]` vs plain fields

`Opt[T]` distinguishes "zero" from "not set". Use it for primitives where zero
is a real choice:

- Zero is a real choice (a scrollbar inset of 0): `Opt[float32]`.
- Zero is not meaningful (widths, heights, counts): plain field.
- The type knows whether it was set (`Color`, `Padding`, `Spacing`, `Radius`,
  `Border`): plain field.

`ScrollbarCfg.GapEdge`/`GapEnd` are the example: `0` is a real inset, so a plain
`float32` cannot tell that from "not specified". They are `Opt[float32]`
defaulting to the theme's `SizeScrollbarGap`/`SizeScrollbarGapEnd` (write
`gui.SomeF(4)`). Gaps, corner radii and border widths have theme roles, so they
use their own types: `Spacing: gui.SpacingMedium`, `Radius: gui.RadiusMedium`,
`SizeBorder: gui.BorderThin`, with `gui.NoBorder` / `gui.NoRadius` for an
explicit zero (#866, #867).

## Colors

Use plain `Color`, never `Opt[Color]`. Build values with `RGBA`, `RGB`, or
`Hex`. `Color{}` is unset, and `ColorTransparent` is explicitly transparent. A
`ColorSet` groups the per-state colors, and an assigned flat `Color` field wins
over it.

## Which hover API

Three APIs see the pointer. Pick by what the code needs.

| The code needs to…                                 | Use                                         |
| -------------------------------------------------- | ------------------------------------------- |
| pick a look (padding, colors, children) from hover | `gui.Interactive`                           |
| pick a look from a held press                      | `gui.Interactive`                           |
| read hover or press inside an own `GenerateLayout` | `w.IsHovered(effID)` / `w.IsPressed(effID)` |
| read the pointer position, set a cursor, hit-test  | `OnHover`                                   |
| react once when the pointer leaves a shape         | `OnMouseLeave`                              |

`gui.Interactive` reads the state for the leaf ID in its scope and gives it to a
builder. The root of the built view must carry the same ID:

```go
gui.Interactive("ok", func(s gui.InteractionState) gui.View {
    face := normal
    if s.Armed { // pressed and still over the button
        face = pressed
    }
    return gui.Row(gui.ContainerCfg{ID: "ok", Color: face})
})
```

A view that already has its own `GenerateLayout` can call `IsHovered` and
`IsPressed` directly, with `w.EffID(cfg.ID)`.

The state answers differently from `OnHover` on purpose. `OnHover` is dispatch:
it reaches the deepest shape with an `OnHover`, even under a float that has no
handler. `IsHovered` is visible state: anything drawn on top blocks it, and an
ID-bearing ancestor of the hovered shape is hovered too.

Change only what is inside the widget's bounds. A look that moves or resizes the
hovered shape can pull it out from under a still pointer, and it then flickers
every frame. `examples/custom_controls/buttons` shows the pattern;
`docs/specs/build-time-interaction-state.md` has the rules.

## `AmendLayout` and floats

`AmendLayout` runs after sizing with absolute coordinates. Moving a parent there
does not move its children. To move an element with its children, use the float
fields: `FloatAnchor`, `FloatTieOff`, `FloatOffsetX`, `FloatOffsetY`.

### Do not call window APIs from a hook

The hook runs inside the frame pass, which holds the window mutex. `SetFocus`,
`ClearFocus`, `SetView`, `ClearDrawCanvasCache` and `Window.Lock` all take that
mutex. It is not reentrant, so a call from a hook froze the app outright (issue
#394). It now panics, naming the API. Queue the work instead:

```go
AmendLayout: func(ctx gui.EventCtx) {
    ctx.Layout.Shape.X += 4              // fine: this is what the hook is for
    ctx.Window.QueueCommand(func(w *gui.Window) {
        w.SetFocus("next-field")         // runs next frame, no lock held
    })
},
```

The same applies to callbacks the library raises from the pass. A blur-triggered
`OnTextCommit` (`InputCommitBlur`), `OnBlur`, or an `OnTextChanged` fired by
`PostCommitNormalize` on blur now runs after the pass unlocks. It is free to
call `SetFocus`. Its `ctx.Layout` is **nil**, because the pass already recycled
that tree. Read `ctx.Window` and the arguments instead. The Enter commit path is
unaffected and still carries a live `ctx.Layout`.

## Group-box titles

`ContainerCfg.Title` draws a label in the top border, like an HTML fieldset. Set
`TitleBG` to the parent's background color, so the border line disappears behind
the label.

```go
// The label sits on a patch of the mismatched color.
Column{Title: "Account", TitleBG: RGB(255, 255, 255), ...}
```

## Layout transitions: snap a channel or a subtree

`AnimateLayout` eases X, Y, Width and Height on every ID-bearing shape.
`AnimSnap` removes a channel from that, and the zero value keeps today's
behavior, so it is an opt-out:

```go
// Slides to its new position, jumps to its new size.
Column(ContainerCfg{ID: "card", AnimSnap: gui.AnimSnapSize, ...})

// Holds a scroll viewport or grid body still while the chrome animates.
Column(ContainerCfg{ID: "grid", AnimSnap: gui.AnimSnapAll, ...})
```

The mask inherits down the tree, and only down: a snapped container snaps
everything inside it, and a child cannot escape the container's mask. The hero
transition (`Shape.Hero`) ignores `AnimSnap` — it is already an explicit
per-shape opt-in.

## Virtualized lists

`ListBox`, `Table`, `Tree` and `Combobox` virtualize automatically when the
scroll container has a bounded height. They always scroll — there is no
`Scrollable` opt-in — so a bounded height is the whole requirement: `Height` or
`MaxHeight`, or — `ListBox` only — a height Fill sizing resolved last frame.
Every row is the same height there, which is exact because the widget owns the
row shape. `Combobox` additionally clips its label so the arrow stays inside the
field, and its dropdown scrolls unconditionally.

For rows the app builds, of heights only the layout engine knows, use
`VirtualList`:

```go
gui.VirtualList(gui.VirtualListCfg{
    ID:        "feed",
    ItemCount: len(msgs),
    Sizing:    gui.FillFill,
    // Optional but wanted whenever items can be inserted or reordered:
    // measured heights follow the key, not the index.
    ItemKey: func(i int) string { return msgs[i].ID },
    ItemView: func(i int, width float32) gui.View {
        // width is the inner width recorded by the previous arrange.
        // It is 0 on the first frame.
        return card(msgs[i], gui.ScopeIDN("feed", "row", i), width)
    },
})
```

Set `ItemHeight` when the height is cheap to compute: heights are then exact
from the first frame and nothing is measured. Put spacing _inside_ the row. The
list's own spacing is fixed at zero, because a gap between rows is height the
model does not account for.

**Use `width` for decisions, never for a minimum.** A row that sets `MinWidth`
from it asks the list for at least the width the list just reported. The row's
own border pushes that further, the list widens, and the cycle repeats every
frame. It re-wraps and re-measures each time, so nothing settles. Rows fill the
width they are handed through `Sizing`. `gui.Debug(true)` reports the ratchet.

Scroll by index, not by ID or percentage. A row outside the viewport has no
shape for `FindByID` to resolve. The content height under virtualization is an
estimate, so a percentage drifts:

```go
w.ScrollToIndex("feed", 4000)          // row at the viewport top
w.ScrollToIndexAt("feed", 4000, 0.5)   // centred
w.ScrollIndexIntoView("feed", 4000)    // nearest edge; no-op when visible
w.ScrollToEnd("feed")                  // pin to bottom, exactly
```

These work on the uniform widgets too, in each one's own index space (a frozen
table header is data index 0 but sits outside the scrollable). Call
`w.InvalidateListHeights(id)` when a row's content changed under a stable key —
nothing detects that. See `docs/specs/virtualized-variable-height-lists.md`.

## Numeric input steps

`NumericInput` steps on Up/Down by default (`KeyboardDisabled` opts out) and on
the mouse wheel only when `MouseWheel` is set. Shift scales the step 10x, Alt
0.1x; `ShiftMultiplier`/`AltMultiplier` override each (zero takes the default).

## Date picker sizing

`DatePicker` is self-sized: `Height` is deliberately not pinned. A cell's height
comes from the theme font, and the month grid always emits six rows so the
height holds across months. The month/year roller floats over the grid on a card
smaller than the picker instead of swapping the body out, so opening it never
moves the outer box. `HideTodayIndicator` opts out of the today ring;
`ShowAdjacentMonths` opts into filling the edge cells.

## `Wrap` with Fit width

`Wrap` with a Fit width resolves as **fit-content** (issue #379). The width is
`min(single-row sum, nearest definite-width ancestor's available)`, so the
container wraps within its parent instead of rendering one unwrapped row wider
than it. A Fit chain with no Fixed/Fill width above it has no width to wrap
within and keeps the single-row sum. That combination behaves as a `Row`, not a
wrap. When the wrap must always fill its parent, use Fill width, which is what
every example in this repo does.

## Centering wrapped text

Two different controls, and they are not interchangeable:

- `ContainerCfg.HAlign` places the **box** across a column's cross axis.
- `TextStyle.Align` places the **lines** inside that box.

`TextModeWrap` defaults `Sizing` to `FillFit`, because wrapping needs a width to
wrap to. After the wrap pass the box shrinks back to its longest line when its
column aligns it, so `HAlign: HAlignCenter` centers a short wrapped label with
no extra config (issue #577). Text long enough to fill every line has no slack
to give back, so the box stays full width and only `TextStyle.Align` moves
anything:

```go
gui.Text(gui.TextCfg{
    Text:      "a long paragraph that fills every line it is given",
    Mode:      gui.TextModeWrap,
    TextStyle: gui.TextStyle{Align: gui.TextAlignCenter},
})
```

Naming `Sizing` yourself opts out of the shrink: an explicit `Sizing: FillFit`
is an instruction, so the box keeps the full width. Same for `TextStyle.Align`,
where the lines are already placed against the wrap width.

## Canvas gradients

A `DrawContext` fill takes a `*gui.CanvasGradient` in place of a flat `Color`:
`FilledRectGradient`, `FilledCircleGradient`, `FilledArcGradient`,
`FilledPolygonGradient`, `FilledRoundedRectGradient`, and
`FillTrianglesGradient` for geometry you tessellate yourself. Strokes stay flat.

**Geometry left at zero is derived from the shape being filled.** A radial
gradient with `R <= 0` centers on the fill's bounding box and matches its larger
extent. A linear one whose endpoints coincide runs top to bottom. So a glow is
its stops and nothing else:

```go
dc.FilledCircleGradient(cx, cy, r, &gui.CanvasGradient{
	Radial: true,
	Stops: []gui.GradientStop{
		{Color: core, Pos: 0},
		{Color: core.WithOpacity(0), Pos: 1},
	},
})
```

There is no stop-count limit — the shader's five-stop cap belongs to the shape
gradient path (`ContainerCfg`), not this one. Do not stack concentric discs to
fake a falloff. That is what this replaces.

A gradient fill always starts its own batch and never merges with the flat fill
before it, so interleaving the two keeps painter's order.

### When a gradient cannot express the shading

`FillTrianglesColors(tris, colors)` is the primitive underneath all six. You
supply the geometry and one color per vertex (`len(colors)*2 == len(tris)`).
Nothing is evaluated on the way through: no projection, no stop isolines, no
subdivision. A mismatched color count is a no-op, and the fill starts its own
batch under the same rule a gradient does.

When the shading is not a gradient and cannot be made into one, reach for it. A
gradient's level sets are conic curves, nested inside one another, stepped
linearly along the ramp. A shading whose isolines are not nested (a Lambert
sphere, whose isophotes open to the terminator and close again on both sides) is
out of reach. So is a shading that varies around a center rather than away from
it (a hue sweep), or one that stops at a silhouette. This holds however the
stops are arranged. `examples/solar_system` is the worked case.
`examples/draw_canvas` shows the two short ones.

**Wind every triangle the same way.** `gui/backend/soft` rasterizes a
vertex-colored batch as a single path and accumulates _signed_ coverage. A
triangle wound against its neighbors cancels along their shared edge and cuts a
hairline through the mesh. Share vertices between adjacent triangles rather than
overlapping them. With per-vertex color, an overlap paints twice and shows.

## Canvas transforms

A `DrawContext` carries a transform stack: `Translate(dx, dy)`,
`ScaleBy(sx, sy)`, `Rotate(rad)`, `Save()` and `Restore()`. The matrix is
batch-stamped, so a transform between two fills does not merge them. See
`docs/specs/draw-canvas-transform.md`.

## The one-event rule

Nothing is marked handled for you. A callback that acts on an event calls
`ctx.Consume()`. One that declines lets the event travel on. `ctx.Event` is nil
in `AmendLayout` and `OnScroll`, and both `EventCtx` methods are nil-safe. See
`docs/specs/eventctx-callback-refactor.md`.

## Widget sound

Silent unless the app opts in twice: `theme.Sounds = gui.SoundsDefault()` names
a cue per role, and `w.SetSoundPlayer(...)` renders them. Either alone is
silent, and a nil player is the default, so tests need no setup. Per instance,
`Cfg.Sound` overrides the theme cue and `Cfg.SoundDisabled` suppresses it;
`w.SetSoundVolume(0..1)` is the gain, where `0` is mute. The cue fires before
`OnClick` and regardless of `ctx.Consume()`. Roles are `Click`, `ToggleOn`,
`ToggleOff`, `Selection`, `Error`, `Notify`, `Open` and `Success`; a widget
picks the role, the app picks the sound. Four interactions sound without a
click: a toast appearing (`Notify`, or `Error` at `ToastError` severity), a
dialog opening (`Open`), a form submit accepted (`Success`) or blocked
(`Error`), and a `datagrid` CRUD save failure (`Error`).
`gui.NewSystemSoundPlayer(w)` renders every cue with the platform's own event
sounds, no assets and no audio library; it ignores gain. Feeding a resolved cue
into a nested `ButtonCfg` or `ToggleCfg` means passing `SoundDisabled` too —
those resolve their own precedence, and a resolved `SoundNone` reads there as
"unset". See `docs/widget-sound.md`.

## Markdown render callback

`MarkdownCfg.RenderBlock(w, el)` overrides rendering per block: return a `View`
and `true` to replace the block (or nil to drop it), `false` to keep the
default. The hook runs every frame over cached blocks, so it must be cheap and
pure — build `View` structs, do not fetch or parse. It runs during
`GenerateLayout` under the frame lock: no window-mutating calls, `QueueCommand`
for those. Hook writers own their IDs: compose with `ScopeID(el.DocID, …)`. See
`docs/specs/markdown-render-callback.md`.

## Streaming background data into a window

A background producer — stdin, a socket, a ticker — must schedule a frame per
value. A channel the view drains on its own never paints: the backend idles
until the next input event, so the window sits stale until a mouse move and then
shows everything at once (issue #559). `Stream` wraps the `QueueCommand` +
`InvalidateLayout` path for this shape:

```go
type streamLines struct {
	Lines []string
}

func streamLinesView(w *gui.Window) gui.View {
	state := gui.State[streamLines](w)
	text := ""
	if len(state.Lines) > 0 {
		text = state.Lines[len(state.Lines)-1]
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Content: []gui.View{gui.Label(text, gui.TextStyle{})},
	})
}

func ExampleStream() {
	w := gui.NewWindow(gui.WindowCfg{State: &streamLines{}})
	defer w.WindowCleanup()
	w.SetView(streamLinesView)

	lines := make(chan string, 4)
	done := gui.Stream(w, lines, func(w *gui.Window, line string) {
		st := gui.State[streamLines](w)
		st.Lines = append(st.Lines, line)
	})
	lines <- "hello"
	lines <- "world"
	close(lines)
	<-done
	w.FrameFn()

	fmt.Println(gui.State[streamLines](w).Lines)
	// Output: [hello world]
}
```

Delivery is per item in channel order with no coalescing, so bound the cadence
at the producer. The goroutine exits when the channel closes or the window
closes, and the returned channel reports it.

## App settings

`gui.LoadSettings` and `gui.SaveSettings` store one typed struct per app,
JSON-encoded (issue #848). The key is `WindowCfg.AppInfo.ID`, usually from an
embedded `appinfo.toml`; with no ID both return `gui.ErrNoAppID`. Fill the
defaults first, then load: a field the saved data does not have keeps its
default, so adding a field needs no migration code.

```go
// Settings is what the app saves. Adding a field later is the whole
// migration: a file saved by an older version leaves the new field at
// the default set before LoadSettings.
type Settings struct {
	Launches int  `json:"launches"`
	Dark     bool `json:"dark"`
}

// App is the window state. It holds the settings the view shows.
type App struct {
	Settings Settings
	Err      string // last load or save error, shown in the view
}

func windowCfg() gui.WindowCfg {
	return gui.WindowCfg{
		AppInfo: info,                                 // the store keys the saved data by info.ID
		State:   &App{Settings: Settings{Dark: true}}, // defaults
		Width:   360,
		Height:  180,
		OnInit: func(w *gui.Window) {
			app := gui.State[App](w)
			// A first run finds nothing and keeps the defaults. A
			// corrupt file returns an error and also keeps them.
			if err := gui.LoadSettings(w, &app.Settings); err != nil {
				app.Err = err.Error()
			}
			app.Settings.Launches++
			save(w)
			applyTheme(w)
			w.SetView(mainView)
		},
	}
}

// save writes the whole struct. Call it after each change: settings are
// small, and the file is replaced atomically.
func save(w *gui.Window) {
	app := gui.State[App](w)
	if err := gui.SaveSettings(w, app.Settings); err != nil {
		app.Err = err.Error()
	}
}
```

- **Where it goes.** Desktop and iOS:
  `os.UserConfigDir()/<app ID>/settings.json`, replaced atomically. Web:
  `localStorage`. Android: the files directory the host passes to
  `android.SetFilesDir`.
- **Tests.** `gui.NewTestWindow` gives the window an in-memory store, so a test
  never touches the real file. A plain `NewWindow` with no backend uses the file
  store: a real app that saves before `backend.Run` still reaches disk.
- **Not user config.** The file is state the app writes for itself. A running
  app overwrites a hand edit. A file the user edits stays a file the app owns.
- **Not secrets.** The data is plain JSON. Use the secret store below.

## Secrets

`gui.SaveSecret(w, key, value)`, `gui.LoadSecret(w, key)` and
`gui.DeleteSecret(w, key)` keep an access token or a password in the OS
credential store (issue #920). The service name is `WindowCfg.AppInfo.ID`; with
no ID all three return `gui.ErrNoAppID`.

- **Where it goes.** macOS and iOS: the Keychain. Windows: Credential Manager.
  Linux: the Secret Service (GNOME Keyring, KWallet).
- **No store, no fallback.** Web, Android and a Linux session with no Secret
  Service daemon return `gui.ErrSecretsUnsupported`. go-gui never writes the
  secret to a plain file. The app decides what to do instead.
- **Errors.** A missing key returns `gui.ErrSecretNotFound`. `DeleteSecret` of a
  missing key returns nil.
- **Limits.** A key holds letters, digits, `.`, `-` and `_`. A value is 1 to
  2560 bytes on every platform (the Windows limit).
- **Blocking.** The OS can ask the user to unlock the store, and the call waits
  for the answer. Call from a goroutine and post the result with
  `w.QueueCommand`, not from a view function or an event callback.
- **Tests.** `gui.NewTestWindow` gives the window an in-memory store.

## Golden appearance tests

`(*Window).TestGolden` pins what a window looks like as text. It runs one frame
under `ThemeDark` and one under `ThemeLight`, serializes the emitted render
commands with floats rounded to two decimals, and diffs each against
`testdata/<name>.dark.golden` and `testdata/<name>.light.golden`. Interact
first, then assert:

```go
func TestGoldenExample(t *testing.T) {
	w := gui.NewTestWindow(t, gui.WindowCfg{Width: 320, Height: 240})
	w.TestRender(func(win *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing: gui.FillFill,
			Content: []gui.View{
				gui.Button(gui.ButtonCfg{ID: "save", Label: "Save"}),
			},
		})
	})
	w.TestGolden(t, gui.GoldenCfg{Name: "save_button"})
}
```

- A styling change shows up as a diff; a backend rasterizer bug does not. These
  pin commands, not pixels.
- Re-record with `GOGUI_UPDATE_GOLDEN=1`, after reading the diff. The trigger is
  an env var because a flag in a library package collides with the app's own
  flags.
- Files carry a `# go-gui-golden v1` header. A format change bumps the version
  and fails with a re-record hint, not a wall of diffs.
- The clock is pinned, so a view reading `w.Now()` records the same output every
  day. The window's theme and pointer state are restored afterwards.
- A mismatch writes the recording under `testdata/failures/` for review.
  Git-ignore that directory.
- `GoldenCfg` also carries `FocusID`, `KeyPressID`, `HoverX`/`HoverY` and
  `MousePressed` for appearances `TestClick` cannot reach: a held Space, a hover
  with no click.

## Find it early

`gui.Debug(true)`, or `GOGUI_DEBUG=1`, checks the layout every frame. It reports
duplicate IDs, focusable shapes without IDs, handlers that act without
consuming, and the rest of the sweep: scrollable or `OnMouseLeave` shapes
without IDs, a height-0 virtualized listbox, an over-stop gradient, a
`Wrap`+`Overflow` container, unresolved state keys, unclaimed focus IDs, stamp
drift, dropped callbacks and links, and refused window features. Findings print
once per window. In tests, use `(*Window).TestDuplicateIDs` and
`(*Window).TestUnconsumedEvents`.

`DebugUnscopedIDs` is separate and opt-in: it is not part of `DebugAll`. It
reports an ID with no ID-bearing ancestor — the widget cannot move into a second
panel as it stands. When you plan to reuse a screen, ask for it:

```go
gui.DebugCategories(gui.DebugAll | gui.DebugUnscopedIDs)
```

`DebugUnresolvedKeys` is part of `DebugAll`, so `gui.Debug(true)` already runs
it. It reports a widget whose state key and whose identity are different strings
— either because the widget never resolved its `cfg.ID`, or because it resolved
at the wrong time and got the bare leaf back. Such a widget works while it is
the only instance of its `cfg.ID`, and two of them under different scopes share
one state slot.

The remedy is a resolved key: `w.EffID(cfg.ID)` during `GenerateLayout`, or
`ctx.EffID(leaf)` in a handler.

The two seams are not interchangeable. `w.EffID(cfg.ID)` takes a leaf in the
**current** scope and joins it. `ctx.EffID(leaf)` takes a leaf captured at build
time that names a shape **at or above** the handler, and searches the ancestor
chain for it — which is why a handler attached to an ID-less shape can still
resolve its owner's key. Pass the leaf the owner was written with, not the
handler's own (issue #526).

**Resolve during `GenerateLayout`, never in a factory body.** A factory runs
while the parent's `Content` slice is being built, which is before the framework
descends into the container the widget will sit in. `w.EffID` there joins the
enclosing scope, not the widget's own. The answer it returns for a widget with
no scope above it is the same string a correct resolve returns, so the mistake
is invisible until someone puts the widget in a panel. A factory that needs to
resolve returns a view struct, or defers its body:

```go
func (w *Window) Thing(cfg ThingCfg) View {
	return ViewFunc(func(vw *Window) View { return thingView(cfg, vw) })
}
```

`DebugUnknownFocus` is also part of `DebugAll`. It reports the same mistake from
the other side: a frame that finished with a focus ID nothing focusable claims,
which is what `SetFocus("nav")` leaves behind when the frame stamped
`"detail:nav"`. The finding names the spelling that would have worked. It fires
from the frame audit, not from `SetFocus`, because a view function may
legitimately focus a control it is still returning.

`DebugStampDrift` is also part of `DebugAll`. It reports a shape whose stamp
disagrees with the scope it was arranged under, or an ID-bearing shape with no
stamp at all — the signature of a hand-built `Layout` spliced into a generated
tree. Every store then keys it on its bare leaf, which works until a second
widget of that leaf appears elsewhere in the window.

`DebugCategories` prints to stderr. To assert a category in a test, including an
opt-in one, use `(*Window).TestFindings(mask)`, which returns the findings as
data:

```go
if found := w.TestFindings(gui.DebugAll | gui.DebugUnscopedIDs); len(found) > 0 {
	t.Fatalf("ID defects: %v", found)
}
```
