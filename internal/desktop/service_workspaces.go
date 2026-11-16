package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	WorkspaceLayoutSettingKey = "desktop.workspaces.layout"
	workspaceLayoutVersion    = 1
	workspaceLayoutMaxBytes   = 64 * 1024
	workspaceMinCount         = 1
	workspaceMaxCount         = 8
	workspaceMaxNameRunes     = 64
	workspaceMaxSafeID        = uint64(9007199254740991)
	workspaceMaxSafeRevision  = int64(9007199254740991)

	workspaceSessionSetting  = "session.windows"
	workspaceWallpaperByID   = "appearance.wallpaper_by_space"
	workspaceWallpaperGlobal = "appearance.wallpaper"
)

var (
	ErrWorkspaceRevisionConflict = errors.New("workspace_revision_conflict")
	ErrWorkspaceRevisionRequired = errors.New("workspace_revision_required")
	ErrWorkspaceInvalid          = errors.New("invalid_workspace")
	ErrWorkspaceLimit            = errors.New("workspace_limit")
	ErrWorkspaceNotFound         = errors.New("workspace_not_found")
	ErrWorkspaceReadOnly         = errors.New("desktop_readonly")
	ErrWorkspaceStateInvalid     = errors.New("workspace_state_invalid")
)

// Workspace is one stable workspace identity and its user-defined label.
// An empty name asks the browser to use its localized default label.
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// WorkspaceLayout is stored in the existing desktop_settings table. RemovedTo
// is cumulative so a stale browser can move unsaved windows to the same target
// chosen by the tab that removed a workspace.
type WorkspaceLayout struct {
	Version    int               `json:"version"`
	Revision   int64             `json:"revision"`
	NextID     uint64            `json:"next_id"`
	Workspaces []Workspace       `json:"workspaces"`
	RemovedTo  map[string]string `json:"removed_to"`
}

// WorkspaceState is a consistent layout and protected-settings snapshot.
type WorkspaceState struct {
	Layout   WorkspaceLayout   `json:"layout"`
	Settings map[string]string `json:"settings"`
}

// WorkspaceMutation is one revision-bound workspace command.
type WorkspaceMutation struct {
	Operation string   `json:"operation"`
	Revision  int64    `json:"revision"`
	ID        string   `json:"id,omitempty"`
	Name      string   `json:"name,omitempty"`
	Order     []string `json:"order,omitempty"`
	MoveTo    string   `json:"move_to,omitempty"`
}

type workspaceReadState struct {
	public WorkspaceState
	values map[string]string
}

func defaultWorkspaceLayout() WorkspaceLayout {
	return WorkspaceLayout{
		Version:  workspaceLayoutVersion,
		Revision: 1,
		NextID:   4,
		Workspaces: []Workspace{
			{ID: "1"},
			{ID: "2"},
			{ID: "3"},
		},
		RemovedTo: map[string]string{},
	}
}

// DefaultWorkspaceLayout returns the initial stable workspace list.
func DefaultWorkspaceLayout() WorkspaceLayout { return defaultWorkspaceLayout() }

// DefaultWorkspaceLayoutValue returns the versioned default JSON stored in
// desktop_settings and registered as the protected setting default.
func DefaultWorkspaceLayoutValue() string {
	value, _ := json.Marshal(defaultWorkspaceLayout())
	return string(value)
}

func workspaceRevisionSensitiveSetting(key string) bool {
	switch key {
	case workspaceSessionSetting, workspaceWallpaperByID, workspaceWallpaperGlobal:
		return true
	default:
		return false
	}
}

func rejectUnversionedWorkspaceSettings(values map[string]string) error {
	for key := range values {
		if key == WorkspaceLayoutSettingKey {
			return fmt.Errorf("%w: layout is service-managed", ErrWorkspaceInvalid)
		}
		if workspaceRevisionSensitiveSetting(key) {
			return ErrWorkspaceRevisionRequired
		}
	}
	return nil
}

func canonicalWorkspaceID(value string) (uint64, bool) {
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 53)
	if err != nil || parsed == 0 || parsed > workspaceMaxSafeID || strconv.FormatUint(parsed, 10) != value {
		return 0, false
	}
	return parsed, true
}

func validateWorkspaceName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) > workspaceMaxNameRunes {
		return "", ErrWorkspaceInvalid
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", ErrWorkspaceInvalid
		}
	}
	return name, nil
}

func decodeWorkspaceLayout(raw string) (WorkspaceLayout, error) {
	if raw == "" {
		return defaultWorkspaceLayout(), nil
	}
	if len(raw) > workspaceLayoutMaxBytes {
		return WorkspaceLayout{}, ErrWorkspaceStateInvalid
	}
	var layout WorkspaceLayout
	if err := json.Unmarshal([]byte(raw), &layout); err != nil {
		return WorkspaceLayout{}, fmt.Errorf("%w: invalid layout JSON", ErrWorkspaceStateInvalid)
	}
	if layout.Version != workspaceLayoutVersion || layout.Revision < 1 || layout.Revision > workspaceMaxSafeRevision || layout.NextID == 0 || layout.NextID > workspaceMaxSafeID || len(layout.Workspaces) < workspaceMinCount || len(layout.Workspaces) > workspaceMaxCount {
		return WorkspaceLayout{}, ErrWorkspaceStateInvalid
	}
	if layout.RemovedTo == nil {
		layout.RemovedTo = map[string]string{}
	}
	seen := make(map[string]struct{}, len(layout.Workspaces)+len(layout.RemovedTo))
	active := make(map[string]struct{}, len(layout.Workspaces))
	maxID := uint64(0)
	for i := range layout.Workspaces {
		workspace := &layout.Workspaces[i]
		parsed, ok := canonicalWorkspaceID(workspace.ID)
		if !ok {
			return WorkspaceLayout{}, ErrWorkspaceStateInvalid
		}
		if _, duplicate := seen[workspace.ID]; duplicate {
			return WorkspaceLayout{}, ErrWorkspaceStateInvalid
		}
		seen[workspace.ID] = struct{}{}
		active[workspace.ID] = struct{}{}
		if parsed > maxID {
			maxID = parsed
		}
		name, err := validateWorkspaceName(workspace.Name)
		if err != nil || name != workspace.Name {
			return WorkspaceLayout{}, ErrWorkspaceStateInvalid
		}
	}
	for removed, target := range layout.RemovedTo {
		parsed, ok := canonicalWorkspaceID(removed)
		if !ok {
			return WorkspaceLayout{}, ErrWorkspaceStateInvalid
		}
		if parsed > maxID {
			maxID = parsed
		}
		if _, duplicate := seen[removed]; duplicate {
			return WorkspaceLayout{}, ErrWorkspaceStateInvalid
		}
		if _, ok := canonicalWorkspaceID(target); !ok {
			return WorkspaceLayout{}, ErrWorkspaceStateInvalid
		}
		if _, ok := active[target]; !ok {
			return WorkspaceLayout{}, ErrWorkspaceStateInvalid
		}
		seen[removed] = struct{}{}
	}
	if layout.NextID <= maxID {
		return WorkspaceLayout{}, ErrWorkspaceStateInvalid
	}
	return layout, nil
}

func encodeWorkspaceLayout(layout WorkspaceLayout) (string, error) {
	encoded, err := json.Marshal(layout)
	if err != nil {
		return "", fmt.Errorf("encode workspace layout: %w", err)
	}
	if len(encoded) > workspaceLayoutMaxBytes {
		return "", ErrWorkspaceLimit
	}
	return string(encoded), nil
}

func workspaceByID(layout WorkspaceLayout, id string) int {
	for index, item := range layout.Workspaces {
		if item.ID == id {
			return index
		}
	}
	return -1
}

func resolveWorkspaceID(layout WorkspaceLayout, id string) (string, bool) {
	if workspaceByID(layout, id) >= 0 {
		return id, true
	}
	resolved, ok := layout.RemovedTo[id]
	if !ok || workspaceByID(layout, resolved) < 0 {
		return "", false
	}
	return resolved, true
}

func applyWorkspaceMutation(layout WorkspaceLayout, currentSettings map[string]string, mutation WorkspaceMutation) (WorkspaceLayout, map[string]string, error) {
	next := layout
	next.Workspaces = append([]Workspace(nil), layout.Workspaces...)
	next.RemovedTo = make(map[string]string, len(layout.RemovedTo)+1)
	for id, target := range layout.RemovedTo {
		next.RemovedTo[id] = target
	}
	settings := map[string]string{}

	switch mutation.Operation {
	case "add":
		if len(next.Workspaces) >= workspaceMaxCount {
			return layout, nil, ErrWorkspaceLimit
		}
		name, err := validateWorkspaceName(mutation.Name)
		if err != nil {
			return layout, nil, err
		}
		if next.NextID >= workspaceMaxSafeID {
			return layout, nil, ErrWorkspaceLimit
		}
		next.Workspaces = append(next.Workspaces, Workspace{ID: strconv.FormatUint(next.NextID, 10), Name: name})
		next.NextID++
	case "rename":
		index := workspaceByID(next, mutation.ID)
		if index < 0 {
			return layout, nil, ErrWorkspaceNotFound
		}
		name, err := validateWorkspaceName(mutation.Name)
		if err != nil {
			return layout, nil, err
		}
		next.Workspaces[index].Name = name
	case "reorder":
		if len(mutation.Order) != len(next.Workspaces) {
			return layout, nil, ErrWorkspaceInvalid
		}
		ordered := make([]Workspace, 0, len(next.Workspaces))
		seen := make(map[string]struct{}, len(mutation.Order))
		for _, id := range mutation.Order {
			if _, duplicate := seen[id]; duplicate {
				return layout, nil, ErrWorkspaceInvalid
			}
			index := workspaceByID(next, id)
			if index < 0 {
				return layout, nil, ErrWorkspaceInvalid
			}
			seen[id] = struct{}{}
			ordered = append(ordered, next.Workspaces[index])
		}
		next.Workspaces = ordered
	case "delete":
		index := workspaceByID(next, mutation.ID)
		if index < 0 {
			return layout, nil, ErrWorkspaceNotFound
		}
		if len(next.Workspaces) <= workspaceMinCount {
			return layout, nil, ErrWorkspaceInvalid
		}
		if mutation.MoveTo == mutation.ID || workspaceByID(next, mutation.MoveTo) < 0 {
			return layout, nil, ErrWorkspaceInvalid
		}
		for removed, target := range next.RemovedTo {
			if target == mutation.ID {
				next.RemovedTo[removed] = mutation.MoveTo
			}
		}
		next.RemovedTo[mutation.ID] = mutation.MoveTo
		next.Workspaces = append(next.Workspaces[:index], next.Workspaces[index+1:]...)

		migrated, err := migrateWorkspaceSession(layout, mutation.ID, mutation.MoveTo, currentSettings[workspaceSessionSetting])
		if err != nil {
			return layout, nil, err
		}
		if migrated != "" {
			settings[workspaceSessionSetting] = migrated
		}
		wallpapers, err := removeWorkspaceWallpaper(layout, mutation.ID, currentSettings[workspaceWallpaperByID])
		if err != nil {
			return layout, nil, err
		}
		if wallpapers != "" {
			settings[workspaceWallpaperByID] = wallpapers
		}
	default:
		return layout, nil, ErrWorkspaceInvalid
	}
	if layout.Revision >= workspaceMaxSafeRevision {
		return layout, nil, ErrWorkspaceLimit
	}
	next.Revision++
	return next, settings, nil
}

func migrateWorkspaceSession(oldLayout WorkspaceLayout, removedID, targetID, raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if len(raw) > 65536 {
		return "", fmt.Errorf("%w: saved session exceeds limit", ErrWorkspaceStateInvalid)
	}
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil || snapshot == nil {
		return "", fmt.Errorf("%w: invalid saved session", ErrWorkspaceStateInvalid)
	}
	var version int
	if data := snapshot["version"]; len(data) > 0 {
		if err := json.Unmarshal(data, &version); err != nil || version < 1 {
			return "", fmt.Errorf("%w: invalid saved session version", ErrWorkspaceStateInvalid)
		}
	} else {
		version = 1
	}
	activeID := "1"
	activeOriginal := activeID
	if data := snapshot["activeSpaceId"]; len(data) > 0 {
		if err := json.Unmarshal(data, &activeID); err != nil || activeID == "" {
			return "", fmt.Errorf("%w: invalid active workspace", ErrWorkspaceStateInvalid)
		}
	}
	activeOriginal = activeID
	activeID, ok := resolveWorkspaceID(oldLayout, activeID)
	if !ok {
		return "", fmt.Errorf("%w: unknown active workspace", ErrWorkspaceStateInvalid)
	}
	if activeID == removedID {
		activeID = targetID
	}
	var windows []json.RawMessage
	if data := snapshot["windows"]; len(data) > 0 {
		if err := json.Unmarshal(data, &windows); err != nil || windows == nil {
			return "", fmt.Errorf("%w: invalid saved windows", ErrWorkspaceStateInvalid)
		}
	} else {
		return "", fmt.Errorf("%w: missing saved windows", ErrWorkspaceStateInvalid)
	}
	changed := false
	windowsChanged := false
	for index, encoded := range windows {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &item); err != nil || item == nil {
			return "", fmt.Errorf("%w: invalid saved window", ErrWorkspaceStateInvalid)
		}
		spaceID := "1"
		if data := item["spaceId"]; len(data) > 0 {
			if err := json.Unmarshal(data, &spaceID); err != nil || spaceID == "" {
				return "", fmt.Errorf("%w: invalid saved window workspace", ErrWorkspaceStateInvalid)
			}
		}
		resolved, ok := resolveWorkspaceID(oldLayout, spaceID)
		if !ok {
			return "", fmt.Errorf("%w: unknown saved window workspace", ErrWorkspaceStateInvalid)
		}
		if resolved == removedID {
			resolved = targetID
		}
		if resolved != spaceID || (version < 2 && removedID == "1") {
			item["spaceId"], _ = json.Marshal(resolved)
			windows[index], _ = json.Marshal(item)
			changed = true
			windowsChanged = true
		}
	}
	if windowsChanged {
		snapshot["windows"], _ = json.Marshal(windows)
	}
	if activeID != activeOriginal || (version < 2 && removedID == "1") {
		encoded, _ := json.Marshal(activeID)
		snapshot["activeSpaceId"] = encoded
		changed = true
	}
	if version < 2 && removedID == "1" {
		version = 2
		encoded, _ := json.Marshal(version)
		snapshot["version"] = encoded
		snapshot["activeSpaceId"], _ = json.Marshal(activeID)
		snapshot["windows"], _ = json.Marshal(windows)
		changed = true
	}
	if changed {
		encoded, err := json.Marshal(snapshot)
		if err != nil || len(encoded) > 65536 {
			return "", fmt.Errorf("%w: saved session exceeds limit", ErrWorkspaceStateInvalid)
		}
		return string(encoded), nil
	}
	return raw, nil
}

func removeWorkspaceWallpaper(layout WorkspaceLayout, removedID, raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	wallpapers, err := parseWorkspaceWallpaperMap(raw)
	if err != nil {
		return "", err
	}
	if _, ok := wallpapers[removedID]; !ok {
		return raw, nil
	}
	delete(wallpapers, removedID)
	encoded, err := json.Marshal(wallpapers)
	if err != nil {
		return "", err
	}
	if len(encoded) > 4096 {
		return "", ErrWorkspaceLimit
	}
	return string(encoded), nil
}

func parseWorkspaceWallpaperMap(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}
	if len(raw) > 4096 {
		return nil, fmt.Errorf("invalid %s value", workspaceWallpaperByID)
	}
	var wallpapers map[string]string
	if err := json.Unmarshal([]byte(raw), &wallpapers); err != nil || wallpapers == nil {
		return nil, fmt.Errorf("invalid %s value", workspaceWallpaperByID)
	}
	return wallpapers, nil
}

func workspaceLayoutHasID(layout WorkspaceLayout, id string) bool {
	return workspaceByID(layout, id) >= 0
}

func (s *Service) validateWallpaperSettings(ctx context.Context, values, currentSettings map[string]string) error {
	layout, err := decodeWorkspaceLayout(currentSettings[WorkspaceLayoutSettingKey])
	if err != nil {
		return err
	}
	if value, changed := values[workspaceWallpaperGlobal]; changed {
		if err := s.validateWorkspaceWallpaperValue(ctx, value, currentSettings[workspaceWallpaperGlobal]); err != nil {
			return err
		}
	}
	value, changed := values[workspaceWallpaperByID]
	if !changed {
		return nil
	}
	wallpapers, err := parseWorkspaceWallpaperMap(value)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrWorkspaceInvalid, err)
	}
	oldWallpapers, _ := parseWorkspaceWallpaperMap(currentSettings[workspaceWallpaperByID])
	for id, wallpaper := range wallpapers {
		if !workspaceLayoutHasID(layout, id) {
			return fmt.Errorf("%w: wallpaper references an unknown workspace", ErrWorkspaceInvalid)
		}
		if err := s.validateWorkspaceWallpaperValue(ctx, wallpaper, oldWallpapers[id]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) validateWorkspaceWallpaperValue(ctx context.Context, value, previous string) error {
	if _, ok := desktopWallpaperValueSet()[value]; ok {
		return nil
	}
	if !strings.HasPrefix(value, "file:") || len(value) <= len("file:") {
		return fmt.Errorf("%w: invalid desktop wallpaper value", ErrWorkspaceInvalid)
	}
	if value == previous {
		return nil
	}
	if err := s.ValidateWallpaperValue(ctx, value); err != nil {
		return err
	}
	return nil
}

func validateSessionWorkspaceAssignments(raw string, layout WorkspaceLayout) error {
	if raw == "" {
		return nil
	}
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil || snapshot == nil {
		return fmt.Errorf("%w: invalid saved session", ErrWorkspaceInvalid)
	}
	version := 1
	if data := snapshot["version"]; len(data) > 0 {
		if err := json.Unmarshal(data, &version); err != nil || version < 1 {
			return fmt.Errorf("%w: invalid saved session version", ErrWorkspaceInvalid)
		}
	}
	if data := snapshot["activeSpaceId"]; len(data) > 0 {
		var activeID string
		if err := json.Unmarshal(data, &activeID); err != nil || activeID == "" || !workspaceLayoutHasID(layout, activeID) {
			return fmt.Errorf("%w: unknown active workspace", ErrWorkspaceInvalid)
		}
	} else if !workspaceLayoutHasID(layout, "1") {
		return fmt.Errorf("%w: saved session has no active workspace", ErrWorkspaceInvalid)
	}
	var windows []json.RawMessage
	if data := snapshot["windows"]; len(data) == 0 || json.Unmarshal(data, &windows) != nil || windows == nil {
		return fmt.Errorf("%w: invalid saved windows", ErrWorkspaceInvalid)
	}
	for _, encoded := range windows {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &item); err != nil || item == nil {
			return fmt.Errorf("%w: invalid saved window", ErrWorkspaceInvalid)
		}
		spaceID := "1"
		if data := item["spaceId"]; len(data) > 0 {
			if err := json.Unmarshal(data, &spaceID); err != nil || spaceID == "" {
				return fmt.Errorf("%w: invalid saved window workspace", ErrWorkspaceInvalid)
			}
		}
		if !workspaceLayoutHasID(layout, spaceID) {
			return fmt.Errorf("%w: unknown saved window workspace", ErrWorkspaceInvalid)
		}
	}
	_ = version
	return nil
}

func (s *Service) GetWorkspaceState(ctx context.Context) (WorkspaceState, error) {
	if err := s.ensureReady(ctx); err != nil {
		return WorkspaceState{}, err
	}
	desktopMutationMu.RLock()
	defer desktopMutationMu.RUnlock()
	if err := s.ensureReady(ctx); err != nil {
		return WorkspaceState{}, err
	}
	read, err := readWorkspaceState(ctx, s.getDB())
	if err != nil {
		return WorkspaceState{}, err
	}
	return read.public, nil
}

func readWorkspaceState(ctx context.Context, db *sql.DB) (workspaceReadState, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return workspaceReadState{}, fmt.Errorf("read workspace state: %w", err)
	}
	defer tx.Rollback()
	state, err := readWorkspaceStateTx(ctx, tx)
	if err != nil {
		return workspaceReadState{}, err
	}
	if err := tx.Commit(); err != nil {
		return workspaceReadState{}, fmt.Errorf("finish workspace state read: %w", err)
	}
	return state, nil
}

func readWorkspaceStateTx(ctx context.Context, tx *sql.Tx) (workspaceReadState, error) {
	defaults := DesktopSettingDefaults()
	values := map[string]string{
		workspaceSessionSetting:  defaults[workspaceSessionSetting],
		workspaceWallpaperByID:   defaults[workspaceWallpaperByID],
		workspaceWallpaperGlobal: defaults[workspaceWallpaperGlobal],
	}
	layoutRaw := ""
	rows, err := tx.QueryContext(ctx, `SELECT key, value FROM desktop_settings WHERE key IN (?, ?, ?, ?)`,
		WorkspaceLayoutSettingKey, workspaceSessionSetting, workspaceWallpaperByID, workspaceWallpaperGlobal)
	if err != nil {
		return workspaceReadState{}, fmt.Errorf("read workspace settings: %w", err)
	}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			_ = rows.Close()
			return workspaceReadState{}, fmt.Errorf("scan workspace settings: %w", err)
		}
		if key == WorkspaceLayoutSettingKey {
			layoutRaw = value
		} else {
			values[key] = value
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return workspaceReadState{}, fmt.Errorf("read workspace settings: %w", err)
	}
	if err := rows.Close(); err != nil {
		return workspaceReadState{}, fmt.Errorf("close workspace settings: %w", err)
	}
	layout, err := decodeWorkspaceLayout(layoutRaw)
	if err != nil {
		return workspaceReadState{}, err
	}
	values[WorkspaceLayoutSettingKey] = layoutRaw
	if err := validateStoredWorkspaceWallpapers(layout, values[workspaceWallpaperByID], values[workspaceWallpaperGlobal]); err != nil {
		return workspaceReadState{}, err
	}
	if err := validateSessionWorkspaceAssignments(values[workspaceSessionSetting], layout); err != nil {
		return workspaceReadState{}, fmt.Errorf("%w: invalid saved session assignments", ErrWorkspaceStateInvalid)
	}
	return workspaceReadState{
		public: WorkspaceState{
			Layout: layout,
			Settings: map[string]string{
				workspaceSessionSetting:  values[workspaceSessionSetting],
				workspaceWallpaperByID:   values[workspaceWallpaperByID],
				workspaceWallpaperGlobal: values[workspaceWallpaperGlobal],
			},
		},
		values: values,
	}, nil
}

func validateStoredWorkspaceWallpapers(layout WorkspaceLayout, rawMap, global string) error {
	wallpapers, err := parseWorkspaceWallpaperMap(rawMap)
	if err != nil {
		return fmt.Errorf("%w: invalid saved wallpaper map", ErrWorkspaceStateInvalid)
	}
	allowed := desktopWallpaperValueSet()
	for id, wallpaper := range wallpapers {
		if !workspaceLayoutHasID(layout, id) {
			return fmt.Errorf("%w: wallpaper references an unknown workspace", ErrWorkspaceStateInvalid)
		}
		if _, ok := allowed[wallpaper]; !ok && !isCustomWorkspaceWallpaper(wallpaper) {
			return fmt.Errorf("%w: invalid saved wallpaper", ErrWorkspaceStateInvalid)
		}
	}
	if _, ok := allowed[global]; !ok && !isCustomWorkspaceWallpaper(global) {
		return fmt.Errorf("%w: invalid global wallpaper", ErrWorkspaceStateInvalid)
	}
	return nil
}

func (s *Service) MutateWorkspace(ctx context.Context, mutation WorkspaceMutation, source string) (WorkspaceState, error) {
	if err := s.ensureReady(ctx); err != nil {
		return WorkspaceState{}, err
	}
	if s.Config().ReadOnly {
		return WorkspaceState{}, ErrWorkspaceReadOnly
	}
	desktopMutationMu.Lock()
	locked := true
	defer func() {
		if locked {
			desktopMutationMu.Unlock()
		}
	}()
	if err := s.ensureReady(ctx); err != nil {
		return WorkspaceState{}, err
	}
	if s.Config().ReadOnly {
		return WorkspaceState{}, ErrWorkspaceReadOnly
	}
	currentRead, err := readWorkspaceState(ctx, s.getDB())
	if err != nil {
		return WorkspaceState{}, err
	}
	if mutation.Revision < 1 {
		return currentRead.public, ErrWorkspaceRevisionRequired
	}
	if mutation.Revision != currentRead.public.Layout.Revision {
		return currentRead.public, ErrWorkspaceRevisionConflict
	}
	next, changes, err := applyWorkspaceMutation(currentRead.public.Layout, currentRead.values, mutation)
	if err != nil {
		return currentRead.public, err
	}
	nextRaw, err := encodeWorkspaceLayout(next)
	if err != nil {
		return currentRead.public, err
	}
	validationSettings := make(map[string]string, len(currentRead.values)+1)
	for key, value := range currentRead.values {
		validationSettings[key] = value
	}
	validationSettings[WorkspaceLayoutSettingKey] = nextRaw
	if wallpaper, ok := changes[workspaceWallpaperByID]; ok {
		if err := s.validateWallpaperSettings(ctx, map[string]string{workspaceWallpaperByID: wallpaper}, validationSettings); err != nil {
			return currentRead.public, err
		}
	}
	if session, ok := changes[workspaceSessionSetting]; ok {
		if err := validateSessionWorkspaceAssignments(session, next); err != nil {
			return currentRead.public, err
		}
	}
	changes[WorkspaceLayoutSettingKey] = nextRaw

	tx, err := s.getDB().BeginTx(ctx, nil)
	if err != nil {
		return currentRead.public, fmt.Errorf("begin workspace update: %w", err)
	}
	defer tx.Rollback()
	latest, err := readWorkspaceStateTx(ctx, tx)
	if err != nil {
		return WorkspaceState{}, err
	}
	if mutation.Revision != latest.public.Layout.Revision {
		return latest.public, ErrWorkspaceRevisionConflict
	}
	if err := upsertWorkspaceSettingsTx(ctx, tx, changes); err != nil {
		return latest.public, err
	}
	if err := tx.Commit(); err != nil {
		return latest.public, fmt.Errorf("commit workspace update: %w", err)
	}
	updated := latest.public
	updated.Layout = next
	for key, value := range changes {
		if key != WorkspaceLayoutSettingKey {
			updated.Settings[key] = value
		}
	}
	s.InvalidateSettings()
	desktopMutationMu.Unlock()
	locked = false
	_ = s.Audit(ctx, "workspace_"+mutation.Operation, mutation.ID, map[string]interface{}{"revision": next.Revision}, source)
	return updated, nil
}

func (s *Service) SetWorkspaceSettings(ctx context.Context, expectedRevision *int64, values map[string]string, source string) (WorkspaceState, error) {
	normalized := make(map[string]string, len(values))
	for rawKey, rawValue := range values {
		key, value := strings.TrimSpace(rawKey), strings.TrimSpace(rawValue)
		if key == "" {
			return WorkspaceState{}, ErrWorkspaceInvalid
		}
		if _, duplicate := normalized[key]; duplicate {
			return WorkspaceState{}, fmt.Errorf("%w: duplicate desktop setting %s", ErrWorkspaceInvalid, key)
		}
		normalized[key] = value
	}
	if err := rejectLayoutWrite(normalized); err != nil {
		return WorkspaceState{}, err
	}
	sensitive := false
	for key := range normalized {
		sensitive = sensitive || workspaceRevisionSensitiveSetting(key)
	}
	if expectedRevision == nil && sensitive {
		state, err := s.GetWorkspaceState(ctx)
		if err != nil {
			return WorkspaceState{}, err
		}
		return state, ErrWorkspaceRevisionRequired
	}
	if expectedRevision == nil {
		if s.Config().ReadOnly {
			state, err := s.GetWorkspaceState(ctx)
			if err != nil {
				return WorkspaceState{}, err
			}
			return state, ErrWorkspaceReadOnly
		}
		for key, value := range normalized {
			if err := validateDesktopSetting(key, value); err != nil {
				state, stateErr := s.GetWorkspaceState(ctx)
				if stateErr != nil {
					return WorkspaceState{}, stateErr
				}
				return state, fmt.Errorf("%w: %v", ErrWorkspaceInvalid, err)
			}
		}
		if err := s.SetSettings(ctx, normalized, source); err != nil {
			state, stateErr := s.GetWorkspaceState(ctx)
			if stateErr != nil {
				return WorkspaceState{}, stateErr
			}
			return state, err
		}
		return s.GetWorkspaceState(ctx)
	}
	if *expectedRevision < 1 {
		state, err := s.GetWorkspaceState(ctx)
		if err != nil {
			return WorkspaceState{}, err
		}
		return state, ErrWorkspaceRevisionRequired
	}
	if len(normalized) == 0 {
		state, err := s.GetWorkspaceState(ctx)
		if err != nil {
			return WorkspaceState{}, err
		}
		if state.Layout.Revision != *expectedRevision {
			return state, ErrWorkspaceRevisionConflict
		}
		return state, nil
	}
	if err := s.ensureReady(ctx); err != nil {
		return WorkspaceState{}, err
	}
	if s.Config().ReadOnly {
		return WorkspaceState{}, ErrWorkspaceReadOnly
	}
	desktopMutationMu.Lock()
	locked := true
	defer func() {
		if locked {
			desktopMutationMu.Unlock()
		}
	}()
	if err := s.ensureReady(ctx); err != nil {
		return WorkspaceState{}, err
	}
	if s.Config().ReadOnly {
		return WorkspaceState{}, ErrWorkspaceReadOnly
	}
	currentRead, err := readWorkspaceState(ctx, s.getDB())
	if err != nil {
		return WorkspaceState{}, err
	}
	if currentRead.public.Layout.Revision != *expectedRevision {
		return currentRead.public, ErrWorkspaceRevisionConflict
	}
	for key, value := range normalized {
		if key == workspaceSessionSetting {
			if len(value) > 65536 {
				return currentRead.public, fmt.Errorf("%w: invalid desktop setting value for %s", ErrWorkspaceInvalid, key)
			}
		} else if key != workspaceWallpaperByID && key != workspaceWallpaperGlobal {
			if err := validateDesktopSetting(key, value); err != nil {
				return currentRead.public, fmt.Errorf("%w: %v", ErrWorkspaceInvalid, err)
			}
		}
		if key == workspaceSessionSetting {
			if err := validateSessionWorkspaceAssignments(value, currentRead.public.Layout); err != nil {
				return currentRead.public, err
			}
		}
	}
	if err := s.validateDefaultAppSetting(ctx, normalized); err != nil {
		return currentRead.public, fmt.Errorf("%w: %v", ErrWorkspaceInvalid, err)
	}
	if err := s.validateWallpaperSettings(ctx, normalized, currentRead.values); err != nil {
		return currentRead.public, err
	}
	nextLayout := currentRead.public.Layout
	if sensitive {
		if nextLayout.Revision >= workspaceMaxSafeRevision {
			return currentRead.public, ErrWorkspaceLimit
		}
		nextLayout.Revision++
		layoutValue, err := encodeWorkspaceLayout(nextLayout)
		if err != nil {
			return currentRead.public, err
		}
		normalized[WorkspaceLayoutSettingKey] = layoutValue
	}

	tx, err := s.getDB().BeginTx(ctx, nil)
	if err != nil {
		return currentRead.public, fmt.Errorf("begin revision-bound settings update: %w", err)
	}
	defer tx.Rollback()
	latest, err := readWorkspaceStateTx(ctx, tx)
	if err != nil {
		return WorkspaceState{}, err
	}
	if latest.public.Layout.Revision != *expectedRevision {
		return latest.public, ErrWorkspaceRevisionConflict
	}
	if err := upsertWorkspaceSettingsTx(ctx, tx, normalized); err != nil {
		return latest.public, err
	}
	if err := tx.Commit(); err != nil {
		return latest.public, fmt.Errorf("commit revision-bound settings: %w", err)
	}
	updated := latest.public
	updated.Layout = nextLayout
	for key, value := range normalized {
		if key == workspaceSessionSetting || key == workspaceWallpaperByID || key == workspaceWallpaperGlobal {
			updated.Settings[key] = value
		}
	}
	s.InvalidateSettings()
	desktopMutationMu.Unlock()
	locked = false
	_ = s.Audit(ctx, "set_workspace_settings", "desktop_settings", map[string]interface{}{"keys": sortedWorkspaceSettingKeys(normalized), "revision": nextLayout.Revision}, source)
	return updated, nil
}

func rejectLayoutWrite(values map[string]string) error {
	if _, exists := values[WorkspaceLayoutSettingKey]; exists {
		return fmt.Errorf("%w: layout is service-managed", ErrWorkspaceInvalid)
	}
	return nil
}

func sortedWorkspaceSettingKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func upsertWorkspaceSettingsTx(ctx context.Context, tx *sql.Tx, values map[string]string) error {
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO desktop_settings(key, value, updated_at)
		VALUES(?, ?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`)
	if err != nil {
		return fmt.Errorf("prepare workspace settings update: %w", err)
	}
	defer stmt.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for key, value := range values {
		if _, err := stmt.ExecContext(ctx, key, value, now); err != nil {
			return fmt.Errorf("save workspace setting %s: %w", key, err)
		}
	}
	return nil
}
