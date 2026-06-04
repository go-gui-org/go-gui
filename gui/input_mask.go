package gui

import (
	"errors"
	"unicode"
	"unicode/utf8"
)

// InputMaskPreset defines common mask patterns.
type InputMaskPreset uint8

// InputMaskPreset constants.
const (
	MaskNone InputMaskPreset = iota
	MaskPhoneUS
	MaskCreditCard16
	MaskCreditCardAmex
	MaskExpiryMMYY
	MaskCVC
)

// MaskTokenDef defines one token symbol in a mask pattern.
type MaskTokenDef struct {
	Matcher   func(rune) bool
	Transform func(rune) rune
	Symbol    rune
}

// MaskEditResult stores the output of a mask edit operation.
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
type CompiledInputMask struct {
	Pattern          string
	entries          []compiledMaskEntry
	slotEntryIndexes []int
}

func identityRune(r rune) rune { return r }
func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }
func isMaskLetter(r rune) bool { return unicode.IsLetter(r) }
func isMaskAlnum(r rune) bool  { return unicode.IsLetter(r) || unicode.IsNumber(r) }

// InputMaskDefaultTokens returns built-in mask tokens.
func InputMaskDefaultTokens() []MaskTokenDef {
	return []MaskTokenDef{
		{Symbol: '9', Matcher: isASCIIDigit},
		{Symbol: 'a', Matcher: isMaskLetter},
		{Symbol: '*', Matcher: isMaskAlnum},
	}
}

// InputMaskFromPreset returns the mask pattern for a preset.
func InputMaskFromPreset(preset InputMaskPreset) string {
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
func CompileInputMask(mask string, custom []MaskTokenDef) (CompiledInputMask, error) {
	if len(mask) == 0 {
		return CompiledInputMask{}, errors.New("mask pattern is empty")
	}

	tokenMap := make(map[rune]MaskTokenDef)
	for _, def := range InputMaskDefaultTokens() {
		tokenMap[def.Symbol] = def
	}
	for _, def := range custom {
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

	return CompiledInputMask{
		Pattern:          mask,
		entries:          entries,
		slotEntryIndexes: slotIndexes,
	}, nil
}

func (m *CompiledInputMask) slotCount() int {
	return len(m.slotEntryIndexes)
}

func (m *CompiledInputMask) slotEntry(slotIndex int) compiledMaskEntry {
	return m.entries[m.slotEntryIndexes[slotIndex]]
}

func (m *CompiledInputMask) hasSlotAfter(entryIndex int) bool {
	for i := entryIndex + 1; i < len(m.entries); i++ {
		if m.entries[i].kind == maskSlot {
			return true
		}
	}
	return false
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
		} else if rawIndex < len(raw) && m.hasSlotAfter(i) {
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
func InputMaskInsert(formatted string, cursorPos int, selectBeg, selectEnd uint32, input string, compiled *CompiledInputMask) MaskEditResult {
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
func InputMaskBackspace(formatted string, cursorPos int, selectBeg, selectEnd uint32, compiled *CompiledInputMask) MaskEditResult {
	return inputMaskRemove(formatted, cursorPos, selectBeg, selectEnd, true, compiled)
}

// InputMaskDelete removes one editable slot at/after cursor.
func InputMaskDelete(formatted string, cursorPos int, selectBeg, selectEnd uint32, compiled *CompiledInputMask) MaskEditResult {
	return inputMaskRemove(formatted, cursorPos, selectBeg, selectEnd, false, compiled)
}

// u32Sort returns (a, b) sorted so that a <= b.
func u32Sort(a, b uint32) (uint32, uint32) {
	if a <= b {
		return a, b
	}
	return b, a
}
