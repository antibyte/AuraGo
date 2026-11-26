package kaggle

import (
	"net/url"
	"regexp"
	"strings"
)

// A download URL is a credential: Kaggle signs it in the query (Google Cloud Storage) or carries a token in the
// path (kaggleusercontent.com), and anyone holding it can fetch the file without a login. The cleaning therefore
// removes such URLs by position (the url of every file entry) and by shape (everything that grants access by
// itself), so they never reach model context, logs or stored job results.

// removedURL replaces a download URL inside longer text.
const removedURL = "[download URL removed]"

var outputURLPattern = regexp.MustCompile("(?i)https?://[^\\s\"'<>\\\\]+")

// fileEntry reports whether obj describes a file: it has a fileName key, or it sits in a list whose key
// (parentKey, lower case) ends in "files".
func fileEntry(obj map[string]any, parentKey string) bool {
	for k := range obj {
		if strings.EqualFold(k, "fileName") {
			return true
		}
	}
	return strings.HasSuffix(parentKey, "files")
}

// credentialURL reports whether raw is a single http(s) URL that grants access by itself.
func credentialURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	if strings.ContainsAny(raw, " \t\r\n\v\f") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return true
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	if u.User != nil {
		return true
	}
	if host == "kaggleusercontent.com" || strings.HasSuffix(host, ".kaggleusercontent.com") {
		return true
	}
	return hasCredentialParam(u.RawQuery) || hasCredentialParam(u.Fragment)
}

func hasCredentialParam(s string) bool {
	if s == "" {
		return false
	}
	for _, pair := range strings.FieldsFunc(s, func(r rune) bool { return r == '&' || r == ';' }) {
		name, _, _ := strings.Cut(pair, "=")
		if dec, err := url.QueryUnescape(name); err == nil {
			name = dec
		}
		if credentialParamName(strings.ToLower(name)) {
			return true
		}
	}
	return false
}

func credentialParamName(n string) bool {
	if strings.HasPrefix(n, "x-goog-") || strings.HasPrefix(n, "x-amz-") {
		return true
	}
	if strings.Contains(n, "token") || strings.Contains(n, "signature") || strings.Contains(n, "credential") {
		return true
	}
	switch n {
	case "sig", "key", "api_key", "apikey", "googleaccessid", "awsaccesskeyid":
		return true
	}
	return false
}

// redactOutputURLs replaces every credential URL inside text with removedURL and keeps trailing sentence
// punctuation outside the replacement.
func redactOutputURLs(text string) string {
	return outputURLPattern.ReplaceAllStringFunc(text, func(m string) string {
		trimmed := strings.TrimRight(m, ".,;:!?)]}")
		if credentialURL(trimmed) {
			return removedURL + m[len(trimmed):]
		}
		return m
	})
}
