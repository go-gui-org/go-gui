//go:build darwin && !ios

// Package spellcheck provides native spell checking via NSSpellChecker.
package spellcheck

import (
	"math"
	"strings"
	"sync"
	"unicode/utf8"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// NSSpellChecker is called through purego, so spell check works with
// cgo off (#1312). A check runs when the text changes, not per
// frame, so the objc.Send form, which allocates per call, is fine.

// nsRange is NSRange: two NSUIntegers. At 16 bytes it comes back in
// two integer registers, so objc.Send returns it by value.
type nsRange struct {
	Location uint
	Length   uint
}

// nsNotFound is NSNotFound (NSIntegerMax), the location of an empty
// result.
const nsNotFound = math.MaxInt

// nsUTF8StringEncoding is NSUTF8StringEncoding.
const nsUTF8StringEncoding = 4

var sc struct {
	once sync.Once

	clsString, clsChecker objc.ID

	selAlloc, selInitBytes, selRelease, selLength objc.SEL
	selShared, selCheck, selGuesses, selLearn     objc.SEL
	selCount, selObjectAt                         objc.SEL
}

func load() {
	sc.once.Do(func() {
		darwinffi.Open(darwinffi.PathAppKit) // NSSpellChecker
		sc.clsString = darwinffi.Class("NSString")
		sc.clsChecker = darwinffi.Class("NSSpellChecker")
		sc.selAlloc = darwinffi.Sel("alloc")
		sc.selInitBytes = darwinffi.Sel("initWithBytes:length:encoding:")
		sc.selRelease = darwinffi.Sel("release")
		sc.selLength = darwinffi.Sel("length")
		sc.selShared = darwinffi.Sel("sharedSpellChecker")
		sc.selCheck = darwinffi.Sel(
			"checkSpellingOfString:startingAt:language:wrap:inSpellDocumentWithTag:wordCount:")
		sc.selGuesses = darwinffi.Sel(
			"guessesForWordRange:inString:language:inSpellDocumentWithTag:")
		sc.selLearn = darwinffi.Sel("learnWord:")
		sc.selCount = darwinffi.Sel("count")
		sc.selObjectAt = darwinffi.Sel("objectAtIndex:")
	})
}

// newString returns a new NSString that holds the bytes of s, or 0 when
// s is not valid UTF-8. The caller releases it. The length is explicit,
// so an embedded NUL does not end the string (darwinffi.NSString stops
// at the first NUL). The NSString copies the bytes before it returns.
func newString(s string) objc.ID {
	if s == "" {
		return 0
	}
	str := sc.clsString.Send(sc.selAlloc)
	return str.Send(sc.selInitBytes, unsafe.Pointer(unsafe.StringData(s)),
		uint(len(s)), uint(nsUTF8StringEncoding))
}

// droppedBOMs returns the byte length of the leading byte order marks
// (U+FEFF) that NSString dropped when it decoded text. strLen is the
// NSString length. NSString offsets start after the marks it dropped,
// so Check and Suggest add this length back to the byte offsets. The
// count is measured, not assumed: AppKit drops more than one leading
// mark. The cgo form did not correct for this, and put every range
// after a mark 3 bytes early.
func droppedBOMs(text string, strLen uint) int {
	const bom = "\uFEFF" // 3 bytes in UTF-8, 1 UTF-16 unit
	missing := utf16Len(text) - strLen
	n := 0
	for missing > 0 && strings.HasPrefix(text[n:], bom) {
		n += len(bom)
		missing--
	}
	return n
}

// utf16Len returns the number of UTF-16 code units in s, which is the
// NSString length of s. s must be valid UTF-8.
func utf16Len(s string) uint {
	var n uint
	for _, r := range s {
		n++
		if r >= 0x10000 { // a surrogate pair
			n++
		}
	}
	return n
}

// byteCursor maps UTF-16 offsets in an NSString to byte offsets in the
// Go string it holds. The offsets must not go back, so one walk over
// the text serves every range of a check.
type byteCursor struct {
	text string
	b    int  // byte offset
	u    uint // UTF-16 offset of b
}

// at returns the byte offset of UTF-16 offset u16.
func (c *byteCursor) at(u16 uint) int {
	for c.u < u16 && c.b < len(c.text) {
		r, size := utf8.DecodeRuneInString(c.text[c.b:])
		c.b += size
		c.u++
		if r >= 0x10000 {
			c.u++
		}
	}
	return c.b
}

// Check returns byte ranges of misspelled words in text.
func Check(text string) []gui.SpellRange {
	if len(text) == 0 || len(text) > math.MaxInt32 {
		return nil
	}
	load()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	str := newString(text)
	if str == 0 {
		return nil
	}
	defer str.Send(sc.selRelease)
	strLen := objc.Send[uint](str, sc.selLength)
	bom := droppedBOMs(text, strLen)
	body := text[bom:]
	checker := sc.clsChecker.Send(sc.selShared)

	var ranges []gui.SpellRange
	cur := byteCursor{text: body}
	var offset uint
	for offset < strLen {
		// language nil, wrap NO, document tag 0, wordCount NULL.
		m := objc.Send[nsRange](checker, sc.selCheck, str, offset,
			objc.ID(0), false, 0, uintptr(0))
		if m.Location == nsNotFound || m.Length == 0 {
			break
		}
		start := cur.at(m.Location)
		end := cur.at(m.Location + m.Length)
		ranges = append(ranges, gui.SpellRange{StartByte: bom + start, LenBytes: end - start})
		offset = m.Location + m.Length
	}
	return ranges
}

// Suggest returns spelling suggestions for a misspelled range.
// Out-of-range spans clamp to the text remainder; see clampRange.
func Suggest(text string, startByte, lenBytes int) []string {
	startByte, lenBytes, ok := clampRange(text, startByte, lenBytes)
	if !ok {
		return nil
	}
	// A span that cuts a character in two names no word. The cgo form
	// gave nil here too: NSString refused the cut bytes.
	if !utf8.ValidString(text) {
		return nil
	}
	load()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	str := newString(text)
	if str == 0 {
		return nil
	}
	defer str.Send(sc.selRelease)
	// A span inside the dropped marks names no word.
	bom := droppedBOMs(text, objc.Send[uint](str, sc.selLength))
	if startByte < bom {
		return nil
	}
	prefix := text[bom:startByte]
	word := text[startByte : startByte+lenBytes]
	if !utf8.ValidString(prefix) || !utf8.ValidString(word) {
		return nil
	}
	r := nsRange{Location: utf16Len(prefix), Length: utf16Len(word)}
	checker := sc.clsChecker.Send(sc.selShared)
	guesses := checker.Send(sc.selGuesses, r, str, objc.ID(0), 0)
	if guesses == 0 {
		return nil
	}
	n := objc.Send[uint](guesses, sc.selCount)
	if n == 0 {
		return nil
	}
	out := make([]string, 0, n)
	for i := range n {
		// NSString is toll-free bridged to CFString.
		g := guesses.Send(sc.selObjectAt, i)
		out = append(out, darwinffi.GoString(uintptr(g)))
	}
	return out
}

// Learn adds a word to the user's dictionary. Empty words are
// ignored: AppKit would otherwise learn the empty string.
func Learn(word string) {
	if word == "" || len(word) > math.MaxInt32 {
		return
	}
	load()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	str := newString(word)
	if str == 0 {
		return
	}
	defer str.Send(sc.selRelease)
	sc.clsChecker.Send(sc.selShared).Send(sc.selLearn, str)
}
