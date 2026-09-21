//go:build windows

package nativehost

import (
	"encoding/xml"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"golang.org/x/sys/windows"
)

var (
	notificationRuntime = windows.NewLazySystemDLL("combase.dll")
	toastManagerIID     = windows.GUID{Data1: 0x50ac103f, Data2: 0xd235, Data3: 0x4598, Data4: [8]byte{0xbb, 0xef, 0x98, 0xfe, 0x4d, 0x1a, 0x3a, 0xd4}}
	toastFactoryIID     = windows.GUID{Data1: 0x04124b20, Data2: 0x82c6, Data3: 0x4229, Data4: [8]byte{0xb1, 0x09, 0xfd, 0x9e, 0xd4, 0x66, 0x2b, 0x53}}
	toastXMLIID         = windows.GUID{Data1: 0xf7f3a506, Data2: 0x1e87, Data3: 0x42d6, Data4: [8]byte{0xbc, 0xfb, 0xb8, 0xc8, 0x09, 0xfa, 0x54, 0x94}}
	toastXMLIOIID       = windows.GUID{Data1: 0x6cd0e74e, Data2: 0xee65, Data3: 0x4489, Data4: [8]byte{0x9e, 0xbf, 0xca, 0x43, 0xe8, 0x7b, 0xa6, 0x37}}
)

// notificationCOM owns a native COM interface. All calls and releases
// occur in one initialized apartment, before RoUninitialize.
type notificationCOM struct{ vtable *[32]uintptr }

//go:uintptrescapes
func (object *notificationCOM) call(slot int, args ...uintptr) error {
	params := append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)
	hr, _, _ := syscall.SyscallN(object.vtable[slot], params...)
	if err := notificationHRESULT(hr); err != nil {
		return fmt.Errorf("COM method %d: %w", slot, err)
	}
	return nil
}

func (object *notificationCOM) release() {
	syscall.SyscallN(object.vtable[2], uintptr(unsafe.Pointer(object)))
}

func (object *notificationCOM) query(iid *windows.GUID) (*notificationCOM, error) {
	var result *notificationCOM
	err := object.call(0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&result)))
	return result, err
}

func notificationHRESULT(hr uintptr) error {
	if int32(hr) < 0 {
		return notificationCOMError(uint32(hr))
	}
	return nil
}

type notificationCOMError uint32

func (err notificationCOMError) Error() string {
	return fmt.Sprintf("Windows notification HRESULT 0x%08x", uint32(err))
}

func toastString(text string) (uintptr, error) {
	encoded, err := windows.UTF16FromString(text)
	if err != nil {
		return 0, err
	}
	var result uintptr
	hr, _, _ := notificationRuntime.NewProc("WindowsCreateString").Call(
		uintptr(unsafe.Pointer(&encoded[0])), uintptr(len(encoded)-1), uintptr(unsafe.Pointer(&result)))
	return result, notificationHRESULT(hr)
}

func freeToastString(value uintptr) {
	_, _, _ = notificationRuntime.NewProc("WindowsDeleteString").Call(value)
}

func toastFactory(class string, iid *windows.GUID) (*notificationCOM, error) {
	name, err := toastString(class)
	if err != nil {
		return nil, err
	}
	defer freeToastString(name)
	var result *notificationCOM
	hr, _, _ := notificationRuntime.NewProc("RoGetActivationFactory").Call(name,
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&result)))
	return result, notificationHRESULT(hr)
}

func toastDocument(source string) (*notificationCOM, error) {
	class, err := toastString("Windows.Data.Xml.Dom.XmlDocument")
	if err != nil {
		return nil, err
	}
	defer freeToastString(class)
	var instance *notificationCOM
	hr, _, _ := notificationRuntime.NewProc("RoActivateInstance").Call(class, uintptr(unsafe.Pointer(&instance)))
	if err = notificationHRESULT(hr); err != nil {
		return nil, err
	}
	defer instance.release()
	loader, err := instance.query(&toastXMLIOIID)
	if err != nil {
		return nil, err
	}
	defer loader.release()
	xmlText, err := toastString(source)
	if err != nil {
		return nil, err
	}
	defer freeToastString(xmlText)
	if err = loader.call(6, xmlText); err != nil {
		return nil, err
	} // IXmlDocumentIO.LoadXml
	return instance.query(&toastXMLIID)
}

func notificationToastXML(title, body, protocol string) string {
	escape := func(text string) string {
		var escaped strings.Builder
		_ = xml.EscapeText(&escaped, []byte(text))
		return escaped.String()
	}
	return `<toast activationType="protocol" launch="` + escape(protocol) + `:"><visual><binding template="ToastGeneric"><text>` +
		escape(title) + `</text><text>` + escape(body) + `</text></binding></visual></toast>`
}

func sendWindowsToast(title, body string) gui.NativeNotificationResult {
	// NativeNotification already calls from a goroutine. Lock it while
	// owning WinRT interfaces; no helper process or tray icon is involved.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := notificationRuntime.NewProc("RoInitialize").Call(1) // RO_INIT_MULTITHREADED
	if err := notificationHRESULT(hr); err != nil {
		return toastError(err)
	}
	defer notificationRuntime.NewProc("RoUninitialize").Call()
	identity, err := registeredNotificationIdentity()
	if err != nil {
		return toastError(err)
	}
	manager, err := toastFactory("Windows.UI.Notifications.ToastNotificationManager", &toastManagerIID)
	if err != nil {
		return toastError(err)
	}
	defer manager.release()
	appID, err := toastString(identity.appID)
	if err != nil {
		return toastError(err)
	}
	defer freeToastString(appID)
	var notifier *notificationCOM
	if err = manager.call(7, appID, uintptr(unsafe.Pointer(&notifier))); err != nil {
		return toastError(err)
	}
	defer notifier.release()
	document, err := toastDocument(notificationToastXML(title, body, identity.protocol))
	if err != nil {
		return toastError(err)
	}
	defer document.release()
	factory, err := toastFactory("Windows.UI.Notifications.ToastNotification", &toastFactoryIID)
	if err != nil {
		return toastError(err)
	}
	defer factory.release()
	var toast *notificationCOM
	if err = factory.call(6, uintptr(unsafe.Pointer(document)), uintptr(unsafe.Pointer(&toast))); err != nil {
		return toastError(err)
	}
	defer toast.release()
	if err = notifier.call(6, uintptr(unsafe.Pointer(toast))); err != nil {
		return toastError(err)
	}
	var setting int32
	// Query settings after Show: querying a new identity first can prevent
	// its first toast from appearing on Windows 10. If settings cannot be
	// read (including ERROR_NOT_FOUND for a new identity), Show's successful
	// acceptance is still authoritative; only a known opt-out is denied.
	if err = notifier.call(8, uintptr(unsafe.Pointer(&setting))); err == nil && setting != 0 {
		return gui.NativeNotificationResult{Status: gui.NotificationDenied, ErrorCode: "notifications_disabled", ErrorMessage: "Windows notifications are disabled for this application"}
	}
	return gui.NativeNotificationResult{Status: gui.NotificationOK}
}

func toastError(err error) gui.NativeNotificationResult {
	return gui.NativeNotificationResult{Status: gui.NotificationError, ErrorCode: "notification_failed", ErrorMessage: err.Error()}
}
