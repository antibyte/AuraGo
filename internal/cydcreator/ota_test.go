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
	guess := func(i int) string { return fmt.Sprintf("ccdt_%064x", i+1) }
	attacker := DeviceRequest{ClientIP: "10.0.0.9"}
	for i := 0; i < authFailuresPerWindow; i++ {
		attacker.Token = guess(i)
		if _, err := env.svc.DeviceHeartbeat(ctx, attacker, heartbeatFor(minted.Device, build.ID)); ErrorCode(err) != CodeUnauthorized {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	attacker.Token = guess(authFailuresPerWindow)
	if _, err := env.svc.DeviceHeartbeat(ctx, attacker, heartbeatFor(minted.Device, build.ID)); ErrorCode(err) != CodeRateLimited {
		t.Fatalf("distinct guess %d: %v", authFailuresPerWindow+1, err)
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

// Revoked devices behind one address keep sending their stale token; each
// token counts once, so the healthy device next to them is never refused.
func TestDeviceAuthFailuresCountDistinctTokensPerAddress(t *testing.T) {
	env, clock := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Nat")
	build := succeededBuild(t, env, p.ID)
	good, goodToken := mintTestDevice(t, env, p.ID, build.ID)
	var revoked []string
	for i := 0; i < 2; i++ {
		d, token := mintTestDevice(t, env, p.ID, build.ID)
		if err := env.svc.RevokeDevice(ctx, d.Device.ID); err != nil {
			t.Fatal(err)
		}
		revoked = append(revoked, token)
	}
	nat := func(token string) DeviceRequest { return DeviceRequest{Token: token, ClientIP: "172.17.0.1"} }
	for minute := 0; minute < 30; minute++ {
		for _, token := range revoked {
			if _, err := env.svc.DeviceHeartbeat(ctx, nat(token), Heartbeat{DeviceID: "ccd_x"}); ErrorCode(err) != CodeUnauthorized {
				t.Fatalf("minute %d, revoked device: %v", minute, err)
			}
		}
		if _, err := env.svc.DeviceHeartbeat(ctx, nat(goodToken), heartbeatFor(good.Device, build.ID)); err != nil {
			t.Fatalf("minute %d, healthy device behind the same address: %v", minute, err)
		}
		clock.advance(time.Minute)
	}

	// IPv6 clients share the limit of their /64.
	v6 := func(addr string, i int) DeviceRequest {
		return DeviceRequest{Token: fmt.Sprintf("ccdt_%064x", 1000+i), ClientIP: addr}
	}
	for i := 0; i < authFailuresPerWindow; i++ {
		addr := fmt.Sprintf("2001:db8:1:2::%x", i+1)
		if _, err := env.svc.DeviceHeartbeat(ctx, v6(addr, i), heartbeatFor(good.Device, build.ID)); ErrorCode(err) != CodeUnauthorized {
			t.Fatalf("v6 guess %d: %v", i, err)
		}
	}
	if _, err := env.svc.DeviceHeartbeat(ctx, v6("2001:db8:1:2:ffff::1", 99), heartbeatFor(good.Device, build.ID)); ErrorCode(err) != CodeRateLimited {
		t.Fatalf("another address of the same /64: %v", err)
	}
	other := DeviceRequest{Token: goodToken, ClientIP: "2001:db8:1:3::1"}
	if _, err := env.svc.DeviceHeartbeat(ctx, other, heartbeatFor(good.Device, build.ID)); err != nil {
		t.Fatalf("another /64: %v", err)
	}
}

func TestDeviceAuthFailureKeysAreCapped(t *testing.T) {
	var l deviceLimiter
	now := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	for i := 0; i < maxAuthClients; i++ {
		l.authFailed(fmt.Sprintf("10.%d.%d.1", i/256, i%256), "ccdt_bad", now)
	}
	// The map is full of fresh keys: failures of new addresses are dropped, so
	// memory stays bounded and no client is blocked through a shared bucket.
	for i := 0; i < authFailuresPerWindow; i++ {
		l.authFailed("192.168.0.1", fmt.Sprintf("guess-%d", i), now)
		l.authFailed(fmt.Sprintf("192.168.%d.1", i+1), fmt.Sprintf("guess-%d", i), now)
	}
	if n := len(l.auth); n != maxAuthClients {
		t.Fatalf("keys = %d, want %d", n, maxAuthClients)
	}
	for _, client := range []string{"192.168.0.1", "192.168.200.1"} {
		if l.authBlocked(client, now) {
			t.Fatalf("%s is blocked without recorded failures", client)
		}
	}
	// A known address keeps its own key and is limited as usual, with at most
	// authFailuresPerWindow tokens.
	for i := 0; i < 2*authFailuresPerWindow; i++ {
		l.authFailed("10.0.0.1", fmt.Sprintf("guess-%d", i), now)
	}
	if !l.authBlocked("10.0.0.1", now) || len(l.auth["10.0.0.1"]) != authFailuresPerWindow {
		t.Fatalf("known address: blocked %v, %d tokens", l.authBlocked("10.0.0.1", now), len(l.auth["10.0.0.1"]))
	}
	// After the window the sweep frees the map: new addresses get their own key.
	later := now.Add(authFailureWindow)
	if l.authBlocked("192.168.201.1", later) {
		t.Fatal("blocked after the window")
	}
	l.authFailed("192.168.201.1", "guess", later)
	if n := len(l.auth); n != 1 {
		t.Fatalf("keys after the sweep = %d, want 1", n)
	}
}

// An attacker who fills the failure map and then guesses from one more
// address must not lock out a clean client (heartbeat and firmware download).
func TestDeviceAuthOverflowNeverBlocksACleanClient(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Flut")
	first := succeededBuild(t, env, p.ID)
	minted, token := mintTestDevice(t, env, p.ID, first.ID)
	second := succeededBuild(t, env, p.ID)
	target := second.ID
	if _, err := env.svc.UpdateDevice(ctx, minted.Device.ID, DeviceUpdate{TargetBuildID: &target}); err != nil {
		t.Fatal(err)
	}
	now := env.svc.now()
	for i := 0; i < maxAuthClients; i++ {
		env.svc.deviceLimits.authFailed(fmt.Sprintf("10.%d.%d.1", i/256, i%256), "ccdt_bad", now)
	}
	attacker := DeviceRequest{ClientIP: "203.0.113.7"}
	for i := 0; i <= authFailuresPerWindow; i++ {
		attacker.Token = fmt.Sprintf("ccdt_%064x", i+1)
		if _, err := env.svc.DeviceHeartbeat(ctx, attacker, heartbeatFor(minted.Device, first.ID)); ErrorCode(err) != CodeUnauthorized {
			t.Fatalf("overflow guess %d: %v", i, err)
		}
	}
	clean := DeviceRequest{Token: token, ClientIP: "198.51.100.20"}
	if _, err := env.svc.DeviceHeartbeat(ctx, clean, heartbeatFor(minted.Device, first.ID)); err != nil {
		t.Fatalf("clean client heartbeat: %v", err)
	}
	f, _, err := env.svc.DeviceFirmware(ctx, clean, second.ID)
	if err != nil {
		t.Fatalf("clean client firmware: %v", err)
	}
	_ = f.Close()
	if n := len(env.svc.deviceLimits.auth); n != maxAuthClients {
		t.Fatalf("failure keys = %d, want %d", n, maxAuthClients)
	}
}

func TestDeviceLimiterForgetsIdleKeys(t *testing.T) {
	var l deviceLimiter
	start := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	for i := 0; i < deviceLimiterSweepKeys; i++ {
		l.allow(fmt.Sprintf("hb:ccd_%04d", i), start, heartbeatsPerMinute, time.Minute)
	}
	// One more key after every earlier one left the longest window: the sweep
	// drops the idle keys, so keys of removed devices cannot pile up.
	l.allow("hb:ccd_new", start.Add(longestDeviceWindow), heartbeatsPerMinute, time.Minute)
	if n := len(l.hits); n != 1 {
		t.Fatalf("keys after the sweep = %d, want 1", n)
	}
}

func TestClientKey(t *testing.T) {
	for in, want := range map[string]string{
		"192.168.1.50":           "192.168.1.50",
		" 10.0.0.9 ":             "10.0.0.9",
		"::ffff:192.168.1.50":    "192.168.1.50",
		"2001:db8:1:2:3:4:5:6":   "2001:db8:1:2::/64",
		"2001:db8:1:2::1%eth0":   "2001:db8:1:2::/64",
		"":                       "",
		"not an address":         "not an address",
		strings.Repeat("x", 300): clipEcho(strings.Repeat("x", 300)),
	} {
		if got := clientKey(in); got != want {
			t.Errorf("clientKey(%q) = %q, want %q", in, got, want)
		}
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
	third := succeededBuild(t, env, p.ID)
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

	// The firmware keeps reporting rolled_back for the rest of the boot; another
	// target the user sets meanwhile survives those heartbeats.
	target = third.ID
	setTarget()
	resp, d = beat(OTAStateRolledBack, "rolled back from "+second.ID)
	if d.TargetBuildID != third.ID || d.LastError != "" || resp.Update == nil || resp.Update.BuildID != third.ID {
		t.Fatalf("new target during the rolled back boot: %+v, update %+v", d, resp.Update)
	}

	// Picking the build that rolled back in this boot again is reset: it can be
	// retried once the board restarted (the firmware then reports idle).
	target = second.ID
	setTarget()
	if resp, d = beat(OTAStateRolledBack, "rolled back from "+second.ID); d.TargetBuildID != first.ID || resp.Update != nil {
		t.Fatalf("the failed build picked again in the same boot: %+v, update %+v", d, resp.Update)
	}
	setTarget()
	if resp, d = beat(OTAStateIdle, ""); d.TargetBuildID != second.ID || resp.Update == nil {
		t.Fatalf("retry after a restart: %+v, update %+v", d, resp.Update)
	}

	// Format characters in the device message are dropped.
	if _, d = beat(OTAStateFailed, "time"+string(rune(0x200b))+"out"+string(rune(0x202e))); d.LastError != "failed: timeout" {
		t.Fatalf("last_error with format characters = %q", d.LastError)
	}
}

// A second rollback while the stored state already is rolled_back (the user
// picked another build during the rolled back boot and it failed too) must
// stop offering that build: no reinstall loop. Heartbeat and ota-result agree.
func TestDeviceSecondRollbackInTheSameStateStopsTheOffer(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Zweimal")
	first := succeededBuild(t, env, p.ID)
	minted, token := mintTestDevice(t, env, p.ID, first.ID)
	second := succeededBuild(t, env, p.ID)
	third := succeededBuild(t, env, p.ID)
	set := func(id string) {
		t.Helper()
		if _, err := env.svc.UpdateDevice(ctx, minted.Device.ID, DeviceUpdate{TargetBuildID: &id}); err != nil {
			t.Fatal(err)
		}
	}
	beat := func(state, message string) HeartbeatResponse {
		t.Helper()
		hb := heartbeatFor(minted.Device, first.ID)
		hb.OTAState, hb.OTAError = state, message
		resp, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), hb)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	device := func() Device {
		t.Helper()
		list, _ := env.svc.ListDevices(ctx, p.ID)
		return list[0]
	}
	rolledBack := func(id string) {
		t.Helper()
		if err := env.svc.DeviceOTAResult(ctx, deviceReq(token), OTAResult{BuildID: id, Error: "rolled_back"}); err != nil {
			t.Fatal(err)
		}
	}

	beat(OTAStateIdle, "")
	set(second.ID)
	if beat(OTAStateIdle, "").Update == nil {
		t.Fatal("second is not offered")
	}
	// second crashed before its first heartbeat; the old image reports the rollback.
	if r := beat(OTAStateRolledBack, "rolled back from "+second.ID); r.Update != nil {
		t.Fatalf("offer after the first rollback: %+v", r.Update)
	}
	rolledBack(second.ID)
	set(third.ID)
	if r := beat(OTAStateRolledBack, "rolled back from "+second.ID); r.Update == nil || r.Update.BuildID != third.ID {
		t.Fatalf("third is not offered: %+v", r.Update)
	}
	// third crashed too: the old image boots again and reports it.
	if r := beat(OTAStateRolledBack, "rolled back from "+third.ID); r.Update != nil {
		t.Fatalf("REINSTALL LOOP: %s is offered again", r.Update.BuildID)
	}
	if d := device(); d.TargetBuildID != first.ID || d.LastError != "rolled_back: rolled back from "+third.ID || d.OTAState != OTAStateRolledBack {
		t.Fatalf("after the second rollback: %+v", d)
	}

	// The ota-result alone (heartbeat lost) stops the offer as well.
	set(third.ID)
	rolledBack(third.ID)
	if d := device(); d.TargetBuildID != first.ID || d.LastError != OTAStateRolledBack {
		t.Fatalf("after the second rollback report: %+v", d)
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

// A target for another board (a stale row) is neither offered nor served.
func TestDeviceUpdatesNeedTheDevicesBoard(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Platine")
	first := succeededBuild(t, env, p.ID)
	minted, token := mintTestDevice(t, env, p.ID, first.ID)
	second := succeededBuild(t, env, p.ID)
	target := second.ID
	if _, err := env.svc.UpdateDevice(ctx, minted.Device.ID, DeviceUpdate{TargetBuildID: &target}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.db.Exec(`UPDATE cc_devices SET board='jc4827w543' WHERE id=?`, minted.Device.ID); err != nil {
		t.Fatal(err)
	}
	resp, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), heartbeatFor(minted.Device, first.ID))
	if err != nil || resp.Update != nil {
		t.Fatalf("heartbeat = %+v, %v; want no offer for another board", resp, err)
	}
	if _, _, err := env.svc.DeviceFirmware(ctx, deviceReq(token), second.ID); ErrorCode(err) != CodeNotFound {
		t.Fatalf("firmware for another board: %v", err)
	}
}

func deviceEventCount(t *testing.T, env *testEnv, projectID string) int {
	t.Helper()
	var n int
	if err := env.svc.db.QueryRow(`SELECT COUNT(*) FROM cc_events WHERE project_id=? AND type=?`, projectID, EventDeviceChanged).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A device that comes back online is announced; IP or template version
// changes alone are announced at most once per heartbeat interval.
func TestDeviceHeartbeatEvents(t *testing.T) {
	env, clock := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Ereignis")
	build := succeededBuild(t, env, p.ID)
	minted, token := mintTestDevice(t, env, p.ID, build.ID)
	beat := func(ip, version string) {
		t.Helper()
		hb := heartbeatFor(minted.Device, build.ID)
		hb.IP, hb.TemplateVersion = ip, version
		if _, err := env.svc.DeviceHeartbeat(ctx, deviceReq(token), hb); err != nil {
			t.Fatal(err)
		}
	}
	beat("192.168.1.50", "1.0.0") // activation
	base := deviceEventCount(t, env, p.ID)
	beat("192.168.1.50", "1.0.0")
	if n := deviceEventCount(t, env, p.ID); n != base {
		t.Fatalf("plain heartbeat: %d events, want %d", n, base)
	}

	// Offline (three silent intervals and more), then back: one event.
	clock.advance(3*time.Minute + 5*time.Second)
	beat("192.168.1.50", "1.0.0")
	if n := deviceEventCount(t, env, p.ID); n != base+1 {
		t.Fatalf("back online: %d events, want %d", n, base+1)
	}

	clock.advance(time.Second)
	beat("192.168.1.51", "1.0.0")
	beat("192.168.1.50", "1.0.0")
	beat("192.168.1.50", "1.0.1")
	if n := deviceEventCount(t, env, p.ID); n != base+2 {
		t.Fatalf("alternating details within one interval: %d events, want %d", n, base+2)
	}
	clock.advance(time.Minute)
	beat("192.168.1.52", "1.0.1")
	if n := deviceEventCount(t, env, p.ID); n != base+3 {
		t.Fatalf("details change in the next interval: %d events, want %d", n, base+3)
	}
	list, _ := env.svc.ListDevices(ctx, p.ID)
	if list[0].IP != "192.168.1.52" {
		t.Fatalf("stored ip = %q", list[0].IP)
	}
}

// The expiry delete only removes a row that is still pending and overdue: a
// row read before the project lock may have been activated meanwhile.
func TestRemoveExpiredDeviceKeepsAnActivatedRow(t *testing.T) {
	env, clock := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Ablauf")
	build := succeededBuild(t, env, p.ID)
	minted, _ := mintTestDevice(t, env, p.ID, build.ID)
	stale, err := env.svc.loadDeviceRow(ctx, minted.Device.ID)
	if err != nil {
		t.Fatal(err)
	}
	clock.advance(DevicePendingTTL + time.Second)
	if _, err := env.svc.db.Exec(`UPDATE cc_devices SET status=?, expires_at='' WHERE id=?`, DeviceActive, stale.ID); err != nil {
		t.Fatal(err)
	}
	if removed, err := env.svc.removeExpiredDevice(ctx, stale, env.svc.now()); removed || err != nil {
		t.Fatalf("activated row: removed %v, %v", removed, err)
	}
	if _, err := env.svc.loadDeviceRow(ctx, stale.ID); err != nil {
		t.Fatalf("the activated device must stay: %v", err)
	}
	if _, err := env.svc.db.Exec(`UPDATE cc_devices SET status=?, expires_at=? WHERE id=?`, DevicePending, stale.ExpiresAt, stale.ID); err != nil {
		t.Fatal(err)
	}
	if removed, err := env.svc.removeExpiredDevice(ctx, stale, env.svc.now()); !removed || err != nil {
		t.Fatalf("expired pending row: removed %v, %v", removed, err)
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
