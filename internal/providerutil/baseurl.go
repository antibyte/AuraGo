package providerutil

import "strings"

// NormalizeBaseURL strips common chat-completions suffixes that users often
// paste into provider URLs. The OpenAI-compatible client appends the endpoint
// path itself, so keeping these suffixes would duplicate the final request URL.
func NormalizeBaseURL(u string) string {
	u = strings.TrimRight(u, "/")
	for _, suffix := range []string{"/chat/completions", "/v1/chat/completions"} {
		if strings.HasSuffix(strings.ToLower(u), suffix) {
			u = u[:len(u)-len(suffix)]
			break
		}
	}
	return u
}
