package cydcreator

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func deviceReq(token string) DeviceRequest {
	return DeviceRequest{Token: token, ClientIP: "192.168.1.50"}
}

func heartbeatFor(d Device, buildID string) Heartbeat {
	return Heartbeat{DeviceID: d.ID, BuildID: buildID, TemplateVersion: "1.0.0", UptimeS: 12, IP: "192.168.1.50",
		RSSI: -61, FreeHeap: 81234, ResetReason: "poweron", OTAState: OTAStateIdle}
}

func TestDeviceHeartbeatActivatesTheDeviceAndOffersItsTarget(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Heartbeat")
	first := succeededBuild(t, env, p.ID)
	minted, token := mintTestDevice(t, env, p.ID, first.ID)
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()

	resp, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(minted.Device, first.ID))
	if err != nil {
		t.Fatal(err)
	}
	if resp.IntervalS != 60 || resp.Update != nil {
		t.Fatalf("first answer = %+v", resp)
	}
	list, _ := env.svc.ListDevices(ctx, p.ID)
	d := list[0]
	if d.Status != DeviceActive || !d.Online || d.IP != "192.168.1.50" || d.RSSI != -61 || d.FreeHeap != 81234 ||
		d.TemplateVersion != "1.0.0" || d.OTAState != OTAStateIdle || d.LastSeen == "" {
		t.Fatalf("device after the first heartbeat = %+v", d)
	}
	if row, _ := env.svc.loadDeviceRow(ctx, d.ID); row.ExpiresAt != "" {
		t.Fatalf("an active device must not expire: %+v", row)
	}
	collectEvents(t, events, 1, 2*time.Second, EventDeviceChanged)

	// A plain repeat heartbeat changes nothing worth an event.
	if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(minted.Device, first.ID)); err != nil {
		t.Fatal(err)
	}
	second := succeededBuild(t, env, p.ID)
	target := second.ID
	if _, err := env.svc.UpdateDevice(ctx, d.ID, DeviceUpdate{TargetBuildID: &target}); err != nil {
		t.Fatal(err)
	}
	resp, err = env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(minted.Device, first.ID))
	if err != nil {
		t.Fatal(err)
	}
	// update.url is a path below the device URL (the firmware rejects anything
	// else and falls back to its default path), never an absolute URL.
	want := FirmwareUpdate{BuildID: second.ID, Size: int64(len("firmware-" + second.ID)), SHA256: sha256Hex([]byte("firmware-" + second.ID)),
		URL: "/api/cyd-creator/device/firmware/" + second.ID}
	if resp.Update == nil || *resp.Update != want {
		t.Fatalf("update offer = %+v, want %+v", resp.Update, want)
	}

	// The device reports the new build: the update is done, nothing more to offer.
	resp, err = env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(minted.Device, second.ID))
	if err != nil || resp.Update != nil {
		t.Fatalf("after the update = %+v, %v", resp, err)
	}
	list, _ = env.svc.ListDevices(ctx, p.ID)
	if list[0].BuildID != second.ID || list[0].TargetBuildID != second.ID || list[0].LastError != "" {
		t.Fatalf("device after the update = %+v", list[0])
	}
	got := collectEvents(t, events, 2, 2*time.Second, EventDeviceChanged)
	if !strings.Contains(string(got[1].Data), `"build_id":"`+second.ID+`"`) {
		t.Fatalf("update event = %s", got[1].Data)
	}
}

func TestDeviceAuthenticationIsBoundToTheDevice(t *testing.T) {
	env, clock := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Bindung")
	build := succeededBuild(t, env, p.ID)
	a, tokenA := mintTestDevice(t, env, p.ID, build.ID)
	b, tokenB := mintTestDevice(t, env, p.ID, build.ID)

	if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(tokenA), heartbeatFor(b.Device, build.ID)); ErrorCode(err) != CodeUnauthorized {
		t.Fatalf("token of A for device B: %v", err)
	}
	for _, token := range []string{"", "ccdt_" + strings.Repeat("0", 64), "aura_" + strings.Repeat("a", 32), tokenA + "x", strings.ToUpper(tokenA)} {
		if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(a.Device, build.ID)); ErrorCode(err) != CodeUnauthorized {
			t.Errorf("token %q: %v", token, err)
		}
	}
	if err := env.svc.RevokeDevice(ctx, b.Device.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(tokenB), heartbeatFor(b.Device, build.ID)); ErrorCode(err) != CodeUnauthorized {
		t.Fatalf("revoked token: %v", err)
	}

	clock.advance(DevicePendingTTL + time.Second)
	if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(tokenA), heartbeatFor(a.Device, build.ID)); ErrorCode(err) != CodeUnauthorized {
		t.Fatalf("expired pending device: %v", err)
	}
	if _, err := env.svc.loadDeviceRow(ctx, a.Device.ID); ErrorCode(err) != CodeDeviceNotFound {
		t.Fatalf("the expired device must be removed: %v", err)
	}
}

func TestDeviceAuthenticationFailuresAreRateLimitedPerClient(t *testing.T) {
	env, clock := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Brute")
	build := succeededBuild(t, env, p.ID)
	minted, token := mintTestDevice(t, env, p.ID, build.ID)
	attacker := DeviceRequest{Token: "ccdt_" + strings.Repeat("1", 64), ClientIP: "10.0.0.9"}
	for i := 0; i < authFailuresPerWindow; i++ {
		if _, err := env.svc.DeviceHeartbeat(ctx, attacker, heartbeatFor(minted.Device, build.ID)); ErrorCode(err) != CodeUnauthorized {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	attacker.Token = token
	if _, err := env.svc.DeviceHeartbeat(ctx, attacker, heartbeatFor(minted.Device, build.ID)); ErrorCode(err) != CodeRateLimited {
		t.Fatalf("blocked client with a valid token: %v", err)
	}
	if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(minted.Device, build.ID)); err != nil {
		t.Fatalf("another client must not be blocked: %v", err)
	}
	clock.advance(authFailureWindow)
	if _, err := env.svc.DeviceHeartbeat(ctx, attacker, heartbeatFor(minted.Device, build.ID)); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}

func TestDeviceLimiterForgetsIdleKeys(t *testing.T) {
	var l deviceLimiter
	start := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	for i := 0; i < deviceLimiterSweepKeys; i++ {
		l.record(fmt.Sprintf("auth:10.0.%d.%d", i/256, i%256), start, authFailureWindow)
	}
	// One more key after every earlier one left its window: the sweep drops
	// the idle keys, so client addresses that never return cannot pile up.
	l.record("auth:10.9.9.9", start.Add(authFailureWindow), authFailureWindow)
	if n := len(l.hits); n != 1 {
		t.Fatalf("keys after the sweep = %d, want 1", n)
	}
}

func TestDeviceHeartbeatValidatesTheBodyAndTheRate(t *testing.T) {
	env, clock := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Rate")
	build := succeededBuild(t, env, p.ID)
	minted, token := mintTestDevice(t, env, p.ID, build.ID)
	for _, mutate := range []func(*Heartbeat){
		func(h *Heartbeat) { h.OTAState = "updating" },
		func(h *Heartbeat) { h.RSSI = 5 },
		func(h *Heartbeat) { h.IP = "not-an-ip" },
		func(h *Heartbeat) { h.BuildID = "../x" },
		func(h *Heartbeat) { h.TemplateVersion = "1.0.0;rm" },
		func(h *Heartbeat) { h.FreeHeap = -1 },
		func(h *Heartbeat) { h.OTAError = strings.Repeat("x", maxOTAErrorRunes+1) },
	} {
		hb := heartbeatFor(minted.Device, build.ID)
		mutate(&hb)
		if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), hb); ErrorCode(err) != CodeBadRequest {
			t.Errorf("heartbeat %+v: %v", hb, err)
		}
	}
	for i := 0; i < heartbeatsPerMinute; i++ {
		if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(minted.Device, build.ID)); err != nil {
			t.Fatalf("heartbeat %d: %v", i, err)
		}
	}
	if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(minted.Device, build.ID)); ErrorCode(err) != CodeRateLimited {
		t.Fatalf("heartbeat over the limit: %v", err)
	}
	clock.advance(time.Minute)
	if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(minted.Device, build.ID)); err != nil {
		t.Fatalf("heartbeat after a minute: %v", err)
	}
}

func TestDeviceHeartbeatMirrorsRollbackAndFailure(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Rollback")
	first := succeededBuild(t, env, p.ID)
	minted, token := mintTestDevice(t, env, p.ID, first.ID)
	second := succeededBuild(t, env, p.ID)
	target := second.ID
	setTarget := func() {
		t.Helper()
		if _, err := env.svc.UpdateDevice(ctx, minted.Device.ID, DeviceUpdate{TargetBuildID: &target}); err != nil {
			t.Fatal(err)
		}
	}
	beat := func(state, message string) (HeartbeatResponse, Device) {
		t.Helper()
		hb := heartbeatFor(minted.Device, first.ID)
		hb.OTAState, hb.OTAError = state, message
		resp, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), hb)
		if err != nil {
			t.Fatalf("%s heartbeat: %v", state, err)
		}
		list, _ := env.svc.ListDevices(ctx, p.ID)
		return resp, list[0]
	}

	// A failed download keeps the target: the firmware retries after 10 minutes.
	setTarget()
	resp, d := beat(OTAStateFailed, "download\ntimeout")
	if d.LastError != "failed: download timeout" || d.TargetBuildID != second.ID || d.OTAState != OTAStateFailed || resp.Update == nil {
		t.Fatalf("after failed: %+v, update %+v", d, resp.Update)
	}

	// A rollback stops offering the build that did not start.
	resp, d = beat(OTAStateRolledBack, "rolled back from "+second.ID)
	if d.LastError != "rolled_back: rolled back from "+second.ID || d.TargetBuildID != first.ID || resp.Update != nil {
		t.Fatalf("after rolled_back: %+v, update %+v", d, resp.Update)
	}

	// The firmware keeps reporting rolled_back for the rest of the boot; a target
	// the user sets meanwhile survives those heartbeats.
	setTarget()
	resp, d = beat(OTAStateRolledBack, "rolled back from "+second.ID)
	if d.TargetBuildID != second.ID || d.LastError != "" || resp.Update == nil {
		t.Fatalf("new target during the rolled back boot: %+v, update %+v", d, resp.Update)
	}
}

func TestDeviceFirmwareServesOnlyTheCurrentTarget(t *testing.T) {
	env, clock := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Firmware")
	first := succeededBuild(t, env, p.ID)
	minted, token := mintTestDevice(t, env, p.ID, first.ID)
	second := succeededBuild(t, env, p.ID)
	target := second.ID
	if _, err := env.svc.UpdateDevice(ctx, minted.Device.ID, DeviceUpdate{TargetBuildID: &target}); err != nil {
		t.Fatal(err)
	}
	other := env.createProject(t, "Andere")
	foreign := succeededBuild(t, env, other.ID)
	for _, id := range []string{first.ID, foreign.ID, "", "ccb_missing"} {
		if _, _, err := env.svc.DeviceFirmware(ctx, deviceReq(token), id); ErrorCode(err) != CodeNotFound {
			t.Errorf("build %q: %v", id, err)
		}
	}
	for i := 0; i < firmwareDownloadsPerWindow; i++ {
		f, info, err := env.svc.DeviceFirmware(ctx, deviceReq(token), second.ID)
		if err != nil {
			t.Fatalf("download %d: %v", i, err)
		}
		data, _ := io.ReadAll(f)
		_ = f.Close()
		if string(data) != "firmware-"+second.ID || info.SHA256 != sha256Hex(data) {
			t.Fatalf("download %d = %q %+v", i, data, info)
		}
	}
	if _, _, err := env.svc.DeviceFirmware(ctx, deviceReq(token), second.ID); ErrorCode(err) != CodeRateLimited {
		t.Fatalf("fourth download: %v", err)
	}
	clock.advance(firmwareDownloadWindow)
	f, _, err := env.svc.DeviceFirmware(ctx, deviceReq(token), second.ID)
	if err != nil {
		t.Fatalf("download after the window: %v", err)
	}
	_ = f.Close()

	// A firmware.bin that no longer matches its build record is never served.
	bin, err := os.OpenFile(filepath.Join(env.svc.artifactsDir, second.ID, ArtifactFirmwareBin), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, werr := bin.WriteString("tail")
	if cerr := bin.Close(); werr != nil || cerr != nil {
		t.Fatal(werr, cerr)
	}
	if f, _, err := env.svc.DeviceFirmware(ctx, deviceReq(token), second.ID); ErrorCode(err) != CodeInternal {
		if f != nil {
			_ = f.Close()
		}
		t.Fatalf("grown firmware.bin: %v", err)
	}

	if _, _, err := env.svc.DeviceFirmware(ctx, deviceReq("ccdt_"+strings.Repeat("2", 64)), second.ID); ErrorCode(err) != CodeUnauthorized {
		t.Fatalf("unknown token: %v", err)
	}
}

func TestDeviceOTAResultMirrorsTheFirmwareReports(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Ergebnis")
	first := succeededBuild(t, env, p.ID)
	minted, token := mintTestDevice(t, env, p.ID, first.ID)
	second := succeededBuild(t, env, p.ID)
	target := second.ID
	if _, err := env.svc.UpdateDevice(ctx, minted.Device.ID, DeviceUpdate{TargetBuildID: &target}); err != nil {
		t.Fatal(err)
	}
	other := env.createProject(t, "Fremd")
	foreign := succeededBuild(t, env, other.ID)
	for _, id := range []string{"", "ccb_missing", foreign.ID} {
		if err := env.svc.DeviceOTAResult(ctx, deviceReq(token), OTAResult{BuildID: id, OK: true}); ErrorCode(err) != CodeBadRequest {
			t.Errorf("result for build %q: %v", id, err)
		}
	}
	device := func() Device {
		t.Helper()
		list, _ := env.svc.ListDevices(ctx, p.ID)
		return list[0]
	}

	// Download or verification failure: last_error, the target stays.
	if err := env.svc.DeviceOTAResult(ctx, deviceReq(token), OTAResult{BuildID: second.ID, Error: "sha256 mismatch"}); err != nil {
		t.Fatal(err)
	}
	if d := device(); d.LastError != "failed: sha256 mismatch" || d.TargetBuildID != second.ID || d.OTAState != OTAStateFailed {
		t.Fatalf("after a failed result: %+v", d)
	}

	// The old image reports a rollback once; a repeat changes nothing.
	for i := 0; i < 2; i++ {
		if err := env.svc.DeviceOTAResult(ctx, deviceReq(token), OTAResult{BuildID: second.ID, Error: "rolled_back"}); err != nil {
			t.Fatal(err)
		}
		if d := device(); d.LastError != "rolled_back" || d.TargetBuildID != first.ID || d.OTAState != OTAStateRolledBack {
			t.Fatalf("after rollback report %d: %+v", i, d)
		}
	}

	// The new image confirms itself after its first good heartbeat.
	if _, err := env.svc.UpdateDevice(ctx, minted.Device.ID, DeviceUpdate{TargetBuildID: &target}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(minted.Device, second.ID)); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.DeviceOTAResult(ctx, deviceReq(token), OTAResult{BuildID: second.ID, OK: true}); err != nil {
		t.Fatal(err)
	}
	if d := device(); d.LastError != "" || d.BuildID != second.ID || d.TargetBuildID != second.ID || d.OTAState != OTAStateIdle {
		t.Fatalf("after the confirmed update: %+v", d)
	}
}

func TestDeviceEndpointsRefuseWhileOTAIsOff(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Aus")
	build := succeededBuild(t, env, p.ID)
	minted, token := mintTestDevice(t, env, p.ID, build.ID)
	limits := env.svc.Limits()
	limits.OTA.Enabled = false
	env.svc.UpdateLimits(limits)
	if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(minted.Device, build.ID)); ErrorCode(err) != CodeOTADisabled {
		t.Fatalf("heartbeat: %v", err)
	}
	if _, _, err := env.svc.DeviceFirmware(ctx, deviceReq(token), build.ID); ErrorCode(err) != CodeOTADisabled {
		t.Fatalf("firmware: %v", err)
	}
	if err := env.svc.DeviceOTAResult(ctx, deviceReq(token), OTAResult{BuildID: build.ID, OK: true}); ErrorCode(err) != CodeOTADisabled {
		t.Fatalf("ota result: %v", err)
	}
}
