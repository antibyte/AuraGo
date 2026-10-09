package desktop

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSameHostWebSocketOriginIsStrict(t *testing.T) {
	for _, tc := range []struct {
		name, host, origin string
		want               bool
	}{
		{"same host", "aurago.local:8088", "https://aurago.local:8088", true},
		{"host case", "AuraGo.local:8088", "https://aurago.local:8088", true},
		{"foreign host", "aurago.local:8088", "https://evil.example", false},
		{"other port", "aurago.local:8088", "https://aurago.local:9999", false},
		{"empty origin", "aurago.local:8088", "", false},
		{"unparsable origin", "aurago.local:8088", "::not a url", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/desktop/retronet/connect", nil)
			r.Host = tc.host
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if got := SameHostWebSocketOrigin(r); got != tc.want {
				t.Fatalf("SameHostWebSocketOrigin = %v, want %v", got, tc.want)
			}
		})
	}
	if SameHostWebSocketOrigin(nil) {
		t.Fatal("a nil request was accepted")
	}
}
