# Mission Trigger System Fix Plan - 2026-05-01

## Context

This plan reviews `reports/mission_trigger_system_audit_2026-04-30.md` against the current codebase and turns it into an implementation sequence. The audit file itself stays in `reports/` and must not be committed.

The current mission trigger core lives mainly in:

- `internal/tools/missions_v2.go`
- `internal/tools/mission_remote.go`
- `internal/tools/email_watcher.go`
- `internal/server/server.go`
- `internal/server/server_routes.go`
- `internal/server/mission_adapters.go`
- `internal/server/mission_v2_handlers.go`
- `ui/missions_v2.html`
- `ui/js/missions/main.js`
- `ui/lang/missions/*.json`
- `prompts/tools_manuals/manage_missions.md`

## Audit Validation

Confirmed current defects:

- `OnMissionComplete` still has the double-unlock crash path. It unlocks explicitly on the double-completion guard and later has `defer m.mu.Unlock()`.
- Email-triggered missions are still not functional after restart. `StartEmailWatcher` returns a real `*tools.EmailWatcher`, but `server_routes.go` discards it and never calls `SetEmailWatcher`.
- Email needs more than simple wiring because `MissionManagerV2.Start()` scans triggers before `server_routes.go` starts the EmailWatcher. Late manager wiring must trigger a safe rescan.
- `NotifyInvasionEvent` still persists mission metadata but not the queue file.
- `activeRunID` is still deleted only when `historyDB != nil`.
- `processQueue` still recovers from `processNext` panics without clearing the queue's `running` slot.
- Remote mission run timeout is still 40 minutes while remote run RPC calls use 20 seconds.
- `loadQueueLocked` still changes mission statuses to queued without persisting those metadata changes immediately.

Partially stale or already mitigated audit points:

- Webhook and MQTT managers are currently set before `MissionManagerV2.Start()` in `server.go`, so their restart registration ordering bug appears fixed in this branch.
- MQTT already has per-trigger and config fallback minimum interval support through `TriggerConfig.MQTTMinIntervalSeconds` and `config.mqtt.trigger_min_interval_seconds`.
- `OnMissionComplete` has already been partly optimized versus the report by deferring mission saves until later, but it still writes queue state twice in dependent-trigger paths.

## Recommended Approach

Use a three-wave implementation.

Wave 1 stabilizes the existing trigger runtime without changing user-facing trigger types. This should land first because it reduces crash and data-loss risk and gives later trigger integrations a safer base.

Wave 2 adds a generic trigger rate-limit field and idempotent registration behavior. This keeps webhook, email, MQTT, and later integration triggers from accumulating duplicate callbacks or producing event storms.

Wave 3 adds missing trigger integrations in small vertical slices. Each trigger slice should include backend event constants, filtering fields, notify method, source integration hook, REST/UI support, all 15 mission translations, prompt manual updates, and tests.

## Wave 1: Runtime Safety Fixes

### 1. Fix `OnMissionComplete` locking and history cleanup

Files:

- Modify `internal/tools/missions_v2.go`
- Modify `internal/tools/missions_v2_test.go`

Implementation:

- Move `defer m.mu.Unlock()` immediately after `m.mu.Lock()`.
- Remove the explicit `m.mu.Unlock()` before the double-completion early return.
- Split active run cleanup into two steps:
  - Always `delete(m.activeRunID, missionID)` when a run ID exists.
  - Only write to history when `historyDB != nil`.
- Add a test that calls `OnMissionComplete` twice for the same running mission and verifies no panic.
- Add a test that seeds `activeRunID` with nil history DB, completes the mission, and verifies the map entry is removed.

### 2. Persist invasion-triggered queue state

Files:

- Modify `internal/tools/missions_v2.go`
- Modify `internal/tools/missions_v2_test.go`

Implementation:

- Track whether `NotifyInvasionEvent` queued at least one mission.
- Only save when something was queued.
- Call both `m.save()` and `m.saveQueueLocked()` when queued, matching device, Fritz!Box, budget, and Home Assistant behavior.
- Add a test that queues an invasion event, restarts a fresh manager against the same temp data directory, and verifies the queued mission is restored.

### 3. Make queue panic recovery release `running`

Files:

- Modify `internal/tools/missions_v2.go`
- Modify `internal/tools/missions_v2_test.go`

Implementation:

- Move the panic recovery responsibility into `processNext` or wrap the body after `TryStartNext`.
- After a mission item is claimed, a panic before callback dispatch must:
  - set the mission status back to `idle` or `queued` based on whether it should retry,
  - call `m.queue.Done()`,
  - persist mission metadata and queue state,
  - log the panic with the mission ID.
- Prefer marking the mission `idle` with `LastResult=error` and `LastOutput="mission dispatch panic: ..."` to avoid an infinite crash loop.
- Add a test with a deliberately panic-inducing hook. If no hook is practical, extract a small `dispatchQueuedMission(item QueueItem)` method and unit-test panic recovery around it.

### 4. Fix queue/status persistence on startup restore

Files:

- Modify `internal/tools/missions_v2.go`
- Modify `internal/tools/missions_v2_test.go`

Implementation:

- Have `loadQueueLocked` return `(statusChanged bool, err error)` or set a local flag in `Start()`.
- If restored queue entries change mission statuses to `queued`, persist `missions_v2.json` before returning from `Start()`.
- Keep queue persistence as it is, but make metadata persistence explicit.

### 5. Persist remote sync pending state on partial failures

Files:

- Modify `internal/tools/missions_v2.go`
- Modify `internal/tools/missions_v2_test.go`

Implementation:

- Keep `SyncRemoteMissionsForNest` saving when `attempted > 0`, even if `firstErr != nil`.
- If saving also fails and there is already a remote sync error, log the save error and return the original remote error.
- Add a test for a temporary remote sync error that verifies `RemoteSyncStatus=pending` survives reload.

### 6. Reduce remote mission result timeout

Files:

- Modify `internal/tools/mission_remote.go`
- Modify or add tests in `internal/tools/missions_v2_test.go`

Implementation:

- Change `remoteMissionResultTimeout` from `40 * time.Minute` to `3 * time.Minute`.
- Keep it as a package variable so tests can override it.
- Update timeout-related tests if they assert the old duration.
- Add an inline comment explaining that the result timeout is longer than the 20-second run ACK timeout but short enough to surface dead eggs quickly.

## Wave 2: Trigger Registration and Storm Control

### 7. Add idempotent trigger registration

Files:

- Modify `internal/tools/missions_v2.go`
- Modify `internal/tools/missions_v2_test.go`

Implementation:

- Add a mission-manager-owned `registeredTriggerKeys map[string]string`.
- Compute a stable registration key from mission ID, trigger type, and trigger config fields used by external callback registries.
- Before registering email, webhook, or MQTT callbacks, skip registration when the key is unchanged.
- When mission trigger config changes, update the key and register the new callback. Existing old callbacks can remain in external registries because `triggerRegistrationIsCurrent` already ignores stale callbacks.
- Add tests for update-with-same-config to prove duplicate current callbacks are not added.

### 8. Make late external manager wiring rescan safely

Files:

- Modify `internal/tools/missions_v2.go`
- Modify `internal/server/server_routes.go`
- Modify `internal/server/mission_adapters.go`
- Modify `internal/tools/missions_v2_test.go`

Implementation:

- Add `RescanTriggers()` or have `SetEmailWatcher`, `SetWebhookManager`, and `SetMQTTManager` call a shared locked `setupTriggers()` after assigning the manager.
- Use idempotent registration from Task 7 so rescans are safe.
- In `server_routes.go`, capture `emailWatcher := tools.StartEmailWatcher(...)`.
- If `emailWatcher != nil`, call `s.MissionManagerV2.SetEmailWatcher(emailWatcher)`.
- Remove `dummyEmailWatcher` from `mission_adapters.go`; the real `*tools.EmailWatcher` satisfies `tools.EmailWatcherInterface`.
- Add a test that starts a manager with an email-triggered mission and then sets a fake watcher afterward; the fake watcher must receive the registration.

### 9. Add generic per-mission trigger minimum interval

Files:

- Modify `internal/tools/missions_v2.go`
- Modify `internal/server/mission_v2_handlers.go`
- Modify `ui/missions_v2.html`
- Modify `ui/js/missions/main.js`
- Modify all `ui/lang/missions/*.json`
- Modify `prompts/tools_manuals/manage_missions.md`
- Modify tests in `internal/tools/missions_v2_test.go`

Implementation:

- Add `MinIntervalSeconds int json:"min_interval_seconds,omitempty"` to `TriggerConfig`.
- Apply it consistently in mission manager notify paths: mission completed, email, webhook, invasion, device, Fritz!Box, budget, and Home Assistant.
- Keep `MQTTMinIntervalSeconds` for backward compatibility. For MQTT, use the specific field when set, otherwise fall back to the generic field, then to config fallback.
- Store last fire time in memory as `lastTriggerFire map[string]time.Time` keyed by mission ID and trigger type.
- Do not persist last-fire state in Wave 2; restart reset is acceptable for a debounce feature.
- UI: add a compact "Minimum interval" numeric field common to all triggered missions, keeping the existing MQTT-specific field temporarily labeled as MQTT override or migrating it into the generic field.
- Translations: add labels and hints for all 15 mission language files.

## Wave 3: Missing Trigger Integration Slices

The audit lists many possible triggers. Implement them in this order, because these have clear existing event sources and high home-lab value.

### 10. Planner triggers

Trigger types:

- `planner_appointment_due`
- `planner_todo_overdue`
- `planner_operational_issue`

Files:

- Modify `internal/tools/missions_v2.go`
- Modify `internal/planner/notifier.go`
- Modify `internal/planner/operational_issues.go` or call from server-side issue recording points
- Modify `internal/server/server_routes.go`
- Modify `ui/missions_v2.html`
- Modify `ui/js/missions/main.js`
- Modify all `ui/lang/missions/*.json`
- Modify `prompts/tools_manuals/manage_missions.md`

Implementation:

- Add trigger config filters for appointment ID/title contains, todo ID/title contains, issue source/severity.
- Extend planner notifier so due appointments can trigger missions in addition to existing `wake_agent` loopback behavior.
- Add a small overdue todo scanner using the existing planner DB, or integrate with the existing planner summary path if there is already a scheduled loop.
- Fire operational issue triggers from `planner.RecordOperationalIssue` call sites, preferably through a server-level notifier to avoid making the planner package depend on tools.

### 11. Docker triggers

Trigger types:

- `docker_container_started`
- `docker_container_stopped`
- `docker_container_unhealthy`

Files:

- Modify `internal/tools/missions_v2.go`
- Add a small Docker event poller/runtime file near existing Docker tooling or server services.
- Modify `internal/server/server.go`
- Modify `ui/missions_v2.html`
- Modify `ui/js/missions/main.js`
- Modify all `ui/lang/missions/*.json`
- Modify `prompts/tools_manuals/manage_missions.md`

Implementation:

- Use Docker event stream or periodic inspect based on the existing Docker tool capabilities.
- Add filters for container ID/name and event type.
- Respect Docker enable/read-only config. Read-only is enough for event observation.
- Add tests with a fake event source rather than requiring Docker in CI.

### 12. Telnyx triggers

Trigger types:

- `telnyx_sms_received`
- `telnyx_call_received`

Files:

- Modify `internal/tools/missions_v2.go`
- Modify Telnyx webhook handling in `internal/server/server_routes.go` or the Telnyx package handler path
- Modify `ui/missions_v2.html`
- Modify `ui/js/missions/main.js`
- Modify all `ui/lang/missions/*.json`
- Modify `prompts/tools_manuals/manage_missions.md`

Implementation:

- Add filters for sender/caller, message contains, and event kind.
- Preserve existing SMS loopback behavior; mission triggers are additive.
- Wrap inbound webhook payload-derived trigger context with existing external data isolation path when appended to prompts.

### 13. Chat integration triggers

Trigger types:

- `telegram_message_received`
- `discord_message_received`
- `rocketchat_message_received`

Files:

- Modify `internal/tools/missions_v2.go`
- Modify `internal/telegram`, `internal/discord`, and `internal/rocketchat` message handlers
- Modify `ui/missions_v2.html`
- Modify `ui/js/missions/main.js`
- Modify all `ui/lang/missions/*.json`
- Modify `prompts/tools_manuals/manage_missions.md`

Implementation:

- Add filters for channel/chat ID, sender, and text contains.
- Keep current loopback behavior; mission triggers are additive.
- Do not pass raw external message content into mission prompts without `security.IsolateExternalData`, which already happens through `appendIsolatedTriggerContext`.

### 14. Infrastructure status triggers

Trigger types:

- `tailscale_node_joined`
- `tailscale_node_left`
- `cloudflare_tunnel_up`
- `cloudflare_tunnel_down`
- `health_check_degraded`

Implementation:

- Add these after planner, Docker, Telnyx, and chat triggers.
- Prefer reusing existing runtime monitors over creating new polling loops.
- Add filters only where users need them: node name/IP for Tailscale, tunnel name for Cloudflare, health check component/severity for health.

## Alternative Approaches Considered

### Minimal patch only

Fix only the P0/P1 bugs and leave missing triggers for later. This is fastest and safest but does not answer the integration gap in the audit.

### Broad trigger framework first

Build a generalized event bus and migrate all integrations onto it before adding individual triggers. This is cleaner long term but too large for the immediate bug-fix need.

### Recommended hybrid

Stabilize the existing runtime first, add small registration/rate-limit primitives second, then implement missing triggers as vertical slices. This keeps the code shippable after every wave and avoids a speculative event-bus rewrite.

## Testing Plan

Run focused tests first:

```bash
go test ./internal/tools -run "Mission|Email|MQTT" -count=1
go test ./internal/server -run "Mission|Webhook|Telnyx|Planner" -count=1
go test ./internal/planner -count=1
```

Then run broader validation:

```bash
go test ./internal/tools/...
go test ./internal/server/...
go test ./internal/planner/...
go test ./...
```

For UI changes:

- Open the Missions page.
- Create and edit one mission for every new trigger type.
- Confirm trigger badges render correctly.
- Confirm no text overflows in the trigger selector and trigger detail fields.
- Verify all 15 `ui/lang/missions/*.json` files contain the new keys.

## Suggested Commit Sequence

1. `fix: harden mission completion and queue persistence`
2. `fix: wire email mission triggers after watcher startup`
3. `feat: add mission trigger debounce controls`
4. `feat: add planner mission triggers`
5. `feat: add docker mission triggers`
6. `feat: add telnyx mission triggers`
7. `feat: add chat message mission triggers`

## Open Decision

Wave 3 should not try to implement every missing integration from the audit at once. The recommended first trigger integration set is Planner, Docker, Telnyx, and chat messages because they have clear event sources and user-visible automation value. Proxmox, TrueNAS, S3, MeshCentral, A2A, general cron completion, tool failure, health check, memory maintenance, and deeper Invasion events should be planned as follow-up slices after the trigger runtime is stable.
