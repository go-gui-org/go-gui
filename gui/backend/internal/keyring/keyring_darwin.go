//go:build darwin && cgo

package keyring

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <TargetConditionals.h>
#include <stdlib.h>
#include <string.h>

// gg_query builds the query that names one generic password item:
// class, service and account. The caller releases it.
static CFMutableDictionaryRef gg_query(const char *service, const char *account) {
	CFMutableDictionaryRef q = CFDictionaryCreateMutable(NULL, 0,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFStringRef s = CFStringCreateWithCString(NULL, service, kCFStringEncodingUTF8);
	CFStringRef a = CFStringCreateWithCString(NULL, account, kCFStringEncodingUTF8);
	CFDictionarySetValue(q, kSecClass, kSecClassGenericPassword);
	CFDictionarySetValue(q, kSecAttrService, s);
	CFDictionarySetValue(q, kSecAttrAccount, a);
	CFRelease(s);
	CFRelease(a);
	return q;
}

// gg_loaded is gg_secret_load's result. A struct return, not out
// parameters, so the Go side passes no Go pointers into C.
typedef struct {
	void *data;
	long len;
	OSStatus status;
} gg_loaded;

// gg_secret_load copies the item's data into a malloc'd buffer. The
// caller clears and frees it.
static gg_loaded gg_secret_load(const char *service, const char *account) {
	gg_loaded r = {NULL, 0, errSecSuccess};
	CFMutableDictionaryRef q = gg_query(service, account);
	CFDictionarySetValue(q, kSecReturnData, kCFBooleanTrue);
	CFDictionarySetValue(q, kSecMatchLimit, kSecMatchLimitOne);
	CFTypeRef res = NULL;
	r.status = SecItemCopyMatching(q, &res);
	CFRelease(q);
	if (r.status != errSecSuccess) {
		return r;
	}
	if (res == NULL || CFGetTypeID(res) != CFDataGetTypeID()) {
		if (res != NULL) CFRelease(res);
		r.status = errSecInternalError;
		return r;
	}
	CFDataRef d = (CFDataRef)res;
	long n = (long)CFDataGetLength(d);
	void *buf = malloc(n > 0 ? (size_t)n : 1);
	if (buf == NULL) {
		CFRelease(res);
		r.status = errSecAllocate;
		return r;
	}
	memcpy(buf, CFDataGetBytePtr(d), (size_t)n);
	CFRelease(res);
	r.data = buf;
	r.len = n;
	return r;
}

// gg_secret_save updates the item if it exists, else adds it.
static OSStatus gg_secret_save(const char *service, const char *account,
		const void *data, long n) {
	CFDataRef d = CFDataCreate(NULL, (const UInt8 *)data, (CFIndex)n);
	CFMutableDictionaryRef q = gg_query(service, account);
	CFMutableDictionaryRef upd = CFDictionaryCreateMutable(NULL, 0,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFDictionarySetValue(upd, kSecValueData, d);
	OSStatus st = SecItemUpdate(q, upd);
	CFRelease(upd);
	if (st == errSecItemNotFound) {
		CFDictionarySetValue(q, kSecValueData, d);
#if TARGET_OS_IPHONE
		// Readable by a background task after the first unlock, and
		// never moved to another device by a backup restore.
		CFDictionarySetValue(q, kSecAttrAccessible,
			kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly);
#endif
		st = SecItemAdd(q, NULL);
	}
	CFRelease(q);
	CFRelease(d);
	return st;
}

static OSStatus gg_secret_delete(const char *service, const char *account) {
	CFMutableDictionaryRef q = gg_query(service, account);
	OSStatus st = SecItemDelete(q);
	CFRelease(q);
	return st;
}

// gg_status_message writes the Security framework's text for st into
// buf as UTF-8. It writes an empty string when there is none.
static void gg_status_message(OSStatus st, char *buf, long n) {
	buf[0] = 0;
	CFStringRef s = SecCopyErrorMessageString(st, NULL);
	if (s == NULL) return;
	CFStringGetCString(s, buf, (CFIndex)n, kCFStringEncodingUTF8);
	CFRelease(s);
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// Load returns the Keychain item for service and key.
func Load(service, key string) ([]byte, error) {
	cs, ck := C.CString(service), C.CString(key)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(ck))
	r := C.gg_secret_load(cs, ck)
	if r.status != 0 { // errSecSuccess
		return nil, statusErr(r.status)
	}
	// An item another program wrote can be any size. Refuse one over
	// gui's 2560-byte cap before GoBytes copies it, so a huge item costs
	// no Go allocation, and its length cannot overflow C.int.
	if r.len > maxItemBytes {
		clear(unsafe.Slice((*byte)(r.data), int(r.len)))
		C.free(r.data)
		return nil, fmt.Errorf("keychain: item is %d bytes, over the %d-byte limit",
			int64(r.len), maxItemBytes)
	}
	out := C.GoBytes(r.data, C.int(r.len))
	// Clear the C copy before it goes back to the allocator, so the
	// secret does not stay in freed memory.
	clear(unsafe.Slice((*byte)(r.data), int(r.len)))
	C.free(r.data)
	return out, nil
}

// Save adds or replaces the Keychain item for service and key.
func Save(service, key string, value []byte) error {
	cs, ck := C.CString(service), C.CString(key)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(ck))
	// value holds no Go pointers, so cgo allows passing it for the
	// length of the call. CFDataCreate copies it; no C-heap copy is
	// left to clear.
	st := C.gg_secret_save(cs, ck, unsafe.Pointer(unsafe.SliceData(value)), C.long(len(value)))
	if st != 0 { // errSecSuccess
		return statusErr(st)
	}
	return nil
}

// Delete removes the Keychain item for service and key. A missing item
// is not an error.
func Delete(service, key string) error {
	cs, ck := C.CString(service), C.CString(key)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(ck))
	st := C.gg_secret_delete(cs, ck)
	if st != 0 && st != C.errSecItemNotFound {
		return statusErr(st)
	}
	return nil
}

// statusErr maps a Security framework status to an error: not found to
// gui.ErrSecretNotFound, no usable keychain to ErrSecretsUnsupported,
// and the rest to an error with the framework's own message.
func statusErr(st C.OSStatus) error {
	switch st {
	case C.errSecItemNotFound:
		return gui.ErrSecretNotFound
	case C.errSecNotAvailable, C.errSecNoSuchKeychain:
		return unsupported{reason: fmt.Sprintf("keychain not available (OSStatus %d)", int(st))}
	}
	var msg [256]C.char
	C.gg_status_message(st, &msg[0], C.long(len(msg)))
	if text := C.GoString(&msg[0]); text != "" {
		return fmt.Errorf("keychain: %s (OSStatus %d)", text, int(st))
	}
	return fmt.Errorf("keychain: OSStatus %d", int(st))
}
