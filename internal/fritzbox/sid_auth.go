// Package fritzbox – SID (Session-ID) authentication via /login_sid.lua.
// Fritz!OS supports two challenge-response schemes:
//   - PBKDF2 (modern, Fritz!OS 7.24+) – challenge starts with "2$"
//   - MD5/UTF-16LE (legacy, all versions)
//
// The SID returned here is used for AHA-HTTP (Smart Home) requests.
package fritzbox

import (
	"bytes"
	"crypto/md5" //nolint:gosec // MD5 is required by the legacy Fritz!Box auth protocol
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"crypto/sha256"

	"golang.org/x/crypto/pbkdf2"
)

const (
	sidLength       = 16
	sidUnauthorized = "0000000000000000"
	sidEndpoint     = "/login_sid.lua?version=2"
)

// SIDAuth manages Fritz!OS session authentication via login_sid.lua.
type SIDAuth struct {
	baseURL  string
	username string
	password string
	client   *http.Client

	mu        sync.Mutex
	sid       string
	expiresAt time.Time
}

// newSIDAuth creates an SIDAuth for the given host.
func newSIDAuth(baseURL, username, password string, transport http.RoundTripper) *SIDAuth {
	return &SIDAuth{
		baseURL:  baseURL,
		username: username,
		password: password,
		client: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
		},
	}
}

// SID returns a valid session ID, refreshing if expired or missing.
func (a *SIDAuth) SID() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.sid != "" && a.sid != sidUnauthorized && time.Now().Before(a.expiresAt) {
		return a.sid, nil
	}
	return a.login()
}

// Invalidate forces re-login on next SID() call.
func (a *SIDAuth) Invalidate() {
	a.mu.Lock()
	a.sid = ""
	a.mu.Unlock()
}

// Logout ends the Fritz!OS session.
func (a *SIDAuth) Logout() {
	a.mu.Lock()
	sid := a.sid
	a.sid = ""
	a.mu.Unlock()

	if sid != "" && sid != sidUnauthorized {
		logoutURL := fmt.Sprintf("%s%s&sid=%s&logout=1", a.baseURL, sidEndpoint, url.QueryEscape(sid))
		req, err := http.NewRequest(http.MethodGet, logoutURL, nil)
		if err == nil {
			resp, _ := a.client.Do(req)
			if resp != nil {
				io.Copy(io.Discard, resp.Body) //nolint:errcheck
				resp.Body.Close()
			}
		}
	}
}

// GetWithSID performs an authenticated GET request using the current SID.
// The SID is sent both as a query parameter and as a Cookie header, because
// Fritz!Box endpoints like download.lua authenticate via cookie rather than
// the query parameter.
func (a *SIDAuth) GetWithSID(rawURL string) (*http.Response, error) {
	sid, err := a.SID()
	if err != nil {
		return nil, fmt.Errorf("get SID for download: %w", err)
	}
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	req, err := http.NewRequest(http.MethodGet, rawURL+sep+"sid="+sid, nil)
	if err != nil {
		return nil, fmt.Errorf("build SID request: %w", err)
	}
	// Send SID as cookie too — download.lua and similar endpoints check the cookie,
	// not (only) the query parameter.
	req.AddCookie(&http.Cookie{Name: "sid", Value: sid})
	return a.client.Do(req)
}

// login performs the full challenge-response login and stores the resulting SID.
// Caller must hold a.mu.
func (a *SIDAuth) login() (string, error) {
	challenge, blockSecs, isPBKDF2, err := a.fetchChallenge()
	if err != nil {
		return "", fmt.Errorf("fritzbox sid: fetch challenge: %w", err)
	}

	if blockSecs > 0 {
		time.Sleep(time.Duration(blockSecs) * time.Second)
	}

	var response string
	if isPBKDF2 {
		response, err = calcPBKDF2Response(challenge, a.password)
	} else {
		response, err = calcMD5Response(challenge, a.password)
	}
	if err != nil {
		return "", fmt.Errorf("fritzbox sid: compute response: %w", err)
	}

	sid, err := a.sendResponse(response)
	if err != nil {
		return "", fmt.Errorf("fritzbox sid: send response: %w", err)
	}
	if sid == sidUnauthorized {
		return "", fmt.Errorf("fritzbox sid: authentication failed – check username/password")
	}

	a.sid = sid
	a.expiresAt = time.Now().Add(19 * time.Minute) // Fritz!OS sessions expire after 20 min of inactivity
	return sid, nil
}

// challengeResponse is the XML schema returned by /login_sid.lua.
type challengeResponse struct {
	XMLName   xml.Name `xml:"SessionInfo"`
	SID       string   `xml:"SID"`
	Challenge string   `xml:"Challenge"`
	BlockTime int      `xml:"BlockTime"`
}

func (a *SIDAuth) fetchChallenge() (challenge string, blockSecs int, isPBKDF2 bool, err error) {
	resp, err := a.client.Get(a.baseURL + sidEndpoint)
	if err != nil {
		return "", 0, false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result challengeResponse
	if err := xml.Unmarshal(body, &result); err != nil {
		return "", 0, false, fmt.Errorf("parse challenge xml: %w", err)
	}

	isPBKDF2 = strings.HasPrefix(result.Challenge, "2$")
	return result.Challenge, result.BlockTime, isPBKDF2, nil
}

func (a *SIDAuth) sendResponse(response string) (string, error) {
	form := url.Values{
		"response": {response},
		"username": {a.username},
	}
	resp, err := a.client.PostForm(a.baseURL+sidEndpoint, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result challengeResponse
	if err := xml.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse sid response xml: %w", err)
	}
	return result.SID, nil
}

// ──────────────────────────────────────────────
// Challenge-response calculation helpers
// ──────────────────────────────────────────────

// calcPBKDF2Response computes the modern PBKDF2-based response.
// Challenge format: "2$<iter1>$<salt1hex>$<iter2>$<salt2hex>"
func calcPBKDF2Response(challenge, password string) (string, error) {
	parts := strings.Split(challenge, "$")
	if len(parts) != 5 {
		return "", fmt.Errorf("unexpected PBKDF2 challenge format: %q", challenge)
	}
	iter1, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid iter1: %w", err)
	}
	salt1, err := hex.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid salt1: %w", err)
	}
	iter2, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", fmt.Errorf("invalid iter2: %w", err)
	}
	salt2, err := hex.DecodeString(parts[4])
	if err != nil {
		return "", fmt.Errorf("invalid salt2: %w", err)
	}

	hash1 := pbkdf2.Key([]byte(password), salt1, iter1, 32, sha256.New)
	hash2 := pbkdf2.Key(hash1, salt2, iter2, 32, sha256.New)
	return fmt.Sprintf("%s$%s", parts[4], hex.EncodeToString(hash2)), nil
}

// calcMD5Response computes the legacy MD5/UTF-16LE response.
// The Fritz!Box expects: MD5(challenge "-" password) encoded as UTF-16 LE.
func calcMD5Response(challenge, password string) (string, error) {
	combined := challenge + "-" + password
	runes := []rune(combined)
	utf16Words := utf16.Encode(runes)

	var buf bytes.Buffer
	for _, w := range utf16Words {
		if err := binary.Write(&buf, binary.LittleEndian, w); err != nil {
			return "", err
		}
	}

	hash := md5.Sum(buf.Bytes()) //nolint:gosec
	return challenge + "-" + hex.EncodeToString(hash[:]), nil
}
