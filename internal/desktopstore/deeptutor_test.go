package desktopstore

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	deepTutorFixtureOrigin = "http://host.docker.internal:8088"
	deepTutorTokenVaultKey = "desktop_store_deeptutor_llm_relay_token"
)

type fakeLLMRelay struct {
	mu       sync.Mutex
	info     RelayInfo
	err      error
	networks []string
}

func (f *fakeLLMRelay) RelayInfo(_ context.Context, networkMode string) (RelayInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.networks = append(f.networks, networkMode)
	if f.err != nil {
		return RelayInfo{}, f.err
	}
	info := f.info
	info.ExtraHosts = append([]string(nil), f.info.ExtraHosts...)
	return info, nil
}

func (f *fakeLLMRelay) set(info RelayInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.info = info
}

func newDeepTutorRelay() *fakeLLMRelay {
	return &fakeLLMRelay{info: RelayInfo{
		BaseURL:       deepTutorFixtureOrigin,
		ExtraHosts:    []string{"host.docker.internal:host-gateway"},
		Vision:        true,
		ContextWindow: 128000,
	}}
}

// installDeepTutor installs DeepTutor through the real install path with fake
// Docker, Desktop and Vault adapters.
func installDeepTutor(t *testing.T, relay LLMRelayProvider) (*Service, *fakeDockerAdapter, *fakeSecretStore) {
	t.Helper()
	docker := &fakeDockerAdapter{}
	secrets := &fakeSecretStore{data: map[string]string{}}
	s := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(13782), secrets)
	s.cfg.LLMRelay = relay
	op, err := s.StartInstall(context.Background(), InstallRequest{AppID: DeepTutorAppID, BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := s.RunOperation(context.Background(), op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	return s, docker, secrets
}

func TestDeepTutorCatalogEntry(t *testing.T) {
	var entry CatalogEntry
	for _, candidate := range DefaultCatalog() {
		if candidate.ID == DeepTutorAppID {
			entry = candidate
		}
	}
	if entry.ID == "" {
		t.Fatal("deeptutor missing from the catalog")
	}
	if !regexp.MustCompile(`^ghcr\.io/hkuds/deeptutor@sha256:[0-9a-f]{64}$`).MatchString(entry.Image) {
		t.Fatalf("image %q is not the digest pin of the official image", entry.Image)
	}
	if entry.Category != "ai" || entry.Icon != "book" || entry.LogoURL != "" || entry.Metadata["open_maximized"] != "true" {
		t.Fatalf("entry presentation = %#v", entry)
	}
	if entry.PrimaryPort.ContainerPort != 3782 || entry.PrimaryPort.Protocol != "tcp" {
		t.Fatalf("primary port = %#v", entry.PrimaryPort)
	}
	if len(entry.Volumes) != 1 || entry.Volumes[0].NameSuffix != "data" || entry.Volumes[0].ContainerPath != "/app/data" {
		t.Fatalf("volumes = %#v", entry.Volumes)
	}
	if len(entry.GeneratedSecrets) != 1 || entry.GeneratedSecrets[0] != (GeneratedSecret{Key: "llm_relay_token", Env: "AURAGO_DT_RELAY_TOKEN", Label: "AuraGo LLM relay token"}) {
		t.Fatalf("generated secrets = %#v (the token must stay unexposed)", entry.GeneratedSecrets)
	}
	if entry.Hardening != nil || len(entry.Env) != 0 || len(entry.HostBinds) != 0 || len(entry.Companions) != 0 {
		t.Fatalf("deeptutor must not opt into hardening, host binds or companions: %#v", entry)
	}
	if !strings.Contains(entry.Description, "no login of its own") {
		t.Fatalf("description must warn that DeepTutor has no login: %q", entry.Description)
	}
	if got := appReadinessTimeoutFor(InstalledApp{AppID: DeepTutorAppID}); got < 5*time.Minute {
		t.Fatalf("deeptutor readiness budget = %s, want a generous first-start budget", got)
	}
	if got := appReadinessTimeoutFor(InstalledApp{AppID: "n8n"}); got != appReadinessTimeout {
		t.Fatalf("other apps keep the default readiness budget, got %s", got)
	}
}

func TestInstallDeepTutorWiresRelayWrapperAndKeepsTokenOutOfTheRecord(t *testing.T) {
	relay := newDeepTutorRelay()
	s, docker, secrets := installDeepTutor(t, relay)

	token := secrets.data[deepTutorTokenVaultKey]
	if len(token) < 32 {
		t.Fatalf("generated relay token missing or short in the vault: %#v", secrets.data)
	}
	if len(docker.created) != 1 {
		t.Fatalf("created containers = %d, want 1", len(docker.created))
	}
	spec := docker.created[0]
	wantEntrypoint := []string{"/bin/sh", "-c", deepTutorWrapper, "aurago-deeptutor"}
	if strings.Join(spec.Entrypoint, "\x00") != strings.Join(wantEntrypoint, "\x00") {
		t.Fatalf("entrypoint = %#v", spec.Entrypoint)
	}
	for _, want := range []string{`python -c "$AURAGO_DT_MERGE_PY"`, `exec /app/entrypoint.sh "$@"`} {
		if !strings.Contains(spec.Entrypoint[2], want) {
			t.Fatalf("wrapper lacks %q: %s", want, spec.Entrypoint[2])
		}
	}
	for key, want := range map[string]string{
		"AURAGO_DT_RELAY_URL":   deepTutorFixtureOrigin + DeepTutorRelayPath,
		"AURAGO_DT_RELAY_TOKEN": token,
		"AURAGO_DT_VISION":      "1",
		"AURAGO_DT_EMBEDDINGS":  "0",
		"AURAGO_DT_CONTEXT":     "128000",
		"AURAGO_DT_MERGE_PY":    deepTutorMergeScript,
	} {
		if got, ok := envValue(spec.Env, key); !ok || got != want {
			t.Fatalf("container env %s = %q (set=%v), want %q", key, got, ok, want)
		}
	}
	if !containsString(spec.ExtraHosts, "host.docker.internal:host-gateway") {
		t.Fatalf("extra hosts = %#v", spec.ExtraHosts)
	}
	if len(relay.networks) == 0 || relay.networks[0] != "" {
		t.Fatalf("relay asked for networks %#v, want the default bridge", relay.networks)
	}

	app, ok, err := s.GetInstalled(context.Background(), DeepTutorAppID)
	if err != nil || !ok {
		t.Fatalf("installed record: ok=%v err=%v", ok, err)
	}
	if app.Status != AppStatusRunning || len(app.ExtraHosts) != 0 {
		t.Fatalf("record status/extra hosts = %q / %#v", app.Status, app.ExtraHosts)
	}
	if _, found := envValue(app.Env, "AURAGO_DT_RELAY_TOKEN"); found {
		t.Fatalf("record env carries the relay token: %#v", app.Env)
	}
	if len(app.SecretRefs) != 1 || app.SecretRefs[0].Expose || app.SecretRefs[0].VaultKey != deepTutorTokenVaultKey {
		t.Fatalf("secret refs = %#v", app.SecretRefs)
	}
	public, _ := json.Marshal(app)
	var envJSON, refsJSON, hostsJSON string
	if err := s.db.QueryRow(`SELECT env_json, secret_refs_json, extra_hosts_json FROM desktop_store_apps WHERE app_id = ?`, DeepTutorAppID).Scan(&envJSON, &refsJSON, &hostsJSON); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"public JSON": string(public), "env_json": envJSON, "secret_refs_json": refsJSON, "extra_hosts_json": hostsJSON} {
		if strings.Contains(text, token) || strings.Contains(text, "AURAGO_DT_MERGE_PY") || strings.Contains(text, "aurago_catalog_merge") {
			t.Fatalf("%s leaks the relay token or the merge script: %s", name, text)
		}
	}
}

func TestUpdateDeepTutorReResolvesRelayAndToken(t *testing.T) {
	relay := newDeepTutorRelay()
	s, docker, secrets := installDeepTutor(t, relay)
	oldToken := secrets.data[deepTutorTokenVaultKey]

	relay.set(RelayInfo{BaseURL: "http://aura-host.lan:9090", EmbeddingsAvailable: true})
	secrets.data[deepTutorTokenVaultKey] = "rotated-token-" + strings.Repeat("x", 32)
	op, err := s.StartAppOperation(context.Background(), DeepTutorAppID, OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunOperation(context.Background(), op.ID); err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(docker.created) != 2 {
		t.Fatalf("created containers = %d, want install + update", len(docker.created))
	}
	spec := docker.created[1]
	for key, want := range map[string]string{
		"AURAGO_DT_RELAY_URL":   "http://aura-host.lan:9090" + DeepTutorRelayPath,
		"AURAGO_DT_RELAY_TOKEN": secrets.data[deepTutorTokenVaultKey],
		"AURAGO_DT_VISION":      "0",
		"AURAGO_DT_EMBEDDINGS":  "1",
	} {
		if got, _ := envValue(spec.Env, key); got != want {
			t.Fatalf("updated container env %s = %q, want %q", key, got, want)
		}
	}
	if _, found := envValue(spec.Env, "AURAGO_DT_CONTEXT"); found {
		t.Fatalf("unknown context window must not be passed: %#v", spec.Env)
	}
	if containsString(spec.ExtraHosts, "host.docker.internal:host-gateway") || len(spec.Entrypoint) != 4 {
		t.Fatalf("updated spec extra hosts / entrypoint = %#v / %#v", spec.ExtraHosts, spec.Entrypoint)
	}
	app, _, _ := s.GetInstalled(context.Background(), DeepTutorAppID)
	var envJSON string
	if err := s.db.QueryRow(`SELECT env_json FROM desktop_store_apps WHERE app_id = ?`, DeepTutorAppID).Scan(&envJSON); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(envJSON, oldToken) || strings.Contains(envJSON, "rotated-token-") || len(app.SecretRefs) != 1 {
		t.Fatalf("update persisted a token or lost the secret ref: %s %#v", envJSON, app.SecretRefs)
	}
}

func TestInstallDeepTutorFailsClearlyWithoutAUsableRelay(t *testing.T) {
	cases := map[string]struct {
		relay LLMRelayProvider
		want  string
	}{
		"nil provider":   {relay: nil, want: "AuraGo LLM relay"},
		"provider error": {relay: &fakeLLMRelay{err: errors.New("AuraGo listens on loopback only")}, want: "AuraGo listens on loopback only"},
		"bad address":    {relay: &fakeLLMRelay{info: RelayInfo{BaseURL: "ftp://x"}}, want: "invalid"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			docker := &fakeDockerAdapter{}
			secrets := &fakeSecretStore{data: map[string]string{}}
			s := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(13782), secrets)
			s.cfg.LLMRelay = tc.relay
			op, err := s.StartInstall(context.Background(), InstallRequest{AppID: DeepTutorAppID, BindMode: BindModeLocal})
			if err != nil {
				t.Fatal(err)
			}
			err = s.RunOperation(context.Background(), op.ID)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("install error = %v, want text %q", err, tc.want)
			}
			stored, _ := s.Operation(context.Background(), op.ID)
			if stored.Status != OperationFailed || !strings.Contains(stored.Error, tc.want) {
				t.Fatalf("operation = %#v", stored)
			}
			if len(docker.pulled) != 0 || len(docker.created) != 0 || len(secrets.data) != 0 {
				t.Fatalf("a failed relay check must not pull, create or keep secrets: pulled=%v created=%d secrets=%v", docker.pulled, len(docker.created), secrets.data)
			}
			if _, found, _ := s.GetInstalled(context.Background(), DeepTutorAppID); found {
				t.Fatal("a failed install left a record")
			}
		})
	}
}

func TestVerifyRelayToken(t *testing.T) {
	ctx := context.Background()
	s, _, secrets := installDeepTutor(t, newDeepTutorRelay())
	token := secrets.data[deepTutorTokenVaultKey]

	check := func(name string, got, want bool) {
		t.Helper()
		if got != want {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
	check("matching token", s.VerifyRelayToken(ctx, DeepTutorAppID, token), true)
	check("matching token, app id needs normalizing", s.VerifyRelayToken(ctx, "  DeepTutor ", token), true)
	check("wrong token", s.VerifyRelayToken(ctx, DeepTutorAppID, token+"x"), false)
	check("prefix of the token", s.VerifyRelayToken(ctx, DeepTutorAppID, token[:len(token)-1]), false)
	check("empty token", s.VerifyRelayToken(ctx, DeepTutorAppID, ""), false)
	check("other app id", s.VerifyRelayToken(ctx, "n8n", token), false)
	check("empty app id", s.VerifyRelayToken(ctx, "", token), false)

	secrets.data[deepTutorTokenVaultKey] = ""
	check("empty vault secret", s.VerifyRelayToken(ctx, DeepTutorAppID, ""), false)
	check("empty vault secret with a token", s.VerifyRelayToken(ctx, DeepTutorAppID, token), false)
	delete(secrets.data, deepTutorTokenVaultKey)
	check("missing vault secret", s.VerifyRelayToken(ctx, DeepTutorAppID, token), false)

	secrets.data[deepTutorTokenVaultKey] = token
	uninstall, err := s.StartAppOperation(ctx, DeepTutorAppID, OperationUninstall, OperationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunOperation(ctx, uninstall.ID); err != nil {
		t.Fatal(err)
	}
	// An uninstall without delete_data keeps the Vault secret, but the app is gone.
	if secrets.data[deepTutorTokenVaultKey] != token {
		t.Fatal("test setup: the vault token should survive an uninstall without delete_data")
	}
	check("uninstalled app", s.VerifyRelayToken(ctx, DeepTutorAppID, token), false)

	var nobody *Service
	check("nil service", nobody.VerifyRelayToken(ctx, DeepTutorAppID, token), false)
}

func TestDockerCreatePayloadEntrypoint(t *testing.T) {
	plain := dockerCreatePayload(ContainerSpec{Name: "a", Image: "x"})
	if _, found := plain["Entrypoint"]; found {
		t.Fatalf("a spec without an entrypoint must keep the image's own: %#v", plain)
	}
	spec := ContainerSpec{Name: "a", Image: "x", Entrypoint: []string{"/bin/sh", "-c", "echo hi", "wrapper"}}
	payload := dockerCreatePayload(spec)
	spec.Entrypoint[0] = "mutated"
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"Entrypoint":["/bin/sh","-c","echo hi","wrapper"]`) {
		t.Fatalf("payload does not carry the entrypoint as a JSON array: %s", raw)
	}
}
