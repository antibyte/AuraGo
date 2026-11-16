package a2a

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteHTTPClientBindsPeerOrigin(t *testing.T) {
	hits := 0
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer foreign.Close()
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL, http.StatusTemporaryRedirect)
	}))
	defer peer.Close()
	client, err := remoteHTTPClient(peer.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	req, _ := http.NewRequest(http.MethodGet, peer.URL, nil)
	req.Header.Set("X-API-Key", "fixture-peer-key")
	if resp, err := client.Do(req); err == nil {
		resp.Body.Close()
		t.Error("cross-origin redirect accepted")
	}
	req, _ = http.NewRequest(http.MethodGet, foreign.URL, nil)
	if resp, err := client.Do(req); err == nil {
		resp.Body.Close()
		t.Error("foreign advertised endpoint accepted")
	}
	if hits != 0 {
		t.Fatalf("foreign requests=%d", hits)
	}
	for _, raw := range []string{"file:///tmp/card", "https://secret@peer.test/card", "http://peer.test/card?token=secret", "http://peer.test/#fragment"} {
		if _, err := remoteHTTPClient(raw); err == nil {
			t.Errorf("accepted invalid peer URL %s", raw)
		}
	}
}
