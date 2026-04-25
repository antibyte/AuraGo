package fritzbox

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Digest Auth helper tests ───

func TestParseDigestParams(t *testing.T) {
	header := `realm="fritz.box", nonce="abc123", opaque="xyz", qop="auth"`
	got := parseDigestParams(header)
	if got["realm"] != "fritz.box" {
		t.Errorf("realm = %q, want %q", got["realm"], "fritz.box")
	}
	if got["nonce"] != "abc123" {
		t.Errorf("nonce = %q, want %q", got["nonce"], "abc123")
	}
	if got["opaque"] != "xyz" {
		t.Errorf("opaque = %q, want %q", got["opaque"], "xyz")
	}
	if got["qop"] != "auth" {
		t.Errorf("qop = %q, want %q", got["qop"], "auth")
	}
}

func TestParseDigestParams_Empty(t *testing.T) {
	got := parseDigestParams("")
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestParseQOP_Auth(t *testing.T) {
	if got := parseQOP("auth,auth-int"); got != "auth" {
		t.Errorf("parseQOP = %q, want %q", got, "auth")
	}
}

func TestParseQOP_NoAuth(t *testing.T) {
	if got := parseQOP("auth-int"); got != "" {
		t.Errorf("parseQOP = %q, want empty", got)
	}
}

func TestMd5Hex(t *testing.T) {
	// MD5("hello") = 5d41402abc4b2a76b9719d911017c592
	got := md5Hex("hello")
	want := "5d41402abc4b2a76b9719d911017c592"
	if got != want {
		t.Errorf("md5Hex(\"hello\") = %q, want %q", got, want)
	}
}

func TestNewCnonceReturnsErrorWhenRandomFails(t *testing.T) {
	prev := digestRandRead
	digestRandRead = func([]byte) (int, error) {
		return 0, errors.New("entropy unavailable")
	}
	defer func() { digestRandRead = prev }()

	if _, err := newCnonce(); err == nil {
		t.Fatal("expected random failure to be returned")
	}
}

func TestDigestTransport_NonDigestChallenge(t *testing.T) {
	// Server returns 401 with Basic auth — DigestTransport should return the response as-is.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	dt := NewDigestTransport("user", "pass", nil)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := dt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestDigestTransport_SuccessNoAuth(t *testing.T) {
	// Server returns 200 — no auth needed.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dt := NewDigestTransport("user", "pass", nil)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := dt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDigestTransport_DigestChallenge(t *testing.T) {
	// Server returns 401 with Digest, then accepts on retry.
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if call == 1 {
			w.Header().Set("WWW-Authenticate",
				`Digest realm="fritz.box", nonce="testnonce123", qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Verify Authorization header is present.
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("missing Authorization header on retry")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dt := NewDigestTransport("admin", "secret", nil)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := dt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if call != 2 {
		t.Errorf("expected 2 server calls, got %d", call)
	}
}

// ─── SID Auth helper tests ───

func TestCalcMD5Response(t *testing.T) {
	// Known challenge-response pair for verification.
	challenge := "1234567z"
	password := "äbc"
	resp, err := calcMD5Response(challenge, password)
	if err != nil {
		t.Fatalf("calcMD5Response: %v", err)
	}
	// The response should be "challenge-md5hash".
	if len(resp) == 0 {
		t.Fatal("calcMD5Response returned empty string")
	}
	if resp[:9] != "1234567z-" {
		t.Errorf("response should start with challenge, got %q", resp[:9])
	}
	// Hash length: 32 hex chars
	hash := resp[9:]
	if len(hash) != 32 {
		t.Errorf("MD5 hash length = %d, want 32", len(hash))
	}
}

func TestCalcPBKDF2Response_InvalidFormat(t *testing.T) {
	_, err := calcPBKDF2Response("invalid", "pass")
	if err == nil {
		t.Error("expected error for invalid PBKDF2 challenge format")
	}
}

func TestCalcPBKDF2Response_Valid(t *testing.T) {
	// Construct a valid PBKDF2 challenge.
	// Format: 2$<iter1>$<salt1hex>$<iter2>$<salt2hex>
	challenge := "2$10000$deadbeef01020304$10000$cafebabe05060708"
	resp, err := calcPBKDF2Response(challenge, "testpass")
	if err != nil {
		t.Fatalf("calcPBKDF2Response: %v", err)
	}
	// Response format: salt2hex$hash
	if len(resp) == 0 {
		t.Fatal("empty response")
	}
	// Should start with salt2 hex.
	if resp[:16] != "cafebabe05060708" {
		t.Errorf("response should start with salt2, got %q", resp[:16])
	}
	// Check separator.
	if resp[16] != '$' {
		t.Errorf("expected $ separator at position 16, got %c", resp[16])
	}
	// Hash is 64 hex chars (32 bytes SHA-256).
	hash := resp[17:]
	if len(hash) != 64 {
		t.Errorf("PBKDF2 hash length = %d, want 64", len(hash))
	}
}
