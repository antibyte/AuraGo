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

func TestExecuteOneDriveReadOnlyBlocksDirectMutations(t *testing.T) {
	client := &OneDriveClient{
		AccessToken: "token",
		ReadOnly:    true,
	}

	cases := []struct {
		name        string
		operation   string
		path        string
		destination string
		content     string
	}{
		{name: "upload", operation: "upload", path: "/folder/file.txt", content: "content"},
		{name: "write", operation: "write", path: "/folder/file.txt", content: "content"},
		{name: "mkdir", operation: "mkdir", path: "/folder"},
		{name: "delete", operation: "delete", path: "/folder/file.txt"},
		{name: "move", operation: "move", path: "/folder/file.txt", destination: "/archive/file.txt"},
		{name: "copy", operation: "copy", path: "/folder/file.txt", destination: "/archive/file.txt"},
		{name: "share", operation: "share", path: "/folder/file.txt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := client.ExecuteOneDrive(tc.operation, tc.path, tc.destination, tc.content, 0)
			var parsed map[string]string
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("failed to parse result: %v", err)
			}
			if parsed["status"] != "error" {
				t.Fatalf("status = %q, want error; result=%s", parsed["status"], result)
			}
			if !strings.Contains(parsed["message"], "read-only mode") {
				t.Fatalf("message = %q, want read-only mode guidance", parsed["message"])
			}
		})
	}
}

type oneDriveRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f oneDriveRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
