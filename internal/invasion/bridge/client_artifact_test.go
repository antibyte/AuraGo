package bridge

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEggClientMasterHTTPBaseURL(t *testing.T) {
	tests := []struct {
		name      string
		masterURL string
		want      string
	}{
		{name: "ws path", masterURL: "ws://127.0.0.1:8443/api/invasion/ws", want: "http://127.0.0.1:8443"},
		{name: "wss path", masterURL: "wss://aurago.example/api/invasion/ws", want: "https://aurago.example"},
		{name: "ws root", masterURL: "ws://127.0.0.1:8443", want: "http://127.0.0.1:8443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &EggClient{MasterURL: tt.masterURL}
			if got := c.masterHTTPBaseURL(); got != tt.want {
				t.Fatalf("masterHTTPBaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEggClientSendHostMessageSignsAndPostsMessage(t *testing.T) {
	sharedKey := strings.Repeat("a", 64)
	var seenPath string
	var seenBody string
	c := &EggClient{
		MasterURL: "https://aurago.example/api/invasion/ws",
		NestID:    "nest-1",
		EggID:     "egg-1",
		SharedKey: sharedKey,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			seenPath = req.URL.Path
			body, _ := io.ReadAll(req.Body)
			seenBody = string(body)
			if req.Header.Get("X-AuraGo-Nest-ID") != "nest-1" || req.Header.Get("X-AuraGo-Egg-ID") != "egg-1" {
				t.Fatalf("missing egg auth headers: %v", req.Header)
			}
			if req.Header.Get("X-AuraGo-Signature") == "" || req.Header.Get("X-AuraGo-Timestamp") == "" {
				t.Fatalf("missing signature headers: %v", req.Header)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"status":"stored","message_id":"msg-1","wakeup_allowed":true}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	result, err := c.SendHostMessage(context.Background(), EggHostMessage{
		Severity:        "info",
		Title:           "Report ready",
		Body:            "Created artifact report.txt",
		ArtifactIDs:     []string{"artifact-1"},
		WakeupRequested: true,
	})
	if err != nil {
		t.Fatalf("SendHostMessage: %v", err)
	}
	if result.MessageID != "msg-1" || !result.WakeupAllowed {
		t.Fatalf("result = %#v, want stored wakeup", result)
	}
	if seenPath != "/api/invasion/messages" {
		t.Fatalf("path = %q, want /api/invasion/messages", seenPath)
	}
	if !bytes.Contains([]byte(seenBody), []byte(`"artifact_ids":["artifact-1"]`)) {
		t.Fatalf("body missing artifact IDs: %s", seenBody)
	}
}

func TestEggClientUploadArtifactOffersAndUploadsFile(t *testing.T) {
	sharedKey := strings.Repeat("a", 64)
	seenOffer := false
	seenUpload := false
	c := &EggClient{
		MasterURL: "wss://aurago.example/api/invasion/ws",
		NestID:    "nest-1",
		EggID:     "egg-1",
		SharedKey: sharedKey,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/api/invasion/artifacts/offer":
				seenOffer = true
				if req.Header.Get("X-AuraGo-Signature") == "" {
					t.Fatalf("offer request is unsigned")
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"status":"ready","artifact_id":"artifact-1","upload_url":"/api/invasion/artifacts/upload/token-1","web_path":"/api/invasion/artifacts/artifact-1/download"}`)),
					Header:     make(http.Header),
				}, nil
			case "/api/invasion/artifacts/upload/token-1":
				seenUpload = true
				body, _ := io.ReadAll(req.Body)
				if string(body) != "hello" {
					t.Fatalf("upload body = %q, want hello", string(body))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"status":"completed","artifact_id":"artifact-1","size_bytes":5,"sha256":"abc"}`)),
					Header:     make(http.Header),
				}, nil
			default:
				t.Fatalf("unexpected request path: %s", req.URL.Path)
				return nil, nil
			}
		})},
	}

	result, err := c.UploadArtifact(context.Background(), EggArtifactUpload{
		Filename:     "report.txt",
		MIMEType:     "text/plain",
		ExpectedSize: 5,
		Reader:       strings.NewReader("hello"),
	})
	if err != nil {
		t.Fatalf("UploadArtifact: %v", err)
	}
	if !seenOffer || !seenUpload {
		t.Fatalf("seenOffer=%v seenUpload=%v, want both true", seenOffer, seenUpload)
	}
	if result.ArtifactID != "artifact-1" || result.WebPath == "" {
		t.Fatalf("result = %#v, want artifact metadata", result)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
