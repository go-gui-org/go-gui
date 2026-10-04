//go:build !(darwin && cgo) && !(linux && !android) && !windows

package keyring

// Load, Save and Delete report that this platform has no credential
// store go-gui can use. No backend for these platforms embeds Platform
// today; the file keeps the package building everywhere.

// Load reports ErrSecretsUnsupported.
func Load(_, _ string) ([]byte, error) { return nil, unsupported{reason: "platform"} }

// Save reports ErrSecretsUnsupported.
func Save(_, _ string, _ []byte) error { return unsupported{reason: "platform"} }

// Delete reports ErrSecretsUnsupported.
func Delete(_, _ string) error { return unsupported{reason: "platform"} }
