package homedash

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func storeDoc(slug string) Dashboard {
	return Dashboard{
		SchemaVersion: 1, Slug: slug, Name: "Board",
		Look:  Look{RadiusScale: 1},
		Grid:  Grid{RowUnit: RowUnit{Auto: true}},
		Pages: []Page{{ID: "main", Name: "Main", Widgets: []Widget{}, Layouts: Layouts{LG: []LayoutItem{}}}},
	}
}

func openTestStore(t *testing.T) (*Store, *time.Time, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hd.db")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	now := time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC)
	st.now = func() time.Time { return now }
	return st, &now, path
}

func code(err error) string {
	var he *Error
	if errors.As(err, &he) {
		return he.Code
	}
	if err != nil {
		return "untyped: " + err.Error()
	}
	return ""
}

func TestStoreRevisionConflictAndHistoryCap20(t *testing.T) {
	st, _, _ := openTestStore(t)
	ctx := context.Background()
	d, err := st.CreateDashboard(ctx, storeDoc("a"), SavedByUser)
	if err != nil || d.Revision != 1 || d.ApprovedRevision != 1 || !ValidDashboardID(d.ID) {
		t.Fatalf("create: %+v %v", d, err)
	}
	d2, err := st.UpdateDashboard(ctx, d, 1, SavedByAgent)
	if err != nil || d2.Revision != 2 || d2.ApprovedRevision != 1 {
		t.Fatalf("agent update: %+v %v", d2, err)
	}
	_, err = st.UpdateDashboard(ctx, d, 1, SavedByUser)
	var he *Error
	if !errors.As(err, &he) || he.Code != CodeRevisionConflict || he.CurrentRevision != 2 {
		t.Fatalf("stale update: %v", err)
	}
	cur := d2
	for cur.Revision < 30 {
		if cur, err = st.UpdateDashboard(ctx, cur, cur.Revision, SavedByAgent); err != nil {
			t.Fatal(err)
		}
	}
	revs, err := st.Revisions(ctx, d.ID)
	if err != nil || len(revs) != 21 || revs[0].Revision != 30 || revs[19].Revision != 11 || revs[20].Revision != 1 {
		t.Fatalf("revisions (newest first, last 20 plus the approved one): %+v %v", revs, err)
	}
	if appr, err := st.GetApproved(ctx, d.ID); err != nil || appr.Revision != 1 {
		t.Fatalf("approved revision lost: %+v %v", appr, err)
	}
	if _, err := st.CreateDashboard(ctx, storeDoc("a"), SavedByUser); code(err) != CodeSlugTaken {
		t.Fatalf("duplicate slug: %v", err)
	}
}

func TestStoreApprovedRevisionAdvancesOnlyForUserSaves(t *testing.T) {
	st, _, _ := openTestStore(t)
	ctx := context.Background()
	d, _ := st.CreateDashboard(ctx, storeDoc("b"), SavedByAgent)
	if d.ApprovedRevision != 0 {
		t.Fatalf("agent create approved %d", d.ApprovedRevision)
	}
	if _, err := st.GetApproved(ctx, d.ID); code(err) != CodeDashboardNotFound {
		t.Fatalf("no approved revision yet: %v", err)
	}
	u, _ := st.UpdateDashboard(ctx, d, 1, SavedByUser)
	a, _ := st.UpdateDashboard(ctx, u, 2, SavedByAgent)
	if u.ApprovedRevision != 2 || a.ApprovedRevision != 2 || a.Revision != 3 {
		t.Fatalf("user %d agent %d/%d", u.ApprovedRevision, a.ApprovedRevision, a.Revision)
	}
	if ap, err := st.Approve(ctx, d.ID, 3); err != nil || ap.ApprovedRevision != 3 {
		t.Fatalf("approve: %+v %v", ap, err)
	}
	if _, err := st.Approve(ctx, d.ID, 99); code(err) != CodeRevisionNotFound {
		t.Fatalf("approve unknown revision: %v", err)
	}
}

func TestStoreKioskPairingSingleUseAndExpiry(t *testing.T) {
	st, now, _ := openTestStore(t)
	ctx := context.Background()
	d, _ := st.CreateDashboard(ctx, storeDoc("c"), SavedByUser)
	codeStr, _, err := st.CreatePairing(ctx, d.ID, "Hallway", false, 15*time.Minute)
	if err != nil || len(codeStr) != 26 {
		t.Fatalf("pairing %q %v", codeStr, err)
	}
	if _, _, err := st.RedeemPairing(ctx, "d_aaaaaaaaaaaa", codeStr, "10.0.0.2", 0, 10); code(err) != CodePairingInvalid {
		t.Fatalf("wrong dashboard: %v", err)
	}
	dev, token, err := st.RedeemPairing(ctx, d.ID, strings.ToLower(codeStr), "10.0.0.2", time.Hour, 10)
	if err != nil || !strings.HasPrefix(token, DeviceTokenPrefix) || dev.Label != "Hallway" {
		t.Fatalf("redeem: %+v %v", dev, err)
	}
	if _, _, err := st.RedeemPairing(ctx, d.ID, codeStr, "10.0.0.2", 0, 10); code(err) != CodePairingInvalid {
		t.Fatalf("second redeem: %v", err)
	}
	if got, err := st.DeviceByToken(ctx, token); err != nil || got.ID != dev.ID {
		t.Fatalf("lookup: %+v %v", got, err)
	}
	late, _, _ := st.CreatePairing(ctx, d.ID, "", true, 15*time.Minute)
	*now = now.Add(2 * time.Hour)
	if _, _, err := st.RedeemPairing(ctx, d.ID, late, "10.0.0.3", 0, 10); code(err) != CodePairingInvalid {
		t.Fatalf("expired code: %v", err)
	}
	if _, err := st.DeviceByToken(ctx, token); code(err) != CodeKioskRevoked {
		t.Fatalf("expired device: %v", err)
	}
	if _, err := st.DeviceByToken(ctx, DeviceTokenPrefix+strings.Repeat("0", 64)); code(err) != CodeKioskUnpaired {
		t.Fatalf("unknown token: %v", err)
	}
}

func TestStoreKioskTokensStoredOnlyAsHash(t *testing.T) {
	st, _, path := openTestStore(t)
	ctx := context.Background()
	d, _ := st.CreateDashboard(ctx, storeDoc("e"), SavedByUser)
	codeStr, _, _ := st.CreatePairing(ctx, d.ID, "Wall", false, 15*time.Minute)
	_, token, err := st.RedeemPairing(ctx, d.ID, codeStr, "10.0.0.2", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	for _, p := range []string{path, path + "-wal"} {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if bytes.Contains(data, []byte(token)) || bytes.Contains(data, []byte(codeStr)) {
			t.Fatalf("%s contains a clear-text token or pairing code", p)
		}
	}
}

func TestStoreDeleteDashboardCascadesDevices(t *testing.T) {
	st, _, _ := openTestStore(t)
	ctx := context.Background()
	d, _ := st.CreateDashboard(ctx, storeDoc("f"), SavedByUser)
	codeStr, _, _ := st.CreatePairing(ctx, d.ID, "Wall", false, 15*time.Minute)
	dev, token, _ := st.RedeemPairing(ctx, d.ID, codeStr, "10.0.0.2", 0, 10)
	if _, err := st.DeleteDashboard(ctx, d.ID, 99); code(err) != CodeRevisionConflict {
		t.Fatalf("delete with stale revision: %v", err)
	}
	revoked, err := st.DeleteDashboard(ctx, d.ID, 1)
	if err != nil || !slices.Equal(revoked, []string{dev.ID}) {
		t.Fatalf("delete: %v %v", revoked, err)
	}
	if list, _ := st.ListDevices(ctx, d.ID); len(list) != 0 {
		t.Fatalf("devices left: %+v", list)
	}
	if _, err := st.DeviceByToken(ctx, token); code(err) != CodeKioskUnpaired {
		t.Fatalf("token after delete: %v", err)
	}
	if _, err := st.GetDashboard(ctx, d.ID); code(err) != CodeDashboardNotFound {
		t.Fatalf("dashboard after delete: %v", err)
	}
}

func TestStoreRefusesUpdateOfNewerSchema(t *testing.T) {
	st, _, _ := openTestStore(t)
	ctx := context.Background()
	d, _ := st.CreateDashboard(ctx, storeDoc("g"), SavedByUser)
	if _, err := st.db.Exec(`UPDATE hd_dashboards SET schema_version = ? WHERE id = ?`, SchemaVersion+1, d.ID); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := st.db.QueryRow(`SELECT document_json FROM hd_dashboards WHERE id = ?`, d.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	d.Name = "Changed"
	if _, err := st.UpdateDashboard(ctx, d, 1, SavedByUser); code(err) != CodeSchemaTooNew {
		t.Fatalf("update of newer schema: %v", err)
	}
	var after string
	var rev int64
	var schema int
	if err := st.db.QueryRow(`SELECT document_json, revision, schema_version FROM hd_dashboards WHERE id = ?`, d.ID).Scan(&after, &rev, &schema); err != nil {
		t.Fatal(err)
	}
	if after != before || rev != 1 || schema != SchemaVersion+1 {
		t.Fatalf("row changed: rev %d schema %d", rev, schema)
	}
}

func TestStoreDeviceCapIgnoresExpiredAndFailsClosed(t *testing.T) {
	st, now, _ := openTestStore(t)
	ctx := context.Background()
	d, _ := st.CreateDashboard(ctx, storeDoc("h"), SavedByUser)
	pair := func() string {
		c, _, err := st.CreatePairing(ctx, d.ID, "Wall", false, 15*time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if _, _, err := st.RedeemPairing(ctx, d.ID, pair(), "10.0.0.1", time.Hour, 2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.RedeemPairing(ctx, d.ID, pair(), "10.0.0.2", 0, 2); err != nil {
		t.Fatal(err)
	}
	full := pair()
	if _, _, err := st.RedeemPairing(ctx, d.ID, full, "10.0.0.3", 0, 2); code(err) != CodePairingInvalid {
		t.Fatalf("cap reached: %v", err)
	}
	// maxDevices <= 0 refuses with the same error value and keeps the code usable.
	_, _, err0 := st.RedeemPairing(ctx, d.ID, full, "10.0.0.3", 0, 0)
	_, _, errFull := st.RedeemPairing(ctx, d.ID, full, "10.0.0.3", 0, 2)
	if err0 != errPairingInvalid || errFull != errPairingInvalid {
		t.Fatalf("not the uniform error: %v / %v", err0, errFull)
	}
	*now = now.Add(2 * time.Minute) // the hour-long device is still live
	if _, _, err := st.RedeemPairing(ctx, d.ID, full, "10.0.0.3", 0, 3); err != nil {
		t.Fatalf("code consumed by a refusal: %v", err)
	}
	// Expire the first device: it no longer counts against the cap.
	*now = now.Add(2 * time.Hour)
	if _, _, err := st.RedeemPairing(ctx, d.ID, pair(), "10.0.0.4", 0, 3); err != nil {
		t.Fatalf("expired device still blocks pairing: %v", err)
	}
	if list, _ := st.ListDevices(ctx, d.ID); len(list) != 3 {
		t.Fatalf("devices after cleanup: %d", len(list))
	}
}

func TestStoreCreatePairingDropsExpiredRows(t *testing.T) {
	st, now, _ := openTestStore(t)
	ctx := context.Background()
	d, _ := st.CreateDashboard(ctx, storeDoc("i"), SavedByUser)
	first, _, _ := st.CreatePairing(ctx, d.ID, "A", false, 15*time.Minute)
	if _, _, err := st.RedeemPairing(ctx, d.ID, first, "10.0.0.1", 0, 5); err != nil { // used, then expires
		t.Fatal(err)
	}
	st.CreatePairing(ctx, d.ID, "B", false, 15*time.Minute) // unused, then expires
	*now = now.Add(time.Hour)
	if _, _, err := st.CreatePairing(ctx, d.ID, "C", false, 15*time.Minute); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM hd_kiosk_pairings`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("pairing rows left: %d %v", n, err)
	}
}

func TestStoreDashboardNames(t *testing.T) {
	st, _, _ := openTestStore(t)
	ctx := context.Background()
	a, err := st.CreateDashboard(ctx, storeDoc("a"), SavedByUser)
	if err != nil {
		t.Fatal(err)
	}
	bdoc := storeDoc("b")
	bdoc.Name = "Küche"
	b, err := st.CreateDashboard(ctx, bdoc, SavedByUser)
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.DashboardNames(ctx, []string{a.ID, b.ID, "d_zzzzzzzzzzzz"})
	if err != nil || len(got) != 2 || got[a.ID] != "Board" || got[b.ID] != "Küche" {
		t.Fatalf("names = %v, %v", got, err)
	}
	if _, ok := got["d_zzzzzzzzzzzz"]; ok {
		t.Fatal("unknown id must be absent")
	}
	empty, err := st.DashboardNames(ctx, nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty input = %v, %v", empty, err)
	}
}
