package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/infra"
	"aurago/internal/inventory"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func newDesktopInfraWriteFixture(t *testing.T) (*Server, *config.Config, *desktopInfraRuntime) {
	t.Helper()
	cfg := &config.Config{}
	configureDesktopInfraTestWrite(cfg)
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "mock://docker"
	s := &Server{Cfg: cfg}
	s.initConfigSnapshot()
	runtime := &desktopInfraRuntime{cache: make(map[string]*desktopInfraProviderCache)}
	s.infraRuntime = runtime
	return s, cfg, runtime
}

func beginDesktopInfraTestOperation(t *testing.T, s *Server, cfg *config.Config, runtime *desktopInfraRuntime) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, finish := runtime.beginOperation(context.Background(), desktopInfraRuntimeConfigKey(s, cfg), cfg)
	if ctx.Err() != nil {
		t.Fatal("Infra operation was not admitted")
	}
	return ctx, finish
}

func dockerInspectFixture(id, name, image, status string) []byte {
	return []byte(`{"Id":"` + id + `","Name":"/` + name + `","Config":{"Image":"` + image + `","Labels":{}},"State":{"Status":"` + status + `"}}`)
}

func TestDesktopInfraControlRevocationCancelsOnlyInfraOperationBeforePost(t *testing.T) {
	s, cfg, runtime := newDesktopInfraWriteFixture(t)
	infraCtx, finishInfra := beginDesktopInfraTestOperation(t, s, cfg, runtime)
	defer finishInfra()
	desktopCtx, finishDesktop, err := s.beginDesktopRun(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer finishDesktop()

	writes := 0
	runtime.dockerRequest = func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if method == http.MethodGet && endpoint == "/containers/deadbeef/json" {
			next := *s.ConfigSnapshot()
			next.VirtualDesktop.Infra.AllowControl = false
			s.replaceConfigSnapshot(&next)
			if infraCtx.Err() == nil {
				t.Fatal("Infra grant revocation did not cancel the active Infra operation")
			}
			if desktopCtx.Err() != nil {
				t.Fatalf("Infra grant revocation canceled an unrelated Desktop run: %v", desktopCtx.Err())
			}
			return dockerInspectFixture("deadbeef", "web", "nginx", "exited"), http.StatusOK, nil
		}
		if method == http.MethodPost {
			writes++
		}
		return nil, 0, errors.New("unexpected Docker request")
	}
	source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "deadbeef"}}
	result := desktopInfraRunAction(infraCtx, s, cfg, nil, runtime, "", source, desktopInfraActionRequest{Action: "start", RequestID: "revoked-control"})
	if result.Status != "failed" || writes != 0 {
		t.Fatalf("revoked control result=%+v, provider POSTs=%d; want failed and zero POSTs", result, writes)
	}
	if desktopCtx.Err() != nil {
		t.Fatalf("unrelated Desktop run ended after Infra revocation: %v", desktopCtx.Err())
	}
}

func TestDesktopInfraSourceEditRevocationAndProviderDisableBlockRename(t *testing.T) {
	s, cfg, runtime := newDesktopInfraWriteFixture(t)
	infraCtx, finish := beginDesktopInfraTestOperation(t, s, cfg, runtime)
	defer finish()
	writes := 0
	runtime.dockerRequest = func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if method == http.MethodGet && endpoint == "/containers/deadbeef/json" {
			next := *s.ConfigSnapshot()
			next.VirtualDesktop.Infra.AllowSourceEdit = false
			next.Docker.Enabled = false
			next.Docker.ReadOnly = true
			s.replaceConfigSnapshot(&next)
			if infraCtx.Err() == nil {
				t.Fatal("source-edit or Docker policy change did not cancel the Infra operation")
			}
			return dockerInspectFixture("deadbeef", "web", "nginx", "running"), http.StatusOK, nil
		}
		if method == http.MethodPost {
			writes++
		}
		return nil, 0, errors.New("unexpected Docker request")
	}
	source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "deadbeef"}}
	before := desktopInfraDockerInspect{ID: "deadbeef", Name: "/web"}
	before.Config.Image = "nginx"
	before.State.Status = "running"
	result, status, err := desktopInfraWriteSourceEdit(infraCtx, s, cfg, nil, runtime, "", source, desktopInfraSourceEditRequest{
		Source: "docker", Version: desktopInfraDockerVersion(before), Fields: []byte(`{"name":"web-new"}`),
	})
	if err == nil || status != http.StatusForbidden || writes != 0 {
		t.Fatalf("revoked rename result=%+v status=%d err=%v POSTs=%d; want forbidden and zero POSTs", result, status, err, writes)
	}
}

func TestDesktopInfraProviderIdentityChangeBlocksPreparedAction(t *testing.T) {
	s, cfg, runtime := newDesktopInfraWriteFixture(t)
	infraCtx, finish := beginDesktopInfraTestOperation(t, s, cfg, runtime)
	defer finish()
	writes := 0
	runtime.dockerRequest = func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if method == http.MethodGet && endpoint == "/containers/deadbeef/json" {
			next := *s.ConfigSnapshot()
			next.Docker.Host = "mock://different-docker"
			s.replaceConfigSnapshot(&next)
			return dockerInspectFixture("deadbeef", "web", "nginx", "exited"), http.StatusOK, nil
		}
		if method == http.MethodPost {
			writes++
		}
		return nil, 0, errors.New("unexpected Docker request")
	}
	source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "deadbeef"}}
	result := desktopInfraRunAction(infraCtx, s, cfg, nil, runtime, "", source, desktopInfraActionRequest{Action: "start", RequestID: "changed-provider"})
	if result.Status != "failed" || writes != 0 {
		t.Fatalf("provider identity change result=%+v, POSTs=%d; want failed and zero POSTs", result, writes)
	}
}

func TestDesktopInfraNodeUnlinkDuringPreflightBlocksProviderAction(t *testing.T) {
	s, cfg, runtime := newDesktopInfraWriteFixture(t)
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := infra.New(db)
	if err != nil {
		t.Fatal(err)
	}
	identity := infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "deadbeef"}
	observation := infra.SourceObservation{
		Identity: identity, Name: "web",
		Observed: infra.SourceObserved{Kind: infra.NodeContainer, Name: "web", Icon: "container"},
	}
	if err := store.ObserveSources(context.Background(), "docker", identity.Instance, []infra.SourceObservation{observation}, true); err != nil {
		t.Fatal(err)
	}
	candidate, err := store.GetCandidate(context.Background(), infra.CandidateID(identity))
	if err != nil {
		t.Fatal(err)
	}
	node, err := store.CreateImportedNode(context.Background(), candidate.Node, identity)
	if err != nil {
		t.Fatal(err)
	}
	infraCtx, finish := beginDesktopInfraTestOperation(t, s, cfg, runtime)
	defer finish()
	writes := 0
	runtime.dockerRequest = func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if method == http.MethodGet && endpoint == "/containers/deadbeef/json" {
			if err := store.DeleteNode(context.Background(), node.ID, node.Version); err != nil {
				t.Fatalf("delete node during provider preflight: %v", err)
			}
			return dockerInspectFixture("deadbeef", "web", "nginx", "exited"), http.StatusOK, nil
		}
		if method == http.MethodPost {
			writes++
		}
		return nil, 0, errors.New("unexpected Docker request")
	}
	result := desktopInfraRunAction(infraCtx, s, cfg, store, runtime, node.ID, node.Sources[0], desktopInfraActionRequest{Action: "start", RequestID: "unlinked-source"})
	if result.Status != "failed" || writes != 0 {
		t.Fatalf("unlinked source result=%+v, provider POSTs=%d; want failed and zero POSTs", result, writes)
	}
}

func TestDesktopInfraUnrelatedConfigSaveKeepsInfraOperationAndReadmitsWrites(t *testing.T) {
	s, cfg, runtime := newDesktopInfraWriteFixture(t)
	active, finishActive := beginDesktopInfraTestOperation(t, s, cfg, runtime)
	defer finishActive()
	next := *s.ConfigSnapshot()
	next.LLMRouter.Enabled = !next.LLMRouter.Enabled
	s.replaceConfigSnapshot(&next)
	if active.Err() != nil {
		t.Fatalf("unrelated config save canceled active Infra operation: %v", active.Err())
	}

	newCtx, finishNew := runtime.beginOperation(context.Background(), desktopInfraRuntimeConfigKey(s, &next), &next)
	defer finishNew()
	if newCtx.Err() != nil {
		t.Fatalf("unrelated config save left future Infra admission stale: %v", newCtx.Err())
	}
}

func TestDesktopInfraAdmissionFenceRejectsRequestCapturedBeforeRuntimeCreation(t *testing.T) {
	cfg := &config.Config{}
	configureDesktopInfraTestWrite(cfg)
	s := &Server{Cfg: cfg}
	s.initConfigSnapshot()
	oldSnapshot := s.ConfigSnapshot()
	next := *oldSnapshot
	next.VirtualDesktop.Infra.AllowSourceEdit = false
	s.replaceConfigSnapshot(&next)

	// Model the race where access captured oldSnapshot, config publication saw
	// no runtime to cancel, and the request lazily initializes Infra afterward.
	runtime := &desktopInfraRuntime{cache: make(map[string]*desktopInfraProviderCache)}
	ctx, finish := runtime.beginOperation(context.Background(), desktopInfraRuntimeConfigKey(s, oldSnapshot), oldSnapshot)
	defer finish()
	if ctx.Err() != nil {
		t.Fatal("fixture request was unexpectedly rejected before the admission fence")
	}
	if s.desktopInfraConfigAdmitted(oldSnapshot) {
		t.Fatal("stale request snapshot passed admission after config publication")
	}
	if !s.desktopInfraConfigAdmitted(&next) {
		t.Fatal("current config snapshot was not admitted")
	}
}

func TestDesktopInfraSentActionRemainsUnknownAfterRevocation(t *testing.T) {
	s, cfg, runtime := newDesktopInfraWriteFixture(t)
	infraCtx, finish := beginDesktopInfraTestOperation(t, s, cfg, runtime)
	defer finish()
	writes := 0
	runtime.dockerRequest = func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if method == http.MethodGet && endpoint == "/containers/deadbeef/json" {
			return dockerInspectFixture("deadbeef", "web", "nginx", "exited"), http.StatusOK, nil
		}
		if method == http.MethodPost && endpoint == "/containers/deadbeef/start" {
			writes++
			next := *s.ConfigSnapshot()
			next.VirtualDesktop.Infra.AllowControl = false
			s.replaceConfigSnapshot(&next)
			return nil, 0, context.Canceled
		}
		return nil, 0, errors.New("unexpected Docker request")
	}
	source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "deadbeef"}}
	result := desktopInfraRunAction(infraCtx, s, cfg, nil, runtime, "", source, desktopInfraActionRequest{Action: "start", RequestID: "sent-before-revoke"})
	if result.Status != infra.ActionUnknown || writes != 1 {
		t.Fatalf("post-dispatch revocation result=%+v, POSTs=%d; want unknown and exactly one POST", result, writes)
	}
}

func TestDesktopInfraVaultCredentialChangeBlocksPreparedFritzWriteWithoutStalingAdmission(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("fritzbox_password", "before-rotation"); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	configureDesktopInfraTestWrite(cfg)
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.Host = "fritz.lan"
	cfg.FritzBox.Network.Enabled = true
	cfg.FritzBox.Network.SubFeatures.Hosts = true
	s := &Server{Cfg: cfg, Vault: vault}
	s.initConfigSnapshot()
	runtime := &desktopInfraRuntime{cache: make(map[string]*desktopInfraProviderCache)}
	s.infraRuntime = runtime
	admissionKey := desktopInfraRuntimeConfigKey(s, cfg)
	providerKey := desktopInfraProviderConfigKey(s, cfg, "fritzbox")
	ctx, finish := beginDesktopInfraTestOperation(t, s, cfg, runtime)
	defer finish()
	if err := vault.WriteSecret("fritzbox_password", "after-rotation"); err != nil {
		t.Fatal(err)
	}
	if desktopInfraRuntimeConfigKey(s, cfg) != admissionKey {
		t.Fatal("Vault-only credential rotation changed the persistent Infra admission key")
	}
	newCtx, finishNew := runtime.beginOperation(context.Background(), desktopInfraRuntimeConfigKey(s, cfg), cfg)
	defer finishNew()
	if newCtx.Err() != nil {
		t.Fatalf("future Infra operation was denied after Vault-only rotation: %v", newCtx.Err())
	}
	source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "fritzbox", Instance: desktopInfraFritzInstance(cfg), Key: "AA:BB:CC:DD:EE:FF"}}
	if _, ok := desktopInfraCurrentWriteConfig(ctx, s, cfg, nil, "", source, providerKey, true, ""); ok {
		t.Fatal("prepared Fritz!Box write remained authorized after its Vault credential changed")
	}
}

func TestDesktopInfraDockerRenameProtectsManagedOwnersAndConfiguredNames(t *testing.T) {
	for _, tc := range []struct {
		name, owner, containerName string
		labels                     map[string]string
	}{
		{name: "managed go2rtc", owner: "go2rtc", containerName: "/aurago_go2rtc", labels: map[string]string{"aurago.managed": "go2rtc", "aurago.go2rtc.owner": "data-dir-owner"}},
		{name: "managed RuView", owner: "ruview", containerName: "/aurago-ruview", labels: map[string]string{"aurago.managed": "ruview", "aurago.ruview.owner": "data-dir-owner"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owners := tools.DockerContainerOwnersFromMetadata([]string{tc.containerName}, tc.labels, containerProtectedOwners...)
			if !owners[tc.owner] {
				t.Fatalf("fixture is not protected by shared policy: %#v", owners)
			}
			cfg := &config.Config{}
			configureDesktopInfraTestWrite(cfg)
			cfg.Docker.Enabled = true
			cfg.Docker.Host = "mock://docker"
			writes := 0
			runtime := &desktopInfraRuntime{dockerRequest: func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
				if method == http.MethodGet && endpoint == "/containers/deadbeef/json" {
					body, err := json.Marshal(map[string]any{
						"Id": "deadbeef", "Name": tc.containerName,
						"Config": map[string]any{"Image": "managed", "Labels": tc.labels},
						"State":  map[string]string{"Status": "running"},
					})
					return body, http.StatusOK, err
				}
				if method == http.MethodPost {
					writes++
				}
				return nil, 0, errors.New("unexpected Docker request")
			}}
			source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "deadbeef"}}
			before := desktopInfraDockerInspect{ID: "deadbeef", Name: tc.containerName}
			before.Config.Image = "managed"
			before.Config.Labels = tc.labels
			before.State.Status = "running"
			result, status, err := desktopInfraWriteSourceEdit(context.Background(), nil, cfg, nil, runtime, "", source, desktopInfraSourceEditRequest{
				Source: "docker", Version: desktopInfraDockerVersion(before), Fields: []byte(`{"name":"renamed"}`),
			})
			if err == nil || status != http.StatusForbidden || writes != 0 {
				t.Fatalf("rename of managed %s was not rejected: result=%+v status=%d err=%v POSTs=%d", tc.owner, result, status, err, writes)
			}
		})
	}

	cfg := &config.Config{}
	cfg.Go2RTC.ContainerName = "custom-go2rtc"
	if !desktopInfraDockerReservedName(cfg, "custom-go2rtc") {
		t.Fatal("configured managed go2rtc name was not reserved")
	}
}

func TestDesktopInfraRenameRechecksCurrentConfiguredReservedNames(t *testing.T) {
	s, cfg, runtime := newDesktopInfraWriteFixture(t)
	infraCtx, finish := beginDesktopInfraTestOperation(t, s, cfg, runtime)
	defer finish()
	writes := 0
	runtime.dockerRequest = func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if method == http.MethodGet && endpoint == "/containers/deadbeef/json" {
			next := *s.ConfigSnapshot()
			next.Go2RTC.ContainerName = "custom-managed-go2rtc"
			s.replaceConfigSnapshot(&next)
			return dockerInspectFixture("deadbeef", "web", "nginx", "running"), http.StatusOK, nil
		}
		if method == http.MethodPost {
			writes++
		}
		return nil, 0, errors.New("unexpected Docker request")
	}
	source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "deadbeef"}}
	before := desktopInfraDockerInspect{ID: "deadbeef", Name: "/web"}
	before.Config.Image = "nginx"
	before.State.Status = "running"
	result, status, err := desktopInfraWriteSourceEdit(infraCtx, s, cfg, nil, runtime, "", source, desktopInfraSourceEditRequest{
		Source: "docker", Version: desktopInfraDockerVersion(before), Fields: []byte(`{"name":"custom-managed-go2rtc"}`),
	})
	if err == nil || status != http.StatusBadRequest || writes != 0 {
		t.Fatalf("rename to newly configured managed name result=%+v status=%d err=%v POSTs=%d; want rejected and zero POSTs", result, status, err, writes)
	}
}
