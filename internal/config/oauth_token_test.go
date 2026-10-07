package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOAuthTokenReadsLegacyExpiryAndWritesCanonical(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`{"access_token":"fixture","token_expiry":"2027-01-01T00:00:00Z"}`, "2027-01-01T00:00:00Z"},
		{`{"expiry":"2028-01-01T00:00:00Z","token_expiry":"2027-01-01T00:00:00Z"}`, "2028-01-01T00:00:00Z"},
	} {
		var token OAuthToken
		if err := json.Unmarshal([]byte(tc.raw), &token); err != nil || token.Expiry != tc.want {
			t.Fatalf("legacy token: %+v %v", token, err)
		}
		data, err := json.Marshal(token)
		if err != nil || strings.Contains(string(data), "token_expiry") {
			t.Fatal("writer emitted legacy expiry")
		}
	}
}
