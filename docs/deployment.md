# Deploying your app

`go build` produces a bare binary. `cmd/buildapp` turns it into something you
can hand to a user: a signed `.app` on macOS, an icon-embedded `.exe` in a
`.zip` on Windows, a menu-installable tarball on Linux. One binary in, one
artefact out.

| Platform | Output                               | What packaging adds                        |
| -------- | ------------------------------------ | ------------------------------------------ |
| macOS    | signed `.app` (+ `.dmg` with `-dmg`) | `Info.plist`, `.icns` icon, code signature |
| Windows  | `.zip` holding the `.exe`            | icon resource embedded in the PE image     |
| Linux    | `.tar.gz`                            | `.desktop` entry, icon, `install.sh`       |

The full flag reference lives in
[`cmd/buildapp/README.md`](../cmd/buildapp/README.md). This page covers the
standard path end to end.

## Step 1: compile for the target

A C toolchain is needed only on **macOS** (the Metal backend is Objective-C).
Linux and Windows build fully cgo-free.

```bash
# Windows (amd64). -H windowsgui marks the PE as a GUI-subsystem image:
# without it the loader allocates a console, so an empty terminal window
# opens behind the app window.
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -ldflags "-H windowsgui" -o build/myapp.exe ./myapp/

# Linux (amd64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -o build/myapp ./myapp/

# macOS (host toolchain, Apple Silicon or Intel)
go build -o build/myapp ./myapp/
```

Or let buildapp compile for you with `-build-pkg` (same flags, one step —
`-H windowsgui` is added on Windows automatically). It takes no positional
binary then:

```bash
go run ./cmd/buildapp -platform windows -arch amd64 -build-pkg ./myapp/ \
  -o build -name "My App" -icon icon.png
```

Cross-compiling to macOS from another OS still needs the macOS SDK, so build the
macOS binary on a Mac.

Stage the binary under a clean name before packaging: the executable's basename
becomes the installed program name (the `Exec=` line on Linux, the file inside
the `.zip` on Windows, `Contents/MacOS/<name>` on macOS). `myapp-linux` in means
`Exec=myapp-linux` out.

On Linux/X11, transparent and translucent windows need a running compositing
manager at runtime — without one the window renders black, which `gui.Debug`
reports. Either depend on one in the package or document it for the user. See
`docs/specs/transparent-windows.md`.

On a Wayland desktop the app runs through XWayland unless `GOGUI_WAYLAND=1` is
set, which selects the experimental native Wayland backend (amd64 and arm64). It
opens these libraries at run time, so the binary does not link them:

| Library                        | Debian / Ubuntu package   | Without it                                 |
| ------------------------------ | ------------------------- | ------------------------------------------ |
| `libwayland-client.so.0`       | `libwayland-client0`      | warning, falls back to X11                 |
| `libwayland-egl.so.1`          | `libwayland-egl1`         | warning, falls back to X11                 |
| `libxkbcommon.so.0`            | `libxkbcommon0`           | warning, falls back to X11                 |
| `libdecor-0.so.0` and a plugin | `libdecor-0-plugin-1-gtk` | no window frame on GNOME, Cinnamon, weston |

KDE and wlroots compositors (sway) draw the frame themselves and need no
libdecor. See `docs/specs/wayland-backend.md`.

## Step 2: write the app manifest

Keep the app's identity in one file, `appinfo.toml`, beside `main.go`. Then the
name, ID and version are not typed again in each Makefile target, CI workflow
and Go file.

```toml
id      = "com.example.myapp"
name    = "My App"
version = "1.0.0"
icon    = "icon.png"
```

buildapp reads this file (step 3). The app reads the same bytes at run time, so
the bundle ID and the runtime ID are the same. If they differ, macOS keeps
preferences and permission grants under two identities and reports no error.

## Reading the manifest at run time

Embed the file and give it to `gui.WindowCfg.AppInfo`. The window takes its
title from `name`. The app takes its file-access ID, X11 `WM_CLASS` and native
menubar name from the file. A value that you set in Go wins over the file.

```go
//go:embed appinfo.toml
var manifest []byte

// info is parsed once at start-up. MustParse panics on a bad file: the
// file is compiled into the program, so a bad one is a build mistake.
var info = appinfo.MustParse(manifest)

func newWindow() *gui.Window {
	// AppInfo gives the window its title, and the app its file-access
	// ID, X11 WM_CLASS and menubar name. No other place spells them.
	return gui.NewWindow(gui.WindowCfg{
		AppInfo: info,
		State:   &App{},
		Width:   480,
		Height:  320,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
	})
}
```

This code is from `examples/app_manifest/main.go`. The format and the flag
precedence are in
[`cmd/buildapp/README.md`](../cmd/buildapp/README.md#app-manifest).

## Step 3: package with buildapp

Run buildapp from the directory that holds `appinfo.toml`, or pass
`-manifest path`. A flag that you give wins over the file. Without a manifest,
give the identity as flags (`-name`, `-id`, `-version`, `-icon`), as below.

A `.png` icon works on every platform. macOS also accepts `.icns`, Windows also
accepts `.ico`.

```bash
# Windows → myapp-1.0.0-windows-amd64.zip
go run ./cmd/buildapp -platform windows -o build -version 1.0.0 \
  -name "My App" -icon icon.png build/myapp.exe

# Linux → myapp-1.0.0-linux-amd64.tar.gz
go run ./cmd/buildapp -platform linux -o build -version 1.0.0 \
  -name "My App" -icon icon.png build/myapp

# macOS → "My App.app" plus my-app-1.0.0.dmg (run on a Mac:
# needs sips, iconutil, codesign, hdiutil for -dmg)
go run ./cmd/buildapp -o build -dmg -version 1.0.0 \
  -name "My App" -icon icon.png build/myapp
```

`-platform` defaults to the host `GOOS`, so the macOS invocation above omits it.
The binary must match the platform: Mach-O for `darwin`, PE for `windows`, ELF
for `linux`. `make release` runs this whole flow over `examples/showcase`.

## Icons

- Windows embeds the icon by appending a `.rsrc` section (`RT_ICON` /
  `RT_GROUP_ICON`). A PNG is embedded verbatim (valid since Vista); a `.ico`
  contributes all of its images.
- Refused when the binary already carries a resource directory, which happens
  when a `.syso` file was linked in. Use one or the other, not both.
- On Linux the `.desktop` entry sets `Terminal=false`, so no terminal emulator
  opens beside the app. Install with `./install.sh` (copies into `~/.local`, no
  privileges needed) or `./install.sh /usr/local` with `sudo` for a system-wide
  install.

## macOS signing

The bundle is always signed, ad-hoc (`-`) by default. Ad-hoc is fine for local
testing and for freshly downloaded releases, but every rebuild changes the
cdhash, which silently revokes TCC-gated permissions (camera, microphone, screen
recording, accessibility, full disk access) granted to the previous build. For a
stable identity during development, create a self-signed Code Signing
certificate (Keychain Access → Certificate Assistant → Create a Certificate) and
pass it explicitly:

```bash
go run ./cmd/buildapp -sign "My Dev Cert" -name "My App" build/myapp
```

or set it once per machine with `BUILDAPP_SIGN_IDENTITY`. Verify with
`codesign -dv --verbose=4 "My App.app"` (ad-hoc prints `Signature=adhoc`; a real
identity prints an `Authority=` line).

`-bundle-deps` copies non-system dylibs into `Contents/Frameworks` and rewrites
their load paths — needed only if the app links libraries outside `/usr/lib` and
`/System`.

Distribution signing (hardened runtime, entitlements, notarization with
`notarytool`) is built in: pass `-entitlements` for the hardened-runtime
signature and add `-notarize -notary-profile <profile>` to submit and staple.
Without `-entitlements`, `-notarize` is an error. See
[`cmd/buildapp/README.md`](../cmd/buildapp/README.md#distribution-signing-and-notarization).
The Windows `.exe` is not Authenticode signed — that is a separate issue; run
`signtool` separately if needed.

## Mobile

Mobile targets build through the standard Go mobile tooling rather than buildapp
— see `make build-ios` (`go build -buildmode=c-archive` for `examples/ios_demo`)
and `make build-android` (`gomobile bind` to an `.aar` for
`examples/android_demo`) in the `Makefile`.

On mobile, the app still reads the embedded `appinfo.toml` at run time, so the
per-app storage ID is the manifest ID. But the package ID comes from the host
project: the Xcode project's `Info.plist` on iOS and the Gradle `applicationId`
on Android. The Go tools build a library (`c-archive` or `.aar`) and do not
write those files, so buildapp cannot set them. Copy the manifest `id` into the
host project by hand.

## Worked example

`examples/app_manifest` has an `appinfo.toml`, so buildapp needs no identity
flags:

```bash
go build -o /tmp/app_manifest ./examples/app_manifest/
cd examples/app_manifest
go run ../../cmd/buildapp -o /tmp /tmp/app_manifest
open "/tmp/App Manifest Demo.app"  # macOS; on Linux/Windows add -platform
```
