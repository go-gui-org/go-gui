//go:build windows

package keyring

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	advapi32 = windows.NewLazySystemDLL("advapi32.dll")

	procCredWriteW  = advapi32.NewProc("CredWriteW")
	procCredReadW   = advapi32.NewProc("CredReadW")
	procCredDeleteW = advapi32.NewProc("CredDeleteW")
	procCredFree    = advapi32.NewProc("CredFree")
)

const (
	// CRED_TYPE_GENERIC: an app-defined credential.
	credTypeGeneric = 1
	// CRED_PERSIST_LOCAL_MACHINE: survives logoff; not roamed to other machines.
	credPersistLocalMachine = 2
	// ERROR_NOT_FOUND
	errorNotFound = windows.Errno(1168)
	// ERROR_NO_SUCH_LOGON_SESSION: the session has no credential store.
	errorNoSuchLogonSession = windows.Errno(1312)
)

// credential mirrors CREDENTIALW. Go lays out the fields as the C
// compiler does on 386, amd64 and arm64: FILETIME is two DWORDs, and
// the pointer after CredentialBlobSize is padded to its alignment.
type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// target is the credential's name in Credential Manager. The app ID
// and the key hold only letters, digits, '.', '-' and '_' (gui checks
// both), so "/" cannot be part of either and the name is unambiguous.
func target(service, key string) (*uint16, error) {
	return windows.UTF16PtrFromString(service + "/" + key)
}

// Load returns the generic credential for service and key.
func Load(service, key string) ([]byte, error) {
	t, err := target(service, key)
	if err != nil {
		return nil, err
	}
	var c *credential
	r, _, callErr := procCredReadW.Call(
		uintptr(unsafe.Pointer(t)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&c)))
	if r == 0 {
		return nil, credErr("CredReadW", callErr)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(c))) //nolint:errcheck // CredFree returns void
	n := int(c.CredentialBlobSize)
	if n == 0 || c.CredentialBlob == nil {
		return []byte{}, nil
	}
	blob := unsafe.Slice(c.CredentialBlob, n)
	out := make([]byte, n)
	copy(out, blob)
	// Clear the system's copy before CredFree hands it back to the heap.
	clear(blob)
	return out, nil
}

// Save adds or replaces the generic credential for service and key.
func Save(service, key string, value []byte) error {
	if len(value) > maxItemBytes {
		return fmt.Errorf("credential is %d bytes, over the %d-byte limit",
			len(value), maxItemBytes)
	}
	t, err := target(service, key)
	if err != nil {
		return err
	}
	user, err := windows.UTF16PtrFromString(key)
	if err != nil {
		return err
	}
	c := credential{
		Type:               credTypeGeneric,
		TargetName:         t,
		CredentialBlobSize: uint32(len(value)),
		CredentialBlob:     unsafe.SliceData(value),
		Persist:            credPersistLocalMachine,
		UserName:           user,
	}
	// CredWriteW replaces an existing credential with the same target
	// and type, so no separate update path is needed.
	r, _, callErr := procCredWriteW.Call(uintptr(unsafe.Pointer(&c)), 0)
	if r == 0 {
		return credErr("CredWriteW", callErr)
	}
	return nil
}

// Delete removes the generic credential for service and key. A missing
// credential is not an error.
func Delete(service, key string) error {
	t, err := target(service, key)
	if err != nil {
		return err
	}
	r, _, callErr := procCredDeleteW.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0)
	if r == 0 {
		err = credErr("CredDeleteW", callErr)
		if errors.Is(err, gui.ErrSecretNotFound) {
			return nil
		}
		return err
	}
	return nil
}

// credErr maps a Cred* failure to an error: ERROR_NOT_FOUND to
// gui.ErrSecretNotFound, a session with no credential store (a service
// account, some remote sessions) to ErrSecretsUnsupported.
func credErr(op string, callErr error) error {
	if errno, ok := errors.AsType[windows.Errno](callErr); ok {
		switch errno {
		case errorNotFound:
			return gui.ErrSecretNotFound
		case errorNoSuchLogonSession:
			return unsupported{reason: "no logon session credential store"}
		}
	}
	return fmt.Errorf("%s: %w", op, callErr)
}
