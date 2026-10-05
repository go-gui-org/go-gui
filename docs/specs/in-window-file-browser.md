# Spec: in-window file browser fallback

Issue: #831. Status: **implemented** (unreleased).

## Problem

`NativeOpenDialog`, `NativeSaveDialog` and `NativeFolderDialog` had no fallback.
When there was no native picker, `OnDone` got a `DialogError` at once and the
user saw nothing. Three cases reach this:

- A nil `NativePlatform` (tests and headless use): `"unsupported"`.
- A build with `gui/backend/filedialog/dialog_other.go`: `"unsupported"`.
- Linux with no `zenity` or `kdialog` on `PATH`: `"no_dialog_tool"`.

Message, confirm and prompt already had in-window versions. The file dialogs did
not.

## Decision

The three file dialog impls in `gui/native_dialog.go` open an in-window file
browser (`gui/file_browser.go`) when the platform is nil, or when its result is
a `DialogError` with code `"unsupported"` or `"no_dialog_tool"`. The fallback is
always on. There is no opt-in flag.

`Window.InWindowOpenDialog`, `InWindowSaveDialog` and `InWindowFolderDialog`
open the same browser on purpose and skip the native picker. They take the
`Native*DialogCfg` types and check the `Cfg` the same way. They were added so
the showcase can show the browser on macOS, where the native picker always
exists. Their doc comments say that the paths have no macOS security-scoped
grant.

- **Chosen by default, in tests too.** A nil platform shows the browser, so
  dialog flows can be driven with `TestKey`/`TestClick` and recorded as goldens.
- **Not for real errors.** A bad `Cfg` (`"invalid_cfg"`) and any other platform
  error still go to `OnDone`.
- **A `DialogCustom` dialog.** Escape, the focus trap and the one dialog slot
  come from `gui/view_dialog.go`. The state is in `Window.fileBrowser`, because
  a window shows at most one dialog.
- **No I/O in the view.** The folder is read when it changes, in an event
  handler. The view reads the cached entries.
- **Same filter checks as the native path.** The browser uses
  `nativeExtensionsFromFilters` and `nativeNormalizeExtension`, so the two
  cannot drift.

## Behaviour

- Layout, top to bottom: path bar (Enter goes to the typed folder), filter
  select (only with two or more filters), entry list, name field (save only), an
  error or overwrite line, and the OK/Cancel buttons.
- The list puts `..` first when there is a folder and it has a parent, then
  folders, then files. Names sort without case. Dotfiles are hidden. A file
  shows only when its extension is in the active filter. Folder mode lists
  folders only. A folder shows with a `/` after its name on every OS.
- **Open.** The row under the cursor is the choice. With `AllowMultiple`,
  Ctrl/Cmd-click marks files, and the marked files are the choice. A
  Ctrl/Cmd-click is never half of a double-click, so two quick ones unmark a
  file. Enter or a double-click opens a folder and accepts a file. OK is
  disabled until the choice is a file.
- **Save.** A click on a file copies its name to the name field. A name that is
  empty, `.`, `..`, or holds a path separator shows an error. On Windows so does
  a name with `<>:"|?*`, as in the native dialog: `a.txt:x` would write an NTFS
  alternate data stream inside `a.txt`. A name that is a folder opens that
  folder; this is checked before `DefaultExtension` is added to a name with no
  extension, so `sub` opens `sub` rather than saving `sub.txt`. If the file
  exists and `ConfirmOverwrite` is set, the browser asks. The second OK for the
  same path saves. A check that fails for a reason other than "no such file"
  shows the error and does not save. A folder name that cannot be opened keeps
  the typed name.
- **Folder.** OK returns the current folder. Enter or a double-click goes into a
  folder.
- **Start folder.** `StartDir` (after `safeStartDir`), then the working folder,
  then the home folder. The first readable one is used. If none can be read, the
  browser opens with the error shown, so the user can type a path. A folder that
  exists but may not be read shows the OS permission error, not "Not a folder".
  If a fallback opens, the start folder's error is still shown. Every folder is
  made absolute, so a relative `StartDir` gives absolute paths, as the native
  pickers do, and `..` can go above the working folder.
- **Result.** `DialogOK` with clean, joined paths and a zero `Grant`, or
  `DialogCancel`.
- **Exactly one report.** `Window.Dialog` and `Window.DialogDismiss` end an open
  browser with `DialogCancel`. So a browser that loses the dialog slot reports
  `DialogCancel`: Escape, Cancel, a second file dialog, an app dialog that
  replaces it, or an app `DialogDismiss`. Accept clears the state before it
  dismisses, so it does not also report a cancel. `Window.Dialog` reports the
  cancel after the new dialog holds the slot, so an `OnDone` that opens a dialog
  is not overwritten. If that dialog takes the slot from a browser that was
  opening, the new browser reports `DialogCancel` at once and keeps no state.
- **Path bar.** A typed path that starts at a root (a drive, a volume, or a
  leading separator) is used as given. Any other path is relative to the current
  folder. `filepath.IsAbs` alone is wrong on Windows, where `\proj` is not
  "absolute". Enter on an empty path bar does nothing.

## Rejected Approaches

- **An exported `FileBrowser` widget.** No consumer needs to embed one. Export
  would fix the Cfg shape and the state keying before anyone uses them. Revisit
  when a sibling asks. The `InWindow*Dialog` methods open the browser as a
  dialog only, and add no new `Cfg` type.
- **A new `FileBrowserCfg` for the `InWindow*Dialog` methods.** The
  `Native*DialogCfg` types already carry every field the browser reads. Reusing
  them makes a switch between the native and in-window calls a rename.
- **A `WindowCfg` mode (auto / native / in-window).** A forced in-window mode on
  macOS gives no security-scoped grant, so a sandboxed app would lose file
  access with no error.
- **Opt-in per window.** Apps that never set the flag would keep the dead end.
- **A second dialog for the overwrite question.** The window has one dialog
  slot, so it would replace the browser. The question is a line in the browser.
- **ListBox for the rows.** ListBox takes Enter as select and has no activate
  hook. Adding `OnActivate` to `ListBoxCfg` is new API for one internal user.
  The rows are a `VirtualList`.
- **Reading the folder in the view function.** That is disk I/O on every frame
  under the frame lock.

## Open items

- The overwrite question, the bad-name error and the "not a folder" error are
  English. `Locale` has no fields for them yet.
- No toggle to show dotfiles.
