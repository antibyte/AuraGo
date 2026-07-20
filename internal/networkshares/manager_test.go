package networkshares

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

type fakeAdapter struct {
	status              Status
	shares              []observedShare
	createErr           error
	updateErr           error
	deleteErr           error
	hideCreated         bool
	createDoesNotAdd    bool
	deleteDoesNotRemove bool
	transformCreate     func(share ShareSpec) ShareSpec
	updateErrAt         map[int]error
	transformUpdate     func(call int, desired ShareSpec) ShareSpec
	createCall          int
	updateCall          int
	deleteCall          int
	listOptions         []Options
	listErrWhenSMB      bool
}

func (f *fakeAdapter) Probe(context.Context, Options) (Status, error) {
	return f.status, nil
}

func (f *fakeAdapter) Validate(context.Context, Options, ShareSpec) error {
	return nil
}

func (f *fakeAdapter) List(_ context.Context, options Options) ([]observedShare, error) {
	f.listOptions = append(f.listOptions, options)
	if f.listErrWhenSMB && options.SMBEnabled {
		return nil, errors.New("SMB backend is unreadable")
	}
	return append([]observedShare(nil), f.shares...), nil
}

func (f *fakeAdapter) Create(_ context.Context, _ Options, share ShareSpec) error {
	f.createCall++
	if f.createErr != nil {
		return f.createErr
	}
	if f.hideCreated {
		return nil
	}
	if f.createDoesNotAdd {
		return nil
	}
	if f.transformCreate != nil {
		share = f.transformCreate(share)
	}
	f.shares = append(f.shares, observedShare{
		ShareSpec:       share,
		MarkerID:        markerID(share.Comment),
		MarkerSupported: true,
		Active:          true,
		CommentObserved: true,
	})
	return nil
}

func (f *fakeAdapter) Update(_ context.Context, _ Options, _, desired ShareSpec) error {
	f.updateCall++
	if err := f.updateErrAt[f.updateCall]; err != nil {
		return err
	}
	if f.updateErr != nil {
		return f.updateErr
	}
	if f.transformUpdate != nil {
		desired = f.transformUpdate(f.updateCall, desired)
	}
	for index := range f.shares {
		if stringsEqualFold(f.shares[index].Protocol, desired.Protocol) &&
			stringsEqualFold(f.shares[index].Name, desired.Name) {
			f.shares[index].ShareSpec = desired
			f.shares[index].MarkerID = markerID(desired.Comment)
			f.shares[index].CommentObserved = true
			return nil
		}
	}
	return errors.New("share not found")
}

func (f *fakeAdapter) Delete(_ context.Context, _ Options, share ShareSpec) error {
	f.deleteCall++
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if f.deleteDoesNotRemove {
		return nil
	}
	for index := range f.shares {
		if stringsEqualFold(f.shares[index].Protocol, share.Protocol) &&
			stringsEqualFold(f.shares[index].Name, share.Name) {
			f.shares = append(f.shares[:index], f.shares[index+1:]...)
			return nil
		}
	}
	return errors.New("share not found")
}

func stringsEqualFold(left, right string) bool {
	return len(left) == len(right) && (left == right || equalFoldASCII(left, right))
}

func equalFoldASCII(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		a, b := left[index], right[index]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

func testManager(t *testing.T, root string, adapter *fakeAdapter) *Manager {
	t.Helper()
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "network_shares.db"))
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	manager := &Manager{
		adapter: adapter,
		ledger:  ledger,
		logger:  slog.Default(),
		options: Options{
			Enabled:              true,
			AllowCreate:          true,
			AllowUpdate:          true,
			AllowDelete:          true,
			AllowedRoots:         []string{root},
			SMBEnabled:           true,
			SMBAllowedPrincipals: []string{"share-users"},
			NFSEnabled:           true,
			NFSAllowedClients:    []string{"192.0.2.0/24"},
		},
		status: adapter.status,
	}
	t.Cleanup(func() { _ = manager.Close() })
	return manager
}

func TestManagerLifecycleOwnsOnlyLedgerSharesAndPreservesDirectory(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "documents")
	if err := os.Mkdir(shareDir, 0o750); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeAdapter{status: Status{
		Supported: true,
		Usable:    true,
		SMB:       ProtocolStatus{Readable: true, Writable: true},
		NFS:       ProtocolStatus{Readable: true, Writable: true},
	}}
	manager := testManager(t, root, adapter)

	created, err := manager.Create(context.Background(), ShareSpec{
		Protocol: ProtocolSMB,
		Name:     "documents",
		Path:     shareDir,
		ReadOnly: true,
		Access: ShareAccess{
			ACL: []ACLEntry{{Principal: "share-users", Level: "read"}},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !created.Managed || !created.Mutable || created.ID == "" {
		t.Fatalf("created share state = %+v", created)
	}

	comment := "Team documents"
	updated, err := manager.Update(context.Background(), created.ID, SharePatch{Comment: &comment})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Comment != comment {
		t.Fatalf("updated comment = %q", updated.Comment)
	}

	if err := manager.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(shareDir); err != nil {
		t.Fatalf("share directory was modified or deleted: %v", err)
	}
	if adapter.createCall != 1 || adapter.updateCall != 1 || adapter.deleteCall != 1 {
		t.Fatalf("unexpected adapter calls: create=%d update=%d delete=%d",
			adapter.createCall, adapter.updateCall, adapter.deleteCall)
	}
}

func TestManagerListsExternalSharesReadOnlyAndHidesOutsideRoots(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside")
	outside := t.TempDir()
	if err := os.Mkdir(inside, 0o750); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeAdapter{
		status: Status{
			Supported: true, Usable: true,
			SMB: ProtocolStatus{Readable: true, Writable: true},
		},
		shares: []observedShare{
			{ShareSpec: ShareSpec{Protocol: ProtocolSMB, Name: "external", Path: inside}, Active: true},
			{ShareSpec: ShareSpec{Protocol: ProtocolSMB, Name: "hidden", Path: outside}, Active: true},
		},
	}
	manager := testManager(t, root, adapter)
	shares, err := manager.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(shares) != 1 || shares[0].Managed || shares[0].Mutable || shares[0].Name != "external" {
		t.Fatalf("scoped external shares = %+v", shares)
	}
	if err := manager.Delete(context.Background(), shares[0].ID); ErrorCode(err) != ErrorNotManaged {
		t.Fatalf("external Delete error = %v, code=%q", err, ErrorCode(err))
	}
}

func TestManagerCreateRejectsManagedConflictBeforeNativeMutation(t *testing.T) {
	root := t.TempDir()
	adapter := &fakeAdapter{status: Status{
		Usable: true,
		NFS:    ProtocolStatus{Readable: true, Writable: true},
	}}
	manager := testManager(t, root, adapter)
	existing := ShareSpec{
		ID:       "11111111-1111-4111-8111-111111111111",
		Protocol: ProtocolNFS,
		Name:     "archive",
		Path:     root,
		ReadOnly: true,
		Access:   ShareAccess{Clients: []string{"192.0.2.0/24"}},
	}
	if err := manager.ledger.put(context.Background(), existing, "missing"); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
	_, err := manager.Create(context.Background(), ShareSpec{
		Protocol: ProtocolNFS,
		Name:     "replacement",
		Path:     root,
		ReadOnly: true,
		Access:   ShareAccess{Clients: []string{"192.0.2.0/24"}},
	})
	if ErrorCode(err) != ErrorConflict {
		t.Fatalf("Create conflict error = %v, want %s", err, ErrorConflict)
	}
	if adapter.createCall != 0 {
		t.Fatalf("native create calls = %d, want 0", adapter.createCall)
	}
}

func TestManagerHidesManagedShareAfterAllowedRootIsRemoved(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "data")
	if err := os.Mkdir(shareDir, 0o750); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeAdapter{status: Status{
		Supported: true, Usable: true,
		SMB: ProtocolStatus{Readable: true, Writable: true},
	}}
	manager := testManager(t, root, adapter)
	created, err := manager.Create(context.Background(), ShareSpec{
		Protocol: ProtocolSMB, Name: "data", Path: shareDir, ReadOnly: true,
		Access: ShareAccess{ACL: []ACLEntry{{Principal: "share-users", Level: "read"}}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	manager.options.AllowedRoots = []string{t.TempDir()}
	shares, err := manager.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(shares) != 0 {
		t.Fatalf("out-of-scope managed share was exposed: %+v", shares)
	}
	record, err := manager.ledger.get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("ledger get: %v", err)
	}
	if record.Drift != "outside_allowed_roots" {
		t.Fatalf("removed-root drift = %q", record.Drift)
	}
}

func TestManagerBlocksDriftedManagedShare(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "data")
	if err := os.Mkdir(shareDir, 0o750); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeAdapter{status: Status{
		Supported: true, Usable: true,
		NFS: ProtocolStatus{Readable: true, Writable: true},
	}}
	manager := testManager(t, root, adapter)
	created, err := manager.Create(context.Background(), ShareSpec{
		Protocol: ProtocolNFS, Name: "data", Path: shareDir, ReadOnly: true,
		Access: ShareAccess{Clients: []string{"192.0.2.0/24"}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	adapter.shares[0].ReadOnly = false
	comment := "must not apply"
	_, err = manager.Update(context.Background(), created.ID, SharePatch{Comment: &comment})
	if ErrorCode(err) != ErrorDrift {
		t.Fatalf("Update drift error = %v, code=%q", err, ErrorCode(err))
	}
}

func TestManagerReadOnlyOverridesGranularPermissions(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "data")
	if err := os.Mkdir(shareDir, 0o750); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeAdapter{status: Status{
		Supported: true, Usable: true,
		SMB: ProtocolStatus{Readable: true, Writable: true},
	}}
	manager := testManager(t, root, adapter)
	manager.options.ReadOnly = true
	_, err := manager.Create(context.Background(), ShareSpec{
		Protocol: ProtocolSMB, Name: "data", Path: shareDir, ReadOnly: true,
		Access: ShareAccess{ACL: []ACLEntry{{Principal: "share-users", Level: "read"}}},
	})
	if ErrorCode(err) != ErrorReadOnly {
		t.Fatalf("Create error = %v, code=%q", err, ErrorCode(err))
	}
}

func TestManagerReprobeKeepsReadableStatusWithoutAvailableRoots(t *testing.T) {
	adapter := &fakeAdapter{status: Status{
		Supported: true,
		Usable:    true,
		SMB:       ProtocolStatus{Supported: true, Installed: true, Readable: true, Writable: true},
	}}
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "network_shares.db"))
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	manager := &Manager{
		adapter: adapter,
		ledger:  ledger,
		logger:  slog.Default(),
		options: Options{Enabled: true, SMBEnabled: true},
	}
	t.Cleanup(func() { _ = manager.Close() })

	status := manager.Reprobe(context.Background())
	if !status.Usable || !status.SMB.Readable {
		t.Fatalf("readable protocol was hidden by missing roots: %+v", status)
	}
	if status.SMB.Writable {
		t.Fatalf("missing roots did not disable mutation capability: %+v", status.SMB)
	}
	if status.Reason == "" {
		t.Fatal("missing-root mutation restriction is not reported")
	}
	if status.LastProbedAt.IsZero() || status.SMB.LastProbedAt.IsZero() || status.NFS.LastProbedAt.IsZero() {
		t.Fatalf("probe timestamps are incomplete: %+v", status)
	}
}

func TestManagerListSkipsEnabledButUnreadableProtocol(t *testing.T) {
	root := t.TempDir()
	adapter := &fakeAdapter{
		status: Status{
			Usable: true,
			SMB:    ProtocolStatus{Readable: false},
			NFS:    ProtocolStatus{Readable: true},
		},
		listErrWhenSMB: true,
		shares: []observedShare{{
			ShareSpec: ShareSpec{
				Protocol: ProtocolNFS,
				Name:     "documents",
				Path:     root,
			},
			Active: true,
		}},
	}
	manager := testManager(t, root, adapter)
	shares, err := manager.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(shares) != 1 || shares[0].Protocol != ProtocolNFS {
		t.Fatalf("shares = %+v, want one NFS share", shares)
	}
	if len(adapter.listOptions) != 1 || adapter.listOptions[0].SMBEnabled || !adapter.listOptions[0].NFSEnabled {
		t.Fatalf("adapter list options = %+v", adapter.listOptions)
	}
}

func TestManagerMarksCreateDriftWhenVerificationAndRollbackFail(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "data")
	if err := os.Mkdir(shareDir, 0o750); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeAdapter{
		status: Status{
			Supported: true, Usable: true,
			NFS: ProtocolStatus{Readable: true, Writable: true},
		},
		transformCreate: func(share ShareSpec) ShareSpec {
			share.ReadOnly = false
			return share
		},
		deleteDoesNotRemove: true,
	}
	manager := testManager(t, root, adapter)
	_, err := manager.Create(context.Background(), ShareSpec{
		Protocol: ProtocolNFS, Name: "data", Path: shareDir, ReadOnly: true,
		Access: ShareAccess{Clients: []string{"192.0.2.0/24"}},
	})
	if ErrorCode(err) != ErrorApplyFailed {
		t.Fatalf("Create error = %v, code=%q", err, ErrorCode(err))
	}
	records, err := manager.ledger.list(context.Background())
	if err != nil {
		t.Fatalf("ledger list: %v", err)
	}
	if len(records) != 1 || records[0].Drift != "rollback_failed" {
		t.Fatalf("rollback drift records = %+v", records)
	}
}

func TestManagerCreateRollbackTrustsVerifiedAbsenceOverCommandError(t *testing.T) {
	root := t.TempDir()
	adapter := &fakeAdapter{
		status: Status{
			Supported: true, Usable: true,
			NFS: ProtocolStatus{Readable: true, Writable: true},
		},
		hideCreated: true,
		deleteErr:   errors.New("share was already absent"),
	}
	manager := testManager(t, root, adapter)
	_, err := manager.Create(context.Background(), ShareSpec{
		Protocol: ProtocolNFS, Name: "data", Path: root, ReadOnly: true,
		Access: ShareAccess{Clients: []string{"192.0.2.0/24"}},
	})
	if ErrorCode(err) != ErrorApplyFailed {
		t.Fatalf("Create error = %v, code=%q", err, ErrorCode(err))
	}
	records, listErr := manager.ledger.list(context.Background())
	if listErr != nil {
		t.Fatalf("ledger list: %v", listErr)
	}
	if len(records) != 0 {
		t.Fatalf("verified-absent rollback must not create drift record: %+v", records)
	}
}

func TestManagerMarksUpdateDriftWhenVerificationRollbackFails(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "data")
	if err := os.Mkdir(shareDir, 0o750); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeAdapter{status: Status{
		Supported: true, Usable: true,
		SMB: ProtocolStatus{Readable: true, Writable: true},
	}}
	manager := testManager(t, root, adapter)
	created, err := manager.Create(context.Background(), ShareSpec{
		Protocol: ProtocolSMB, Name: "data", Path: shareDir, ReadOnly: true,
		Access: ShareAccess{ACL: []ACLEntry{{Principal: "share-users", Level: "read"}}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	adapter.transformUpdate = func(call int, desired ShareSpec) ShareSpec {
		if call == 1 {
			desired.Comment = "unexpected native value"
		}
		return desired
	}
	adapter.updateErrAt = map[int]error{2: errors.New("rollback failed")}
	comment := "requested"
	_, err = manager.Update(context.Background(), created.ID, SharePatch{Comment: &comment})
	if ErrorCode(err) != ErrorApplyFailed {
		t.Fatalf("Update error = %v, code=%q", err, ErrorCode(err))
	}
	record, err := manager.ledger.get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("ledger get: %v", err)
	}
	if record.Drift != "rollback_failed" {
		t.Fatalf("update rollback drift = %q", record.Drift)
	}
}

func TestCompareDesiredAllowsLedgerOnlyNFSComment(t *testing.T) {
	desired := ShareSpec{
		Protocol: ProtocolNFS,
		Path:     filepath.Join(t.TempDir(), "data"),
		Comment:  "ledger-only description",
		ReadOnly: true,
		Access:   ShareAccess{Clients: []string{"192.0.2.0/24"}},
	}
	observed := observedShare{
		ShareSpec: ShareSpec{
			Protocol: ProtocolNFS,
			Path:     desired.Path,
			ReadOnly: true,
			Access:   desired.Access,
		},
		CommentObserved: false,
	}
	if drift := compareDesired(desired, observed); drift != "" {
		t.Fatalf("ledger-only NFS comment produced drift %q", drift)
	}
	observed.CommentObserved = true
	if drift := compareDesired(desired, observed); drift != "comment_changed" {
		t.Fatalf("observed missing comment drift = %q, want comment_changed", drift)
	}
}

func TestManagerMarkerMatchWinsOverReusedOriginalName(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-4111-8111-111111111111"
	spec := ShareSpec{
		ID: id, Protocol: ProtocolSMB, Name: "documents", Path: root, ReadOnly: true,
		Access: ShareAccess{ACL: []ACLEntry{{Principal: "share-users", Level: "read"}}},
	}
	adapter := &fakeAdapter{
		status: Status{
			Supported: true, Usable: true,
			SMB: ProtocolStatus{Readable: true, Writable: true},
		},
		shares: []observedShare{
			{
				ShareSpec: ShareSpec{
					Protocol: ProtocolSMB, Name: "renamed", Path: root,
					Comment: nativeComment("", id), ReadOnly: true, Access: spec.Access,
				},
				MarkerID: id, MarkerSupported: true, Active: true, CommentObserved: true,
			},
			{
				ShareSpec: ShareSpec{
					Protocol: ProtocolSMB, Name: "documents", Path: root,
					ReadOnly: true, Access: spec.Access,
				},
				MarkerSupported: true, Active: true, CommentObserved: true,
			},
		},
	}
	manager := testManager(t, root, adapter)
	if err := manager.ledger.put(context.Background(), spec, ""); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
	shares, err := manager.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var managed Share
	for _, share := range shares {
		if share.ID == id {
			managed = share
			break
		}
	}
	if len(shares) != 2 || managed.ID != id || managed.Drift != "name_changed" || managed.Mutable {
		t.Fatalf("marker-priority shares = %+v", shares)
	}
	if err := manager.Delete(context.Background(), id); ErrorCode(err) != ErrorDrift {
		t.Fatalf("Delete renamed share error = %v, code=%q", err, ErrorCode(err))
	}
	if adapter.deleteCall != 0 {
		t.Fatalf("drifted marker share triggered %d native deletes", adapter.deleteCall)
	}
}

func TestManagerOwnershipEvidenceAndInactiveDrift(t *testing.T) {
	root := t.TempDir()
	base := ShareSpec{
		Protocol: ProtocolNFS, Name: "data", Path: root, ReadOnly: true,
		Access: ShareAccess{Clients: []string{"192.0.2.0/24"}},
	}
	tests := []struct {
		name     string
		id       string
		observed observedShare
		want     string
		mutable  bool
	}{
		{
			name: "marker missing", id: "11111111-1111-4111-8111-111111111111",
			observed: observedShare{ShareSpec: base, MarkerSupported: true, Active: true},
			want:     "marker_missing",
		},
		{
			name: "ownership mismatch", id: "44444444-4444-4444-8444-444444444444",
			observed: observedShare{
				ShareSpec: base, MarkerID: "55555555-5555-4555-8555-555555555555",
				MarkerSupported: true, Active: true,
			},
			want: "ownership_mismatch",
		},
		{
			name: "markerless backend exact identity", id: "22222222-2222-4222-8222-222222222222",
			observed: observedShare{ShareSpec: base, MarkerSupported: false, Active: true},
			mutable:  true,
		},
		{
			name: "inactive marker", id: "33333333-3333-4333-8333-333333333333",
			observed: observedShare{
				ShareSpec: base, MarkerID: "33333333-3333-4333-8333-333333333333",
				MarkerSupported: true, Active: false,
			},
			want: "inactive",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := base
			spec.ID = test.id
			observed := test.observed
			observed.ID = ""
			if observed.MarkerID != "" {
				observed.Comment = nativeComment("", observed.MarkerID)
				observed.CommentObserved = true
			}
			adapter := &fakeAdapter{
				status: Status{Supported: true, Usable: true, NFS: ProtocolStatus{Readable: true, Writable: true}},
				shares: []observedShare{observed},
			}
			manager := testManager(t, root, adapter)
			if err := manager.ledger.put(context.Background(), spec, ""); err != nil {
				t.Fatalf("seed ledger: %v", err)
			}
			shares, err := manager.List(context.Background())
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(shares) != 1 || shares[0].Drift != test.want || shares[0].Mutable != test.mutable {
				t.Fatalf("reconciled share = %+v, want drift=%q mutable=%t", shares, test.want, test.mutable)
			}
		})
	}
}

func TestManagerShowsOrphanedNativeMarkerLocked(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-4111-8111-111111111111"
	adapter := &fakeAdapter{
		status: Status{Supported: true, Usable: true, NFS: ProtocolStatus{Readable: true, Writable: true}},
		shares: []observedShare{{
			ShareSpec: ShareSpec{
				Protocol: ProtocolNFS, Name: "orphan", Path: root,
				Comment: nativeComment("", id), ReadOnly: true,
				Access: ShareAccess{Clients: []string{"192.0.2.0/24"}},
			},
			MarkerID: id, MarkerSupported: true, Active: false, CommentObserved: true,
		}},
	}
	manager := testManager(t, root, adapter)
	shares, err := manager.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(shares) != 1 || shares[0].Managed || shares[0].Mutable ||
		shares[0].Active || shares[0].Drift != "orphaned_marker" {
		t.Fatalf("orphan marker share = %+v", shares)
	}
}

func TestManagerLocksManagedSambaAdminUsers(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-4111-8111-111111111111"
	spec := ShareSpec{
		ID: id, Protocol: ProtocolSMB, Name: "data", Path: root, ReadOnly: false,
		Access: ShareAccess{ACL: []ACLEntry{{Principal: "share-users", Level: "change"}}},
	}
	adapter := &fakeAdapter{
		status: Status{Supported: true, Usable: true, SMB: ProtocolStatus{Readable: true, Writable: true}},
		shares: []observedShare{{
			ShareSpec: spec, MarkerID: id, MarkerSupported: true, UnsafeAdminUsers: true,
			Active: true, CommentObserved: true,
		}},
	}
	manager := testManager(t, root, adapter)
	if err := manager.ledger.put(context.Background(), spec, ""); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
	shares, err := manager.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(shares) != 1 || shares[0].Drift != "unsafe_admin_users" || shares[0].Mutable {
		t.Fatalf("unsafe Samba share = %+v", shares)
	}
}

func TestManagerDeleteDetectsSilentNativeNoopWithoutDrift(t *testing.T) {
	root := t.TempDir()
	adapter := &fakeAdapter{
		status: Status{Supported: true, Usable: true, NFS: ProtocolStatus{Readable: true, Writable: true}},
	}
	manager := testManager(t, root, adapter)
	created, err := manager.Create(context.Background(), ShareSpec{
		Protocol: ProtocolNFS, Name: "data", Path: root, ReadOnly: true,
		Access: ShareAccess{Clients: []string{"192.0.2.0/24"}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	adapter.deleteDoesNotRemove = true
	if err := manager.Delete(context.Background(), created.ID); ErrorCode(err) != ErrorApplyFailed {
		t.Fatalf("Delete no-op error = %v, code=%q", err, ErrorCode(err))
	}
	record, err := manager.ledger.get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("ledger get: %v", err)
	}
	if record.Drift != "" {
		t.Fatalf("unchanged native state was incorrectly drifted: %q", record.Drift)
	}
}

func TestManagerValidationUsesOperationGatesAndDoesNotMutate(t *testing.T) {
	root := t.TempDir()
	adapter := &fakeAdapter{
		status: Status{Supported: true, Usable: true, NFS: ProtocolStatus{Readable: true, Writable: true}},
	}
	manager := testManager(t, root, adapter)
	manager.options.AllowCreate = false
	desired := ShareSpec{
		Protocol: ProtocolNFS, Name: "data", Path: root, ReadOnly: true,
		Access: ShareAccess{Clients: []string{"192.0.2.0/24"}},
	}
	if _, err := manager.ValidateCreate(context.Background(), desired); ErrorCode(err) != ErrorPermissionDenied {
		t.Fatalf("ValidateCreate permission error = %v, code=%q", err, ErrorCode(err))
	}
	if adapter.createCall != 0 || adapter.updateCall != 0 || adapter.deleteCall != 0 {
		t.Fatalf("validation mutated adapter: create=%d update=%d delete=%d",
			adapter.createCall, adapter.updateCall, adapter.deleteCall)
	}
}

func TestManagerValidateUpdateRequiresExactIdentityWithoutWriting(t *testing.T) {
	root := t.TempDir()
	adapter := &fakeAdapter{
		status: Status{Supported: true, Usable: true, SMB: ProtocolStatus{Readable: true, Writable: true}},
	}
	manager := testManager(t, root, adapter)
	created, err := manager.Create(context.Background(), ShareSpec{
		Protocol: ProtocolSMB, Name: "data", Path: root, ReadOnly: true,
		Access: ShareAccess{ACL: []ACLEntry{{Principal: "share-users", Level: "read"}}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	before, err := manager.ledger.get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("ledger get before validate: %v", err)
	}
	desired := created.ShareSpec
	desired.Comment = "validated only"
	validated, err := manager.ValidateUpdate(context.Background(), created.ID, desired)
	if err != nil {
		t.Fatalf("ValidateUpdate: %v", err)
	}
	if validated.Comment != desired.Comment || adapter.updateCall != 0 {
		t.Fatalf("validated update = %+v, native update calls=%d", validated, adapter.updateCall)
	}
	after, err := manager.ledger.get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("ledger get after validate: %v", err)
	}
	if after.Spec.Comment != before.Spec.Comment || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("ValidateUpdate changed ledger: before=%+v after=%+v", before, after)
	}
	desired.Name = "renamed"
	if _, err := manager.ValidateUpdate(context.Background(), created.ID, desired); ErrorCode(err) != ErrorInvalidArgument {
		t.Fatalf("identity-changing ValidateUpdate error = %v, code=%q", err, ErrorCode(err))
	}
}

func TestManagerDeleteLedgerFailureLocksUnverifiedRestore(t *testing.T) {
	root := t.TempDir()
	adapter := &fakeAdapter{
		status: Status{Supported: true, Usable: true, NFS: ProtocolStatus{Readable: true, Writable: true}},
	}
	manager := testManager(t, root, adapter)
	created, err := manager.Create(context.Background(), ShareSpec{
		Protocol: ProtocolNFS, Name: "data", Path: root, ReadOnly: true,
		Access: ShareAccess{Clients: []string{"192.0.2.0/24"}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := manager.ledger.db.Exec(`
		CREATE TRIGGER block_managed_share_delete
		BEFORE DELETE ON managed_shares
		BEGIN
			SELECT RAISE(ABORT, 'test delete failure');
		END;
	`); err != nil {
		t.Fatalf("create delete trigger: %v", err)
	}
	adapter.createDoesNotAdd = true
	if err := manager.Delete(context.Background(), created.ID); ErrorCode(err) != ErrorApplyFailed {
		t.Fatalf("Delete ledger failure error = %v, code=%q", err, ErrorCode(err))
	}
	record, err := manager.ledger.get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("ledger get: %v", err)
	}
	if record.Drift != "rollback_failed" {
		t.Fatalf("delete rollback drift = %q", record.Drift)
	}
}

func TestManagerReprobeEmitsStableReasonCode(t *testing.T) {
	adapter := &fakeAdapter{status: Status{
		Supported: true,
		Usable:    true,
		SMB:       ProtocolStatus{Readable: true, Writable: true},
	}}
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "network_shares.db"))
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	manager := &Manager{
		adapter: adapter,
		ledger:  ledger,
		logger:  slog.Default(),
		options: Options{Enabled: true, SMBEnabled: true},
	}
	t.Cleanup(func() { _ = manager.Close() })
	if status := manager.Reprobe(context.Background()); status.ReasonCode != "no_available_root" {
		t.Fatalf("reason code = %q, status=%+v", status.ReasonCode, status)
	}
}
