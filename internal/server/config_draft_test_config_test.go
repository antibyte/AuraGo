package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

const draftTestSavedYAML = "home_assistant:\n  enabled: true\n  url: http://saved.invalid:8123\nfritzbox:\n  host: saved.fritz.box\n"

func newDraftTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(draftTestSavedYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	vault, err := security.NewVault(strings.Repeat("42", 32), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("home_assistant_access_token", "saved-ha-token"); err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("fritzbox_password", "saved-fritz-password"); err != nil {
		t.Fatal(err)
	}
	cfg.ApplyVaultSecrets(vault)
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
	s.initConfigSnapshot()
	return s
}

func draftRequest(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(body))
}

func TestDraftTestConfigEmptyBodyReturnsDetachedSavedClone(t *testing.T) {
	s := newDraftTestServer(t)
	for _, body := range []string{"", "{}", `{"draft":null}`, `{"draft":{}}`} {
		cfg, err := s.draftTestConfig(draftRequest(body))
		if err != nil {
			t.Fatalf("body %q: %v", body, err)
		}
		saved := s.ConfigSnapshot()
		if cfg == saved {
			t.Fatalf("body %q: result is the published snapshot", body)
		}
		if cfg.HomeAssistant.URL != "http://saved.invalid:8123" || cfg.HomeAssistant.AccessToken != "saved-ha-token" {
			t.Fatalf("body %q: saved values lost: %q %q", body, cfg.HomeAssistant.URL, cfg.HomeAssistant.AccessToken)
		}
		cfg.HomeAssistant.URL = "mutated"
		if saved.HomeAssistant.URL != "http://saved.invalid:8123" {
			t.Fatal("mutating the clone changed the published snapshot")
		}
	}
	cfg, err := s.draftTestConfig(httptest.NewRequest(http.MethodGet, "/api/test", nil))
	if err != nil || cfg.HomeAssistant.AccessToken != "saved-ha-token" {
		t.Fatalf("GET without body: %v", err)
	}
}

func TestDraftTestConfigUsesDraftValues(t *testing.T) {
	s := newDraftTestServer(t)
	cfg, err := s.draftTestConfig(draftRequest(`{"draft":{"home_assistant":{"url":"http://draft.invalid:8123","readonly":true,"access_token":" draft-ha-token "}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HomeAssistant.URL != "http://draft.invalid:8123" || !cfg.HomeAssistant.ReadOnly {
		t.Fatalf("draft values not applied: %+v", cfg.HomeAssistant)
	}
	if !cfg.HomeAssistant.Enabled {
		t.Fatal("saved enabled flag lost")
	}
	if cfg.HomeAssistant.AccessToken != "draft-ha-token" {
		t.Fatalf("draft secret = %q, want trimmed draft token", cfg.HomeAssistant.AccessToken)
	}
}

func TestDraftTestConfigMaskedOrEmptySecretKeepsSavedValue(t *testing.T) {
	s := newDraftTestServer(t)
	for _, token := range []string{"••••••••", "", "   "} {
		body, _ := json.Marshal(map[string]interface{}{"draft": map[string]interface{}{
			"home_assistant": map[string]interface{}{"url": "http://saved.invalid:8123", "readonly": true, "access_token": token},
		}})
		cfg, err := s.draftTestConfig(draftRequest(string(body)))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.HomeAssistant.AccessToken != "saved-ha-token" || !cfg.HomeAssistant.ReadOnly {
			t.Fatalf("token %q: got %q / %v", token, cfg.HomeAssistant.AccessToken, cfg.HomeAssistant.ReadOnly)
		}
	}
}

func TestDraftTestConfigVaultOnlySecretFromDraft(t *testing.T) {
	s := newDraftTestServer(t)
	cfg, err := s.draftTestConfig(draftRequest(`{"draft":{"fritzbox":{"password":"draft-fritz-password"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FritzBox.Password != "draft-fritz-password" || cfg.FritzBox.Host != "saved.fritz.box" {
		t.Fatalf("vault-only draft secret: %q host %q", cfg.FritzBox.Password, cfg.FritzBox.Host)
	}
	cfg, err = s.draftTestConfig(draftRequest(`{"draft":{"fritzbox":{"password":"••••••••","host":"saved.fritz.box","timeout":7}}}`))
	if err != nil || cfg.FritzBox.Password != "saved-fritz-password" || cfg.FritzBox.Timeout != 7 {
		t.Fatalf("masked vault-only secret: %v %q %d", err, cfg.FritzBox.Password, cfg.FritzBox.Timeout)
	}
}

func TestDraftTestConfigPersistsNothing(t *testing.T) {
	s := newDraftTestServer(t)
	path := s.Cfg.ConfigPath
	beforeFile, _ := os.ReadFile(path)
	beforeInfo, _ := os.Stat(path)
	beforeKeys, _ := s.Vault.ListKeys()
	sort.Strings(beforeKeys)
	beforeSnapshot := s.ConfigSnapshot()
	beforeURL := beforeSnapshot.HomeAssistant.URL
	beforeToken := beforeSnapshot.HomeAssistant.AccessToken
	time.Sleep(20 * time.Millisecond)

	_, err := s.draftTestConfig(draftRequest(`{"draft":{"home_assistant":{"url":"http://draft.invalid","access_token":"draft-ha-token"},"fritzbox":{"password":"draft-fritz-password"},"mqtt":{"password":"draft-mqtt"}}}`))
	if err != nil {
		t.Fatal(err)
	}

	afterFile, _ := os.ReadFile(path)
	afterInfo, _ := os.Stat(path)
	if string(beforeFile) != string(afterFile) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("config file was touched")
	}
	afterKeys, _ := s.Vault.ListKeys()
	sort.Strings(afterKeys)
	if !reflect.DeepEqual(beforeKeys, afterKeys) {
		t.Fatalf("vault keys changed: %v -> %v", beforeKeys, afterKeys)
	}
	for key, want := range map[string]string{"home_assistant_access_token": "saved-ha-token", "fritzbox_password": "saved-fritz-password"} {
		if got, _ := s.Vault.ReadSecret(key); got != want {
			t.Fatalf("vault %s changed", key)
		}
	}
	if s.ConfigSnapshot() != beforeSnapshot || beforeSnapshot.HomeAssistant.URL != beforeURL || beforeSnapshot.HomeAssistant.AccessToken != beforeToken {
		t.Fatal("published snapshot was mutated")
	}
	if beforeSnapshot.MQTT.Password != "" {
		t.Fatal("draft MQTT password leaked into the snapshot")
	}
}

func TestDraftTestConfigRejectsBadDrafts(t *testing.T) {
	s := newDraftTestServer(t)
	for name, body := range map[string]string{
		"bad json":         `{"draft":`,
		"draft not object": `{"draft":"x"}`,
		"unknown section":  `{"draft":{"not_a_section":{"a":1}}}`,
		"sip":              `{"draft":{"sip":{"enabled":true}}}`,
		"providers":        `{"draft":{"providers":[{"id":"x"}]}}`,
		"sparse list":      `{"draft":{"go2rtc":{"streams":[{"name":"no id"}]}}}`,
		"wrong type":       `{"draft":{"home_assistant":{"enabled":"definitely"}}}`,
	} {
		_, err := s.draftTestConfig(draftRequest(body))
		if err == nil || draftTestConfigStatus(err) != http.StatusBadRequest {
			t.Fatalf("%s: err = %v, want a 400 draft error", name, err)
		}
		if strings.Contains(err.Error(), "definitely") {
			t.Fatalf("%s: error echoes input", name)
		}
	}
}

func TestDraftTestConfigRestoresRequestBody(t *testing.T) {
	s := newDraftTestServer(t)
	req := draftRequest(`{"draft":{"home_assistant":{"readonly":true}},"extra":"kept"}`)
	if _, err := s.draftTestConfig(req); err != nil {
		t.Fatal(err)
	}
	var again map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&again); err != nil || again["extra"] != "kept" {
		t.Fatalf("body not restored: %v %v", err, again)
	}
}

func TestIntegrationTestHandlerTestsDraftWithoutSaving(t *testing.T) {
	const draftToken = "draft-handler-token"
	hits := 0
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("Authorization") != "Bearer "+draftToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"message":"API running."}`)
	}))
	defer probe.Close()

	s := newDraftTestServer(t)
	beforeFile, _ := os.ReadFile(s.Cfg.ConfigPath)

	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"draft": map[string]interface{}{
		"home_assistant": map[string]interface{}{"url": probe.URL, "access_token": draftToken},
	}})
	handleHomeAssistantTest(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/home-assistant/test", strings.NewReader(string(body))))
	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if rec.Code != http.StatusOK || resp["status"] != "ok" || hits == 0 {
		t.Fatalf("draft test: %d %s (hits %d)", rec.Code, rec.Body.String(), hits)
	}
	if strings.Contains(rec.Body.String(), draftToken) {
		t.Fatal("response leaked the draft token")
	}

	// A disabled draft is judged by the draft, not by the saved flag.
	rec = httptest.NewRecorder()
	handleHomeAssistantTest(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/home-assistant/test", strings.NewReader(`{"draft":{"home_assistant":{"enabled":false}}}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "disabled") {
		t.Fatalf("disabled draft: %d %s", rec.Code, rec.Body.String())
	}

	// Invalid draft body is a 400 with a reason.
	rec = httptest.NewRecorder()
	handleHomeAssistantTest(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/home-assistant/test", strings.NewReader(`{"draft":`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Invalid JSON") {
		t.Fatalf("bad body: %d %s", rec.Code, rec.Body.String())
	}

	if afterFile, _ := os.ReadFile(s.Cfg.ConfigPath); string(afterFile) != string(beforeFile) {
		t.Fatal("test request changed the config file")
	}
	if token, _ := s.Vault.ReadSecret("home_assistant_access_token"); token != "saved-ha-token" {
		t.Fatal("test request changed the vault")
	}
}

func TestNewTestRoutesRequireAdminAndPost(t *testing.T) {
	cfg := &config.Config{}
	cfg.WebConfig.Enabled = true
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "draft-route-session-secret"
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/telegram/test", func(http.ResponseWriter, *http.Request) {})
	s.registerConfigAPIRoutes(mux, nil)
	for _, path := range []string{"/api/jellyfin/test", "/api/truenas/test", "/api/obsidian/test", "/api/google-workspace/test", "/api/onedrive/test", "/api/video-generation/test", "/api/image-generation/test", "/api/frigate/test"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", path, rec.Code)
		}
	}
	plain := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	for name, handler := range map[string]http.HandlerFunc{
		"jellyfin": handleJellyfinTest(plain), "truenas": handleTrueNASTest(plain), "obsidian": handleObsidianTest(plain),
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s GET = %d, want 405", name, rec.Code)
		}
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{}`)))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "disabled") {
			t.Errorf("%s disabled POST = %d %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestJellyfinTestUsesDraftHostAndKey(t *testing.T) {
	const key = "draft-jellyfin-key"
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization")+r.Header.Get("X-Emby-Token")+r.URL.RawQuery, key) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"ServerName":"x","Version":"1"}`)
	}))
	defer probe.Close()
	hostPort := strings.TrimPrefix(probe.URL, "http://")
	host, port, _ := strings.Cut(hostPort, ":")

	s := newDraftTestServer(t)
	body, _ := json.Marshal(map[string]interface{}{"draft": map[string]interface{}{
		"jellyfin": map[string]interface{}{"enabled": true, "host": host, "port": mustAtoi(t, port), "api_key": key},
	}})
	rec := httptest.NewRecorder()
	handleJellyfinTest(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/jellyfin/test", strings.NewReader(string(body))))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("jellyfin draft test: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), key) {
		t.Fatal("response leaked the api key")
	}
}

func mustAtoi(t *testing.T, value string) int {
	t.Helper()
	var n int
	if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
		t.Fatal(err)
	}
	return n
}

// vaultKeyMap (PUT /api/config) and plaintextSecretVaultPaths (startup
// migration) must agree wherever both list a path. Paths only in one map are
// intentional: the migration also moves provider and SIP keys that have their
// own APIs, the server map also covers fields that never lived in plaintext.
func TestVaultKeyMapAgreesWithSecretMigrationMap(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "config", "config_secret_migration.go"))
	if err != nil {
		t.Fatal(err)
	}
	block := string(source)
	start := strings.Index(block, "var plaintextSecretVaultPaths")
	end := strings.Index(block[start:], "\n}\n")
	if start < 0 || end < 0 {
		t.Fatal("migration map not found")
	}
	entry := regexp.MustCompile(`"([a-z0-9_.]+)":\s+("[a-z0-9_]+"|[A-Za-z]+),`)
	migration := map[string]string{}
	for _, m := range entry.FindAllStringSubmatch(block[start:start+end], -1) {
		migration[m[1]] = strings.Trim(m[2], `"`)
	}
	if len(migration) < 50 {
		t.Fatalf("parsed only %d migration entries", len(migration))
	}
	for path, vaultKey := range vaultKeyMap {
		if want, ok := migration[path]; ok && want != vaultKey && !strings.ContainsAny(want, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			t.Errorf("%s: vaultKeyMap=%q migration=%q", path, vaultKey, want)
		}
	}
	for _, path := range []string{"fritzbox.password", "homepage.deploy_password", "homepage.deploy_key"} {
		if _, ok := vaultKeyMap[path]; !ok {
			t.Errorf("vaultKeyMap lacks %s", path)
		}
		if _, ok := migration[path]; !ok {
			t.Errorf("migration map lacks %s", path)
		}
	}
}

func putConfig(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body)))
	return rec
}

func TestPutConfigWritesFritzBoxAndHomepageSecretsToVaultOnly(t *testing.T) {
	s := newDraftTestServer(t)
	rec := putConfig(t, s, `{"fritzbox":{"password":"  new-fritz-fixture \n"},"homepage":{"deploy_password":"deploy-pass-fixture","deploy_key":"-----BEGIN KEY-----\nkey-fixture\n-----END KEY-----\n"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	file, _ := os.ReadFile(s.Cfg.ConfigPath)
	for _, secret := range []string{"new-fritz-fixture", "deploy-pass-fixture", "key-fixture"} {
		if strings.Contains(string(file), secret) {
			t.Fatalf("secret %q reached config.yaml:\n%s", secret, file)
		}
	}
	for key, want := range map[string]string{
		"fritzbox_password":        "new-fritz-fixture",
		"homepage_deploy_password": "deploy-pass-fixture",
		"homepage_deploy_key":      "-----BEGIN KEY-----\nkey-fixture\n-----END KEY-----",
	} {
		if got, _ := s.Vault.ReadSecret(key); got != want {
			t.Errorf("vault %s = %q, want %q (trimmed)", key, got, want)
		}
	}
	if got := s.ConfigSnapshot().FritzBox.Password; got != "new-fritz-fixture" {
		t.Errorf("live fritzbox password = %q", got)
	}
}

func TestGetConfigShowsMaskedIndicatorForFritzBoxAndHomepageSecrets(t *testing.T) {
	s := newDraftTestServer(t)
	for key, value := range map[string]string{"homepage_deploy_password": "x-fixture", "homepage_deploy_key": "k-fixture"} {
		if err := s.Vault.WriteSecret(key, value); err != nil {
			t.Fatal(err)
		}
	}
	raw := map[string]interface{}{}
	injectVaultIndicators(raw, s.Vault)
	for _, path := range [][]string{{"fritzbox", "password"}, {"homepage", "deploy_password"}, {"homepage", "deploy_key"}} {
		section, _ := raw[path[0]].(map[string]interface{})
		if section[path[1]] != "••••••••" {
			t.Errorf("%v not masked in GET: %v", path, section[path[1]])
		}
	}
}

func TestPutConfigRejectsInvalidTsNetKeyBeforeWriting(t *testing.T) {
	s := newDraftTestServer(t)
	before, _ := os.ReadFile(s.Cfg.ConfigPath)
	keysBefore, _ := s.Vault.ListKeys()
	rec := putConfig(t, s, `{"home_assistant":{"url":"http://changed.invalid"},"fritzbox":{"password":"must-not-be-written"},"tailscale":{"tsnet":{"auth_key":"not-a-valid-key"}}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "not-a-valid-key") {
		t.Fatal("error echoes the key")
	}
	after, _ := os.ReadFile(s.Cfg.ConfigPath)
	keysAfter, _ := s.Vault.ListKeys()
	if string(before) != string(after) || len(keysBefore) != len(keysAfter) {
		t.Fatal("rejected save wrote something")
	}
	if got, _ := s.Vault.ReadSecret("fritzbox_password"); got != "saved-fritz-password" {
		t.Fatal("rejected save changed another secret")
	}
}

func TestPutConfigAcceptsTsNetKeyTrimmedAndAppliesIt(t *testing.T) {
	s := newDraftTestServer(t)
	const key = "tskey-auth-abcdefghijklmnopqrstuvwx"
	rec := putConfig(t, s, `{"tailscale":{"tsnet":{"auth_key":"  `+key+` \n"}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	if got, _ := s.Vault.ReadSecret("tailscale_tsnet_authkey"); got != key {
		t.Fatalf("vault key = %q, want trimmed key", got)
	}
}

func TestConfigSecretSideEffectsMirrorVaultPost(t *testing.T) {
	patch := map[string]interface{}{
		"mqtt":      map[string]interface{}{"password": "  mqtt-fixture-secret "},
		"tailscale": map[string]interface{}{"tsnet": map[string]interface{}{"auth_key": "tskey-auth-abcdefghijklmnopqrstuvwx"}},
	}
	effects, err := prepareConfigSecretSideEffects(patch)
	if err != nil || effects.mqttPassword != "mqtt-fixture-secret" || effects.tsnetAuthKey != "tskey-auth-abcdefghijklmnopqrstuvwx" {
		t.Fatalf("effects = %+v, err = %v", effects, err)
	}
	masked, err := prepareConfigSecretSideEffects(map[string]interface{}{
		"mqtt": map[string]interface{}{"password": "••••••••"},
	})
	if err != nil || masked != (configSecretSideEffects{}) {
		t.Fatalf("masked patch produced effects: %+v %v", masked, err)
	}
	if _, err := prepareConfigSecretSideEffects(map[string]interface{}{
		"tailscale": map[string]interface{}{"tsnet": map[string]interface{}{"auth_key": "short"}},
	}); err == nil {
		t.Fatal("invalid tsnet key accepted")
	}

	// The MQTT hook republishes the vault password into the live snapshot.
	s := newMQTTConfigTestServer(t)
	if err := s.Vault.WriteSecret("mqtt_password", "mqtt-fixture-secret"); err != nil {
		t.Fatal(err)
	}
	before := s.ConfigSnapshot()
	if err := s.applyConfigSecretSideEffects(effects, false); err != nil {
		t.Fatal(err)
	}
	after := s.ConfigSnapshot()
	if after == before || after.MQTT.Password != "mqtt-fixture-secret" || before.MQTT.Password != "" {
		t.Fatal("MQTT password was not republished as a new snapshot")
	}
}

func TestGetTestEndpointsAcceptPostWithDraft(t *testing.T) {
	s := newDraftTestServer(t)

	// Video: the draft supplies the provider and key that the saved config lacks.
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		path    string
	}{
		{"video", handleVideoGenerationTest(s), "/api/video-generation/test"},
		{"image", handleImageGenerationTest(s), "/api/image-generation/test"},
		{"music", handleMusicGenerationTest(s), "/api/music-generation/test"},
		{"google", handleGoogleWorkspaceTest(s), "/api/google-workspace/test"},
		{"onedrive", handleOneDriveTest(s), "/api/onedrive/test"},
	} {
		rec := httptest.NewRecorder()
		tc.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code == http.StatusMethodNotAllowed {
			t.Errorf("%s: GET rejected", tc.name)
		}
		rec = httptest.NewRecorder()
		tc.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"draft":`)))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Invalid JSON") {
			t.Errorf("%s: bad POST body = %d %s", tc.name, rec.Code, rec.Body.String())
		}
		rec = httptest.NewRecorder()
		tc.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{}`)))
		if rec.Code == http.StatusMethodNotAllowed || rec.Code == http.StatusBadRequest {
			t.Errorf("%s: POST {} = %d %s", tc.name, rec.Code, rec.Body.String())
		}
		rec = httptest.NewRecorder()
		tc.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, tc.path, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: DELETE = %d, want 405", tc.name, rec.Code)
		}
	}

	// Draft really reaches the handler: enable video generation without a key.
	rec := httptest.NewRecorder()
	handleVideoGenerationTest(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"draft":{"video_generation":{"enabled":true}}}`)))
	if !strings.Contains(rec.Body.String(), "not configured") {
		t.Errorf("draft enable not seen by handler: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleVideoGenerationTest(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if !strings.Contains(rec.Body.String(), "not enabled") {
		t.Errorf("GET must use the saved config: %s", rec.Body.String())
	}
}

// newDraftProbeServer returns a server whose Home Assistant points at probe and
// records whether any request carried a credential.
func newTargetProbe(t *testing.T) (*httptest.Server, *int, *string) {
	t.Helper()
	hits := 0
	auth := ""
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		auth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"message":"API running."}`)
	}))
	t.Cleanup(probe.Close)
	return probe, &hits, &auth
}

func newDraftTestServerAt(t *testing.T, haURL string) *Server {
	t.Helper()
	s := newDraftTestServer(t)
	yamlText := "home_assistant:\n  enabled: true\n  url: " + haURL + "\nfrigate:\n  enabled: true\n  url: " + haURL + "\n"
	if err := os.WriteFile(s.Cfg.ConfigPath, []byte(yamlText), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(s.Cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = s.Cfg.ConfigPath
	if err := s.Vault.WriteSecret("frigate_api_token", "saved-frigate-token"); err != nil {
		t.Fatal(err)
	}
	cfg.ApplyVaultSecrets(s.Vault)
	s.Cfg = cfg
	s.initConfigSnapshot()
	return s
}

func postDraft(handler http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return rec
}

func TestDraftTargetChangeNeverSendsSavedSecrets(t *testing.T) {
	probe, hits, auth := newTargetProbe(t)
	s := newDraftTestServerAt(t, probe.URL)

	// Changed target + masked/empty secret: refused, nothing sent anywhere.
	for name, tc := range map[string]struct {
		handler http.HandlerFunc
		path    string
		body    string
	}{
		"ha url":         {handleHomeAssistantTest(s), "/ha", `{"draft":{"home_assistant":{"url":"http://127.0.0.1:1","access_token":"••••••••"}}}`},
		"ha url, no key": {handleHomeAssistantTest(s), "/ha", `{"draft":{"home_assistant":{"url":"http://127.0.0.1:1"}}}`},
		"frigate insec":  {handleFrigateTest(s), "/f", `{"draft":{"frigate":{"insecure":true,"api_token":"••••••••"}}}`},
		"frigate port":   {handleFrigateTest(s), "/f", `{"draft":{"frigate":{"internal_port":true}}}`},
	} {
		rec := postDraft(tc.handler, tc.path, tc.body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "enter the credentials again") {
			t.Errorf("%s: %d %s, want 400 re-enter credentials", name, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "saved-") {
			t.Errorf("%s: response leaks a saved secret", name)
		}
	}
	if *hits != 0 {
		t.Fatalf("a request reached the target although the target changed (auth %q)", *auth)
	}

	// Unchanged target + masked secret: the saved secret is used.
	rec := postDraft(handleFrigateTest(s), "/f", `{"draft":{"frigate":{"url":"`+probe.URL+`","api_token":"••••••••","readonly":true}}}`)
	if rec.Code != http.StatusOK || *auth != "Bearer saved-frigate-token" {
		t.Fatalf("unchanged target: %d %s auth=%q", rec.Code, rec.Body.String(), *auth)
	}

	// Changed target + typed secret: only the typed secret is used.
	probe2, hits2, auth2 := newTargetProbe(t)
	rec = postDraft(handleFrigateTest(s), "/f", `{"draft":{"frigate":{"url":"`+probe2.URL+`","api_token":"typed-frigate-token"}}}`)
	if rec.Code != http.StatusOK || *hits2 != 1 || *auth2 != "Bearer typed-frigate-token" {
		t.Fatalf("typed secret: %d %s hits=%d auth=%q", rec.Code, rec.Body.String(), *hits2, *auth2)
	}
}

func TestDraftTargetChangeClearsVaultOnlySecretsAndKeepsOtherSections(t *testing.T) {
	s := newDraftTestServer(t)
	_, err := s.draftTestConfig(draftRequest(`{"draft":{"fritzbox":{"host":"other.fritz.box"}}}`))
	if err == nil || draftTestConfigStatus(err) != http.StatusBadRequest {
		t.Fatalf("fritzbox host change without password: %v", err)
	}
	cfg, err := s.draftTestConfig(draftRequest(`{"draft":{"fritzbox":{"host":"other.fritz.box","password":"typed"}}}`))
	if err != nil || cfg.FritzBox.Password != "typed" {
		t.Fatalf("typed password: %v %q", err, cfg.FritzBox.Password)
	}
	// A different section keeps its saved secret.
	if cfg.HomeAssistant.AccessToken != "saved-ha-token" {
		t.Fatal("unrelated section lost its saved secret")
	}
}

func TestDraftProviderChangeResetsResolvedSlots(t *testing.T) {
	s := newDraftTestServer(t)
	yamlText := "providers:\n  - id: p1\n    name: P1\n    type: openai\n    base_url: http://img.invalid/v1\n    model: m1\nimage_generation:\n  enabled: true\n  provider: p1\n"
	if err := os.WriteFile(s.Cfg.ConfigPath, []byte(yamlText), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Vault.WriteSecret("provider_p1_api_key", "saved-image-key"); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(s.Cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = s.Cfg.ConfigPath
	cfg.ApplyVaultSecrets(s.Vault)
	cfg.ResolveProviders()
	s.Cfg = cfg
	s.initConfigSnapshot()
	if s.ConfigSnapshot().ImageGeneration.APIKey != "saved-image-key" {
		t.Fatal("fixture: provider key not resolved")
	}

	same, err := s.draftTestConfig(draftRequest(`{"draft":{"image_generation":{"enabled":true}}}`))
	if err != nil || same.ImageGeneration.APIKey != "saved-image-key" {
		t.Fatalf("unchanged provider: %v %q", err, same.ImageGeneration.APIKey)
	}
	cleared, err := s.draftTestConfig(draftRequest(`{"draft":{"image_generation":{"provider":""}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cleared.ImageGeneration.APIKey != "" || cleared.ImageGeneration.ProviderType != "" || cleared.ImageGeneration.BaseURL != "" {
		t.Fatalf("cleared provider still resolves: %+v", cleared.ImageGeneration)
	}
	unknown, err := s.draftTestConfig(draftRequest(`{"draft":{"image_generation":{"provider":"nope"}}}`))
	if err == nil && (unknown.ImageGeneration.APIKey != "" || unknown.ImageGeneration.BaseURL != "") {
		t.Fatalf("unknown provider keeps testing the old one: %+v", unknown.ImageGeneration)
	}
}

func TestRejectedPutLeavesNoSecretSideEffects(t *testing.T) {
	s := newMQTTConfigTestServer(t)
	before := s.ConfigSnapshot()
	file, _ := os.ReadFile(s.Cfg.ConfigPath)
	rec := putConfig(t, s, `{"mqtt":{"password":"late-rejected-fixture"},"server":{"https":{"enabled":true}},"security_proxy":{"enabled":true}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	if s.ConfigSnapshot() != before || before.MQTT.Password != "" {
		t.Fatal("rejected save republished the snapshot or MQTT password")
	}
	if after, _ := os.ReadFile(s.Cfg.ConfigPath); string(after) != string(file) {
		t.Fatal("rejected save changed the file")
	}
}

// Draft secrets are scoped to the request: they must not join the permanent
// scrubber registry and stop redacting once the request context ends.
func TestDraftTestConfigScopesTypedSecretsToTheRequest(t *testing.T) {
	s := newDraftTestServer(t)
	const secret = "scoped-draft-secret-Hq4v82Lm"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := draftRequest(`{"draft":{"home_assistant":{"access_token":"` + secret + `"}}}`).WithContext(ctx)
	if _, err := s.draftTestConfig(req); err != nil {
		t.Fatal(err)
	}
	if got := security.Scrub("token " + secret); strings.Contains(got, secret) {
		t.Fatalf("typed draft secret is not scrubbed while the request runs: %q", got)
	}
	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for strings.Contains(security.Scrub("token "+secret), "[redacted]") {
		if time.Now().After(deadline) {
			t.Fatal("typed draft secret stayed registered after the request ended")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
