package fritzbox

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBuildWebURLHonorsConfiguredWebPort(t *testing.T) {
	if got := buildWebURL("fritz.box", false, 0); got != "http://fritz.box" {
		t.Fatalf("default HTTP web URL = %q, want http://fritz.box", got)
	}
	if got := buildWebURL("fritz.box", true, 0); got != "https://fritz.box" {
		t.Fatalf("default HTTPS web URL = %q, want https://fritz.box", got)
	}
	if got := buildWebURL("fritz.box", true, 4443); got != "https://fritz.box:4443" {
		t.Fatalf("custom HTTPS web URL = %q, want https://fritz.box:4443", got)
	}
}

func TestTR064ClientAllowsSelfSignedTLSWhenConfigured(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTR064Client(srv.URL, "", "", time.Second, true)
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		t.Fatalf("self-signed TLS request with insecure_skip_verify should succeed: %v", err)
	}
	resp.Body.Close()
}

func TestAHAClientUsesConfiguredWebPortForCommands(t *testing.T) {
	var gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		if r.URL.Path != "/webservices/homeautoswitch.lua" {
			t.Fatalf("path = %q, want /webservices/homeautoswitch.lua", r.URL.Path)
		}
		if r.URL.Query().Get("switchcmd") != "getswitchlist" {
			t.Fatalf("switchcmd = %q, want getswitchlist", r.URL.Query().Get("switchcmd"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test URL: %v", err)
	}
	host := strings.Split(u.Host, ":")[0]
	port := strings.Split(u.Host, ":")[1]
	baseURL := buildWebURL(host, false, mustAtoi(t, port))
	client := newAHAClient(baseURL, "", "", time.Second, false)
	client.sid.sid = "0123456789abcdef"
	client.sid.expiresAt = time.Now().Add(time.Hour)

	if _, err := client.Command("", "getswitchlist", nil); err != nil {
		t.Fatalf("Command: %v", err)
	}
	if gotHost != u.Host {
		t.Fatalf("request host = %q, want %q", gotHost, u.Host)
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	v, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", s, err)
	}
	return v
}
