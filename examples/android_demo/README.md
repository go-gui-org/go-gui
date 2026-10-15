# Android Demo

> **Framework:** platform **Description:** Go-gui on Android via gomobile.
> Builds as an AAR and links into a native project.

![Preview](screenshot.png)

<!-- explorer: tags=platform category=platform run=mobile -->

---

# Android Demo

Demonstrates go-gui on Android via OpenGL ES 3.0.

The demo also supplies Android's host-side implementation of `gui.SaveSecret`,
`gui.LoadSecret` and `gui.DeleteSecret`. Entering a token exercises the same API
as `examples/secrets`; the UI reports only whether a token exists and its byte
length.

## Prerequisites

- Go 1.26+
- JDK 21 (Gradle 8.14 does not run on JDK 25)
- [gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile):
  `go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init`
- Android SDK Platform 35, Build Tools 35.0.0, and an NDK (r30 tested)
- Android device (arm64) or emulator (x86_64), API 24+

Use the checked-in Gradle wrapper; the project pins Gradle 8.14, Android Gradle
Plugin 8.10.0 and Kotlin 2.1.20. Android Studio may offer newer versions, but an
upgrade is not required to build this example. Keep SDK and JDK locations in
environment variables or the untracked `android/local.properties`, never in a
committed machine-specific Gradle setting.

## Build

```bash
# Generate AAR from Go code
make bind

# Build debug APK
make build

# Install on connected device
make install
```

## Architecture

```
demo.go (Go, gomobile bind)
  → android.SetWindow / Start / Render
    → gles_android.c (GLES3 via NDK)

Kotlin host:
  MainActivity → GoGuiGLSurfaceView (EGL3 + stencil)
    → GoGuiRenderer.onDrawFrame → Render()
    → touch events → TouchBegan/Moved/Ended
```

## Secret storage bridge

The Android backend cannot put application-owned Kotlin into an AAR generated
from Go. An application using this bridge therefore supplies three small pieces,
all shown by this example:

1. Its bound Go package exports a gomobile-compatible `SecretStore` interface
   and a `SetSecretStore` function that forwards to the Android backend.
2. Its Android host supplies `AndroidSecretStore.kt`, which implements the
   generated app interface (`androidapp.SecretStore` in this demo).
3. Its Activity registers `AndroidSecretStore(applicationContext)` before the Go
   window starts.

Secret calls can block, so the Go demo runs them in goroutines and changes
window state only through `QueueCommand`.

The Kotlin implementation generates a non-exportable 256-bit AES key in Android
Keystore and encrypts each value with AES-GCM. Only versioned ciphertext is
atomically written under `noBackupFilesDir`; the app ID and key are
authenticated with the ciphertext. Missing loads return an empty byte array,
which go-gui maps to `gui.ErrSecretNotFound`, and deleting a missing value
succeeds.

The embedded `AppInfo.ID` is `com.example.androiddemo`, matching Gradle's
`applicationId`. Applications copying the bridge should likewise use their
application ID as `AppInfo.ID`, and should register their own store before
`android.Start`. If no store is registered, the Android backend returns
`gui.ErrSecretsUnsupported`. An exception from a registered Kotlin store crosses
gomobile as an ordinary Go error, so it cannot preserve that Go sentinel.

Gomobile copies byte arrays at the language boundary. The Kotlin store clears
its Java-owned save input after encryption; load plaintext must remain alive
until gomobile copies it into the returned Go slice. If the Keystore key is lost
while ciphertext remains, load reports an authentication error. Saving replaces
the unreadable entry with one encrypted under the new key; deleting removes it.

## Troubleshooting

- **gomobile not found**: Make sure that `$GOPATH/bin` is in `$PATH`
- **javac not found**: Set `JAVA_HOME` to JDK 21 and put its `bin` directory on
  `PATH`; gomobile invokes `javac` by name
- **NDK not found**: Set `ANDROID_NDK_HOME`. Alternatively, install the NDK
  through the Android Studio SDK Manager
- **Linker errors**: Make sure that the NDK includes the GLES3 headers
  (`$NDK/toolchains/llvm/prebuilt/*/sysroot/usr/include/GLES3/`)
- **Black screen**: Make sure that logcat shows no shader compilation errors
  under the `go-gui-gles` tag
