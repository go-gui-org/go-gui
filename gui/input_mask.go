package gui

import (
	"errors"
	"sync"
	"unicode"
	"unicode/utf8"
)

// InputMaskPreset defines common mask patterns.
// exportaudit:keep — reachable from an exported signature
type InputMaskPreset uint8

// InputMaskPreset constants.
// exportaudit:keep — caller-facing mask presets
const (
	// MaskNone selects no mask: the field edits plain text. It is
	// the zero value, so an unset MaskPreset already means this;
	// the name exists so call sites can say it explicitly.
	// exportaudit:keep — caller-facing mask presets
	MaskNone InputMaskPreset = iota
	MaskPhoneUS
	MaskCreditCard16
	MaskCreditCardAmex
	MaskExpiryMMYY
	MaskCVC
)

// MaskTokenDef defines one token symbol in a mask pattern.
// exportaudit:keep — reachable from an exported signature
type MaskTokenDef struct {
	// Matcher reports whether r may occupy this slot. It must be
	// non-nil: compileInputMask rejects a token without one, because
	// a nil matcher compiles into a slot no keystroke can ever fill.
	// exportaudit:keep — caller-facing mask API
	Matcher   func(rune) bool
	Transform func(rune) rune
	Symbol    rune
}

// MaskEditResult stores the output of a mask edit operation.
// exportaudit:keep — reachable from an exported signature
type MaskEditResult struct {
	Text      string
	CursorPos int
	Changed   bool
}

type maskEntryKind uint8

const (
	maskLiteral maskEntryKind = iota
	maskSlot
)

type compiledMaskEntry struct {
	matcher   func(rune) bool
	transform func(rune) rune
	literal   rune
	symbol    rune
	kind      maskEntryKind
}

// CompiledInputMask stores parsed mask entries and lookup indexes.
// exportaudit:keep — reachable from an exported signature
type CompiledInputMask struct {
	pattern string
	entries []compiledMaskEntry
	// lastSlot is the entry index of the final slot, or -1 when
	// the pattern holds no slots. formatRaw emits a literal only
	// when a slot follows it (i < lastSlot); precomputed here
	// because entries are read-only after compile and the per-call
	// suffix scan would allocate on every keystroke.
	lastSlot         int
	slotEntryIndexes []int
}

func identityRune(r rune) rune { return r }
func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }
func isMaskLetter(r rune) bool { return unicode.IsLetter(r) }
func isMaskAlnum(r rune) bool  { return unicode.IsLetter(r) || unicode.IsNumber(r) }

// InputMaskDefaultTokens returns built-in mask tokens.
func inputMaskDefaultTokens() []MaskTokenDef {
	return []MaskTokenDef{
		{Symbol: '9', Matcher: isASCIIDigit},
		{Symbol: 'a', Matcher: isMaskLetter},
		{Symbol: '*', Matcher: isMaskAlnum},
	}
}

// defaultMaskTokens is the shared default table compileInputMask
// reads from. Package-level because the mask compiles on every Input
// generation — once per field per frame — and rebuilding the table
// each time is pure garbage. Read-only after init; never mutated.
var defaultMaskTokens = inputMaskDefaultTokens()

// compiledMaskCache shares compiled masks across generations. A mask
// is read-only after compile, so one instance serves every field
// using the pattern. Keyed by pattern only when no custom tokens are
// in play: custom tables make the key unbounded, and those fields
// compile fresh. Patterns are static app strings, so the map settles
// at a handful of entries and never grows per frame. Capped anyway:
// a dynamic pattern source must degrade to recompiling, not to
// unbounded growth.
const compiledMaskCacheMax = 256

var compiledMaskCache = struct {
	sync.RWMutex
	m map[string]*CompiledInputMask
}{m: make(map[string]*CompiledInputMask)}

// cachedCompiledMask returns the shared instance for pattern, or nil
// when no custom tokens apply and the pattern is not cached yet.
func cachedCompiledMask(pattern string) *CompiledInputMask {
	compiledMaskCache.RLock()
	c := compiledMaskCache.m[pattern]
	compiledMaskCache.RUnlock()
	return c
}

// storeCompiledMaskCache records a compiled mask for reuse. On
// overflow the cache is emptied and refilled from the newcomer rather
// than the newcomer being refused: compiledMask() runs from the Input
// factory every frame, so a pattern that can never be admitted
// recompiles every frame forever — the exact per-frame garbage this
// cache exists to remove. Dropping the whole set costs one recompile
// per live pattern and cannot pin a live field behind 256 dead ones.
func storeCompiledMaskCache(pattern string, c *CompiledInputMask) {
	compiledMaskCache.Lock()
	defer compiledMaskCache.Unlock()
	if _, ok := compiledMaskCache.m[pattern]; ok {
		return
	}
	if len(compiledMaskCache.m) >= compiledMaskCacheMax {
		clear(compiledMaskCache.m)
	}
	compiledMaskCache.m[pattern] = c
}

// InputMaskFromPreset returns the mask pattern for a preset.
func inputMaskFromPreset(preset InputMaskPreset) string {
	switch preset {
	case MaskPhoneUS:
		return "(999) 999-9999"
	case MaskCreditCard16:
		return "9999 9999 9999 9999"
	case MaskCreditCardAmex:
		return "9999 999999 99999"
	case MaskExpiryMMYY:
		return "99/99"
	case MaskCVC:
		return "999"
	default:
		return ""
	}
}

// CompileInputMask parses a mask pattern and resolves token
// definitions.
func compileInputMask(mask string, custom []MaskTokenDef) (CompiledInputMask, error) {
	if len(mask) == 0 {
		return CompiledInputMask{}, errors.New("mask pattern is empty")
	}

	tokenMap := make(map[rune]MaskTokenDef)
	for _, def := range defaultMaskTokens {
		tokenMap[def.Symbol] = def
	}
	for _, def := range custom {
		if def.Matcher == nil {
			return CompiledInputMask{}, errors.New(
				"mask token missing matcher")
		}
		tokenMap[def.Symbol] = def
	}

	maskRunes := []rune(mask)
	entries := make([]compiledMaskEntry, 0, len(maskRunes))
	escaped := false
	for _, r := range maskRunes {
		if escaped {
			entries = append(entries, compiledMaskEntry{
				kind:    maskLiteral,
				literal: r,
			})
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if def, ok := tokenMap[r]; ok {
			transform := def.Transform
			if transform == nil {
				transform = identityRune
			}
			entries = append(entries, compiledMaskEntry{
				kind:      maskSlot,
				symbol:    def.Symbol,
				matcher:   def.Matcher,
				transform: transform,
			})
		} else {
			entries = append(entries, compiledMaskEntry{
				kind:    maskLiteral,
				literal: r,
			})
		}
	}
	if escaped {
		entries = append(entries, compiledMaskEntry{
			kind:    maskLiteral,
			literal: '\\',
		})
	}

	slotIndexes := make([]int, 0)
	for i, entry := range entries {
		if entry.kind == maskSlot {
			slotIndexes = append(slotIndexes, i)
		}
	}
	lastSlot := -1
	if len(slotIndexes) > 0 {
		lastSlot = slotIndexes[len(slotIndexes)-1]
	}

	return CompiledInputMask{
		pattern:          mask,
		entries:          entries,
		lastSlot:         lastSlot,
		slotEntryIndexes: slotIndexes,
	}, nil
}

func (m *CompiledInputMask) slotCount() int {
	return len(m.slotEntryIndexes)
}

func (m *CompiledInputMask) slotEntry(slotIndex int) compiledMaskEntry {
	// Defensive: every caller pre-checks slotCount, but an index
	// from a stale cursor must degrade to a skipped literal, not
	// a panic. The zero entry's nil matcher reads as "no slot".
	if slotIndex < 0 || slotIndex >= len(m.slotEntryIndexes) {
		return compiledMaskEntry{}
	}
	return m.entries[m.slotEntryIndexes[slotIndex]]
}

func (m *CompiledInputMask) rawFromFormattedRunes(formatted []rune) []rune {
	raw := make([]rune, 0, m.slotCount())
	limit := min(len(formatted), len(m.entries))
	for i := range limit {
		entry := m.entries[i]
		if entry.kind == maskSlot && entry.matcher != nil {
			ch := formatted[i]
			if entry.matcher(ch) {
				raw = append(raw, entry.transform(ch))
			}
		}
	}
	return raw
}

func (m *CompiledInputMask) formatRaw(raw []rune) string {
	if len(raw) == 0 || len(m.entries) == 0 {
		return ""
	}
	// A literal shows only when a slot follows it, so trailing
	// literals stay hidden until a slot past them fills. lastSlot
	// is fixed at compile time; a per-call suffix scan here would
	// allocate on every keystroke.
	out := make([]rune, 0, len(m.entries))
	rawIndex := 0
	for i, entry := range m.entries {
		if entry.kind == maskSlot {
			if rawIndex >= len(raw) {
				break
			}
			ch := raw[rawIndex]
			rawIndex++
			if entry.matcher != nil && entry.matcher(ch) {
				out = append(out, entry.transform(ch))
			}
		} else if rawIndex < len(raw) && i < m.lastSlot {
			out = append(out, entry.literal)
		}
	}
	return string(out)
}

func (m *CompiledInputMask) formattedToRawIndex(formattedLen, formattedIndex, rawLen int) int {
	idx := max(0, min(formattedLen, formattedIndex))
	limit := min(idx, len(m.entries))
	rawIndex := 0
	for i := range limit {
		if m.entries[i].kind == maskSlot {
			rawIndex++
		}
	}
	return max(0, min(rawLen, rawIndex))
}

func (m *CompiledInputMask) selectionRawRange(formattedLen, cursorPos int, selectBeg, selectEnd uint32, rawLen int) (int, int) {
	if selectBeg != selectEnd {
		beg, end := u32Sort(selectBeg, selectEnd)
		return m.formattedToRawIndex(formattedLen, int(beg), rawLen),
			m.formattedToRawIndex(formattedLen, int(end), rawLen)
	}
	idx := m.formattedToRawIndex(formattedLen, cursorPos, rawLen)
	return idx, idx
}

func (m *CompiledInputMask) rebuildRaw(prefix, suffix []rune) []rune {
	out := make([]rune, 0, m.slotCount())
	for _, ch := range prefix {
		if len(out) >= m.slotCount() {
			break
		}
		entry := m.slotEntry(len(out))
		if entry.matcher != nil && entry.matcher(ch) {
			out = append(out, entry.transform(ch))
		}
	}
	for _, ch := range suffix {
		if len(out) >= m.slotCount() {
			break
		}
		entry := m.slotEntry(len(out))
		if entry.matcher != nil && entry.matcher(ch) {
			out = append(out, entry.transform(ch))
		}
	}
	return out
}

func (m *CompiledInputMask) cursorFromRawIndex(raw []rune, rawIndex int) int {
	idx := max(0, min(len(raw), rawIndex))
	if idx == 0 {
		return 0
	}
	return utf8.RuneCountInString(m.formatRaw(raw[:idx]))
}

// InputMaskInsert inserts input into a masked formatted string.
func inputMaskInsert(formatted string, cursorPos int, selectBeg, selectEnd uint32, input string, compiled *CompiledInputMask) MaskEditResult {
	if compiled == nil || compiled.slotCount() == 0 {
		return MaskEditResult{Text: formatted, CursorPos: cursorPos}
	}

	formattedRunes := []rune(formatted)
	raw := compiled.rawFromFormattedRunes(formattedRunes)
	start, end := compiled.selectionRawRange(
		len(formattedRunes), cursorPos, selectBeg, selectEnd, len(raw))

	prefix := make([]rune, 0, compiled.slotCount())
	prefix = append(prefix, raw[:start]...)
	insertSlot := start

	for _, ch := range input {
		if insertSlot >= compiled.slotCount() {
			break
		}
		entry := compiled.slotEntry(insertSlot)
		if entry.matcher != nil && entry.matcher(ch) {
			prefix = append(prefix, entry.transform(ch))
			insertSlot++
		}
	}

	newRaw := compiled.rebuildRaw(prefix, raw[end:])
	newText := compiled.formatRaw(newRaw)
	newCursor := compiled.cursorFromRawIndex(newRaw, insertSlot)
	return MaskEditResult{
		Text:      newText,
		CursorPos: newCursor,
		Changed:   newText != formatted,
	}
}

func inputMaskRemove(formatted string, cursorPos int, selectBeg, selectEnd uint32, removeBackward bool, compiled *CompiledInputMask) MaskEditResult {
	if compiled == nil || compiled.slotCount() == 0 {
		return MaskEditResult{Text: formatted, CursorPos: cursorPos}
	}

	formattedRunes := []rune(formatted)
	raw := compiled.rawFromFormattedRunes(formattedRunes)
	start, end := compiled.selectionRawRange(
		len(formattedRunes), cursorPos, selectBeg, selectEnd, len(raw))

	if start == end {
		if removeBackward {
			if start == 0 {
				return MaskEditResult{Text: formatted, CursorPos: cursorPos}
			}
			start--
			end = start + 1
		} else {
			if start >= len(raw) {
				return MaskEditResult{Text: formatted, CursorPos: cursorPos}
			}
			end = start + 1
		}
	}

	newRaw := compiled.rebuildRaw(raw[:start], raw[end:])
	newText := compiled.formatRaw(newRaw)
	newCursor := compiled.cursorFromRawIndex(newRaw, start)
	return MaskEditResult{
		Text:      newText,
		CursorPos: newCursor,
		Changed:   newText != formatted,
	}
}

// InputMaskBackspace removes one editable slot left of cursor.
func inputMaskBackspace(formatted string, cursorPos int, selectBeg, selectEnd uint32, compiled *CompiledInputMask) MaskEditResult {
	return inputMaskRemove(formatted, cursorPos, selectBeg, selectEnd, true, compiled)
}

// InputMaskDelete removes one editable slot at/after cursor.
func inputMaskDelete(formatted string, cursorPos int, selectBeg, selectEnd uint32, compiled *CompiledInputMask) MaskEditResult {
	return inputMaskRemove(formatted, cursorPos, selectBeg, selectEnd, false, compiled)
}

// u32Sort returns (a, b) sorted so that a <= b.
func u32Sort(a, b uint32) (uint32, uint32) {
	if a <= b {
		return a, b
	}
	return b, a
}
