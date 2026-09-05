# Deploying your app

`go build` produces a bare binary. `cmd/buildapp` turns it into something you
can hand to a user: a signed `.app` on macOS, an icon-embedded `.exe` in a
`.zip` on Windows, a menu-installable tarball on Linux. One binary in, one
artefact out.

| Platform | Output                                 | What packaging adds                        |
| -------- | -------------------------------------- | ------------------------------------------ |
| macOS    | signed `.app` (+ `.dmg` via `hdiutil`) | `Info.plist`, `.icns` icon, code signature |
| Windows  | `.zip` holding the `.exe`              | icon resource embedded in the PE image     |
| Linux    | `.tar.gz`                              | `.desktop` entry, icon, `install.sh`       |

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

Stage the binary under a clean name before packaging: the executable's basename
becomes the installed program name (the `Exec=` line on Linux, the file inside
the `.zip` on Windows, `Contents/MacOS/<name>` on macOS). `myapp-linux` in means
`Exec=myapp-linux` out.

## Step 2: package with buildapp

A `.png` icon works on every platform. macOS also accepts `.icns`, Windows also
accepts `.ico`.

```bash
# Windows → myapp-1.0.0-windows-amd64.zip
go run ./cmd/buildapp -platform windows -o build -version 1.0.0 \
  -name "My App" -icon icon.png build/myapp.exe

# Linux → myapp-1.0.0-linux-amd64.tar.gz
go run ./cmd/buildapp -platform linux -o build -version 1.0.0 \
  -name "My App" -icon icon.png build/myapp

# macOS → "My App.app" (run on a Mac: needs sips, iconutil, codesign)
go run ./cmd/buildapp -o build -version 1.0.0 \
  -name "My App" -icon icon.png build/myapp

# macOS disk image for distribution
hdiutil create -srcfolder "build/My App.app" -volname "My App 1.0.0" \
  -format UDZO "build/My-App-1.0.0.dmg"
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
`notarytool`) is out of scope: run `codesign` and `notarytool` separately.
Likewise, the Windows `.exe` is not Authenticode signed — run `signtool`
separately if needed.

## Mobile

Mobile targets build through the standard Go mobile tooling rather than buildapp
— see `make build-ios` (`go build -buildmode=c-archive` for `examples/ios_demo`)
and `make build-android` (`gomobile bind` to an `.aar` for
`examples/android_demo`) in the `Makefile`.

## Worked example

```bash
go build -o /tmp/getstarted ./examples/get_started/
go run ./cmd/buildapp -o /tmp -name GetStarted \
  -icon gui/default_icon.png /tmp/getstarted
open /tmp/GetStarted.app  # macOS; on Linux/Windows add -platform
```
