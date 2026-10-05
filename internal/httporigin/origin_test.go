package httporigin

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestSameOriginRedirectRequiresExactOrigin(t *testing.T) {
	original, _ := http.NewRequest(http.MethodGet, "https://example.com/api", nil)
	for name, raw := range map[string]string{
		"https downgrade": "http://example.com/api",
		"other port":      "https://example.com:444/api",
		"other host":      "https://sub.example.com/api",
		"userinfo":        "https://user@example.com/api",
	} {
		target, _ := http.NewRequest(http.MethodGet, raw, nil)
		if SameOriginRedirect(target, []*http.Request{original}) == nil {
			t.Errorf("%s: unsafe redirect accepted: %s", name, raw)
		}
	}
	same, _ := http.NewRequest(http.MethodGet, "https://EXAMPLE.com:443/other", nil)
	if err := SameOriginRedirect(same, []*http.Request{original}); err != nil {
		t.Fatalf("same-origin redirect rejected: %v", err)
	}
}

func TestSameOriginRedirectCapsHops(t *testing.T) {
	next, _ := http.NewRequest(http.MethodGet, "https://example.com/hop", nil)
	var via []*http.Request
	for i := 0; i < 9; i++ {
		hop, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("https://example.com/hop%d", i), nil)
		via = append(via, hop)
	}
	if err := SameOriginRedirect(next, via); err != nil {
		t.Fatalf("ninth same-origin redirect rejected: %v", err)
	}
	extra, _ := http.NewRequest(http.MethodGet, "https://example.com/hop9", nil)
	via = append(via, extra)
	if SameOriginRedirect(next, via) == nil {
		t.Fatal("same-origin redirect after 10 requests accepted; the policy stops after 10 redirects")
	}
	if SameOriginRedirect(next, nil) == nil {
		t.Fatal("redirect without an original request accepted")
	}
}

func TestSameOriginRedirectErrorText(t *testing.T) {
	original, _ := http.NewRequest(http.MethodGet, "https://example.com/api", nil)
	target, _ := http.NewRequest(http.MethodGet, "https://evil.example.net/api", nil)
	err := SameOriginRedirect(target, []*http.Request{original})
	if !errors.Is(err, ErrCrossOriginRedirect) {
		t.Fatalf("error = %v, want ErrCrossOriginRedirect", err)
	}
	if err.Error() != "cross-origin integration redirect rejected" {
		t.Fatalf("error text = %q, want the stable rejection text", err.Error())
	}
	wrapped := &url.Error{Op: "Get", URL: target.URL.String(), Err: err}
	if !errors.Is(wrapped, ErrCrossOriginRedirect) {
		t.Fatal("ErrCrossOriginRedirect must survive net/http's *url.Error wrapping")
	}
}

func TestSameOriginUsesEffectivePort(t *testing.T) {
	parse := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	if !SameOrigin(parse("http://lan.example:80/a"), parse("http://LAN.example/b")) {
		t.Error("explicit default HTTP port must match the implicit one")
	}
	if SameOrigin(parse("http://lan.example/a"), parse("https://lan.example/a")) {
		t.Error("scheme change must not count as the same origin")
	}
	if SameOrigin(nil, parse("https://lan.example/a")) {
		t.Error("nil URL must never match")
	}
}
