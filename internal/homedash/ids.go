package homedash

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type SourceID string
type EntityID string

const MaxNativeIDBytes = 200

var (
	sourceIDRe     = regexp.MustCompile(`^[a-z][a-z0-9_]{1,15}$`)
	dashboardIDRe  = regexp.MustCompile(`^d_[a-z2-7]{12}$`)
	pageIDRe       = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	slugRe         = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)
	iconRe         = regexp.MustCompile(`^[a-z0-9-]{1,48}$`)
	errBadEntityID = errors.New("invalid entity id")
)

func ValidSourceID(s string) bool    { return sourceIDRe.MatchString(s) }
func ValidDashboardID(s string) bool { return dashboardIDRe.MatchString(s) }
func ValidPageID(s string) bool      { return pageIDRe.MatchString(s) }
func ValidWidgetID(s string) bool    { return pageIDRe.MatchString(s) }
func ValidSlug(s string) bool        { return slugRe.MatchString(s) }
func ValidIcon(s string) bool        { return iconRe.MatchString(s) }

// ParseEntityID splits "<source>:<native>". The native part is 1..200 bytes of printable UTF-8
// without whitespace or bidi controls; it may contain '/' (ids travel in query strings only).
func ParseEntityID(s string) (SourceID, string, error) {
	src, native, ok := strings.Cut(s, ":")
	if !ok || !ValidSourceID(src) || native == "" || len(native) > MaxNativeIDBytes || !utf8.ValidString(native) {
		return "", "", errBadEntityID
	}
	for _, r := range native {
		if !unicode.IsPrint(r) || unicode.IsSpace(r) || isBidiControl(r) {
			return "", "", errBadEntityID
		}
	}
	return SourceID(src), native, nil
}

func MakeEntityID(src SourceID, native string) EntityID { return EntityID(string(src) + ":" + native) }

// NewDashboardID returns "d_" plus 12 lower-case base32 characters.
func NewDashboardID() string { return "d_" + randomBase32(12) }

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("homedash: crypto/rand: " + err.Error())
	}
	return b
}

func randomBase32(n int) string {
	s := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes(n)))
	return s[:n]
}

func randomHex(nBytes int) string { return hex.EncodeToString(randomBytes(nBytes)) }
