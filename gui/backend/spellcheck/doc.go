// Package spellcheck provides native spell checking via platform APIs.
//
// Check, Suggest and Learn must run on the frame (main) thread:
// NSSpellChecker on macOS and the Hunspell handle on Linux assume
// single-threaded use. In production Check runs there via the
// deferred command queue (spellCheckTrigger enqueues, FrameFn
// flushes); Linux serializes the handle with an internal mutex as
// defense in depth. Callers passing larger inputs rely on the
// platform forwarder to cap text at 64 KB first.
package spellcheck
