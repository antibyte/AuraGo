package cydcreator

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var testDeviceEndpoint = DeviceEndpoint{URL: "https://aurago.example:8443/", CertSHA256: strings.Repeat("ab", 32)}

// deviceClock is a manual clock: Now returns the same instant until advance
// moves it, so rate-limit windows and expiry times are exact in tests.
type deviceClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *deviceClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *deviceClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// newDeviceEnv is newTestService with a manual clock, so tests can let a
// pending device expire, a heartbeat age or a rate-limit window pass.
func newDeviceEnv(t *testing.T) (*testEnv, *deviceClock) {
	t.Helper()
	dir := t.TempDir()
	clock := &deviceClock{t: time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)}
	builder := newFakeBuilder()
	cfg := ServiceConfig{
		WorkspaceDir: filepath.Join(dir, "workspace"),
		DataDir:      filepath.Join(dir, "data"),
		DBPath:       filepath.Join(dir, "data", "cyd_creator.db"),
		Builder:      builder,
		Generate:     fakeGenerate,
		IDs:          &SeqIDs{},
		Now:          clock.Now,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return &testEnv{svc: svc, builder: builder, dir: dir, cfg: cfg}, clock
}

// succeededBuild runs a build with the fake builder and returns it finished.
func succeededBuild(t *testing.T, env *testEnv, projectID string) Build {
	t.Helper()
	b, err := env.svc.StartBuild(context.Background(), projectID, BuildOptions{})
	if err != nil {
		t.Fatalf("StartBuild: %v", err)
	}
	done := waitBuild(t, env.svc, b.ID)
	if done.Status != BuildSucceeded {
		t.Fatalf("build %s = %s", done.ID, done.Status)
	}
	return done
}

// mintTestDevice mints a device for build and returns it with its raw token.
func mintTestDevice(t *testing.T, env *testEnv, projectID, buildID string) (MintedDevice, string) {
	t.Helper()
	minted, err := env.svc.MintDevice(context.Background(), projectID, MintDeviceInput{BuildID: buildID, Endpoint: testDeviceEndpoint})
	if err != nil {
		t.Fatalf("MintDevice: %v", err)
	}
	cfg, err := DecodeDeviceConfig(minted.ConfigBlob)
	if err != nil {
		t.Fatalf("DecodeDeviceConfig: %v", err)
	}
	return minted, cfg.Token
}

func TestMintDeviceWritesTheConfigBlobAndStoresOnlyTheTokenHash(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Wetter")
	build := succeededBuild(t, env, p.ID)
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()

	minted, token := mintTestDevice(t, env, p.ID, build.ID)
	d := minted.Device
	if d.Status != DevicePending || d.Name != "Wetter #1" || d.Board != "esp32-2432s028r" || d.BuildID != build.ID ||
		d.TargetBuildID != build.ID || d.Online || d.ProjectID != p.ID {
		t.Fatalf("device = %+v", d)
	}
	if minted.ConfigOffset != 0x3D0000 || len(minted.ConfigBlob) != ConfigSize {
		t.Fatalf("offset %#x, blob %d bytes", minted.ConfigOffset, len(minted.ConfigBlob))
	}
	cfg, err := DecodeDeviceConfig(minted.ConfigBlob)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "https://aurago.example:8443" || cfg.DeviceID != d.ID || cfg.CertSHA256 != testDeviceEndpoint.CertSHA256 ||
		!strings.HasPrefix(token, "ccdt_") || len(token) != 69 {
		t.Fatalf("config = %+v", cfg)
	}
	row, err := env.svc.loadDeviceRow(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.TokenHash != sha256Hex([]byte(token)) || row.ExpiresAt == "" {
		t.Fatalf("stored row = %+v", row)
	}
	var dump string
	if err := env.svc.db.QueryRow(`SELECT id||name||token_hash||build_id||target_build_id||expires_at FROM cc_devices`).Scan(&dump); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dump, token) {
		t.Fatal("the raw device token is stored")
	}
	ev := collectEvents(t, events, 1, 2*time.Second, EventDeviceChanged)[0]
	var data struct {
		Device Device `json:"device"`
	}
	if err := json.Unmarshal(ev.Data, &data); err != nil || data.Device.ID != d.ID || data.Device.Status != DevicePending {
		t.Fatalf("device_changed = %s (%v)", ev.Data, err)
	}
	if strings.Contains(string(ev.Data), token) {
		t.Fatal("the device token leaked into an event")
	}

	second, _ := mintTestDevice(t, env, p.ID, build.ID)
	if second.Device.Name != "Wetter #2" || second.Device.ID == d.ID {
		t.Fatalf("second device = %+v", second.Device)
	}
}

func TestMintDeviceNeedsASucceededBuildOfTheSameProject(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Eins")
	other := env.createProject(t, "Zwei")
	otherBuild := succeededBuild(t, env, other.ID)

	mint := func(projectID, buildID string) error {
		_, err := env.svc.MintDevice(ctx, projectID, MintDeviceInput{BuildID: buildID, Endpoint: testDeviceEndpoint})
		return err
	}
	wantCode(t, mint(p.ID, otherBuild.ID), CodeNotFound)
	wantCode(t, mint(p.ID, "ccb_missing"), CodeNotFound)
	wantCode(t, mint(p.ID, ""), CodeBadRequest)
	wantCode(t, mint("ccp_missing", otherBuild.ID), CodeNotFound)

	env.builder.build = func(context.Context, BuildRequest, func(string)) (BuildOutput, error) {
		return BuildOutput{ExitCode: BuildExitCompileError, Log: "src/app/app_main.cpp:1:1: error: boom\n"}, nil
	}
	b, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if failed := waitBuild(t, env.svc, b.ID); failed.Status != BuildFailed {
		t.Fatalf("build = %s", failed.Status)
	}
	wantCode(t, mint(p.ID, b.ID), CodeBadRequest)
	if list, err := env.svc.ListDevices(ctx, p.ID); err != nil || len(list) != 0 {
		t.Fatalf("devices after refused mints = %v, %v", list, err)
	}
}

func TestMintDeviceChecksOTAAndTheEndpoint(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Endpoint")
	build := succeededBuild(t, env, p.ID)
	mint := func(e DeviceEndpoint, name string) (MintedDevice, error) {
		return env.svc.MintDevice(ctx, p.ID, MintDeviceInput{BuildID: build.ID, Name: name, Endpoint: e})
	}
	// The device URL rule is the config blob's rule (ValidateDeviceURL): every
	// refusal names the field and points at cyd_creator.ota.public_url.
	for _, tc := range []struct {
		e      DeviceEndpoint
		reason string
	}{
		{DeviceEndpoint{URL: "ftp://aurago.example"}, "device_url"},
		{DeviceEndpoint{URL: "https://user:pw@aurago.example", CertSHA256: testDeviceEndpoint.CertSHA256}, "device_url"},
		{DeviceEndpoint{URL: "https://aurago.example?x=1", CertSHA256: testDeviceEndpoint.CertSHA256}, "device_url"},
		{DeviceEndpoint{URL: "https://aurago.example"}, "cert_sha256"},
		{DeviceEndpoint{URL: "http://aurago.example", CertSHA256: testDeviceEndpoint.CertSHA256}, "cert_sha256"},
		{DeviceEndpoint{URL: "aurago.example"}, "device_url"},
		{DeviceEndpoint{URL: "HTTPS://aurago.example", CertSHA256: testDeviceEndpoint.CertSHA256}, "device_url"},
		{DeviceEndpoint{URL: "http://[fd00::5]:8088"}, "device_url"},
		{DeviceEndpoint{URL: "http://aurago.example:0"}, "device_url"},
	} {
		_, err := mint(tc.e, "")
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != CodeBadRequest {
			t.Errorf("endpoint %+v: error = %v, want CYD_BAD_REQUEST", tc.e, err)
			continue
		}
		if typed.Details["reason"] != tc.reason || typed.Details["hint"] != "cyd_creator.ota.public_url" {
			t.Errorf("endpoint %+v: details = %v, want reason %s and the public_url hint", tc.e, typed.Details, tc.reason)
		}
	}
	if _, err := mint(testDeviceEndpoint, "\x07bell"); ErrorCode(err) != CodeBadRequest {
		t.Errorf("control character name: %v", err)
	}
	insecure, err := mint(DeviceEndpoint{URL: "http://192.168.1.20:8088"}, "Küche")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := DecodeDeviceConfig(insecure.ConfigBlob)
	if err != nil || cfg.URL != "http://192.168.1.20:8088" || cfg.CertSHA256 != "" || insecure.Device.Name != "Küche" {
		t.Fatalf("insecure config = %+v, device %+v (%v)", cfg, insecure.Device, err)
	}

	limits := env.svc.Limits()
	limits.OTA.Enabled = false
	env.svc.UpdateLimits(limits)
	if _, err := mint(testDeviceEndpoint, ""); ErrorCode(err) != CodeOTADisabled {
		t.Fatalf("mint with OTA off: %v", err)
	}
}

func TestListDevicesOnlineStateAndPendingExpiry(t *testing.T) {
	env, clock := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Liste")
	build := succeededBuild(t, env, p.ID)
	active, _ := mintTestDevice(t, env, p.ID, build.ID)
	pending, _ := mintTestDevice(t, env, p.ID, build.ID)
	if _, err := env.svc.db.Exec(`UPDATE cc_devices SET status=?, expires_at='', last_seen=? WHERE id=?`,
		DeviceActive, formatTime(env.svc.now()), active.Device.ID); err != nil {
		t.Fatal(err)
	}
	list, err := env.svc.ListDevices(ctx, p.ID)
	if err != nil || len(list) != 2 || list[0].ID != active.Device.ID || !list[0].Online || list[1].Online {
		t.Fatalf("list = %+v, %v", list, err)
	}

	clock.advance(3*time.Minute + 5*time.Second) // three 60 s intervals without a heartbeat
	list, err = env.svc.ListDevices(ctx, p.ID)
	if err != nil || len(list) != 2 || list[0].Online {
		t.Fatalf("list after silence = %+v, %v", list, err)
	}

	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	clock.advance(DevicePendingTTL)
	list, err = env.svc.ListDevices(ctx, p.ID)
	if err != nil || len(list) != 1 || list[0].ID != active.Device.ID {
		t.Fatalf("list after the pending device expired = %+v, %v", list, err)
	}
	ev := collectEvents(t, events, 1, 2*time.Second, EventDeviceChanged)[0]
	if !strings.Contains(string(ev.Data), pending.Device.ID) || !strings.Contains(string(ev.Data), `"status":"revoked"`) {
		t.Fatalf("expiry event = %s", ev.Data)
	}
	if _, err := env.svc.ListDevices(ctx, "ccp_missing"); ErrorCode(err) != CodeNotFound {
		t.Fatalf("unknown project: %v", err)
	}
}

func TestUpdateDeviceRenamesAndSetsTheOTATarget(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Ziel")
	first := succeededBuild(t, env, p.ID)
	minted, _ := mintTestDevice(t, env, p.ID, first.ID)
	id := minted.Device.ID
	if _, err := env.svc.db.Exec(`UPDATE cc_devices SET last_error='failed: timeout' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	second := succeededBuild(t, env, p.ID)

	name := "  Flur  "
	target := second.ID
	d, err := env.svc.UpdateDevice(ctx, id, DeviceUpdate{Name: &name, TargetBuildID: &target})
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Flur" || d.TargetBuildID != second.ID || d.BuildID != first.ID || d.LastError != "" {
		t.Fatalf("updated = %+v", d)
	}

	other := env.createProject(t, "Fremd")
	foreign := succeededBuild(t, env, other.ID).ID
	empty := ""
	bad := "\x00"
	for _, tc := range []struct {
		in   DeviceUpdate
		code string
	}{
		{DeviceUpdate{}, CodeBadRequest},
		{DeviceUpdate{Name: &empty}, CodeBadRequest},
		{DeviceUpdate{Name: &bad}, CodeBadRequest},
		{DeviceUpdate{TargetBuildID: &foreign}, CodeNotFound},
	} {
		if _, err := env.svc.UpdateDevice(ctx, id, tc.in); ErrorCode(err) != tc.code {
			t.Errorf("update %+v: %v, want %s", tc.in, err, tc.code)
		}
	}
	if _, err := env.svc.UpdateDevice(ctx, "ccd_missing", DeviceUpdate{Name: &name}); ErrorCode(err) != CodeDeviceNotFound {
		t.Fatalf("missing device: %v", err)
	}
	limits := env.svc.Limits()
	limits.OTA.Enabled = false
	env.svc.UpdateLimits(limits)
	if _, err := env.svc.UpdateDevice(ctx, id, DeviceUpdate{TargetBuildID: &target}); ErrorCode(err) != CodeOTADisabled {
		t.Fatalf("target with OTA off: %v", err)
	}
	renamed := "Keller"
	if d, err := env.svc.UpdateDevice(ctx, id, DeviceUpdate{Name: &renamed}); err != nil || d.Name != "Keller" {
		t.Fatalf("rename with OTA off = %+v, %v", d, err)
	}
}

func TestRevokeDeviceDeletesTheDevice(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Weg")
	build := succeededBuild(t, env, p.ID)
	minted, _ := mintTestDevice(t, env, p.ID, build.ID)
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	if err := env.svc.RevokeDevice(ctx, minted.Device.ID); err != nil {
		t.Fatal(err)
	}
	ev := collectEvents(t, events, 1, 2*time.Second, EventDeviceChanged)[0]
	if !strings.Contains(string(ev.Data), `"status":"revoked"`) {
		t.Fatalf("revoke event = %s", ev.Data)
	}
	if list, _ := env.svc.ListDevices(ctx, p.ID); len(list) != 0 {
		t.Fatalf("devices after revoke = %+v", list)
	}
	if err := env.svc.RevokeDevice(ctx, minted.Device.ID); ErrorCode(err) != CodeDeviceNotFound {
		t.Fatalf("second revoke: %v", err)
	}
}
