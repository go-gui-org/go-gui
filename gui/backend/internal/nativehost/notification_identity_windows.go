//go:build windows

package nativehost

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type notificationIdentity struct {
	executable, appID, name, protocol string
	activator                         windows.GUID
}

func identityForExecutable(executable string) notificationIdentity {
	// The path remains stable over rebuilds; unrelated programs with the
	// same basename must not share notification settings or activation.
	canonical := strings.ToLower(filepath.Clean(executable))
	return notificationIdentityForKey(executable, canonical)
}

func notificationIdentityForKey(executable, key string) notificationIdentity {
	hash := sha256.Sum256([]byte(key))
	id := hex.EncodeToString(hash[:16])
	guid := windows.GUID{Data1: binary.LittleEndian.Uint32(hash[:4]), Data2: binary.LittleEndian.Uint16(hash[4:6]), Data3: binary.LittleEndian.Uint16(hash[6:8])}
	copy(guid.Data4[:], hash[8:16])
	guid.Data3 = (guid.Data3 & 0x0fff) | 0x8000 // custom name-derived UUID
	guid.Data4[0] = (guid.Data4[0] & 0x3f) | 0x80
	return notificationIdentity{
		// AUMID and protocol scheme are Windows registry identity,
		// not widget IDs.
		executable: executable, appID: "GoGui.App." + id, // ergonomics-audit:not-an-id
		name:     strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable)),
		protocol: "gogui-notification-" + id, activator: guid, // ergonomics-audit:not-an-id
	}
}

// Registration is per user and persists so Notification Center can name
// and activate notifications after the sending process has exited.
var registeredNotificationIdentity = cacheNotificationRegistration(func() (notificationIdentity, error) {
	executable, err := os.Executable()
	if err != nil {
		return notificationIdentity{}, err
	}
	identity := identityForExecutable(executable)
	if goRunExecutable(executable, notificationGoCacheDirectory()) {
		if notificationStartDirectoryError != nil {
			return notificationIdentity{}, notificationStartDirectoryError
		}
		build, _ := debug.ReadBuildInfo()
		program := ""
		if build != nil {
			program = build.Path
		}
		identity = developmentNotificationIdentity(executable, notificationStartDirectory, program)
	}
	err = registerNotificationIdentity(identity)
	return identity, err
})

// Capture the launch directory before application code can change it. Separate
// working trees and programs must not share notification settings or activation.
var notificationStartDirectory, notificationStartDirectoryError = os.Getwd()

func developmentNotificationIdentity(executable, directory, program string) notificationIdentity {
	key := "go-run\x00" + strings.ToLower(filepath.Clean(directory)) + "\x00" + program + "\x00" + strings.ToLower(filepath.Base(executable))
	return notificationIdentityForKey(executable, key)
}

func notificationGoCacheDirectory() string {
	if directory := os.Getenv("GOCACHE"); directory != "" {
		return directory
	}
	directory, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(directory, "go-build")
}

func goRunExecutable(executable, cacheDirectory string) bool {
	directory := filepath.Dir(executable)
	// Recent Go versions also run cached executables from
	// GOCACHE/<hash prefix>/<64 hex digits>-d/<program>.exe.
	hash, cached := strings.CutSuffix(filepath.Base(directory), "-d")
	if cached && len(hash) == 64 && cacheDirectory != "" {
		_, err := hex.DecodeString(hash)
		parent := filepath.Dir(directory)
		if err == nil && strings.EqualFold(filepath.Base(parent), hash[:2]) &&
			strings.EqualFold(filepath.Dir(parent), filepath.Clean(cacheDirectory)) {
			return true
		}
	}
	// cmd/go places run executables in go-build<digits>/b<digits>/exe.
	// Match the layout, including custom GOTMPDIR roots, not just "Temp".
	if !strings.EqualFold(filepath.Base(directory), "exe") {
		return false
	}
	directory = filepath.Dir(directory)
	if !notificationNumberedDirectory(filepath.Base(directory), "b") {
		return false
	}
	return notificationNumberedDirectory(filepath.Base(filepath.Dir(directory)), "go-build")
}

func notificationNumberedDirectory(name, prefix string) bool {
	suffix, ok := strings.CutPrefix(strings.ToLower(name), prefix)
	if !ok || suffix == "" {
		return false
	}
	for _, char := range suffix {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func cacheNotificationRegistration(register func() (notificationIdentity, error)) func() (notificationIdentity, error) {
	var mu sync.Mutex
	var cached notificationIdentity
	var ready bool
	return func() (notificationIdentity, error) {
		mu.Lock()
		defer mu.Unlock()
		if ready {
			return cached, nil
		}
		identity, err := register()
		if err == nil {
			cached, ready = identity, true
		}
		return identity, err
	}
}

type notificationRegistration struct {
	write func(key, name, value string) error
}

// Registry-only registration needs no COM apartment or Start menu shortcut.
func registerNotificationIdentity(identity notificationIdentity) error {
	return installNotificationIdentity(identity, notificationRegistration{
		write: func(path, name, value string) error {
			key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
			if err != nil {
				return err
			}
			defer func() { _ = key.Close() }()
			return key.SetStringValue(name, value)
		},
	})
}

func installNotificationIdentity(identity notificationIdentity, registration notificationRegistration) error {
	// Protocol-only activation uses the documented stub-CLSID option.
	// The command deliberately takes no URL arguments: notification text
	// can never become executable arguments or a shell command.
	values := []struct{ key, name, value string }{
		{`Software\Classes\` + identity.protocol, "", "URL:" + identity.name},
		{`Software\Classes\` + identity.protocol, "URL Protocol", ""},
		{`Software\Classes\` + identity.protocol + `\shell\open\command`, "", windows.EscapeArg(identity.executable)},
		{`Software\Classes\AppUserModelId\` + identity.appID, "DisplayName", identity.name},
		{`Software\Classes\AppUserModelId\` + identity.appID, "CustomActivator", identity.activator.String()},
	}
	for _, entry := range values {
		if err := registration.write(entry.key, entry.name, entry.value); err != nil {
			return err
		}
	}
	return nil
}
