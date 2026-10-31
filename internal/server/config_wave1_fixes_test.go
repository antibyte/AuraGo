package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
)

// setSavedConfig edits the saved configuration of a draft test server in place
// and republishes it as the snapshot.
func setSavedConfig(s *Server, edit func(cfg *config.Config)) {
	edit(s.Cfg)
	s.initConfigSnapshot()
}

// B: deploy_port and deploy_user decide where the SFTP password is sent.
func TestHomepageTestPortOrUserChangeDropsSavedDeployPassword(t *testing.T) {
	for name, draft := range map[string]string{
		"port": `{"draft":{"homepage":{"deploy_port":1}}}`,
		"user": `{"draft":{"homepage":{"deploy_user":"someone-else"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := newDraftTestServer(t)
			setSavedConfig(s, func(cfg *config.Config) {
				cfg.Homepage.DeployHost = "127.0.0.1"
				cfg.Homepage.DeployPort = 22
				cfg.Homepage.DeployUser = "deploy"
				cfg.Homepage.DeployPassword = "saved-deploy-pw-Wk39"
			})
			rec := postDraft(handleHomepageTestConnection(s), "/api/homepage/test-connection", draft)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "enter the credentials again") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "saved-deploy-pw-Wk39") {
				t.Fatalf("response leaked the password: %s", rec.Body.String())
			}
		})
	}
}

// C: an identifier is not a secret.
func TestCredentialIDIsNotADraftSecret(t *testing.T) {
	field, ok := reflect.TypeOf(config.VirtualComputersControlPlane{}).FieldByName("CredentialID")
	if !ok {
		t.Fatal("CredentialID field is missing")
	}
	if isDraftSecretField(field) {
		t.Fatal("control_plane.credential_id must not count as a secret")
	}
	secret, _ := reflect.TypeOf(config.VirtualComputersControlPlane{}).FieldByName("SSHSecret")
	if !isDraftSecretField(secret) {
		t.Fatal("control_plane SSH secret must stay a secret")
	}
}

func newVirtualComputersDraftServer(t *testing.T, mode string) *Server {
	t.Helper()
	s := newDraftTestServer(t)
	setSavedConfig(s, func(cfg *config.Config) {
		vc := &cfg.VirtualComputers
		vc.AllowVolumes = true
		vc.Storage.Mode = mode
		vc.Storage.Endpoint = "saved-s3.invalid:9000"
		vc.Storage.Bucket = "boring-volumes"
		vc.ControlPlane.Host = "saved-host.invalid"
		vc.ControlPlane.CredentialID = "cred-1"
		vc.BoringToken = "saved-boring-token-Pz81"
		vc.S3AccessKeyID = "saved-s3-access-Pz81"
		vc.S3SecretKey = "saved-s3-secret-Pz81"
		vc.GarageAccessKeyID = "saved-garage-access-Pz81"
		vc.GarageSecretKey = "saved-garage-secret-Pz81"
		vc.GarageRPCSecret = "saved-garage-rpc-Pz81"
	})
	return s
}

func TestVirtualComputersExternalS3EndpointChangeNeedsTypedS3KeysOnly(t *testing.T) {
	s := newVirtualComputersDraftServer(t, config.VirtualComputersStorageModeExternalS3)
	const endpoint = `"storage":{"mode":"external_s3","endpoint":"other-s3.invalid:9000"}`

	cfg, err := s.draftTestConfig(draftRequest(`{"draft":{"virtual_computers":{` + endpoint + `,"s3_access_key_id":"typed-access-Qd47","s3_secret_key":"typed-secret-Qd47"}}}`))
	if err != nil {
		t.Fatalf("typed S3 keys must be enough: %v", err)
	}
	vc := cfg.VirtualComputers
	if vc.S3AccessKeyID != "typed-access-Qd47" || vc.S3SecretKey != "typed-secret-Qd47" {
		t.Fatalf("typed keys were not used: %q %q", vc.S3AccessKeyID, vc.S3SecretKey)
	}
	if vc.GarageSecretKey != "" || vc.BoringToken != "" || vc.GarageRPCSecret != "" {
		t.Fatal("saved secrets of the section followed the new target")
	}

	// A control plane host change must not trip over the credential id.
	cfg, err = s.draftTestConfig(draftRequest(`{"draft":{"virtual_computers":{"control_plane":{"host":"elsewhere.invalid"},"s3_access_key_id":"typed-access-Qd47","s3_secret_key":"typed-secret-Qd47"}}}`))
	if err != nil {
		t.Fatalf("control plane change with typed keys: %v", err)
	}
	if cfg.VirtualComputers.ControlPlane.CredentialID != "cred-1" {
		t.Fatalf("credential id = %q, want it kept", cfg.VirtualComputers.ControlPlane.CredentialID)
	}

	_, err = s.draftTestConfig(draftRequest(`{"draft":{"virtual_computers":{` + endpoint + `}}}`))
	if err == nil || !strings.Contains(err.Error(), "enter the credentials again") || draftTestConfigStatus(err) != http.StatusBadRequest {
		t.Fatalf("without typed keys: err = %v", err)
	}
}

func TestVirtualComputersManagedEndpointChangeAsksToSaveFirst(t *testing.T) {
	s := newVirtualComputersDraftServer(t, config.VirtualComputersStorageModeManagedGarage)
	_, err := s.draftTestConfig(draftRequest(`{"draft":{"virtual_computers":{"storage":{"endpoint":"other-s3.invalid:9000"}}}}`))
	if err == nil || !strings.Contains(err.Error(), "save it first") || draftTestConfigStatus(err) != http.StatusBadRequest {
		t.Fatalf("managed target change: err = %v", err)
	}
}

// D: the modal sends no base_url for the auto-URL provider types, the saved entry
// carries the resolved one. The saved key must still follow an unchanged target.
func TestProviderDraftWithoutURLKeepsSavedKeyForAutoURLTypes(t *testing.T) {
	fixture := &authRecorder{}
	srv := httptest.NewServer(fixture)
	defer srv.Close()
	yamlText := fmt.Sprintf("manifest:\n  mode: external\n  external_base_url: %s/v1\nproviders:\n  - id: mf\n    type: manifest\n    model: fixture\n  - id: cf\n    type: cloudflare-rest\n    account_id: acc123\n    model: fixture\nllm:\n  provider: mf\n", srv.URL)
	s, vault := newProviderTestServer(t, yamlText)
	_ = vault.WriteSecret("provider_mf_api_key", "saved-manifest-key-Lr62-aaaa")
	_ = vault.WriteSecret("provider_cf_api_key", "saved-cloudflare-key-Lr62-bbbb")
	s.Cfg.ApplyVaultSecrets(vault)
	s.Cfg.ResolveProviders()

	// Full probe for manifest: the resolved URL is hit with the saved key.
	rec := httptest.NewRecorder()
	body := `{"provider_id":"mf","probe_kind":"metadata","provider":{"type":"manifest","base_url":"","api_key":"••••••••","model":"fixture"}}`
	handleProviderTest(s).ServeHTTP(rec, httptest.NewRequest("POST", "/api/providers/test", strings.NewReader(body)))
	if !fixture.saw("Bearer saved-manifest-key-Lr62-aaaa") {
		t.Fatalf("manifest draft without URL did not use the saved key at the resolved URL: %d %s auth=%v", rec.Code, rec.Body.String(), fixture.auths)
	}

	// cloudflare-rest: the account URL is derived like on load.
	saved := s.Cfg.FindProvider("cf")
	draft := (&providerTestDraft{Type: "cloudflare-rest", AccountID: "acc123", APIKey: "••••••••", Model: "fixture"}).entry("cf", saved, s.Cfg)
	if want := "https://api.cloudflare.com/client/v4/accounts/acc123/ai/v1"; draft.BaseURL != want {
		t.Fatalf("draft URL = %q, want %q", draft.BaseURL, want)
	}
	if draft.APIKey != "saved-cloudflare-key-Lr62-bbbb" {
		t.Fatalf("saved key not reused for the unchanged target: %q", draft.APIKey)
	}
	other := (&providerTestDraft{Type: "cloudflare-rest", AccountID: "other456", APIKey: "••••••••", Model: "fixture"}).entry("cf", saved, s.Cfg)
	if other.APIKey != "" {
		t.Fatal("saved key followed a changed account")
	}
}

// A saved entry may keep OAuth endpoints; an API-key draft sends none.
func TestProviderDraftSameTargetIgnoresOAuthURLsForAPIKeyAuth(t *testing.T) {
	saved := &config.ProviderEntry{Type: "openai", BaseURL: "https://x.invalid/v1", OAuthTokenURL: "https://x.invalid/token"}
	apiKey := config.ProviderEntry{Type: "openai", BaseURL: "https://x.invalid/v1"}
	if !providerDraftSameTarget(saved, apiKey) {
		t.Fatal("stale OAuth URLs made an API-key draft look like another target")
	}
	oauth := apiKey
	oauth.AuthType = "oauth2"
	if providerDraftSameTarget(saved, oauth) {
		t.Fatal("an OAuth draft without the saved token URL must count as changed")
	}
}

// E: the MeshCentral test runs on the draft; saved secrets follow an unchanged target only.
func TestMeshCentralTestUsesDraftAndTargetRule(t *testing.T) {
	var mu sync.Mutex
	seen := map[string][]string{}
	record := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				var login struct{ U, P string }
				_ = json.NewDecoder(r.Body).Decode(&login)
				mu.Lock()
				seen[name] = append(seen[name], login.U+"|"+login.P)
				mu.Unlock()
			}
			http.Error(w, "nope", http.StatusForbidden)
		})
	}
	saved, other := httptest.NewServer(record("saved")), httptest.NewServer(record("other"))
	defer saved.Close()
	defer other.Close()
	creds := func(user, password string) string {
		return user + "|" + password
	}
	newServer := func() *Server {
		s := newDraftTestServer(t)
		setSavedConfig(s, func(cfg *config.Config) {
			cfg.MeshCentral.URL = saved.URL
			cfg.MeshCentral.Username = "mesh-admin"
			cfg.MeshCentral.Password = "saved-mesh-pw-Vx58"
		})
		return s
	}
	post := func(s *Server, body string) *httptest.ResponseRecorder {
		return postDraft(handleMeshCentralTest(s), "/api/meshcentral/test", body)
	}
	got := func(name string) []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen[name]...)
	}

	// Unchanged target (empty body and masked legacy fields): the saved password is used.
	s := newServer()
	for _, body := range []string{``, `{"url":"","username":"","password":"","login_token":""}`, `{"draft":{"meshcentral":{"password":"••••••••"}}}`} {
		before := len(got("saved"))
		rec := post(s, body)
		if rec.Code != http.StatusOK || len(got("saved")) != before+1 || got("saved")[before] != creds("mesh-admin", "saved-mesh-pw-Vx58") {
			t.Fatalf("body %q: status=%d saved requests=%v resp=%s", body, rec.Code, got("saved"), rec.Body.String())
		}
	}

	// A typed password in the legacy flat body or the draft form reaches the saved server.
	before := len(got("saved"))
	post(s, `{"password":"typed-mesh-pw-Vx58"}`)
	post(s, `{"draft":{"meshcentral":{"password":"typed-mesh-pw-Vx59"}}}`)
	if now := got("saved"); len(now) != before+2 || now[before] != creds("mesh-admin", "typed-mesh-pw-Vx58") || now[before+1] != creds("mesh-admin", "typed-mesh-pw-Vx59") {
		t.Fatalf("typed passwords were not used: %v", now)
	}

	// Another URL (any origin) is tested with the typed credentials only.
	rec := post(s, `{"url":"`+other.URL+`","username":"someone","password":"typed-other-pw-Vx58"}`)
	if rec.Code != http.StatusOK || len(got("other")) != 1 || got("other")[0] != creds("someone", "typed-other-pw-Vx58") {
		t.Fatalf("other origin: status=%d requests=%v resp=%s", rec.Code, got("other"), rec.Body.String())
	}

	// Changed target without a typed password: the saved one never travels.
	for name, body := range map[string]string{
		"url":      `{"url":"` + other.URL + `"}`,
		"insecure": `{"draft":{"meshcentral":{"insecure":true}}}`,
		"ca_file":  `{"ca_file":"/etc/other-ca.pem"}`,
		"username": `{"username":"someone-else"}`,
	} {
		beforeSaved, beforeOther := len(got("saved")), len(got("other"))
		rec := post(s, body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "enter the credentials again") {
			t.Fatalf("%s change: status=%d body=%s", name, rec.Code, rec.Body.String())
		}
		if len(got("saved")) != beforeSaved || len(got("other")) != beforeOther {
			t.Fatalf("%s change still contacted a server", name)
		}
	}
}
