# Keys and shortcuts on every keyboard layout

Issue: #1188. Status: **planned**. Linux (X11 and Wayland) first; Windows, macOS
and web follow as separate issues.

## Problem

A key press reaches the app as two events. `EventChar` carries the text the key
types. `EventKeyDown` carries a `KeyCode` and the `Modifiers`, and shortcuts,
navigation and editing keys read only that. On Linux the two are built from
different data:

- `EventChar` (Wayland) comes from the key's symbol under the full state —
  Shift, AltGr, NumLock, active layout — so text is right on every layout.
- `KeyCode` comes from the key's first-level symbol in the active layout, sent
  through `x11key.MapKeySym`, which knows US ASCII only. Anything else is
  `KeyInvalid`.

`Shortcut.matches` then compares `KeyCode` and `Modifiers` exactly. The result,
measured on the reporter's KDE machine (`de nodeadkeys`) and reproduced in an
Xorg container on `de`, `fr`, `ru` and `us dvorak`:

| Symptom                                                    | Cause                                                        |
| ---------------------------------------------------------- | ------------------------------------------------------------ |
| `Shortcut{KeySlash}` never fires on German                 | `/` is Shift+7: the press is `Key7` + Shift                  |
| `#`, `+`, `<`, `^`, ä, ö, ü, ß give `KeyInvalid`           | first-level symbol is not in the US table                    |
| AltGr+8 (`[`) fires a `{Key8}` shortcut                    | AltGr is not a `Modifier`, so the press is plain `Key8`      |
| numpad keys give `KeyInvalid`; NumLock-off arrows dead     | keypad symbols are not in the table                          |
| `KeyNumLock`, `KeyMenu`, `KeyRightAlt`, F13–F25 never sent | not in the table (right Alt on German is `ISO_Level3_Shift`) |
| Ctrl+1 dead on French                                      | the AZERTY number row types `&é"'(` without Shift            |
| Ctrl+C/V/X/Z/A dead on Russian, in inputs too              | first-level symbol is Cyrillic                               |

The X11 path has three more defects, in the text it types:

| Symptom (X11 only)                         | Cause                                                        |
| ------------------------------------------ | ------------------------------------------------------------ |
| AltGr+Q types `q`, not `@`                 | the keysym column is chosen from Shift alone                 |
| numpad digits type nothing                 | the X11 NumLock rule is not applied                          |
| Cyrillic, Greek, Hebrew text types nothing | `x11key.KeysymToRune` knows Latin-1 and Unicode keysyms only |

## Goals

1. Every shortcut whose key a layout can type fires on that layout.
2. A press never fires a shortcut for a key it did not produce (AltGr+8 is not
   `8`).
3. Ctrl+letter and Ctrl+digit shortcuts work on non-Latin and AZERTY layouts.
4. The numpad has key codes in both NumLock states.
5. The X11 path types the same text as the Wayland path.
6. Backends that are not ported yet behave exactly as today.

## Non-goals

- Shortcuts bound to national letters (ä, ß, é). They have no `KeyCode` and keep
  none; a later issue can add a rune-based `Shortcut`.
- Physical-position shortcuts (games). A separate feature if wanted.
- Windows, macOS, web. Their ports reuse the rules below in later issues.

## Design

### 1. Two key identities per press

`EventKeyDown` and `EventKeyUp` keep `KeyCode` and `Modifiers` and gain two
fields:

- `TypedKey KeyCode` — the key the press produced with Shift, AltGr and NumLock
  applied. Shift+7 on German is `KeySlash`; AltGr+8 is `KeyLeftBracket`; Shift+A
  is still `KeyA`.
- `TypedModifiers Modifier` — `Modifiers` without the ones the translation used
  up (libxkbcommon's _consumed modifiers_). For Shift+7 on German it is
  `ModNone`; for Ctrl+Shift+7 it is `ModCtrl`.

When the press produced the same key as `KeyCode` (letters, Shift+A), or a
symbol with no key code (Cyrillic с, ä), the backend sets `TypedKey = KeyCode`
and `TypedModifiers = Modifiers`. Case is not a different key: Ctrl+Shift+A must
not fire `Ctrl+A`.

A backend that leaves both fields zero gets today's matching.

### 2. `KeyCode`: one rule list for every layout

The first rule that applies wins. "Unmodified" means NumLock applied, nothing
else; "shifted" adds Shift.

1. **Modifier and named keys** by their own symbol: Shift, Control, Alt, Super,
   `ISO_Level3_Shift` on the right Alt key (`KeyRightAlt`), `Num_Lock`, `Menu`,
   `Scroll_Lock`, `Print`, `Pause`, F1–F25, Escape, Tab, Return, BackSpace,
   Insert, Delete, arrows, Home, End, Page Up/Down.
2. **Keypad.** NumLock on: `KP_0`–`KP_9` → `KeyKP0`–`KeyKP9`, `KP_Decimal` and
   `KP_Separator` → `KeyKPDecimal`. NumLock off: `KP_Home` → `KeyHome` and so on
   for every navigation symbol, `KP_Delete` → `KeyDelete`, `KP_Insert` →
   `KeyInsert`, `KP_Begin` → `KeyInvalid`. In both states the operators map to
   `KeyKPDivide`, `KeyKPMultiply`, `KeyKPSubtract`, `KeyKPAdd`, and `KP_Enter`
   stays `KeyEnter` (as on web).
3. **Unmodified ASCII letter or digit** → that key (`KeyA`…`KeyZ`,
   `Key0`…`Key9`).
4. **Shifted ASCII digit** → that digit. This is the AZERTY row.
5. **Non-Latin layout** (the active layout's A–D keys type no ASCII letter):
   - the unmodified letter or digit of the first configured Latin layout on the
     same physical key, else
   - the US letter or digit at that physical position.
6. **Unmodified printable ASCII** → its key code (§ 3).
7. **Dead keys** by their spacing ASCII character: `dead_grave` → `` ` ``,
   `dead_circumflex` → `^`, `dead_tilde` → `~`. Others → `KeyInvalid`.
8. Otherwise `KeyInvalid`.

`EventKeyUp` repeats the `KeyCode` and `TypedKey` its press reported, even if
NumLock or the layout changed while the key was held.

### 3. Key codes for the rest of printable ASCII

`KeyCode` values for ASCII already equal the character code. The 21 printable
characters without a constant get one, with the character code as value:

`KeyExclamation` (!), `KeyQuotation` ("), `KeyNumberSign` (#), `KeyDollar` ($),
`KeyPercent` (%), `KeyAmpersand` (&), `KeyLeftParen` ((), `KeyRightParen` ()),
`KeyAsterisk` (\*), `KeyPlus` (+), `KeyColon` (:), `KeyLess` (<), `KeyGreater`
(>), `KeyQuestion` (?), `KeyAt` (@), `KeyCircumflex` (^), `KeyUnderscore` (\_),
`KeyLeftBrace` ({), `KeyBar` (|), `KeyRightBrace` (}), `KeyTilde` (~).

None collides: the named keys start at 256 and the letters are 65–90.

`keyScrollLock`, `keyPrintScreen`, `keyPause` and `keyF21`–`keyF24` are
exported, since rule 1 now emits them.

### 4. `ModAltGr`

A new modifier bit, `ModAltGr` (`0x10`), is set while ISO level 3 is active
(AltGr, or whatever key the layout binds to it). It takes part in exact
matching, so AltGr+8 (`Key8` + `ModAltGr`) no longer matches `{Key8}`. It is
consumed when it changed the symbol, so AltGr+8 matches `{KeyLeftBracket}`.

### 5. Matching

`Shortcut.matches` stays exact. Command dispatch makes two passes per key down:

1. **Exact:** `e.KeyCode == s.Key && e.Modifiers == s.Modifiers`.
2. **Typed**, only if no command matched in pass 1 and
   `e.TypedKey != KeyInvalid`:
   `e.TypedKey == s.Key && e.TypedModifiers == s.Modifiers`.

This is the rule libxkbcommon documents for shortcuts (see Sources). The pass
order settles a press that fits two commands: on US, Ctrl+Shift+= is exact
`{KeyEqual, ModCtrlShift}` and typed `{KeyPlus, ModCtrl}`; the exact one wins.

Widgets that compare `KeyCode` directly keep doing so and gain rules 2, 4 and 5
for free (numpad navigation in inputs, Ctrl+C on Russian).

The two passes run within each dispatch phase: `Global` commands before the
widgets, the others after. A `Global` command's typed match therefore still runs
before a non-global command's exact match, as `Global` already takes precedence.

"Used up" is libxkbcommon's `XKB_CONSUMED_MODE_GTK`, the mode made for
shortcuts: a modifier counts only if it changes what the key produces. The X11
default mode would also count Ctrl and Alt on keys that have a Ctrl+Alt level
(F1 → `XF86Switch_VT_1`).

### 6. Backends

- **Wayland:** libxkbcommon is already loaded. `TypedKey` comes from
  `xkb_state_key_get_one_sym`, `TypedModifiers` from
  `xkb_state_key_get_consumed_mods2`, `ModAltGr` from the `LevelThree` modifier.
  Missing libxkbcommon still falls back to X11, as today.
- **X11:** stays on the core keymap read through xgb, with no new library
  (`gui/backend/gl/keys_x11.go`). Measured on Xorg 21 with an xgb client:
  - Xorg publishes per key the first group's two levels, the second group's two
    (a copy of the first's on a one-group key), then the first group's further
    levels and after them the second group's. Where the second group's further
    levels start depends on how many levels the key has in the first, which the
    core keymap does not say, so level 3 is read for the first group only.
  - The modifier map names the modifiers of `Num_Lock` (Mod2) and
    `ISO_Level3_Shift` (Mod5, through a virtual key, not RALT); they are read
    from it, not assumed.
  - A core client sees no group in the key event state, with or without
    `XkbUseExtension`, under `xdotool` injection. The path reads the group from
    bits 13-14 when present; whether a real X11 desktop reports it is the
    hardware test of phase 7.

  Where the core protocol and XKB disagree, the path follows XKB, which every
  Xlib and GTK application on the same desktop uses: on a keypad key only
  NumLock picks the keysym (the protocol would let Shift+keypad 7 type 7), and
  the used-up modifiers are counted as libxkbcommon's GTK mode counts them. Caps
  Lock capitalizes as the protocol says, Latin-1 and Unicode keysyms directly
  and legacy ones (Cyrillic, Greek) through the key's shifted keysym. Characters
  come from libxkbcommon's `xkb_keysym_to_utf32` when it is loaded, else from
  `x11key.KeysymToRune`: Latin-1, the keypad's printable keys, `EuroSign`
  (`€`) and direct-Unicode keysyms, each typed as libxkbcommon types it (a
  test compares the two tables). Without libxkbcommon, text in a script with
  legacy keysyms (Cyrillic, Greek, Hebrew) still types nothing on X11; its
  shortcuts work.

  The core keymap cannot describe every key as XKB does. On these the X11 result
  follows the core keymap and may differ from Wayland; the sweep counts them and
  compares everything else:
  - a level up to the fourth explicitly NoSymbol (the core keymap's empty column
    means "absent"): AltGr on a key whose levels 3 and 4 are both empty types
    the base symbol (ara), Shift on an empty level 2 types the unshifted one
    (ma(tifinagh));
  - a one-level key with a cased letter: the protocol (#948) gives the capital
    with Shift, XKB does not (in(tamilnet_TAB));
  - keypad types other than the plain two-level one (be(oss_latin9),
    ir(pes_keypad), ml(fr-oss));
  - ISO level 5 (Mod3: de(T3), neo).

  libxkbcommon-x11 is not used: Debian installs it only with Qt. Compiling the
  server's layout from `_XKB_RULES_NAMES` with libxkbcommon is the way to close
  the gaps above if they matter; the sweep did not show a reason to.

## Decisions taken

Confirmed by the maintainer on 2026-10-28, on condition that US is not made
worse (see § US regression guard).

| #   | Decision                                 | Taken                                     | Alternative                                   |
| --- | ---------------------------------------- | ----------------------------------------- | --------------------------------------------- |
| D1  | Names of the two new `Event` fields      | `TypedKey`, `TypedModifiers`              | `ShortcutKey`, `ConsumedModifiers` (raw mask) |
| D2  | 21 ASCII key constants (§ 3)             | add them now                              | later; `#`, `+`, `<`, `^` stay `KeyInvalid`   |
| D3  | AltGr as `ModAltGr` in `Modifiers` (§ 4) | yes                                       | keep it out; only suppress the exact pass     |
| D4  | NumLock-off keypad keys                  | navigation codes (`KeyHome`)              | always `KeyKP7` (GLFW, physical)              |
| D5  | Dead keys (rule 7)                       | spacing ASCII character                   | `KeyInvalid` as today                         |
| D6  | Non-Latin fallback order (rule 5)        | configured Latin layout, then US position | US position only (SDL)                        |

### US regression guard

On the default `us` layout nothing that works today may change:

- Every key with a `KeyCode` today keeps it, with the same `Modifiers`, for
  every combination of Shift, Ctrl, Alt and Super. Keys that are `KeyInvalid`
  today (numpad, the 105-key `<>` key, NumLock, Menu) gain a code.
- `ModAltGr` is never set: `us` has no level 3.
- The typed pass never fires a shortcut built from a constant that exists before
  this change. Every character US types with Shift
  (`! @ # $ % ^ & * ( ) _ + { } | : " < > ? ~`) maps to a constant from § 3, so
  only new shortcuts can match it.

The one intended change on US: numpad keys with NumLock off now navigate in text
fields.

## Golden tables

From xkeyboard-config 2.42 (libxkbcommon 1.7, Debian trixie). Key names are XKB
names (`AE07` is the key US labels 7). `—` is `KeyInvalid`/none. Modifier
columns list `Modifiers` → `TypedModifiers`.

### `de(nodeadkeys)` — the reporter's layout

| Press            | KeyCode                                | Char | TypedKey         | Modifiers → Typed | Fires                                        |
| ---------------- | -------------------------------------- | ---- | ---------------- | ----------------- | -------------------------------------------- |
| AE07             | `Key7`                                 | 7    | `Key7`           | – → –             | `{Key7}`                                     |
| Shift+AE07       | `Key7`                                 | /    | `KeySlash`       | Shift → –         | `{Key7, Shift}`, else `{KeySlash}`           |
| Ctrl+Shift+AE07  | `Key7`                                 | —    | `KeySlash`       | Ctrl+Shift → Ctrl | `{Key7, CtrlShift}`, else `{KeySlash, Ctrl}` |
| AltGr+AE08       | `Key8`                                 | [    | `KeyLeftBracket` | AltGr → –         | `{KeyLeftBracket}`; not `{Key8}`             |
| AltGr+AD01       | `KeyQ`                                 | @    | `KeyAt`          | AltGr → –         | `{KeyAt}`; not `{KeyQ}`                      |
| AltGr+AE11       | —                                      | \\   | `KeyBackslash`   | AltGr → –         | `{KeyBackslash}`                             |
| BKSL             | `KeyNumberSign`                        | #    | `KeyNumberSign`  | – → –             | `{KeyNumberSign}`                            |
| Shift+BKSL       | `KeyNumberSign`                        | '    | `KeyApostrophe`  | Shift → –         | `{KeyApostrophe}`                            |
| Shift+AB08       | `KeyComma`                             | ;    | `KeySemicolon`   | Shift → –         | `{KeySemicolon}`                             |
| Shift+AE10       | `Key0`                                 | =    | `KeyEqual`       | Shift → –         | `{KeyEqual}`                                 |
| Shift+AE12       | —                                      | \`   | `KeyGraveAccent` | Shift → –         | `{KeyGraveAccent}`                           |
| AD12             | `KeyPlus`                              | +    | `KeyPlus`        | – → –             | `{KeyPlus}`                                  |
| TLDE             | `KeyCircumflex`                        | ^    | `KeyCircumflex`  | – → –             | `{KeyCircumflex}`                            |
| LSGT             | `KeyLess`                              | <    | `KeyLess`        | – → –             | `{KeyLess}`                                  |
| AC11             | —                                      | ä    | —                | – → –             | nothing                                      |
| KP7, NumLock on  | `KeyKP7`                               | 7    | `KeyKP7`         | – → –             | `{KeyKP7}`                                   |
| KP7, NumLock off | `KeyHome`                              | —    | `KeyHome`        | – → –             | `{KeyHome}`; Input moves the caret           |
| KPDL, NumLock on | `KeyKPDecimal`                         | ,    | `KeyKPDecimal`   | – → –             | `{KeyKPDecimal}`                             |
| NMLK, RALT, COMP | `KeyNumLock`, `KeyRightAlt`, `KeyMenu` | —    | same             | – → –             | the named key                                |

`de` (with dead keys) differs only in TLDE (`dead_circumflex` → `KeyCircumflex`
by rule 7) and AE12 (`dead_acute` → `KeyInvalid`).

### `fr` — AZERTY digits

| Press      | KeyCode    | Char | TypedKey        | Modifiers → Typed | Fires                                       |
| ---------- | ---------- | ---- | --------------- | ----------------- | ------------------------------------------- |
| AE01       | `Key1`     | &    | `KeyAmpersand`  | – → –             | `{Key1}`, else `{KeyAmpersand}`             |
| Shift+AE01 | `Key1`     | 1    | `Key1`          | Shift → Shift     | `{Key1, Shift}`                             |
| Ctrl+AE01  | `Key1`     | —    | `KeyAmpersand`  | Ctrl → Ctrl       | `{Key1, Ctrl}`, else `{KeyAmpersand, Ctrl}` |
| AE08       | `Key8`     | \_   | `KeyUnderscore` | – → –             | `{Key8}`, else `{KeyUnderscore}`            |
| AltGr+AE10 | `Key0`     | @    | `KeyAt`         | AltGr → –         | `{KeyAt}`                                   |
| AD01       | `KeyA`     | a    | `KeyA`          | – → –             | `{KeyA}`                                    |
| Shift+AB09 | `KeyColon` | /    | `KeySlash`      | Shift → –         | `{KeySlash}`                                |

Shift+AE01 keeps Shift: the typed key equals `KeyCode` (§ 1).

### `ru` and `us,ru` — non-Latin

| Layout (active)      | Press      | KeyCode    | Char | TypedKey      | Fires           |
| -------------------- | ---------- | ---------- | ---- | ------------- | --------------- |
| `ru`                 | Ctrl+AB03  | `KeyC`     | —    | `KeyC`        | `{KeyC, Ctrl}`  |
| `ru`                 | AB03       | `KeyC`     | с    | `KeyC`        | `{KeyC}`        |
| `ru`                 | Shift+AE07 | `Key7`     | ?    | `KeyQuestion` | `{KeyQuestion}` |
| `ru`                 | LSGT       | `KeySlash` | /    | `KeySlash`    | `{KeySlash}`    |
| `us,ru` (ru)         | Ctrl+AB03  | `KeyC`     | —    | `KeyC`        | `{KeyC, Ctrl}`  |
| `us(dvorak),ru` (ru) | Ctrl+AB03  | `KeyJ`     | —    | `KeyJ`        | `{KeyJ, Ctrl}`  |

The last row is where D6 matters: the configured Latin layout (Dvorak) wins over
the US position.

### Others

| Layout                   | Press            | KeyCode        | Char | TypedKey         | Note                                 |
| ------------------------ | ---------------- | -------------- | ---- | ---------------- | ------------------------------------ |
| `us(dvorak)`             | AB03             | `KeyJ`         | j    | `KeyJ`           | letters by layout, not position      |
| `ch`                     | Shift+AE01       | `Key1`         | +    | `KeyPlus`        |                                      |
| `ch`                     | AltGr+AD11       | —              | [    | `KeyLeftBracket` |                                      |
| `ch`                     | BKSL             | `KeyDollar`    | $    | `KeyDollar`      |                                      |
| `br`                     | AB11             | `KeySlash`     | /    | `KeySlash`       | ABNT2 extra key                      |
| `br`                     | KPDL, NumLock on | `KeyKPDecimal` | ,    | `KeyKPDecimal`   | `KP_Separator`                       |
| `jp`                     | AE13 and AB11    | `KeyBackslash` | \\   | `KeyBackslash`   | two keys, one code: the layout's own |
| `jp`                     | AD11             | `KeyAt`        | @    | `KeyAt`          |                                      |
| `ru`, `jp`, `us(dvorak)` | RALT             | `KeyRightAlt`  | —    | `KeyRightAlt`    | `Alt_R`, so `ModAlt`, not `ModAltGr` |

## Verification

1. **Golden tests** (`gui/backend/gl`, Linux): the tables above as key-sequence
   tests in the style of libxkbcommon's `test/keyseq.c`. Keymaps are compiled
   from layout names at test time; nothing from xkeyboard-config is checked in.
2. **US regression test:** every key of `us` × {none, Shift, Ctrl, Ctrl+Shift,
   Alt, Super} against today's `KeyCode`/`Modifiers`, as § US regression guard
   states.
3. **All-layout sweep:** for every layout and variant in `evdev.xml`, every key,
   with none/Shift/AltGr/Shift+AltGr and NumLock on/off:
   - `EventChar` equals libxkbcommon's character, on both paths;
   - every key that types something or has a name gets a `KeyCode`, or is listed
     as an exception;
   - key up repeats key down's codes;
   - `ModAltGr` is set only with level 3;
   - Caps Lock and NumLock never appear in `Modifiers`.
4. **X11 core path against Wayland:** the tests build the core keymap Xorg
   derives for a layout from libxkbcommon's compile of it, in the column order
   measured on Xorg (§ 6), and press keys through the X11 path. The golden
   tables run on both paths; the sweep checks that X11 reports what Wayland does
   on every key the core keymap can describe.
5. **Backend check:** the Docker Xorg harness (Xorg with the dummy driver, keys
   injected by keycode with `xdotool`) runs the reporter's probe built from the
   branch: the de(nodeadkeys) and ru presses of the golden tables give the same
   results on real Xorg.
6. **Hardware:** the reporter (KDE, Wayland) and a Debian laptop.

CI installs `libxkbcommon0` and `xkb-data`. `GOGUI_REQUIRE_XKB=1` turns the
tests' skip into a failure there, as `GOGUI_REQUIRE_GL` does. Under `-race` the
sweep checks every tenth layout: it is one goroutine, so the race detector adds
nothing, and the full sweep takes ~100 s there instead of ~10 s.

## Phasing

| Phase | Content                                                                 | Status  |
| ----- | ----------------------------------------------------------------------- | ------- |
| 1     | This spec; decisions D1–D6                                              | done    |
| 2     | Test harness: libxkbcommon bindings for tests, golden tables, sweep     | done    |
| 3     | `KeyCode` rules (§ 2), new constants (§ 3), `ModAltGr`; Wayland         | done    |
| 4     | `TypedKey`/`TypedModifiers`, two-pass matching; Wayland                 | done    |
| 5     | X11 core path: NumLock, level 3, groups, consumed, characters           | done    |
| 6     | Docs: `docs/deployment.md`, CHANGELOG, a keyboard note in the guides    | pending |
| 7     | Hardware: reporter (KDE, Wayland), Debian laptop (X11 layout switching) | pending |

## Sources consulted

Read for understanding; no code, comments or tables were copied. Qt, GTK and
Firefox are under copyleft licenses and must not be ported.

- libxkbcommon, "Consumed Modifiers" in `include/xkbcommon/xkbcommon.h` — the
  matching rule of § 5.
- X Window System Protocol, § 5 Keyboards — the NumLock rule.
- The X Keyboard Extension: Protocol Specification — levels, groups.
- xkeyboard-config, `symbols/*` and `rules/evdev.xml` — the golden tables.
- W3C UI Events (legacy `keyCode`, `code` and `key` values).
- GTK 4 `gdk_key_event_matches`, Qt `QXkbCommon::possibleKeyCombinations` and
  `lookupLatinKeysym`, Firefox `KeymapWrapper::ComputeDOMKeyCode`, SDL3 keycode
  options, GLFW `createKeyTables`, Chromium `DifficultKeyboardCode`.

## Rejected Approaches

- **Physical key codes as the fix** (GLFW, W3C `code`): `KeyZ` would be the key
  labelled Y on QWERTZ, and none of the reported shortcuts would work.
- **Per-layout tables in go-gui:** fix one layout and miss the next.
- **Unicode code points as `KeyCode`** (SDL3): U+0100–U+017F collides with the
  named keys at 256–348.
- **Chromium's character-to-Windows-key tables:** large, carry their own
  copyright, and tie Linux to Windows semantics.
- **libxkbcommon-x11 for the X11 path:** Debian pulls it in only with Qt; GTK,
  XFCE and MATE machines often lack it.
- **An XKB-extension client in Go:** re-implements libxkbcommon-x11.
- **Bundling libxkbcommon:** needs cgo.
- **Porting GTK, Qt or Firefox code:** LGPL, GPL and MPL; go-gui is MIT.
