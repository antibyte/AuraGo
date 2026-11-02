package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestJSONRequestLimitsAndSingleDocument(t *testing.T) {
	for _, tc := range []struct {
		name, body   string
		limit, inner int64
		want         int
	}{
		{"valid", `{"value":"quotes\" and ü"}`, 64, 0, 200},
		{"second document", `{} {}`, 64, 0, 400},
		{"trailing junk", `{}x`, 64, 0, 400},
		{"oversized value", `{"value":"` + strings.Repeat("x", 64) + `"}`, 64, 0, 413},
		{"oversized whitespace", `{}` + strings.Repeat(" ", 64), 64, 0, 413},
		{"smaller route limit", `{"value":"12345678"}`, 64, 8, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{Cfg: &config.Config{}}
			s.Cfg.Server.MaxBodyBytes = tc.limit
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			r.ContentLength = -1 // Chunked requests have exactly the same cap.
			w := httptest.NewRecorder()
			if tc.inner > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, tc.inner)
			}
			var value map[string]string
			if err := decodeJSONRequest(w, r, &value, s); err != nil {
				w.WriteHeader(jsonDecodeStatus(err))
			}
			if w.Code != tc.want {
				t.Fatalf("status=%d, want %d", w.Code, tc.want)
			}
		})
	}
}

func TestJSONRequestContextLimitAndDefault(t *testing.T) {
	for _, limit := range []int64{16, 0} {
		budget := limit
		if budget == 0 {
			budget = defaultJSONBodyLimit
		}
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`+strings.Repeat(" ", int(budget))))
		r = r.WithContext(withJSONBodyLimit(r.Context(), limit))
		var value any
		err := decodeOptionalJSONRequest(httptest.NewRecorder(), r, &value)
		if err == nil || jsonDecodeStatus(err) != http.StatusRequestEntityTooLarge {
			t.Fatalf("limit=%d: %v", limit, err)
		}
	}
	for _, body := range []string{"", " \n", "{", "{}{}"} {
		var value any
		err := decodeOptionalJSONRequest(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(body)), &value)
		if (err == nil) != (strings.TrimSpace(body) == "") {
			t.Fatalf("optional body %q: %v", body, err)
		}
	}
}
