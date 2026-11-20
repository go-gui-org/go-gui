//go:build !darwin || (ios && !cgo)

package gui

// localeDetect returns the BCP 47 locale ID from environment
// variables.
func localeDetect() string { return localeFromEnv() }
