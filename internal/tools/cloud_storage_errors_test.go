package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudStorageProviderFailuresAreNotReadonly(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	for _, status := range []int{401, 403, 429, 500, 502, 503, 504} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			requests := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.WriteHeader(status)
				io.WriteString(w, "private provider details must not reach the browser")
			}))
			defer provider.Close()
			client := CloudStorage{Koofr: KoofrConfig{BaseURL: provider.URL}}
			loc := StorageLocation{Provider: "koofr", MountID: "external", Path: "file.txt"}
			operations := map[string]func() error{
				"list": func() error { _, err := client.List(context.Background(), loc); return err },
				"stat": func() error { _, err := client.Stat(context.Background(), loc); return err },
				"open": func() error {
					body, err := client.Open(context.Background(), loc, "")
					if body != nil {
						body.Close()
					}
					return err
				},
				"upload": func() error {
					_, err := client.Put(context.Background(), loc, strings.NewReader("new data"), "")
					return err
				},
			}
			code := "storage_unavailable"
			if status == 401 || status == 403 {
				code = "storage_access_denied"
			}
			for name, operation := range operations {
				before := requests
				err := operation()
				want := fmt.Sprintf("%s (HTTP %d)", code, status)
				if err == nil || err.Error() != want {
					t.Errorf("%s: got %v, want %s", name, err, want)
				}
				if requests != before+1 {
					t.Errorf("%s retried a provider failure", name)
				}
			}
		})
	}
}
