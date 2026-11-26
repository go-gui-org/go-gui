//go:build darwin && (!ios || cgo)

package keyring

import (
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// Security framework status codes (SecBase.h). OSStatus is an int32.
const (
	errSecSuccess        = 0
	errSecAllocate       = -108
	errSecNotAvailable   = -25291
	errSecNoSuchKeychain = -25294
	errSecItemNotFound   = -25300
	errSecInternalError  = -26276
)

// sec holds the Security framework functions and the kSec* keys. It is
// bound on first use through purego, so the Keychain works with cgo off
// (#1307). A secret call is not on a per-frame path, so the typed
// RegisterFunc form, which allocates per call, is fine.
var sec struct {
	once sync.Once

	copyMatching func(query uintptr, result *uintptr) int32
	add          func(attrs uintptr, result uintptr) int32
	update       func(query, attrs uintptr) int32
	del          func(query uintptr) int32
	errorMessage func(status int32, reserved uintptr) uintptr

	class, classGenericPassword, attrService, attrAccount uintptr
	returnData, matchLimit, matchLimitOne, valueData      uintptr

	// attrAccessible and accessibleAfterFirstUnlockThisDeviceOnly
	// are set on a new item on iOS only (see Save).
	attrAccessible, accessibleAfterFirstUnlockThisDeviceOnly uintptr
}

func loadSec() {
	sec.once.Do(func() {
		lib := darwinffi.Open(darwinffi.PathSecurity)
		purego.RegisterLibFunc(&sec.copyMatching, lib, "SecItemCopyMatching")
		purego.RegisterLibFunc(&sec.add, lib, "SecItemAdd")
		purego.RegisterLibFunc(&sec.update, lib, "SecItemUpdate")
		purego.RegisterLibFunc(&sec.del, lib, "SecItemDelete")
		purego.RegisterLibFunc(&sec.errorMessage, lib, "SecCopyErrorMessageString")
		sec.class = darwinffi.Data(lib, "kSecClass")
		sec.classGenericPassword = darwinffi.Data(lib, "kSecClassGenericPassword")
		sec.attrService = darwinffi.Data(lib, "kSecAttrService")
		sec.attrAccount = darwinffi.Data(lib, "kSecAttrAccount")
		sec.returnData = darwinffi.Data(lib, "kSecReturnData")
		sec.matchLimit = darwinffi.Data(lib, "kSecMatchLimit")
		sec.matchLimitOne = darwinffi.Data(lib, "kSecMatchLimitOne")
		sec.valueData = darwinffi.Data(lib, "kSecValueData")
		sec.attrAccessible = darwinffi.Data(lib, "kSecAttrAccessible")
		sec.accessibleAfterFirstUnlockThisDeviceOnly = darwinffi.Data(lib,
			"kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly")
	})
}

// errInvalidName is the error for a service or key that is not valid
// UTF-8. The Keychain names items with CFStrings, which cannot hold one.
var errInvalidName = errors.New("keychain: service and key must be valid UTF-8")

// query builds the dictionary that names one generic password item:
// class, service and account. The caller releases it.
//
// CFStringCreateWithCString returns NULL for bytes that are not valid
// UTF-8, and a NULL value in the dictionary crashes the process: the
// kCFType callbacks call CFRetain on it. So a bad name is an error
// before any dictionary is built. gui does not check the names.
func query(service, account string) (uintptr, error) {
	loadSec()
	s := darwinffi.CFString(service)
	a := darwinffi.CFString(account)
	if s == 0 || a == 0 {
		darwinffi.CFRelease(s) // CFRelease of 0 is a no-op
		darwinffi.CFRelease(a)
		return 0, errInvalidName
	}
	q := darwinffi.CFMutableDict()
	darwinffi.CFDictSet(q, sec.class, sec.classGenericPassword)
	darwinffi.CFDictSet(q, sec.attrService, s)
	darwinffi.CFDictSet(q, sec.attrAccount, a)
	// The dictionary retains both strings.
	darwinffi.CFRelease(s)
	darwinffi.CFRelease(a)
	return q, nil
}

// Load returns the Keychain item for service and key.
func Load(service, key string) ([]byte, error) {
	q, err := query(service, key)
	if err != nil {
		return nil, err
	}
	darwinffi.CFDictSet(q, sec.returnData, darwinffi.CFTrue())
	darwinffi.CFDictSet(q, sec.matchLimit, sec.matchLimitOne)
	// res is a heap variable: its address goes to C as an out-parameter.
	// RegisterFunc keeps a Go pointer argument alive for the call.
	res := new(uintptr)
	st := sec.copyMatching(q, res)
	darwinffi.CFRelease(q)
	if st != errSecSuccess {
		return nil, statusErr(st)
	}
	d := *res
	defer darwinffi.CFRelease(d)
	if d == 0 || darwinffi.CFTypeID(d) != darwinffi.CFDataTypeID() {
		return nil, statusErr(errSecInternalError)
	}
	// An item another program wrote can be any size. Refuse one over
	// gui's 2560-byte cap before it is copied, so a huge item costs no
	// Go allocation.
	n := darwinffi.CFDataLen(d)
	if n > maxItemBytes {
		return nil, fmt.Errorf("keychain: item is %d bytes, over the %d-byte limit",
			int64(n), maxItemBytes)
	}
	// The cgo form copied the bytes to a malloc buffer first and
	// cleared that copy. Here they go straight into Go memory, so there
	// is no second C copy to clear.
	return darwinffi.CFDataBytes(d), nil
}

// Save adds or replaces the Keychain item for service and key.
func Save(service, key string, value []byte) error {
	// CFDataCreate copies value, so no C copy of the secret is left to
	// clear.
	d := darwinffi.CFData(value)
	if d == 0 {
		return statusErr(errSecAllocate)
	}
	defer darwinffi.CFRelease(d)
	q, err := query(service, key)
	if err != nil {
		return err
	}
	defer darwinffi.CFRelease(q)
	upd := darwinffi.CFMutableDict()
	darwinffi.CFDictSet(upd, sec.valueData, d)
	st := sec.update(q, upd)
	darwinffi.CFRelease(upd)
	if st == errSecItemNotFound {
		darwinffi.CFDictSet(q, sec.valueData, d)
		if runtime.GOOS == "ios" {
			// Readable by a background task after the first unlock,
			// and never moved to another device by a backup restore.
			// The cgo iOS form set this; macOS keeps the default.
			darwinffi.CFDictSet(q, sec.attrAccessible,
				sec.accessibleAfterFirstUnlockThisDeviceOnly)
		}
		st = sec.add(q, 0)
	}
	if st != errSecSuccess {
		return statusErr(st)
	}
	return nil
}

// Delete removes the Keychain item for service and key. A missing item
// is not an error.
func Delete(service, key string) error {
	q, err := query(service, key)
	if err != nil {
		return err
	}
	st := sec.del(q)
	darwinffi.CFRelease(q)
	if st != errSecSuccess && st != errSecItemNotFound {
		return statusErr(st)
	}
	return nil
}

// statusErr maps a Security framework status to an error: not found to
// gui.ErrSecretNotFound, no usable keychain to ErrSecretsUnsupported,
// and the rest to an error with the framework's own message.
func statusErr(st int32) error {
	switch st {
	case errSecItemNotFound:
		return gui.ErrSecretNotFound
	case errSecNotAvailable, errSecNoSuchKeychain:
		return unsupported{reason: fmt.Sprintf("keychain not available (OSStatus %d)", int(st))}
	}
	loadSec()
	s := sec.errorMessage(st, 0)
	text := darwinffi.GoString(s)
	darwinffi.CFRelease(s)
	if text != "" {
		return fmt.Errorf("keychain: %s (OSStatus %d)", text, int(st))
	}
	return fmt.Errorf("keychain: OSStatus %d", int(st))
}
