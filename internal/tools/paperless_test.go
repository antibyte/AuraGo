package tools

import (
	"aurago/internal/testutil"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newTestPaperlessServer creates a mock Paperless-ngx API server for testing.
func newTestPaperlessServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, PaperlessConfig) {
	t.Helper()
	srv := testutil.NewHTTPServer(t, handler)
	return srv, PaperlessConfig{URL: srv.URL, APIToken: "test-token"}
}

func TestPaperlessSearch_WithResults(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token test-token" {
			t.Error("missing or wrong Authorization header")
		}
		switch {
		case strings.Contains(r.URL.Path, "/api/tags/"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count": 2,
				"results": []map[string]interface{}{
					{"id": 1, "name": "invoice"},
					{"id": 2, "name": "receipt"},
				},
			})
		case strings.Contains(r.URL.Path, "/api/documents/"):
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
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
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
		if strings.Contains(r.URL.Path, "/api/documents/") && r.URL.Query().Get("page_size") != "25" {
			t.Errorf("page_size = %q, want 25", r.URL.Query().Get("page_size"))
		}
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
	const taskID = "6b7c8d9e-0000-4000-8000-000000000001"
	posted := false
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/api/tags/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":   1,
				"results": []map[string]interface{}{{"id": 1, "name": "invoice"}},
			})
		case strings.Contains(r.URL.Path, "/api/correspondents/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":   1,
				"results": []map[string]interface{}{{"id": 8, "name": "Other"}},
			})
		case strings.Contains(r.URL.Path, "post_document"):
			posted = true
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			tags, correspondent, filename := readPaperlessUpload(t, r)
			if strings.Join(tags, ",") != "1,4" {
				t.Errorf("tags = %#v, want IDs 1 and 4", tags)
			}
			if correspondent != "7" {
				t.Errorf("correspondent = %q, want numeric ID 7", correspondent)
			}
			if !strings.HasSuffix(filename, ".txt") {
				t.Errorf("filename = %q, want .txt", filename)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%q", taskID)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	})
	defer srv.Close()

	result := PaperlessUpload(cfg, "My Document", "Document content here", "invoice,4", "7", "")
	if !posted {
		t.Fatal("upload did not POST")
	}

	var parsed struct {
		Status  string         `json:"status"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %s, message = %s", parsed.Status, parsed.Message)
	}
	if !strings.Contains(parsed.Message, "Processing is still running") {
		t.Fatalf("message = %s", parsed.Message)
	}
	if parsed.Data["task_id"] != taskID {
		t.Fatalf("task_id = %#v", parsed.Data["task_id"])
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

func TestPaperlessServer_OversizedErrorBody(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(strings.Repeat("x", int(maxHTTPResponseSize)+1)))
	})
	defer srv.Close()

	result := PaperlessGet(cfg, "1")

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "error" {
		t.Fatalf("expected error, got %s", parsed.Status)
	}
	if !strings.Contains(parsed.Message, "response body exceeds limit") {
		t.Fatalf("expected oversized body message, got: %s", parsed.Message)
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
	gets, patches := 0, 0
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			gets++
		case http.MethodPatch:
			patches++
		default:
			t.Errorf("method = %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":7,"title":"Updated Title"}`))
	})
	defer srv.Close()

	result := PaperlessUpdate(cfg, "7", "Updated Title", "", "", "")
	if gets != 0 || patches != 1 {
		t.Fatalf("gets = %d, patches = %d, want one title PATCH", gets, patches)
	}

	var parsed FSResult
	json.Unmarshal([]byte(result), &parsed)
	if parsed.Status != "success" {
		t.Errorf("expected success, got %s: %s", parsed.Status, parsed.Message)
	}
}

func TestPaperlessDirectMutationsRespectReadOnly(t *testing.T) {
	cfg := PaperlessConfig{URL: "https://paperless.example", APIToken: "token", ReadOnly: true}

	tests := map[string]string{
		"upload": PaperlessUpload(cfg, "title", "content", "", "", ""),
		"update": PaperlessUpdate(cfg, "7", "new title", "", "", ""),
		"delete": PaperlessDelete(cfg, "7"),
	}

	for name, got := range tests {
		t.Run(name, func(t *testing.T) {
			var parsed FSResult
			if err := json.Unmarshal([]byte(got), &parsed); err != nil {
				t.Fatalf("parse result: %v, raw=%s", err, got)
			}
			if parsed.Status != "error" || !strings.Contains(strings.ToLower(parsed.Message), "read-only") {
				t.Fatalf("expected read-only error, got %#v", parsed)
			}
		})
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

func readPaperlessUpload(t *testing.T, r *http.Request) (tags []string, correspondent, filename string) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("content type %q: %v", r.Header.Get("Content-Type"), err)
	}
	reader := multipart.NewReader(r.Body, params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return tags, correspondent, filename
		}
		if err != nil {
			t.Fatalf("multipart: %v", err)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("part: %v", err)
		}
		switch part.FormName() {
		case "tags":
			tags = append(tags, string(body))
		case "correspondent":
			correspondent = string(body)
		case "document":
			filename = part.FileName()
		}
	}
}

func TestPaperlessUploadUnknownTagDoesNotPost(t *testing.T) {
	posted := false
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "post_document") {
			posted = true
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":   1,
			"results": []map[string]interface{}{{"id": 1, "name": "invoice"}},
		})
	})
	defer srv.Close()

	result := PaperlessUpload(cfg, "Notes", "hello", "missing", "", "")
	if posted {
		t.Fatal("unknown tag still posted")
	}
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "error" || !strings.Contains(parsed.Message, "not found") || strings.Contains(parsed.Message, "incomplete") {
		t.Fatalf("result = %+v", parsed)
	}
}

func TestPaperlessSearchExactFiltersAndLimit(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/api/tags/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count": 2,
				"results": []map[string]interface{}{
					{"id": 1, "name": "invoice"},
					{"id": 2, "name": "receipt"},
				},
			})
		case strings.Contains(r.URL.Path, "/api/correspondents/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":   1,
				"results": []map[string]interface{}{{"id": 4, "name": "Stadtwerke"}},
			})
		case strings.Contains(r.URL.Path, "/api/document_types/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":   1,
				"results": []map[string]interface{}{{"id": 9, "name": "Bill"}},
			})
		case strings.Contains(r.URL.Path, "/api/documents/"):
			query := r.URL.Query()
			if query.Get("query") != "invoice" || query.Get("page_size") != "50" {
				t.Errorf("query = %s page_size = %s", query.Get("query"), query.Get("page_size"))
			}
			if strings.Contains(r.URL.RawQuery, "icontains") {
				t.Errorf("substring filter in %s", r.URL.RawQuery)
			}
			gotTags := query["tags__id__all"]
			if len(gotTags) != 2 || gotTags[0] != "1" || gotTags[1] != "2" {
				t.Errorf("tags = %#v", gotTags)
			}
			if query.Get("correspondent__id") != "4" || query.Get("document_type__id") != "9" {
				t.Errorf("correspondent = %s document_type = %s", query.Get("correspondent__id"), query.Get("document_type__id"))
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":   80,
				"results": []map[string]interface{}{{"id": 1, "title": "Invoice", "tags": []interface{}{float64(1)}}},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	defer srv.Close()

	result := PaperlessSearch(cfg, "invoice", "invoice,receipt", "Stadtwerke", "Bill", 1000)
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "success" || !strings.Contains(parsed.Message, "Showing 1 of 80 documents") {
		t.Fatalf("result = %+v", parsed)
	}
}

func TestPaperlessSearchUnknownNameIsExact(t *testing.T) {
	documents := 0
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/documents/") {
			documents++
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":   1,
			"results": []map[string]interface{}{{"id": 4, "name": "Stadtwerke"}},
		})
	})
	defer srv.Close()

	result := PaperlessSearch(cfg, "invoice", "", "Stadt", "", 10)
	if documents != 0 {
		t.Fatalf("documents requests = %d", documents)
	}
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "error" || !strings.Contains(parsed.Message, "not found") || strings.Contains(parsed.Message, "incomplete") {
		t.Fatalf("result = %+v", parsed)
	}
}

func TestPaperlessSearchFollowsNextPage(t *testing.T) {
	var srv *httptest.Server
	var cfg PaperlessConfig
	srv, cfg = newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/api/tags/"):
			if r.URL.Query().Get("page") == "2" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"count":   2,
					"results": []map[string]interface{}{{"id": 6, "name": "late"}},
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":   2,
				"next":    srv.URL + "/api/tags/?page=2&page_size=200",
				"results": []map[string]interface{}{{"id": 1, "name": "early"}},
			})
		case strings.Contains(r.URL.Path, "/api/documents/"):
			if got := r.URL.Query()["tags__id__all"]; len(got) != 1 || got[0] != "6" {
				t.Errorf("tags = %#v", got)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"count": 0, "results": []interface{}{}})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	defer srv.Close()

	result := PaperlessSearch(cfg, "", "late", "", "", 10)
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "success" {
		t.Fatalf("result = %+v", parsed)
	}
}

func TestPaperlessSearchIncompleteCatalog(t *testing.T) {
	var srv *httptest.Server
	pages := 0
	documents := 0
	var cfg PaperlessConfig
	srv, cfg = newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/api/documents/") {
			documents++
			return
		}
		pages++
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":   10000,
			"next":    fmt.Sprintf("%s/api/tags/?page=%d&page_size=200", srv.URL, pages+1),
			"results": []map[string]interface{}{{"id": pages, "name": fmt.Sprintf("other-%d", pages)}},
		})
	})
	defer srv.Close()

	result := PaperlessSearch(cfg, "", "missing", "", "", 10)
	if pages != paperlessMaxLookupPages || documents != 0 {
		t.Fatalf("pages = %d, documents = %d", pages, documents)
	}
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "error" || !strings.Contains(parsed.Message, "incomplete") {
		t.Fatalf("result = %+v", parsed)
	}
}

func TestPaperlessSearchRejectsForeignNext(t *testing.T) {
	documents := 0
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/documents/") {
			documents++
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":   2,
			"next":    "https://evil.example/api/tags/?page=2",
			"results": []map[string]interface{}{{"id": 1, "name": "early"}},
		})
	})
	defer srv.Close()

	result := PaperlessSearch(cfg, "", "missing", "", "", 10)
	if documents != 0 {
		t.Fatalf("documents requests = %d", documents)
	}
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "error" || !strings.Contains(parsed.Message, "incomplete") || strings.Contains(parsed.Message, "not found:") {
		t.Fatalf("result = %+v", parsed)
	}
}

func TestPaperlessUpdateAddsTags(t *testing.T) {
	var patched []int
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/api/tags/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count": 3,
				"results": []map[string]interface{}{
					{"id": 1, "name": "invoice"},
					{"id": 2, "name": "receipt"},
					{"id": 3, "name": "corrected"},
				},
			})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/documents/7/"):
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 7, "tags": []interface{}{float64(1), float64(2)}})
		case r.Method == http.MethodPatch:
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			raw, _ := body["tags"].([]interface{})
			for _, item := range raw {
				patched = append(patched, int(item.(float64)))
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":7}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	result := PaperlessUpdate(cfg, "7", "", "invoice,corrected", "", "")
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "success" || !strings.Contains(parsed.Message, "Added tags: corrected") || !strings.Contains(parsed.Message, "Existing tags were kept") {
		t.Fatalf("result = %+v", parsed)
	}
	if strings.Join(intsToStrings(patched), ",") != "1,2,3" {
		t.Fatalf("patched tags = %#v", patched)
	}
}

func TestPaperlessListTagsReportsTruncation(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":   5,
			"results": []map[string]interface{}{{"id": 1, "name": "invoice"}},
		})
	})
	defer srv.Close()

	result := PaperlessListTags(cfg)
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "success" || !strings.Contains(parsed.Message, "Showing 1 of 5 tags") {
		t.Fatalf("result = %+v", parsed)
	}
}

func TestPaperlessDownloadReportsTruncation(t *testing.T) {
	content := strings.Repeat("a", paperlessTextLimit+1)
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 10, "title": "Report", "content": content})
	})
	defer srv.Close()

	result := PaperlessDownload(cfg, "10")
	var parsed struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "success" || !strings.Contains(parsed.Message, "Report") || !strings.Contains(parsed.Message, "returned 8000 of 8001 characters") {
		t.Fatalf("message = %s", parsed.Message)
	}
	if !strings.Contains(parsed.Data, strings.Repeat("a", paperlessTextLimit)) || strings.Contains(parsed.Data, strings.Repeat("a", paperlessTextLimit+1)) || strings.Contains(parsed.Data, "...") {
		t.Fatalf("clipped text was not %d characters", paperlessTextLimit)
	}
}

func TestPaperlessTestConnectionCancelled(t *testing.T) {
	started := make(chan struct{})
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	})
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cfg.Context = ctx

	done := make(chan string, 1)
	go func() {
		done <- PaperlessTestConnection(cfg)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("connection test did not reach the server")
	}
	cancel()
	select {
	case result := <-done:
		if !strings.Contains(result, "error") {
			t.Fatalf("result = %s", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled context did not stop the connection test")
	}
}

func TestPaperlessSearchUsesNameFoundBeforeForeignNext(t *testing.T) {
	var srv *httptest.Server
	tagPages := 0
	documents := 0
	var cfg PaperlessConfig
	srv, cfg = newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/api/tags/"):
			tagPages++
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":   2,
				"next":    "https://evil.example/api/tags/?page=2",
				"results": []map[string]interface{}{{"id": 1, "name": "early"}},
			})
		case strings.Contains(r.URL.Path, "/api/documents/"):
			documents++
			if got := r.URL.Query()["tags__id__all"]; len(got) != 1 || got[0] != "1" {
				t.Errorf("tags = %#v", got)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"count": 0, "results": []interface{}{}})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	defer srv.Close()

	result := PaperlessSearch(cfg, "", "early", "", "", 10)
	if tagPages != 1 || documents != 1 {
		t.Fatalf("tag pages = %d, documents = %d", tagPages, documents)
	}
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "success" {
		t.Fatalf("result = %+v", parsed)
	}
}

func TestPaperlessSearchDisplayReadsOneTagPage(t *testing.T) {
	var srv *httptest.Server
	tagPages := 0
	var cfg PaperlessConfig
	srv, cfg = newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/api/tags/"):
			tagPages++
			if r.URL.Query().Get("page") == "2" {
				t.Errorf("display lookup followed next")
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":   3,
				"next":    srv.URL + "/api/tags/?page=2&page_size=200",
				"results": []map[string]interface{}{{"id": 1, "name": "invoice"}},
			})
		case strings.Contains(r.URL.Path, "/api/documents/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":   1,
				"results": []map[string]interface{}{{"id": 4, "title": "Invoice", "tags": []interface{}{float64(1), float64(99)}}},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	defer srv.Close()

	result := PaperlessSearch(cfg, "invoice", "", "", "", 10)
	if tagPages != 1 {
		t.Fatalf("tag pages = %d", tagPages)
	}
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "success" || !strings.Contains(result, "invoice") || !strings.Contains(result, "99") {
		t.Fatalf("result = %s", result)
	}
}

func TestPaperlessNextEndpointRejectsEncodedDots(t *testing.T) {
	base, err := url.Parse("https://paperless.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, next := range []string{
		"https://paperless.example/api/tags/%2e%2e/%2e%2e/secret/?page=2",
		"https://paperless.example/api/tags/%252e%252e/%252e%252e/secret/?page=2",
	} {
		if _, err := paperlessNextEndpoint(base, "tags", next); err == nil {
			t.Fatalf("accepted %s", next)
		}
	}
	got, err := paperlessNextEndpoint(base, "tags", "https://paperless.example/api/tags/?page=2&page_size=200")
	if err != nil {
		t.Fatal(err)
	}
	if got != "tags/?page=2&page_size=200" {
		t.Fatalf("next = %q", got)
	}
}

func TestPaperlessListTagsKeepsRowsWhenNextIsForeign(t *testing.T) {
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":   4,
			"next":    "https://evil.example/api/tags/?page=2",
			"results": []map[string]interface{}{{"id": 1, "name": "invoice"}},
		})
	})
	defer srv.Close()

	result := PaperlessListTags(cfg)
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "success" || !strings.Contains(parsed.Message, "Showing 1 of 4 tags") {
		t.Fatalf("result = %+v", parsed)
	}
}

func TestPaperlessGetReportsTruncation(t *testing.T) {
	content := strings.Repeat("b", paperlessTextLimit+1)
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 10, "title": "Report", "content": content})
	})
	defer srv.Close()

	result := PaperlessGet(cfg, "10")
	var parsed struct {
		Status  string                 `json:"status"`
		Message string                 `json:"message"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "success" || !strings.Contains(parsed.Message, "Report") || !strings.Contains(parsed.Message, "returned 8000 of 8001 characters") {
		t.Fatalf("message = %s", parsed.Message)
	}
	body, _ := parsed.Data["content"].(string)
	if !strings.Contains(body, strings.Repeat("b", paperlessTextLimit)) || strings.Contains(body, strings.Repeat("b", paperlessTextLimit+1)) || strings.Contains(body, "...") {
		t.Fatal("clipped get text had the wrong length")
	}
}

func TestPaperlessUpdateSkipsPatchWhenTagsAlreadyPresent(t *testing.T) {
	patches := 0
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/api/tags/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":   1,
				"results": []map[string]interface{}{{"id": 1, "name": "invoice"}},
			})
		case r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 7, "tags": []interface{}{float64(1)}})
		case r.Method == http.MethodPatch:
			patches++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":7}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	result := PaperlessUpdate(cfg, "7", "", "invoice", "", "")
	if patches != 0 {
		t.Fatalf("patches = %d", patches)
	}
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "success" || !strings.Contains(parsed.Message, "Existing tags were kept") || strings.Contains(parsed.Message, "Added tags") {
		t.Fatalf("result = %+v", parsed)
	}
}

func TestPaperlessUpdateRefusesMissingTagList(t *testing.T) {
	patches := 0
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/api/tags/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":   1,
				"results": []map[string]interface{}{{"id": 3, "name": "corrected"}},
			})
		case r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 7})
		case r.Method == http.MethodPatch:
			patches++
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	result := PaperlessUpdate(cfg, "7", "", "corrected", "", "")
	if patches != 0 {
		t.Fatalf("patches = %d", patches)
	}
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "error" || !strings.Contains(parsed.Message, "no tags were changed") {
		t.Fatalf("result = %+v", parsed)
	}
}

func TestPaperlessUpdateDropsUnknownCurrentTag(t *testing.T) {
	var patched []int
	srv, cfg := newTestPaperlessServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/api/tags/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count": 2,
				"results": []map[string]interface{}{
					{"id": 1, "name": "invoice"},
					{"id": 3, "name": "corrected"},
				},
			})
		case r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 7, "tags": []interface{}{float64(1), float64(99)}})
		case r.Method == http.MethodPatch:
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			raw, _ := body["tags"].([]interface{})
			for _, item := range raw {
				patched = append(patched, int(item.(float64)))
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":7}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	result := PaperlessUpdate(cfg, "7", "", "corrected", "", "")
	var parsed FSResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "success" {
		t.Fatalf("result = %+v", parsed)
	}
	if strings.Join(intsToStrings(patched), ",") != "1,3" {
		t.Fatalf("patched tags = %#v", patched)
	}
}

func intsToStrings(values []int) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = fmt.Sprintf("%d", value)
	}
	return out
}
