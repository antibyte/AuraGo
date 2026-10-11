package server

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDesktopStorageExternalAccountErrorsAndReadonlyReads(t *testing.T) {
	for _, test := range []struct {
		name, mount, errorCode string
		providerStatus, status int
		readonly, offline      bool
	}{
		{"onedrive_expired", "onedrive", "storage_access_denied (HTTP 401)", 401, 403, false, false},
		{"google_denied", "google", "storage_access_denied (HTTP 403)", 403, 403, false, false},
		{"google_refreshing", "google", "storage_unavailable (HTTP 500)", 500, 503, false, false},
		{"offline", "onedrive", "storage_unavailable", 200, 503, false, true},
		{"read_only_mount", "onedrive", "", 200, 200, false, false},
		{"read_only_integration", "google", "", 200, 200, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, _ := newStorageTestSession(t)
			fileRequests := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v2/mounts" {
					fmt.Fprintf(w, `{"mounts":[{"id":%q,"name":%q,"online":%t,"permissions":{"READ":true,"write":false}}]}`, test.mount, test.name, !test.offline)
					return
				}
				fileRequests++
				if r.Method != http.MethodGet || r.URL.Path != "/api/v2/mounts/"+test.mount+"/files/list" || r.URL.Query().Get("path") != "/" {
					t.Errorf("unexpected provider request: %s %s", r.Method, r.URL)
				}
				w.WriteHeader(test.providerStatus)
				io.WriteString(w, `{"files":[{"name":"example.txt","type":"file","size":3}]}`)
			}))
			defer provider.Close()
			d.s.Cfg.Koofr.BaseURL = provider.URL
			d.s.Cfg.Koofr.ReadOnly = test.readonly
			handler := handleDesktopStorage(d.s)
			query := url.Values{"provider": {"koofr"}, "mount_id": {test.mount}}.Encode()
			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/storage/files?"+query, nil))
			if rec.Code != test.status || test.errorCode != "" && !strings.Contains(rec.Body.String(), test.errorCode) {
				t.Fatalf("got %d %s, want %d %q", rec.Code, rec.Body.String(), test.status, test.errorCode)
			}
			wantRequests := 1
			if test.offline {
				wantRequests = 0
			}
			if fileRequests != wantRequests {
				t.Fatalf("provider file requests = %d, want %d", fileRequests, wantRequests)
			}
			if test.status == 200 {
				if !strings.Contains(rec.Body.String(), "example.txt") {
					t.Fatal("read-only listing was lost")
				}
				rec = httptest.NewRecorder()
				handler(rec, httptest.NewRequest(http.MethodPost, "/api/desktop/storage/upload?"+query+"&path=new.txt", strings.NewReader("write blocked")))
				if rec.Code != 403 || !strings.Contains(rec.Body.String(), "storage_readonly") || fileRequests != 1 {
					t.Fatalf("read-only write escaped: %d %s", rec.Code, rec.Body.String())
				}
			}
		})
	}
}
