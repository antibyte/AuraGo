package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestPaperlessServer creates a mock Paperless-ngx API server for testing.
func newTestPaperlessServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, PaperlessConfig) {
	t.Helper()
	srv := httptest.NewServer(handler)
	return srv, PaperlessConfig{URL: srv.URL, APIToken: "test-token"}
}

func TestPaperlessSearch_WithResults(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token test-token" {
			t.Error("missing or wrong Authorization header")
		}
		if !strings.Contains(r.URL.Path, "/api/documents/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "invoice" {
			t.Errorf("expected query=invoice, got %s", r.URL.Query().Get("query"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count": 2,
			"results": []map[string]interface{}{
				{"id": 1, "title": "Invoice 2025", "created": "2025-01-15", "tags": []interface{}{1, 2}},
				{"id": 2, "title": "Invoice 2024", "created": "2024-06-10", "tags": []interface{}{}},
			},
		})
	})
	defer srv.Close()

	result := PaperlessSearch(cfg, "invoice", "", "", "", 10)

	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed.Status != "success" {
		t.Errorf("expected success, got %s: %s", parsed.Status, parsed.Message)
	}
	if !strings.Contains(parsed.Message, "2 documents") {
		t.Errorf("expected 2 documents in message, got: %s", parsed.Message)
	}
}

func TestPaperlessSearch_Empty(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":   0,
			"results": []interface{}{},
		})
	})
	defer srv.Close()

	result := PaperlessSearch(cfg, "nonexistent", "", "", "", 0)

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "success" {
		t.Errorf("expected success, got %s", parsed.Status)
	}
	if !strings.Contains(parsed.Message, "0 documents") {
		t.Errorf("expected 0 documents in message, got: %s", parsed.Message)
	}
}

func TestPaperlessGet(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/documents/42/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      42,
			"title":   "Test Document",
			"content": "This is the document content.",
			"tags":    []interface{}{1},
		})
	})
	defer srv.Close()

	result := PaperlessGet(cfg, "42")

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "success" {
		t.Errorf("expected success, got %s: %s", parsed.Status, parsed.Message)
	}
}

func TestPaperlessGet_MissingID(t *testing.T) {
	result := PaperlessGet(PaperlessConfig{URL: "http://localhost", APIToken: "x"}, "")

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "error" {
		t.Errorf("expected error for missing ID, got %s", parsed.Status)
	}
}

func TestPaperlessDownload(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      10,
			"title":   "Report",
			"content": "Full text content of the report document.",
		})
	})
	defer srv.Close()

	result := PaperlessDownload(cfg, "10")

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "success" {
		t.Errorf("expected success, got %s: %s", parsed.Status, parsed.Message)
	}
	if !strings.Contains(parsed.Message, "Report") {
		t.Errorf("expected document title in message, got: %s", parsed.Message)
	}
}

func TestPaperlessUpload_Success(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "post_document") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("expected multipart/form-data content type, got: %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`"OK"`))
	})
	defer srv.Close()

	result := PaperlessUpload(cfg, "My Document", "Document content here", "invoice,2025", "", "")

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "success" {
		t.Errorf("expected success, got %s: %s", parsed.Status, parsed.Message)
	}
}

func TestPaperlessUpload_MissingContent(t *testing.T) {
	result := PaperlessUpload(PaperlessConfig{URL: "http://localhost", APIToken: "x"}, "Title", "", "", "", "")

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "error" {
		t.Errorf("expected error for missing content, got %s", parsed.Status)
	}
}

func TestPaperlessDelete(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	result := PaperlessDelete(cfg, "5")

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "success" {
		t.Errorf("expected success, got %s: %s", parsed.Status, parsed.Message)
	}
}

func TestPaperlessListTags(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count": 3,
			"results": []map[string]interface{}{
				{"id": 1, "name": "invoice"},
				{"id": 2, "name": "receipt"},
				{"id": 3, "name": "contract"},
			},
		})
	})
	defer srv.Close()

	result := PaperlessListTags(cfg)

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "success" {
		t.Errorf("expected success, got %s: %s", parsed.Status, parsed.Message)
	}
	if !strings.Contains(parsed.Message, "3") {
		t.Errorf("expected 3 tags in message, got: %s", parsed.Message)
	}
}

func TestPaperlessAuth_Failure(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":"Invalid token."}`))
	})
	defer srv.Close()

	result := PaperlessSearch(cfg, "test", "", "", "", 0)

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "error" {
		t.Errorf("expected error for 401, got %s", parsed.Status)
	}
	if !strings.Contains(parsed.Message, "401") {
		t.Errorf("expected HTTP 401 in error message, got: %s", parsed.Message)
	}
}

func TestPaperlessServer_Error(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`Internal Server Error`))
	})
	defer srv.Close()

	result := PaperlessGet(cfg, "1")

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "error" {
		t.Errorf("expected error for 500, got %s", parsed.Status)
	}
	if !strings.Contains(parsed.Message, "500") {
		t.Errorf("expected HTTP 500 in error message, got: %s", parsed.Message)
	}
}

func TestPaperlessMissingConfig(t *testing.T) {
	result := PaperlessSearch(PaperlessConfig{}, "test", "", "", "", 0)

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "error" {
		t.Errorf("expected error for empty config, got %s", parsed.Status)
	}
}

func TestPaperlessUpdate(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":7,"title":"Updated Title"}`))
	})
	defer srv.Close()

	result := PaperlessUpdate(cfg, "7", "Updated Title", "", "", "")

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "success" {
		t.Errorf("expected success, got %s: %s", parsed.Status, parsed.Message)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"", "document"},
		{"normal.pdf", "normal.pdf"},
		{"a/b\\c:d", "a_b_c_d"},
		{strings.Repeat("x", 150), strings.Repeat("x", 100)},
	}
	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
