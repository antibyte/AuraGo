package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestGo2RTCTuyaSharedAccountTransaction(t *testing.T) {
	s, path, vault := newGo2RTCMutationTestServer(t, "http://127.0.0.1:1984", false)
	inputs := go2RTCTuyaInputs{Accounts: map[string]map[string]string{"home": {"email": "test@example.org", "password": "secret&+#%ä"}}, Devices: map[string]string{"one": "device1", "two": "device2"}}
	result, err := updateManagedGo2RTCConfigWithTuya(context.Background(), s, func(c *config.Go2RTCConfig) error {
		c.TuyaAccounts = []config.Go2RTCTuyaAccount{{ID: "home", Name: "Home", Method: "smart", Region: "eu-central"}}
		for _, id := range []string{"one", "two"} {
			c.Streams = append(c.Streams, config.Go2RTCStreamConfig{ID: id, Name: id, Tuya: &config.Go2RTCTuyaBinding{AccountID: "home", MainResolution: "hd", SubResolution: "sd"}})
		}
		return nil
	}, nil, false, inputs)
	if err != nil || !result.Published {
		t.Fatalf("create: %v", err)
	}
	before, _ := vault.ReadSecret(config.Go2RTCSourceKey("one", "main"))
	revision := result.Config.TuyaAccounts[0].Revision
	result, err = updateManagedGo2RTCConfig(context.Background(), s, func(c *config.Go2RTCConfig) error { c.TuyaAccounts[0].Name = "Renamed"; return nil }, nil, false)
	after, _ := vault.ReadSecret(config.Go2RTCSourceKey("one", "main"))
	if err != nil || before != after || result.Config.TuyaAccounts[0].Revision != revision {
		t.Fatalf("rename changed sources: %v", err)
	}
	_, err = updateManagedGo2RTCConfigWithTuya(context.Background(), s, func(c *config.Go2RTCConfig) error { return nil }, nil, false, go2RTCTuyaInputs{Accounts: map[string]map[string]string{"home": {"password": "replacement&+#"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two"} {
		for _, role := range []string{"main", "sub"} {
			raw, _ := vault.ReadSecret(config.Go2RTCSourceKey(id, role))
			u, _ := url.Parse(raw)
			if u.Query().Get("password") != "replacement&+#" {
				t.Fatal("account rotation missed camera role")
			}
		}
	}
	data, _ := os.ReadFile(path)
	for _, secret := range []string{"replacement", "test@example.org", "device1", "tuya://"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("secret in configuration YAML")
		}
	}
	_, err = updateManagedGo2RTCConfig(context.Background(), s, func(c *config.Go2RTCConfig) error { c.TuyaAccounts = nil; return nil }, nil, false)
	if err == nil {
		t.Fatal("removed an account still in use")
	}
}

func TestGo2RTCTuyaConfigPatchAndRollback(t *testing.T) {
	s, path, vault := newGo2RTCStreamsPatchTestServer(t, false)
	patch := `{"go2rtc":{"tuya_accounts":[{"id":"home","name":"Home","method":"cloud","region":"eu-central","client_id":"client123","client_secret":"secretABC","uid":"uid123"}]}}`
	r := httptest.NewRecorder()
	handleUpdateConfig(s)(r, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch)))
	if r.Code != 200 {
		t.Fatalf("config save: %d %s", r.Code, r.Body.String())
	}
	c, err := readTuyaCredentials(vault, "home")
	if err != nil || c.ClientSecret != "secretABC" {
		t.Fatalf("credentials: %v", err)
	}
	before, _ := os.ReadFile(path)
	bad := strings.Replace(patch, "secretABC", "changedSECRET", 1)
	bad = strings.Replace(bad, `"go2rtc":{`, `"go2rtc":{"hardware":{"backend":"invalid"},`, 1)
	r = httptest.NewRecorder()
	handleUpdateConfig(s)(r, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(bad)))
	if r.Code != 400 {
		t.Fatalf("invalid save: %d %s", r.Code, r.Body.String())
	}
	c, _ = readTuyaCredentials(vault, "home")
	after, _ := os.ReadFile(path)
	if c.ClientSecret != "secretABC" || string(before) != string(after) {
		t.Fatal("failed save changed account or YAML")
	}
	var cfg map[string]interface{}
	r = httptest.NewRecorder()
	handleGetConfig(s)(r, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	_ = json.Unmarshal(r.Body.Bytes(), &cfg)
	if strings.Contains(r.Body.String(), "secretABC") {
		t.Fatal("GET config leaked credentials")
	}
}
