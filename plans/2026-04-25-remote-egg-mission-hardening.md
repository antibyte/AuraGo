# Remote Egg Mission Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden remote egg missions so startup, sync, run, delete, metadata preservation, and result reporting behave predictably in production.

**Architecture:** Keep the master-side Mission Control UI as the source of truth and route all master-to-egg mission operations through the existing signed Invasion bridge. On the egg, replace public mission UI API calls with internal loopback endpoints guarded by `X-Internal-Token`, so synced missions can preserve metadata while still allowing bridge-controlled force apply/delete behavior. The bridge gets unique message IDs, non-blocking egg handlers, and active target validation.

**Tech Stack:** Go 1.26, `net/http`, existing `MissionManagerV2`, Invasion Control WebSocket bridge, SQLite-backed Invasion registry, vanilla JS Mission Control UI, existing Go unit tests.

---

## Audit Decision

The report `reports/remote_egg_mission_audit_2026-04-25.md` is relevant. The critical and high findings match the current code:

- `cmd/aurago/main.go` starts `eggClient.Start()` before `server.Start(...)` has a ready loopback listener.
- `internal/invasion/bridge/client.go` processes mission sync/run/delete synchronously in the egg `readLoop`.
- `internal/invasion/bridge/protocol.go` uses `time.Now().UnixNano()` alone for bridge message IDs.
- `internal/server/mission_v2_handlers.go` maps every delete error to `404`.
- `internal/server/mission_remote_sync.go` does not reject inactive nests/eggs in target listing or validation.
- `cmd/aurago/main.go` drops `AutoPrepare`, `CreatedAt`, and currently hardcodes synced mission `Locked` to `false`.
- `internal/tools/missions_v2.go` deletes the old remote target before confirming the new remote target can accept an updated mission.
- `internal/server/server.go` forwards every egg-side mission completion to the master, including local egg missions that were never synced from the master.

Not every medium item needs the same urgency:

- The CSS `:has()` concern is already partially mitigated because `selectRunnerType` toggles `.active`; remove the `:has()` dependency in the same UI touch, but it is not a blocker.
- Live remote-target refresh is useful but not required for correctness. Leave it out of this hardening pass so the critical lifecycle fixes stay small.
- `runWithContext` is a small leak bounded by the bridge ack timeout; fix it by making the bridge send methods context-aware while changing the hub.

## Files

- Modify: `internal/invasion/bridge/protocol.go`
  - Add collision-resistant message IDs using a package-level atomic counter.
  - Extend `MissionSyncPayload` with `AutoPrepare` and `CreatedAt`.
- Modify: `internal/invasion/bridge/protocol_test.go`
  - Add deterministic tests for message ID uniqueness with the same timestamp.
  - Extend mission sync payload round-trip test for new fields.
- Modify: `internal/invasion/bridge/client.go`
  - Dispatch mission sync/run/delete callbacks in goroutines and ack after handler completion.
  - Track active mission handlers in the same `activeTasks` telemetry counter used by regular tasks.
- Modify: `internal/invasion/bridge/hub.go`
  - Add context-aware send methods for mission messages.
  - Keep existing `SendMissionSync`, `SendMissionRun`, and `SendMissionDelete` as wrappers using `context.Background()`.
- Modify: `internal/invasion/bridge/hub_test.go`
  - Add ack collision regression coverage through direct message ID helper tests and a context cancellation test for mission sends.
- Modify: `internal/server/mission_remote_sync.go`
  - Validate `nest.Active` and `egg.Active`.
  - Use context-aware hub methods instead of wrapping blocking calls in `runWithContext`.
  - Include `AutoPrepare`, `CreatedAt`, and `Locked` in sync payloads.
- Create: `internal/server/mission_internal_sync_handlers.go`
  - Add egg-local internal loopback handlers for bridge-driven mission sync/run/delete.
  - Guard handlers with `X-Internal-Token` and loopback address validation.
- Modify: `internal/server/auth.go`
  - Allow `/api/internal/missions/...` through auth only when `isValidInternalLoopbackToken` passes.
- Modify: `internal/server/server_routes_config.go`
  - Register the new internal mission endpoints next to Mission Control routes.
- Modify: `internal/server/mission_v2_handlers.go`
  - Use `missionErrorStatus(err)` for delete errors.
  - Support `DELETE /api/missions/v2/{id}?force=true` for local removal of remote missions when an egg is offline.
- Modify: `internal/server/server.go`
  - Gate `EggMissionResultSink` so it forwards only completions for missions synced from the master.
- Modify: `internal/tools/missions_v2.go`
  - Add synced-mission tracking metadata and manager helpers for internal bridge apply/delete.
  - Add force delete support for remote missions.
  - Reorder remote target changes so the new target is synced before the old target is deleted.
  - Preserve `CreatedAt`, `Locked`, and `AutoPrepare` during sync.
- Modify: `internal/tools/mission_remote.go`
  - Add helper predicates for remote mission local execution and synced-from-master checks.
- Modify: `internal/tools/missions_v2_test.go`
  - Add tests for force delete, target-switch ordering, synced metadata preservation, and result forwarding scope.
- Modify: `internal/server/mission_v2_handlers_test.go`
  - Add delete error status and force-delete handler coverage.
- Create: `internal/server/mission_remote_sync_test.go`
  - Add remote target filtering and inactive target validation tests.
- Modify: `cmd/aurago/main.go`
  - Replace egg mission public API calls with the new internal mission loopback API.
  - Add loopback readiness wait before starting the egg bridge client.
- Modify: `ui/js/missions/main.js`
  - Show force-delete confirmation when a remote delete fails due unavailable egg.
  - Keep remote trigger disabling intact.
- Modify: `ui/css/missions.css`
  - Remove reliance on `:has()` for selected runner styling; keep `.active` styling.
- Modify all 15 affected mission translation files when the force-delete confirmation adds a new visible UI string.

## Task 1: Bridge IDs And Non-Blocking Egg Mission Handlers

**Files:**
- Modify: `internal/invasion/bridge/protocol.go`
- Modify: `internal/invasion/bridge/protocol_test.go`
- Modify: `internal/invasion/bridge/client.go`

- [ ] **Step 1: Add failing tests for message ID uniqueness and mission payload metadata**

Add this test block to `internal/invasion/bridge/protocol_test.go`:

```go
func TestNewMessageIDIsUniqueForSameTimestamp(t *testing.T) {
	first := newMessageID(time.Unix(1777140000, 123))
	second := newMessageID(time.Unix(1777140000, 123))
	if first == second {
		t.Fatalf("newMessageID returned duplicate IDs for same timestamp: %q", first)
	}
	if !strings.HasPrefix(first, "1777140000000000123-") {
		t.Fatalf("newMessageID prefix = %q, want timestamp prefix", first)
	}
}
```

Extend `TestNewMessage_MissionSyncPayloadSerialization` to include:

```go
createdAt := time.Date(2026, 4, 25, 20, 30, 0, 0, time.UTC)
AutoPrepare: true,
CreatedAt:   createdAt,
```

and after unmarshalling assert:

```go
if !decoded.AutoPrepare {
	t.Fatal("AutoPrepare was not preserved")
}
if !decoded.CreatedAt.Equal(createdAt) {
	t.Fatalf("CreatedAt = %s, want %s", decoded.CreatedAt, createdAt)
}
```

- [ ] **Step 2: Run the focused protocol tests and confirm failure**

Run:

```powershell
go test -count=1 ./internal/invasion/bridge -run 'TestNewMessageIDIsUniqueForSameTimestamp|TestNewMessage_MissionSyncPayloadSerialization'
```

Expected: compile failure because `newMessageID`, `AutoPrepare`, and `CreatedAt` are not defined yet.

- [ ] **Step 3: Implement unique message IDs and payload metadata**

In `internal/invasion/bridge/protocol.go`, add `sync/atomic` to imports and this package variable/helper near `NewMessage`:

```go
var bridgeMessageCounter atomic.Uint64

func newMessageID(now time.Time) string {
	return fmt.Sprintf("%d-%d", now.UnixNano(), bridgeMessageCounter.Add(1))
}
```

Extend `MissionSyncPayload`:

```go
AutoPrepare bool      `json:"auto_prepare,omitempty"`
CreatedAt   time.Time `json:"created_at,omitempty"`
```

Change `NewMessage` to capture a single timestamp:

```go
now := time.Now().UTC()
msg := &Message{
	Type:      msgType,
	EggID:     eggID,
	NestID:    nestID,
	ID:        newMessageID(now),
	Payload:   raw,
	Timestamp: now.Format(time.RFC3339),
}
```

- [ ] **Step 4: Run protocol tests**

Run:

```powershell
go test -count=1 ./internal/invasion/bridge -run 'TestNewMessageIDIsUniqueForSameTimestamp|TestNewMessage_MissionSyncPayloadSerialization|TestNewMessage_AllTypes'
```

Expected: PASS.

- [ ] **Step 5: Make egg mission callbacks asynchronous but ack after completion**

In `internal/invasion/bridge/client.go`, add a helper:

```go
func (c *EggClient) runAckedHandler(refID, successDetail string, handler func() error) {
	c.mu.Lock()
	c.activeTasks++
	c.mu.Unlock()
	go func() {
		defer func() {
			c.mu.Lock()
			c.activeTasks--
			c.mu.Unlock()
		}()
		if err := handler(); err != nil {
			c.sendAck(refID, false, err.Error())
			return
		}
		c.sendAck(refID, true, successDetail)
	}()
}
```

Change mission cases in the `readLoop` so only invalid payload or missing handler acks immediately. Valid mission handler calls should use:

```go
c.runAckedHandler(msg.ID, "mission synced", func() error {
	return c.OnMissionSync(payload)
})
```

Use `"mission run queued"` for `MsgMissionRun` and `"mission deleted"` for `MsgMissionDelete`.

- [ ] **Step 6: Run bridge package tests**

Run:

```powershell
go test -count=1 ./internal/invasion/bridge
```

Expected: PASS.

- [ ] **Step 7: Commit bridge hardening**

```powershell
git add internal/invasion/bridge/protocol.go internal/invasion/bridge/protocol_test.go internal/invasion/bridge/client.go
git commit -m "fix: harden egg bridge mission messaging"
```

## Task 2: Internal Egg Mission API And Startup Readiness

**Files:**
- Create: `internal/server/mission_internal_sync_handlers.go`
- Modify: `internal/server/auth.go`
- Modify: `internal/server/server_routes_config.go`
- Modify: `cmd/aurago/main.go`
- Modify: `internal/tools/missions_v2.go`
- Modify: `internal/tools/mission_remote.go`

- [ ] **Step 1: Add failing manager tests for synced mission metadata and result scope**

Add tests to `internal/tools/missions_v2_test.go`:

```go
func TestApplySyncedMissionPreservesMetadata(t *testing.T) {
	mgr := NewMissionManagerV2(t.TempDir()+"/missions.json", nil, nil)
	createdAt := time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC)
	mission := &MissionV2{
		ID:            "mission_remote_1",
		Name:          "Remote backup",
		Prompt:        "Run backup",
		ExecutionType: ExecutionManual,
		Priority:      "high",
		Enabled:       true,
		Locked:        true,
		CreatedAt:     createdAt,
		AutoPrepare:   true,
		SyncedFromMaster: true,
	}
	if err := mgr.ApplySyncedMission(mission); err != nil {
		t.Fatalf("ApplySyncedMission: %v", err)
	}
	got, ok := mgr.Get("mission_remote_1")
	if !ok {
		t.Fatal("synced mission was not stored")
	}
	if !got.Locked || !got.AutoPrepare || !got.CreatedAt.Equal(createdAt) || !got.SyncedFromMaster {
		t.Fatalf("synced metadata not preserved: %+v", got)
	}
}

func TestDeleteSyncedMissionBypassesLock(t *testing.T) {
	mgr := NewMissionManagerV2(t.TempDir()+"/missions.json", nil, nil)
	if err := mgr.ApplySyncedMission(&MissionV2{
		ID: "mission_locked_remote", Name: "Locked", Prompt: "x",
		ExecutionType: ExecutionManual, Enabled: true, Locked: true, SyncedFromMaster: true,
	}); err != nil {
		t.Fatalf("ApplySyncedMission: %v", err)
	}
	if err := mgr.DeleteSyncedMission("mission_locked_remote"); err != nil {
		t.Fatalf("DeleteSyncedMission: %v", err)
	}
	if _, ok := mgr.Get("mission_locked_remote"); ok {
		t.Fatal("locked synced mission still exists")
	}
}

func TestApplySyncedScheduledMissionRegistersCron(t *testing.T) {
	cronMgr := NewCronManager(nil)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	mgr := NewMissionManagerV2(t.TempDir()+"/missions.json", cronMgr, nil)
	if err := mgr.ApplySyncedMission(&MissionV2{
		ID: "mission_scheduled_remote", Name: "Remote schedule", Prompt: "x",
		ExecutionType: ExecutionScheduled, Schedule: "0 4 * * *",
		Enabled: true, SyncedFromMaster: true,
	}); err != nil {
		t.Fatalf("ApplySyncedMission: %v", err)
	}
	if !hasCronJob(cronMgr, "mission_mission_scheduled_remote") {
		t.Fatal("synced scheduled mission was not registered with cron")
	}
}
```

- [ ] **Step 2: Run tests and confirm failure**

Run:

```powershell
go test -count=1 ./internal/tools -run 'TestApplySyncedMissionPreservesMetadata|TestDeleteSyncedMissionBypassesLock'
```

Expected: compile failure because the synced mission field and methods do not exist yet.

- [ ] **Step 3: Add synced mission metadata and manager helpers**

In `internal/tools/missions_v2.go`, add to `MissionV2` remote fields:

```go
SyncedFromMaster bool `json:"synced_from_master,omitempty"`
```

Add helpers:

```go
func (m *MissionManagerV2) ApplySyncedMission(mission *MissionV2) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mission == nil {
		return fmt.Errorf("mission is required")
	}
	mission.RunnerType = MissionRunnerLocal
	mission.RemoteNestID = ""
	mission.RemoteEggID = ""
	mission.RemoteSyncStatus = ""
	mission.RemoteSyncError = ""
	mission.RemoteRevision = ""
	mission.SyncedFromMaster = true
	if mission.ID == "" {
		return fmt.Errorf("mission id is required")
	}
	if mission.CreatedAt.IsZero() {
		mission.CreatedAt = time.Now()
	}
	if mission.Priority == "" {
		mission.Priority = "medium"
	}
	if mission.ExecutionType == "" {
		mission.ExecutionType = ExecutionManual
	}
	mission.Status = MissionStatusIdle
	if existing, ok := m.missions[mission.ID]; ok && existing.ExecutionType == ExecutionScheduled && existing.Schedule != "" && m.cron != nil {
		_ = m.cron.ManageSchedule("remove", "mission_"+mission.ID, "", "", "")
	}
	m.missions[mission.ID] = mission
	if mission.Enabled {
		if mission.ExecutionType == ExecutionTriggered {
			m.registerTrigger(mission)
		} else if mission.ExecutionType == ExecutionScheduled && mission.Schedule != "" {
			if m.cron == nil {
				return fmt.Errorf("cron manager is not configured")
			}
			if _, err := m.cron.ManageScheduleWithSource("add", "mission_"+mission.ID, mission.Schedule, mission.Prompt, "", "mission"); err != nil {
				return fmt.Errorf("failed to register synced mission with cron: %w", err)
			}
		}
	}
	return m.save()
}

func (m *MissionManagerV2) DeleteSyncedMission(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mission, ok := m.missions[id]
	if !ok {
		return fmt.Errorf("mission not found")
	}
	if !mission.SyncedFromMaster {
		return fmt.Errorf("mission is not synced from master")
	}
	if mission.ExecutionType == ExecutionScheduled && mission.Schedule != "" && m.cron != nil {
		_ = m.cron.ManageSchedule("remove", "mission_"+id, "", "", "")
	}
	delete(m.missions, id)
	m.queue.Remove(id)
	if m.preparedDB != nil {
		DeletePreparedMission(m.preparedDB, id)
	}
	return m.save()
}

func (m *MissionManagerV2) IsSyncedFromMaster(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mission, ok := m.missions[id]
	return ok && mission.SyncedFromMaster
}
```

- [ ] **Step 4: Add internal mission sync handlers**

Create `internal/server/mission_internal_sync_handlers.go`:

```go
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurago/internal/invasion/bridge"
	"aurago/internal/tools"
)

func handleInternalMissionSync(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isValidInternalLoopbackToken(r, s.internalToken) {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		var payload bridge.MissionSyncPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		mission, err := missionFromSyncPayload(payload)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.MissionManagerV2.ApplySyncedMission(mission); err != nil {
			jsonError(w, err.Error(), missionErrorStatus(err))
			return
		}
		broadcastMissionState(s)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleInternalMissionRun(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isValidInternalLoopbackToken(r, s.internalToken) {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/internal/missions/")
		id = strings.TrimSuffix(id, "/run")
		if id == "" {
			jsonError(w, "Mission ID is required", http.StatusBadRequest)
			return
		}
		if !s.MissionManagerV2.IsSyncedFromMaster(id) {
			jsonError(w, "Mission is not synced from master", http.StatusForbidden)
			return
		}
		if err := s.MissionManagerV2.RunNow(id); err != nil {
			jsonError(w, err.Error(), missionErrorStatus(err))
			return
		}
		broadcastMissionState(s)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
	}
}

func handleInternalMissionDelete(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isValidInternalLoopbackToken(r, s.internalToken) {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/internal/missions/")
		if id == "" {
			jsonError(w, "Mission ID is required", http.StatusBadRequest)
			return
		}
		if err := s.MissionManagerV2.DeleteSyncedMission(id); err != nil {
			jsonError(w, err.Error(), missionErrorStatus(err))
			return
		}
		broadcastMissionState(s)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func missionFromSyncPayload(payload bridge.MissionSyncPayload) (*tools.MissionV2, error) {
	if payload.MissionID == "" {
		return nil, fmt.Errorf("mission_id is required")
	}
	var triggerConfig *tools.TriggerConfig
	if len(payload.TriggerConfig) > 0 {
		var cfg tools.TriggerConfig
		if err := json.Unmarshal(payload.TriggerConfig, &cfg); err != nil {
			return nil, fmt.Errorf("invalid trigger config: %w", err)
		}
		triggerConfig = &cfg
	}
	createdAt := payload.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return &tools.MissionV2{
		ID:               payload.MissionID,
		Name:             payload.Name,
		Prompt:           payload.PromptSnapshot,
		ExecutionType:    tools.ExecutionType(payload.ExecutionType),
		Schedule:         payload.Schedule,
		TriggerType:      tools.TriggerType(payload.TriggerType),
		TriggerConfig:    triggerConfig,
		Priority:         payload.Priority,
		Enabled:          payload.Enabled,
		Locked:           payload.Locked,
		CreatedAt:        createdAt,
		AutoPrepare:      payload.AutoPrepare,
		SyncedFromMaster: true,
	}, nil
}
```

- [ ] **Step 5: Register and auth-bypass internal mission endpoints**

In `internal/server/auth.go`, extend the internal exception:

```go
if (strings.HasPrefix(r.URL.Path, "/api/internal/tool-bridge/") ||
	strings.HasPrefix(r.URL.Path, "/api/internal/missions/")) &&
	isValidInternalLoopbackToken(r, s.internalToken) {
	next.ServeHTTP(w, r)
	return
}
```

In `internal/server/server_routes_config.go`, register:

```go
mux.HandleFunc("/api/internal/missions/sync", handleInternalMissionSync(s))
mux.HandleFunc("/api/internal/missions/", func(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/run") {
		handleInternalMissionRun(s)(w, r)
		return
	}
	handleInternalMissionDelete(s)(w, r)
})
```

Add `strings` to imports if that file does not already import it.

- [ ] **Step 6: Replace egg-mode public API callbacks**

In `cmd/aurago/main.go`, keep `eggMissionAPI`, but change callback paths:

```go
eggClient.OnMissionSync = func(payload bridge.MissionSyncPayload) error {
	appLog.Info("Mission sync received from master", "mission_id", payload.MissionID, "revision", payload.Revision)
	return eggMissionAPI(http.MethodPost, "/api/internal/missions/sync", payload)
}
eggClient.OnMissionRun = func(payload bridge.MissionRunPayload) error {
	appLog.Info("Mission run received from master", "mission_id", payload.MissionID, "trigger_type", payload.TriggerType)
	return eggMissionAPI(http.MethodPost, "/api/internal/missions/"+payload.MissionID+"/run", map[string]string{
		"trigger_type": payload.TriggerType,
		"trigger_data": payload.TriggerData,
	})
}
eggClient.OnMissionDelete = func(payload bridge.MissionDeletePayload) error {
	appLog.Info("Mission delete received from master", "mission_id", payload.MissionID)
	return eggMissionAPI(http.MethodDelete, "/api/internal/missions/"+payload.MissionID, nil)
}
```

Add a readiness guard:

```go
func waitForInternalAPIReady(ctx context.Context, cfg *config.Config, token string, client *http.Client) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	url := server.InternalAPIURL(cfg) + "/api/health"
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-Internal-FollowUp", "true")
		req.Header.Set("X-Internal-Token", token)
		resp, err := client.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
```

Store `eggClient` in a variable and start it from a goroutine that first waits for readiness:

```go
go func() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := waitForInternalAPIReady(ctx, cfg, loopbackToken, cronHTTPClient); err != nil {
		appLog.Error("Egg client startup aborted because internal API is not ready", "error", err)
		return
	}
	eggClient.Start()
}()
```

- [ ] **Step 7: Gate mission result forwarding**

In `internal/server/server.go`, change the completion callback:

```go
if opts.EggMissionResultSink != nil {
	s.MissionManagerV2.SetCompletionCallback(func(missionID, result, output string) {
		if !s.MissionManagerV2.IsSyncedFromMaster(missionID) {
			return
		}
		payload := bridge.MissionResultPayload{
			MissionID: missionID,
			Result:    result,
			Output:    output,
		}
		if result != tools.MissionResultSuccess && result != "success" {
			payload.Error = output
		}
		if err := opts.EggMissionResultSink(payload); err != nil {
			logger.Warn("[MissionV2] Failed to send remote mission result", "mission_id", missionID, "error", err)
		}
	})
}
```

- [ ] **Step 8: Run manager and compile tests**

Run:

```powershell
go test -count=1 ./internal/tools -run 'TestApplySyncedMissionPreservesMetadata|TestDeleteSyncedMissionBypassesLock'
go test -run '^$' ./cmd/aurago ./internal/server ./internal/tools
```

Expected: PASS.

- [ ] **Step 9: Commit internal egg mission API**

```powershell
git add cmd/aurago/main.go internal/server/auth.go internal/server/server.go internal/server/server_routes_config.go internal/server/mission_internal_sync_handlers.go internal/tools/missions_v2.go internal/tools/mission_remote.go internal/tools/missions_v2_test.go
git commit -m "fix: apply egg missions through internal loopback API"
```

## Task 3: Remote Target Validation, Delete Semantics, And Target Switch Ordering

**Files:**
- Modify: `internal/server/mission_remote_sync.go`
- Modify: `internal/server/mission_v2_handlers.go`
- Modify: `internal/tools/missions_v2.go`
- Modify: `internal/tools/missions_v2_test.go`
- Create: `internal/server/mission_remote_sync_test.go`
- Modify: `internal/server/mission_v2_handlers_test.go`

- [ ] **Step 1: Add failing tests for inactive remote targets**

Create `internal/server/mission_remote_sync_test.go` with tests that use `httptest.NewRecorder` and direct handler calls, not a listening TCP server:

```go
package server

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
)

func setupInvasionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := invasion.InitDB(filepath.Join(t.TempDir(), "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func registerFakeEggConnection(t *testing.T, hub *bridge.EggHub, nestID, eggID string) {
	t.Helper()
	if err := hub.Register(nestID, &bridge.EggConnection{
		EggID:     eggID,
		NestID:    nestID,
		SharedKey: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatalf("hub.Register(%s): %v", nestID, err)
	}
}

func TestMissionRemoteTargetsFiltersInactiveNestsAndEggs(t *testing.T) {
	db := setupInvasionTestDB(t)
	activeEggID, _ := invasion.CreateEgg(db, invasion.EggRecord{Name: "Active Egg", Active: true})
	inactiveEggID, _ := invasion.CreateEgg(db, invasion.EggRecord{Name: "Inactive Egg", Active: false})
	activeNestID, _ := invasion.CreateNest(db, invasion.NestRecord{Name: "Active Nest", Active: true, EggID: activeEggID})
	inactiveNestID, _ := invasion.CreateNest(db, invasion.NestRecord{Name: "Inactive Nest", Active: false, EggID: activeEggID})
	inactiveEggNestID, _ := invasion.CreateNest(db, invasion.NestRecord{Name: "Inactive Egg Nest", Active: true, EggID: inactiveEggID})

	hub := bridge.NewEggHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	registerFakeEggConnection(t, hub, activeNestID, activeEggID)
	registerFakeEggConnection(t, hub, inactiveNestID, activeEggID)
	registerFakeEggConnection(t, hub, inactiveEggNestID, inactiveEggID)

	s := &Server{InvasionDB: db, EggHub: hub, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	req := httptest.NewRequest(http.MethodGet, "/api/missions/v2/remote-targets", nil)
	rr := httptest.NewRecorder()
	handleMissionRemoteTargets(s)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Targets []remoteMissionTarget `json:"targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body.Targets) != 1 || body.Targets[0].NestID != activeNestID {
		t.Fatalf("targets = %+v, want only active nest", body.Targets)
	}
}
```

- [ ] **Step 2: Add failing tests for force delete and delete error status**

Add to `internal/tools/missions_v2_test.go`:

```go
func TestForceDeleteRemoteMissionSkipsRemoteClient(t *testing.T) {
	client := &fakeRemoteMissionClient{deleteErr: errors.New("remote nest nest-1 is not connected")}
	mgr := NewMissionManagerV2(t.TempDir()+"/missions.json", nil, nil)
	mgr.SetRemoteMissionClient(client)
	mission := &MissionV2{
		ID: "mission_remote_delete", Name: "Remote", Prompt: "x",
		ExecutionType: ExecutionManual, Priority: "medium", Enabled: true,
		RunnerType: MissionRunnerRemote, RemoteNestID: "nest-1", RemoteEggID: "egg-1",
	}
	if err := mgr.Create(mission); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := mgr.DeleteWithOptions("mission_remote_delete", DeleteMissionOptions{ForceRemote: true}); err != nil {
		t.Fatalf("DeleteWithOptions(force): %v", err)
	}
	if _, ok := mgr.Get("mission_remote_delete"); ok {
		t.Fatal("mission still exists after force delete")
	}
}
```

Add to `internal/server/mission_v2_handlers_test.go` a direct handler test that configures a manager whose remote client returns `"remote nest nest-1 is not connected"` and asserts the response is not `404`.

- [ ] **Step 3: Implement active target validation**

In `internal/server/mission_remote_sync.go`:

```go
eggsByID := make(map[string]invasion.EggRecord, len(eggs))
for _, egg := range eggs {
	eggsByID[egg.ID] = egg
}
```

Filter target listing:

```go
egg, ok := eggsByID[nest.EggID]
if !nest.Active || !ok || !egg.Active {
	continue
}
```

In `validateTarget`, after `GetNest`:

```go
if !nest.Active {
	return fmt.Errorf("remote nest %s is inactive", mission.RemoteNestID)
}
egg, err := invasion.GetEgg(c.db, mission.RemoteEggID)
if err != nil {
	return fmt.Errorf("remote egg %s not found: %w", mission.RemoteEggID, err)
}
if !egg.Active {
	return fmt.Errorf("remote egg %s is inactive", mission.RemoteEggID)
}
```

- [ ] **Step 4: Implement context-aware hub mission sends**

In `internal/invasion/bridge/hub.go`, add:

```go
func (h *EggHub) SendMissionSyncContext(ctx context.Context, nestID string, payload MissionSyncPayload) error {
	return h.sendMissionMessageContext(ctx, nestID, MsgMissionSync, payload)
}

func (h *EggHub) SendMissionRunContext(ctx context.Context, nestID string, payload MissionRunPayload) error {
	return h.sendMissionMessageContext(ctx, nestID, MsgMissionRun, payload)
}

func (h *EggHub) SendMissionDeleteContext(ctx context.Context, nestID string, payload MissionDeletePayload) error {
	return h.sendMissionMessageContext(ctx, nestID, MsgMissionDelete, payload)
}
```

Change `sendWithAck` to accept `ctx context.Context` and select on `ctx.Done()` instead of leaking a goroutine around a blocking send.

In `internal/server/mission_remote_sync.go`, replace `runWithContext` calls with these context-aware hub methods and delete `runWithContext`.

- [ ] **Step 5: Implement delete error status and force delete option**

In `internal/tools/missions_v2.go`, add:

```go
type DeleteMissionOptions struct {
	ForceRemote bool
}

func (m *MissionManagerV2) DeleteWithOptions(id string, opts DeleteMissionOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mission, ok := m.missions[id]
	if !ok {
		return fmt.Errorf("mission not found")
	}
	if mission.Locked {
		return fmt.Errorf("mission is locked")
	}

	if !isRemoteMission(mission) && mission.ExecutionType == ExecutionScheduled && mission.Schedule != "" && m.cron != nil {
		_ = m.cron.ManageSchedule("remove", "mission_"+id, "", "", "")
	}
	if isRemoteMission(mission) && m.remoteClient != nil && !opts.ForceRemote {
		ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
		err := m.remoteClient.DeleteMission(ctx, *mission)
		cancel()
		if err != nil {
			return err
		}
	}

	delete(m.missions, id)
	m.queue.Remove(id)
	if m.preparedDB != nil {
		DeletePreparedMission(m.preparedDB, id)
	}
	return m.save()
}
```

Make existing `Delete(id string)` call:

```go
return m.DeleteWithOptions(id, DeleteMissionOptions{})
```

In `internal/server/mission_v2_handlers.go`:

```go
force := r.URL.Query().Get("force") == "true"
var err error
if force {
	err = s.MissionManagerV2.DeleteWithOptions(id, tools.DeleteMissionOptions{ForceRemote: true})
} else {
	err = s.MissionManagerV2.Delete(id)
}
if err != nil {
	jsonError(w, err.Error(), missionErrorStatus(err))
	return
}
```

- [ ] **Step 6: Reorder remote target switch update**

In `internal/tools/missions_v2.go`, update `Update` ordering:

1. Preserve metadata on `updated`.
2. If `updated` is remote, sync it to the new/current target first.
3. Only after new sync succeeds, delete the old remote copy if the old mission was remote and the target changed or the runner became local.
4. If old cleanup fails after new sync succeeds, store `updated`, set `RemoteSyncStatus = RemoteSyncError`, set `RemoteSyncError` to a cleanup warning, and return the cleanup error so the UI can show it without losing the new target state.

Use this shape:

```go
oldRemoteNeedsCleanup := isRemoteMission(mission) && (!isRemoteMission(updated) || mission.RemoteNestID != updated.RemoteNestID)
if isRemoteMission(updated) {
	if err := m.syncRemoteMissionLocked(updated); err != nil {
		return err
	}
}
cleanupErr := error(nil)
if oldRemoteNeedsCleanup && m.remoteClient != nil {
	ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
	cleanupErr = m.remoteClient.DeleteMission(ctx, *mission)
	cancel()
}
m.missions[id] = updated
if cleanupErr != nil {
	updated.RemoteSyncStatus = RemoteSyncError
	updated.RemoteSyncError = "new remote target synced, but old target cleanup failed: " + cleanupErr.Error()
	_ = m.save()
	return cleanupErr
}
```

- [ ] **Step 7: Run focused tests**

Run:

```powershell
go test -count=1 ./internal/tools -run 'TestForceDeleteRemoteMissionSkipsRemoteClient|TestRemote'
go test -count=1 ./internal/server -run 'TestMissionRemoteTargets|TestMissionDelete'
go test -count=1 ./internal/invasion/bridge
```

Expected: PASS.

- [ ] **Step 8: Commit remote lifecycle hardening**

```powershell
git add internal/server/mission_remote_sync.go internal/server/mission_remote_sync_test.go internal/server/mission_v2_handlers.go internal/server/mission_v2_handlers_test.go internal/tools/missions_v2.go internal/tools/missions_v2_test.go internal/invasion/bridge/hub.go internal/invasion/bridge/hub_test.go
git commit -m "fix: harden remote mission lifecycle"
```

## Task 4: UI Cleanup For Force Delete And Runner Styling

**Files:**
- Modify: `ui/js/missions/main.js`
- Modify: `ui/css/missions.css`
- Modify translation files in `ui/lang/` because the force-delete confirmation adds visible text.

- [ ] **Step 1: Add force-delete UX for failed remote deletes**

In `ui/js/missions/main.js`, find the delete mission flow. When the first `DELETE /api/missions/v2/{id}` response fails with a message containing `remote nest` or `not connected`, show the existing custom confirmation modal with German/translated copy equivalent to:

```text
The remote egg is not reachable. Remove this mission locally anyway? The old copy may remain on the egg until it reconnects.
```

On confirmation, retry:

```js
await fetch(`/api/missions/v2/${encodeURIComponent(id)}?force=true`, { method: 'DELETE' });
```

Do not use `alert()`.

- [ ] **Step 2: Remove CSS reliance on `:has()`**

In `ui/css/missions.css`, replace:

```css
.mission-runner-option:has(input:checked) { ... }
```

with:

```css
.mission-runner-option.active { ... }
```

Keep the existing JS class toggle in `selectRunnerType`.

- [ ] **Step 3: Add translations for the new modal text**

Replace the delete confirmation text with an i18n key and add that key to all 15 language files. Keep translations concise and do not use English-only placeholders.

- [ ] **Step 4: Run frontend syntax checks**

Run:

```powershell
node --check ui/js/missions/main.js
```

Expected: PASS where Node is healthy. If the local Node runtime fails before parsing with the known `CSPRNG` assertion, document that exact runtime failure in the verification notes and rely on browser/manual verification.

- [ ] **Step 5: Commit UI cleanup**

```powershell
git add ui/js/missions/main.js ui/css/missions.css ui/missions_v2.html ui/lang
git commit -m "fix: improve remote mission delete UX"
```

## Task 5: Final Verification

**Files:**
- No new files required.

- [ ] **Step 1: Run focused regression suite**

Run:

```powershell
go test -count=1 ./internal/invasion/bridge
go test -count=1 ./internal/tools -run 'TestRemote|TestApplySyncedMission|TestDeleteSyncedMission|TestForceDeleteRemoteMission'
go test -count=1 ./internal/server -run 'TestMissionRemoteTargets|TestMissionDelete|TestInternalMission'
go test -run '^$' ./cmd/aurago ./internal/server ./internal/tools ./internal/invasion/bridge
```

Expected: PASS.

- [ ] **Step 2: Run broader compile smoke**

Run:

```powershell
go test -run '^$' ./...
```

Expected: PASS. If Windows loopback `httptest` fails with `failed to listen on tcp6 [::1]:0`, record it as the known local environment limitation and keep the targeted package results as verification evidence.

- [ ] **Step 3: Run git review**

Run:

```powershell
git status --short
git diff --stat HEAD
git log --oneline -5
```

Expected:

- Worktree contains only intended files before the final commit.
- Last commits are the bridge, internal API, lifecycle, and UI hardening commits.

- [ ] **Step 4: Final commit if any verification-only changes remain**

```powershell
git add -A
git commit -m "test: cover remote egg mission hardening"
```

Skip this commit when there are no remaining uncommitted changes.

## Acceptance Criteria

- Remote mission sync/run/delete does not race local HTTP server readiness on egg startup.
- Egg bridge read loop remains responsive while mission callbacks perform loopback work.
- Bridge message IDs are unique even for multiple messages created in the same nanosecond.
- Remote delete failures return meaningful HTTP statuses and messages.
- A user can force-delete a remote mission locally when the egg is offline.
- Inactive nests and inactive eggs are not listed or accepted as remote mission targets.
- `Locked`, `AutoPrepare`, and `CreatedAt` survive master-to-egg sync.
- Locked synced missions can still be removed by the bridge-controlled internal delete path.
- Moving a remote mission from one nest to another syncs the new target before old-target cleanup.
- Egg mission result forwarding only sends results for missions synced from the master.
- Focused Go tests pass for bridge, tools, and server hardening.
