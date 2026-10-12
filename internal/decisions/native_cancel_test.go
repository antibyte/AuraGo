package decisions

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeCancelUsesAuthenticatedRequestIDEndpoint(t *testing.T) {
	token := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(token, []byte("secret-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	session := &nativeRuntimeSession{
		baseURL: "http://127.0.0.1:18765", token: token,
		client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			called = true
			if request.Method != http.MethodPost || request.URL.Path != "/cancel" {
				t.Errorf("cancel request = %s %s", request.Method, request.URL.Path)
			}
			if request.Header.Get("Authorization") != "Bearer secret-token" {
				t.Errorf("Authorization header = %q", request.Header.Get("Authorization"))
			}
			var body struct {
				RequestID string `json:"request_id"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode cancel request: %v", err)
			} else if body.RequestID != "0123456789abcdef0123456789abcdef" {
				t.Errorf("request_id = %q", body.RequestID)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"cancelled":true}`)), Request: request}, nil
		})},
	}
	session.cancelRequest("0123456789abcdef0123456789abcdef")
	if !called {
		t.Fatal("native cancel endpoint was not called")
	}
}
