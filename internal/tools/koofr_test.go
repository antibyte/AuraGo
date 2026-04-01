package tools

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKoofrSuccessContentResponseEscapesStructuredContent(t *testing.T) {
	raw := []byte("line1\n\"quoted\"\tend")
	result := koofrSuccessContentResponse(raw)
	if !strings.HasPrefix(result, "Tool Output: ") {
		t.Fatalf("unexpected prefix: %q", result)
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(strings.TrimPrefix(result, "Tool Output: ")), &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if parsed["status"] != "success" {
		t.Fatalf("status = %q, want success", parsed["status"])
	}
	if parsed["content"] != string(raw) {
		t.Fatalf("content = %q, want %q", parsed["content"], string(raw))
	}
}

func TestDoKoofrRequestRejectsOversizeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("x"), int(maxHTTPResponseSize+1)))
	}))
	defer srv.Close()

	_, err := doKoofrRequest(http.MethodGet, srv.URL, "user", "pass", "", nil)
	if err == nil {
		t.Fatal("expected oversize response error")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}
