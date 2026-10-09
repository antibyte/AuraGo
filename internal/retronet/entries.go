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
	ownIDPattern   = regexp.MustCompile(`^own-[a-z0-9]{8,32}$`)
	sshUserPattern = regexp.MustCompile(`^[a-z0-9._-]{1,32}$`)
	hostKeyPattern = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]{43}$`)
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
// all-numeric label is rejected because resolvers may read names such as
// "127.1" as IPv4 shorthand.
func validHostname(host string) bool {
	if host == "" || len(host) > maxHostnameLength {
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
	return strings.Trim(labels[len(labels)-1], "0123456789") != ""
}

// validName requires 1-40 runes, not blank, without control characters.
func validName(name string) bool {
	return strings.TrimSpace(name) != "" && utf8.RuneCountInString(name) <= maxNameRunes && !hasControl(name)
}

// validDescription allows up to 80 runes without control characters.
func validDescription(description string) bool {
	return utf8.RuneCountInString(description) <= maxDescriptionRunes && !hasControl(description)
}

func hasControl(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) >= 0
}
