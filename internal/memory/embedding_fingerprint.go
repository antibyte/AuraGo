package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// FingerprintEmbeddingBaseURL returns the form of an embedding endpoint that may
// be stored in fingerprints. Fingerprints are written into every document's
// metadata, the corpus snapshots, the status API and the file-index state, so
// they must never carry the credentials that compatible providers accept in the
// URL. URLs without userinfo, query, fragment or path parameters keep exactly the
// previous form (trimmed, without trailing slashes), so existing installations
// keep their fingerprints and do not re-embed.
func FingerprintEmbeddingBaseURL(raw string) string {
	return strings.TrimRight(RedactURLForFingerprint(raw), "/")
}

// RedactURLForFingerprint trims the URL and removes anything that can carry a
// credential: userinfo and fragment are dropped, the query is replaced by a short
// digest so that a changed query still changes the fingerprint, and so is every
// path parameter (the part of a path segment after a ';'). Input without any of
// these is only trimmed, byte for byte. Trailing slashes are kept;
// FingerprintEmbeddingBaseURL trims them.
//
// The redaction works on the string and never on a parsed URL: URLs that
// net/url rejects (bad escapes, spaces, a bad port, no scheme or host) are exactly
// where a parse-based version would hand the raw credential back. It only looks
// for the delimiters of the URL grammar. A secret that sits in a plain path
// segment (https://host/v1/SECRET) has no delimiter and cannot be recognized
// generically; such endpoints are a known limit of this function.
func RedactURLForFingerprint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.ContainsAny(trimmed, "?#@;") {
		return trimmed
	}
	rest := trimmed
	changed := false

	// Everything from the first '#' is a fragment.
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		changed = i+1 < len(rest)
		rest = rest[:i]
	}
	// Everything after the first '?' is the query.
	query := ""
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		query = rest[i+1:]
		rest = rest[:i]
		changed = changed || query != ""
	}

	// Userinfo sits between the start of the authority and the last '@' before the
	// first '/' behind that start.
	authStart := fingerprintAuthorityStart(rest)
	authority := rest[authStart:]
	if slash := strings.IndexByte(authority, '/'); slash >= 0 {
		authority = authority[:slash]
	}
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		rest = rest[:authStart] + rest[authStart+at+1:]
		changed = true
	}

	// Path parameters: each ';'-separated parameter of a segment is hashed.
	if strings.IndexByte(rest[authStart:], ';') >= 0 {
		segments := strings.Split(rest[authStart:], "/")
		for i, segment := range segments {
			semicolon := strings.IndexByte(segment, ';')
			if semicolon < 0 {
				continue
			}
			var hashed strings.Builder
			hashed.WriteString(segment[:semicolon])
			for _, param := range strings.Split(segment[semicolon+1:], ";") {
				hashed.WriteByte(';')
				if param != "" {
					hashed.WriteString("p=" + fingerprintDigest(param))
					changed = true
				}
			}
			segments[i] = hashed.String()
		}
		rest = rest[:authStart] + strings.Join(segments, "/")
	}

	if !changed {
		return trimmed
	}
	if query != "" {
		rest += "?q=" + fingerprintDigest(query)
	}
	return rest
}

// fingerprintAuthorityStart returns where the authority of a URL-like string
// begins: behind "://" (when no '/' precedes it), behind a leading "//", or at the
// start for scheme-less input such as host:port/path.
func fingerprintAuthorityStart(s string) int {
	if i := strings.Index(s, "://"); i >= 0 && strings.IndexByte(s[:i], '/') < 0 {
		return i + 3
	}
	if strings.HasPrefix(s, "//") {
		return 2
	}
	return 0
}

// fingerprintDigest is the short, stable stand-in for a redacted URL part.
func fingerprintDigest(part string) string {
	sum := sha256.Sum256([]byte(part))
	return hex.EncodeToString(sum[:])[:16]
}
