package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// FingerprintEmbeddingBaseURL returns the form of an embedding endpoint that may
// be stored in fingerprints. Fingerprints are written into every document's
// metadata, the corpus snapshots, the status API and the file-index state, so
// they must never carry the credentials that compatible providers accept in the
// URL. URLs without userinfo, query or fragment keep exactly the previous form
// (trimmed, without trailing slashes), so existing installations keep their
// fingerprints and do not re-embed.
func FingerprintEmbeddingBaseURL(raw string) string {
	return strings.TrimRight(RedactURLForFingerprint(raw), "/")
}

// RedactURLForFingerprint trims the URL and removes anything that can carry a
// credential: userinfo and fragment are dropped, and the query is replaced by a
// short digest so that a changed query still changes the fingerprint. Input that
// is not an absolute URL, cannot be parsed or has none of these parts is only
// trimmed. Trailing slashes are kept; FingerprintEmbeddingBaseURL trims them.
func RedactURLForFingerprint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	u, err := url.Parse(trimmed)
	if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
		return trimmed
	}
	if u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.RawFragment == "" {
		return trimmed
	}
	if u.RawQuery != "" {
		sum := sha256.Sum256([]byte(u.RawQuery))
		u.RawQuery = "q=" + hex.EncodeToString(sum[:])[:16]
	}
	u.ForceQuery = false
	u.User = nil
	u.Fragment = ""
	u.RawFragment = ""
	return u.String()
}
