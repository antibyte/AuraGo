// Package uid provides stdlib-only UUID v4 generation as a drop-in replacement
// for github.com/google/uuid, using only crypto/rand from the standard library.
package uid

import (
	crand "crypto/rand"
	"fmt"
)

// New returns a randomly generated UUID v4 string in canonical form
// (xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx). Drop-in for uuid.New().String().
func New() string {
	var b [16]byte
	_, _ = crand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10xx
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NewString is an alias for New(), matching uuid.NewString().
func NewString() string { return New() }
