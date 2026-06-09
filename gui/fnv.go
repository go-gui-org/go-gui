package gui

// FNV-1a 64-bit constants.
const (
	Fnv64Offset = uint64(14695981039346656037)
	Fnv64Prime  = uint64(1099511628211)
)

// Fnv64Str hashes a string into an existing FNV-1a 64-bit hash.
func Fnv64Str(h uint64, s string) uint64 {
	for i := range len(s) {
		h = (h ^ uint64(s[i])) * Fnv64Prime
	}
	return h
}

// Fnv64Byte hashes a byte into an existing FNV-1a 64-bit hash.
func Fnv64Byte(h uint64, b byte) uint64 {
	return (h ^ uint64(b)) * Fnv64Prime
}

// FnvSum32 returns the 32-bit FNV-1a hash of a string.
func FnvSum32(s string) uint32 {
	const offset uint32 = 2166136261
	const prime uint32 = 16777619
	h := offset
	for i := range len(s) {
		h ^= uint32(s[i])
		h *= prime
	}
	return h
}
