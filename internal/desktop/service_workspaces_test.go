package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

func TestWorkspaceLayoutDefaultsCRUDAndSessionMigration(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	state, err := svc.GetWorkspaceState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Layout.Version != workspaceLayoutVersion || state.Layout.Revision != 1 || state.Layout.NextID != 4 || len(state.Layout.Workspaces) != 3 {
		t.Fatalf("unexpected default layout: %+v", state.Layout)
	}
	for index, want := range []string{"1", "2", "3"} {
		if state.Layout.Workspaces[index].ID != want || state.Layout.Workspaces[index].Name != "" {
			t.Fatalf("default workspace %d = %+v", index, state.Layout.Workspaces[index])
		}
	}

	session := `{"version":2,"activeSpaceId":"2","activeWindowKey":"files-a","windows":[{"key":"files-a","appId":"files","spaceId":"2","left":123,"top":45,"context":{"path":"Documents/draft.txt"}}],"future_field":{"preserve":true}}`
	wallpapers := `{"2":"aurora","3":"midnight"}`
	state, err = svc.SetWorkspaceSettings(ctx, &state.Layout.Revision, map[string]string{
		workspaceSessionSetting:  session,
		workspaceWallpaperByID:   wallpapers,
		workspaceWallpaperGlobal: "groupshoot",
	}, SourceUser)
	if err != nil {
		t.Fatalf("save initial session/wallpapers: %v", err)
	}

	state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "add", Revision: state.Layout.Revision, Name: "Research"}, SourceUser)
	if err != nil || state.Layout.Workspaces[len(state.Layout.Workspaces)-1] != (Workspace{ID: "4", Name: "Research"}) {
		t.Fatalf("add workspace: layout=%+v err=%v", state.Layout, err)
	}
	state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "rename", Revision: state.Layout.Revision, ID: "4", Name: "Archive"}, SourceUser)
	if err != nil || state.Layout.Workspaces[3].Name != "Archive" {
		t.Fatalf("rename workspace: layout=%+v err=%v", state.Layout, err)
	}
	state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "reorder", Revision: state.Layout.Revision, Order: []string{"4", "3", "2", "1"}}, SourceUser)
	if err != nil || state.Layout.Workspaces[0].ID != "4" || state.Layout.Workspaces[3].ID != "1" {
		t.Fatalf("reorder workspace: layout=%+v err=%v", state.Layout, err)
	}
	state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "delete", Revision: state.Layout.Revision, ID: "2", MoveTo: "3"}, SourceUser)
	if err != nil {
		t.Fatalf("delete workspace: %v", err)
	}
	if state.Layout.Revision != 6 || state.Layout.RemovedTo["2"] != "3" || workspaceByID(state.Layout, "2") >= 0 {
		t.Fatalf("delete did not persist layout/redirect: %+v", state.Layout)
	}
	if got := state.Settings[workspaceWallpaperByID]; got != `{"3":"midnight"}` {
		t.Fatalf("delete changed an unrelated wallpaper assignment: %s", got)
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal([]byte(state.Settings[workspaceSessionSetting]), &saved); err != nil {
		t.Fatal(err)
	}
	var active string
	if err := json.Unmarshal(saved["activeSpaceId"], &active); err != nil || active != "3" {
		t.Fatalf("active space was not moved to explicit target: %q %v", active, err)
	}
	var windows []map[string]json.RawMessage
	if err := json.Unmarshal(saved["windows"], &windows); err != nil || len(windows) != 1 {
		t.Fatalf("saved windows were not preserved: %v %v", windows, err)
	}
	var windowSpace string
	if err := json.Unmarshal(windows[0]["spaceId"], &windowSpace); err != nil || windowSpace != "3" {
		t.Fatalf("window was not moved: %q %v", windowSpace, err)
	}
	if _, ok := saved["future_field"]; !ok || string(windows[0]["context"]) != `{"path":"Documents/draft.txt"}` {
		t.Fatalf("session fields were lost during migration: %s", state.Settings[workspaceSessionSetting])
	}

	state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "add", Revision: state.Layout.Revision, Name: "Next"}, SourceUser)
	if err != nil || state.Layout.Workspaces[len(state.Layout.Workspaces)-1].ID != "5" {
		t.Fatalf("removed ID was reused: layout=%+v err=%v", state.Layout, err)
	}
	state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "delete", Revision: state.Layout.Revision, ID: "3", MoveTo: "1"}, SourceUser)
	if err != nil || state.Layout.RemovedTo["2"] != "1" || state.Layout.RemovedTo["3"] != "1" {
		t.Fatalf("redirects were not transitively rewritten: layout=%+v err=%v", state.Layout, err)
	}
}

func TestWorkspaceLegacySessionMigrationAndInvalidIDs(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	state, err := svc.GetWorkspaceState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	legacy := `{"version":1,"windows":[{"appId":"files","left":88}],"opaque":"kept"}`
	state, err = svc.SetWorkspaceSettings(ctx, &state.Layout.Revision, map[string]string{workspaceSessionSetting: legacy}, SourceUser)
	if err != nil {
		t.Fatal(err)
	}
	state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "delete", Revision: state.Layout.Revision, ID: "1", MoveTo: "3"}, SourceUser)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Version       int    `json:"version"`
		ActiveSpaceID string `json:"activeSpaceId"`
		Windows       []struct {
			SpaceID string `json:"spaceId"`
		} `json:"windows"`
		Opaque string `json:"opaque"`
	}
	if err := json.Unmarshal([]byte(state.Settings[workspaceSessionSetting]), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Version != 2 || saved.ActiveSpaceID != "3" || len(saved.Windows) != 1 || saved.Windows[0].SpaceID != "3" || saved.Opaque != "kept" {
		t.Fatalf("legacy session migration lost/defaulted state: %+v", saved)
	}

	bad := `{"version":2,"activeSpaceId":"999","windows":[]}`
	_, err = svc.SetWorkspaceSettings(ctx, &state.Layout.Revision, map[string]string{workspaceSessionSetting: bad}, SourceUser)
	if !errors.Is(err, ErrWorkspaceInvalid) {
		t.Fatalf("unknown new workspace ID accepted: %v", err)
	}
}

func TestWorkspaceRevisionGuardsAndProtectedGenericWrites(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	state, err := svc.GetWorkspaceState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SetWorkspaceSettings(ctx, nil, map[string]string{workspaceSessionSetting: `{"version":2,"activeSpaceId":"1","windows":[]}`}, SourceUser)
	if !errors.Is(err, ErrWorkspaceRevisionRequired) {
		t.Fatalf("missing session revision accepted: %v", err)
	}
	if err := svc.SetSetting(ctx, workspaceWallpaperGlobal, "aurora", SourceUser); !errors.Is(err, ErrWorkspaceRevisionRequired) {
		t.Fatalf("generic protected SetSetting bypassed revision: %v", err)
	}
	if err := svc.SetSettings(ctx, map[string]string{workspaceWallpaperByID: `{"1":"aurora"}`}, SourceUser); !errors.Is(err, ErrWorkspaceRevisionRequired) {
		t.Fatalf("generic protected SetSettings bypassed revision: %v", err)
	}
	if err := svc.SetSetting(ctx, WorkspaceLayoutSettingKey, `{"version":1}`, SourceUser); !errors.Is(err, ErrWorkspaceInvalid) {
		t.Fatalf("generic layout write was allowed: %v", err)
	}

	state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "rename", Revision: state.Layout.Revision, ID: "1", Name: "Primary"}, SourceUser)
	if err != nil {
		t.Fatal(err)
	}
	conflictState, err := svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "rename", Revision: 1, ID: "1", Name: "Stale"}, SourceUser)
	if !errors.Is(err, ErrWorkspaceRevisionConflict) || conflictState.Layout.Revision != state.Layout.Revision || conflictState.Layout.Workspaces[0].Name != "Primary" {
		t.Fatalf("stale mutation did not return current state: state=%+v err=%v", conflictState, err)
	}
	if err := svc.SetSetting(ctx, "appearance.theme", "standard", SourceUser); err != nil {
		t.Fatalf("unrelated setting compatibility regressed: %v", err)
	}

	svc.SetReadOnly(true)
	_, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "rename", Revision: state.Layout.Revision, ID: "1", Name: "Blocked"}, SourceUser)
	if !errors.Is(err, ErrWorkspaceReadOnly) {
		t.Fatalf("read-only mutation was accepted: %v", err)
	}
}

func TestWorkspaceSettingsUnversionedValidationAndReadOnlyErrors(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	for key, value := range map[string]string{"sound.volume": "2", "desktop.unsupported": "true"} {
		if _, err := svc.SetWorkspaceSettings(ctx, nil, map[string]string{key: value}, SourceUser); !errors.Is(err, ErrWorkspaceInvalid) {
			t.Fatalf("invalid unversioned setting %s was not typed: %v", key, err)
		}
	}
	svc.SetReadOnly(true)
	if _, err := svc.SetWorkspaceSettings(ctx, nil, map[string]string{"appearance.theme": "fruity"}, SourceUser); !errors.Is(err, ErrWorkspaceReadOnly) {
		t.Fatalf("readonly unversioned setting was not typed: %v", err)
	}
}

func TestWorkspaceCountBoundariesAndNoIDReuse(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	state, err := svc.GetWorkspaceState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for len(state.Layout.Workspaces) < workspaceMaxCount {
		state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "add", Revision: state.Layout.Revision}, SourceUser)
		if err != nil {
			t.Fatal(err)
		}
	}
	before := state
	state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "add", Revision: state.Layout.Revision}, SourceUser)
	if !errors.Is(err, ErrWorkspaceLimit) || state.Layout.Revision != before.Layout.Revision || len(state.Layout.Workspaces) != workspaceMaxCount {
		t.Fatalf("ninth workspace did not fail atomically: state=%+v err=%v", state.Layout, err)
	}

	state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "delete", Revision: state.Layout.Revision, ID: "8", MoveTo: "1"}, SourceUser)
	if err != nil {
		t.Fatal(err)
	}
	state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "add", Revision: state.Layout.Revision}, SourceUser)
	if err != nil || state.Layout.Workspaces[len(state.Layout.Workspaces)-1].ID != "9" {
		t.Fatalf("deleted workspace ID was reused: layout=%+v err=%v", state.Layout, err)
	}

	state, err = svc.GetWorkspaceState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for len(state.Layout.Workspaces) > workspaceMinCount {
		id := state.Layout.Workspaces[len(state.Layout.Workspaces)-1].ID
		state, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "delete", Revision: state.Layout.Revision, ID: id, MoveTo: state.Layout.Workspaces[0].ID}, SourceUser)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "delete", Revision: state.Layout.Revision, ID: state.Layout.Workspaces[0].ID, MoveTo: state.Layout.Workspaces[0].ID}, SourceUser)
	if !errors.Is(err, ErrWorkspaceInvalid) {
		t.Fatalf("final workspace deletion accepted: %v", err)
	}
}

func TestWorkspaceConcurrentDeleteAndAutosave(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	state, err := svc.GetWorkspaceState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	initial := `{"version":2,"activeSpaceId":"2","windows":[{"key":"old","appId":"files","spaceId":"2"}]}`
	state, err = svc.SetWorkspaceSettings(ctx, &state.Layout.Revision, map[string]string{workspaceSessionSetting: initial}, SourceUser)
	if err != nil {
		t.Fatal(err)
	}
	revision := state.Layout.Revision
	autosave := `{"version":2,"activeSpaceId":"2","windows":[{"key":"draft","appId":"writer","spaceId":"2","context":{"path":"Documents/draft.txt"}}]}`
	start := make(chan struct{})
	type result struct {
		state WorkspaceState
		err   error
	}
	var wg sync.WaitGroup
	var saved, deleted result
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		saved.state, saved.err = svc.SetWorkspaceSettings(ctx, &revision, map[string]string{workspaceSessionSetting: autosave}, SourceUser)
	}()
	go func() {
		defer wg.Done()
		<-start
		deleted.state, deleted.err = svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "delete", Revision: revision, ID: "2", MoveTo: "3"}, SourceUser)
	}()
	close(start)
	wg.Wait()
	if saved.err == nil {
		if !errors.Is(deleted.err, ErrWorkspaceRevisionConflict) {
			t.Fatalf("delete should conflict after a protected autosave advanced revision: %v", deleted.err)
		}
	} else if errors.Is(saved.err, ErrWorkspaceRevisionConflict) {
		if deleted.err != nil {
			t.Fatalf("delete should succeed when autosave conflicts: %v", deleted.err)
		}
	} else {
		t.Fatalf("autosave returned unexpected error: %v", saved.err)
	}
	latest, err := svc.GetWorkspaceState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		ActiveSpaceID string `json:"activeSpaceId"`
		Windows       []struct {
			SpaceID string `json:"spaceId"`
		} `json:"windows"`
	}
	if err := json.Unmarshal([]byte(latest.Settings[workspaceSessionSetting]), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Windows) != 1 {
		t.Fatalf("concurrent delete/autosave lost saved windows: %+v", snapshot)
	}
	if workspaceByID(latest.Layout, "2") < 0 {
		if latest.Layout.RemovedTo["2"] != "3" || snapshot.ActiveSpaceID != "3" || snapshot.Windows[0].SpaceID != "3" {
			t.Fatalf("concurrent delete left stale assignments: layout=%+v session=%+v", latest.Layout, snapshot)
		}
	} else if snapshot.ActiveSpaceID != "2" || snapshot.Windows[0].SpaceID != "2" {
		t.Fatalf("concurrent autosave lost the current workspace assignment: %+v", snapshot)
	}
}

func TestWorkspaceConcurrentMutationsUseRevisionCAS(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	state, err := svc.GetWorkspaceState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	revision := state.Layout.Revision
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, name := range []string{"First", "Second"} {
		name := name
		go func() {
			<-start
			_, err := svc.MutateWorkspace(ctx, WorkspaceMutation{Operation: "rename", Revision: revision, ID: "1", Name: name}, SourceUser)
			results <- err
		}()
	}
	close(start)
	first, second := <-results, <-results
	if (first == nil && errors.Is(second, ErrWorkspaceRevisionConflict)) || (second == nil && errors.Is(first, ErrWorkspaceRevisionConflict)) {
		return
	}
	t.Fatalf("concurrent revision writes were not serialized: first=%v second=%v", first, second)
}

func TestWorkspaceRevisionMustFitJavaScriptSafeInteger(t *testing.T) {
	maxLayout := defaultWorkspaceLayout()
	maxLayout.Revision = workspaceMaxSafeRevision
	if _, err := decodeWorkspaceLayout(mustWorkspaceLayoutJSON(t, maxLayout)); err != nil {
		t.Fatalf("largest safe revision rejected: %v", err)
	}
	unsafe := maxLayout
	unsafe.Revision++
	if _, err := decodeWorkspaceLayout(mustWorkspaceLayoutJSON(t, unsafe)); !errors.Is(err, ErrWorkspaceStateInvalid) {
		t.Fatalf("unsafe revision accepted: %v", err)
	}
	if _, _, err := applyWorkspaceMutation(maxLayout, map[string]string{}, WorkspaceMutation{Operation: "rename", ID: "1", Name: "Near limit"}); !errors.Is(err, ErrWorkspaceLimit) {
		t.Fatalf("mutation beyond safe revision was accepted: %v", err)
	}
}

func TestWorkspaceReadRejectsPersistedUnknownSessionIDs(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	if _, err := svc.getDB().ExecContext(ctx, `INSERT INTO desktop_settings(key, value, updated_at) VALUES(?, ?, 'now')`, workspaceSessionSetting,
		`{"version":2,"activeSpaceId":"999","windows":[{"key":"draft","appId":"writer","spaceId":"999"}]}`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetWorkspaceState(ctx); !errors.Is(err, ErrWorkspaceStateInvalid) {
		t.Fatalf("workspace snapshot normalized an unknown persisted ID: %v", err)
	}
}

func mustWorkspaceLayoutJSON(t *testing.T, layout WorkspaceLayout) string {
	t.Helper()
	encoded, err := json.Marshal(layout)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
