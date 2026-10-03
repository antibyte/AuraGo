package server

import (
	"aurago/internal/config"
	"aurago/internal/security"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogoutRevocationSurvivesReloadAndLeavesNewSessionValid(t *testing.T) {
	secret := "synthetic-session-secret-for-test"
	cfg := &config.Config{}
	cfg.Auth.SessionSecret = secret
	cfg.Directories.DataDir = t.TempDir()
	s := &Server{Cfg: cfg}
	cookie := createSessionValue(secret, time.Now().Add(time.Hour))
	other := createSessionValue(secret, time.Now().Add(time.Hour))
	request := httptest.NewRequest("POST", "http://example.com/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie})
	if !revokeRequestSession(s, request) {
		t.Fatal("revocation failed")
	}
	key := sha256.Sum256([]byte(secret + "\x00" + cookie))
	revokedSessions.Lock()
	delete(revokedSessions.entries, key)
	revokedSessions.Unlock()
	if err := loadSessionRevocations(s); err != nil {
		t.Fatal(err)
	}
	if validateSessionValue(secret, cookie) || !validateSessionValue(secret, other) {
		t.Fatal("replay accepted or unrelated session revoked")
	}
}

func TestHTTPDrainCancelsActiveHandlerBeforeDependenciesClose(t *testing.T) {
	s := &Server{}
	started, done := make(chan struct{}), make(chan struct{})
	handler := s.trackHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); w.WriteHeader(200) }))
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	<-started
	s.beginHTTPDrain()
	s.httpRequests.Wait()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("active handler did not drain")
	}
	late := httptest.NewRecorder()
	handler.ServeHTTP(late, httptest.NewRequest("GET", "/", nil))
	if late.Code != 503 {
		t.Fatalf("accepted request during drain: %d", late.Code)
	}
}

func TestCastAssetsRequireTicketAndSupportRangeAndHead(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "one.wav"), []byte("0123456789"), 0600)
	handler := castMediaAssetHandler(dir, "/tts/", true)
	signed := security.SignCastMediaURL("http://example.com/tts/one.wav")
	for _, test := range []struct {
		method, url, rangeHeader string
		status                   int
		body                     string
	}{
		{"GET", "/tts/one.wav", "", 403, ""}, {"GET", signed, "bytes=2-4", 206, "234"}, {"HEAD", signed, "", 200, ""}, {"GET", "/tts/", "", 403, ""},
	} {
		request := httptest.NewRequest(test.method, test.url, nil)
		request.Header.Set("Range", test.rangeHeader)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status || test.status < 400 && response.Body.String() != test.body {
			t.Fatalf("%s %s: %d %q", test.method, test.url, response.Code, response.Body.String())
		}
	}
}

func TestDesktopLogAPIRedactsRegisteredSecrets(t *testing.T) {
	token := "synthetic-log-secret-839284-audit"
	security.RegisterSensitive(token)
	dir := t.TempDir()
	writeDesktopLogFixture(t, dir, "aurago.log", `level=INFO msg="`+token+`" credential="`+token+`"`)
	s := testDesktopLogServer(t, dir)
	for _, route := range []string{"tail", "search", "download"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/api/desktop/logs/"+route+"?file=aurago.log", nil)
		handlers := map[string]http.HandlerFunc{"tail": handleDesktopLogTail(s), "search": handleDesktopLogSearch(s), "download": handleDesktopLogDownload(s)}
		handlers[route].ServeHTTP(response, request)
		if response.Code != 200 || strings.Contains(response.Body.String(), token) {
			t.Fatalf("%s leaked a secret or failed (%d)", route, response.Code)
		}
		if route != "download" && !json.Valid(response.Body.Bytes()) {
			t.Fatal("invalid log JSON")
		}
	}
}
