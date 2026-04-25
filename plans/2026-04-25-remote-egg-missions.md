# Remote Egg Missions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Missions can be created, edited, run, and deleted either locally or on a deployed Invasion Control egg, with remote mission definitions synchronized to the target nest and existing cheatsheet attachments included in the remote payload.

**Architecture:** Add an explicit mission execution location (`local` or `remote`) to `MissionV2`. The master remains the source of truth for the Mission Control UI, while remote missions are delivered to a running egg through new signed WebSocket bridge messages with acknowledgement handling. Remote eggs apply the synchronized mission to their local `MissionManagerV2`; remote run/result events are reported back to the master so the UI stays coherent.

**Tech Stack:** Go 1.26, `net/http`, existing `MissionManagerV2`, Invasion Control WebSocket bridge, vanilla JS Mission Control UI, JSON translation files.

---

## Current Findings

- Missions are currently stored in `data/missions_v2.json` via `internal/tools/missions_v2.go`.
- Mission create/update/delete handlers live in `internal/server/mission_v2_handlers.go`.
- Egg communication currently supports generic `task`, `result`, `secret`, `safe_reconfigure`, `stop`, and `ack` messages in `internal/invasion/bridge/protocol.go`.
- The current Invasion `send-task` path only sends a natural-language one-off task; it does not install or activate a mission on the egg.
- The Mission UI already fetches eggs/nests for trigger selectors, but it does not provide a mission execution target selector.
- The Mission modal prompt textarea is `#mission-prompt` in `ui/missions_v2.html` and is currently too small.
- Mission Control currently has linked cheatsheets; cheatsheet attachments are appended by `tools.CheatsheetGetMultiple`, so the first remote implementation can satisfy "attachments must be sent" by sending the resolved cheatsheet + attachment text snapshot with the remote mission payload.

## Remote Trigger Decision

For the first implementation, remote missions should allow only trigger types that can be evaluated on the egg without master-side event routing:

- Allowed for remote: `manual`, `scheduled`, triggered `system_startup`, triggered `mqtt_message`.
- Keep backend-ready but do not expose in UI yet: `home_assistant_state`, because it exists in backend but is not present in the Mission UI today.
- Disable in UI for remote missions: `mission_completed`, `email_received`, `webhook`, `egg_hatched`, `nest_cleared`, `device_connected`, `device_disconnected`, `fritzbox_call`, `budget_warning`, `budget_exceeded`.

Reasoning:

- `mission_completed` needs same-target mission scoping before it is safe.
- `email_received`, `webhook`, `device_*`, `fritzbox_call`, and budget triggers are currently master-side or config-dependent and not safely discoverable per egg from the Mission UI.
- `egg_hatched` and `nest_cleared` are Invasion master events; eggs do not manage the nest fleet.
- `scheduled`, `system_startup`, and `mqtt_message` can be evaluated on the egg's own runtime.

## Files

- Modify: `internal/tools/missions_v2.go`
  - Add local/remote mission fields.
  - Add remote-aware create/update/delete/run helpers.
  - Skip local cron/trigger registration for remote missions.
  - Add prompt snapshot builder for remote sync that includes selected cheatsheets and their attachments.
- Create: `internal/tools/mission_remote.go`
  - Define remote mission constants, DTOs, validation helpers, and `RemoteMissionClient` interface used by `MissionManagerV2`.
- Modify: `internal/invasion/bridge/protocol.go`
  - Add message types and payloads for mission sync, run, delete, and result.
- Modify: `internal/invasion/bridge/hub.go`
  - Add acknowledgement correlation and `SendMissionSync`, `SendMissionRun`, `SendMissionDelete`.
- Modify: `internal/invasion/bridge/client.go`
  - Add egg-side callbacks for mission sync/run/delete.
- Create: `internal/server/mission_remote_sync.go`
  - Implement a server-side remote mission client backed by `EggHub`, `InvasionDB`, and `CheatsheetDB`.
  - Implement `/api/missions/v2/remote-targets`.
- Modify: `internal/server/mission_v2_handlers.go`
  - Validate remote fields.
  - Use remote-aware manager methods instead of plain `Create`, `Update`, `Delete`, and `RunNow`.
- Modify: `internal/server/server_routes_infrastructure.go` or mission route registration file
  - Register remote-target endpoint and bridge mission result handler.
- Modify: `cmd/aurago/main.go`
  - In egg mode, handle mission sync/run/delete messages by applying them to the local `MissionManagerV2`.
- Modify: `ui/missions_v2.html`
  - Add Local/Remote execution target selector and remote egg select.
  - Disable incompatible trigger buttons when remote is selected.
- Modify: `ui/js/missions/main.js`
  - Load remote targets.
  - Save remote mission fields.
  - Re-render disabled trigger states when target changes.
  - Preserve target fields on edit/duplicate.
- Modify: `ui/css/missions.css`
  - Make prompt field larger.
  - Style remote target selector and disabled trigger buttons.
- Modify all 15 mission translation files in `ui/lang/missions/*.json`.
- Add tests in:
  - `internal/tools/missions_v2_test.go`
  - `internal/invasion/bridge/protocol_test.go`
  - `internal/invasion/bridge/hub_test.go`
  - `internal/server/mission_v2_handlers_test.go`
  - `ui/i18n_lint_test.go` should continue to pass with new keys.

---

### Task 1: Mission Model and Validation

**Files:**
- Modify: `internal/tools/missions_v2.go`
- Create: `internal/tools/mission_remote.go`
- Test: `internal/tools/missions_v2_test.go`

- [ ] **Step 1: Write failing tests for remote fields and validation**

Add tests that prove a remote mission requires a target nest and that remote mission copies do not register local cron jobs.

```go
func TestRemoteMissionRequiresNest(t *testing.T) {
	mgr := NewMissionManagerV2(t.TempDir(), NewCronManager())
	err := mgr.Create(&MissionV2{
		Name:          "Remote missing target",
		Prompt:        "run remotely",
		ExecutionType: ExecutionManual,
		RunnerType:    MissionRunnerRemote,
	})
	if err == nil || !strings.Contains(err.Error(), "remote_nest_id is required") {
		t.Fatalf("Create remote without target error = %v, want remote_nest_id validation", err)
	}
}

func TestRemoteScheduledMissionDoesNotRegisterLocalCron(t *testing.T) {
	cronMgr := NewCronManager()
	mgr := NewMissionManagerV2(t.TempDir(), cronMgr)
	mgr.SetRemoteMissionClient(fakeRemoteMissionClient{syncOK: true})

	err := mgr.Create(&MissionV2{
		Name:          "Remote schedule",
		Prompt:        "remote cron",
		ExecutionType: ExecutionScheduled,
		Schedule:      "0 * * * *",
		RunnerType:    MissionRunnerRemote,
		RemoteNestID:  "nest-1",
		RemoteEggID:   "egg-1",
	})
	if err != nil {
		t.Fatalf("Create remote scheduled mission: %v", err)
	}
	if cronMgr.HasJob("mission_" + "nest-1") {
		t.Fatal("remote mission registered a local cron job")
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/tools -run 'TestRemoteMissionRequiresNest|TestRemoteScheduledMissionDoesNotRegisterLocalCron'
```

Expected: build or assertion failure because `RunnerType`, `MissionRunnerRemote`, `RemoteNestID`, `SetRemoteMissionClient`, and `HasJob` do not exist yet.

- [ ] **Step 3: Add remote mission types**

Create `internal/tools/mission_remote.go`:

```go
package tools

import (
	"context"
	"fmt"
	"strings"
)

type MissionRunner string

const (
	MissionRunnerLocal  MissionRunner = "local"
	MissionRunnerRemote MissionRunner = "remote"
)

type RemoteMissionClient interface {
	SyncMission(ctx context.Context, mission MissionV2, promptSnapshot string) error
	DeleteMission(ctx context.Context, mission MissionV2) error
	RunMission(ctx context.Context, mission MissionV2, triggerType, triggerData string) error
}

func normalizeMissionRunner(r MissionRunner) MissionRunner {
	if strings.TrimSpace(string(r)) == "" {
		return MissionRunnerLocal
	}
	return r
}

func validateRemoteMission(m MissionV2) error {
	if normalizeMissionRunner(m.RunnerType) != MissionRunnerRemote {
		return nil
	}
	if strings.TrimSpace(m.RemoteNestID) == "" {
		return fmt.Errorf("remote_nest_id is required for remote missions")
	}
	if strings.TrimSpace(m.RemoteEggID) == "" {
		return fmt.Errorf("remote_egg_id is required for remote missions")
	}
	if !RemoteTriggerAllowed(m.ExecutionType, m.TriggerType) {
		return fmt.Errorf("trigger %q is not supported for remote missions", m.TriggerType)
	}
	return nil
}

func RemoteTriggerAllowed(exec ExecutionType, trig TriggerType) bool {
	switch exec {
	case ExecutionManual, ExecutionScheduled:
		return true
	case ExecutionTriggered:
		switch trig {
		case TriggerSystemStartup, TriggerMQTTMessage:
			return true
		default:
			return false
		}
	default:
		return false
	}
}
```

Extend `MissionV2` in `internal/tools/missions_v2.go`:

```go
RunnerType       MissionRunner `json:"runner_type,omitempty"`        // local | remote
RemoteNestID     string        `json:"remote_nest_id,omitempty"`     // Invasion nest connection target
RemoteNestName   string        `json:"remote_nest_name,omitempty"`   // Display cache
RemoteEggID      string        `json:"remote_egg_id,omitempty"`      // Assigned egg template ID
RemoteEggName    string        `json:"remote_egg_name,omitempty"`    // Display cache
RemoteSyncStatus string        `json:"remote_sync_status,omitempty"` // synced | pending | error
RemoteSyncError  string        `json:"remote_sync_error,omitempty"`
RemoteRevision   string        `json:"remote_revision,omitempty"`
```

Add to `MissionManagerV2`:

```go
remoteClient RemoteMissionClient
```

Add setter:

```go
func (m *MissionManagerV2) SetRemoteMissionClient(client RemoteMissionClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.remoteClient = client
}
```

- [ ] **Step 4: Apply validation and local trigger skip**

In `Create`, after defaults and before saving:

```go
mission.RunnerType = normalizeMissionRunner(mission.RunnerType)
if err := validateRemoteMission(*mission); err != nil {
	return err
}
```

Wrap local trigger and cron registration:

```go
if mission.RunnerType != MissionRunnerRemote && mission.ExecutionType == ExecutionTriggered {
	m.registerTrigger(mission)
}

if mission.RunnerType != MissionRunnerRemote && mission.ExecutionType == ExecutionScheduled && mission.Schedule != "" {
	// existing local cron registration
}
```

Apply the same `RunnerType != MissionRunnerRemote` condition in `Update`, `Start`, `RunNow`, and all `Notify*` trigger methods so the master never locally queues remote missions from master-only events.

- [ ] **Step 5: Run tests and commit**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/tools -run 'RemoteMission|Mission'
git add internal/tools/missions_v2.go internal/tools/mission_remote.go internal/tools/missions_v2_test.go
git commit -m "feat: add remote mission model"
```

Expected: tests pass.

---

### Task 2: Bridge Protocol for Mission Sync

**Files:**
- Modify: `internal/invasion/bridge/protocol.go`
- Modify: `internal/invasion/bridge/hub.go`
- Modify: `internal/invasion/bridge/client.go`
- Test: `internal/invasion/bridge/protocol_test.go`
- Test: `internal/invasion/bridge/hub_test.go`

- [ ] **Step 1: Write failing bridge protocol tests**

Add to `protocol_test.go`:

```go
func TestMissionSyncPayloadRoundTrip(t *testing.T) {
	key := strings.Repeat("ab", 32)
	payload := MissionSyncPayload{
		Operation: "upsert",
		MissionID: "mission-1",
		Revision: "rev-1",
		MissionJSON: json.RawMessage(`{"id":"mission-1","name":"Remote"}`),
		PromptSnapshot: "base prompt\n\n[Cheat Sheet: \"Docs\"]\ncontent",
	}
	msg, err := NewMessage(MsgMissionSync, "egg-1", "nest-1", key, payload)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	var got MissionSyncPayload
	if err := json.Unmarshal(msg.Payload, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got.Operation != "upsert" || got.MissionID != "mission-1" || !strings.Contains(got.PromptSnapshot, "Cheat Sheet") {
		t.Fatalf("payload roundtrip = %+v", got)
	}
}
```

Add to `hub_test.go`:

```go
func TestSendMissionSyncWaitsForAck(t *testing.T) {
	hub := NewEggHub(testLogger())
	conn, writes := newTestEggConnection(t, "nest-1", "egg-1")
	if err := hub.Register("nest-1", conn); err != nil {
		t.Fatalf("Register: %v", err)
	}
	go func() {
		msg := <-writes
		ack, _ := NewMessage(MsgAck, "egg-1", "nest-1", conn.SharedKey, AckPayload{
			RefID: msg.ID,
			Success: true,
			Detail: "mission synced",
		})
		hub.dispatchAckForTest(*ack)
	}()
	err := hub.SendMissionSync(context.Background(), "nest-1", MissionSyncPayload{
		Operation: "upsert",
		MissionID: "mission-1",
	})
	if err != nil {
		t.Fatalf("SendMissionSync: %v", err)
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/invasion/bridge -run 'MissionSync|SendMissionSync'
```

Expected: build fails because message types, payloads, and ack correlation do not exist.

- [ ] **Step 3: Add protocol payloads**

In `protocol.go`, add constants:

```go
MsgMissionSync   = "mission_sync"
MsgMissionRun    = "mission_run"
MsgMissionDelete = "mission_delete"
MsgMissionResult = "mission_result"
```

Add payloads:

```go
type MissionSyncPayload struct {
	Operation      string          `json:"operation"` // upsert
	MissionID      string          `json:"mission_id"`
	Revision       string          `json:"revision"`
	MissionJSON    json.RawMessage `json:"mission_json"`
	PromptSnapshot string          `json:"prompt_snapshot"`
}

type MissionDeletePayload struct {
	MissionID string `json:"mission_id"`
	Revision  string `json:"revision,omitempty"`
}

type MissionRunPayload struct {
	MissionID   string `json:"mission_id"`
	TriggerType string `json:"trigger_type,omitempty"`
	TriggerData string `json:"trigger_data,omitempty"`
}

type MissionResultPayload struct {
	MissionID string `json:"mission_id"`
	Status    string `json:"status"` // success | error
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
}
```

- [ ] **Step 4: Add ack wait support in hub**

In `EggHub`, add:

```go
pendingAcks map[string]chan AckPayload
```

Initialize it in `NewEggHub`.

Add helper:

```go
func (h *EggHub) sendAndWaitAck(ctx context.Context, nestID, msgType string, payload interface{}) error {
	conn := h.GetConnection(nestID)
	if conn == nil {
		return fmt.Errorf("no active connection for nest %s", nestID)
	}
	msg, err := NewMessage(msgType, conn.EggID, nestID, conn.SharedKey, payload)
	if err != nil {
		return err
	}
	ch := make(chan AckPayload, 1)
	h.mu.Lock()
	h.pendingAcks[msg.ID] = ch
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.pendingAcks, msg.ID)
		h.mu.Unlock()
	}()
	if err := conn.Send(msg); err != nil {
		return err
	}
	select {
	case ack := <-ch:
		if !ack.Success {
			return fmt.Errorf(ack.Detail)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(15 * time.Second):
		return fmt.Errorf("timed out waiting for egg acknowledgement")
	}
}
```

Add public methods:

```go
func (h *EggHub) SendMissionSync(ctx context.Context, nestID string, payload MissionSyncPayload) error {
	return h.sendAndWaitAck(ctx, nestID, MsgMissionSync, payload)
}

func (h *EggHub) SendMissionDelete(ctx context.Context, nestID string, payload MissionDeletePayload) error {
	return h.sendAndWaitAck(ctx, nestID, MsgMissionDelete, payload)
}

func (h *EggHub) SendMissionRun(ctx context.Context, nestID string, payload MissionRunPayload) error {
	return h.sendAndWaitAck(ctx, nestID, MsgMissionRun, payload)
}
```

In `HandleMessages`, change `MsgAck` handling:

```go
var ack AckPayload
if err := json.Unmarshal(msg.Payload, &ack); err == nil {
	h.mu.RLock()
	ch := h.pendingAcks[ack.RefID]
	h.mu.RUnlock()
	if ch != nil {
		ch <- ack
		continue
	}
}
h.logger.Debug("Ack received from egg", "nest_id", conn.NestID, "msg_id", msg.ID)
```

- [ ] **Step 5: Add egg-side callbacks**

In `client.go`, add callbacks:

```go
OnMissionSync   func(payload MissionSyncPayload) error
OnMissionDelete func(payload MissionDeletePayload) error
OnMissionRun    func(payload MissionRunPayload) error
```

In the `readLoop` switch:

```go
case MsgMissionSync:
	var payload MissionSyncPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendAck(msg.ID, false, "invalid mission sync payload")
		continue
	}
	if c.OnMissionSync != nil {
		if err := c.OnMissionSync(payload); err != nil {
			c.sendAck(msg.ID, false, err.Error())
			continue
		}
	}
	c.sendAck(msg.ID, true, "mission synced")
case MsgMissionDelete:
	var payload MissionDeletePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendAck(msg.ID, false, "invalid mission delete payload")
		continue
	}
	if c.OnMissionDelete != nil {
		if err := c.OnMissionDelete(payload); err != nil {
			c.sendAck(msg.ID, false, err.Error())
			continue
		}
	}
	c.sendAck(msg.ID, true, "mission deleted")
case MsgMissionRun:
	var payload MissionRunPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendAck(msg.ID, false, "invalid mission run payload")
		continue
	}
	if c.OnMissionRun != nil {
		if err := c.OnMissionRun(payload); err != nil {
			c.sendAck(msg.ID, false, err.Error())
			continue
		}
	}
	c.sendAck(msg.ID, true, "mission run queued")
```

- [ ] **Step 6: Run tests and commit**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/invasion/bridge
git add internal/invasion/bridge/protocol.go internal/invasion/bridge/hub.go internal/invasion/bridge/client.go internal/invasion/bridge/*_test.go
git commit -m "feat: add mission sync bridge messages"
```

Expected: bridge tests pass.

---

### Task 3: Server Remote Mission Client and Targets API

**Files:**
- Create: `internal/server/mission_remote_sync.go`
- Modify: `internal/server/mission_v2_handlers.go`
- Modify: `internal/server/server_routes_infrastructure.go`
- Test: `internal/server/mission_v2_handlers_test.go`

- [ ] **Step 1: Write failing server tests**

Add tests that prove remote target listing returns connected deployed eggs and create rejects disconnected targets.

```go
func TestMissionRemoteTargetsListsConnectedEggNests(t *testing.T) {
	s := newMissionServerForTest(t)
	nestID := seedInvasionNest(t, s.InvasionDB, invasion.NestRecord{
		Name: "Mini PC",
		EggID: "egg-1",
		Active: true,
		HatchStatus: "running",
	})
	seedInvasionEgg(t, s.InvasionDB, invasion.EggRecord{ID: "egg-1", Name: "Worker Egg", Active: true})
	registerFakeEggConnection(t, s.EggHub, nestID, "egg-1")

	req := httptest.NewRequest(http.MethodGet, "/api/missions/v2/remote-targets", nil)
	rec := httptest.NewRecorder()
	handleMissionRemoteTargets(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Worker Egg") || !strings.Contains(rec.Body.String(), "Mini PC") {
		t.Fatalf("remote targets body = %s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/server -run 'MissionRemoteTargets|RemoteMission'
```

Expected: build fails because handler/client do not exist.

- [ ] **Step 3: Implement remote target DTO and endpoint**

Create `internal/server/mission_remote_sync.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
	"aurago/internal/tools"
)

type missionRemoteTarget struct {
	NestID      string `json:"nest_id"`
	NestName    string `json:"nest_name"`
	EggID       string `json:"egg_id"`
	EggName     string `json:"egg_name"`
	Connected   bool   `json:"connected"`
	HatchStatus string `json:"hatch_status"`
	Label       string `json:"label"`
}

func handleMissionRemoteTargets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		targets, err := listMissionRemoteTargets(s)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to load remote targets", "Failed to list mission remote targets", err)
			return
		}
		writeJSON(w, map[string]interface{}{"targets": targets})
	}
}

func listMissionRemoteTargets(s *Server) ([]missionRemoteTarget, error) {
	if s.InvasionDB == nil {
		return []missionRemoteTarget{}, nil
	}
	nests, err := invasion.ListNests(s.InvasionDB)
	if err != nil {
		return nil, err
	}
	eggs, err := invasion.ListEggs(s.InvasionDB)
	if err != nil {
		return nil, err
	}
	eggNames := map[string]string{}
	activeEggs := map[string]bool{}
	for _, egg := range eggs {
		eggNames[egg.ID] = egg.Name
		activeEggs[egg.ID] = egg.Active
	}
	out := []missionRemoteTarget{}
	for _, nest := range nests {
		if nest.EggID == "" || !nest.Active || !activeEggs[nest.EggID] {
			continue
		}
		connected := s.EggHub != nil && s.EggHub.IsConnected(nest.ID)
		eggName := eggNames[nest.EggID]
		label := fmt.Sprintf("%s on %s", eggName, nest.Name)
		out = append(out, missionRemoteTarget{
			NestID: nest.ID, NestName: nest.Name,
			EggID: nest.EggID, EggName: eggName,
			Connected: connected, HatchStatus: nest.HatchStatus,
			Label: label,
		})
	}
	return out, nil
}
```

- [ ] **Step 4: Implement server remote client**

In the same file:

```go
type serverRemoteMissionClient struct {
	server *Server
}

func (c serverRemoteMissionClient) SyncMission(ctx context.Context, mission tools.MissionV2, promptSnapshot string) error {
	if c.server.EggHub == nil {
		return fmt.Errorf("egg hub is not available")
	}
	mission.Prompt = promptSnapshot
	mission.CheatsheetIDs = nil
	raw, err := json.Marshal(mission)
	if err != nil {
		return err
	}
	payload := bridge.MissionSyncPayload{
		Operation: "upsert",
		MissionID: mission.ID,
		Revision: mission.RemoteRevision,
		MissionJSON: raw,
		PromptSnapshot: promptSnapshot,
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return c.server.EggHub.SendMissionSync(ctx, mission.RemoteNestID, payload)
}

func (c serverRemoteMissionClient) DeleteMission(ctx context.Context, mission tools.MissionV2) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return c.server.EggHub.SendMissionDelete(ctx, mission.RemoteNestID, bridge.MissionDeletePayload{MissionID: mission.ID})
}

func (c serverRemoteMissionClient) RunMission(ctx context.Context, mission tools.MissionV2, triggerType, triggerData string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return c.server.EggHub.SendMissionRun(ctx, mission.RemoteNestID, bridge.MissionRunPayload{
		MissionID: mission.ID,
		TriggerType: triggerType,
		TriggerData: triggerData,
	})
}
```

Register the client during server start after `MissionManagerV2` is initialized:

```go
s.MissionManagerV2.SetRemoteMissionClient(serverRemoteMissionClient{server: s})
```

Register route:

```go
mux.HandleFunc("/api/missions/v2/remote-targets", handleMissionRemoteTargets(s))
```

- [ ] **Step 5: Build remote prompt snapshot**

Add in `internal/tools/mission_remote.go`:

```go
func BuildRemoteMissionPromptSnapshot(db *sql.DB, mission MissionV2) string {
	prompt := mission.Prompt
	if db != nil && len(mission.CheatsheetIDs) > 0 {
		prompt += CheatsheetGetMultiple(db, mission.CheatsheetIDs)
	}
	return prompt
}
```

Use it when calling `remoteClient.SyncMission`.

- [ ] **Step 6: Run tests and commit**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/server -run 'MissionRemoteTargets|RemoteMission'
go test -count=1 ./internal/tools -run 'RemoteMission|Cheatsheet'
git add internal/server/mission_remote_sync.go internal/server/mission_v2_handlers.go internal/server/server_routes_infrastructure.go internal/tools/mission_remote.go internal/server/*_test.go internal/tools/*_test.go
git commit -m "feat: sync remote missions to eggs"
```

Expected: server and tools tests pass.

---

### Task 4: Remote-Aware Create, Update, Delete, and Run

**Files:**
- Modify: `internal/tools/missions_v2.go`
- Modify: `internal/server/mission_v2_handlers.go`
- Test: `internal/tools/missions_v2_test.go`
- Test: `internal/server/mission_v2_handlers_test.go`

- [ ] **Step 1: Write failing lifecycle tests**

Add tests:

```go
func TestRemoteMissionCreateSyncsBeforeSave(t *testing.T) {
	client := &recordingRemoteMissionClient{}
	mgr := NewMissionManagerV2(t.TempDir(), NewCronManager())
	mgr.SetRemoteMissionClient(client)

	err := mgr.Create(&MissionV2{
		Name: "Remote",
		Prompt: "do work",
		RunnerType: MissionRunnerRemote,
		RemoteNestID: "nest-1",
		RemoteEggID: "egg-1",
	})
	if err != nil {
		t.Fatalf("Create remote: %v", err)
	}
	if len(client.synced) != 1 || client.synced[0].ID == "" {
		t.Fatalf("synced = %+v, want one mission with generated ID", client.synced)
	}
}

func TestRemoteMissionDeleteFailsIfRemoteDeleteFails(t *testing.T) {
	client := &recordingRemoteMissionClient{deleteErr: errors.New("offline")}
	mgr := NewMissionManagerV2(t.TempDir(), NewCronManager())
	mgr.SetRemoteMissionClient(client)
	mission := &MissionV2{Name: "Remote", Prompt: "do work", RunnerType: MissionRunnerRemote, RemoteNestID: "nest-1", RemoteEggID: "egg-1"}
	if err := mgr.Create(mission); err != nil {
		t.Fatalf("Create remote: %v", err)
	}
	err := mgr.Delete(mission.ID)
	if err == nil || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("Delete error = %v, want remote failure", err)
	}
	if _, ok := mgr.Get(mission.ID); !ok {
		t.Fatal("mission deleted locally despite remote delete failure")
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/tools -run 'RemoteMissionCreateSyncs|RemoteMissionDeleteFails'
```

Expected: failing behavior until lifecycle methods call the remote client.

- [ ] **Step 3: Implement remote lifecycle**

In `Create`:

```go
if mission.RunnerType == MissionRunnerRemote {
	if m.remoteClient == nil {
		return fmt.Errorf("remote mission client is not available")
	}
	mission.RemoteRevision = fmt.Sprintf("%d", time.Now().UnixNano())
	promptSnapshot := BuildRemoteMissionPromptSnapshot(m.cheatsheetDB, *mission)
	if err := m.remoteClient.SyncMission(context.Background(), *mission, promptSnapshot); err != nil {
		mission.RemoteSyncStatus = "error"
		mission.RemoteSyncError = err.Error()
		return err
	}
	mission.RemoteSyncStatus = "synced"
	mission.RemoteSyncError = ""
}
```

In `Update`, preserve remote metadata and sync before replacing the stored mission:

```go
if updated.RunnerType == MissionRunnerRemote {
	if m.remoteClient == nil {
		return fmt.Errorf("remote mission client is not available")
	}
	updated.RemoteRevision = fmt.Sprintf("%d", time.Now().UnixNano())
	promptSnapshot := BuildRemoteMissionPromptSnapshot(m.cheatsheetDB, *updated)
	if err := m.remoteClient.SyncMission(context.Background(), *updated, promptSnapshot); err != nil {
		return err
	}
	updated.RemoteSyncStatus = "synced"
	updated.RemoteSyncError = ""
}
```

In `Delete`, remote-delete first:

```go
if mission.RunnerType == MissionRunnerRemote {
	if m.remoteClient == nil {
		return fmt.Errorf("remote mission client is not available")
	}
	if err := m.remoteClient.DeleteMission(context.Background(), *mission); err != nil {
		return err
	}
}
```

In `RunNow`, remote-run instead of local queue:

```go
if mission.RunnerType == MissionRunnerRemote {
	if m.remoteClient == nil {
		return fmt.Errorf("remote mission client is not available")
	}
	if err := m.remoteClient.RunMission(context.Background(), *mission, "manual", ""); err != nil {
		return err
	}
	mission.Status = MissionStatusQueued
	m.save()
	return nil
}
```

- [ ] **Step 4: Run tests and commit**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/tools -run 'RemoteMission|Mission'
go test -count=1 ./internal/server -run 'RemoteMission'
git add internal/tools/missions_v2.go internal/tools/mission_remote.go internal/tools/missions_v2_test.go internal/server/mission_v2_handlers.go internal/server/mission_v2_handlers_test.go
git commit -m "feat: make mission lifecycle remote-aware"
```

Expected: remote lifecycle tests pass.

---

### Task 5: Egg Applies Remote Missions

**Files:**
- Modify: `cmd/aurago/main.go`
- Modify: `internal/tools/missions_v2.go`
- Test: `internal/tools/missions_v2_test.go`

- [ ] **Step 1: Write failing apply tests**

Add tests:

```go
func TestApplyRemoteMissionUpsertStoresMissionEnabled(t *testing.T) {
	mgr := NewMissionManagerV2(t.TempDir(), NewCronManager())
	raw := []byte(`{
		"id":"mission-remote-1",
		"name":"Remote synced",
		"prompt":"synced prompt",
		"execution_type":"manual",
		"runner_type":"remote",
		"enabled":true
	}`)
	if err := mgr.ApplyRemoteMissionJSON(raw); err != nil {
		t.Fatalf("ApplyRemoteMissionJSON: %v", err)
	}
	got, ok := mgr.Get("mission-remote-1")
	if !ok {
		t.Fatal("remote mission not stored")
	}
	if !got.Enabled || got.Prompt != "synced prompt" {
		t.Fatalf("mission = %+v", got)
	}
}

func TestDeleteRemoteMissionRemovesMission(t *testing.T) {
	mgr := NewMissionManagerV2(t.TempDir(), NewCronManager())
	_ = mgr.ApplyRemoteMissionJSON([]byte(`{"id":"mission-remote-1","name":"Remote","prompt":"p","execution_type":"manual","enabled":true}`))
	if err := mgr.DeleteRemoteMission("mission-remote-1"); err != nil {
		t.Fatalf("DeleteRemoteMission: %v", err)
	}
	if _, ok := mgr.Get("mission-remote-1"); ok {
		t.Fatal("remote mission still exists")
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/tools -run 'ApplyRemoteMission|DeleteRemoteMission'
```

Expected: methods do not exist.

- [ ] **Step 3: Implement apply/delete/run helpers**

In `internal/tools/missions_v2.go`:

```go
func (m *MissionManagerV2) ApplyRemoteMissionJSON(raw []byte) error {
	var mission MissionV2
	if err := json.Unmarshal(raw, &mission); err != nil {
		return fmt.Errorf("invalid remote mission: %w", err)
	}
	if mission.ID == "" || mission.Name == "" || mission.Prompt == "" {
		return fmt.Errorf("remote mission requires id, name, and prompt")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	mission.Status = MissionStatusIdle
	mission.RunnerType = MissionRunnerLocal
	m.missions[mission.ID] = &mission
	if mission.Enabled {
		if mission.ExecutionType == ExecutionTriggered {
			m.registerTrigger(&mission)
		} else if mission.ExecutionType == ExecutionScheduled && mission.Schedule != "" && m.cron != nil {
			cronID := "mission_" + mission.ID
			if _, err := m.cron.ManageScheduleWithSource("add", cronID, mission.Schedule, mission.Prompt, "", "mission"); err != nil {
				return err
			}
		}
	}
	return m.save()
}

func (m *MissionManagerV2) DeleteRemoteMission(id string) error {
	return m.Delete(id)
}

func (m *MissionManagerV2) RunRemoteMission(id, triggerType, triggerData string) error {
	return m.TriggerMission(id, triggerType, triggerData)
}
```

- [ ] **Step 4: Wire egg client callbacks in `cmd/aurago/main.go`**

Inside the `cfg.EggMode.Enabled` block:

```go
eggClient.OnMissionSync = func(payload bridge.MissionSyncPayload) error {
	if strings.TrimSpace(string(payload.MissionJSON)) == "" {
		return fmt.Errorf("mission_json is required")
	}
	return srv.MissionManagerV2.ApplyRemoteMissionJSON(payload.MissionJSON)
}

eggClient.OnMissionDelete = func(payload bridge.MissionDeletePayload) error {
	if payload.MissionID == "" {
		return fmt.Errorf("mission_id is required")
	}
	return srv.MissionManagerV2.DeleteRemoteMission(payload.MissionID)
}

eggClient.OnMissionRun = func(payload bridge.MissionRunPayload) error {
	if payload.MissionID == "" {
		return fmt.Errorf("mission_id is required")
	}
	return srv.MissionManagerV2.RunRemoteMission(payload.MissionID, payload.TriggerType, payload.TriggerData)
}
```

If `srv` is not in scope at that point, move egg-client setup after server initialization or pass the initialized `MissionManagerV2` reference into a helper function.

- [ ] **Step 5: Send remote mission results back to master**

When the egg's `MissionManagerV2` completes a remotely synced mission, send `MsgMissionResult` to the master. Implement this by adding an optional result callback to `MissionManagerV2`:

```go
remoteResultCallback func(missionID, result, output string)
```

Call it at the end of `OnMissionComplete` after saving. In egg mode, set it to:

```go
eggClient.SendMissionResult(bridge.MissionResultPayload{
	MissionID: missionID,
	Status: result,
	Output: output,
})
```

On the master `EggHub`, add `OnMissionResult func(nestID string, result MissionResultPayload)` and update the local remote mission status/result when received.

- [ ] **Step 6: Run tests and commit**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/tools -run 'ApplyRemoteMission|DeleteRemoteMission|RemoteMission'
go test -count=1 -run '^$' ./cmd/aurago
git add cmd/aurago/main.go internal/tools/missions_v2.go internal/tools/missions_v2_test.go
git commit -m "feat: apply synced missions on eggs"
```

Expected: tools tests pass; `cmd/aurago` compiles.

---

### Task 6: Mission UI Target Selector, Trigger Gating, and Larger Prompt Field

**Files:**
- Modify: `ui/missions_v2.html`
- Modify: `ui/js/missions/main.js`
- Modify: `ui/css/missions.css`
- Modify: `ui/lang/missions/*.json`

- [ ] **Step 1: Add target selector markup**

In `ui/missions_v2.html`, add after mission name and before prompt:

```html
<div class="form-group">
    <label data-i18n="missions.form_runner_label">Run on</label>
    <div class="runner-type-selector">
        <label class="runner-type-option" data-runner-type="local">
            <input type="radio" name="runner-type" value="local" checked>
            <span class="runner-icon">⌂</span>
            <span class="runner-label" data-i18n="missions.form_runner_local_label">Local AuraGo</span>
            <span class="runner-desc" data-i18n="missions.form_runner_local_desc">Run on this instance.</span>
        </label>
        <label class="runner-type-option" data-runner-type="remote">
            <input type="radio" name="runner-type" value="remote">
            <span class="runner-icon">⇄</span>
            <span class="runner-label" data-i18n="missions.form_runner_remote_label">Remote egg</span>
            <span class="runner-desc" data-i18n="missions.form_runner_remote_desc">Send to a deployed egg.</span>
        </label>
    </div>
</div>

<div id="remote-target-config" class="trigger-config is-hidden">
    <div class="form-group">
        <label for="remote-target-select" data-i18n="missions.form_remote_target_label">Egg</label>
        <select id="remote-target-select" class="form-select">
            <option value="" data-i18n="missions.form_remote_target_loading">Loading eggs...</option>
        </select>
        <div class="form-hint" data-i18n="missions.form_remote_target_hint">Only connected eggs can receive remote missions.</div>
    </div>
</div>
```

- [ ] **Step 2: Add UI state and loading**

In `ui/js/missions/main.js`, add:

```js
let remoteTargets = [];

async function loadRemoteTargets(selectedNestId = '') {
    const select = document.getElementById('remote-target-select');
    if (!select) return;
    try {
        const resp = await fetch('/api/missions/v2/remote-targets');
        if (!resp.ok) throw new Error(await resp.text());
        const data = await resp.json();
        remoteTargets = data.targets || [];
        const connected = remoteTargets.filter(t => t.connected);
        select.innerHTML = connected.length === 0
            ? `<option value="">${t('missions.form_remote_target_none')}</option>`
            : connected.map(target => `<option value="${escapeAttr(target.nest_id)}" data-egg-id="${escapeAttr(target.egg_id)}" data-nest-name="${escapeAttr(target.nest_name)}" data-egg-name="${escapeAttr(target.egg_name)}">${escapeHtml(target.label)}</option>`).join('');
        select.value = selectedNestId || '';
    } catch (err) {
        select.innerHTML = `<option value="">${t('missions.form_remote_target_unavailable')}</option>`;
        remoteTargets = [];
    }
}
```

Add event binding in `bindMissionUI`:

```js
document.querySelectorAll('input[name="runner-type"]').forEach(input => {
    input.addEventListener('change', () => selectRunnerType(input.value));
});
```

Add:

```js
function selectRunnerType(type) {
    document.querySelectorAll('.runner-type-option').forEach(opt => {
        const input = opt.querySelector('input');
        input.checked = input.value === type;
    });
    document.getElementById('remote-target-config')?.classList.toggle('is-hidden', type !== 'remote');
    updateRemoteTriggerAvailability();
}

function updateRemoteTriggerAvailability() {
    const remote = document.querySelector('input[name="runner-type"]:checked')?.value === 'remote';
    const allowedRemoteTriggers = new Set(['system_startup', 'mqtt_message']);
    document.querySelectorAll('.trigger-type-btn[data-trigger]').forEach(btn => {
        const allowed = !remote || allowedRemoteTriggers.has(btn.dataset.trigger);
        btn.disabled = !allowed;
        btn.classList.toggle('is-disabled', !allowed);
        if (!allowed && btn.classList.contains('active')) {
            btn.classList.remove('active');
        }
    });
}
```

- [ ] **Step 3: Save and edit remote fields**

In `openMissionModal`, set:

```js
selectRunnerType(mission.runner_type || 'local');
loadRemoteTargets(mission.remote_nest_id || '');
```

For new missions:

```js
selectRunnerType('local');
loadRemoteTargets('');
```

In `saveMission`, add:

```js
const runnerType = document.querySelector('input[name="runner-type"]:checked')?.value || 'local';
mission.runner_type = runnerType;
if (runnerType === 'remote') {
    const targetSelect = document.getElementById('remote-target-select');
    const opt = targetSelect.options[targetSelect.selectedIndex];
    if (!targetSelect.value) {
        showToast(t('missions.toast_select_remote_target'), 'error');
        return;
    }
    mission.remote_nest_id = targetSelect.value;
    mission.remote_egg_id = opt?.dataset?.eggId || '';
    mission.remote_nest_name = opt?.dataset?.nestName || '';
    mission.remote_egg_name = opt?.dataset?.eggName || '';
}
```

Before accepting triggered remote missions:

```js
if (runnerType === 'remote' && execType === 'triggered') {
    const allowedRemoteTriggers = new Set(['system_startup', 'mqtt_message']);
    if (!allowedRemoteTriggers.has(triggerType)) {
        showToast(t('missions.toast_remote_trigger_unsupported'), 'error');
        return;
    }
}
```

- [ ] **Step 4: Make prompt field bigger**

In `ui/css/missions.css`:

```css
#modal .modal {
    max-width: min(940px, calc(100vw - 32px));
}

#mission-prompt {
    min-height: 260px;
    resize: vertical;
    line-height: 1.5;
}

.runner-type-selector {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 10px;
}

.runner-type-option {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px;
    background: var(--bg-glass);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    cursor: pointer;
}

.runner-type-option:has(input:checked) {
    border-color: var(--accent);
    background: rgba(45, 212, 191, 0.08);
}

.trigger-type-btn.is-disabled,
.trigger-type-btn:disabled {
    opacity: 0.42;
    cursor: not-allowed;
}
```

- [ ] **Step 5: Add translations to all mission language files**

Add these keys to every file in `ui/lang/missions/`:

```json
"missions.form_runner_label": "Run on",
"missions.form_runner_local_label": "Local AuraGo",
"missions.form_runner_local_desc": "Run on this instance.",
"missions.form_runner_remote_label": "Remote egg",
"missions.form_runner_remote_desc": "Send to a deployed egg.",
"missions.form_remote_target_label": "Egg",
"missions.form_remote_target_loading": "Loading eggs...",
"missions.form_remote_target_none": "No connected eggs available",
"missions.form_remote_target_unavailable": "Egg list unavailable",
"missions.form_remote_target_hint": "Only connected eggs can receive remote missions.",
"missions.toast_select_remote_target": "Select a remote egg first.",
"missions.toast_remote_trigger_unsupported": "This trigger is not available for remote missions."
```

Use real translations for all supported languages: `cs`, `da`, `de`, `el`, `en`, `es`, `fr`, `hi`, `it`, `ja`, `nl`, `no`, `pl`, `pt`, `sv`, `zh`.

- [ ] **Step 6: Run UI/i18n checks and commit**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./ui -run 'TestTranslations'
git add ui/missions_v2.html ui/js/missions/main.js ui/css/missions.css ui/lang/missions
git commit -m "feat: add remote mission target controls"
```

Expected: translation lint passes.

---

### Task 7: Remote Trigger and Result Integration

**Files:**
- Modify: `internal/tools/missions_v2.go`
- Modify: `internal/invasion/bridge/hub.go`
- Modify: `internal/server/server_routes_infrastructure.go`
- Modify: `internal/server/mission_remote_sync.go`
- Test: `internal/tools/missions_v2_test.go`
- Test: `internal/invasion/bridge/hub_test.go`

- [ ] **Step 1: Write failing result update test**

```go
func TestRemoteMissionResultUpdatesMasterMission(t *testing.T) {
	mgr := NewMissionManagerV2(t.TempDir(), NewCronManager())
	mgr.SetRemoteMissionClient(&recordingRemoteMissionClient{syncOK: true})
	m := &MissionV2{
		Name: "Remote",
		Prompt: "p",
		RunnerType: MissionRunnerRemote,
		RemoteNestID: "nest-1",
		RemoteEggID: "egg-1",
	}
	if err := mgr.Create(m); err != nil {
		t.Fatalf("Create: %v", err)
	}
	mgr.ApplyRemoteMissionResult(m.ID, "success", "done")
	got, _ := mgr.Get(m.ID)
	if got.LastResult != "success" || got.LastOutput != "done" || got.Status != MissionStatusIdle {
		t.Fatalf("mission after result = %+v", got)
	}
}
```

- [ ] **Step 2: Implement master result application**

In `MissionManagerV2`:

```go
func (m *MissionManagerV2) ApplyRemoteMissionResult(missionID, result, output string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mission, ok := m.missions[missionID]
	if !ok {
		return
	}
	mission.Status = MissionStatusIdle
	mission.LastResult = result
	mission.LastOutput = truncateString(output, 500)
	mission.RunCount++
	mission.LastRun = time.Now()
	m.save()
}
```

In `EggHub.HandleMessages`, handle `MsgMissionResult` and call `OnMissionResult`.

In route setup:

```go
s.EggHub.OnMissionResult = func(nestID string, result bridge.MissionResultPayload) {
	output := result.Output
	status := result.Status
	if status == "" && result.Error != "" {
		status = tools.MissionResultError
		output = result.Error
	}
	s.MissionManagerV2.ApplyRemoteMissionResult(result.MissionID, status, output)
	broadcastMissionState(s)
}
```

- [ ] **Step 3: Run tests and commit**

Run:

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/tools -run 'RemoteMissionResult|RemoteMission'
go test -count=1 ./internal/invasion/bridge -run 'MissionResult|MissionSync'
git add internal/tools/missions_v2.go internal/invasion/bridge/hub.go internal/server/server_routes_infrastructure.go internal/server/mission_remote_sync.go internal/tools/*_test.go internal/invasion/bridge/*_test.go
git commit -m "feat: report remote mission results"
```

Expected: tests pass and master UI can update from remote results.

---

### Task 8: End-to-End Verification

**Files:**
- No new files unless failures require fixes.

- [ ] **Step 1: Run targeted backend tests**

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./internal/tools ./internal/invasion ./internal/invasion/bridge ./internal/server
```

Expected: all pass.

- [ ] **Step 2: Run UI translation tests**

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 ./ui
```

Expected: all translation files contain the new keys and no nested JSON values.

- [ ] **Step 3: Run full compile smoke test**

```powershell
$env:GOCACHE=(Resolve-Path 'disposable\go-cache').Path
go test -count=1 -run '^$' ./cmd/aurago ./internal/server ./internal/tools ./internal/invasion/bridge
```

Expected: all packages compile.

- [ ] **Step 4: Manual UI flow**

1. Start AuraGo locally.
2. Open Mission Control V2.
3. Create a mission with `Run on: Local AuraGo`; verify existing behavior is unchanged.
4. Open the modal again; verify `#mission-prompt` is visibly larger and resizable.
5. Create or use an active deployed egg in Invasion Control.
6. Create a mission with `Run on: Remote egg`.
7. Verify the remote egg selector lists connected deployed eggs as `Egg Name on Nest Name`.
8. Select `Triggered` and verify unsupported trigger buttons are disabled.
9. Save the mission and confirm it appears in the local UI with remote metadata.
10. Check logs for `mission synced` acknowledgement.
11. Run the mission manually and confirm the egg receives and queues it.
12. Delete the mission and confirm the local UI removes it only after the egg acknowledges deletion.

- [ ] **Step 5: Final commit if verification fixes were required**

```powershell
git status --short
git add <only-files-changed-for-fixes>
git commit -m "fix: stabilize remote mission verification"
```

Expected: no unrelated files staged.

---

## Risk Notes

- The selected remote target should be stored by `nest_id`, not only by `egg_id`, because the WebSocket bridge routes to nests and the same egg template can be deployed to multiple nests.
- Remote mission sync should fail clearly when the egg is not connected. A future enhancement can add pending remote sync, but the first implementation should avoid pretending activation succeeded.
- Existing cheatsheet attachments are text-only and already have a 25k-character limit per cheatsheet. Sending the resolved prompt snapshot is the lowest-risk way to include attachments without creating a new mission attachment storage model.
- Direct mission file attachments are not present in the current Mission UI. If that UX is desired later, add a separate mission attachment table/API rather than overloading cheatsheet attachments.
- Result delivery needs a new mission result bridge message; otherwise the master UI will not know what happened on the egg.

## Self-Review

- Spec coverage: local vs remote selection, egg list, create/update sync, delete sync, attachment transfer via cheatsheet attachment prompt snapshot, remote trigger disabling, and larger prompt field are all covered.
- Placeholder scan: no `TODO`, `TBD`, or unspecified test steps remain.
- Type consistency: `RunnerType`, `MissionRunnerRemote`, `RemoteNestID`, `RemoteEggID`, and bridge payload names are used consistently across tasks.

