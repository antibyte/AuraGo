package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestGo2RTCTuyaSetupReservationIdentityAndRevision(t *testing.T) {
	s, _, _ := newGo2RTCMutationTestServer(t, "http://127.0.0.1:1984", false)
	a := config.Go2RTCTuyaAccount{ID: "home", Revision: "r1"}
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Authorization", "Bearer identity-one")
	token, items, err := s.newTuyaSession(r, a, []go2RTCTuyaDevice{{ID: "device123", Name: "Door"}})
	if err != nil {
		t.Fatal(err)
	}
	input := go2RTCTuyaCameraRequest{AccountID: a.ID, SetupToken: token, CandidateID: items[0]["id"]}
	other := httptest.NewRequest("POST", "/", nil)
	other.Header.Set("Authorization", "Bearer other-identity")
	if _, _, err := s.tuyaDevice(other, a, input, true); err == nil {
		t.Fatal("foreign identity reserved selection")
	}
	changed := a
	changed.Revision = "r2"
	if _, _, err := s.tuyaDevice(r, changed, input, true); err == nil {
		t.Fatal("changed account accepted")
	}
	device, reserved, err := s.tuyaDevice(r, a, input, true)
	if err != nil || device != "device123" {
		t.Fatal(err)
	}
	if _, _, err := s.tuyaDevice(r, a, input, true); err == nil {
		t.Fatal("parallel reservation accepted")
	}
	reserved.finish(false)
	_, reserved, err = s.tuyaDevice(r, a, input, true)
	if err != nil {
		t.Fatal("rollback consumed selection", err)
	}
	reserved.finish(true)
	if _, _, err := s.tuyaDevice(r, a, input, true); err == nil {
		t.Fatal("consumed selection accepted")
	}
	token, items, _ = s.newTuyaSession(r, a, []go2RTCTuyaDevice{{ID: "device123"}})
	input.SetupToken = token
	input.CandidateID = items[0]["id"]
	s.go2RTCTuyaSessions[token].expires = time.Now().Add(-time.Second)
	if _, _, err := s.tuyaDevice(r, a, input, true); err == nil {
		t.Fatal("expired selection accepted")
	}
	release, err := s.claimTuyaAccount(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := s.claimTuyaAccount(a.ID); err == nil {
		t.Fatal("parallel account search accepted")
	}
}

func TestGo2RTCTuyaCameraCreationAndAccountSwitch(t *testing.T) {
	s, path, vault := newGo2RTCMutationTestServer(t, "http://127.0.0.1:1984", false)
	_, err := updateManagedGo2RTCConfigWithTuya(context.Background(), s, func(c *config.Go2RTCConfig) error {
		c.TuyaAccounts = []config.Go2RTCTuyaAccount{{ID: "home", Name: "Home", Method: "cloud", Region: "eu-central"}}
		return nil
	}, nil, false, go2RTCTuyaInputs{Accounts: map[string]map[string]string{"home": {"client_id": "client123", "client_secret": "secret123", "uid": "uid123"}}})
	if err != nil {
		t.Fatal(err)
	}
	// A stopped sidecar stays stopped; create a disabled camera through the
	// general config path, then update its qualities through the app path.
	r := httptest.NewRecorder()
	handleUpdateConfig(s)(r, httptest.NewRequest("PUT", "/api/config", strings.NewReader(`{"go2rtc":{"streams":[{"id":"tuya-door","name":"Door","enabled":false,"tuya":{"account_id":"home","main_resolution":"hd","sub_resolution":"sd","device_id":"device123"}}]}}`)))
	if r.Code != 200 {
		t.Fatalf("camera creation: %d %s", r.Code, r.Body.String())
	}
	for _, role := range []string{"main", "sub"} {
		source, err := vault.ReadSecret(config.Go2RTCSourceKey("tuya-door", role))
		if err != nil || config.ValidateGo2RTCTuyaSource(source) != nil {
			t.Fatal("camera source missing", err)
		}
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "device123") || strings.Contains(string(data), "secret123") {
		t.Fatal("source identifier or credential in YAML")
	}
	r = httptest.NewRecorder()
	handleGo2RTCTuyaAccount(s)(r, httptest.NewRequest("DELETE", "/api/go2rtc/tuya/accounts/home", nil))
	if r.Code != 409 {
		t.Fatalf("referenced account removed: %d", r.Code)
	}
}

func TestGo2RTCTuyaFullAuthenticationAndProxyBoundaries(t *testing.T) {
	s, _, vault := newGo2RTCMutationTestServer(t, "http://127.0.0.1:1984", false)
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.PasswordHash = "configured"
	s.Cfg.Auth.SessionSecret = "fixture"
	tokens, err := security.NewTokenManager(vault, t.TempDir()+"/tokens.bin")
	if err != nil {
		t.Fatal(err)
	}
	s.TokenManager = tokens
	viewer, _, _ := tokens.Create("viewer", []string{go2RTCViewScope}, nil)
	admin, _, _ := tokens.Create("admin", []string{"admin"}, nil)
	mux := http.NewServeMux()
	registerGo2RTCRoutes(mux, s)
	handler := authMiddleware(s, mux)
	for _, test := range []struct{ method, path string }{
		{"GET", "/api/go2rtc/tuya/accounts"}, {"POST", "/api/go2rtc/tuya/accounts"}, {"PATCH", "/api/go2rtc/tuya/accounts/home"}, {"DELETE", "/api/go2rtc/tuya/accounts/home"}, {"POST", "/api/go2rtc/tuya/discover"}, {"POST", "/api/go2rtc/tuya/test"},
	} {
		r := httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer "+viewer)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("viewer %s %s: %d", test.method, test.path, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/api/go2rtc/tuya/accounts", strings.NewReader(`{"id":"home","name":"Home","method":"smart","region":"eu-central","email":"email@example.org","password":"secret123"}`))
	r.Header.Set("Authorization", "Bearer "+admin)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("administrator account save: %d %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if strings.Contains(w.Body.String(), "secret123") || strings.Contains(w.Body.String(), "email@example.org") {
		t.Fatal("account response leaked login")
	}
	for _, path := range []string{"api/tuya", "api/aurago/tuya"} {
		if go2RTCProxyPathAllowed(path, "GET", true) || go2RTCProxyPathAllowed(path, "POST", true) {
			t.Fatal("Tuya raw endpoint proxied")
		}
	}
	r = httptest.NewRequest("POST", "/api/go2rtc/tuya/test", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer "+admin)
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("foreign origin accepted")
	}
}

func TestGo2RTCTuyaCreateReservesUntilPublication(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "offline", 503) }))
	defer upstream.Close()
	s, _, vault := newGo2RTCMutationTestServer(t, upstream.URL, true)
	a := config.Go2RTCTuyaAccount{ID: "home", Name: "Home", Method: "smart", Region: "eu-central", Revision: "fixture"}
	s.Cfg.Go2RTC.TuyaAccounts = []config.Go2RTCTuyaAccount{a}
	s.Cfg.Go2RTC.Streams = []config.Go2RTCStreamConfig{{ID: "existing", Name: "Existing", Enabled: false}}
	_ = vault.WriteSecret(config.Go2RTCTuyaAccountKey("home"), `{"email":"email@example.org","password":"secretABC"}`)
	identity := httptest.NewRequest("POST", "http://aurago.local/api/go2rtc/streams", nil)
	token, items, err := s.newTuyaSession(identity, a, []go2RTCTuyaDevice{{ID: "device123", Name: "Door"}})
	if err != nil {
		t.Fatal(err)
	}
	create := func(id string) *httptest.ResponseRecorder {
		body := map[string]interface{}{"id": id, "name": "Door", "tuya": map[string]string{"account_id": "home", "setup_token": token, "candidate_id": items[0]["id"], "main_resolution": "hd", "sub_resolution": "sd"}}
		encoded, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		handleGo2RTCCreateStream(s)(w, httptest.NewRequest("POST", "http://aurago.local/api/go2rtc/streams", strings.NewReader(string(encoded))))
		return w
	}
	if w := create("existing"); w.Code != 409 {
		t.Fatalf("duplicate: %d %s", w.Code, w.Body.String())
	}
	w := create("tuya-door")
	if w.Code != 201 && w.Code != 202 {
		t.Fatalf("Tuya create: %d %s", w.Code, w.Body.String())
	}
	for _, secret := range []string{"device123", "secretABC", token} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("camera response leaked setup data")
		}
	}
	if w := create("replay"); w.Code != 400 {
		t.Fatal("committed selection reused", w.Code)
	}
}
