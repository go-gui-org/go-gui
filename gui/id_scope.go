package gui

import (
	"strconv"
	"strings"
)

// IDSep separates the segments of a composed widget ID.
//
// A widget ID is a colon-joined path: the owning widget's ID, then the
// segments that distinguish one inner shape from another. Colon was
// chosen because it was already the dominant separator in the codebase
// and is rare in application-supplied IDs, which favour "-" and "_".
const IDSep = ":"

// ScopeID composes an inner widget's ID from the owning widget's ID and
// one or more parts.
//
// IDs stay flat, window-global strings: the result is the whole
// identity, and it is what SetFocus, ScrollVerticalTo, FindByID and the
// test helpers take. The owner may itself be a composed ID, which is
// how a scope nests:
//
//	base := ScopeID(cfg.ID, "header", col.ID) // "grid:header:name"
//	ScopeID(base, "resize")                   // "grid:header:name:resize"
//
// Parts must not contain IDSep. Nothing enforces that, because the only
// way a stray separator does harm is by colliding with another ID, and
// a collision is already reported by gui.Debug and asserted by
// (*Window).TestDuplicateIDs. A leaf value derived from data — a row
// key, a heading slug — is a part, not a scope, and keeps whatever
// spelling its own producer uses.
//
// Empty segments are dropped, so no leading or doubled separator
// appears. An empty owner is therefore not an error: an ID-less
// composite still produces workable inner IDs, and two of them in one
// window collide loudly rather than silently.
//
// Costs exactly one allocation: the returned string. parts does not
// escape, so the variadic backing array stays on the caller's stack.
func ScopeID(owner string, parts ...string) string {
	// Fast paths. Each arm is a single expression, so the compiler emits
	// one runtime.concatstrings call regardless of operand count.
	switch len(parts) {
	case 0:
		return owner
	case 1:
		if parts[0] == "" {
			return owner
		}
		if owner == "" {
			return parts[0]
		}
		return owner + IDSep + parts[0]
	case 2:
		if owner != "" && parts[0] != "" && parts[1] != "" {
			return owner + IDSep + parts[0] + IDSep + parts[1]
		}
	}

	// General path: size the buffer exactly, then fill it. Builder.String
	// hands over the buffer without copying, so this is still one alloc.
	size, count := scopeIDSize(owner, parts)
	if count == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(size)
	scopeIDWrite(&b, owner)
	for _, p := range parts {
		scopeIDWrite(&b, p)
	}
	return b.String()
}

// ScopeIDN composes an ID whose last segment is a number, without
// materialising the number as its own string.
//
// Use it for loop-derived identity — a list row, a calendar day, a radio
// option — where the index or key is what distinguishes one sibling from
// the next. part may be empty, in which case the result is owner:n.
//
// Costs exactly one allocation: the returned string.
func ScopeIDN(owner, part string, n int) string {
	// 20 bytes holds any int64 in base 10 including the sign, so
	// AppendInt never grows the slice and the array stays on the stack:
	// Builder.Write copies out of it.
	var digits [20]byte
	d := strconv.AppendInt(digits[:0], int64(n), 10)

	size, count := scopeIDSize(owner, nil)
	if part != "" {
		size += len(part) + len(IDSep)
		count++
	}
	size += len(d)
	if count > 0 {
		size += len(IDSep)
	}

	var b strings.Builder
	b.Grow(size)
	scopeIDWrite(&b, owner)
	scopeIDWrite(&b, part)
	if b.Len() > 0 {
		b.WriteString(IDSep)
	}
	b.Write(d)
	return b.String()
}

// scopeIDSize reports the exact byte length of the composed ID and how
// many non-empty segments it has, so the builder can be sized once.
func scopeIDSize(owner string, parts []string) (size, count int) {
	if owner != "" {
		size = len(owner)
		count = 1
	}
	for _, p := range parts {
		if p == "" {
			continue
		}
		size += len(p)
		count++
	}
	if count > 1 {
		size += (count - 1) * len(IDSep)
	}
	return size, count
}

// scopeIDWrite appends one segment, prefixing the separator only when
// something has already been written. Empty segments are dropped.
func scopeIDWrite(b *strings.Builder, seg string) {
	if seg == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteString(IDSep)
	}
	b.WriteString(seg)
}
