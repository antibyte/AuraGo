package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type radioBrowserRoundTripper func(*http.Request) (*http.Response, error)

func (f radioBrowserRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func radioBrowserResponse(r *http.Request, status int, headers http.Header, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     headers,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}
}

func TestRadioBrowserProxyPinsAllowedMirrorRedirectAndSetsNosniff(t *testing.T) {
	originalFactory := newRadioBrowserPinnedHTTPClient
	t.Cleanup(func() { newRadioBrowserPinnedHTTPClient = originalFactory })
	var requested []string
	newRadioBrowserPinnedHTTPClient = func(rawURL string, _ time.Duration) (*http.Client, error) {
		requested = append(requested, rawURL)
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		return &http.Client{
			Transport: radioBrowserRoundTripper(func(r *http.Request) (*http.Response, error) {
				switch parsed.Hostname() {
				case "all.api.radio-browser.info":
					return radioBrowserResponse(r, http.StatusTemporaryRedirect, http.Header{"Location": []string{"https://de1.api.radio-browser.info/json/stations"}}, ""), nil
				case "de1.api.radio-browser.info":
					return radioBrowserResponse(r, http.StatusOK, http.Header{"Content-Type": []string{"text/html; charset=utf-8; x=+json"}}, `[]`), nil
				default:
					t.Fatalf("unapproved target %q reached transport", r.URL.Host)
					return nil, nil
				}
			}),
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}, nil
	}

	s := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/radio-browser/json/stations?name=fixture", nil)
	handleRadioBrowserProxy(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != `[]` {
		t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("missing JSON nosniff headers: %#v", rec.Header())
	}
	if len(requested) != 2 {
		t.Fatalf("pinned client URLs = %#v, want one client per mirror hop", requested)
	}
	if !strings.Contains(requested[0], "all.api.radio-browser.info") || !strings.Contains(requested[1], "de1.api.radio-browser.info") {
		t.Fatalf("redirect did not remain on official mirror set: %#v", requested)
	}
}

func TestRadioBrowserProxyRejectsForeignRedirectAndFailsOver(t *testing.T) {
	originalFactory := newRadioBrowserPinnedHTTPClient
	t.Cleanup(func() { newRadioBrowserPinnedHTTPClient = originalFactory })
	var requested []string
	newRadioBrowserPinnedHTTPClient = func(rawURL string, _ time.Duration) (*http.Client, error) {
		requested = append(requested, rawURL)
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		return &http.Client{
			Transport: radioBrowserRoundTripper(func(r *http.Request) (*http.Response, error) {
				if parsed.Hostname() == "all.api.radio-browser.info" {
					return radioBrowserResponse(r, http.StatusFound, http.Header{"Location": []string{"https://attacker.example/json/stations"}}, ""), nil
				}
				return radioBrowserResponse(r, http.StatusOK, http.Header{"Content-Type": []string{"application/json"}}, `["mirror"]`), nil
			}),
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}, nil
	}

	s := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rec := httptest.NewRecorder()
	handleRadioBrowserProxy(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/radio-browser/json/stations", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != `["mirror"]` {
		t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
	}
	if len(requested) != 2 || strings.Contains(strings.Join(requested, " "), "attacker.example") {
		t.Fatalf("foreign redirect was followed or failover stopped: %#v", requested)
	}
}
