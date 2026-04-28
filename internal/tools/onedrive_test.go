package tools

import (
	"aurago/internal/testutil"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestOneDriveRequestRejectsOversizedResponseBody(t *testing.T) {
	oldClient := odHTTPClient
	t.Cleanup(func() { odHTTPClient = oldClient })

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("z", int(maxHTTPResponseSize)+1)))
	}))
	defer srv.Close()

	odHTTPClient = srv.Client()

	client := &OneDriveClient{AccessToken: "token"}
	_, _, err := client.request(http.MethodGet, srv.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestOneDriveUploadRejectsOversizedErrorBody(t *testing.T) {
	oldClient := odHTTPClient
	t.Cleanup(func() { odHTTPClient = oldClient })

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("q", int(maxHTTPResponseSize)+1)))
	}))
	defer srv.Close()

	serverURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}
	odHTTPClient = srv.Client()
	baseTransport := odHTTPClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	odHTTPClient.Transport = oneDriveRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = serverURL.Scheme
		clone.URL.Host = serverURL.Host
		clone.Host = serverURL.Host
		return baseTransport.RoundTrip(clone)
	})
	client := &OneDriveClient{AccessToken: "token"}

	result := client.uploadFile("folder/file.txt", "hello")

	var parsed map[string]string
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed["status"] != "error" {
		t.Fatalf("expected error status, got %q", parsed["status"])
	}
	if !strings.Contains(parsed["message"], "response body exceeds limit") {
		t.Fatalf("expected oversized body message, got %q", parsed["message"])
	}
}

type oneDriveRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f oneDriveRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
