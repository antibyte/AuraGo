package security

import (
	"net/http"
	"testing"
)

func TestCredentialRedirectRequiresExactOrigin(t *testing.T) {
	original, _ := http.NewRequest(http.MethodGet, "https://example.com/api", nil)
	for _, raw := range []string{"http://example.com/api", "https://example.com:444/api", "https://sub.example.com/api", "https://user@example.com/api"} {
		target, _ := http.NewRequest(http.MethodGet, raw, nil)
		if SameOriginRedirect(target, []*http.Request{original}) == nil {
			t.Errorf("unsafe redirect accepted: %s", raw)
		}
	}
	same, _ := http.NewRequest(http.MethodGet, "https://EXAMPLE.com:443/other", nil)
	if err := SameOriginRedirect(same, []*http.Request{original}); err != nil {
		t.Fatal(err)
	}
}
