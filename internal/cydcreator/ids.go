package cydcreator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
)

// IDSource creates element and record ids. Prefixes in use: "ccp" project,
// "ccb" build, "ccj" job, "ccd" device, "ccr" revision, "ccm" message,
// "w" widget, "s" screen, "a" asset.
type IDSource interface{ NewID(prefix string) string }

// RandomIDs returns prefix + "_" + 12 lowercase hex characters from crypto/rand.
// The browser helper P.util.uid produces the same shape.
type RandomIDs struct{}

// NewID implements IDSource.
func (RandomIDs) NewID(prefix string) string {
	var b [6]byte
	// crypto/rand.Read never returns an error since Go 1.24 (it aborts the
	// process if the OS source fails), so the result needs no check.
	_, _ = rand.Read(b[:])
	return prefix + "_" + hex.EncodeToString(b[:])
}

// SeqIDs is a deterministic IDSource for tests: prefix + "_" + a zero-padded
// per-prefix counter ("w_0001", "w_0002", "s_0001"). The zero value is ready.
type SeqIDs struct{ n map[string]int }

// NewID implements IDSource.
func (s *SeqIDs) NewID(prefix string) string {
	if s.n == nil {
		s.n = map[string]int{}
	}
	s.n[prefix]++
	return fmt.Sprintf("%s_%04d", prefix, s.n[prefix])
}

// elementIDPattern is the structural rule ParseProject enforces for screen,
// widget and asset ids. Generated ids always match; hand-edited layouts with
// other ids are rejected as CYD_INVALID_LAYOUT.
var elementIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func validElementID(id string) bool { return elementIDPattern.MatchString(id) }
