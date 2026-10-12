package cydcreator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
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
	minted, token := mintTestDevice(t, env, p.ID, build.ID)
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
	if n := countDeviceRows(t, env, `token_hash=?`, sha256Hex([]byte(token))); n != 0 {
		t.Fatalf("%d rows still carry the revoked token's hash", n)
	}
	if err := env.svc.RevokeDevice(ctx, minted.Device.ID); ErrorCode(err) != CodeDeviceNotFound {
		t.Fatalf("second revoke: %v", err)
	}
}

func countDeviceRows(t *testing.T, env *testEnv, where string, args ...any) int {
	t.Helper()
	var n int
	if err := env.svc.db.QueryRow(`SELECT COUNT(*) FROM cc_devices WHERE `+where, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// storedDeviceEvents returns the persisted device_changed payloads of a project in order.
func storedDeviceEvents(t *testing.T, env *testEnv, projectID string) []string {
	t.Helper()
	rows, err := env.svc.db.Query(`SELECT data FROM cc_events WHERE project_id=? AND type=? ORDER BY seq`, projectID, EventDeviceChanged)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			t.Fatal(err)
		}
		out = append(out, data)
	}
	return out
}

func TestListDevicesPurgesUnderTheProjectLock(t *testing.T) {
	env, clock := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Sperre")
	build := succeededBuild(t, env, p.ID)
	minted, _ := mintTestDevice(t, env, p.ID, build.ID)
	clock.advance(DevicePendingTTL)

	// A writer (UpdateDevice, a heartbeat) holds the project lock after
	// re-reading the row: the purge must wait for it instead of deleting the
	// row underneath it.
	unlock := env.svc.lockProject(p.ID)
	done := make(chan []Device, 1)
	go func() {
		list, _ := env.svc.ListDevices(ctx, p.ID)
		done <- list
	}()
	select {
	case <-done:
		unlock()
		t.Fatal("ListDevices purged an expired device without the project lock")
	case <-time.After(200 * time.Millisecond):
	}
	n := countDeviceRows(t, env, `id=?`, minted.Device.ID)
	unlock()
	if n != 1 {
		t.Fatal("the expired device vanished while another writer held the project lock")
	}
	select {
	case list := <-done:
		if len(list) != 0 {
			t.Fatalf("list after the purge = %+v", list)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ListDevices did not finish after the lock was released")
	}
}

func TestUpdateDeviceOnAnExpiredOrRemovedDevice(t *testing.T) {
	env, clock := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Spaet")
	build := succeededBuild(t, env, p.ID)
	minted, _ := mintTestDevice(t, env, p.ID, build.ID)
	clock.advance(DevicePendingTTL)
	name := "Flur"
	if _, err := env.svc.UpdateDevice(ctx, minted.Device.ID, DeviceUpdate{Name: &name}); ErrorCode(err) != CodeDeviceNotFound {
		t.Fatalf("update of an expired pending device: %v", err)
	}
	stored := storedDeviceEvents(t, env, p.ID)
	if len(stored) != 2 || !strings.Contains(stored[1], `"status":"revoked"`) {
		t.Fatalf("device events = %v, want the mint and then only the removal", stored)
	}
	if n := countDeviceRows(t, env, `id=?`, minted.Device.ID); n != 0 {
		t.Fatal("the expired device is still stored")
	}

	// A row deleted after the writer read it: the UPDATE hits nothing and the
	// writer reports the device as gone instead of announcing it again.
	other, _ := mintTestDevice(t, env, p.ID, build.ID)
	row, err := env.svc.loadDeviceRow(ctx, other.Device.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.db.Exec(`DELETE FROM cc_devices WHERE id=?`, row.ID); err != nil {
		t.Fatal(err)
	}
	row.Name = "Geist"
	wantCode(t, env.svc.saveDeviceSettings(ctx, row), CodeDeviceNotFound)
}

func TestDeviceIDsAreCheckedBeforeAnyQuery(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	long := "ccd_" + strings.Repeat("x", 300)
	ids := []string{"", "x", "ccp_0001", "ccd_a/b", "ccd_\x00", long}
	// With the database closed any query would fail with CYD_INTERNAL; an id the
	// service never issues is "not found" before the database is asked.
	_ = env.svc.db.Close()
	name := "Flur"
	for _, id := range ids {
		_, err := env.svc.UpdateDevice(ctx, id, DeviceUpdate{Name: &name})
		e := wantCode(t, err, CodeDeviceNotFound)
		if echo, _ := e.Details["device_id"].(string); len(echo) > maxEchoBytes+len("…") {
			t.Errorf("id of %d bytes echoed as %d bytes", len(id), len(echo))
		}
		wantCode(t, env.svc.RevokeDevice(ctx, id), CodeDeviceNotFound)
	}
}

func TestDeviceNamesRefuseInvisibleAndBrokenText(t *testing.T) {
	for _, bad := range []string{
		"\xe2\x80\xaeabc", // U+202E right-to-left override
		"a\xffb",          // invalid UTF-8
		"a\xe2\x80\xa8b",  // U+2028 line separator
		"a\xe2\x80\x8bb",  // U+200B zero width space
		"a\xef\xbb\xbfb",  // U+FEFF byte order mark
		"a\tb",
		strings.Repeat("x", maxDeviceNameRunes+1),
	} {
		if _, err := validDeviceName(bad); ErrorCode(err) != CodeBadRequest {
			t.Errorf("name %q accepted", bad)
		}
	}
	for _, good := range []string{"Küche", "Flur #2", "温度计", strings.Repeat("ä", maxDeviceNameRunes)} {
		if got, err := validDeviceName(" " + good + " "); err != nil || got != good {
			t.Errorf("name %q = %q, %v", good, got, err)
		}
	}

	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Namen")
	build := succeededBuild(t, env, p.ID)
	rlo := "\xe2\x80\xaeabc"
	if _, err := env.svc.MintDevice(ctx, p.ID, MintDeviceInput{BuildID: build.ID, Name: rlo, Endpoint: testDeviceEndpoint}); ErrorCode(err) != CodeBadRequest {
		t.Errorf("mint with a bidi override: %v", err)
	}
	minted, _ := mintTestDevice(t, env, p.ID, build.ID)
	broken := "a\xffb"
	if _, err := env.svc.UpdateDevice(ctx, minted.Device.ID, DeviceUpdate{Name: &broken}); ErrorCode(err) != CodeBadRequest {
		t.Errorf("rename to invalid UTF-8: %v", err)
	}
}

func TestDefaultDeviceNames(t *testing.T) {
	if got := defaultDeviceName("  Wetter  ", 3); got != "Wetter #3" {
		t.Errorf("short name = %q", got)
	}
	got := defaultDeviceName(strings.Repeat("ä", 70), 12)
	if utf8.RuneCountInString(got) != maxDeviceNameRunes || got != strings.Repeat("ä", maxDeviceNameRunes-4)+" #12" {
		t.Errorf("long name = %q (%d runes)", got, utf8.RuneCountInString(got))
	}
	// The cut lands on a space: no double space before the number.
	if got := defaultDeviceName(strings.Repeat("a", 60)+" bcd", 1); got != strings.Repeat("a", 60)+" #1" {
		t.Errorf("cut at a space = %q", got)
	}

	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Zahl")
	build := succeededBuild(t, env, p.ID)
	var ids []string
	for i := 0; i < 3; i++ {
		m, _ := mintTestDevice(t, env, p.ID, build.ID)
		ids = append(ids, m.Device.ID)
	}
	if err := env.svc.RevokeDevice(ctx, ids[1]); err != nil { // "Zahl #2"
		t.Fatal(err)
	}
	next, _ := mintTestDevice(t, env, p.ID, build.ID)
	if next.Device.Name != "Zahl #4" {
		t.Fatalf("name after a revoke = %q, want the next free number Zahl #4", next.Device.Name)
	}
}

func TestMintDeviceRefusesTheDeviceOverTheLimit(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Voll")
	build := succeededBuild(t, env, p.ID)
	created := formatTime(env.svc.now())
	for i := 0; i < maxDevicesPerProject; i++ {
		if _, err := env.svc.db.Exec(`INSERT INTO cc_devices(id,project_id,name,board,status,token_hash,created_at) VALUES(?,?,?,?,?,?,?)`,
			fmt.Sprintf("ccd_full%02d", i), p.ID, fmt.Sprintf("Voll #%d", i+1), "esp32-2432s028r", DeviceActive,
			fmt.Sprintf("hash-%02d", i), created); err != nil {
			t.Fatal(err)
		}
	}
	_, err := env.svc.MintDevice(ctx, p.ID, MintDeviceInput{BuildID: build.ID, Endpoint: testDeviceEndpoint})
	if e := wantCode(t, err, CodeBusy); e.Details["limit"] != maxDevicesPerProject {
		t.Fatalf("details = %v", e.Details)
	}
	if err := env.svc.RevokeDevice(ctx, "ccd_full00"); err != nil {
		t.Fatal(err)
	}
	minted, _ := mintTestDevice(t, env, p.ID, build.ID)
	if minted.Device.Name != "Voll #65" {
		t.Fatalf("name = %q", minted.Device.Name)
	}
}

func TestFirmwareBuildRules(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Regeln")
	first := succeededBuild(t, env, p.ID)
	mint := func(buildID string) error {
		_, err := env.svc.MintDevice(ctx, p.ID, MintDeviceInput{BuildID: buildID, Endpoint: testDeviceEndpoint})
		return err
	}
	wantCode(t, mint("ccp_x"), CodeBadRequest)
	wantCode(t, mint("ccb_a/b"), CodeBadRequest)
	minted, _ := mintTestDevice(t, env, p.ID, first.ID)

	other := succeededBuild(t, env, p.ID)
	if _, err := env.svc.db.Exec(`UPDATE cc_builds SET board='esp32-8048s043' WHERE id=?`, other.ID); err != nil {
		t.Fatal(err)
	}
	target := other.ID
	_, err := env.svc.UpdateDevice(ctx, minted.Device.ID, DeviceUpdate{TargetBuildID: &target})
	if e := wantCode(t, err, CodeBadRequest); e.Details["device_board"] != "esp32-2432s028r" {
		t.Fatalf("target for another board: %v", err)
	}

	bare := succeededBuild(t, env, p.ID)
	if _, err := env.svc.db.Exec(`UPDATE cc_builds SET artifacts='[]' WHERE id=?`, bare.ID); err != nil {
		t.Fatal(err)
	}
	if e := wantCode(t, mint(bare.ID), CodeBadRequest); !strings.Contains(e.Message, "no firmware image") {
		t.Fatalf("build without firmware.bin: %v", e)
	}
	target = bare.ID
	_, err = env.svc.UpdateDevice(ctx, minted.Device.ID, DeviceUpdate{TargetBuildID: &target})
	wantCode(t, err, CodeBadRequest)
}

func TestDeleteProjectRemovesItsDevices(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Abriss")
	keep := env.createProject(t, "Bleibt")
	build := succeededBuild(t, env, p.ID)
	_, token := mintTestDevice(t, env, p.ID, build.ID)
	kept, _ := mintTestDevice(t, env, keep.ID, succeededBuild(t, env, keep.ID).ID)
	if err := env.svc.DeleteProject(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if n := countDeviceRows(t, env, `project_id=? OR token_hash=?`, p.ID, sha256Hex([]byte(token))); n != 0 {
		t.Fatalf("%d device rows of the deleted project remain", n)
	}
	if n := countDeviceRows(t, env, `id=?`, kept.Device.ID); n != 1 {
		t.Fatal("DeleteProject removed another project's device")
	}
}
