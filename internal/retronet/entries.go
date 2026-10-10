package retronet

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"aurago/internal/security"
)

// Own-entry storage in the desktop setting EntriesSetting.
const (
	EntriesSetting         = "retronet.entries"
	DefaultEntriesDocument = `{"version":1,"entries":[]}`
	MaxOwnEntries          = 64
	MaxEntriesDocumentSize = 64 << 10
)

// Field limits of own entries.
const (
	maxNameRunes        = 40
	maxDescriptionRunes = 80
	maxHostnameLength   = 253
	maxLabelLength      = 63
)

var (
	ownIDPattern    = regexp.MustCompile(`^own-[a-z0-9]{8,32}$`)
	sshUserPattern  = regexp.MustCompile(`^[a-z0-9._-]{1,32}$`)
	hostKeyPattern  = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]{43}$`)
	hexLabelPattern = regexp.MustCompile(`^0[xX][0-9a-fA-F]*$`)
)

// errInvalidEntries is the only error the validators return, so the settings
// API never echoes user input.
var errInvalidEntries = fmt.Errorf("invalid desktop setting value for %s", EntriesSetting)

// entriesDocument is the stored JSON document.
type entriesDocument struct {
	Version int           `json:"version"`
	Entries []storedEntry `json:"entries"`
}

// storedEntry holds exactly the stored own-entry fields; every other field is
// rejected by DisallowUnknownFields.
type storedEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Protocol    Protocol `json:"protocol"`
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	Kind        Kind     `json:"kind,omitempty"`
	Charset     Charset  `json:"charset,omitempty"`
	User        string   `json:"user,omitempty"`
	HostKey     string   `json:"host_key,omitempty"`
}

// ValidateEntriesDocument checks the stored JSON document (strict decode, DisallowUnknownFields,
// trailing data rejected, version 1, entries non-nil, <= 64 entries, <= 64 KiB, unique IDs).
// Errors are always fmt.Errorf("invalid desktop setting value for %s", EntriesSetting) so the
// settings API never echoes user input.
func ValidateEntriesDocument(value string) error {
	_, err := decodeEntriesDocument(value)
	return err
}

// ParseEntriesDocument validates and returns the own entries with Own=true, Category=CategoryOwn.
// An empty string parses as DefaultEntriesDocument.
func ParseEntriesDocument(value string) ([]Entry, error) {
	if value == "" {
		value = DefaultEntriesDocument
	}
	stored, err := decodeEntriesDocument(value)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(stored))
	for _, s := range stored {
		entries = append(entries, s.entry())
	}
	return entries, nil
}

// EncodeEntriesDocument serializes own entries (Own/Category/DescriptionKey omitted from storage)
// and validates the result before returning it.
func EncodeEntriesDocument(entries []Entry) (string, error) {
	doc := entriesDocument{Version: 1, Entries: make([]storedEntry, 0, len(entries))}
	for _, e := range entries {
		doc.Entries = append(doc.Entries, storedFrom(e))
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", errInvalidEntries
	}
	value := string(raw)
	if err := ValidateEntriesDocument(value); err != nil {
		return "", err
	}
	return value, nil
}

// decodeEntriesDocument applies every document and entry rule.
func decodeEntriesDocument(value string) ([]storedEntry, error) {
	if len(value) > MaxEntriesDocumentSize || !utf8.ValidString(value) {
		return nil, errInvalidEntries
	}
	if !hasExactKeys(value) {
		return nil, errInvalidEntries
	}
	dec := json.NewDecoder(strings.NewReader(value))
	dec.DisallowUnknownFields()
	var doc entriesDocument
	if err := dec.Decode(&doc); err != nil {
		return nil, errInvalidEntries
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, errInvalidEntries
	}
	if doc.Version != 1 || doc.Entries == nil || len(doc.Entries) > MaxOwnEntries {
		return nil, errInvalidEntries
	}
	seen := make(map[string]struct{}, len(doc.Entries))
	for _, s := range doc.Entries {
		if !ownIDPattern.MatchString(s.ID) || !validName(s.Name) || !validDescription(s.Description) || !validTarget(s.entry()) {
			return nil, errInvalidEntries
		}
		if _, duplicate := seen[s.ID]; duplicate {
			return nil, errInvalidEntries
		}
		seen[s.ID] = struct{}{}
	}
	return doc.Entries, nil
}

// Exact field names of the stored document, used by hasExactKeys.
var (
	documentKeys = map[string]bool{"version": true, "entries": true}
	entryKeys    = map[string]bool{
		"id": true, "name": true, "description": true, "protocol": true, "host": true,
		"port": true, "kind": true, "charset": true, "user": true, "host_key": true,
	}
)

// hasExactKeys walks the first JSON value of value and reports whether every
// object key is exactly a stored field name, no key repeats within an object,
// and no object or array appears where the document has none. encoding/json
// folds key case ("HOST", "hoſt" and "host" are one field there), so without
// this walk a document could carry one value for the validator and another
// in the raw string that the browser parses. Syntax errors report false.
func hasExactKeys(value string) bool {
	dec := json.NewDecoder(strings.NewReader(value))
	if !expectDelim(dec, '{') {
		return false
	}
	seen := map[string]bool{}
	for dec.More() {
		key, ok := nextKey(dec, documentKeys, seen)
		if !ok {
			return false
		}
		if key != "entries" {
			if !scalarValue(dec) {
				return false
			}
			continue
		}
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if tok == nil {
			continue // null: the document rules reject it
		}
		if d, isDelim := tok.(json.Delim); !isDelim || d != '[' {
			return false
		}
		for dec.More() {
			if !hasExactEntryKeys(dec) {
				return false
			}
		}
		if !expectDelim(dec, ']') {
			return false
		}
	}
	return expectDelim(dec, '}')
}

// hasExactEntryKeys checks one element of the entries array.
func hasExactEntryKeys(dec *json.Decoder) bool {
	if !expectDelim(dec, '{') {
		return false
	}
	seen := map[string]bool{}
	for dec.More() {
		if _, ok := nextKey(dec, entryKeys, seen); !ok || !scalarValue(dec) {
			return false
		}
	}
	return expectDelim(dec, '}')
}

// nextKey reads an object key and accepts it only if it is in allowed and not
// yet in seen.
func nextKey(dec *json.Decoder, allowed, seen map[string]bool) (string, bool) {
	tok, err := dec.Token()
	if err != nil {
		return "", false
	}
	key, isString := tok.(string)
	if !isString || !allowed[key] || seen[key] {
		return "", false
	}
	seen[key] = true
	return key, true
}

// scalarValue consumes one value and reports false if it is an object or array.
func scalarValue(dec *json.Decoder) bool {
	tok, err := dec.Token()
	if err != nil {
		return false
	}
	_, isDelim := tok.(json.Delim)
	return !isDelim
}

// expectDelim consumes one token and reports whether it is the delimiter want.
func expectDelim(dec *json.Decoder, want json.Delim) bool {
	tok, err := dec.Token()
	if err != nil {
		return false
	}
	d, isDelim := tok.(json.Delim)
	return isDelim && d == want
}

// entry converts the stored form into an own Entry.
func (s storedEntry) entry() Entry {
	return Entry{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		Category:    CategoryOwn,
		Protocol:    s.Protocol,
		Host:        s.Host,
		Port:        s.Port,
		Kind:        s.Kind,
		Charset:     s.Charset,
		User:        s.User,
		HostKey:     s.HostKey,
		Own:         true,
	}
}

// storedFrom keeps only the stored fields of e.
func storedFrom(e Entry) storedEntry {
	return storedEntry{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		Protocol:    e.Protocol,
		Host:        e.Host,
		Port:        e.Port,
		Kind:        e.Kind,
		Charset:     e.Charset,
		User:        e.User,
		HostKey:     e.HostKey,
	}
}

// validTarget checks protocol, host, port and the protocol-specific fields.
// The catalog test applies it to every catalog entry as well.
func validTarget(e Entry) bool {
	if !validHost(e.Host) || blockedPort(e.Port) {
		return false
	}
	switch e.Protocol {
	case ProtocolTelnet:
		return e.Kind.valid() && e.Charset.valid() && e.User == "" && e.HostKey == ""
	case ProtocolSSH:
		return e.Kind == "" && e.Charset == "" && sshUserPattern.MatchString(e.User) &&
			(e.HostKey == "" || hostKeyPattern.MatchString(e.HostKey))
	default:
		return false
	}
}

// validHost accepts a public IP literal or an RFC 1123 host name.
func validHost(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return !security.IsRestrictedNetworkIP(unmapIP(ip))
	}
	return validHostname(host)
}

// validHostname accepts RFC 1123 names: at most 253 bytes, labels of 1-63
// letters, digits and hyphens without a leading or trailing hyphen. A final
// all-numeric label or one in hexadecimal form (0x7f000001, 1.0x7f) is
// rejected because resolvers may read such names as IPv4 shorthand
// (inet_aton). localhost and *.localhost are rejected too (any case, with or
// without a trailing dot): they resolve to the loopback interface and can
// never be dialed.
func validHostname(host string) bool {
	if host == "" || len(host) > maxHostnameLength || isLocalhostName(host) {
		return false
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > maxLabelLength || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	last := labels[len(labels)-1]
	return strings.Trim(last, "0123456789") != "" && !hexLabelPattern.MatchString(last)
}

// isLocalhostName reports localhost and every name below it, ignoring case and
// one trailing dot.
func isLocalhostName(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

// zeroWidthJoiner (U+200D) is the one invisible rune allowed in display text:
// emoji sequences need it.
const zeroWidthJoiner rune = 0x200d

// validName requires 1-40 runes with at least one visible character and no
// invisible, format or control runes.
func validName(name string) bool {
	visible := func(r rune) bool { return !unicode.IsSpace(r) && r != zeroWidthJoiner }
	return utf8.RuneCountInString(name) <= maxNameRunes && strings.IndexFunc(name, visible) >= 0 && !hasInvisible(name)
}

// validDescription allows up to 80 runes without invisible, format or control
// runes.
func validDescription(description string) bool {
	return utf8.RuneCountInString(description) <= maxDescriptionRunes && !hasInvisible(description)
}

// hasInvisible reports runes that can hide or reorder text: categories Cc,
// Cf (zero-width and bidirectional controls, tag characters), Co, Zl and Zp,
// and U+FFFD (also what invalid UTF-8 and lone surrogate escapes decode to).
// U+200D is exempt.
func hasInvisible(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		if r == zeroWidthJoiner {
			return false
		}
		return r == unicode.ReplacementChar || unicode.In(r, unicode.Cc, unicode.Cf, unicode.Co, unicode.Zl, unicode.Zp)
	}) >= 0
}
