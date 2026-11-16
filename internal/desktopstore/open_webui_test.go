package desktopstore

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOpenWebUIRelayTokensAreScopedAndNeverPersistedInEnvironment(t *testing.T) {
	secrets := &fakeSecretStore{data: map[string]string{}}
	docker := &fakeDockerAdapter{}
	s := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18080), secrets)
	s.cfg.LLMRelay = newDeepTutorRelay()
	op, err := s.StartInstall(t.Context(), InstallRequest{AppID: OpenWebUIAppID, BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RunOperation(t.Context(), op.ID); err != nil {
		t.Fatal(err)
	}
	app, _, err := s.GetInstalled(t.Context(), OpenWebUIAppID)
	if err != nil {
		t.Fatal(err)
	}
	token := secrets.data[RelayTokenKey(OpenWebUIAppID)]
	if token == "" || !hasOpenWebUIRelay(app) {
		t.Fatal("relay token not provisioned")
	}
	if strings.Contains(strings.Join(app.Env, "\n"), token) {
		t.Fatal("token persisted in Store environment")
	}
	if s.VerifyRelayToken(t.Context(), OpenWebUIAppID, token) {
		t.Fatal("install granted model access")
	}
	data, _ := json.Marshal(IntegrationSettings{Read: true, Revision: "test"})
	secrets.data[IntegrationKey(OpenWebUIAppID)] = string(data)
	if !s.VerifyRelayToken(t.Context(), OpenWebUIAppID, token) || s.VerifyRelayToken(t.Context(), DeepTutorAppID, token) {
		t.Fatal("token scope incorrect")
	}
	spec, err := s.runtimeContainerSpec(t.Context(), app)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(spec.Env, "OPENAI_API_KEY="+token) || containsString(spec.Env, "ENABLE_PERSISTENT_CONFIG=false") {
		t.Fatal("unsafe provider bootstrap")
	}
	stop, err := s.StartAppOperation(t.Context(), OpenWebUIAppID, OperationStop, OperationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if s.VerifyRelayToken(t.Context(), OpenWebUIAppID, token) {
		t.Fatal("lifecycle did not revoke token")
	}
	if err = s.RunOperation(t.Context(), stop.ID); err != nil {
		t.Fatal(err)
	}
	if s.VerifyRelayToken(t.Context(), OpenWebUIAppID, token) {
		t.Fatal("stopped app accepted token")
	}
}

func TestOpenWebUILegacyDoesNotRequireRelay(t *testing.T) {
	s := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18080))
	app := InstalledApp{AppID: OpenWebUIAppID, Status: AppStatusStopped}
	if err := s.saveInstalled(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	if s.requiresRelay(t.Context(), OpenWebUIAppID, OperationStart) {
		t.Fatal("legacy start requires relay")
	}
	needed, err := DeepTutorNeedsLogin(t.Context(), s.cfg.DBPath)
	if err != nil || needed {
		t.Fatalf("legacy install changed login requirements: %v", err)
	}
}
