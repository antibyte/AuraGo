package desktop

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"aurago/internal/retronet"
)

const retroNetTestEntries = `{"version":1,"entries":[` +
	`{"id":"own-sshgame01","name":"SSH Game","protocol":"ssh","host":"game.example.com","port":2222,"user":"guest"},` +
	`{"id":"own-sshgame02","name":"Second SSH Game","protocol":"ssh","host":"game2.example.com","port":22,"user":"guest"},` +
	`{"id":"own-telnetbb1","name":"Telnet BBS","description":"Local board","protocol":"telnet","host":"bbs.example.com","port":23,"kind":"bbs","charset":"cp437"}]}`

func TestRetroNetEntriesSettingIsRegisteredAndValidated(t *testing.T) {
	defaults := DesktopSettingDefaults()
	if defaults[retronet.EntriesSetting] != retronet.DefaultEntriesDocument {
		t.Fatalf("default %s = %q", retronet.EntriesSetting, defaults[retronet.EntriesSetting])
	}
	if err := validateFreeformDesktopSetting(retronet.EntriesSetting, retroNetTestEntries); err != nil {
		t.Fatalf("setting validation did not route to retronet.ValidateEntriesDocument: %v", err)
	}
	svc := testService(t)
	ctx := context.Background()
	entries, err := svc.RetroNetEntries(ctx)
	if err != nil || len(entries) != 0 {
		t.Fatalf("fresh desktop entries = %+v, %v", entries, err)
	}
	if err := svc.SetSetting(ctx, retronet.EntriesSetting, retroNetTestEntries, SourceUser); err != nil {
		t.Fatalf("valid entries rejected: %v", err)
	}
	for _, invalid := range []string{
		`not json`,
		`{"version":1,"entries":[],"extra":true}`,
		strings.Replace(retroNetTestEntries, "bbs.example.com", "192.168.1.10", 1),
		strings.Replace(retroNetTestEntries, "bbs.example.com", "127.0.0.1", 1),
		strings.Replace(retroNetTestEntries, `"port":23`, `"port":25`, 1),
	} {
		err := svc.SetSetting(ctx, retronet.EntriesSetting, invalid, SourceUser)
		if err == nil {
			t.Fatalf("accepted invalid entries: %.120s", invalid)
		}
		if strings.Contains(err.Error(), "192.168") || strings.Contains(err.Error(), "not json") {
			t.Fatalf("validation error echoes user input: %v", err)
		}
	}
	entries, err = svc.RetroNetEntries(ctx)
	if err != nil || len(entries) != 3 {
		t.Fatalf("stored entries = %+v, %v", entries, err)
	}
	for _, entry := range entries {
		if !entry.Own || entry.Category != retronet.CategoryOwn {
			t.Fatalf("own entry not marked: %+v", entry)
		}
	}
	if entries[2].ID != "own-telnetbb1" || entries[2].Kind != retronet.KindBBS || entries[2].Charset != retronet.CharsetCP437 {
		t.Fatalf("telnet entry = %+v", entries[2])
	}
}

func TestRetroNetHostKeyIsStoredOnceForOwnSSHEntries(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.SetSetting(ctx, retronet.EntriesSetting, retroNetTestEntries, SourceUser); err != nil {
		t.Fatal(err)
	}
	first := "SHA256:" + strings.Repeat("A", 43)
	second := "SHA256:" + strings.Repeat("B", 43)
	dialed := dialedRetroNetEntries(t, svc)

	if err := svc.SetRetroNetHostKey(ctx, dialed["own-sshgame01"], "SHA256:short"); err == nil {
		t.Fatal("a malformed fingerprint was stored")
	}
	svc.SetReadOnly(true)
	if err := svc.SetRetroNetHostKey(ctx, dialed["own-sshgame01"], first); err == nil {
		t.Fatal("a host key was stored while the desktop is readonly")
	}
	svc.SetReadOnly(false)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for id, fingerprint := range map[string]string{"own-sshgame01": first, "own-sshgame02": second} {
		wg.Add(1)
		go func(id, fingerprint string) {
			defer wg.Done()
			errs <- svc.SetRetroNetHostKey(ctx, dialed[id], fingerprint)
		}(id, fingerprint)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent first-contact store: %v", err)
		}
	}
	entries, err := svc.RetroNetEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{}
	for _, entry := range entries {
		keys[entry.ID] = entry.HostKey
	}
	if keys["own-sshgame01"] != first || keys["own-sshgame02"] != second || keys["own-telnetbb1"] != "" {
		t.Fatalf("stored keys = %v (lost update?)", keys)
	}
	catalog, ok := retronet.Lookup(retronet.DefaultCatalog(), nil, "telehack-ssh")
	if !ok {
		t.Fatal("catalog entry telehack-ssh is missing")
	}
	missing := dialed["own-sshgame02"]
	missing.ID = "own-missing01"
	for _, tc := range []struct {
		name   string
		dialed retronet.Entry
	}{
		{"already pinned", dialed["own-sshgame01"]},
		{"unknown entry", missing},
		{"telnet entry", dialed["own-telnetbb1"]},
		{"catalog entry", catalog},
	} {
		if err := svc.SetRetroNetHostKey(ctx, tc.dialed, second); err == nil {
			t.Fatalf("%s: SetRetroNetHostKey accepted %s", tc.name, tc.dialed.ID)
		}
	}
	entries, _ = svc.RetroNetEntries(ctx)
	if entries[0].HostKey != first {
		t.Fatalf("a refused call changed the stored key: %+v", entries[0])
	}
}

// dialedRetroNetEntries returns the stored own entries by ID, as a session
// dials them.
func dialedRetroNetEntries(t *testing.T, svc *Service) map[string]retronet.Entry {
	t.Helper()
	entries, err := svc.RetroNetEntries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]retronet.Entry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	return byID
}

// The key belongs to the target that was dialed: an edit of protocol, host,
// port or user while the prompt was open must not pin it to the new target;
// other edits of the same entry keep it.
func TestRetroNetHostKeyIsStoredOnlyForTheDialedTarget(t *testing.T) {
	const original = `{"id":"own-sshgame01","name":"SSH Game","protocol":"ssh","host":"game.example.com","port":2222,"user":"guest"}`
	fingerprint := "SHA256:" + strings.Repeat("A", 43)
	for _, tc := range []struct {
		name, edited string
		stored       bool
	}{
		{"unchanged", original, true},
		{"renamed", strings.Replace(original, `"name":"SSH Game"`, `"name":"Renamed Game"`, 1), true},
		{"described", strings.Replace(original, `"user":"guest"`, `"user":"guest","description":"Now with notes"`, 1), true},
		{"host", strings.Replace(original, "game.example.com", "other.example.com", 1), false},
		{"port", strings.Replace(original, `"port":2222`, `"port":2223`, 1), false},
		{"user", strings.Replace(original, `"user":"guest"`, `"user":"visitor"`, 1), false},
		{"protocol", `{"id":"own-sshgame01","name":"SSH Game","protocol":"telnet","host":"game.example.com","port":2222,"kind":"world","charset":"utf8"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := testService(t)
			ctx := context.Background()
			if err := svc.SetSetting(ctx, retronet.EntriesSetting, `{"version":1,"entries":[`+original+`]}`, SourceUser); err != nil {
				t.Fatal(err)
			}
			dialed := dialedRetroNetEntries(t, svc)["own-sshgame01"]
			// The user edits the entry while the host-key prompt is open.
			if err := svc.SetSetting(ctx, retronet.EntriesSetting, `{"version":1,"entries":[`+tc.edited+`]}`, SourceUser); err != nil {
				t.Fatal(err)
			}
			err := svc.SetRetroNetHostKey(ctx, dialed, fingerprint)
			stored := dialedRetroNetEntries(t, svc)["own-sshgame01"]
			if tc.stored {
				if err != nil || stored.HostKey != fingerprint {
					t.Fatalf("host key = %q, %v; want it stored", stored.HostKey, err)
				}
				return
			}
			if err == nil || stored.HostKey != "" {
				t.Fatalf("host key = %q, %v; want an error and nothing stored for a changed target", stored.HostKey, err)
			}
			if strings.Contains(err.Error(), fingerprint) {
				t.Fatalf("the error carries the fingerprint: %v", err)
			}
		})
	}
}

// An edit of the target that lands between the read and the compare-and-set
// is seen on the retry and refuses the key.
func TestRetroNetHostKeyRefusesATargetChangedDuringTheUpdate(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.SetSetting(ctx, retronet.EntriesSetting, retroNetTestEntries, SourceUser); err != nil {
		t.Fatal(err)
	}
	dialed := dialedRetroNetEntries(t, svc)["own-sshgame01"]
	moved := strings.Replace(retroNetTestEntries, `"port":2222`, `"port":2223`, 1)
	raceRetroNetHostKeyWrite(t, func() {
		if err := svc.SetSetting(ctx, retronet.EntriesSetting, moved, SourceUser); err != nil {
			t.Errorf("competing edit: %v", err)
		}
	})
	if err := svc.SetRetroNetHostKey(ctx, dialed, "SHA256:"+strings.Repeat("A", 43)); err == nil {
		t.Fatal("a host key was stored for a target changed during the update")
	}
	if stored := dialedRetroNetEntries(t, svc)["own-sshgame01"]; stored.Port != 2223 || stored.HostKey != "" {
		t.Fatalf("stored entry = %+v, want the edit kept and no key", stored)
	}
}

// raceRetroNetHostKeyWrite runs competing once between SetRetroNetHostKey's
// read and its write.
func raceRetroNetHostKeyWrite(t *testing.T, competing func()) {
	t.Helper()
	var once sync.Once
	retroNetHostKeyBeforeWrite = func() { once.Do(competing) }
	t.Cleanup(func() { retroNetHostKeyBeforeWrite = nil })
}

func TestRetroNetHostKeyDoesNotRestoreAConcurrentlyDeletedEntry(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.SetSetting(ctx, retronet.EntriesSetting, retroNetTestEntries, SourceUser); err != nil {
		t.Fatal(err)
	}
	withoutFirst := strings.Replace(retroNetTestEntries,
		`{"id":"own-sshgame01","name":"SSH Game","protocol":"ssh","host":"game.example.com","port":2222,"user":"guest"},`, "", 1)
	raceRetroNetHostKeyWrite(t, func() {
		if err := svc.SetSetting(ctx, retronet.EntriesSetting, withoutFirst, SourceUser); err != nil {
			t.Errorf("competing delete: %v", err)
		}
	})
	dialed := dialedRetroNetEntries(t, svc)["own-sshgame01"]
	if err := svc.SetRetroNetHostKey(ctx, dialed, "SHA256:"+strings.Repeat("A", 43)); err == nil {
		t.Fatal("a host key was stored for an entry deleted during the update")
	}
	entries, err := svc.RetroNetEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].ID != "own-sshgame02" || entries[0].HostKey != "" {
		t.Fatalf("the concurrent deletion was overwritten: %+v", entries)
	}
}

func TestRetroNetHostKeyKeepsAConcurrentUnrelatedEdit(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.SetSetting(ctx, retronet.EntriesSetting, retroNetTestEntries, SourceUser); err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(retroNetTestEntries, `"name":"Telnet BBS"`, `"name":"Edited BBS"`, 1)
	raceRetroNetHostKeyWrite(t, func() {
		if err := svc.SetSetting(ctx, retronet.EntriesSetting, edited, SourceUser); err != nil {
			t.Errorf("competing edit: %v", err)
		}
	})
	fingerprint := "SHA256:" + strings.Repeat("A", 43)
	if err := svc.SetRetroNetHostKey(ctx, dialedRetroNetEntries(t, svc)["own-sshgame01"], fingerprint); err != nil {
		t.Fatalf("the host key was not stored after a lost race: %v", err)
	}
	entries, err := svc.RetroNetEntries(ctx)
	if err != nil || len(entries) != 3 {
		t.Fatalf("stored entries = %+v, %v", entries, err)
	}
	if entries[0].HostKey != fingerprint {
		t.Fatalf("host key lost: %+v", entries[0])
	}
	if entries[2].Name != "Edited BBS" {
		t.Fatalf("the concurrent edit was reverted: %+v", entries[2])
	}
}

func TestCompareAndSetSettingHandlesNeverSavedAndStaleValues(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	key := retronet.EntriesSetting
	edited := strings.Replace(retroNetTestEntries, `"name":"Telnet BBS"`, `"name":"Edited BBS"`, 1)

	if ok, err := svc.compareAndSetSetting(ctx, key, retroNetTestEntries, edited, SourceUser); err != nil || ok {
		t.Fatalf("a never-saved setting matched a non-default old value: %v, %v", ok, err)
	}
	if ok, err := svc.compareAndSetSetting(ctx, key, retronet.DefaultEntriesDocument, retroNetTestEntries, SourceUser); err != nil || !ok {
		t.Fatalf("a never-saved setting did not match its default: %v, %v", ok, err)
	}
	if ok, err := svc.compareAndSetSetting(ctx, key, retronet.DefaultEntriesDocument, edited, SourceUser); err != nil || ok {
		t.Fatalf("a stale default overwrote the saved value: %v, %v", ok, err)
	}
	if _, err := svc.compareAndSetSetting(ctx, key, retroNetTestEntries, `not json`, SourceUser); err == nil {
		t.Fatal("an invalid value was accepted")
	}
	svc.SetReadOnly(true)
	if _, err := svc.compareAndSetSetting(ctx, key, retroNetTestEntries, edited, SourceUser); err == nil {
		t.Fatal("a readonly desktop accepted a compare-and-set")
	}
	svc.SetReadOnly(false)
	if ok, err := svc.compareAndSetSetting(ctx, key, retroNetTestEntries, edited, SourceUser); err != nil || !ok {
		t.Fatalf("a current old value did not match: %v, %v", ok, err)
	}
	settings, err := svc.listSettings(ctx)
	if err != nil || settings[key] != edited {
		t.Fatalf("stored value = %q, %v", settings[key], err)
	}
}

func TestRetroNetHostKeyGivesUpAfterThreeLostRaces(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.SetSetting(ctx, retronet.EntriesSetting, retroNetTestEntries, SourceUser); err != nil {
		t.Fatal(err)
	}
	dialed := dialedRetroNetEntries(t, svc)["own-sshgame01"]
	races := 0
	retroNetHostKeyBeforeWrite = func() {
		races++
		edited := strings.Replace(retroNetTestEntries, `"name":"Telnet BBS"`, fmt.Sprintf(`"name":"Edited BBS %d"`, races), 1)
		if err := svc.SetSetting(ctx, retronet.EntriesSetting, edited, SourceUser); err != nil {
			t.Errorf("competing edit: %v", err)
		}
	}
	t.Cleanup(func() { retroNetHostKeyBeforeWrite = nil })
	if err := svc.SetRetroNetHostKey(ctx, dialed, "SHA256:"+strings.Repeat("A", 43)); err == nil {
		t.Fatal("a host key was stored although every attempt lost its race")
	}
	if races != retroNetHostKeyAttempts {
		t.Fatalf("attempts = %d, want %d", races, retroNetHostKeyAttempts)
	}
	entries, err := svc.RetroNetEntries(ctx)
	if err != nil || entries[0].HostKey != "" || entries[2].Name != "Edited BBS 3" {
		t.Fatalf("stored entries = %+v, %v", entries, err)
	}
}
