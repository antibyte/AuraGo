package desktopstore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestGodsEyeConfigurationLifecycle(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	desktop := &fakeDesktopAdapter{}
	links := &fakeLaunchpadAdapter{}
	vault := &fakeSecretStore{data: map[string]string{godsEyeVaultKey: ""}}
	s := newTestServiceWithSecrets(t, docker, desktop, links, func(context.Context, int) (int, error) { return 14173, nil }, vault)
	op, err := s.StartInstall(ctx, InstallRequest{AppID: GodsEyeAppID, BindMode: BindModeLocal, AllowedOrigins: []string{"http://localhost:8088"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunOperation(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	if desktop.installed.ID != "store-gods-eye-view" || desktop.installed.Metadata["open_maximized"] != "true" || desktop.shortcutAppID != "store-gods-eye-view" || links.upserted.URL != "aurago-store://gods-eye-view" {
		t.Fatal("desktop registration missing")
	}
	app, _, _ := s.GetInstalled(ctx, GodsEyeAppID)
	if len(app.HostBinds) != 0 || app.HostIP != "127.0.0.1" || len(app.Volumes) != 1 {
		t.Fatal("unexpected installation boundary")
	}
	const key = "synthetic-gev-provider-credential-592861"
	providerKeys := map[string]string{}
	for _, name := range godsEyeKeys {
		providerKeys[name] = "fixture-provider-" + name
	}
	providerKeys["OPENAI_API_KEY"] = key
	op, err = s.ConfigureGodsEye(ctx, GodsEyeConfigUpdate{Keys: providerKeys})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfigureGodsEye(ctx, GodsEyeConfigUpdate{}); !errors.Is(err, ErrOperationInProgress) {
		t.Fatalf("concurrent configuration admitted: %v", err)
	}
	status, err := s.GodsEyeConfiguration(ctx)
	if err != nil || !status.Pending || !status.Configured["OPENAI_API_KEY"] {
		t.Fatalf("desired status missing: %v", err)
	}
	pulls := len(docker.pulled)
	if err := s.RunOperation(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	if len(docker.pulled) != pulls {
		t.Fatal("configure pulled an image")
	}
	app, _, _ = s.GetInstalled(ctx, GodsEyeAppID)
	spec, err := s.runtimeContainerSpec(app)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := envValue(spec.Env, "OPENAI_API_KEY"); value != key {
		t.Fatal("runtime key missing")
	}
	for name, expected := range providerKeys {
		if value, _ := envValue(spec.Env, name); value != expected {
			t.Fatalf("missing runtime provider %s", name)
		}
	}
	status, _ = s.GodsEyeConfiguration(ctx)
	if status.Pending {
		t.Fatal("successful configuration still pending")
	}
	public, _ := json.Marshal(status)
	if strings.Contains(string(public), key) || strings.Contains(strings.Join(app.Env, "\n"), key) {
		t.Fatal("key leaked outside runtime/vault")
	}
	var persisted string
	if err := s.db.QueryRowContext(ctx, "SELECT env_json FROM desktop_store_apps WHERE app_id = ?", GodsEyeAppID).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(persisted, key) {
		t.Fatal("key persisted in SQLite")
	}
	ledger, _ := s.Operation(ctx, op.ID)
	if strings.Contains(ledger.RequestJSON, key) {
		t.Fatal("key persisted in operation")
	}

	// A final database write failure must also restore the running container.
	_, err = s.db.Exec(`CREATE TEMP TRIGGER gev_fail_final_save BEFORE UPDATE ON desktop_store_apps
		WHEN NEW.app_id = 'gods-eye-view' AND NEW.last_operation_state = 'succeeded'
		BEGIN SELECT RAISE(FAIL, 'fixture persistence failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	op, err = s.ConfigureGodsEye(ctx, GodsEyeConfigUpdate{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("expected final persistence failure")
	}
	if _, err := s.db.Exec(`DROP TRIGGER gev_fail_final_save`); err != nil {
		t.Fatal(err)
	}
	app, _, _ = s.GetInstalled(ctx, GodsEyeAppID)
	if app.Status != AppStatusRunning || app.LastOperationState != OperationFailed {
		t.Fatal("final write failure lost the previous container")
	}

	// A failed recreation must restore the old keys, not the desired new ones.
	op, err = s.ConfigureGodsEye(ctx, GodsEyeConfigUpdate{Keys: map[string]string{"OPENAI_API_KEY": "synthetic-new-credential-694312"}})
	if err != nil {
		t.Fatal(err)
	}
	docker.startErrors = []error{errors.New("fixture startup failure with synthetic-new-credential-694312"), nil}
	if err := s.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("expected failure")
	}
	app, _, _ = s.GetInstalled(ctx, GodsEyeAppID)
	spec, err = s.runtimeContainerSpec(app)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := envValue(spec.Env, "OPENAI_API_KEY"); value != key {
		t.Fatal("rollback changed active keys")
	}
	status, _ = s.GodsEyeConfiguration(ctx)
	if !status.Pending || app.Status != AppStatusRunning {
		t.Fatal("failed apply lost previous running configuration")
	}
	ledger, _ = s.Operation(ctx, op.ID)
	if strings.Contains(ledger.Error, "synthetic-new-credential-694312") || strings.Contains(app.Error, "synthetic-new-credential-694312") {
		t.Fatal("provider error leaked a credential")
	}

	// Clearing is explicit and survives the normal image update path.
	op, err = s.ConfigureGodsEye(ctx, GodsEyeConfigUpdate{Clear: []string{"OPENAI_API_KEY"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunOperation(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{OperationStop, OperationStart, OperationUpdate} {
		op, err = s.StartAppOperation(ctx, GodsEyeAppID, action, OperationRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.RunOperation(ctx, op.ID); err != nil {
			t.Fatal(err)
		}
	}
	status, _ = s.GodsEyeConfiguration(ctx)
	if status.Configured["OPENAI_API_KEY"] {
		t.Fatal("removed key reappeared")
	}
	for _, deleteData := range []bool{false, true} {
		op, err = s.StartAppOperation(ctx, GodsEyeAppID, OperationUninstall, OperationRequest{DeleteData: deleteData})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.RunOperation(ctx, op.ID); err != nil {
			t.Fatal(err)
		}
		_, kept := vault.data[godsEyeVaultKey]
		if kept == deleteData {
			t.Fatal("settings retention does not match delete_data")
		}
		if !deleteData {
			op, err = s.StartInstall(ctx, InstallRequest{AppID: GodsEyeAppID, BindMode: BindModeLocal})
			if err != nil {
				t.Fatal(err)
			}
			if err := s.RunOperation(ctx, op.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestGodsEyeRejectsInvalidOriginsAndKeys(t *testing.T) {
	for _, origin := range []string{"https://*.example.com", "https://example.com/path", "javascript:alert(1)", "https://user:pass@example.com", "https://example.com?x=1", "https://example.com#x", "https://example.com;https:", "https://example.com\n.evil", "https://example.com:65536", "https://example.com:", "https://example.com:0"} {
		if _, err := validateGodsEyeOrigins([]string{origin}); err == nil {
			t.Fatalf("accepted %q", origin)
		}
	}
	if got, err := validateGodsEyeOrigins([]string{"https://a.example:8443/", "https://a.example:8443"}); err != nil || len(got) != 1 {
		t.Fatal("origin normalization failed")
	}
	s := &Service{}
	for _, req := range []GodsEyeConfigUpdate{{Keys: map[string]string{"UNKNOWN_KEY": "x"}}, {Keys: map[string]string{"OPENAI_API_KEY": "line\nbreak"}}, {Keys: map[string]string{"OPENAI_API_KEY": strings.Repeat("x", 513)}}, {Keys: map[string]string{"OPENAI_API_KEY": "x"}, Clear: []string{"OPENAI_API_KEY"}}} {
		if _, err := s.ConfigureGodsEye(context.Background(), req); err == nil {
			t.Fatal("accepted invalid settings")
		}
	}
}
