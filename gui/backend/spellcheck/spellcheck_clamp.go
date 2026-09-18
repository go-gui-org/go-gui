package spellcheck

import "math"

// clampRange fits a Suggest span to the text remainder, the same as
// the nativehost forwarder: a negative or zero length means the
// rest of the text from startByte, and so does a length past the
// end. The check uses subtraction because startByte+lenBytes
// overflows for a hostile lenBytes. Spans past 2 GB are refused:
// the lengths cross into C as int. It reports false when there is
// no remainder to suggest for. Shared by the platform backends so
// the clamp cannot drift apart per platform.
func clampRange(text string, startByte, lenBytes int) (int, int, bool) {
	if len(text) == 0 || len(text) > math.MaxInt32 {
		return 0, 0, false
	}
	if startByte < 0 {
		startByte = 0
	}
	if startByte >= len(text) {
		return 0, 0, false
	}
	if lenBytes <= 0 || lenBytes > len(text)-startByte {
		lenBytes = len(text) - startByte
	}
	return startByte, lenBytes, true
}
