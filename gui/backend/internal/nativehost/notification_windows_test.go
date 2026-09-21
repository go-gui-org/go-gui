//go:build windows

package nativehost

import (
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func TestWindowsToastIntegration(t *testing.T) {
	if os.Getenv("GOGUI_NOTIFICATION_TEST") != "1" {
		t.Skip("desktop validation only")
	}
	if windows.RtlGetVersion().MajorVersion < 10 {
		t.Skip("WinRT notification identity requires Windows 10 or later")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := notificationRuntime.NewProc("RoInitialize").Call(1)
	if err := notificationHRESULT(hr); err != nil {
		t.Fatal(err)
	}
	defer notificationRuntime.NewProc("RoUninitialize").Call()
	identity, err := registeredNotificationIdentity()
	if err != nil {
		t.Fatal(err)
	}
	defer removeTestNotificationIdentity(t, identity)
	managerIID := windows.GUID{Data1: 0x7ab93c52, Data2: 0x0e48, Data3: 0x4750, Data4: [8]byte{0xba, 0x9d, 0x1a, 0x41, 0x13, 0x98, 0x18, 0x47}}
	manager, err := toastFactory("Windows.UI.Notifications.ToastNotificationManager", &managerIID)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.release()
	var history *notificationCOM
	if err = manager.call(6, uintptr(unsafe.Pointer(&history))); err != nil {
		t.Fatal(err)
	}
	defer history.release()
	appID, err := toastString(identity.appID)
	if err != nil {
		t.Fatal(err)
	}
	defer freeToastString(appID)
	defer func() {
		// Clear only this test executable's notifications, never the user's.
		if clearErr := history.call(12, appID); clearErr != nil {
			t.Error(clearErr)
		}
	}()
	historyIID := windows.GUID{Data1: 0x3bc3d253, Data2: 0x2f31, Data3: 0x4092, Data4: [8]byte{0x91, 0x29, 0x8a, 0xd5, 0xab, 0xf0, 0x67, 0xda}}
	reader, err := history.query(&historyIID)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.release()
	if err = history.call(12, appID); err != nil {
		t.Fatal(err)
	}
	result := sendWindowsToast("go-gui native toast test", "Application identity and Notification Center test")
	if result.Status != gui.NotificationOK {
		t.Fatalf("toast: %+v", result)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		var items *notificationCOM
		if err = reader.call(7, appID, uintptr(unsafe.Pointer(&items))); err != nil {
			t.Fatal(err)
		}
		var count uint32
		err = items.call(7, uintptr(unsafe.Pointer(&count)))
		items.release()
		if err != nil {
			t.Fatal(err)
		}
		if count > 0 {
			t.Logf("Notification Center retained %d notification(s) for %s", count, identity.name)
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("accepted toast did not appear in this application's history")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func removeTestNotificationIdentity(t *testing.T, identity notificationIdentity) {
	t.Helper()
	// This executable owns these exact keys; leave shared
	// parent folders and other applications' registrations untouched.
	protocol := `Software\Classes\` + identity.protocol
	for _, key := range []string{protocol + `\shell\open\command`, protocol + `\shell\open`, protocol + `\shell`, protocol,
		`Software\Classes\AppUserModelId\` + identity.appID} {
		if err := registry.DeleteKey(registry.CURRENT_USER, key); err != nil {
			t.Error(err)
		}
	}
}
