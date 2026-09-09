package tsnetnode

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGodsEyeProxyPreservesFrameAncestors(t *testing.T) {
	for _, policy := range []string{"frame-ancestors 'none'", "frame-ancestors https://aurago.example:8443"} {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", policy)
			w.Header().Set("Cache-Control", "no-store")
			w.Write([]byte("<html>GEV</html>"))
		}))
		handler, err := newStoreAppProxyHandler(StoreAppProxySpec{TargetURL: upstream.URL, PreserveFramePolicy: true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "https://aurago.example:4173/", nil))
		if w.Header().Get("Content-Security-Policy") != policy {
			t.Fatal("frame policy removed")
		}
		upstream.Close()
	}
}
