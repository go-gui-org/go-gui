//go:build (darwin && cgo) || (linux && !android) || windows

package keyring

// maxItemBytes is gui's cap on one secret (maxSecretBytes). It is the
// Windows Credential Manager limit, CRED_MAX_CREDENTIAL_BLOB_SIZE
// (5*512 bytes). gui checks it again on every Load and Save. The build
// tag is the complement of keyring_other.go, where no store uses it.
const maxItemBytes = 5 * 512
