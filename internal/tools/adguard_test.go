package tools

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetAdGuardClientInitializesOnce(t *testing.T) {
	// Note: We cannot store the originalOnce sync.Once value directly due to Go's
	// noCopy mechanism. Instead, we use a pointer to track and restore state.
	originalClient := adguardClient
	originalFactory := adguardClientFactory
	defer func() {
		adguardClient = originalClient
		adguardClientFactory = originalFactory
		// Note: sync.Once cannot be restored, but this is only a test cleanup
		// The test creates a fresh adguardClientOnce via re-initialization
	}()

	adguardClient = nil
	// Create a fresh sync.Once for testing by resetting the variable entirely
	adguardClientOnce = sync.Once{}
	var factoryCalls int32
	adguardClientFactory = func() *http.Client {
		atomic.AddInt32(&factoryCalls, 1)
		return &http.Client{Timeout: 15 * time.Second}
	}

	clients := make(chan *http.Client, 32)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			clients <- getAdGuardClient()
		}()
	}
	wg.Wait()
	close(clients)

	if got := atomic.LoadInt32(&factoryCalls); got != 1 {
		t.Fatalf("factory calls = %d, want 1", got)
	}

	var first *http.Client
	for client := range clients {
		if first == nil {
			first = client
			continue
		}
		if client != first {
			t.Fatal("expected all goroutines to receive the same shared client")
		}
	}
}

func TestAdGuardRequestRejectsOversizeResponse(t *testing.T) {
	adguardClient = nil                                  // reset so getAdGuardClient() re-initializes
	adguardClientOnce = sync.Once{}                      // reset Once so the next call actually runs the init

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("x"), int(maxHTTPResponseSize+1)))
	}))
	defer server.Close()

	_, _, err := adguardRequest(AdGuardConfig{URL: server.URL}, "GET", "/control/status", "")
	if err == nil {
		t.Fatal("expected oversize response error")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdgErrorEscapesStructuredContent(t *testing.T) {
	out := adgError("bad input: %s", "line1\n\"quoted\"")
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if parsed["status"] != "error" {
		t.Fatalf("status = %v, want error", parsed["status"])
	}
	if parsed["message"] != "bad input: line1\n\"quoted\"" {
		t.Fatalf("message = %q", parsed["message"])
	}
}

func TestAdgHTTPErrorEscapesBody(t *testing.T) {
	out := adgHTTPError(http.StatusBadRequest, []byte("body with \"quotes\""))
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if int(parsed["http_code"].(float64)) != http.StatusBadRequest {
		t.Fatalf("http_code = %v, want %d", parsed["http_code"], http.StatusBadRequest)
	}
	if parsed["message"] != "body with \"quotes\"" {
		t.Fatalf("message = %q", parsed["message"])
	}
}
