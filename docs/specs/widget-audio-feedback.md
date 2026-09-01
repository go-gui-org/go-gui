# Spec: opt-in widget audio feedback

Status: **phases 1 to 4 implemented** — phase 1 landed `SoundCue`,
`SoundPlayer`, `Theme.Sounds`, window volume, and the `Button` / `Toggle` proof;
phase 2 (#467) carried the seam to every remaining mechanical widget and to
`gui/datagrid`; phase 3 (#468) reached the paths dispatch never sees; phase 4
(#469) added the non-click semantics and a player backed by the platform's own
event sounds.

Issue: #446 — "Widgets have no audio feedback for interactions". Phase 2: #467 —
"Widget audio feedback phase 2: the mechanical widget pass". Phase 3: #468 —
"Widget audio feedback phase 3: paths dispatch does not see". Phase 4: #469 —
"Widget audio feedback phase 4: non-click cues and a native system-sound
player".

## Phase 4 decisions

Phase 4 covers the interactions with no click site at all, and the second
built-in player.

- **Three new cues, not seven.** `SoundNotify` (a toast appeared), `SoundOpen`
  (a dialog opened) and `SoundSuccess` (a submit was accepted), appended after
  `SoundSelection`, with matching `SoundSet` fields. Every refusal reuses
  `SoundError`: an error-severity toast, a blocked submit and a failed CRUD save
  are all rejections, and a separate cue per site would say nothing extra. No
  code in `gui/` switches exhaustively over the enum, so the additions break no
  player.
- **`(*Window).PlaySoundCue` is exported.** `gui/datagrid` is outside `gui/` and
  cannot reach `playSoundCue`; a CRUD failure has no button to hang a cue off,
  so the emit seam had to be public. It applies no precedence — resolve with
  `ResolveSoundCue` first — and keeps every guarantee of the unexported helper.
- **The cue rides the imperative entry point, never the per-frame view.**
  `(*Window).Toast` and `(*Window).Dialog` are called once per appearance;
  `toastItemView` and `dialogViewGenerator` run every frame. The toast cue is
  emitted at the append rather than in the enter tween's `OnDone`, which a toast
  dismissed early never reaches. Both read `w.Theme()` rather than the
  `guiTheme` mirror: they run outside generation, and `themeRef` takes
  `w.themeMu`, not the frame lock.
- **Toast severity picks the cue; `ToastCfg.Sound` does not.** This is the one
  place a severity chooses a cue. `Sound` names the toast's buttons — an
  activation — so it does not reach the appear cue, while `SoundDisabled`
  suppresses both. The same split governs `DialogCfg` and `FormCfg`.
- **Form submit resolves at generation time and emits under the frame lock.**
  `formProcessRequests` runs from `AmendLayout`, so the `soundCues` pair is
  resolved in `GenerateLayout` and captured by value; the `submitReq` latch
  makes the emit one-shot even though the pass runs every frame. Emitting inline
  is safe because `playSoundCue` takes no lock, but it does mean app player code
  runs under `w.mu` — the `SoundPlayer` contract now says a player must not call
  a window-mutating API.
- **A blocked submit sounds where nothing else reports it.** The
  `blockedInvalid || blockedPending` branch has no callback: the cue is the only
  feedback the app gets for free.
- **One emit site for the grid.** Every DataGrid CRUD failure funnels through
  `dataGridCrudRestoreOnError`, so the cue lives there and `dataGridSounds`
  gains an `err` role resolved beside `click` and `selection`. Keying off
  `state.SaveError` would have fired once per frame.
- **The native player is a second player, not a replacement.**
  `NewSystemSoundPlayer` maps every cue onto the platform's own event sounds;
  `NewBeepSoundPlayer` stays the right choice for an app that only wants to
  signal rejection.
- **It ignores gain, deliberately.** macOS `NSSound` exposes `setVolume`, but
  Windows `PlaySound` and `canberra-gtk-play` do not, so honouring gain would
  make one platform behave differently from the other two. Mute still works: the
  `gain <= 0` gate sits ahead of the player.
- **The native capability is an optional interface, not a `NativePlatform`
  method.** `systemSoundPlatform` (`gui/sound_system.go`) is type-asserted on
  `w.nativePlatform`. Adding the methods to the exported `NativePlatform`
  interface would break every out-of-repo implementation; a platform that does
  not implement the capability is silent, which is also what keeps the noop
  platform and headless tests quiet.
- **`sysbeep` keeps `Play` and gains `PlayEvent`.** `Play` is still the
  out-of-band alert. The event API is a separate, open enum with its own
  per-platform mapping table, so `sysbeep` needs no knowledge of widgets and
  `gui` needs none of system sounds; `nativehost` holds the one map between
  them. An unmapped or out-of-range event is silent.

### Phase 4 sound mapping

| Event     | macOS `NSSound(named:)` | Windows `SND_ALIAS`    | Linux freedesktop id |
| --------- | ----------------------- | ---------------------- | -------------------- |
| Click     | `Tink`                  | `MenuCommand`          | `button-pressed`     |
| ToggleOn  | `Pop`                   | `MenuCommand`          | `button-pressed`     |
| ToggleOff | `Bottle`                | `MenuCommand`          | `button-released`    |
| Selection | `Ping`                  | `MenuPopup`            | `button-pressed`     |
| Error     | `Basso`                 | `SystemHand`           | `dialog-error`       |
| Notify    | `Purr`                  | `Notification.Default` | `message`            |
| Open      | `Blow`                  | `SystemAsterisk`       | `dialog-information` |
| Success   | `Glass`                 | `.Default`             | `complete`           |

macOS caches each `NSSound` by name and restarts it on retrigger, so a second
cue is not swallowed by the first. Windows passes `SND_NODEFAULT` so an alias
the user's scheme leaves unset is silent rather than a default beep. Linux
spawns one `canberra-gtk-play` per cue — acceptable for a dialog or a toast,
wrong for a click-heavy UI, and the guides say so.

### Phase 4 silences

- **`DialogDismiss` and `ToastDismiss`.** The button that closed the surface has
  already sounded through the ordinary click path, so a dismiss cue would double
  up. This retires the dismiss-cue question phase 2 deferred.
- **Native dialogs** (`gui/native_dialog.go`) — the OS plays its own.
- **A save that fails with no data source attached.** It reports through
  `state.SaveError` and never calls `OnCRUDError`, so it never reaches the emit
  site; that path is a wiring mistake, not a user-facing rejection.

## Phase 3 decisions

Phase 3 covers the interactions that call a callback directly, usually with a
nil `Layout`, so `playShapeSound` has no shape to read a cue off.

- **A second helper, `playSoundCue(SoundCue, *Window)`.** Factored out of
  `playShapeSound`, which now resolves the cue off the shape and delegates.
  Every guard past the shape — nil window, `SoundNone`, nil player, zero or
  non-finite gain — lives in the new helper, so a new call site inherits the
  silence guarantees rather than restating them. `(*Window).SoundVolume` does
  not go through `lockForAPI`, so a cue may be emitted inline even under the
  frame lock; no phase-3 site needs `deferCallback`.
- **A resolved pair, `soundCues{act, reject}`.** Most phase-3 paths need two
  cues: the one for a change that lands and the one for a change refused.
  `resolveSoundCues` builds the pair at generation time and it travels by value,
  so a handler closure captures a `SoundCue` rather than a `*Layout` the arena
  pools. Only `act` takes the `Cfg.Sound` override — `Cfg.Sound` names a
  widget's activation sound, not its refusal, so `reject` stays on the theme's
  `Error` role. `SoundDisabled` suppresses both.
- **Refusal sounds; movement does not.** An arrow key that moves a selection is
  silent, because a held arrow key would machine-gun the cue. An arrow key that
  cannot move — already at `Min`, at row 0, at the last row, at the end of a
  menu — emits `Theme.Sounds.Error`. The asymmetry is the point: silence is the
  normal case, so the cue carries information.
- **`NumericInput` reports its own clamp.** `numericInputStepResultClamped`
  returns whether the step was refused, rather than the call site re-deriving it
  by recomputing the seed — a duplicate that would drift the moment the stepping
  rule changed. `numericInputStepResultMode` stays as the two-value wrapper its
  existing callers use.
- **A drag never sounds; a drop sometimes does.** Continuous drag — slider
  thumb, splitter handle, colour plane and wheel — stays silent throughout: it
  has no activation moment, and a cue per move would be a buzz. Only a
  drag-reorder drop that actually moves an item sounds, on `Selection`. A cancel
  (Escape), a drop that lands where the item already was, and a dock panel
  released outside every zone are all silent, and the guards that distinguish
  them were already in the code.
- **The splitter's collapse toggle is the one drag-adjacent activation.**
  `splitterToggleCollapse` already reports direction, so collapse takes
  `ToggleOn` and expand takes `ToggleOff`. The collapse buttons carry the same
  state-dependent cue through the ordinary click path.
- **The Input's Enter commit sounds; its blur commit does not.** Enter is a
  deliberate activation and takes `Click`. Blur is incidental — focus moved for
  some other reason — and its commit runs under the frame lock through
  `deferCallback`, so keeping it silent avoids that seam entirely.
- **`SoundError` gets its first real consumer.** Every Input rejection sounds
  it: a character a mask has no slot for, a paste the mask cannot fit, a
  backspace over a mask literal, and a `PreTextChange` veto. That is what
  `NewBeepSoundPlayer` was built for — it plays `SoundError` and nothing else.
  An unmasked field that cannot delete is at the start or end of its text; that
  is an edge, not a rejection, and stays silent.
- **Five widgets gained sound config.** `TableCfg`, `InputCfg` and `SplitterCfg`
  take the usual `Sound` / `SoundDisabled` pair. `SliderCfg` and
  `NumericInputCfg` take `SoundDisabled` alone: neither has an activation moment
  to sound at, so a `Sound` field would name a cue that never plays.

## Phase 2 decisions

- **A fifth role, `SoundSelection`.** The phase-2 inventory assigns "selection"
  to about ten widgets — a radio, a list row, a tab, a calendar day — and
  `SoundSet` had only click, toggle and error. Overloading `Click` would have
  made a tab indistinguishable from a button, so the constant and the
  `SoundSet.Selection` field were added instead. The cue enum is open and
  append-only, so `SoundSelection` sits after `SoundError`.
- **`ResolveSoundCue` is exported.** `gui/datagrid` is outside `gui/` and cannot
  reach the unexported `resolveSoundCue` or the package-global `guiTheme`. It
  reads `gg.CurrentTheme().Sounds` once per generate and resolves through the
  exported seam, rather than re-deriving the precedence.
- **A resolved cue fed into a nested `ButtonCfg` carries `SoundDisabled` too.**
  `ButtonCfg` and `ToggleCfg` resolve their own precedence, so a resolved
  `SoundNone` reads there as "unset" and falls back to the theme's click cue.
  Every site that hands a resolved cue to one of them passes
  `SoundDisabled: cue == SoundNone` alongside it. `ContainerCfg` is a pure
  carrier and needs no such pairing.
- **All five `Dialog` buttons take `Click`.** There is no cancel or dismiss
  role, and `Error` would misread a cancellation as a failure. A dismiss cue can
  land in phase 4 with the other non-click semantics.
- **`gui.Text` stays out of scope.** `textEventHandlers` is a package-level
  handler record shared by every text shape, so a per-instance cue would cost
  the zero-allocation sharing.
- **Drag starts stay silent.** A `Tree` row, a `ListBox` reorder row and a
  `DockLayout` tab all start a drag from the same handler that activates them.
  The cue marks the activation; sounding a drag is phase 3's question.

## Motivation

No widget in `gui/` emitted sound. Feedback was visual only. Two audio
mechanisms existed and neither reached a widget:

- `Window.Beep` (`gui/native_sound.go`) — the system alert, with no non-test
  caller anywhere in the repo.
- `gui/audio` — sampled and synthesized audio, a leaf subpackage. `gui` must not
  import it: that would pull `beep`, `oto` and `pulse` into every consumer and
  invert the subpackage rule.

An app that wanted a click sound had to wrap `OnClick` on every widget and
handle `BeepAvailable` plus `audio.Init` errors itself.

## Design

### A cue is semantic, not a sample

`SoundCue` names the interaction; the injected player decides what it sounds
like. That indirection is what keeps `gui` free of an audio dependency, and it
lets one app map a cue to a WAV while another synthesizes it.

`SoundNone` is the zero value, so an unset field is silent and needs no `set`
flag — the same reasoning as `ColorSet` (`gui/color_set.go`).

The enum is open. New cues will be added, so no code in `gui/` switches
exhaustively over it, and the `SoundPlayer` contract requires a player to ignore
a cue it does not recognize.

### The seam is injected, like the other three

`SoundPlayer` is stored per window in `windowBackend`, beside `TextMeasurer`,
`SvgParser` and `NativePlatform`, and every call site nil-guards. One
difference, stated in the field comment: the backend installs the other three,
but installs **no** sound player. The app opts in with `SetSoundPlayer`, so
silence is the default and headless tests need no setup.

`NewBeepSoundPlayer` is the zero-dependency fallback: it plays the system alert
for `SoundError` and nothing else, because an alert is the wrong sound for a
click. It ignores gain — `NSBeep` and `MessageBeep` honor the system _alert_
volume and expose no app-level gain.

### Volume is window state, not theme state

`(*Window).SetSoundVolume` / `SoundVolume`, clamped to `0..1`, default `1`.
There is no separate mute flag: `0` is mute, and the gate is one comparison
ahead of the player call, so a muted window costs nothing per click.

Volume was deliberately **not** put on `Theme`. A theme is per-window visual
identity keyed by `Theme.id`, and `Themed(t, build)` scopes a theme to a
_subtree_; a loudness that changed when one panel was restyled would be
incoherent. Volume is a user preference.

It is also not expressed as a percentage of the system level, because it already
is one: the OS mixer applies the user's output volume downstream of the app, so
an app-level gain is inherently relative. An app cannot portably read the system
output level and must not try.

### Theme carries the role → cue map

`Theme.Sounds` is a `SoundSet` — one cue per interaction role. The zero set is
silent and is the intended default, so `ThemeDark` and `ThemeLight` make no
sound. `ThemeMaker` copies it from `ThemeCfg` and adds no default. It is not a
`default*Style` mirror, so `TestDefaultStylesMirrorThemeDark` needs no row and
`applyTheme` is untouched.

An app opts the whole widget set in by rebuilding through `ThemeMaker` with
`cfg.Sounds = gui.SoundsDefault()`. Mutating an installed theme value in place
is wrong here: `needsInstall` compares `Theme.id`, so a mutated copy carrying
the old id can be skipped in a second window. `ThemeMaker` stamps a fresh id.

A struct rather than a fixed array, so a role can be added without breaking an
app that builds one by field name.

### The cue rides the shape, not a closure

The real chokepoint is `eventHandlers`, not `ContainerCfg`: the unexported
`soundCue` field lives there, exactly like `clickOnSpace` and `clickOnEnter`,
which exist to avoid a per-frame closure allocation. Putting it there also
reaches the raw-`Shape` widgets (Image, Svg, Text, RTF, DrawCanvas, termgrid)
that build `eventHandlers` directly and never touch `ContainerCfg`.

Measured across `gui/` and `gui/datagrid/`: 86 click sites — 43 direct
`ContainerCfg`, 28 `ButtonCfg` (which funnels into Button's own container), 9
raw `eventHandlers`, 6 wrapper Cfgs. 71 of 86 inherit the plumbing from the
`ContainerCfg.Sound` field alone.

### Precedence, resolved at generation time

```
SoundDisabled  >  Cfg.Sound  >  Theme.Sounds.<role>
```

Resolution happens during `GenerateLayout`, where the widget already knows its
own state. That is why a state-dependent toggle needs **no** `SoundOn` /
`SoundOff` field pair: `Toggle` reads `cfg.Selected` and stamps `SoundToggleOff`
when the click will turn it off. No runtime branch, no allocation.

### Dispatch

One helper, `playShapeSound(*Layout, *Window)`, nil-safe on every seam. Four
call sites cover all click dispatch:

| Path     | Site                                        |
| -------- | ------------------------------------------- |
| Mouse    | `callRelative`, gated on `class == evClick` |
| Spacebar | `charHandler`, the `clickOnSpace` branch    |
| Enter    | `keydownHandler`, the `clickOnEnter` branch |
| A11y     | `a11yActionCallback`, `A11yActionPress`     |

The cue fires **before** `OnClick` and **independently of `ctx.Consume()`**:
sound confirms that the widget was activated, it does not participate in event
propagation. All four paths are asserted in `gui/sound_test.go`.

It is not routed through `deferCallback`. Dispatch runs from `EventFn` with no
frame lock held, and a deferred callback may not capture a `*Layout` — the arena
pools it.

The paths dispatch never sees raise their own cue with
`playSoundCue(SoundCue, *Window)`, at the call site, keeping the same ordering:
before the callback, independent of consumption.

| Path                     | Site                                              |
| ------------------------ | ------------------------------------------------- |
| Table keyboard activate  | `tableOnKeyDown`, the `listCoreSelectItem` branch |
| ListBox keyboard select  | `listBoxOnKeyDown`, same branch                   |
| Tree keyboard select     | `treeOnKeyDown`, `KeyEnter`/`KeySpace`            |
| Menu keyboard activate   | `menuOnKeyDown`, `KeySpace`/`KeyEnter`            |
| Input Enter commit       | `inputCommitEnter`                                |
| Input reject             | `inputTextChange`, `inputKeyPaste`, delete        |
| NumericInput clamp       | `numericInputApplyStep`                           |
| Slider clamp             | `sliderOnKeyDown`                                 |
| Drag-reorder drop        | `dragReorderOnMouseUp`, `dragReorderKeyboardMove` |
| Dock panel drop          | `dockDragOnMouseUp`                               |
| Splitter collapse toggle | `splitterOnHandleKeyDown`                         |

## What stays silent

Pure click absorbers must never sound, and opt-in-by-default protects them with
no special case: the Toast scrim, the ContextMenu dismiss `OnAnyClick`,
scrollbar track/thumb/arrows, and Input's caret-placement click all consume
without user semantics. **Never add a blanket "any `ContainerCfg.OnClick`
sounds" rule.**

One known exclusion: `gui/view_text.go` uses a package-level shared
`textEventHandlers` singleton, which cannot carry a per-instance cue without
becoming per-view. Link-click sound is out of scope.

Phase 3 adds these deliberate silences. Each was decided, not omitted:

- **Arrow-key movement** in ListBox, Menu, Tree, Table and Slider. Only a move
  that is refused sounds.
- **Tree expand and collapse** on Left and Right. They are arrow keys, and the
  rule above governs them.
- **Continuous drag** — `sliderMouseMove`, `splitterEmitChange` from a drag
  move, and `colorDragTrack` in `gui/view_color_drag.go`.
- **The Input blur commit**, in `inputAmendLayout`.
- **A drag cancel and a no-op drop**, including a dock panel released outside
  every drop zone.
- **An unmasked Input delete at the start or end of its text.**

Two candidates were deferred rather than decided against. A slider **snap or
detent** cue needs drag-path snapping that does not exist — `SliderCfg.Step` is
consulted only by the keyboard path. A **colour drag-end** cue needs a commit
moment the colour widgets do not have: they write the value on every move, so
release commits nothing.

## Rollout

- **Phase 1 (this change)** — the seam, `Button`, `Toggle`, headless tests, both
  guides, the showcase page and player.
- **Phase 2** (#467) — the mechanical pass: every widget whose activation
  already lands on a `ContainerCfg` or `ButtonCfg` (Switch, Radio, ExpandPanel,
  Select, Combobox, ListBox, Tree, TabControl, Menu, Dialog, Toast, DatePicker,
  CommandPalette, DockLayout, Image, Svg, and `gui/datagrid`). Split by file
  count, not one 40-file diff.
- **Phase 3** (#468) — the paths dispatch never sees: Table, ListBox, Tree and
  Menu keyboard activation, Input commit and reject, NumericInput and Slider
  clamp, the splitter's collapse toggle, and the drag-reorder and dock drops.
  Keyboard movement and continuous drag were decided silent; the decisions are
  in "What stays silent" above.
- **Phase 4** (#469) — non-click semantics (Toast appear, Dialog open, Form
  submit, DataGrid `OnCRUDError`) and `NewSystemSoundPlayer`, backed by
  `sysbeep.PlayEvent`: `NSSound(named:)` on macOS, `PlaySound` with `SND_ALIAS`
  on Windows, freedesktop sound-naming ids on Linux. Decisions above.

## Showcase sounds are synthesized

The repo ships no sound assets and does not gain any. The showcase's player
reuses the ADSR voice already in `demo_audio.go` with a percussive, one-shot
envelope — `voiceEnv` was extracted from the pad's package constants so the pad
and the cue share `Fill` and `envelope` rather than forking the oscillator.

Click is A5; toggle-on and toggle-off are a fourth above and below it; error is
a low tone with a longer decay. The `audio.LoadSoundBytes` path for an app with
its own WAVs is documented in the guide but not shipped.

## Rejected

- **`gui` imports `gui/audio`** — an import cycle in spirit and a dependency in
  fact for every consumer.
- **Volume on `Theme`** — see above.
- **A separate mute flag** — `0` already means mute.
- **`SoundOn` / `SoundOff` field pairs on toggles** — unnecessary once the cue
  resolves at generation time.
- **Shipping WAV assets** — binaries, an attribution obligation, and a guide
  snippet nobody could copy without the files.

## Documentation

- `docs/widget-sound.md` and `examples/showcase/docs/widget_sound.md` — the
  app-author guide, mirrored. Every later phase must update the cue table in
  both.
- `examples/showcase/docs/widget_audio.md` — cross-linked, and corrected: it
  documented `audio.Quit`, `audio.LoadSound`, `Sound.SetVolume`,
  `SetMusicVolume`, `PauseMusic` and several other functions that are unexported
  in the package as it stands.
- The guide's player is quoted whole from `examples/showcase/sound_player.go`
  between `// doc:snippet-begin player` markers, not paraphrased, so a reader
  can paste it. `TestSoundGuideSnippetMatchesSource`
  (`examples/showcase/sound_snippet_test.go`) byte-compares the marked region
  against the Go fence under the "A real player" heading in both guides and
  names the first differing line. The block therefore compiles by construction —
  it is part of the showcase build — and cannot rot silently in either
  direction. Two names inside it (`ensureAudioInit`, `newVoice`) are showcase
  plumbing, explained in prose beneath the fence rather than inlined, which
  would have broken the byte comparison.
