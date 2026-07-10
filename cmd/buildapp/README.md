# buildapp

Wraps a compiled go-gui binary into a macOS `.app` bundle so it can be
double-clicked, dragged to `/Applications`, and shown in the Dock with a
proper name and icon.

## Install

```
go install github.com/go-gui-org/go-gui/cmd/buildapp@latest
```

Or run directly from the repo:

```
go run ./cmd/buildapp [flags] <binary>
```

## Usage

```
buildapp [-o outdir] [-name Name] [-id bundle.id] [-icon icon.png|.icns] [-version 1.0] <binary>
```

Positional arg: path to a compiled Mach-O executable.

| Flag           | Default                 | Purpose                                             |
| -------------- | ----------------------- | --------------------------------------------------- |
| `-o`           | `.`                     | Output directory                                    |
| `-name`        | binary basename, capped | Bundle display name                                 |
| `-id`          | `local.gogui.<name>`    | `CFBundleIdentifier`                                |
| `-icon`        | none                    | `.png` (auto-converted) or `.icns`                  |
| `-version`     | `1.0`                   | `CFBundleVersion` / short version                   |
| `-bundle-deps` | `false`                 | Bundle non-system dylibs into `Contents/Frameworks` |

### `-bundle-deps`

When set, `buildapp` walks the binary's `LC_LOAD_DYLIB` entries via `otool -L`,
copies every non-system dylib (anything outside `/usr/lib`, `/System/Library`,
`/Library/Apple`) into `Contents/Frameworks/`, then uses `install_name_tool` to:

- rewrite each bundled dylib's own id to `@rpath/<basename>`
- rewrite every reference in the executable and in bundled dylibs to `@rpath/<basename>`
- add `@executable_path/../Frameworks` as an rpath on the executable

Transitive dependencies are followed. Every modified Mach-O file is ad-hoc
re-signed with `codesign -s -` (required on Apple Silicon). Requires `otool`,
`install_name_tool`, and `codesign` (Xcode Command Line Tools).

Verify a clean bundle:

```
find Foo.app -type f -perm +111 -exec otool -L {} \; | grep -E '/opt/homebrew|/usr/local'
```

Empty output means no host paths leaked into the bundle.

`.png` icons are converted to `.icns` via `sips` and `iconutil` (both
ship with macOS). Intermediate iconset files live in the system temp
directory and are removed on exit.

## Example

```
go build -o /tmp/getstarted ./examples/get_started
buildapp -o /tmp -name GetStarted -icon assets/icon.png /tmp/getstarted
open /tmp/GetStarted.app
```

## Bundle layout

```
GetStarted.app/
  Contents/
    Info.plist
    MacOS/getstarted
    Resources/getstarted.icns   (only when -icon supplied)
```

## Notes

- macOS only.
- Existing `.app` at the destination is overwritten without prompting.
- No code signing is performed. For Gatekeeper-friendly distribution,
  run `codesign` and `notarytool` separately.
- Shared libraries are not bundled; the target machine must
  have them installed.
