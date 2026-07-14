package virtualcomputers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStorageConnectionUsesAuthenticatedReadOnlyHeadBucket(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodHead || r.URL.Path != "/boring-volumes/" {
			t.Errorf("request = %s %s", r.Method, r.URL.RequestURI())
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := TestStorageConnection(ctx, StorageTestConfig{
		Endpoint: strings.TrimPrefix(server.URL, "http://"), Bucket: "boring-volumes", Region: "eu-central-1",
		AccessKeyID: "access-key", SecretKey: "secret-key", UseSSL: false,
	})
	if err != nil {
		t.Fatalf("TestStorageConnection: %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("request count = %d", requests.Load())
	}
}
