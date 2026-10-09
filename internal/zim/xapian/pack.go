package xapian

import (
	"math"
	"math/bits"
)

// Glass key prefixes inside the postlist table (see "Format notes" in the plan).
var (
	keyPrefixMetadata   = []byte{0x00, 0xc0}
	keyPrefixValueChunk = []byte{0x00, 0xd8}
	keyPrefixDocLen     = []byte{0x00, 0xe0}
	keyFirstTerm        = []byte{0x00, 0xff}
)

// unpackUint decodes a little-endian base-128 varint (7 data bits per byte,
// high bit set on every byte except the last). It returns the value and the
// number of bytes consumed.
func unpackUint(b []byte) (uint64, int, error) {
	var v uint64
	var shift uint
	for i, c := range b {
		if shift == 63 && c > 1 {
			// The tenth byte holds only bit 63 and must end the varint, so the
			// loop never shifts past 63.
			return 0, 0, corruptf("varint overflows 64 bits")
		}
		v |= uint64(c&0x7f) << shift
		if c < 0x80 {
			return v, i + 1, nil
		}
		shift += 7
	}
	return 0, 0, corruptf("varint runs past end of data")
}

// unpackUint32 is unpackUint for values that must fit Xapian's 32-bit types.
func unpackUint32(b []byte) (uint32, int, error) {
	v, n, err := unpackUint(b)
	if err != nil {
		return 0, 0, err
	}
	if v > math.MaxUint32 {
		return 0, 0, corruptf("value %d exceeds 32 bits", v)
	}
	return uint32(v), n, nil
}

// appendUint appends v in the varint format read by unpackUint.
func appendUint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

// unpackSortableUint decodes the glass "sort preserving" unsigned integer.
// Values below 0x8000 are two big-endian bytes whose first byte is < 0x80.
// Otherwise the number of leading one bits L in the first byte gives L+1
// following bytes; the remaining low bits of the first byte are the most
// significant bits of the big-endian value. 0xff never starts an encoding.
func unpackSortableUint(b []byte) (uint64, int, error) {
	if len(b) == 0 {
		return 0, 0, corruptf("sortable integer runs past end of data")
	}
	first := b[0]
	if first < 0x80 {
		if len(b) < 2 {
			return 0, 0, corruptf("sortable integer runs past end of data")
		}
		return uint64(first)<<8 | uint64(b[1]), 2, nil
	}
	if first == 0xff {
		return 0, 0, corruptf("invalid sortable integer length byte 0xff")
	}
	extra := bits.LeadingZeros8(^first) + 1
	if len(b) < 1+extra {
		return 0, 0, corruptf("sortable integer runs past end of data")
	}
	v := uint64(first & (0xff >> uint(extra)))
	for _, c := range b[1 : 1+extra] {
		v = v<<8 | uint64(c)
	}
	return v, 1 + extra, nil
}

// appendSortableUint appends v in the format read by unpackSortableUint.
func appendSortableUint(dst []byte, v uint64) []byte {
	if v < 0x8000 {
		return append(dst, byte(v>>8), byte(v))
	}
	n := (bits.Len64(v) + 5) / 7 // total encoded length, 3..9
	out := make([]byte, n)
	for i := n - 1; i >= 1; i-- {
		out[i] = byte(v)
		v >>= 8
	}
	out[0] = byte(v) | byte(0xff<<uint(10-n))
	return append(dst, out...)
}

// appendSortPreservingString appends s with every 0x00 byte followed by 0xff
// and, unless last is true, a 0x00 terminator.
func appendSortPreservingString(dst []byte, s string, last bool) []byte {
	for i := 0; i < len(s); i++ {
		dst = append(dst, s[i])
		if s[i] == 0 {
			dst = append(dst, 0xff)
		}
	}
	if !last {
		dst = append(dst, 0)
	}
	return dst
}

// unpackSortPreservingString reads a string written by
// appendSortPreservingString. terminated reports whether a 0x00 terminator
// (not followed by 0xff) ended it; n counts the terminator.
func unpackSortPreservingString(b []byte) (s string, n int, terminated bool) {
	out := make([]byte, 0, len(b))
	i := 0
	for i < len(b) {
		c := b[i]
		i++
		if c == 0 {
			if i == len(b) || b[i] != 0xff {
				return string(out), i, true
			}
			i++
		}
		out = append(out, c)
	}
	return string(out), i, false
}

// postlistKey is the key of the first posting chunk of term ("" = document
// length list).
func postlistKey(term string) []byte {
	if term == "" {
		return append([]byte(nil), keyPrefixDocLen...)
	}
	return appendSortPreservingString(nil, term, true)
}

// postlistChunkKey is the key of a later posting chunk of term starting at did.
func postlistChunkKey(term string, did uint32) []byte {
	var k []byte
	if term == "" {
		k = append(k, keyPrefixDocLen...)
	} else {
		k = appendSortPreservingString(k, term, false)
	}
	return appendSortableUint(k, uint64(did))
}

// valueChunkKey is the key of the value stream chunk for slot starting at did.
func valueChunkKey(slot, did uint32) []byte {
	k := append([]byte(nil), keyPrefixValueChunk...)
	k = appendUint(k, uint64(slot))
	return appendSortableUint(k, uint64(did))
}

// docdataKey is the docdata table key of did.
func docdataKey(did uint32) []byte {
	return appendSortableUint(nil, uint64(did))
}

// metadataKey is the postlist table key of user metadata entry name.
func metadataKey(name string) []byte {
	return append(append([]byte(nil), keyPrefixMetadata...), name...)
}
