// Package fritzbox provides a client for the AVM Fritz!Box home router.
// This file implements RFC 2617 HTTP Digest Authentication as a custom
// http.RoundTripper, because Go's net/http only supports Basic Auth natively.
package fritzbox

import (
	"crypto/md5" //nolint:gosec // MD5 is required by RFC 2617 Digest Auth, not a security choice
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

var digestRandRead = rand.Read

// DigestTransport wraps an http.RoundTripper and handles HTTP Digest Auth
// transparently for each request. Thread-safe: multiple goroutines may call
// RoundTrip concurrently (the nonce counter is protected by a mutex).
type DigestTransport struct {
	username  string
	password  string
	transport http.RoundTripper

	mu     sync.Mutex
	nc     int    // nonce count, incremented per request to the same nonce
	nonce  string // cached nonce from last 401 challenge
	realm  string
	opaque string
	qop    string
}

// NewDigestTransport creates a DigestTransport that authenticates requests
// using the given username and password.
func NewDigestTransport(username, password string, base http.RoundTripper) *DigestTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &DigestTransport{
		username:  username,
		password:  password,
		transport: base,
	}
}

// RoundTrip performs the HTTP request with Digest Auth.
// First attempt is unauthenticated; if a 401 is returned the challenge is parsed
// and the request is retried with an Authorization header.
func (d *DigestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone request so we can safely re-use it.
	reqCopy, err := cloneRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := d.transport.RoundTrip(reqCopy)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	// Drain and close the 401 body before retrying.
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	resp.Body.Close()

	// Parse WWW-Authenticate header.
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if !strings.HasPrefix(wwwAuth, "Digest ") {
		// Not digest – return original response.
		return resp, nil
	}

	d.mu.Lock()
	d.parseChallenge(wwwAuth)
	authHeader, err := d.buildAuthorization(req.Method, req.URL.RequestURI())
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}

	// Retry with auth header.
	retry, err := cloneRequest(req)
	if err != nil {
		return nil, err
	}
	retry.Header.Set("Authorization", authHeader)
	return d.transport.RoundTrip(retry)
}

// parseChallenge extracts Digest auth parameters from the WWW-Authenticate header.
func (d *DigestTransport) parseChallenge(header string) {
	// Strip "Digest " prefix.
	header = strings.TrimPrefix(header, "Digest ")
	params := parseDigestParams(header)
	d.realm = params["realm"]
	d.nonce = params["nonce"]
	d.opaque = params["opaque"]
	d.qop = parseQOP(params["qop"]) // pick "auth" if offered
	d.nc = 0
}

// buildAuthorization constructs the Authorization header value.
func (d *DigestTransport) buildAuthorization(method, uri string) (string, error) {
	d.nc++
	ncStr := fmt.Sprintf("%08x", d.nc)
	cnonce, err := newCnonce()
	if err != nil {
		return "", fmt.Errorf("digest_auth: generate cnonce: %w", err)
	}

	ha1 := md5Hex(d.username + ":" + d.realm + ":" + d.password) //nolint:gosec
	ha2 := md5Hex(method + ":" + uri)                            //nolint:gosec

	var response string
	if d.qop == "auth" {
		response = md5Hex(ha1 + ":" + d.nonce + ":" + ncStr + ":" + cnonce + ":auth:" + ha2) //nolint:gosec
	} else {
		response = md5Hex(ha1 + ":" + d.nonce + ":" + ha2) //nolint:gosec
	}

	parts := []string{
		fmt.Sprintf(`username="%s"`, d.username),
		fmt.Sprintf(`realm="%s"`, d.realm),
		fmt.Sprintf(`nonce="%s"`, d.nonce),
		fmt.Sprintf(`uri="%s"`, uri),
		fmt.Sprintf(`response="%s"`, response),
	}
	if d.qop == "auth" {
		parts = append(parts,
			`qop=auth`,
			fmt.Sprintf(`nc=%s`, ncStr),
			fmt.Sprintf(`cnonce="%s"`, cnonce),
		)
	}
	if d.opaque != "" {
		parts = append(parts, fmt.Sprintf(`opaque="%s"`, d.opaque))
	}
	return "Digest " + strings.Join(parts, ", "), nil
}

// ──────────────────────────── helpers ────────────────────────────

func md5Hex(s string) string { //nolint:gosec
	h := md5.Sum([]byte(s)) //nolint:gosec
	return hex.EncodeToString(h[:])
}

func newCnonce() (string, error) {
	b := make([]byte, 8)
	if _, err := digestRandRead(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// parseDigestParams parses comma-separated key="value" pairs.
func parseDigestParams(s string) map[string]string {
	out := make(map[string]string)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		k := strings.TrimSpace(part[:idx])
		v := strings.TrimSpace(part[idx+1:])
		v = strings.Trim(v, `"`)
		out[k] = v
	}
	return out
}

// parseQOP returns "auth" if the server offers it; otherwise empty.
func parseQOP(qop string) string {
	for _, q := range strings.Split(qop, ",") {
		if strings.TrimSpace(q) == "auth" {
			return "auth"
		}
	}
	return ""
}

// cloneRequest creates a shallow clone of r with a fresh body reader.
func cloneRequest(r *http.Request) (*http.Request, error) {
	clone := r.Clone(r.Context())
	if r.Body == nil || r.Body == http.NoBody {
		return clone, nil
	}
	// Read and re-set body so it can be re-sent.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("digest_auth: read body: %w", err)
	}
	r.Body.Close()
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	clone.Body = io.NopCloser(strings.NewReader(string(body)))
	return clone, nil
}
