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

func TestGo2RTCTuyaRemovingCameraClearsVaultAndCannotDetachSilently(t *testing.T) {
	s, _, vault := newGo2RTCMutationTestServer(t, "http://127.0.0.1:1984", false)
	_, err := updateManagedGo2RTCConfigWithTuya(context.Background(), s, func(c *config.Go2RTCConfig) error {
		c.TuyaAccounts = []config.Go2RTCTuyaAccount{{ID: "home", Name: "Home", Method: "smart", Region: "eu-central"}}
		c.Streams = []config.Go2RTCStreamConfig{{ID: "door", Name: "Door", Tuya: &config.Go2RTCTuyaBinding{AccountID: "home", MainResolution: "hd", SubResolution: "sd"}}}
		return nil
	}, nil, false, go2RTCTuyaInputs{Accounts: map[string]map[string]string{"home": {"email": "email@example.org", "password": "secretABC"}}, Devices: map[string]string{"door": "device123"}})
	if err != nil {
		t.Fatal(err)
	}
	request := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handleUpdateConfig(s)(w, httptest.NewRequest("PUT", "/api/config", strings.NewReader(body)))
		return w
	}
	if w := request(`{"go2rtc":{"streams":[{"id":"door","name":"Door","enabled":false,"tuya":null}]}}`); w.Code != 400 {
		t.Fatal("binding silently detached", w.Code, w.Body.String())
	}
	if _, err := vault.ReadSecret(config.Go2RTCSourceKey("door", "main")); err != nil {
		t.Fatal("failed edit removed source")
	}
	if w := request(`{"go2rtc":{"streams":[],"tuya_accounts":[]}}`); w.Code != 200 {
		t.Fatal("camera/account removal failed", w.Code, w.Body.String())
	}
	for _, key := range []string{config.Go2RTCSourceKey("door", "main"), config.Go2RTCSourceKey("door", "sub"), config.Go2RTCTuyaAccountKey("home")} {
		if _, err := vault.ReadSecret(key); err == nil {
			t.Fatal("removed camera or account retained credentials", key)
		}
	}
}

func TestGo2RTCTuyaConfigClearSubKeepsManagedRole(t *testing.T) {
	s, _, vault := newGo2RTCMutationTestServer(t, "http://127.0.0.1:1984", false)
	_, err := updateManagedGo2RTCConfigWithTuya(context.Background(), s, func(c *config.Go2RTCConfig) error {
		c.TuyaAccounts = []config.Go2RTCTuyaAccount{{ID: "home", Name: "Home", Method: "smart", Region: "eu-central"}}
		c.Streams = []config.Go2RTCStreamConfig{{ID: "door", Name: "Door", Tuya: &config.Go2RTCTuyaBinding{AccountID: "home", MainResolution: "hd", SubResolution: "sd"}}}
		return nil
	}, nil, false, go2RTCTuyaInputs{Accounts: map[string]map[string]string{"home": {"email": "fixture@example.org", "password": "fixture-password"}}, Devices: map[string]string{"door": "device123"}})
	if err != nil {
		t.Fatal(err)
	}
	before, err := vault.ReadSecret(config.Go2RTCSourceKey("door", "sub"))
	if err != nil || before == "" {
		t.Fatal("fixture role missing")
	}
	w := httptest.NewRecorder()
	handleUpdateConfig(s)(w, httptest.NewRequest("PUT", "/api/config", strings.NewReader(`{"go2rtc":{"streams":[{"id":"door","name":"Door","enabled":false,"tuya":{"account_id":"home","main_resolution":"hd","sub_resolution":"sd"},"clear_sub_source":true}]}}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	after, err := vault.ReadSecret(config.Go2RTCSourceKey("door", "sub"))
	if err != nil || after != before || !s.ConfigSnapshot().Go2RTC.Streams[0].SubSourceConfigured {
		t.Fatal("published SD binding lost its managed source")
	}
}

func TestGo2RTCSourceVaultTransactionFinalChangesAndRollback(t *testing.T) {
	s, _, vault := newGo2RTCMutationTestServer(t, "http://127.0.0.1:1984", false)
	key := config.Go2RTCSourceKey("one", "sub")
	const original = "rtsp://camera.local/original"
	if err := vault.WriteSecret(key, original); err != nil {
		t.Fatal(err)
	}
	txn, err := stageGo2RTCSourceChanges(s, []go2RTCSourceChange{
		{ID: "one", Role: "sub", Delete: true}, {ID: "one", Role: "sub", Value: original},
		{ID: "two", Value: "rtsp://camera.local/temporary"}, {ID: "two", Delete: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if value, err := vault.ReadSecret(key); err != nil || value != original {
		t.Fatal("delete followed by restoring the original source lost it")
	}
	if _, err := vault.ReadSecret(config.Go2RTCSourceKey("two", "main")); err == nil {
		t.Fatal("a final delete left an intermediate source behind")
	}
	for _, value := range []string{"rtsp://camera.local/replacement", original} {
		if err := txn.apply([]go2RTCSourceChange{{ID: "one", Role: "sub", Value: value}}); err != nil {
			t.Fatal(err)
		}
		if saved, err := vault.ReadSecret(key); err != nil || saved != value {
			t.Fatal("a later apply compared against the rollback snapshot")
		}
	}
	if err := txn.apply([]go2RTCSourceChange{{ID: "one", Role: "sub", Delete: true}}); err != nil {
		t.Fatal(err)
	}
	if err := txn.Rollback(); err != nil {
		t.Fatal(err)
	}
	if value, err := vault.ReadSecret(key); err != nil || value != original {
		t.Fatal("rollback lost the original source")
	}
}
