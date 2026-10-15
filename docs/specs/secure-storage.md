# Spec: secure secret storage

Issues: #920, #930. Status: **implemented** (unreleased).

## Problem

Apps keep access tokens for other services. `SaveSettings` (#848) writes plain
JSON, and its spec says "no secrets". Each app had to write its own store, and
go-kite keeps a `0o600` token file. The OS credential stores protect a secret
better: they encrypt it, tie it to the user's login and, on macOS, to the app.

## Decision

Three functions and two errors in `gui`:

```go
func LoadSecret(w *Window, key string) ([]byte, error)
func SaveSecret(w *Window, key string, value []byte) error
func DeleteSecret(w *Window, key string) error
var ErrSecretNotFound, ErrSecretsUnsupported error
```

- **Service name.** `w.AppInfo().ID`, checked as for settings. No ID returns
  `ErrNoAppID`.
- **Key.** 1 to 128 bytes of letters, digits, `.`, `-` and `_`. The key goes
  into store item names, so it has no separators.
- **Value.** Opaque bytes, 1 to 2560 bytes on every platform. 2560 is the
  Windows Credential Manager limit (`CRED_MAX_CREDENTIAL_BLOB_SIZE`). The same
  cap everywhere means a developer on macOS finds the limit, not a Windows user.
  Load also checks the cap, for an item another program wrote.
- **Copies.** Save keeps its own copy and Load returns a fresh slice, so the
  caller can clear its buffer.
- **Missing.** Load of a missing key returns `ErrSecretNotFound`. Delete of a
  missing key returns nil, so a sign-out path needs no check first.
- **No store.** `ErrSecretsUnsupported`. There is no plain-file fallback.

### Store selection

1. A window from `NewTestWindow`: memory, in `settingsState`.
2. A platform with the optional hook `SecretLoad` / `SecretSave` /
   `SecretDelete(service, key …)`: the hook.
3. Otherwise: `ErrSecretsUnsupported`.

The hook is an optional interface found by type assertion, as for settings.
`gui/backend/internal/keyring` implements it once; the Metal, iOS and GL
backends embed `keyring.Platform` in their `nativePlatform`, and the package
asserts the method set at compile time.

| Platform   | Store                                                                                                   |
| ---------- | ------------------------------------------------------------------------------------------------------- |
| macOS, iOS | Keychain generic password, service = app ID, account = key (cgo, Security.framework)                    |
| Windows    | Credential Manager generic credential, target `<app ID>/<key>`, persist local machine (syscall, no cgo) |
| Linux      | Secret Service over D-Bus (godbus), default collection, attributes `application` and `key`              |
| Android    | app-bound callback plus app-supplied Kotlin; the demo uses Android Keystore AES-GCM                     |
| Web        | none: `ErrSecretsUnsupported`                                                                           |

### Platform notes

- **macOS** uses the login keychain, not the data protection keychain. The data
  protection keychain needs a signed app with a keychain access group; the login
  keychain also works for `go run`. An unsigned binary changes at every build,
  so macOS asks the user to allow access again after a rebuild.
- **iOS** items are `AfterFirstUnlockThisDeviceOnly`: readable by background
  work after the first unlock, and not moved to another device by a restore.
- **Linux** opens a private session-bus connection and a "plain" session per
  call. The secret crosses only the user's own session bus. A locked collection
  shows the daemon's unlock prompt; the call waits up to two minutes. No bus, or
  no daemon that owns `org.freedesktop.secrets`, maps to
  `ErrSecretsUnsupported`. A missing default collection does too.
- **Windows**: `ERROR_NO_SUCH_LOGON_SESSION` (a service account, some remote
  sessions) maps to `ErrSecretsUnsupported`.
- **Android** applications own the Activity and Kotlin/Java host; the go-gui
  backend cannot package that application code. The application therefore
  exports a structurally matching Go interface and forwarding setter from the
  package passed to `gomobile bind`. Gomobile exposes that app interface to
  Kotlin (as `androidapp.SecretStore` in the demo), and the Activity registers
  its implementation before `android.Start`. The callback stays specific to
  secure storage instead of creating a general platform message channel.
  `examples/android_demo` supplies the complete host implementation. It uses a
  256-bit AES-GCM key generated in Android Keystore and atomically stores only
  versioned ciphertext under `noBackupFilesDir`. Its `AppInfo.ID` matches the
  Gradle `applicationId`, so the service namespace follows the installed app.
- **Android errors.** With no registered callback the backend returns
  `ErrSecretsUnsupported`. Exceptions raised by a registered Java/Kotlin host
  cross gomobile as ordinary Go errors and cannot retain an `errors.Is`
  relationship to that sentinel. A host that fails to initialize therefore
  reports an operational error rather than `ErrSecretsUnsupported`.
- **Android key loss.** If Android Keystore loses the key while ciphertext
  remains, load reports the authentication error and preserves the file. Save
  replaces it using the new key, and delete removes it. The demo does not
  silently discard data because the same failure can indicate corruption or
  tampering.
- **Android copies.** Gomobile copies byte arrays across the language boundary.
  The demo clears Kotlin's Java-owned save input in `finally`; a load result
  cannot be cleared before gomobile copies it back to Go.

### Blocking

A call can block while the OS shows a prompt (on Linux up to two minutes). Two
rules follow:

- The hook runs with `w.settings.mu` released. Only the memory store takes the
  lock. Otherwise one blocked secret call would also block every settings call,
  and the UI thread that makes it. `TestSecretHookRunsWithoutSettingsLock`
  checks this.
- The caller runs the call in a goroutine and posts the result with
  `QueueCommand`, never from a view function or an event callback.
  `examples/secrets` shows the pattern.

## Verification

- `gui/secrets_test.go`: memory store, copies, key and size checks, no-hook
  error with no file written, the hook path.
- `gui/backend/internal/hostsecret`: normal host tests cover missing-store,
  forwarding, host errors and the empty-load to `ErrSecretNotFound` mapping. CI
  cross-compiles the Android-specific registration and native hook wiring.
- `gui/backend/internal/keyring`: `GOGUI_SECRET_IT=1` runs a round trip against
  the real store. Run on macOS (Keychain) and on Linux in Docker with
  `gnome-keyring` under `dbus-run-session`; without a daemon the test skips on
  `ErrSecretsUnsupported`.

## Rejected Approaches

- **A leaf package with no `*Window`** (`secret.Get(service, account)`). The
  caller spells the app ID again, and there is no test memory store and no route
  to a backend hook.
- **`zalando/go-keyring` or `99designs/keyring`.** go-keyring on macOS runs the
  `security` tool with the secret in its arguments, which other processes can
  read. Neither covers iOS. A new dependency for code the backends can hold.
- **A plain `0o600` file when no store exists.** It quietly breaks the promise
  the API name makes. The app can choose that fallback itself.
- **An encrypted field in `SaveSettings`.** Moves key management to the app; the
  point is the OS store.
- **Typed `LoadSecret[T]` (JSON).** A struct can grow past the 2560-byte cap
  unseen. Secrets are opaque bytes.
- **Chunking large values across Windows credentials.** More failure modes (a
  partial write) for tokens that fit in 2560 bytes.
- **A general Go-to-Java message channel for Android.** It would expose an
  Android-only platform surface with no iOS counterpart. The narrow
  `SecretStore` callback fills the secure-storage gap; apps that need further
  Java APIs can bind their own typed gomobile interfaces.
