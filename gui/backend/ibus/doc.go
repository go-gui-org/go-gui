// Package ibus is a minimal IBus input-method client spoken over D-Bus.
//
// It exists so the X11 backend can support IME (CJK and anything else
// driven by an input method) without XIM, which has no pure-Go
// implementation. Both ibus and fcitx5 expose this interface, the
// latter directly and through the IBus portal, so one client covers
// nearly every Linux IME user.
//
// Only the input-context half of the IBus API is implemented: enough to
// forward key events, receive preedit and commit, and tell the panel
// where the caret is. Candidate windows are left to the IME panel.
package ibus
