# HomeDash M0 Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and test the HomeDash backend foundations (config block, core types, HA REST helpers and mapping, widget catalog and dashboard schema, store, hub, command pipeline, HA websocket client) without any user-visible change.
**Architecture:** A new core package `internal/homedash` (standard library, `internal/security`, `internal/dbutil`, `modernc.org/sqlite` only) holds types, the catalog, document validation, the `homedash.db` store, the demand-driven entity hub and the command pipeline. `internal/homedash/sources/hass` holds the Home Assistant domain/attribute mapping and the websocket client; two thin exported wrappers in `internal/tools` keep a single HA HTTP policy. `internal/config` gains the `homedash` block; nothing is wired into the server.
**Tech Stack:** Go 1.27.4, `modernc.org/sqlite` v1.60.1 via `internal/dbutil`, `github.com/gorilla/websocket` (go.mod:39, already direct), `gopkg.in/yaml.v3`, `net/http/httptest`, `internal/testutil`.

**The code in this plan was written without compiling.** Implementers may fix compile errors, imports and obvious typos, but must not change exported names, signatures, JSON tags or error codes without reporting it to the lead.

Rules for every task (state them in each dispatch):
- Work only in the worktree from Task 0. LF line endings, no BOM. No new modules in go.mod.
- The Write/Edit tools decode `\uXXXX`: never type a backslash-u escape in Go source; use `string(rune(0x202e))` (memory feedback_write_tool_unicode_escapes).
- Before editing an EXISTING symbol run GitNexus impact (`node C:\Users\Andi\Documents\repo\AuraGo\.gitnexus\run.cjs impact "<symbol>" --direction upstream --repo C:\Users\Andi\Documents\repo\AuraGo`, or MCP `impact` with that repo). On 2026-11-23 the CLI answered "index file version 49, installed build 47" (no graph). Fallback: grep the callers, list them in the report, and write `GitNexus unavailable (index version mismatch); callers confirmed with grep: <list>` in the commit body. `risk: UNKNOWN` is not an all-clear. New symbols need no impact run (the index cannot see them).
- Before each commit: MCP `detect_changes({scope:"all", repo:"C:\\Users\\Andi\\Documents\\repo\\AuraGo", worktree:"<worktree path>"})`; if unavailable write `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.` in the commit body. Then `git diff --cached --check` and a secret scan of the staged diff.
- Commit after every task with only the task's paths. New `AGENTS.md` files are git-ignored (`.gitignore:143`): `git add -f`, then confirm with `git ls-files <path>`.
- Go: `go test` per package as named. If `go` fails with `no such tool "vet"`, prefix commands with `$env:GOTOOLCHAIN='go1.27.4';` (memory reference_go_cache_disk_d). If D: is full, report it; do not trim caches yourself. `GOTMPDIR` must stay in backslash form.
- `-run` patterns: group as `(?i)(a|b)`; count PASS lines in `-v` output (memory feedback_go_test_run_patterns).

## Lead amendments (2026-11-23)

Found in the review of this plan. They override the task bodies wherever those say otherwise. Every dispatch
of the named task includes them.

**A1. Task 8, control gate fails closed.** In `admitCommand` the source control gate is `!set.Control` for
every source, with no exemption for an empty `ConfigKey` and none for a source missing from the settings map
(the zero value refuses). The result is `read_only` with `blocked_by` naming the source's control switch, or
`homedash.sources.<source>` when the source has no settings entry. Sources without a control switch in the
config block are built with `Control: false`. Test fakes set `Control: true` explicitly. Add two rows to the
command table test: settings entry with `Control: false` and empty `ConfigKey` → `read_only`; no settings
entry for the source → `read_only`. Correct the comment on `SourceSettings` in Task 2 to match.

**A2. Task 8, kiosk commands need the kiosk switch.** `admitCommand` refuses every command of a kiosk
principal with `CodeKioskDisabled` when the current runtime config has kiosk disabled. The check comes before
the allowed-set check. Add a row to the command table test: kiosk principal, entity allowed, kiosk disabled →
`kiosk_disabled`.

**A3. Task 3, explicit out-of-range values are rejected, not repaired.** `Normalize` fills defaults only for
keys that are absent from the JSON. Decode `look.radius_scale` and `grid.row_unit` so that absence can be told
from an explicit value (pointer fields or a presence flag in the custom unmarshaller). An explicit
`radius_scale` of 0, or outside 0.5–1.5, and an explicit numeric `row_unit` of 0, or outside 48–200, reach
`Validate` and produce an `invalid_document` issue with the field's JSON pointer. Add four rows to the
validation table test: `radius_scale: 0`, `radius_scale: 1.6`, `row_unit: 0`, `row_unit: 47` → one issue each;
keep the rows that prove absent keys default to 1.0 and `"auto"`.

**A4. Scope note.** Task 9 (the package guide and index rows) stays in this plan: the repository's
documentation rule requires it with the code.

## File structure

Created:
- `internal/config/homedash.go` — `HomeDashConfig` types and `NormalizeHomeDashConfig`.
- `internal/config/homedash_test.go` — defaults, ranges, template parity.
- `internal/server/homedash_config_test.go` — config save/patch reject out-of-range values.
- `internal/homedash/errors.go` — `Error`, `Issue`, error codes, sentinel errors.
- `internal/homedash/ids.go` — source/entity/dashboard/page/widget/slug/icon id grammar, random ids.
- `internal/homedash/entity.go` — `Kind`, `Entity`, `State`, `CapabilitySpec`, `MediaInfo`, `StateUpdate`, `SourceStatus`, status/reason constants.
- `internal/homedash/capability.go` — capability constants, registry, `ValidateArgs`.
- `internal/homedash/sanitize.go` — `CleanText`, `ContainsControl`, `ContainsURL`, `BoundAttributes`.
- `internal/homedash/source.go` — `Sink`, `Source`, `HistorySource`, `SnapshotSource`, `Sample`, `Factory`, `Command`, `CommandResult`.
- `internal/homedash/principal.go` — `Principal`, `AllowedSet` (B1); `BuildAllowedSet` (added in B4b).
- `internal/homedash/runtime_config.go` — `RuntimeConfig`, `SourceSettings.Fingerprint`.
- `internal/homedash/core_test.go` — id, args, sanitize tests.
- `internal/homedash/dashboard.go` — document types, `RowUnit`, `DecodeDashboard`, `Normalize`.
- `internal/homedash/dashboard_validate.go` — `Validate`, `Report`, issue codes.
- `internal/homedash/dashboard_layout.go` — layout checks helpers and `Autopack`.
- `internal/homedash/dashboard_migrate.go` — `Migrate`.
- `internal/homedash/dashboard_patch.go` — `PatchOp`, `ApplyPatch`.
- `internal/homedash/catalog.go` — catalog types, `Catalog()`, hash, lookups.
- `internal/homedash/catalog_widgets.go` — the 24 widget types (data).
- `internal/homedash/catalog_test.go`, `dashboard_test.go` — golden, invariants, validation, autopack, migrate, patch.
- `internal/homedash/testdata/widget_catalog.json` — golden catalog (generated with `-update`).
- `internal/homedash/store.go`, `store_dashboards.go`, `store_kiosk.go`, `store_test.go` — `homedash.db`.
- `internal/homedash/hub.go`, `hub_sources.go`, `hub_reconfigure.go`, `subscriber.go` — hub core.
- `internal/homedash/sourcetest/fake.go` — `FakeSource`, `RecordingSink`, `WaitFor`.
- `internal/homedash/hub_test.go` — hub tests (package `homedash_test`).
- `internal/homedash/command.go`, `ratelimit.go`, `history.go` — pipeline, limiter, ring.
- `internal/homedash/command_test.go` (package `homedash_test`), `ratelimit_test.go`, `history_test.go` (package `homedash`).
- `internal/homedash/AGENTS.md` — child guide (Task 9).
- `internal/homedash/sources/hass/domains.go`, `attributes.go`, `mapping_test.go` — HA mapping.
- `internal/homedash/sources/hass/ws_messages.go`, `ws_client.go`, `ws_client_test.go` — HA websocket client.
- `internal/tools/homeassistant_raw.go`, `homeassistant_raw_test.go` — `HARequestJSON`, `HAGetBytes`.

Modified:
- `internal/config/config_types.go:2213` — add `HomeDash` field after `ChibiSensei`.
- `internal/config/config.go:601-602` — loader bool defaults; `:1136-1138` — Normalize call.
- `internal/server/config_handlers_main.go:710-713` (in `handleUpdateConfig`, :491) — Normalize call.
- `internal/server/config_patch.go:118-120` (in `applyConfigPatch`, :26) — Normalize call.
- `config_template.yaml:2437` — `homedash:` block between `chibi_sensei` and `journal` (:2438).
- `internal/config/sqlite_paths.go:28-29, :98` — `HomeDashDBFilename`, archive key `homedash`.
- `internal/config/sqlite_paths_test.go:49` — expected path list gains `homedash.db`.
- `internal/tools/homeassistant.go:43-75` — `haRequestContext` delegates to new `haDo`.
- `internal/tools/AGENTS.md:25-30` — one sentence on the two wrappers (Task 9).
- `AGENTS.md` (root) Child DOX Index — one HomeDash row (Task 9).

## Task 0: Workspace

- [ ] Check the main checkout: `git -C C:\Users\Andi\Documents\repo\AuraGo branch --show-current` is `main`; `git -C C:\Users\Andi\Documents\repo\AuraGo status --short -- internal/config internal/server/config_handlers_main.go internal/server/config_patch.go internal/tools/homeassistant.go config_template.yaml internal/homedash` shows nothing of ours (another session's dirty files there must be reported to the lead before starting; memory project_concurrent_sessions).
- [ ] `git -C C:\Users\Andi\Documents\repo\AuraGo worktree add .worktrees\homedash-m0-backend -b claude/homedash-m0-backend main`. All later commands run in `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\homedash-m0-backend`. A fresh worktree has no foreign untracked files, so no `-overlay` is needed. Run git as plain single commands (memory reference_worktree_node_modules).
- [ ] `go version` (expect go1.27.4); `Get-PSDrive D` (report if < 20 GB free).
- [ ] Baseline (record every failure in `C:\Users\Andi\AppData\Local\Temp\claude\babd9307b9bef681\c0ba1aa9-616f-4cb4-9fb5-eae97a804b4e\scratchpad\homedash-m0-baseline.txt`, not in the repo):
  - `go build ./...`
  - `go test ./internal/config/... ./internal/audit/... ./cmd/config-merger/... -count=1`
  - `go test ./internal/tools/ -run '(?i)(TestHA)' -count=1`
  - `go test ./internal/server/ -run '(?i)(ConfigValidation|ConfigSave|UpdateConfig)' -count=1`
  - `go test ./ui/ -run 'TestConfigDerivedValidationRulesCoverTemplate' -count=1`
- **Done when:** worktree on `claude/homedash-m0-backend` exists and the baseline file lists results.

### Task 1: B0 config block

**Files:** Create `internal/config/homedash.go`, `internal/config/homedash_test.go`, `internal/server/homedash_config_test.go`. Modify `internal/config/config_types.go:2213`, `internal/config/config.go:601-602` and `:1136-1138`, `internal/server/config_handlers_main.go:710-713`, `internal/server/config_patch.go:118-120`, `config_template.yaml:2437`.
**Read first:** backend part §11; `internal/config/chibi_sensei.go:11-68`; `internal/config/chibi_sensei_test.go:21-125`; `internal/server/chibi_sensei_service_test.go:249-278`; `config_template.yaml:2402-2440`; `ui/config_validation_rules_test.go:144-200` (names ending `_seconds`/`_minutes` get a min-0 rule; all template values here are positive). No config UI section and no translations are needed: sections are listed explicitly in `ui/js/config/main.js:133` and no test enumerates template sections (checked `ui/config_*_test.go`, `internal/server/config_handlers_test.go:1593`, `internal/config/template_integrations_test.go:22`, `cmd/config-merger/main_test.go:452-489`). The UI section is M8.

- [ ] Write `internal/config/homedash_test.go`:
```go
package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func loadHomeDashYAML(t *testing.T, content string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func homeDashDefaults() HomeDashConfig {
	src := func(control bool, poll int) HomeDashSourceConfig {
		return HomeDashSourceConfig{Enabled: true, Control: control, PollSeconds: poll}
	}
	return HomeDashConfig{
		Enabled: true, Agent: HomeDashAgentConfig{Access: "write"},
		Kiosk:   HomeDashKioskConfig{MaxDevices: 10, PairingTTLMinutes: 15, AllowControl: true},
		Limits:  HomeDashLimitsConfig{MaxDashboards: 50, MaxStreams: 32, MaxKioskStreams: 16, MaxSubscriptions: 300, CommandsPerMinute: 60},
		History: HomeDashHistoryConfig{RingPoints: 360, RingEntities: 500, SampleSeconds: 60},
		Sources: HomeDashSourcesConfig{
			HomeAssistant: src(true, 5), Go2RTC: src(false, 30), FritzBox: src(true, 15), MQTT: src(true, 0),
			System: src(false, 5), Docker: src(false, 60), Proxmox: src(false, 30), ThreeDPrinters: src(false, 30),
			UptimeKuma: src(false, 60), AdGuard: src(false, 30), TrueNAS: src(false, 60), Bluetooth: src(false, 0),
		},
	}
}

func TestHomeDashDefaults(t *testing.T) {
	for name, content := range map[string]string{"empty": "{}\n", "null section": "homedash:\n"} {
		cfg, err := loadHomeDashYAML(t, content)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if cfg.HomeDash != homeDashDefaults() {
			t.Fatalf("%s: homedash = %+v", name, cfg.HomeDash)
		}
	}
	cfg, err := loadHomeDashYAML(t, "homedash:\n  enabled: false\n  kiosk:\n    allow_control: false\n")
	if err != nil || cfg.HomeDash.Enabled || cfg.HomeDash.Kiosk.AllowControl {
		t.Fatalf("explicit false overridden: %+v %v", cfg.HomeDash, err)
	}
}

func TestNormalizeHomeDashConfigDefaultsAndRanges(t *testing.T) {
	var zero HomeDashConfig
	if err := NormalizeHomeDashConfig(&zero); err != nil {
		t.Fatal(err)
	}
	if zero.Limits != homeDashDefaults().Limits || zero.Agent.Access != "write" || zero.Sources.Docker.PollSeconds != 60 || zero.Kiosk.DeviceTTLDays != 0 {
		t.Fatalf("zero values not defaulted: %+v", zero)
	}
	for i, mut := range []func(*HomeDashConfig){
		func(c *HomeDashConfig) { c.Limits.MaxDashboards = 500 },
		func(c *HomeDashConfig) { c.Kiosk.DeviceTTLDays = 3650 },
		func(c *HomeDashConfig) { c.Sources.HomeAssistant.PollSeconds = 2 },
		func(c *HomeDashConfig) { c.Agent.Access = " READ " },
		func(c *HomeDashConfig) { c.Sources.MQTT.PollSeconds = 999 }, // no poll for mqtt: ignored
	} {
		c := homeDashDefaults()
		mut(&c)
		if err := NormalizeHomeDashConfig(&c); err != nil {
			t.Errorf("ok case %d rejected: %v", i, err)
		}
	}
	for _, tc := range []struct {
		key string
		mut func(*HomeDashConfig)
	}{
		{"limits.max_dashboards", func(c *HomeDashConfig) { c.Limits.MaxDashboards = 501 }},
		{"limits.max_streams", func(c *HomeDashConfig) { c.Limits.MaxStreams = 3 }},
		{"limits.max_kiosk_streams", func(c *HomeDashConfig) { c.Limits.MaxKioskStreams = 101 }},
		{"limits.max_subscriptions", func(c *HomeDashConfig) { c.Limits.MaxSubscriptions = 49 }},
		{"limits.commands_per_minute", func(c *HomeDashConfig) { c.Limits.CommandsPerMinute = 601 }},
		{"kiosk.max_devices", func(c *HomeDashConfig) { c.Kiosk.MaxDevices = 101 }},
		{"kiosk.pairing_ttl_minutes", func(c *HomeDashConfig) { c.Kiosk.PairingTTLMinutes = 4 }},
		{"kiosk.device_ttl_days", func(c *HomeDashConfig) { c.Kiosk.DeviceTTLDays = -1 }},
		{"kiosk.device_ttl_days", func(c *HomeDashConfig) { c.Kiosk.DeviceTTLDays = 3651 }},
		{"history.ring_points", func(c *HomeDashConfig) { c.History.RingPoints = 59 }},
		{"history.ring_entities", func(c *HomeDashConfig) { c.History.RingEntities = 5001 }},
		{"history.sample_seconds", func(c *HomeDashConfig) { c.History.SampleSeconds = 9 }},
		{"sources.home_assistant.poll_seconds", func(c *HomeDashConfig) { c.Sources.HomeAssistant.PollSeconds = 1 }},
		{"sources.docker.poll_seconds", func(c *HomeDashConfig) { c.Sources.Docker.PollSeconds = 601 }},
		{"sources.uptime_kuma.poll_seconds", func(c *HomeDashConfig) { c.Sources.UptimeKuma.PollSeconds = 29 }},
		{"agent.access", func(c *HomeDashConfig) { c.Agent.Access = "admin" }},
	} {
		c := homeDashDefaults()
		tc.mut(&c)
		if err := NormalizeHomeDashConfig(&c); err == nil || !strings.Contains(err.Error(), "homedash: "+tc.key) {
			t.Errorf("%s: err = %v", tc.key, err)
		}
	}
	if _, err := loadHomeDashYAML(t, "homedash:\n  limits:\n    max_streams: 1\n"); err == nil {
		t.Error("Load accepted limits.max_streams = 1")
	}
}

func TestHomeDashTemplateMatchesLoadDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	node, ok := doc["homedash"]
	if !ok {
		t.Fatal("config_template.yaml has no homedash: block")
	}
	raw, _ := yaml.Marshal(&node)
	var tpl HomeDashConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("template block does not decode: %v", err)
	}
	if err := NormalizeHomeDashConfig(&tpl); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadHomeDashYAML(t, "{}\n")
	if err != nil {
		t.Fatal(err)
	}
	if tpl != cfg.HomeDash {
		t.Fatalf("template %+v differs from loader defaults %+v", tpl, cfg.HomeDash)
	}
}
```
- [ ] Write `internal/server/homedash_config_test.go`: a copy of `TestChibiSenseiConfigValidationRejectsBadValuesBeforeWrite` (`chibi_sensei_service_test.go:249-278`) named `TestHomeDashConfigValidationRejectsBadValuesBeforeWrite`, with `original := []byte("server:\n  host: 127.0.0.1\nhomedash:\n  enabled: true\n")`, patch `{"homedash":{"limits":{"max_streams":1}}}` and the checks `strings.Contains(..., "homedash:")`.
- [ ] Run `go test ./internal/config/ -run '(?i)(HomeDash)' -count=1` → expect build failure (undefined `HomeDashConfig`).
- [ ] Implement `internal/config/homedash.go`:
```go
package config

import (
	"fmt"
	"strings"
)

// HomeDashConfig configures the HomeDash dashboards, wall-display kiosk and entity hub.
// Bool defaults (enabled, kiosk.allow_control, sources.*.enabled, control for HA/FritzBox/MQTT)
// are set by the loader before YAML decoding.
type HomeDashConfig struct {
	Enabled  bool                  `yaml:"enabled" json:"enabled"`
	ReadOnly bool                  `yaml:"readonly" json:"readonly"`
	Agent    HomeDashAgentConfig   `yaml:"agent" json:"agent"`
	Kiosk    HomeDashKioskConfig   `yaml:"kiosk" json:"kiosk"`
	Limits   HomeDashLimitsConfig  `yaml:"limits" json:"limits"`
	History  HomeDashHistoryConfig `yaml:"history" json:"history"`
	Sources  HomeDashSourcesConfig `yaml:"sources" json:"sources"`
}

type HomeDashAgentConfig struct {
	Access string `yaml:"access" json:"access"` // off | read | write (default write)
}

type HomeDashKioskConfig struct {
	Enabled           bool `yaml:"enabled" json:"enabled"`
	MaxDevices        int  `yaml:"max_devices" json:"max_devices"`                 // 1..100, default 10
	PairingTTLMinutes int  `yaml:"pairing_ttl_minutes" json:"pairing_ttl_minutes"` // 5..60, default 15
	DeviceTTLDays     int  `yaml:"device_ttl_days" json:"device_ttl_days"`         // 0 = until revoked, else 1..3650
	AllowControl      bool `yaml:"allow_control" json:"allow_control"`
}

type HomeDashLimitsConfig struct {
	MaxDashboards     int `yaml:"max_dashboards" json:"max_dashboards"`
	MaxStreams        int `yaml:"max_streams" json:"max_streams"`
	MaxKioskStreams   int `yaml:"max_kiosk_streams" json:"max_kiosk_streams"`
	MaxSubscriptions  int `yaml:"max_subscriptions" json:"max_subscriptions"`
	CommandsPerMinute int `yaml:"commands_per_minute" json:"commands_per_minute"`
}

type HomeDashHistoryConfig struct {
	RingPoints    int `yaml:"ring_points" json:"ring_points"`
	RingEntities  int `yaml:"ring_entities" json:"ring_entities"`
	SampleSeconds int `yaml:"sample_seconds" json:"sample_seconds"`
}

// HomeDashSourceConfig is one source. Sources without control in v1 ignore Control; mqtt and
// bluetooth ignore PollSeconds.
type HomeDashSourceConfig struct {
	Enabled     bool `yaml:"enabled" json:"enabled"`
	Control     bool `yaml:"control" json:"control"`
	PollSeconds int  `yaml:"poll_seconds" json:"poll_seconds"`
}

type HomeDashSourcesConfig struct {
	HomeAssistant  HomeDashSourceConfig `yaml:"home_assistant" json:"home_assistant"`
	Go2RTC         HomeDashSourceConfig `yaml:"go2rtc" json:"go2rtc"`
	FritzBox       HomeDashSourceConfig `yaml:"fritzbox" json:"fritzbox"`
	MQTT           HomeDashSourceConfig `yaml:"mqtt" json:"mqtt"`
	System         HomeDashSourceConfig `yaml:"system" json:"system"`
	Docker         HomeDashSourceConfig `yaml:"docker" json:"docker"`
	Proxmox        HomeDashSourceConfig `yaml:"proxmox" json:"proxmox"`
	ThreeDPrinters HomeDashSourceConfig `yaml:"three_d_printers" json:"three_d_printers"`
	UptimeKuma     HomeDashSourceConfig `yaml:"uptime_kuma" json:"uptime_kuma"`
	AdGuard        HomeDashSourceConfig `yaml:"adguard" json:"adguard"`
	TrueNAS        HomeDashSourceConfig `yaml:"truenas" json:"truenas"`
	Bluetooth      HomeDashSourceConfig `yaml:"bluetooth" json:"bluetooth"`
}

// NormalizeHomeDashConfig applies defaults for zero values and rejects values outside their
// range or an unknown agent.access. It does not change booleans.
func NormalizeHomeDashConfig(c *HomeDashConfig) error {
	c.Agent.Access = strings.ToLower(strings.TrimSpace(c.Agent.Access))
	switch c.Agent.Access {
	case "":
		c.Agent.Access = "write"
	case "off", "read", "write":
	default:
		return fmt.Errorf("homedash: agent.access must be off, read or write")
	}
	if c.Kiosk.DeviceTTLDays < 0 || c.Kiosk.DeviceTTLDays > 3650 {
		return fmt.Errorf("homedash: kiosk.device_ttl_days must be 0–3650")
	}
	s := &c.Sources
	for _, f := range []struct {
		p           *int
		def, lo, hi int
		key         string
	}{
		{&c.Kiosk.MaxDevices, 10, 1, 100, "kiosk.max_devices"},
		{&c.Kiosk.PairingTTLMinutes, 15, 5, 60, "kiosk.pairing_ttl_minutes"},
		{&c.Limits.MaxDashboards, 50, 1, 500, "limits.max_dashboards"},
		{&c.Limits.MaxStreams, 32, 4, 128, "limits.max_streams"},
		{&c.Limits.MaxKioskStreams, 16, 1, 100, "limits.max_kiosk_streams"},
		{&c.Limits.MaxSubscriptions, 300, 50, 1000, "limits.max_subscriptions"},
		{&c.Limits.CommandsPerMinute, 60, 10, 600, "limits.commands_per_minute"},
		{&c.History.RingPoints, 360, 60, 2880, "history.ring_points"},
		{&c.History.RingEntities, 500, 50, 5000, "history.ring_entities"},
		{&c.History.SampleSeconds, 60, 10, 600, "history.sample_seconds"},
		{&s.HomeAssistant.PollSeconds, 5, 2, 60, "sources.home_assistant.poll_seconds"},
		{&s.Go2RTC.PollSeconds, 30, 10, 300, "sources.go2rtc.poll_seconds"},
		{&s.FritzBox.PollSeconds, 15, 10, 300, "sources.fritzbox.poll_seconds"},
		{&s.System.PollSeconds, 5, 2, 60, "sources.system.poll_seconds"},
		{&s.Docker.PollSeconds, 60, 15, 600, "sources.docker.poll_seconds"},
		{&s.Proxmox.PollSeconds, 30, 10, 600, "sources.proxmox.poll_seconds"},
		{&s.ThreeDPrinters.PollSeconds, 30, 10, 600, "sources.three_d_printers.poll_seconds"},
		{&s.UptimeKuma.PollSeconds, 60, 30, 600, "sources.uptime_kuma.poll_seconds"},
		{&s.AdGuard.PollSeconds, 30, 15, 600, "sources.adguard.poll_seconds"},
		{&s.TrueNAS.PollSeconds, 60, 30, 600, "sources.truenas.poll_seconds"},
	} {
		if *f.p == 0 {
			*f.p = f.def
		}
		if *f.p < f.lo || *f.p > f.hi {
			return fmt.Errorf("homedash: %s must be %d–%d", f.key, f.lo, f.hi)
		}
	}
	return nil
}
```
- [ ] GitNexus impact on `Config` (struct; expect CRITICAL hub symbol — adding a field is additive, report it), `load`, `handleUpdateConfig`, `applyConfigPatch`.
- [ ] `config_types.go`: after line 2213 add `HomeDash          HomeDashConfig          \`yaml:"homedash" json:"homedash"\``.
- [ ] `config.go` after line 602:
```go
	// HomeDash is on by default: nothing connects until a dashboard is open; the kiosk stays off.
	cfg.HomeDash.Enabled = true
	cfg.HomeDash.Kiosk.AllowControl = true
	hs := &cfg.HomeDash.Sources
	for _, src := range []*HomeDashSourceConfig{&hs.HomeAssistant, &hs.Go2RTC, &hs.FritzBox, &hs.MQTT, &hs.System, &hs.Docker,
		&hs.Proxmox, &hs.ThreeDPrinters, &hs.UptimeKuma, &hs.AdGuard, &hs.TrueNAS, &hs.Bluetooth} {
		src.Enabled = true
	}
	hs.HomeAssistant.Control, hs.FritzBox.Control, hs.MQTT.Control = true, true, true
```
- [ ] `config.go` after line 1138, `config_handlers_main.go` after line 713 and `config_patch.go` after line 120: the same block as the Chibi Sensei call beside it, with `NormalizeHomeDashConfig(&cfg.HomeDash)` / `config.NormalizeHomeDashConfig(&validateCfg.HomeDash)` and the same error handling as the neighbouring line.
- [ ] `config_template.yaml`: insert after line 2436 (blank line before and after), 4-space indentation:
```yaml
# HomeDash: dashboards for lights, switches, sensors and cameras from every connected source (desktop app and wall displays).
homedash:
    enabled: true                   # nothing connects to a device until a dashboard is open
    readonly: false                 # blocks every HomeDash command; dashboards stay editable
    agent:
        access: write               # off, read or write; the agent edits dashboards and never sends device commands
    kiosk:
        enabled: false              # wall-display page and pairing; off by default because it opens a public route
        max_devices: 10             # paired displays in total (1-100)
        pairing_ttl_minutes: 15     # lifetime of a pairing link (5-60)
        device_ttl_days: 0          # 0 = until revoked, else 1-3650
        allow_control: true         # false makes every display view-only
    limits:
        max_dashboards: 50          # (1-500)
        max_streams: 32             # open desktop streams (4-128)
        max_kiosk_streams: 16       # open display streams (1-100)
        max_subscriptions: 300      # entities per stream (50-1000)
        commands_per_minute: 60     # per session or display (10-600)
    history:
        ring_points: 360            # in-memory points per entity (60-2880)
        ring_entities: 500          # entities with in-memory history (50-5000)
        sample_seconds: 60          # one point per interval (10-600)
    sources:                        # each source also needs the integration's own enabled switch
        home_assistant:
            enabled: true
            control: true
            poll_seconds: 5         # only in the REST fallback (2-60)
        go2rtc:
            enabled: true
            poll_seconds: 30        # (10-300)
        fritzbox:
            enabled: true
            control: true
            poll_seconds: 15        # (10-300)
        mqtt:
            enabled: true
            control: true
        system:
            enabled: true
            poll_seconds: 5         # (2-60)
        docker:
            enabled: true
            control: false          # start, stop and restart containers from a dashboard
            poll_seconds: 60        # safety poll next to Engine events (15-600)
        proxmox:
            enabled: true
            poll_seconds: 30        # (10-600)
        three_d_printers:
            enabled: true
            poll_seconds: 30        # (10-600; 10 s while printing)
        uptime_kuma:
            enabled: true
            poll_seconds: 60        # (30-600)
        adguard:
            enabled: true
            control: false          # the filtering switch
            poll_seconds: 30        # (15-600)
        truenas:
            enabled: true
            poll_seconds: 60        # (30-600)
        bluetooth:
            enabled: true
```
- [ ] Run `go test ./internal/config/... ./cmd/config-merger/... -count=1`, `go test ./internal/server/ -run '(?i)(HomeDashConfig|ChibiSenseiConfig)' -count=1 -v`, `go test ./ui/ -run TestConfigDerivedValidationRulesCoverTemplate -count=1` → PASS (`TestHomeDashDefaults`, `TestNormalizeHomeDashConfigDefaultsAndRanges`, `TestHomeDashTemplateMatchesLoadDefaults`, `TestHomeDashConfigValidationRejectsBadValuesBeforeWrite`); only baseline failures remain.
- [ ] Commit: `git add internal/config/homedash.go internal/config/homedash_test.go internal/config/config_types.go internal/config/config.go internal/server/config_handlers_main.go internal/server/config_patch.go internal/server/homedash_config_test.go config_template.yaml`; message `feat(homedash): config block with defaults, ranges and template` + GitNexus lines.
- **Done when:** the four tests pass, the template block equals the loader defaults, no other test regressed.

### Task 2: B1 core types

**Files:** Create `internal/homedash/{errors,ids,entity,capability,sanitize,source,principal,runtime_config}.go`, `internal/homedash/core_test.go`.
**Read first:** backend part §3 (core table), §4, §6 (arguments table, error codes), §9; main document §4.5.

- [ ] Write `core_test.go` (package `homedash`):
```go
package homedash

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func f64(v float64) *float64 { return &v }

func TestParseEntityID(t *testing.T) {
	long := strings.Repeat("a", 200)
	for _, tc := range []struct {
		in     string
		ok     bool
		src    SourceID
		native string
	}{
		{"ha:light.kitchen", true, "ha", "light.kitchen"},
		{"docker:local/web", true, "docker", "local/web"},
		{"ha:" + long, true, "ha", long},
		{"ha:" + long + "a", false, "", ""},
		{"ha:", false, "", ""}, {":x", false, "", ""}, {"HA:x", false, "", ""}, {"h:x", false, "", ""},
		{"ha:has space", false, "", ""}, {"ha:tab\tx", false, "", ""}, {"nocolon", false, "", ""},
		{"ha:" + string(rune(0x202e)) + "x", false, "", ""},
	} {
		src, native, err := ParseEntityID(tc.in)
		if (err == nil) != tc.ok || src != tc.src || native != tc.native {
			t.Errorf("%q: %q %q %v", tc.in, src, native, err)
		}
	}
}

func TestValidateArgsPerCapability(t *testing.T) {
	pct := CapabilitySpec{Min: f64(0), Max: f64(100), Step: f64(1)}
	with := func(c Capability, base CapabilitySpec) CapabilitySpec { base.Name = c; return base }
	for _, tc := range []struct {
		name string
		spec CapabilitySpec
		args map[string]any
		code string
	}{
		{"toggle", with(CapToggle, CapabilitySpec{}), map[string]any{}, ""},
		{"toggle extra", with(CapToggle, CapabilitySpec{}), map[string]any{"x": 1.0}, CodeInvalidArgs},
		{"brightness", with(CapBrightness, pct), map[string]any{"value": 40.0}, ""},
		{"brightness range", with(CapBrightness, pct), map[string]any{"value": 150.0}, CodeInvalidArgs},
		{"brightness step", with(CapBrightness, pct), map[string]any{"value": 40.5}, CodeInvalidArgs},
		{"brightness text", with(CapBrightness, pct), map[string]any{"value": "40"}, CodeInvalidArgs},
		{"kelvin", with(CapColorTemp, CapabilitySpec{Min: f64(2200), Max: f64(6500), Step: f64(50)}), map[string]any{"kelvin": 3000.0}, ""},
		{"kelvin step", with(CapColorTemp, CapabilitySpec{Min: f64(2200), Max: f64(6500), Step: f64(50)}), map[string]any{"kelvin": 3025.0}, CodeInvalidArgs},
		{"color", with(CapColor, CapabilitySpec{}), map[string]any{"h": 200.0, "s": 50.0}, ""},
		{"color hue", with(CapColor, CapabilitySpec{}), map[string]any{"h": 400.0, "s": 50.0}, CodeInvalidArgs},
		{"color missing", with(CapColor, CapabilitySpec{}), map[string]any{"h": 200.0}, CodeInvalidArgs},
		{"option", with(CapHVACMode, CapabilitySpec{Options: []string{"heat", "off"}}), map[string]any{"option": "heat"}, ""},
		{"option not offered", with(CapHVACMode, CapabilitySpec{Options: []string{"heat", "off"}}), map[string]any{"option": "cool"}, CodeInvalidArgs},
		{"range", with(CapSetTemperatureRange, CapabilitySpec{Min: f64(7), Max: f64(35)}), map[string]any{"low": 18.0, "high": 22.0}, ""},
		{"range inverted", with(CapSetTemperatureRange, CapabilitySpec{Min: f64(7), Max: f64(35)}), map[string]any{"low": 22.0, "high": 18.0}, CodeInvalidArgs},
		{"bool", with(CapOscillate, CapabilitySpec{}), map[string]any{"on": true}, ""},
		{"bool text", with(CapOscillate, CapabilitySpec{}), map[string]any{"on": "yes"}, CodeInvalidArgs},
		{"nan", with(CapSetValue, CapabilitySpec{}), map[string]any{"value": math.NaN()}, CodeInvalidArgs},
		{"read capability", with(CapStream, CapabilitySpec{}), map[string]any{}, CodeUnsupportedCommand},
	} {
		err := ValidateArgs(tc.spec, tc.args)
		var he *Error
		got := ""
		if errors.As(err, &he) {
			got = he.Code
		} else if err != nil {
			got = "untyped"
		}
		if got != tc.code {
			t.Errorf("%s: code %q, want %q (%v)", tc.name, got, tc.code, err)
		}
	}
}

func TestSanitizeBoundsTextAndAttributes(t *testing.T) {
	for _, tc := range []struct{ in string; max int; want string }{
		{"  Kitchen\nLamp\x00 ", 120, "Kitchen Lamp"},
		{"a" + string(rune(0x202e)) + "b", 10, "ab"},
		{"äöüß", 2, "äö"},
		{"\xffok", 10, "ok"},
	} {
		if got := CleanText(tc.in, tc.max); got != tc.want {
			t.Errorf("CleanText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if !ContainsControl("a\x07") || ContainsControl("plain") || !ContainsURL("see https://x") || !ContainsURL("javascript:alert(1)") || ContainsURL("10:30") {
		t.Error("ContainsControl/ContainsURL")
	}
	list := make([]any, 100)
	for i := range list {
		list[i] = float64(i)
	}
	in := map[string]any{"Bad-Key": 1.0, "ok": "x", "nested": map[string]any{"a": 1.0}, "list": list,
		"nan": math.NaN(), "long": strings.Repeat("z", 9000), "n": 3}
	out := BoundAttributes(in)
	if _, ok := out["Bad-Key"]; ok || out["ok"] != "x" || out["n"] != 3.0 {
		t.Fatalf("keys: %v", out)
	}
	if _, ok := out["nested"]; ok {
		t.Fatal("maps must be dropped")
	}
	if _, ok := out["nan"]; ok {
		t.Fatal("NaN kept")
	}
	if len(out["list"].([]any)) != 64 || len([]rune(out["long"].(string))) != 256 {
		t.Fatal("list or string not bounded")
	}
	big := map[string]any{}
	for i := 0; i < 200; i++ {
		big[strings.Repeat("k", 3)+string(rune('a'+i%26))+strings.Repeat("x", i%10)] = strings.Repeat("v", 200)
	}
	if raw, _ := jsonSize(BoundAttributes(big)); raw > MaxAttrBytes {
		t.Fatalf("attributes %d bytes > %d", raw, MaxAttrBytes)
	}
}
```
(`jsonSize(v any) (int, error)` is a test-file helper: `b, err := json.Marshal(v); return len(b), err` — add `encoding/json` import.)
- [ ] Run `go test ./internal/homedash/ -count=1` → build failure (package empty).
- [ ] Implement `errors.go`:
```go
package homedash

// Error is every refusal HomeDash reports. Code is a fixed machine code; Message is English and
// never contains upstream text.
type Error struct {
	Code            string  `json:"code"`
	Message         string  `json:"message"`
	BlockedBy       string  `json:"blocked_by,omitempty"`
	RetryAfterMS    int64   `json:"retry_after_ms,omitempty"`
	CurrentRevision int64   `json:"current_revision,omitempty"`
	Issues          []Issue `json:"issues,omitempty"`
}

// Issue is one validation finding; Path is a JSON pointer into the document.
type Issue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Is matches by code, so errors.Is(err, ErrTooManyStreams) works for copies.
func (e *Error) Is(target error) bool { t, ok := target.(*Error); return ok && t.Code == e.Code }

func newError(code, msg string) *Error { return &Error{Code: code, Message: msg} }

const (
	CodeBadRequest = "bad_request"; CodeInvalidID = "invalid_id"; CodeInvalidArgs = "invalid_args"
	CodeUnsupportedCommand = "unsupported_command"; CodePairingInvalid = "pairing_invalid"
	CodeKioskUnpaired = "kiosk_unpaired"; CodeKioskRevoked = "kiosk_revoked"; CodeKioskDisabled = "kiosk_disabled"
	CodeReadOnly = "read_only"; CodeForbiddenEntity = "forbidden_entity"; CodeViewOnly = "view_only"
	CodeDashboardNotFound = "dashboard_not_found"; CodeEntityNotFound = "entity_not_found"
	CodeStreamNotFound = "stream_not_found"; CodeRevisionNotFound = "revision_not_found"
	CodeRevisionConflict = "revision_conflict"; CodeConfirmationRequired = "confirmation_required"
	CodeSchemaTooNew = "schema_too_new"; CodeSlugTaken = "slug_taken"; CodeInvalidDocument = "invalid_document"
	CodeInvalidPatch = "invalid_patch"; CodeLimitExceeded = "limit_exceeded"; CodeRateLimited = "rate_limited"
	CodeTooManyStreams = "too_many_streams"; CodeCommandFailed = "command_failed"; CodeSourceError = "source_error"
	CodeHomeDashDisabled = "homedash_disabled"; CodeSourceUnavailable = "source_unavailable"; CodeHubClosed = "hub_closed"
)

var (
	ErrHubClosed         = newError(CodeHubClosed, "HomeDash is shutting down")
	ErrTooManyStreams    = newError(CodeTooManyStreams, "too many open streams")
	ErrEntityNotFound    = newError(CodeEntityNotFound, "unknown entity")
	ErrDashboardNotFound = newError(CodeDashboardNotFound, "unknown dashboard")
)
```
(gofmt splits the const block one per line.)
- [ ] Implement `ids.go`:
```go
package homedash

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type SourceID string
type EntityID string

const MaxNativeIDBytes = 200

var (
	sourceIDRe    = regexp.MustCompile(`^[a-z][a-z0-9_]{1,15}$`)
	dashboardIDRe = regexp.MustCompile(`^d_[a-z2-7]{12}$`)
	pageIDRe      = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	slugRe        = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)
	iconRe        = regexp.MustCompile(`^[a-z0-9-]{1,48}$`)
	errBadEntityID = errors.New("invalid entity id")
)

func ValidSourceID(s string) bool    { return sourceIDRe.MatchString(s) }
func ValidDashboardID(s string) bool { return dashboardIDRe.MatchString(s) }
func ValidPageID(s string) bool      { return pageIDRe.MatchString(s) }
func ValidWidgetID(s string) bool    { return pageIDRe.MatchString(s) }
func ValidSlug(s string) bool        { return slugRe.MatchString(s) }
func ValidIcon(s string) bool        { return iconRe.MatchString(s) }

// ParseEntityID splits "<source>:<native>". The native part is 1..200 bytes of printable UTF-8
// without whitespace or bidi controls; it may contain '/' (ids travel in query strings only).
func ParseEntityID(s string) (SourceID, string, error) {
	src, native, ok := strings.Cut(s, ":")
	if !ok || !ValidSourceID(src) || native == "" || len(native) > MaxNativeIDBytes || !utf8.ValidString(native) {
		return "", "", errBadEntityID
	}
	for _, r := range native {
		if !unicode.IsPrint(r) || unicode.IsSpace(r) || isBidiControl(r) {
			return "", "", errBadEntityID
		}
	}
	return SourceID(src), native, nil
}

func MakeEntityID(src SourceID, native string) EntityID { return EntityID(string(src) + ":" + native) }

// NewDashboardID returns "d_" plus 12 lower-case base32 characters.
func NewDashboardID() string { return "d_" + randomBase32(12) }

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("homedash: crypto/rand: " + err.Error())
	}
	return b
}

func randomBase32(n int) string {
	s := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes(n)))
	return s[:n]
}

func randomHex(nBytes int) string { return hex.EncodeToString(randomBytes(nBytes)) }
```
- [ ] Implement `entity.go`: the `Kind` type and the 24 constants of backend §4 (`KindSwitch` … `KindGeneric`), `AllKinds() []Kind` in declaration order; `CapabilitySpec`, `MediaInfo`, `State`, `Entity`, `SourceStatus` exactly as backend §4 (same fields and JSON tags); `type StateUpdate struct { Native string; State State }`; constants
  `StatusDisabled="disabled" StatusIdle="idle" StatusStarting="starting" StatusLive="live" StatusPolling="polling" StatusDegraded="degraded" StatusUnavailable="unavailable"`, `ReasonCatalogFailed="catalog_failed" ReasonCrashed="crashed" ReasonStopped="stopped"`.
- [ ] Implement `capability.go`: the `Capability` type and constants of backend §4 (all of them, same string values), plus:
```go
const (ArgNumber = "number"; ArgString = "string"; ArgBool = "bool")

// ArgSpec is one argument. EntityRange: the entity's CapabilitySpec Min/Max/Step apply instead of Min/Max.
type ArgSpec struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	EntityRange bool     `json:"entity_range,omitempty"`
}

// CapabilityInfo describes a capability. Sensitive here means "always confirmed"; adapters mark
// context-dependent cases (scripts, entry covers, AdGuard off) on the entity's CapabilitySpec.
type CapabilityInfo struct {
	Name        Capability `json:"name"`
	Command     bool       `json:"command"`
	Args        []ArgSpec  `json:"args"`
	Sensitive   bool       `json:"sensitive,omitempty"`
	KioskDenied bool       `json:"kiosk_denied,omitempty"`
	Continuous  bool       `json:"continuous,omitempty"` // latest-wins slot
}
```
  `CapabilityRegistry() []CapabilityInfo` returns a copy of a package slice in this order and content (`v` = `{Name:"value",Type:number,EntityRange:true}`, `opt` = `{Name:"option",Type:string}`, `on` = `{Name:"on",Type:bool}`; `Args` is `[]ArgSpec{}` when empty):

  | Capability | Args | Flags |
  |---|---|---|
  | toggle, turn_on, turn_off | — | command |
  | brightness | v (Min 0, Max 100) | continuous |
  | color_temp | kelvin number Min 1000 Max 12000 EntityRange | continuous |
  | color | h number 0..360; s number 0..100 | continuous |
  | effect, hvac_mode, preset_mode, fan_mode | opt | |
  | set_temperature | v | continuous |
  | set_temperature_range | low, high: number EntityRange | |
  | open, close, stop | — | |
  | set_position, set_tilt | v (0..100) | continuous |
  | lock | — | |
  | unlock | — | sensitive, kiosk_denied |
  | set_percentage | v (0..100) | continuous |
  | oscillate | on | |
  | media_play_pause, media_next, media_previous | — | |
  | volume_set | v (0..100) | continuous |
  | volume_mute | on | |
  | source_select | opt | |
  | press, activate | — | |
  | set_value | v | continuous |
  | select_option | opt | |
  | set_humidity | v (0..100) | continuous |
  | set_mode | opt | |
  | vacuum_start, vacuum_pause, vacuum_return | — | |
  | container_start | — | kiosk_denied |
  | container_stop, container_restart | — | sensitive, kiosk_denied |
  | stream, snapshot, history, numeric | — | Command false |

  All rows except the last have `Command: true`. Then:
```go
func LookupCapability(c Capability) (CapabilityInfo, bool) // linear scan of the registry

func (i CapabilityInfo) hasArg(name string) bool {
	for _, a := range i.Args {
		if a.Name == name {
			return true
		}
	}
	return false
}

func invalidArgs(format string, a ...any) *Error {
	return &Error{Code: CodeInvalidArgs, Message: fmt.Sprintf(format, a...)}
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, !math.IsNaN(t) && !math.IsInf(t, 0)
	case int:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
	}
	return 0, false
}

// ValidateArgs checks a command's arguments against the registry and the entity's spec
// (min/max/step/options). It rejects unknown and missing arguments.
func ValidateArgs(spec CapabilitySpec, args map[string]any) error {
	info, ok := LookupCapability(spec.Name)
	if !ok || !info.Command {
		return newError(CodeUnsupportedCommand, "not a command capability")
	}
	for k := range args {
		if !info.hasArg(k) {
			return invalidArgs("unknown argument %q", k)
		}
	}
	nums := map[string]float64{}
	for _, a := range info.Args {
		v, present := args[a.Name]
		if !present {
			return invalidArgs("missing argument %q", a.Name)
		}
		switch a.Type {
		case ArgNumber:
			f, ok := toFloat(v)
			if !ok {
				return invalidArgs("%s must be a number", a.Name)
			}
			lo, hi, step := a.Min, a.Max, (*float64)(nil)
			if a.EntityRange {
				if spec.Min != nil {
					lo = spec.Min
				}
				if spec.Max != nil {
					hi = spec.Max
				}
				step = spec.Step
			}
			if (lo != nil && f < *lo) || (hi != nil && f > *hi) {
				return invalidArgs("%s is out of range", a.Name)
			}
			if step != nil && *step > 0 {
				base := 0.0
				if lo != nil {
					base = *lo
				}
				q := (f - base) / *step
				if math.Abs(q-math.Round(q)) > 1e-6 {
					return invalidArgs("%s is not a multiple of %g", a.Name, *step)
				}
			}
			nums[a.Name] = f
		case ArgString:
			s, ok := v.(string)
			if !ok || s == "" || utf8.RuneCountInString(s) > 64 || ContainsControl(s) {
				return invalidArgs("%s must be a short text", a.Name)
			}
			if len(spec.Options) > 0 && !slices.Contains(spec.Options, s) {
				return invalidArgs("%s is not an offered option", a.Name)
			}
		case ArgBool:
			if _, ok := v.(bool); !ok {
				return invalidArgs("%s must be true or false", a.Name)
			}
		}
	}
	if spec.Name == CapSetTemperatureRange && nums["low"] >= nums["high"] {
		return invalidArgs("low must be below high")
	}
	return nil
}
```
- [ ] Implement `sanitize.go`:
```go
package homedash

const (
	MaxAttrStringRunes = 256
	MaxAttrListItems   = 64
	MaxAttrBytes       = 8 << 10
	MaxStateTextRunes  = 255
	MaxNameRunes       = 120
	MaxCatalogEntities = 2000
)

var attrKeyRe = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

func isBidiControl(r rune) bool {
	return (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) || r == 0x200E || r == 0x200F || r == 0x061C
}

// CleanText drops invalid UTF-8, control and bidi characters, folds whitespace runs to one space,
// trims, and cuts to max runes (max <= 0: no cut).
func CleanText(s string, max int) string {
	s = strings.ToValidUTF8(s, "")
	var b strings.Builder
	n, pending := 0, false
	for _, r := range s {
		if unicode.IsSpace(r) {
			pending = b.Len() > 0
			continue
		}
		if unicode.IsControl(r) || isBidiControl(r) {
			continue
		}
		if pending {
			if max > 0 && n+1 >= max {
				break
			}
			b.WriteByte(' ')
			n++
			pending = false
		}
		if max > 0 && n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// ContainsControl reports invalid UTF-8, control or bidi characters.
func ContainsControl(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}
	for _, r := range s {
		if unicode.IsControl(r) || isBidiControl(r) {
			return true
		}
	}
	return false
}

// ContainsURL reports text that carries a URL or script scheme; documents accept no URLs.
func ContainsURL(s string) bool {
	l := strings.ToLower(s)
	for _, m := range []string{"://", "javascript:", "data:", "vbscript:"} {
		if strings.Contains(l, m) {
			return true
		}
	}
	return strings.HasPrefix(strings.TrimSpace(l), "//")
}

// BoundAttributes keeps scalar and list-of-scalar values under keys matching ^[a-z0-9_]{1,64}$,
// cleans strings (256 runes), cuts lists (64), drops maps and non-finite numbers, and keeps keys
// in sorted order while the JSON stays within 8 KiB.
func BoundAttributes(in map[string]any) map[string]any {
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out, size := map[string]any{}, 2
	for _, k := range keys {
		if !attrKeyRe.MatchString(k) {
			continue
		}
		v, ok := boundValue(in[k], true)
		if !ok {
			continue
		}
		enc, err := json.Marshal(v)
		if err != nil || size+len(k)+4+len(enc) > MaxAttrBytes {
			continue
		}
		out[k], size = v, size+len(k)+4+len(enc)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func boundValue(v any, allowList bool) (any, bool) {
	switch t := v.(type) {
	case nil, bool:
		return t, true
	case string:
		return CleanText(t, MaxAttrStringRunes), true
	case float64, int, int64, float32, json.Number:
		var f float64
		switch n := t.(type) {
		case float64:
			f = n
		case float32:
			f = float64(n)
		case int:
			f = float64(n)
		case int64:
			f = float64(n)
		case json.Number:
			var err error
			if f, err = n.Float64(); err != nil {
				return nil, false
			}
		}
		return f, !math.IsNaN(f) && !math.IsInf(f, 0)
	case []string:
		list := make([]any, len(t))
		for i, s := range t {
			list[i] = s
		}
		return boundValue(list, allowList)
	case []any:
		if !allowList {
			return nil, false
		}
		if len(t) > MaxAttrListItems {
			t = t[:MaxAttrListItems]
		}
		out := make([]any, 0, len(t))
		for _, e := range t {
			if b, ok := boundValue(e, false); ok {
				out = append(out, b)
			}
		}
		return out, true
	}
	return nil, false
}
```
- [ ] Implement `source.go` exactly as backend §4 for `Sink`, `Source`, `HistorySource`, `SnapshotSource`, `Sample`, `Factory`, `Command`; and
```go
type CommandResult struct {
	Expected   *State `json:"expected,omitempty"`
	Superseded bool   `json:"superseded,omitempty"` // a newer value for the same control replaced this one before it was sent
}
```
  Doc comments: `Sink.Update` never blocks; `Source.Run` runs while the hub has interest and returns on ctx.Done; `Source.Catalog` works whether running or not (≤ 2000 entities); `Source.Command` must re-check its own integration gates; factories must not block (they run under the hub lock).
- [ ] Implement `principal.go`:
```go
package homedash

type PrincipalKind string

const (
	PrincipalSession PrincipalKind = "session"
	PrincipalKiosk   PrincipalKind = "kiosk"
	PrincipalAgent   PrincipalKind = "agent"
)

// Principal is who sends a request. Key is the rate-limit key (session hash, kiosk device id).
// A kiosk principal with a nil Allowed may read and command nothing.
type Principal struct {
	Kind        PrincipalKind
	Key         string
	DeviceID    string
	DashboardID string
	ViewOnly    bool
	Allowed     *AllowedSet
}

// AllowedSet maps an entity to the commands a kiosk may send; an empty map means read only.
type AllowedSet struct {
	Commands map[EntityID]map[Capability]bool
}

func (a *AllowedSet) Has(id EntityID) bool {
	if a == nil {
		return false
	}
	_, ok := a.Commands[id]
	return ok
}

func (a *AllowedSet) Allows(id EntityID, c Capability) bool { return a != nil && a.Commands[id][c] }
```
- [ ] Implement `runtime_config.go`:
```go
package homedash

// RuntimeConfig is the immutable snapshot the server builds from config.Config. Adapters never read
// config.Config. Source Enabled is already "homedash.sources.<s>.enabled && integration enabled".
type RuntimeConfig struct {
	Enabled, ReadOnly, DesktopReadOnly, EggMode bool
	KioskEnabled, KioskAllowControl             bool
	Limits                                      Limits
	History                                     HistoryConfig
	Sources                                     map[SourceID]SourceSettings
}

type Limits struct{ MaxDashboards, MaxStreams, MaxKioskStreams, MaxSubscriptions, CommandsPerMinute int }
type HistoryConfig struct{ RingPoints, RingEntities, SampleSeconds int }

// SourceSettings: ConfigKey is the key under homedash.sources (e.g. "home_assistant") and is set
// only for sources that have a control switch; Params/Lists carry adapter settings (url, token,
// readonly, allowed_services ...).
type SourceSettings struct {
	Enabled     bool
	Control     bool
	PollSeconds int
	ConfigKey   string
	Params      map[string]string
	Lists       map[string][]string
}

func (c RuntimeConfig) active() bool { return c.Enabled && !c.EggMode }

func (c RuntimeConfig) withDefaults() RuntimeConfig {
	set := func(p *int, def int) {
		if *p <= 0 {
			*p = def
		}
	}
	set(&c.Limits.MaxDashboards, 50)
	set(&c.Limits.MaxStreams, 32)
	set(&c.Limits.MaxKioskStreams, 16)
	set(&c.Limits.MaxSubscriptions, 300)
	set(&c.Limits.CommandsPerMinute, 60)
	set(&c.History.RingPoints, 360)
	set(&c.History.RingEntities, 500)
	set(&c.History.SampleSeconds, 60)
	return c
}

// Fingerprint changes whenever anything a source runtime depends on changes.
func (s SourceSettings) Fingerprint() string {
	h := sha256.New()
	fmt.Fprintf(h, "%t|%t|%d|%s|", s.Enabled, s.Control, s.PollSeconds, s.ConfigKey)
	for _, k := range sortedKeys(s.Params) {
		fmt.Fprintf(h, "p:%q=%q|", k, s.Params[k])
	}
	for _, k := range sortedKeys(s.Lists) {
		fmt.Fprintf(h, "l:%q=%q|", k, s.Lists[k])
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
```
- [ ] Run `go test ./internal/homedash/ -count=1 -v` → PASS `TestParseEntityID`, `TestValidateArgsPerCapability`, `TestSanitizeBoundsTextAndAttributes`; `go vet ./internal/homedash/`.
- [ ] Commit `git add internal/homedash`; `feat(homedash): core ids, entity, capability and sanitize types`.
- **Done when:** three tests pass; package imports only stdlib.

### Task 3: B2 widget catalog and dashboard schema (freeze point)

**Files:** Create `catalog.go`, `catalog_widgets.go`, `dashboard.go`, `dashboard_validate.go`, `dashboard_layout.go`, `dashboard_migrate.go`, `dashboard_patch.go`, `catalog_test.go`, `dashboard_test.go`, `testdata/widget_catalog.json` (all under `internal/homedash/`).
**Read first:** main document §4.2, §4.3 (wins over the parts); backend part §7; visual part §1.1 and §4 (sizes, cut list, aliases); golden convention `internal/ecalc/localize_test.go:14`.

**Catalog types (`catalog.go`)**
```go
const CatalogVersion = 1
const SchemaVersion = 1

type WidgetCatalog struct {
	Version       int              `json:"version"`
	Hash          string           `json:"hash"` // sha256 hex of the JSON with Hash ""
	SchemaVersion int              `json:"schema_version"`
	Grid          GridSpec         `json:"grid"`
	Limits        DocumentLimits   `json:"limits"`
	Families      []string         `json:"families"`
	Kinds         []Kind           `json:"kinds"`
	Capabilities  []CapabilityInfo `json:"capabilities"`
	PaletteTokens []string         `json:"palette_tokens"`
	CommonOptions []OptionSpec     `json:"common_options"`
	Widgets       []WidgetType     `json:"widgets"`
}
type GridColumns struct{ LG, MD, SM int } // json lg, md, sm
type GridSpec struct {
	Columns     GridColumns `json:"columns"`
	RowUnitMin  int         `json:"row_unit_min"`
	RowUnitMax  int         `json:"row_unit_max"`
	AutoCellMin int         `json:"auto_cell_min"`
	AutoCellMax int         `json:"auto_cell_max"`
	GapMax      int         `json:"gap_max"`
	MaxY        int         `json:"max_y"`
}
type DocumentLimits struct {
	Pages, WidgetsPerPage, Widgets, DocumentBytes, Bindings, Options int
} // json pages, widgets_per_page, widgets, document_bytes, bindings, options
type WidgetType struct {
	Type     string                  `json:"type"`
	Category string                  `json:"category"` // control|sensor|climate|media|camera|layout|status
	Variants []Variant               `json:"variants"`
	Bindings []BindingSpec           `json:"bindings"`
	Options  []OptionSpec            `json:"options"`
	MaxSize  Size                    `json:"max_size"`
	Commands map[string][]Capability `json:"commands"` // binding -> commands the widget may issue (kiosk AllowedSet)
}
// FamilyAlias: family -> what that family draws instead. A value naming another variant of the same
// type means the gallery hides this tile in that family; "scale" means the hairline scale primitive.
type Variant struct {
	ID          string            `json:"id"`
	MinSize     Size              `json:"min_size"`
	DefaultSize Size              `json:"default_size"`
	FamilyAlias map[string]string `json:"family_alias,omitempty"`
}
type BindingSpec struct {
	Name          string       `json:"name"`
	Required      bool         `json:"required"`
	Kinds         []Kind       `json:"kinds"` // empty = any kind
	AnyCapability []Capability `json:"any_capability,omitempty"`
}
type OptionSpec struct {
	Key      string   `json:"key"`
	Type     string   `json:"type"` // bool|int|number|string|enum|color|page
	Default  any      `json:"default"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Step     *float64 `json:"step,omitempty"`
	Enum     []string `json:"enum,omitempty"`
	MaxLen   int      `json:"max_len,omitempty"`
	Required bool     `json:"required,omitempty"`
}
type Size struct{ W, H int } // json w, h

func Catalog() WidgetCatalog // built once (sync.Once), Hash set; callers treat it as read-only
func (c WidgetCatalog) Type(name string) (WidgetType, bool)
func (t WidgetType) Variant(id string) (Variant, bool)
func (t WidgetType) Binding(name string) (BindingSpec, bool)
func (c WidgetCatalog) Option(t WidgetType, key string) (OptionSpec, bool) // type options, then common options
func (c WidgetCatalog) MarshalGolden() ([]byte, error)                   // json.MarshalIndent(c, "", "  ") + "\n"
func catalogHash(c WidgetCatalog) string                                 // c.Hash = ""; sha256 of json.Marshal(c)
```
Constant data in `catalog.go`: `Grid{Columns{12,8,4}, 48, 200, 56, 180, 32, 500}`; `Limits{12, 120, 400, 262144, 8, 32}`; `Families: glass, tactile, minimal, instrument`; `Kinds: AllKinds()`; `Capabilities: CapabilityRegistry()`; `CommonOptions: [{accent, color, ""}]` (per-widget accent override, main §6); `PaletteTokens` (palette-owned hex colours of visual §1.1, minus the family-painted `surface` and the derived `accent-soft`; 39 names): `canvas canvas-2 scrim surface-solid surface-raised sunken field ink ink-muted ink-faint ink-on-accent on-ink selected-ink btn-ink accent accent-2 on selected btn ok warn alert info pending unavailable ok-ink warn-ink alert-ink info-ink heat cool ct-2200 ct-4000 ct-6500 light-on edge-tint shadow-ink hairline focus`.

**Catalog data (`catalog_widgets.go`)** — transcribe this table in this order into `func widgetTypes() []WidgetType` using small helpers (`variant(id, minW, minH, defW, defH)`, `bind(name, required, kinds...)`, `optInt(key, def, lo, hi)`, etc.). `*` = required binding; `num` = `AnyCapability: [numeric]`; `hist` = `AnyCapability: [history]`; `onoff` = `toggle, turn_on, turn_off`. Sizes are min / default (W×H). Every `Options` and `Commands` is non-nil (empty slice / empty map).

| # | Type | Cat. | Variants: min / default | Max | Bindings | Commands | Options |
|---|---|---|---|---|---|---|---|
| 1 | switch | control | pill 1×1/2×1; rocker 1×2/2×2; push 1×1/2×1; tile 1×1/2×2 | 4×4 | primary* [switch, light, fan, humidifier] | primary: onoff | — |
| 2 | light | control | slider 2×1/4×1; fader 1×2/1×3; dial 2×2/2×2; tile 1×1/2×2 | 6×4 | primary* [light] | primary: onoff, brightness, color_temp, color, effect | — |
| 3 | climate | climate | dial 2×2/3×3; stepper 2×1/2×1; scale 1×2/1×3 | 6×6 | primary* [climate] | primary: set_temperature, set_temperature_range, hvac_mode, preset_mode, fan_mode | — |
| 4 | temperature | climate | numeral 1×1/2×1; thermometer 1×2/1×2 (alias minimal→scale); comfort 2×2/2×2 | 4×4 | primary* [sensor] num; humidity [sensor] num | — | decimals int 1 (0..2); show_minmax bool true |
| 5 | sensor | sensor | numeral 1×1/2×1; sparkline 2×1/2×2; icon 1×1/1×1 | 6×4 | primary* [sensor] | — | decimals int 1 (0..3); hours int 24 (1..168) |
| 6 | gauge | sensor | arc 2×2/2×2 (alias tactile→needle); needle 2×2/2×2; bar 2×1/3×1; tank 1×2/1×2 | 6×6 | primary* [sensor] num | — | min number 0; max number 100; warn number null; alert number null; decimals int 0 (0..3) |
| 7 | binary | sensor | indicator 1×1/2×1; lamp 1×1/1×1 | 4×2 | primary* [binary_sensor, presence] | — | — |
| 8 | cover | control | shutter 1×2/2×2; buttons 1×1/1×2 | 4×6 | primary* [cover] | primary: open, close, stop, set_position, set_tilt | — |
| 9 | action | control | button 1×1/2×1 | 4×2 | primary* [scene, script, button] | primary: activate, press | — |
| 10 | select | control | segmented 2×1/3×1; list 1×1/2×1 | 6×2 | primary* [select] | primary: select_option | — |
| 11 | number | control | stepper 2×1/2×1; slider 2×1/2×1 | 4×2 | primary* [number] | primary: set_value | — |
| 12 | fan | control | speed 2×1/2×2 | 4×4 | primary* [fan] | primary: onoff, set_percentage, preset_mode, oscillate | — |
| 13 | lock | control | slide 2×1/3×1 | 6×2 | primary* [lock] | primary: lock, unlock | — |
| 14 | media | media | compact 2×1/4×1; cover 2×2/3×2 | 8×4 | primary* [media_player] | primary: media_play_pause, media_next, media_previous, volume_set, volume_mute, source_select, toggle | — |
| 15 | chart | sensor | line 3×2/4×2; timeline 3×1/4×1 | 12×6 | s1* any hist; s2, s3, s4 any hist | — | hours int 24 (1..168) |
| 16 | camera | camera | frame 2×2/4×3 | 12×8 | primary* [camera] | — | live bool true; refresh_seconds int 10 (2..300) |
| 17 | status_list | status | list 2×2/2×3 | 6×8 | e1*, e2*, e3* any; e4..e8 any | — | — |
| 18 | battery | status | glyph 1×1/1×1 | 2×2 | primary* [sensor] num | — | — |
| 19 | room | status | card 2×2/3×2 | 6×4 | temperature [sensor] num; humidity [sensor] num; toggle1*, toggle2*, toggle3, toggle4 [switch, light, fan, humidifier] | toggle1..toggle4: onoff | — |
| 20 | clock | layout | digital 1×1/2×1; analog 2×2/2×2 (alias instrument→digital) | 6×4 | — | — | show_seconds bool false; show_date bool true |
| 21 | heading | layout | title 1×1/4×1 | 12×2 | — | — | align enum start (start, center) |
| 22 | nav | layout | button 1×1/1×1 | 4×2 | — | — | page page "" required |
| 23 | weather | climate | compact 2×1/2×1; forecast 3×2/4×2 | 8×4 | primary* [weather] | — | — |
| 24 | entity | status | tile 1×1/2×1 | 4×2 | primary* any | primary: toggle | — |

Option justifications (keep the list this small): decimals = numeral precision (H7/H8); show_minmax = H7 sub-line; hours = sparkline/chart range (H8, chart); gauge min/max/warn/alert = scale and zone bands (H5/H6); camera live/refresh = snapshot vs live frame (§4.4 camera); clock seconds/date = digital vs date line; heading align; nav page = navigation target. `null` defaults mean "unset" (nullable number). Room toggles and status-list entries are bindings because the document's bindings are a name→id map.

Example of the expected Go shape:
```go
{Type: "switch", Category: "control", MaxSize: Size{4, 4},
	Variants: []Variant{variant("pill", 1, 1, 2, 1), variant("rocker", 1, 2, 2, 2), variant("push", 1, 1, 2, 1), variant("tile", 1, 1, 2, 1)},
	Bindings: []BindingSpec{bind("primary", true, KindSwitch, KindLight, KindFan, KindHumidifier)},
	Options:  []OptionSpec{},
	Commands: map[string][]Capability{"primary": onOff()}},
```

**Document (`dashboard.go`)** — types (JSON names as in main §4.2):
```go
type Dashboard struct {
	SchemaVersion    int            `json:"schema_version"`
	ID               string         `json:"id"`
	Slug             string         `json:"slug"`
	Name             string         `json:"name"`
	Icon             string         `json:"icon,omitempty"`
	Revision         int64          `json:"revision"`
	ApprovedRevision int64          `json:"approved_revision"`
	UpdatedAt        time.Time      `json:"updated_at"`
	SavedBy          string         `json:"saved_by,omitempty"`
	Look             Look           `json:"look"`
	Grid             Grid           `json:"grid"`
	Pages            []Page         `json:"pages"`
	Kiosk            *KioskSettings `json:"kiosk,omitempty"`
}
type Look struct {
	Family      string   `json:"family"`
	Palette     Palette  `json:"palette"`
	Backdrop    Backdrop `json:"backdrop"`
	Density     string   `json:"density"`
	RadiusScale float64  `json:"radius_scale"`
}
type Palette struct {
	Mode     string            `json:"mode"`
	Custom   *CustomPalette    `json:"custom,omitempty"` // only with mode custom
	Snapshot map[string]string `json:"snapshot,omitempty"` // only with mode theme
}
type CustomPalette struct {
	Accent     string `json:"accent"`
	SurfaceHue int    `json:"surface_hue"`
	Scheme     string `json:"scheme"`
}
type Backdrop struct{ Kind string `json:"kind"` }
type Grid struct {
	RowUnit RowUnit `json:"row_unit"`
	Gap     *int    `json:"gap"`
	Fit     string  `json:"fit"`
}
type RowUnit struct{ Auto bool; PX int } // JSON "auto" or a whole number
type Page struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Icon    string   `json:"icon,omitempty"`
	Compact *bool    `json:"compact,omitempty"` // nil = true
	Widgets []Widget `json:"widgets"`
	Layouts Layouts  `json:"layouts"`
}
type Widget struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Variant  string            `json:"variant"`
	Family   string            `json:"family,omitempty"`
	Title    string            `json:"title,omitempty"`
	Bindings map[string]string `json:"bindings"`
	Options  map[string]any    `json:"options"`
	Actions  WidgetActions     `json:"actions"`
	Confirm  bool              `json:"confirm"`
}
type WidgetActions struct {
	Tap  *Action `json:"tap,omitempty"`
	Hold *Action `json:"hold,omitempty"`
}
type Action struct {
	Do   string `json:"do"`
	Page string `json:"page,omitempty"`
}
type Layouts struct {
	LG []LayoutItem `json:"lg"`
	MD []LayoutItem `json:"md,omitempty"` // nil = automatic
	SM []LayoutItem `json:"sm,omitempty"`
}
type LayoutItem struct {
	ID string `json:"id"`
	X  int    `json:"x"`
	Y  int    `json:"y"`
	W  int    `json:"w"`
	H  int    `json:"h"`
}
type KioskSettings struct {
	StartPage           string  `json:"start_page"`
	IdleDimSeconds      int     `json:"idle_dim_seconds"`
	IdleDimLevel        float64 `json:"idle_dim_level"`
	PageRotationSeconds int     `json:"page_rotation_seconds"`
	ScreenSaver         string  `json:"screen_saver"`
	ShowClock           *bool   `json:"show_clock,omitempty"`
}

func (r RowUnit) MarshalJSON() ([]byte, error) {
	if r.Auto || r.PX == 0 {
		return []byte(`"auto"`), nil
	}
	return json.Marshal(r.PX)
}

func (r *RowUnit) UnmarshalJSON(b []byte) error {
	if string(b) == `"auto"` {
		*r = RowUnit{Auto: true}
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return errors.New(`row_unit must be "auto" or a whole number of pixels`)
	}
	*r = RowUnit{PX: n}
	return nil
}
```
Behaviour rules:
1. `DecodeDashboard(data []byte) (Dashboard, error)`: more than `Catalog().Limits.DocumentBytes` → `*Error{Code: CodeLimitExceeded}`; decode with `DisallowUnknownFields`; trailing data after the value or any decode error → `*Error{Code: CodeInvalidDocument, Issues: [{Path: "", Code: "bad_json", Message: <decoder message>}]}`.
2. `Normalize(d *Dashboard)` fills only absent values: schema_version 0→1; family "glass"; palette mode "theme"; backdrop "theme"; density "comfortable"; radius 0→1; row_unit PX 0→Auto; fit "scroll"; per page `Compact` nil→true, nil widgets/LG→empty slices; per widget nil bindings/options→empty maps, nil tap→`{do:"default"}`, nil hold→`{do:"more_info"}`; `Kiosk` nil → `{StartPage: first page id, IdleDimSeconds: 120, IdleDimLevel: 0.3, PageRotationSeconds: 0, ScreenSaver: "clock", ShowClock: true}`; present kiosk with empty start page → first page id; nil `ShowClock` → true. Option defaults are never written into documents.
3. `cloneDashboard(d Dashboard) (Dashboard, error)`: JSON round trip.

**Migrate (`dashboard_migrate.go`)**:
```go
// Migrate brings doc to SchemaVersion in memory. A newer document stays unchanged and returns
// schema_too_new (the caller shows it read-only).
func Migrate(doc *Dashboard) error {
	switch {
	case doc.SchemaVersion > SchemaVersion:
		return &Error{Code: CodeSchemaTooNew, Message: fmt.Sprintf("dashboard schema %d is newer than supported %d", doc.SchemaVersion, SchemaVersion)}
	case doc.SchemaVersion <= 0:
		doc.SchemaVersion = SchemaVersion
	}
	return nil
}
```

**Validation (`dashboard_validate.go`)** — full code:
```go
const (
	IssueRequired = "required"; IssueTooLong = "too_long"; IssueBadFormat = "bad_format"
	IssueOutOfRange = "out_of_range"; IssueNotAllowed = "not_allowed"; IssueControlChars = "control_chars"
	IssueURLNotAllowed = "url_not_allowed"; IssueDuplicateID = "duplicate_id"; IssueUnknownType = "unknown_type"
	IssueUnknownVariant = "unknown_variant"; IssueUnknownBinding = "unknown_binding"; IssueBindingRequired = "binding_required"
	IssueBadEntityID = "bad_entity_id"; IssueTooMany = "too_many"; IssueUnknownOption = "unknown_option"
	IssueBadOption = "bad_option"; IssuePageNotFound = "page_not_found"; IssueToggleUnsupported = "toggle_unsupported"
	IssueLayoutMissing = "layout_missing"; IssueLayoutDuplicate = "layout_duplicate"; IssueLayoutUnknown = "layout_unknown_widget"
	IssueLayoutOverlap = "layout_overlap"; IssueLayoutBounds = "layout_out_of_bounds"; IssueSize = "size_out_of_range"
	IssueDocumentTooLarge = "document_too_large"
	WarnEntityUnknown = "entity_unknown"; WarnKindMismatch = "kind_mismatch"; WarnCapabilityMissing = "capability_missing"
)

type Report struct {
	Issues   []Issue `json:"issues"`
	Warnings []Issue `json:"warnings"`
}

func (r Report) OK() bool { return len(r.Issues) == 0 }

var hexColourRe = regexp.MustCompile(`^#[0-9a-f]{6}$`)

type validator struct {
	cat   WidgetCatalog
	known func(EntityID) (Entity, bool)
	r     Report
}

func (v *validator) err(path, code, msg string)  { v.r.Issues = append(v.r.Issues, Issue{path, code, msg}) }
func (v *validator) warn(path, code, msg string) { v.r.Warnings = append(v.r.Warnings, Issue{path, code, msg}) }

func (v *validator) text(path, s string, min, max int) {
	switch n := utf8.RuneCountInString(s); {
	case n < min:
		v.err(path, IssueRequired, "required")
	case n > max:
		v.err(path, IssueTooLong, fmt.Sprintf("at most %d characters", max))
	case ContainsControl(s):
		v.err(path, IssueControlChars, "control characters are not allowed")
	case ContainsURL(s):
		v.err(path, IssueURLNotAllowed, "URLs are not allowed")
	}
}

func (v *validator) enum(path, s string, allowed ...string) {
	if !slices.Contains(allowed, s) {
		v.err(path, IssueNotAllowed, "one of "+strings.Join(allowed, ", "))
	}
}

func (v *validator) rangeF(path string, f, lo, hi float64) {
	if f < lo || f > hi {
		v.err(path, IssueOutOfRange, fmt.Sprintf("must be %g–%g", lo, hi))
	}
}

// Validate checks a document against the catalog. known (may be nil) reports entities the hub
// knows; unknown entities and kind/capability mismatches are warnings, never errors.
func Validate(doc Dashboard, cat WidgetCatalog, known func(EntityID) (Entity, bool)) Report {
	v := &validator{cat: cat, known: known, r: Report{Issues: []Issue{}, Warnings: []Issue{}}}
	if raw, err := json.Marshal(doc); err != nil || len(raw) > cat.Limits.DocumentBytes {
		v.err("", IssueDocumentTooLarge, "the document is larger than 256 KiB")
	}
	if doc.SchemaVersion != SchemaVersion {
		v.err("/schema_version", IssueOutOfRange, "unsupported schema version")
	}
	if doc.ID != "" && !ValidDashboardID(doc.ID) {
		v.err("/id", IssueBadFormat, "invalid dashboard id")
	}
	if !ValidSlug(doc.Slug) {
		v.err("/slug", IssueBadFormat, "lower-case letters, digits and dashes, at most 48")
	}
	v.text("/name", doc.Name, 1, 80)
	if doc.Icon != "" && !ValidIcon(doc.Icon) {
		v.err("/icon", IssueBadFormat, "invalid icon id")
	}
	v.look(doc.Look)
	v.grid(doc.Grid)
	if len(doc.Pages) == 0 {
		v.err("/pages", IssueRequired, "at least one page")
	} else if len(doc.Pages) > cat.Limits.Pages {
		v.err("/pages", IssueTooMany, fmt.Sprintf("at most %d pages", cat.Limits.Pages))
	}
	pages, total := map[string]bool{}, 0
	for i, p := range doc.Pages {
		path := fmt.Sprintf("/pages/%d", i)
		if !ValidPageID(p.ID) {
			v.err(path+"/id", IssueBadFormat, "invalid page id")
		} else if pages[p.ID] {
			v.err(path+"/id", IssueDuplicateID, "page id used twice")
		}
		pages[p.ID] = true
		v.text(path+"/name", p.Name, 1, 40)
		if p.Icon != "" && !ValidIcon(p.Icon) {
			v.err(path+"/icon", IssueBadFormat, "invalid icon id")
		}
		if len(p.Widgets) > cat.Limits.WidgetsPerPage {
			v.err(path+"/widgets", IssueTooMany, fmt.Sprintf("at most %d widgets per page", cat.Limits.WidgetsPerPage))
		}
		total += len(p.Widgets)
	}
	if total > cat.Limits.Widgets {
		v.err("/pages", IssueTooMany, fmt.Sprintf("at most %d widgets", cat.Limits.Widgets))
	}
	seen := map[string]bool{}
	for i, p := range doc.Pages {
		path := fmt.Sprintf("/pages/%d", i)
		for j, w := range p.Widgets {
			v.widget(fmt.Sprintf("%s/widgets/%d", path, j), w, pages, seen)
		}
		v.layout(path+"/layouts/lg", p, p.Layouts.LG, cat.Grid.Columns.LG)
		if p.Layouts.MD != nil {
			v.layout(path+"/layouts/md", p, p.Layouts.MD, cat.Grid.Columns.MD)
		}
		if p.Layouts.SM != nil {
			v.layout(path+"/layouts/sm", p, p.Layouts.SM, cat.Grid.Columns.SM)
		}
	}
	v.kiosk(doc.Kiosk, pages)
	return v.r
}

func (v *validator) look(l Look) {
	v.enum("/look/family", l.Family, v.cat.Families...)
	v.enum("/look/palette/mode", l.Palette.Mode, "theme", "dark", "light", "custom")
	switch c := l.Palette.Custom; {
	case l.Palette.Mode == "custom" && c == nil:
		v.err("/look/palette/custom", IssueRequired, "a custom palette needs accent, surface_hue and scheme")
	case l.Palette.Mode == "custom":
		if !hexColourRe.MatchString(c.Accent) {
			v.err("/look/palette/custom/accent", IssueBadFormat, "#rrggbb in lower case")
		}
		v.rangeF("/look/palette/custom/surface_hue", float64(c.SurfaceHue), 0, 359)
		v.enum("/look/palette/custom/scheme", c.Scheme, "dark", "light")
	case c != nil:
		v.err("/look/palette/custom", IssueNotAllowed, "only with mode custom")
	}
	if len(l.Palette.Snapshot) > 0 {
		if l.Palette.Mode != "theme" {
			v.err("/look/palette/snapshot", IssueNotAllowed, "only with mode theme")
		}
		if len(l.Palette.Snapshot) > 40 {
			v.err("/look/palette/snapshot", IssueTooMany, "at most 40 tokens")
		}
		for _, k := range sortedKeys(l.Palette.Snapshot) {
			p := "/look/palette/snapshot/" + k
			if !slices.Contains(v.cat.PaletteTokens, k) {
				v.err(p, IssueNotAllowed, "unknown colour token")
			} else if !hexColourRe.MatchString(l.Palette.Snapshot[k]) {
				v.err(p, IssueBadFormat, "#rrggbb in lower case")
			}
		}
	}
	v.enum("/look/backdrop/kind", l.Backdrop.Kind, "theme", "solid", "gradient")
	v.enum("/look/density", l.Density, "compact", "comfortable", "spacious")
	v.rangeF("/look/radius_scale", l.RadiusScale, 0.5, 1.5)
}

func (v *validator) grid(g Grid) {
	if !g.RowUnit.Auto {
		v.rangeF("/grid/row_unit", float64(g.RowUnit.PX), float64(v.cat.Grid.RowUnitMin), float64(v.cat.Grid.RowUnitMax))
	}
	if g.Gap != nil {
		v.rangeF("/grid/gap", float64(*g.Gap), 0, float64(v.cat.Grid.GapMax))
	}
	v.enum("/grid/fit", g.Fit, "scroll", "fit_screen")
}

func (v *validator) widget(path string, w Widget, pages, seen map[string]bool) {
	if !ValidWidgetID(w.ID) {
		v.err(path+"/id", IssueBadFormat, "invalid widget id")
	} else if seen[w.ID] {
		v.err(path+"/id", IssueDuplicateID, "widget id used twice")
	}
	seen[w.ID] = true
	wt, ok := v.cat.Type(w.Type)
	if !ok {
		v.err(path+"/type", IssueUnknownType, "unknown widget type")
		return
	}
	if _, ok := wt.Variant(w.Variant); !ok {
		v.err(path+"/variant", IssueUnknownVariant, "unknown variant")
	}
	if w.Family != "" {
		v.enum(path+"/family", w.Family, v.cat.Families...)
	}
	v.text(path+"/title", w.Title, 0, 80)
	if len(w.Bindings) > v.cat.Limits.Bindings {
		v.err(path+"/bindings", IssueTooMany, "too many bindings")
	}
	for _, b := range wt.Bindings {
		if b.Required && w.Bindings[b.Name] == "" {
			v.err(path+"/bindings/"+b.Name, IssueBindingRequired, "this binding is required")
		}
	}
	for _, name := range sortedKeys(w.Bindings) {
		p, raw := path+"/bindings/"+name, w.Bindings[name]
		b, ok := wt.Binding(name)
		switch {
		case !ok:
			v.err(p, IssueUnknownBinding, "the widget has no such binding")
		case raw == "":
		default:
			if _, _, err := ParseEntityID(raw); err != nil {
				v.err(p, IssueBadEntityID, "invalid entity id")
			} else {
				v.entity(p, EntityID(raw), b)
			}
		}
	}
	v.options(path+"/options", wt, w.Options, pages)
	v.action(path+"/actions/tap", w.Actions.Tap, wt, pages)
	v.action(path+"/actions/hold", w.Actions.Hold, wt, pages)
}

func (v *validator) entity(p string, id EntityID, b BindingSpec) {
	if v.known == nil {
		return
	}
	e, ok := v.known(id)
	if !ok {
		v.warn(p, WarnEntityUnknown, "the entity is not known (offline or removed)")
		return
	}
	if len(b.Kinds) > 0 && !slices.Contains(b.Kinds, e.Kind) {
		v.warn(p, WarnKindMismatch, "the entity kind does not fit this binding")
	}
	if len(b.AnyCapability) > 0 && !slices.ContainsFunc(e.Capabilities, func(c CapabilitySpec) bool { return slices.Contains(b.AnyCapability, c.Name) }) {
		v.warn(p, WarnCapabilityMissing, "the entity lacks a capability this binding needs")
	}
}

func (v *validator) options(path string, wt WidgetType, opts map[string]any, pages map[string]bool) {
	if len(opts) > v.cat.Limits.Options {
		v.err(path, IssueTooMany, "too many options")
	}
	for _, s := range append(slices.Clone(wt.Options), v.cat.CommonOptions...) {
		if val, ok := opts[s.Key]; s.Required && (!ok || val == nil || val == "") {
			v.err(path+"/"+s.Key, IssueRequired, "required")
		}
	}
	for _, k := range sortedKeys(opts) {
		s, ok := v.cat.Option(wt, k)
		if !ok {
			v.err(path+"/"+k, IssueUnknownOption, "unknown option")
			continue
		}
		v.option(path+"/"+k, s, opts[k], pages)
	}
	if wt.Type == "gauge" {
		lo, okLo := toFloat(opts["min"])
		hi, okHi := toFloat(opts["max"])
		if okLo && okHi && lo >= hi {
			v.err(path+"/max", IssueBadOption, "max must be above min")
		}
	}
}

func (v *validator) option(p string, s OptionSpec, val any, pages map[string]bool) {
	bad := func() { v.err(p, IssueBadOption, "wrong type for "+s.Type) }
	switch s.Type {
	case "bool":
		if _, ok := val.(bool); !ok {
			bad()
		}
	case "int", "number":
		if val == nil && s.Default == nil {
			return
		}
		f, ok := toFloat(val)
		if !ok || (s.Type == "int" && f != math.Trunc(f)) {
			bad()
			return
		}
		if (s.Min != nil && f < *s.Min) || (s.Max != nil && f > *s.Max) {
			v.err(p, IssueOutOfRange, "out of range")
		}
	case "string":
		str, ok := val.(string)
		if !ok {
			bad()
			return
		}
		max := s.MaxLen
		if max == 0 || max > 200 {
			max = 200
		}
		v.text(p, str, 0, max)
	case "enum":
		if str, ok := val.(string); !ok || !slices.Contains(s.Enum, str) {
			v.err(p, IssueNotAllowed, "one of "+strings.Join(s.Enum, ", "))
		}
	case "color":
		if str, ok := val.(string); !ok || (str != "" && !hexColourRe.MatchString(str)) {
			v.err(p, IssueBadFormat, "#rrggbb in lower case")
		}
	case "page":
		str, ok := val.(string)
		switch {
		case !ok:
			bad()
		case str != "" && !pages[str]:
			v.err(p, IssuePageNotFound, "no such page")
		}
	}
}

func (v *validator) action(p string, a *Action, wt WidgetType, pages map[string]bool) {
	if a == nil {
		return
	}
	v.enum(p+"/do", a.Do, "default", "toggle", "more_info", "navigate", "none")
	switch {
	case a.Do == "navigate" && a.Page == "":
		v.err(p+"/page", IssueRequired, "navigate needs a page")
	case a.Do == "navigate" && !pages[a.Page]:
		v.err(p+"/page", IssuePageNotFound, "no such page")
	case a.Do != "navigate" && a.Page != "":
		v.err(p+"/page", IssueNotAllowed, "only with navigate")
	}
	if a.Do == "toggle" && !slices.Contains(wt.Commands["primary"], CapToggle) {
		v.err(p+"/do", IssueToggleUnsupported, "this widget cannot toggle")
	}
}

func (v *validator) kiosk(k *KioskSettings, pages map[string]bool) {
	if k == nil {
		v.err("/kiosk", IssueRequired, "required")
		return
	}
	if !pages[k.StartPage] {
		v.err("/kiosk/start_page", IssuePageNotFound, "no such page")
	}
	v.rangeF("/kiosk/idle_dim_seconds", float64(k.IdleDimSeconds), 0, 3600)
	v.rangeF("/kiosk/idle_dim_level", k.IdleDimLevel, 0.05, 1)
	if k.PageRotationSeconds != 0 {
		v.rangeF("/kiosk/page_rotation_seconds", float64(k.PageRotationSeconds), 10, 3600)
	}
	v.enum("/kiosk/screen_saver", k.ScreenSaver, "none", "clock", "black")
}
```
**Layout and Autopack (`dashboard_layout.go`)** — full code:
```go
type cell struct{ x, y int }

func (v *validator) layout(path string, p Page, items []LayoutItem, cols int) {
	byID := map[string]Widget{}
	for _, w := range p.Widgets {
		byID[w.ID] = w
	}
	seen, occ := map[string]bool{}, map[cell]string{}
	for i, it := range items {
		ip := fmt.Sprintf("%s/%d", path, i)
		w, ok := byID[it.ID]
		switch {
		case !ok:
			v.err(ip, IssueLayoutUnknown, "no widget with this id on the page")
			continue
		case seen[it.ID]:
			v.err(ip, IssueLayoutDuplicate, "widget placed twice")
			continue
		}
		seen[it.ID] = true
		if it.X < 0 || it.Y < 0 || it.Y > v.cat.Grid.MaxY || it.W < 1 || it.H < 1 || it.X+it.W > cols {
			v.err(ip, IssueLayoutBounds, fmt.Sprintf("outside the %d-column grid", cols))
			continue
		}
		if wt, ok := v.cat.Type(w.Type); ok {
			if vr, ok := wt.Variant(w.Variant); ok && (it.W < vr.MinSize.W || it.H < vr.MinSize.H || it.W > wt.MaxSize.W || it.H > wt.MaxSize.H) {
				v.err(ip, IssueSize, fmt.Sprintf("size must be %dx%d to %dx%d", vr.MinSize.W, vr.MinSize.H, wt.MaxSize.W, wt.MaxSize.H))
			}
		}
		overlap := ""
		for x := it.X; x < it.X+it.W && overlap == ""; x++ {
			for y := it.Y; y < it.Y+it.H; y++ {
				if other := occ[cell{x, y}]; other != "" {
					overlap = other
					break
				}
			}
		}
		if overlap != "" {
			v.err(ip, IssueLayoutOverlap, "overlaps "+overlap)
			continue
		}
		mark(occ, it)
	}
	for _, w := range p.Widgets {
		if !seen[w.ID] {
			v.err(path, IssueLayoutMissing, "widget "+w.ID+" has no place in this layout")
		}
	}
}

func mark(occ map[cell]string, it LayoutItem) {
	for x := it.X; x < it.X+it.W; x++ {
		for y := it.Y; y < it.Y+it.H; y++ {
			occ[cell{x, y}] = it.ID
		}
	}
}

func free(occ map[cell]string, x, y int, s Size) bool {
	for dx := 0; dx < s.W; dx++ {
		for dy := 0; dy < s.H; dy++ {
			if occ[cell{x + dx, y + dy}] != "" {
				return false
			}
		}
	}
	return true
}

// Autopack gives every widget without an lg item a place: widget order, the variant's default size
// (clamped to the lg columns), first free position scanning rows top-down and columns left-right.
// It never moves existing items and never fills md or sm.
func Autopack(doc *Dashboard, cat WidgetCatalog) {
	cols, maxY := cat.Grid.Columns.LG, cat.Grid.MaxY
	for pi := range doc.Pages {
		p := &doc.Pages[pi]
		occ, placed := map[cell]string{}, map[string]bool{}
		for _, it := range p.Layouts.LG {
			placed[it.ID] = true
			if it.W > 0 && it.H > 0 && it.X >= 0 && it.Y >= 0 {
				mark(occ, it)
			}
		}
		for _, w := range p.Widgets {
			if placed[w.ID] || !ValidWidgetID(w.ID) {
				continue
			}
			size := Size{2, 1}
			if wt, ok := cat.Type(w.Type); ok {
				if vr, ok := wt.Variant(w.Variant); ok {
					size = vr.DefaultSize
				}
			}
			size.W = min(size.W, cols)
			it := LayoutItem{ID: w.ID, X: 0, Y: maxY + 1, W: size.W, H: size.H}
		search:
			for y := 0; y <= maxY; y++ {
				for x := 0; x+size.W <= cols; x++ {
					if free(occ, x, y, size) {
						it.X, it.Y = x, y
						break search
					}
				}
			}
			mark(occ, it)
			p.Layouts.LG = append(p.Layouts.LG, it)
			placed[w.ID] = true
		}
	}
}
```
**Patch (`dashboard_patch.go`)** — full code:
```go
const MaxPatchOps = 50

type PatchOp struct {
	Op       string  `json:"op"` // add_widget|update_widget|move_widget|remove_widget|add_page|rename_page|remove_page|set_name|set_look
	PageID   string  `json:"page_id,omitempty"`
	WidgetID string  `json:"widget_id,omitempty"`
	Widget   *Widget `json:"widget,omitempty"`
	X        *int    `json:"x,omitempty"`
	Y        *int    `json:"y,omitempty"`
	W        *int    `json:"w,omitempty"`
	H        *int    `json:"h,omitempty"`
	Name     string  `json:"name,omitempty"`
	Look     *Look   `json:"look,omitempty"`
	Confirm  *bool   `json:"confirm,omitempty"`
}

// ApplyPatch applies ops to a copy of doc. The caller runs Normalize, Autopack and Validate next.
func ApplyPatch(doc Dashboard, ops []PatchOp) (Dashboard, error) {
	if len(ops) == 0 || len(ops) > MaxPatchOps {
		return doc, newError(CodeInvalidPatch, fmt.Sprintf("1 to %d operations", MaxPatchOps))
	}
	out, err := cloneDashboard(doc)
	if err != nil {
		return doc, err
	}
	for i, op := range ops {
		if err := applyOp(&out, op); err != nil {
			return doc, newError(CodeInvalidPatch, fmt.Sprintf("op %d (%s): %s", i, op.Op, err))
		}
	}
	return out, nil
}

func findPage(d *Dashboard, id string) *Page {
	for i := range d.Pages {
		if d.Pages[i].ID == id {
			return &d.Pages[i]
		}
	}
	return nil
}

func findWidget(d *Dashboard, id string) (*Page, int) {
	for i := range d.Pages {
		for j := range d.Pages[i].Widgets {
			if d.Pages[i].Widgets[j].ID == id {
				return &d.Pages[i], j
			}
		}
	}
	return nil, -1
}

func nextWidgetID(d *Dashboard) string {
	for n := 1; ; n++ {
		id := fmt.Sprintf("w%d", n)
		if p, _ := findWidget(d, id); p == nil {
			return id
		}
	}
}

func dropItem(items []LayoutItem, id string) []LayoutItem {
	return slices.DeleteFunc(items, func(it LayoutItem) bool { return it.ID == id })
}

func removeWidget(p *Page, idx int) Widget {
	w := p.Widgets[idx]
	p.Widgets = slices.Delete(p.Widgets, idx, idx+1)
	p.Layouts.LG, p.Layouts.MD, p.Layouts.SM = dropItem(p.Layouts.LG, w.ID), dropItem(p.Layouts.MD, w.ID), dropItem(p.Layouts.SM, w.ID)
	return w
}

func placement(op PatchOp) (*LayoutItem, error) {
	n := 0
	for _, p := range []*int{op.X, op.Y, op.W, op.H} {
		if p != nil {
			n++
		}
	}
	switch n {
	case 0:
		return nil, nil
	case 4:
		return &LayoutItem{X: *op.X, Y: *op.Y, W: *op.W, H: *op.H}, nil
	}
	return nil, errors.New("x, y, w and h go together")
}

func applyOp(d *Dashboard, op PatchOp) error {
	switch op.Op {
	case "add_widget":
		p := findPage(d, op.PageID)
		if p == nil {
			return errors.New("page not found")
		}
		if op.Widget == nil {
			return errors.New("widget is required")
		}
		w := *op.Widget
		if w.ID == "" {
			w.ID = nextWidgetID(d)
		} else if q, _ := findWidget(d, w.ID); q != nil {
			return errors.New("widget id already used")
		}
		pl, err := placement(op)
		if err != nil {
			return err
		}
		p.Widgets = append(p.Widgets, w)
		if pl != nil {
			pl.ID = w.ID
			p.Layouts.LG = append(p.Layouts.LG, *pl)
		}
	case "update_widget":
		p, idx := findWidget(d, op.WidgetID)
		if p == nil {
			return errors.New("widget not found")
		}
		if op.Widget == nil && op.Confirm == nil {
			return errors.New("widget or confirm is required")
		}
		dst := &p.Widgets[idx]
		if src := op.Widget; src != nil {
			if src.Type != "" {
				dst.Type = src.Type
			}
			if src.Variant != "" {
				dst.Variant = src.Variant
			}
			if src.Family != "" {
				dst.Family = src.Family
			}
			if src.Title != "" {
				dst.Title = src.Title
			}
			if src.Bindings != nil {
				dst.Bindings = src.Bindings
			}
			if src.Options != nil {
				dst.Options = src.Options
			}
			if src.Actions.Tap != nil {
				dst.Actions.Tap = src.Actions.Tap
			}
			if src.Actions.Hold != nil {
				dst.Actions.Hold = src.Actions.Hold
			}
		}
		if op.Confirm != nil {
			dst.Confirm = *op.Confirm
		}
	case "move_widget":
		p, idx := findWidget(d, op.WidgetID)
		if p == nil {
			return errors.New("widget not found")
		}
		pl, err := placement(op)
		if err != nil {
			return err
		}
		if op.PageID != "" && op.PageID != p.ID {
			dst := findPage(d, op.PageID)
			if dst == nil {
				return errors.New("page not found")
			}
			dst.Widgets = append(dst.Widgets, removeWidget(p, idx))
			p = dst
		} else if pl == nil {
			return errors.New("give a page or a position")
		}
		if pl != nil {
			pl.ID = op.WidgetID
			p.Layouts.LG = append(dropItem(p.Layouts.LG, op.WidgetID), *pl)
		}
	case "remove_widget":
		p, idx := findWidget(d, op.WidgetID)
		if p == nil {
			return errors.New("widget not found")
		}
		removeWidget(p, idx)
	case "add_page":
		if !ValidPageID(op.PageID) {
			return errors.New("invalid page id")
		}
		if findPage(d, op.PageID) != nil {
			return errors.New("page id already used")
		}
		d.Pages = append(d.Pages, Page{ID: op.PageID, Name: op.Name, Widgets: []Widget{}, Layouts: Layouts{LG: []LayoutItem{}}})
	case "rename_page":
		p := findPage(d, op.PageID)
		if p == nil {
			return errors.New("page not found")
		}
		p.Name = op.Name
	case "remove_page":
		if findPage(d, op.PageID) == nil {
			return errors.New("page not found")
		}
		if len(d.Pages) == 1 {
			return errors.New("the last page cannot be removed")
		}
		d.Pages = slices.DeleteFunc(d.Pages, func(p Page) bool { return p.ID == op.PageID })
		if d.Kiosk != nil && d.Kiosk.StartPage == op.PageID {
			d.Kiosk.StartPage = d.Pages[0].ID
		}
	case "set_name":
		d.Name = op.Name
	case "set_look":
		if op.Look == nil {
			return errors.New("look is required")
		}
		d.Look = *op.Look
	default:
		return errors.New("unknown operation")
	}
	return nil
}
```

**Tests**
- [ ] `catalog_test.go` (package `homedash`):
```go
var update = flag.Bool("update", false, "rewrite testdata/widget_catalog.json from the Go catalog")

func TestWidgetCatalogGolden(t *testing.T) {
	got, err := Catalog().MarshalGolden()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "widget_catalog.json")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if string(bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))) != string(got) {
		t.Fatal("widget catalog differs from testdata/widget_catalog.json (rerun with -update after an intended change)")
	}
	if h := catalogHash(Catalog()); h != Catalog().Hash || len(h) != 64 {
		t.Fatalf("hash %q", h)
	}
}

func TestWidgetCatalogInvariants(t *testing.T) {
	c := Catalog()
	variants, types := 0, map[string]bool{}
	page := map[string]bool{"main": true}
	for _, wt := range c.Widgets {
		if types[wt.Type] {
			t.Errorf("type %s twice", wt.Type)
		}
		types[wt.Type] = true
		ids := map[string]bool{}
		for _, v := range wt.Variants {
			variants++
			if ids[v.ID] {
				t.Errorf("%s/%s twice", wt.Type, v.ID)
			}
			ids[v.ID] = true
			if v.MinSize.W < 1 || v.MinSize.H < 1 || v.DefaultSize.W < v.MinSize.W || v.DefaultSize.H < v.MinSize.H ||
				v.DefaultSize.W > wt.MaxSize.W || v.DefaultSize.H > wt.MaxSize.H || wt.MaxSize.W > c.Grid.Columns.LG {
				t.Errorf("%s/%s sizes %+v %+v max %+v", wt.Type, v.ID, v.MinSize, v.DefaultSize, wt.MaxSize)
			}
			for fam, to := range v.FamilyAlias {
				if !slices.Contains(c.Families, fam) {
					t.Errorf("%s/%s alias family %s", wt.Type, v.ID, fam)
				}
				if _, ok := wt.Variant(to); !ok && to != "scale" {
					t.Errorf("%s/%s alias target %s", wt.Type, v.ID, to)
				}
			}
		}
		for _, b := range wt.Bindings {
			for _, k := range b.Kinds {
				if !slices.Contains(c.Kinds, k) {
					t.Errorf("%s.%s kind %s", wt.Type, b.Name, k)
				}
			}
		}
		for name, caps := range wt.Commands {
			if _, ok := wt.Binding(name); !ok {
				t.Errorf("%s commands for unknown binding %s", wt.Type, name)
			}
			for _, cp := range caps {
				if info, ok := LookupCapability(cp); !ok || !info.Command {
					t.Errorf("%s.%s command %s", wt.Type, name, cp)
				}
			}
		}
		v := &validator{cat: c}
		for _, o := range wt.Options {
			if o.Default != nil {
				v.option("/"+wt.Type+"/"+o.Key, o, o.Default, page)
			}
		}
		if len(v.r.Issues) > 0 {
			t.Errorf("%s option defaults invalid: %+v", wt.Type, v.r.Issues)
		}
	}
	if len(c.Widgets) != 24 || variants != 47 {
		t.Fatalf("%d types, %d variants; want 24 and 47", len(c.Widgets), variants)
	}
	if len(c.PaletteTokens) > 40 || c.Grid.Columns != (GridColumns{12, 8, 4}) {
		t.Fatal("palette tokens or grid columns")
	}
}
```
- [ ] `dashboard_test.go` (package `homedash`):
```go
func validDoc() Dashboard {
	d := Dashboard{SchemaVersion: 1, Slug: "living-room", Name: "Living room", Icon: "sofa",
		Look: Look{Family: "glass", Palette: Palette{Mode: "theme", Snapshot: map[string]string{"surface-solid": "#1a212b", "ink": "#f1f4f9"}},
			Backdrop: Backdrop{Kind: "theme"}, Density: "comfortable", RadiusScale: 1},
		Grid: Grid{RowUnit: RowUnit{Auto: true}, Fit: "scroll"},
		Pages: []Page{{ID: "main", Name: "Main", Icon: "home", Widgets: []Widget{
			{ID: "w1", Type: "switch", Variant: "rocker", Family: "tactile", Title: "Lamp", Bindings: map[string]string{"primary": "ha:switch.lamp"},
				Actions: WidgetActions{Tap: &Action{Do: "default"}, Hold: &Action{Do: "more_info"}}},
			{ID: "w2", Type: "gauge", Variant: "arc", Bindings: map[string]string{"primary": "system:cpu"}, Options: map[string]any{"min": 0.0, "max": 100.0}},
		}, Layouts: Layouts{LG: []LayoutItem{{ID: "w1", X: 0, Y: 0, W: 2, H: 2}, {ID: "w2", X: 2, Y: 0, W: 2, H: 2}}}}}}
	Normalize(&d)
	return d
}

func hasIssue(list []Issue, path, code string) bool {
	return slices.ContainsFunc(list, func(i Issue) bool { return i.Path == path && i.Code == code })
}

func TestValidateDashboard(t *testing.T) {
	w := func(d *Dashboard, i int) *Widget { return &d.Pages[0].Widgets[i] }
	wp := "/pages/0/widgets/"
	for _, tc := range []struct {
		name, path, code string
		mut              func(*Dashboard)
	}{
		{"valid", "", "", func(*Dashboard) {}},
		{"unknown type", wp + "0/type", IssueUnknownType, func(d *Dashboard) { w(d, 0).Type = "toaster" }},
		{"unknown variant", wp + "0/variant", IssueUnknownVariant, func(d *Dashboard) { w(d, 0).Variant = "flip" }},
		{"missing binding", wp + "0/bindings/primary", IssueBindingRequired, func(d *Dashboard) { delete(w(d, 0).Bindings, "primary") }},
		{"bad entity id", wp + "0/bindings/primary", IssueBadEntityID, func(d *Dashboard) { w(d, 0).Bindings["primary"] = "nocolon" }},
		{"bad widget id", wp + "0/id", IssueBadFormat, func(d *Dashboard) { w(d, 0).ID = "W 1" }},
		{"duplicate widget id", wp + "1/id", IssueDuplicateID, func(d *Dashboard) { w(d, 1).ID = "w1" }},
		{"overlap", "/pages/0/layouts/lg/1", IssueLayoutOverlap, func(d *Dashboard) { d.Pages[0].Layouts.LG[1].X = 1 }},
		{"out of bounds", "/pages/0/layouts/lg/1", IssueLayoutBounds, func(d *Dashboard) { d.Pages[0].Layouts.LG[1].X = 11 }},
		{"below min size", "/pages/0/layouts/lg/0", IssueSize, func(d *Dashboard) { d.Pages[0].Layouts.LG[0].H = 1 }},
		{"missing layout", "/pages/0/layouts/lg", IssueLayoutMissing, func(d *Dashboard) { d.Pages[0].Layouts.LG = d.Pages[0].Layouts.LG[:1] }},
		{"control chars", wp + "0/title", IssueControlChars, func(d *Dashboard) { w(d, 0).Title = "Lamp\x07" }},
		{"url", wp + "0/title", IssueURLNotAllowed, func(d *Dashboard) { w(d, 0).Title = "see https://x" }},
		{"colour format", "/look/palette/snapshot/ink", IssueBadFormat, func(d *Dashboard) { d.Look.Palette.Snapshot["ink"] = "#FFF" }},
		{"unknown token", "/look/palette/snapshot/brand", IssueNotAllowed, func(d *Dashboard) { d.Look.Palette.Snapshot["brand"] = "#000000" }},
		{"custom without inputs", "/look/palette/custom", IssueRequired, func(d *Dashboard) { d.Look.Palette = Palette{Mode: "custom"} }},
		{"navigate missing page", wp + "0/actions/tap/page", IssuePageNotFound, func(d *Dashboard) { w(d, 0).Actions.Tap = &Action{Do: "navigate", Page: "nope"} }},
		{"toggle unsupported", wp + "1/actions/tap/do", IssueToggleUnsupported, func(d *Dashboard) { w(d, 1).Actions.Tap = &Action{Do: "toggle"} }},
		{"option range", wp + "1/options/decimals", IssueOutOfRange, func(d *Dashboard) { w(d, 1).Options["decimals"] = 9.0 }},
		{"unknown option", wp + "1/options/glow", IssueUnknownOption, func(d *Dashboard) { w(d, 1).Options["glow"] = true }},
		{"gauge min max", wp + "1/options/max", IssueBadOption, func(d *Dashboard) { w(d, 1).Options["max"] = -1.0 }},
		{"kiosk start page", "/kiosk/start_page", IssuePageNotFound, func(d *Dashboard) { d.Kiosk.StartPage = "nope" }},
		{"too many pages", "/pages", IssueTooMany, func(d *Dashboard) {
			for i := 0; i < 12; i++ {
				d.Pages = append(d.Pages, Page{ID: fmt.Sprintf("p%d", i), Name: "P", Widgets: []Widget{}, Layouts: Layouts{LG: []LayoutItem{}}})
			}
		}},
	} {
		d := validDoc()
		tc.mut(&d)
		r := Validate(d, Catalog(), nil)
		if tc.code == "" {
			if !r.OK() {
				t.Errorf("%s: unexpected issues %+v", tc.name, r.Issues)
			}
			continue
		}
		if !hasIssue(r.Issues, tc.path, tc.code) {
			t.Errorf("%s: want %s at %s, got %+v", tc.name, tc.code, tc.path, r.Issues)
		}
	}
	r := Validate(validDoc(), Catalog(), func(id EntityID) (Entity, bool) { return Entity{}, false })
	if !r.OK() || !hasIssue(r.Warnings, wp+"0/bindings/primary", WarnEntityUnknown) {
		t.Fatalf("unknown entity must be a warning only: %+v", r)
	}
}

func TestAutopackFillsMissingLayoutsDeterministically(t *testing.T) {
	d := validDoc()
	p := &d.Pages[0]
	p.Widgets = append(p.Widgets, Widget{ID: "w3", Type: "light", Variant: "slider", Bindings: map[string]string{"primary": "ha:light.a"}},
		Widget{ID: "w4", Type: "clock", Variant: "digital"})
	Autopack(&d, Catalog())
	want := []LayoutItem{{"w1", 0, 0, 2, 2}, {"w2", 2, 0, 2, 2}, {"w3", 4, 0, 4, 1}, {"w4", 8, 0, 2, 1}}
	if !slices.Equal(p.Layouts.LG, want) {
		t.Fatalf("lg = %+v, want %+v", p.Layouts.LG, want)
	}
	again := validDoc()
	again.Pages[0].Widgets = p.Widgets
	Autopack(&again, Catalog())
	if !slices.Equal(again.Pages[0].Layouts.LG, want) {
		t.Fatal("autopack is not deterministic")
	}
	if r := Validate(d, Catalog(), nil); !r.OK() {
		t.Fatalf("packed document invalid: %+v", r.Issues)
	}
}

func TestMigrateRejectsNewerSchema(t *testing.T) {
	d := validDoc()
	d.SchemaVersion = 2
	var he *Error
	if err := Migrate(&d); !errors.As(err, &he) || he.Code != CodeSchemaTooNew || d.SchemaVersion != 2 {
		t.Fatalf("newer schema: %v", err)
	}
	d.SchemaVersion = 0
	if err := Migrate(&d); err != nil || d.SchemaVersion != SchemaVersion {
		t.Fatalf("draft: %v %d", err, d.SchemaVersion)
	}
	raw, _ := json.Marshal(validDoc())
	if _, err := DecodeDashboard(append(raw[:len(raw)-1], []byte(`,"extra":1}`)...)); err == nil {
		t.Fatal("unknown field accepted")
	}
	back, err := DecodeDashboard(raw)
	if err != nil || back.Grid.RowUnit != (RowUnit{Auto: true}) || back.Name != "Living room" {
		t.Fatalf("round trip: %v %+v", err, back.Grid)
	}
}

func TestApplyPatchOps(t *testing.T) {
	two, zero := 2, 0
	d, err := ApplyPatch(validDoc(), []PatchOp{
		{Op: "add_page", PageID: "kitchen", Name: "Kitchen"},
		{Op: "add_widget", PageID: "kitchen", Widget: &Widget{Type: "sensor", Variant: "numeral", Bindings: map[string]string{"primary": "ha:sensor.t"}}},
		{Op: "update_widget", WidgetID: "w1", Widget: &Widget{Title: "Desk lamp"}},
		{Op: "move_widget", WidgetID: "w2", X: &zero, Y: &two, W: &two, H: &two},
		{Op: "set_name", Name: "Home"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Home" || d.Pages[0].Widgets[0].Title != "Desk lamp" || len(d.Pages) != 2 || d.Pages[1].Widgets[0].ID != "w3" {
		t.Fatalf("patched: %+v", d)
	}
	Normalize(&d)
	Autopack(&d, Catalog())
	if r := Validate(d, Catalog(), nil); !r.OK() {
		t.Fatalf("patched document invalid: %+v", r.Issues)
	}
	for _, op := range []PatchOp{
		{Op: "remove_widget", WidgetID: "nope"},
		{Op: "remove_page", PageID: "main"},
		{Op: "add_widget", PageID: "main", Widget: &Widget{ID: "w1", Type: "clock", Variant: "digital"}},
		{Op: "move_widget", WidgetID: "w1", X: &zero},
		{Op: "explode"},
	} {
		var he *Error
		if _, err := ApplyPatch(validDoc(), []PatchOp{op}); !errors.As(err, &he) || he.Code != CodeInvalidPatch {
			t.Errorf("%s: %v", op.Op, err)
		}
	}
}
```
- [ ] Run `go test ./internal/homedash/ -count=1` → FAIL (undefined). Implement the files. Generate the golden: `go test ./internal/homedash/ -run TestWidgetCatalogGolden -update -count=1`. Check with `git check-ignore -v internal/homedash/testdata/widget_catalog.json` (use `git add -f` if ignored) and `python -m json.tool internal/homedash/testdata/widget_catalog.json > $null`.
- [ ] Run `go test ./internal/homedash/ -count=1 -v` → PASS `TestWidgetCatalogGolden`, `TestWidgetCatalogInvariants`, `TestValidateDashboard`, `TestAutopackFillsMissingLayoutsDeterministically`, `TestMigrateRejectsNewerSchema`, `TestApplyPatchOps` plus Task 2 tests.
- [ ] Commit `git add internal/homedash`; `feat(homedash): widget catalog v1 and dashboard schema (validate, autopack, migrate, patch)`.
- **Done when:** golden JSON exists with 24 types / 47 variants and all tests pass. This is the frontend freeze point: the lead announces the catalog hash.

### Task 4: B8 HA REST helpers and domain/attribute mapping

**Files:** Create `internal/tools/homeassistant_raw.go`, `internal/tools/homeassistant_raw_test.go`, `internal/homedash/sources/hass/{domains,attributes,mapping_test}.go`. Modify `internal/tools/homeassistant.go:43-75`.
**Read first:** backend §8.1 (domain table, attributes, commands), §2.4; main §10 (humidifier modes and vacuum commands deferred — wins over the backend table); `internal/tools/homeassistant.go:15-75, 248-291`; `internal/tools/http_limits.go:8-22`; `internal/tools/AGENTS.md:25-30`; `internal/tools/http_response_limits_regression_test.go:65-80`; audit tripwires `internal/audit/audit_manifests_test.go:181` and `credentialed_redirect_guard_test.go:75-76` (do not add `&http.Client{`).

- [ ] Write `internal/tools/homeassistant_raw_test.go` (package `tools`):
```go
func TestHARequestJSONKeepsSameOriginRedirectPolicy(t *testing.T) {
	var foreignHits atomic.Int32
	foreign := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignHits.Add(1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer foreign.Close()
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/same":
			http.Redirect(w, r, "/api/target", http.StatusFound)
		case "/api/target":
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/cross":
			http.Redirect(w, r, foreign.URL+"/api/target", http.StatusFound)
		case "/api/denied":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`secret echo`))
		}
	}))
	defer ha.Close()
	cfg, ctx := HAConfig{URL: ha.URL, AccessToken: "tok"}, context.Background()
	var out struct{ OK bool `json:"ok"` }
	if err := HARequestJSON(ctx, cfg, http.MethodGet, "/api/same", nil, &out); err != nil || !out.OK {
		t.Fatalf("same-origin redirect: %v %+v", err, out)
	}
	if err := HARequestJSON(ctx, cfg, http.MethodGet, "/api/cross", nil, &out); err == nil || foreignHits.Load() != 0 {
		t.Fatalf("cross-origin redirect followed: %v hits=%d", err, foreignHits.Load())
	}
	err := HARequestJSON(ctx, cfg, http.MethodGet, "/api/denied", nil, nil)
	var he *HAHTTPError
	if !errors.As(err, &he) || he.Status != http.StatusForbidden || strings.Contains(err.Error(), "secret") {
		t.Fatalf("non-2xx: %v", err)
	}
	if err := HARequestJSON(ctx, cfg, http.MethodGet, "/other", nil, nil); err == nil {
		t.Fatal("endpoint outside /api/ accepted")
	}
}

func TestHAGetBytesCapsSizeAndReturnsContentType(t *testing.T) {
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(bytes.Repeat([]byte{0xff}, 100))
	}))
	defer ha.Close()
	cfg := HAConfig{URL: ha.URL, AccessToken: "tok"}
	if _, _, err := HAGetBytes(context.Background(), cfg, "/api/camera_proxy/camera.x", 50); err == nil {
		t.Fatal("size cap not applied")
	}
	data, ct, err := HAGetBytes(context.Background(), cfg, "/api/camera_proxy/camera.x", 200)
	if err != nil || len(data) != 100 || ct != "image/jpeg" {
		t.Fatalf("%d bytes, %q, %v", len(data), ct, err)
	}
}
```
- [ ] Run `go test ./internal/tools/ -run '(?i)(HARequestJSON|HAGetBytes)' -count=1` → build failure.
- [ ] GitNexus impact on `haRequestContext` (callers in `homeassistant.go`; signature unchanged).
- [ ] In `homeassistant.go` replace the body of `haRequestContext` (lines 43-75) with a call to a new `haDo` that holds the old body unchanged except: signature `func haDo(ctx context.Context, cfg HAConfig, method, endpoint, body string, limit int64) ([]byte, int, http.Header, error)`, `readHTTPResponseBody(resp.Body, limit)`, and returns `resp.Header`. Doc comment: "haDo is the single Home Assistant HTTP path: base URL validation, token registration, the same-origin client and a response size limit."
```go
func haRequestContext(ctx context.Context, cfg HAConfig, method, endpoint string, body string) ([]byte, int, error) {
	data, code, _, err := haDo(ctx, cfg, method, endpoint, body, maxHTTPResponseSize)
	return data, code, err
}
```
- [ ] `homeassistant_raw.go`:
```go
package tools

// HAHTTPError reports a non-2xx Home Assistant answer without its body (bodies may echo request data).
type HAHTTPError struct{ Status int }

func (e *HAHTTPError) Error() string { return fmt.Sprintf("home assistant answered HTTP %d", e.Status) }

func haValidEndpoint(endpoint string) error {
	if !strings.HasPrefix(endpoint, "/api/") || strings.Contains(endpoint, "://") || strings.Contains(endpoint, "/../") || strings.HasSuffix(endpoint, "/..") {
		return errors.New("invalid Home Assistant endpoint")
	}
	return nil
}

// HARequestJSON calls the Home Assistant REST API through the shared client policy and decodes a
// 2xx JSON answer into out (nil out skips decoding). body is JSON-encoded when not nil.
func HARequestJSON(ctx context.Context, cfg HAConfig, method, endpoint string, body, out any) error {
	if err := haValidEndpoint(endpoint); err != nil {
		return err
	}
	payload := ""
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Home Assistant request: %w", err)
		}
		payload = string(raw)
	}
	data, code, _, err := haDo(ctx, cfg, method, endpoint, payload, maxHTTPResponseSize)
	if err != nil {
		return err
	}
	if code < 200 || code > 299 {
		return &HAHTTPError{Status: code}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode Home Assistant answer: %w", err)
	}
	return nil
}

// HAGetBytes reads a binary resource (camera_proxy) up to max bytes and returns its Content-Type.
func HAGetBytes(ctx context.Context, cfg HAConfig, endpoint string, max int64) ([]byte, string, error) {
	if err := haValidEndpoint(endpoint); err != nil {
		return nil, "", err
	}
	data, code, header, err := haDo(ctx, cfg, http.MethodGet, endpoint, "", max)
	if err != nil {
		return nil, "", err
	}
	if code < 200 || code > 299 {
		return nil, "", &HAHTTPError{Status: code}
	}
	return data, header.Get("Content-Type"), nil
}
```
- [ ] Write `internal/homedash/sources/hass/mapping_test.go` (package `hass`):
```go
func capNames(caps []homedash.CapabilitySpec) []string {
	out := []string{}
	for _, c := range caps {
		out = append(out, string(c.Name))
	}
	return out
}

func findCap(caps []homedash.CapabilitySpec, n homedash.Capability) (homedash.CapabilitySpec, bool) {
	for _, c := range caps {
		if c.Name == n {
			return c, true
		}
	}
	return homedash.CapabilitySpec{}, false
}

func TestHADomainMappingTable(t *testing.T) {
	cfg := tools.HAConfig{}
	for _, tc := range []struct {
		id, state string
		attrs     map[string]any
		kind      homedash.Kind
		caps      []string
	}{
		{"switch.a", "on", nil, homedash.KindSwitch, []string{"toggle", "turn_on", "turn_off", "history"}},
		{"input_boolean.a", "off", nil, homedash.KindSwitch, []string{"toggle", "turn_on", "turn_off", "history"}},
		{"light.a", "on", map[string]any{"supported_color_modes": []any{"color_temp", "hs"}, "effect_list": []any{"rainbow"}},
			homedash.KindLight, []string{"toggle", "turn_on", "turn_off", "brightness", "color_temp", "color", "effect"}},
		{"light.b", "on", map[string]any{"supported_color_modes": []any{"onoff"}}, homedash.KindLight, []string{"toggle", "turn_on", "turn_off"}},
		{"sensor.t", "21.5", nil, homedash.KindSensor, []string{"numeric", "history"}},
		{"sensor.s", "ok", nil, homedash.KindSensor, []string{}},
		{"climate.c", "heat", map[string]any{"temperature": 21.0, "hvac_modes": []any{"heat", "off"}}, homedash.KindClimate, []string{"set_temperature", "hvac_mode", "history"}},
		{"cover.c", "open", map[string]any{"current_position": 40.0}, homedash.KindCover, []string{"open", "close", "stop", "set_position"}},
		{"lock.l", "locked", nil, homedash.KindLock, []string{"lock", "unlock"}},
		{"scene.s", "x", nil, homedash.KindScene, []string{"activate"}},
		{"script.s", "off", nil, homedash.KindScript, []string{"activate"}},
		{"humidifier.h", "on", nil, homedash.KindHumidifier, []string{"toggle", "turn_on", "turn_off"}},
		{"vacuum.v", "docked", nil, homedash.KindVacuum, []string{}},
		{"camera.c", "idle", nil, homedash.KindCamera, []string{"snapshot"}},
		{"person.p", "home", nil, homedash.KindPresence, []string{}},
		{"automation.a", "on", nil, homedash.KindGeneric, []string{}},
		{"timer.t", "idle", nil, homedash.KindGeneric, []string{}},
	} {
		domain, _, _ := strings.Cut(tc.id, ".")
		spec, ok := MapDomain(domain)
		got := capNames(Capabilities(cfg, tc.id, tc.state, tc.attrs))
		if !ok || spec.Kind != tc.kind || !slices.Equal(got, tc.caps) {
			t.Errorf("%s: kind %s caps %v, want %s %v", tc.id, spec.Kind, got, tc.kind, tc.caps)
		}
	}
	if _, ok := MapDomain("alarm_control_panel"); ok {
		t.Error("alarm_control_panel must be excluded")
	}
	for id, ok := range map[string]bool{"light.kitchen": true, "_x.y_1": true, "Light.x": false, "light": false, "light.a-b": false} {
		if NativeIDValid(id) != ok {
			t.Errorf("NativeIDValid(%q)", id)
		}
	}
}

func TestHACapabilitiesHonourServicePolicy(t *testing.T) {
	garage := map[string]any{"current_position": 0.0, "device_class": "garage"}
	open, _ := findCap(Capabilities(tools.HAConfig{}, "cover.g", "closed", garage), homedash.CapOpen)
	if open.Allowed || open.BlockedBy != "home_assistant.allowed_services" || !open.Sensitive {
		t.Fatalf("default policy cover: %+v", open)
	}
	allowed, _ := findCap(Capabilities(tools.HAConfig{AllowedServices: []string{"cover.open_cover"}}, "cover.g", "closed", garage), homedash.CapOpen)
	if !allowed.Allowed {
		t.Fatalf("allowlisted cover.open_cover: %+v", allowed)
	}
	unlock, _ := findCap(Capabilities(tools.HAConfig{}, "lock.l", "locked", nil), homedash.CapUnlock)
	if unlock.Allowed || !unlock.Sensitive {
		t.Fatalf("unlock: %+v", unlock)
	}
	act, _ := findCap(Capabilities(tools.HAConfig{}, "script.s", "off", nil), homedash.CapActivate)
	if act.Allowed || !act.Sensitive {
		t.Fatalf("script: %+v", act)
	}
	ro, _ := findCap(Capabilities(tools.HAConfig{ReadOnly: true}, "switch.a", "on", nil), homedash.CapToggle)
	if ro.Allowed || ro.BlockedBy != "home_assistant.readonly" {
		t.Fatalf("readonly: %+v", ro)
	}
	bl, _ := findCap(Capabilities(tools.HAConfig{BlockedServices: []string{"switch.toggle"}}, "switch.a", "on", nil), homedash.CapToggle)
	if bl.Allowed || bl.BlockedBy != "home_assistant.blocked_services" {
		t.Fatalf("blocked: %+v", bl)
	}
}

func TestHAAttributeAllowlistDropsEntityPictureAndCoordinates(t *testing.T) {
	name, out := FilterAttributes(map[string]any{
		"friendly_name": "Kitchen\nLamp", "entity_picture": "/api/camera_proxy/x?token=abc", "latitude": 1.0, "longitude": 2.0,
		"gps_accuracy": 5.0, "access_token": "abc", "stream_url": "rtsp://x", "brightness": 255.0, "unit_of_measurement": "°C",
		"random_key": "x", "hvac_modes": []any{"heat", "off"},
	})
	if name != "Kitchen Lamp" {
		t.Fatalf("name %q", name)
	}
	for _, k := range []string{"entity_picture", "latitude", "longitude", "gps_accuracy", "access_token", "stream_url", "random_key", "brightness", "friendly_name"} {
		if _, ok := out[k]; ok {
			t.Errorf("%s kept", k)
		}
	}
	if out["brightness_pct"] != 100.0 || out["unit_of_measurement"] != "°C" || len(out["hvac_modes"].([]any)) != 2 {
		t.Fatalf("kept attributes: %v", out)
	}
}
```
- [ ] Implement `domains.go`:
```go
package hass

type CommandSpec struct {
	Cap     homedash.Capability
	Service string // Home Assistant service in the entity's domain
}

type DomainSpec struct {
	Domain   string
	Kind     homedash.Kind
	Commands []CommandSpec
	Read     []homedash.Capability
}

var nativeIDRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*\.[a-z0-9_]+$`)

// NativeIDValid checks a Home Assistant entity id (the native part of "ha:<id>").
func NativeIDValid(id string) bool { return len(id) <= homedash.MaxNativeIDBytes && nativeIDRe.MatchString(id) }

func cmds(groups ...[]CommandSpec) []CommandSpec {
	var out []CommandSpec
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func cs(c homedash.Capability, service string) []CommandSpec { return []CommandSpec{{c, service}} }

var (
	onOff   = []CommandSpec{{homedash.CapToggle, "toggle"}, {homedash.CapTurnOn, "turn_on"}, {homedash.CapTurnOff, "turn_off"}}
	history = []homedash.Capability{homedash.CapHistory}
)

var domainTable = map[string]DomainSpec{
	"switch":        {Kind: homedash.KindSwitch, Commands: onOff, Read: history},
	"input_boolean": {Kind: homedash.KindSwitch, Commands: onOff, Read: history},
	"light": {Kind: homedash.KindLight, Commands: cmds(onOff, cs(homedash.CapBrightness, "turn_on"), cs(homedash.CapColorTemp, "turn_on"),
		cs(homedash.CapColor, "turn_on"), cs(homedash.CapEffect, "turn_on"))},
	"sensor":        {Kind: homedash.KindSensor},
	"binary_sensor": {Kind: homedash.KindBinarySensor, Read: history},
	"climate": {Kind: homedash.KindClimate, Read: history, Commands: cmds(cs(homedash.CapSetTemperature, "set_temperature"),
		cs(homedash.CapSetTemperatureRange, "set_temperature"), cs(homedash.CapHVACMode, "set_hvac_mode"),
		cs(homedash.CapPresetMode, "set_preset_mode"), cs(homedash.CapFanMode, "set_fan_mode"))},
	"cover": {Kind: homedash.KindCover, Commands: cmds(cs(homedash.CapOpen, "open_cover"), cs(homedash.CapClose, "close_cover"),
		cs(homedash.CapStop, "stop_cover"), cs(homedash.CapSetPosition, "set_cover_position"), cs(homedash.CapSetTilt, "set_cover_tilt_position"))},
	"fan": {Kind: homedash.KindFan, Commands: cmds(onOff, cs(homedash.CapSetPercentage, "set_percentage"),
		cs(homedash.CapPresetMode, "set_preset_mode"), cs(homedash.CapOscillate, "oscillate"))},
	"lock": {Kind: homedash.KindLock, Commands: cmds(cs(homedash.CapLock, "lock"), cs(homedash.CapUnlock, "unlock"))},
	"media_player": {Kind: homedash.KindMediaPlayer, Commands: cmds(cs(homedash.CapPlayPause, "media_play_pause"),
		cs(homedash.CapNext, "media_next_track"), cs(homedash.CapPrevious, "media_previous_track"), cs(homedash.CapVolumeSet, "volume_set"),
		cs(homedash.CapVolumeMute, "volume_mute"), cs(homedash.CapSourceSelect, "select_source"), cs(homedash.CapToggle, "toggle"))},
	"scene":        {Kind: homedash.KindScene, Commands: cs(homedash.CapActivate, "turn_on")},
	"script":       {Kind: homedash.KindScript, Commands: cs(homedash.CapActivate, "turn_on")},
	"button":       {Kind: homedash.KindButton, Commands: cs(homedash.CapPress, "press")},
	"input_button": {Kind: homedash.KindButton, Commands: cs(homedash.CapPress, "press")},
	"number":       {Kind: homedash.KindNumber, Commands: cs(homedash.CapSetValue, "set_value"), Read: history},
	"input_number": {Kind: homedash.KindNumber, Commands: cs(homedash.CapSetValue, "set_value"), Read: history},
	"select":       {Kind: homedash.KindSelect, Commands: cs(homedash.CapSelectOption, "select_option")},
	"input_select": {Kind: homedash.KindSelect, Commands: cs(homedash.CapSelectOption, "select_option")},
	"humidifier":   {Kind: homedash.KindHumidifier, Commands: onOff}, // modes deferred (main §10)
	"vacuum":       {Kind: homedash.KindVacuum},                     // commands deferred (main §10)
	"camera":       {Kind: homedash.KindCamera, Read: []homedash.Capability{homedash.CapSnapshot}},
	"person":         {Kind: homedash.KindPresence},
	"device_tracker": {Kind: homedash.KindPresence},
	"sun":            {Kind: homedash.KindSun},
	"weather":        {Kind: homedash.KindWeather},
	"update":         {Kind: homedash.KindUpdate},
}

// MapDomain maps a Home Assistant domain. alarm_control_panel is excluded (ok=false); every other
// unknown domain (automation, timer, counter …) is a read-only generic entity.
func MapDomain(domain string) (DomainSpec, bool) {
	if domain == "alarm_control_panel" {
		return DomainSpec{}, false
	}
	s, ok := domainTable[domain]
	if !ok {
		s = DomainSpec{Kind: homedash.KindGeneric}
	}
	s.Domain = domain
	return s, true
}

// Capabilities lists one entity's capabilities. Command capabilities carry Allowed/BlockedBy from the
// shared Home Assistant service policy; attributes decide optional features, ranges and options.
func Capabilities(cfg tools.HAConfig, entityID, state string, attrs map[string]any) []homedash.CapabilitySpec {
	domain, _, _ := strings.Cut(entityID, ".")
	spec, ok := MapDomain(domain)
	if !ok {
		return nil
	}
	out := []homedash.CapabilitySpec{}
	for _, c := range spec.Commands {
		cap, keep := shapeCapability(spec.Kind, c.Cap, attrs)
		if !keep {
			continue
		}
		cap.Allowed = tools.HAServiceAllowed(cfg, domain, c.Service)
		if !cap.Allowed {
			cap.BlockedBy = blockedBy(cfg, domain+"."+c.Service)
		}
		out = append(out, cap)
	}
	for _, r := range spec.Read {
		out = append(out, homedash.CapabilitySpec{Name: r, Allowed: true})
	}
	if spec.Kind == homedash.KindSensor {
		if _, err := strconv.ParseFloat(state, 64); err == nil {
			out = append(out, homedash.CapabilitySpec{Name: homedash.CapNumeric, Allowed: true}, homedash.CapabilitySpec{Name: homedash.CapHistory, Allowed: true})
		}
	}
	return out
}

func blockedBy(cfg tools.HAConfig, full string) string {
	if cfg.ReadOnly {
		return "home_assistant.readonly"
	}
	for _, b := range cfg.BlockedServices {
		if strings.EqualFold(strings.TrimSpace(b), full) {
			return "home_assistant.blocked_services"
		}
	}
	return "home_assistant.allowed_services"
}

func fp(v float64) *float64 { return &v }

func num(attrs map[string]any, key string, def float64) *float64 {
	if f, ok := attrs[key].(float64); ok {
		return &f
	}
	return &def
}

func stringList(v any) []string {
	raw, _ := v.([]any)
	out := []string{}
	for _, e := range raw {
		if s, ok := e.(string); ok && len(out) < 64 {
			if s = homedash.CleanText(s, 64); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func has(attrs map[string]any, key string) bool { _, ok := attrs[key]; return ok }

func entryCover(attrs map[string]any) bool {
	dc, _ := attrs["device_class"].(string)
	return dc == "garage" || dc == "gate" || dc == "door"
}

func shapeCapability(kind homedash.Kind, c homedash.Capability, attrs map[string]any) (homedash.CapabilitySpec, bool) {
	s := homedash.CapabilitySpec{Name: c}
	modes := stringList(attrs["supported_color_modes"])
	anyMode := func(list ...string) bool { return slices.ContainsFunc(modes, func(m string) bool { return slices.Contains(list, m) }) }
	options := func(key string) bool { s.Options = stringList(attrs[key]); return len(s.Options) > 0 }
	switch c {
	case homedash.CapBrightness:
		s.Min, s.Max, s.Step = fp(0), fp(100), fp(1)
		return s, anyMode("brightness", "color_temp", "hs", "xy", "rgb", "rgbw", "rgbww", "white")
	case homedash.CapColorTemp:
		s.Min, s.Max, s.Step = num(attrs, "min_color_temp_kelvin", 2000), num(attrs, "max_color_temp_kelvin", 6500), fp(50)
		return s, anyMode("color_temp")
	case homedash.CapColor:
		return s, anyMode("hs", "xy", "rgb", "rgbw", "rgbww")
	case homedash.CapEffect:
		return s, options("effect_list")
	case homedash.CapSetTemperature, homedash.CapSetTemperatureRange:
		s.Min, s.Max, s.Step = num(attrs, "min_temp", 7), num(attrs, "max_temp", 35), num(attrs, "target_temp_step", 0.5)
		if c == homedash.CapSetTemperature {
			return s, has(attrs, "temperature") || !has(attrs, "target_temp_low")
		}
		return s, has(attrs, "target_temp_low")
	case homedash.CapHVACMode:
		return s, options("hvac_modes")
	case homedash.CapPresetMode:
		return s, options("preset_modes")
	case homedash.CapFanMode:
		return s, options("fan_modes")
	case homedash.CapOpen, homedash.CapClose:
		s.Sensitive = entryCover(attrs)
	case homedash.CapSetPosition:
		s.Min, s.Max, s.Step, s.Sensitive = fp(0), fp(100), fp(1), entryCover(attrs)
		return s, has(attrs, "current_position")
	case homedash.CapSetTilt:
		s.Min, s.Max, s.Step = fp(0), fp(100), fp(1)
		return s, has(attrs, "current_tilt_position")
	case homedash.CapSetPercentage:
		s.Min, s.Max, s.Step = fp(0), fp(100), num(attrs, "percentage_step", 1)
		return s, has(attrs, "percentage")
	case homedash.CapOscillate:
		return s, has(attrs, "oscillating")
	case homedash.CapVolumeSet:
		s.Min, s.Max, s.Step = fp(0), fp(100), fp(1) // percent; the adapter sends volume_level/100
		return s, has(attrs, "volume_level")
	case homedash.CapVolumeMute:
		return s, has(attrs, "is_volume_muted")
	case homedash.CapSourceSelect:
		return s, options("source_list")
	case homedash.CapSetValue:
		s.Min, s.Max, s.Step = num(attrs, "min", 0), num(attrs, "max", 100), num(attrs, "step", 1)
	case homedash.CapSelectOption:
		return s, options("options")
	case homedash.CapUnlock:
		s.Sensitive = true
	case homedash.CapActivate:
		s.Sensitive = kind == homedash.KindScript
	}
	return s, true
}
```
  Note the `light.b` case: attrs without colour modes make every light feature disappear, as the test expects; a light whose `supported_color_modes` is missing gets only on/off.
- [ ] Implement `attributes.go`:
```go
package hass

var attributeAllowlist = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`unit_of_measurement device_class state_class color_mode supported_color_modes
		color_temp_kelvin min_color_temp_kelvin max_color_temp_kelvin hs_color effect effect_list current_temperature
		temperature target_temp_low target_temp_high target_temp_step min_temp max_temp hvac_modes hvac_action
		preset_mode preset_modes fan_mode fan_modes current_position current_tilt_position percentage percentage_step
		oscillating direction media_title media_artist source source_list volume_level is_volume_muted options min max
		step mode battery_level installed_version latest_version next_rising next_setting elevation humidity pressure
		wind_speed wind_bearing`) {
		attributeAllowlist[k] = true
	}
}

func deniedAttribute(key string) bool {
	switch key {
	case "entity_picture", "access_token", "latitude", "longitude", "gps_accuracy":
		return true
	}
	return strings.Contains(key, "url") || strings.Contains(key, "token") || strings.Contains(key, "password")
}

// FilterAttributes keeps the allowlisted attributes (brightness becomes brightness_pct) bounded by
// homedash.BoundAttributes, and returns the cleaned friendly name separately.
func FilterAttributes(attrs map[string]any) (string, map[string]any) {
	name, kept := "", map[string]any{}
	for k, v := range attrs {
		switch {
		case deniedAttribute(k):
		case k == "friendly_name":
			if s, ok := v.(string); ok {
				name = homedash.CleanText(s, homedash.MaxNameRunes)
			}
		case k == "brightness":
			if f, ok := v.(float64); ok {
				kept["brightness_pct"] = math.Round(f / 255 * 100)
			}
		case attributeAllowlist[k]:
			kept[k] = v
		}
	}
	return name, homedash.BoundAttributes(kept)
}
```
- [ ] Run `go test ./internal/tools/ -run '(?i)(TestHA)' -count=1 -v` (new tests plus existing `TestHA*` PASS) and `go test ./internal/homedash/sources/hass/ -count=1 -v` → PASS `TestHADomainMappingTable`, `TestHACapabilitiesHonourServicePolicy`, `TestHAAttributeAllowlistDropsEntityPictureAndCoordinates`. Then `go test ./internal/audit/ -run '(?i)(NetworkClient|CredentialedHTTP)' -count=1`.
- [ ] Commit `git add internal/tools/homeassistant.go internal/tools/homeassistant_raw.go internal/tools/homeassistant_raw_test.go internal/homedash/sources/hass`; `feat(homedash): Home Assistant REST helpers and domain/attribute mapping` + GitNexus lines for `haRequestContext`.
- **Done when:** tests pass; one HA HTTP path (`haDo`); audit tripwires green.

### Task 5: B4a hub core

**Files:** Create `internal/homedash/{hub,hub_sources,hub_reconfigure,subscriber}.go`, `internal/homedash/sourcetest/fake.go`, `internal/homedash/hub_test.go`.
**Read first:** backend §4 (hub API), §5 (lifecycle, interest, catalog, coalescing, reload, shutdown); `internal/dockerwatch/hub.go:184-241` (generation-guarded idle stop) and `internal/dockerwatch/AGENTS.md:9-11`.

Locking contract (put it in the `Hub` doc comment): `Hub.mu` guards configuration, source runtimes, the state map, the subscriber registry and every subscriber's `set`/`srcCount`; `Subscriber.mu` guards its pending maps; order `Hub.mu` → `Subscriber.mu`. No source method is called while `Hub.mu` is held except factories (which must not block); `SetInterest` is delivered after unlocking. Sinks never block.

- [ ] `hub.go`:
```go
package homedash

type Options struct {
	Now              func() time.Time
	Logger           *slog.Logger
	IdleStop         time.Duration   // 60 s
	CatalogTTL       time.Duration   // 30 s
	CatalogTimeout   time.Duration   // 8 s
	CoalesceInterval time.Duration   // 100 ms, read by the stream writer
	CommandTimeout   time.Duration   // 15 s
	Backoff          []time.Duration // 1,2,4,8,16,30,60 s
	Audit            func(ctx context.Context, cmd Command, outcome string)
}

func (o Options) withDefaults() Options {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	def := func(d *time.Duration, v time.Duration) {
		if *d <= 0 {
			*d = v
		}
	}
	def(&o.IdleStop, 60*time.Second)
	def(&o.CatalogTTL, 30*time.Second)
	def(&o.CatalogTimeout, 8*time.Second)
	def(&o.CoalesceInterval, 100*time.Millisecond)
	def(&o.CommandTimeout, 15*time.Second)
	if len(o.Backoff) == 0 {
		o.Backoff = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, time.Minute}
	}
	return o
}

type Filter struct {
	Source        SourceID
	Kind          Kind
	Area, Query   string
	Limit, Offset int
}

type DashboardEvent struct {
	ID               string `json:"id"`
	Revision         int64  `json:"revision"`
	ApprovedRevision int64  `json:"approved_revision"`
}

const (
	CloseShutdown = "shutdown"; CloseDisabled = "disabled"; CloseKioskDisabled = "kiosk_disabled"
	CloseDeviceRevoked = "device_revoked"; CloseAuthLost = "auth_lost"; CloseClosed = "closed"
)

// stateRecorder is the history ring (history.go, Task 8); nil until then.
type stateRecorder interface {
	touch(id EntityID, now time.Time)
	observe(id EntityID, v float64, at time.Time)
	resize(points, entities int, bucket time.Duration)
	series(id EntityID, from, to time.Time, points int) []Sample
}

type Hub struct {
	opts      Options
	log       *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startedAt time.Time
	reconf    chan struct{}

	mu        sync.Mutex
	closed    bool
	cfg       RuntimeConfig
	pending   *RuntimeConfig
	factories map[SourceID]Factory
	order     []SourceID
	rts       map[SourceID]*sourceRT
	subs      map[string]*Subscriber
	states    map[EntityID]State
	approved  map[string]int64
	rec       stateRecorder
}

func NewHub(parent context.Context, opts Options) *Hub {
	opts = opts.withDefaults()
	ctx, cancel := context.WithCancel(parent)
	h := &Hub{opts: opts, log: opts.Logger, ctx: ctx, cancel: cancel, startedAt: opts.Now(), reconf: make(chan struct{}, 1),
		factories: map[SourceID]Factory{}, rts: map[SourceID]*sourceRT{}, subs: map[string]*Subscriber{},
		states: map[EntityID]State{}, approved: map[string]int64{}}
	h.wg.Add(1)
	go h.reconfigureLoop()
	return h
}

// Register adds a source factory; call it before the first Reconfigure.
func (h *Hub) Register(id SourceID, f Factory) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || f == nil || !ValidSourceID(string(id)) {
		return
	}
	if _, dup := h.factories[id]; !dup {
		h.order = append(h.order, id)
		h.rts[id] = &sourceRT{id: id, natives: map[string]int{}, status: SourceStatus{Source: id, State: StatusDisabled, At: h.opts.Now()}}
	}
	h.factories[id] = f
}

// Reconfigure stores the snapshot and wakes the hub goroutine; it never blocks (safe under CfgMu).
func (h *Hub) Reconfigure(cfg RuntimeConfig) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	c := cfg.withDefaults()
	h.pending = &c
	h.mu.Unlock()
	select {
	case h.reconf <- struct{}{}:
	default:
	}
}

// Close cancels every source, ends every stream with "shutdown" and waits up to 10 s. Idempotent.
func (h *Hub) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	for _, rt := range h.rts {
		if rt.idle != nil {
			rt.idle.Stop()
			rt.idle = nil
		}
		h.stopLocked(rt)
	}
	for _, s := range h.subs {
		s.closeWith(CloseShutdown)
	}
	h.subs = map[string]*Subscriber{}
	h.mu.Unlock()
	h.cancel()
	done := make(chan struct{})
	go func() { h.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		h.log.Warn("homedash: sources did not stop within 10 s")
	}
}

func (h *Hub) Sources() []SourceStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]SourceStatus, 0, len(h.order))
	for _, id := range h.order {
		out = append(out, h.rts[id].status)
	}
	return out
}

// DashboardChanged tells desktop streams about every change and kiosk streams of that dashboard
// only when its approved revision moved.
func (h *Hub) DashboardChanged(id string, revision, approved int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	prev, seen := h.approved[id]
	h.approved[id] = approved
	ev := DashboardEvent{ID: id, Revision: revision, ApprovedRevision: approved}
	for _, s := range h.subs {
		if s.p.Kind == PrincipalKiosk && (s.p.DashboardID != id || (seen && prev == approved)) {
			continue
		}
		s.pushDashboard(ev)
	}
}

// RevokeDevice ends the streams of one kiosk device.
func (h *Hub) RevokeDevice(deviceID, reason string) {
	h.mu.Lock()
	var notes []*interestNote
	for _, s := range h.subs {
		if s.p.DeviceID != "" && s.p.DeviceID == deviceID {
			notes = append(notes, h.detachLocked(s, reason)...)
		}
	}
	h.mu.Unlock()
	h.deliver(notes)
}

// viewLocked overlays the hub gates on command capabilities of an entity leaving the hub.
func (h *Hub) viewLocked(e Entity) Entity {
	set := h.cfg.Sources[e.Source]
	block := ""
	switch {
	case h.cfg.ReadOnly:
		block = "homedash.readonly"
	case h.cfg.DesktopReadOnly:
		block = "virtual_desktop.readonly"
	case !set.Control && set.ConfigKey != "":
		block = "homedash.sources." + set.ConfigKey + ".control"
	}
	caps := slices.Clone(e.Capabilities)
	readOnly, reason := true, ""
	for i := range caps {
		info, ok := LookupCapability(caps[i].Name)
		if !ok || !info.Command {
			caps[i].Allowed = true
			continue
		}
		if caps[i].Allowed && block != "" {
			caps[i].Allowed, caps[i].BlockedBy = false, block
		}
		if caps[i].Allowed {
			readOnly = false
		} else if reason == "" {
			reason = caps[i].BlockedBy
		}
	}
	if caps == nil {
		caps = []CapabilitySpec{}
	}
	e.Capabilities, e.ReadOnly, e.ReadOnlyReason = caps, readOnly, ""
	if readOnly {
		e.ReadOnlyReason = reason
	}
	return e
}
```
  `Entities` and `Entity` (also in `hub.go`), rules:
  1. `Entities(ctx, f) (items []Entity, total int, sources []SourceStatus)`: under the lock, for each active runtime in registration order matching `f.Source`: set `warmUntil = now + CatalogTTL`, collect `reconcileLocked` notes, schedule `time.AfterFunc(CatalogTTL+10ms, func(){ h.recheck(id) })` (`recheck` = lock, `reconcileLocked`, unlock, deliver), and queue a `catalogJob{id, src}` when the catalog is nil or older than `CatalogTTL`. Unlock, deliver notes, `fetchCatalogs(ctx, jobs)`.
  2. Then under the lock collect from every active runtime's catalog the entities matching: `f.Kind` equal; `f.Area` equal (case-insensitive) to `Area` or `AreaID`; `f.Query` a case-insensitive substring of name, id or area. Merge `h.states[id]` into `State` when present, pass through `viewLocked`. Sources = statuses of all registered sources.
  3. Sort by lower-case name, then id. `total` = matches. Limit default 100, maximum 500; negative offset = 0; return the page.
  4. `Entity(ctx, id)`: parse (`ErrEntityNotFound` on failure, unknown or inactive source); fetch that source's catalog if stale (one job); return the merged `viewLocked` entity or `ErrEntityNotFound`. It does not warm the source.
- [ ] `hub_sources.go` — full code:
```go
package homedash

type sourceRT struct {
	id        SourceID
	src       Source
	fp        string
	active    bool
	natives   map[string]int // subscribed native -> subscriber count
	warmUntil time.Time
	inflight  int
	running   bool
	runGen    int
	cancel    context.CancelFunc
	done      chan struct{}
	idle      *time.Timer
	idleGen   int
	status    SourceStatus
	catalog   map[EntityID]Entity
	catalogAt time.Time
}

type interestNote struct {
	src     Source
	natives []string
}

type catalogJob struct {
	id  SourceID
	src Source
}

func (rt *sourceRT) interested(now time.Time) bool {
	return rt.active && (len(rt.natives) > 0 || rt.inflight > 0 || now.Before(rt.warmUntil))
}

func (h *Hub) deliver(notes []*interestNote) {
	for _, n := range notes {
		if n != nil {
			n.src.SetInterest(n.natives)
		}
	}
}

// reconcileLocked starts a wanted source, arms the generation-guarded idle stop of an unwanted one,
// and returns the interest note to deliver after unlocking.
func (h *Hub) reconcileLocked(rt *sourceRT) *interestNote {
	if h.closed || rt.src == nil || !rt.active {
		return nil
	}
	if rt.interested(h.opts.Now()) {
		if rt.idle != nil {
			rt.idle.Stop()
			rt.idle = nil
			rt.idleGen++
		}
		if !rt.running {
			h.startLocked(rt)
		}
	} else if rt.running && rt.idle == nil {
		rt.idleGen++
		gen, id := rt.idleGen, rt.id
		rt.idle = time.AfterFunc(h.opts.IdleStop, func() { h.idleStop(id, gen) })
	}
	natives := make([]string, 0, len(rt.natives))
	for n := range rt.natives {
		natives = append(natives, n)
	}
	sort.Strings(natives)
	return &interestNote{src: rt.src, natives: natives}
}

func (h *Hub) recheck(id SourceID) {
	h.mu.Lock()
	var note *interestNote
	if rt := h.rts[id]; rt != nil {
		note = h.reconcileLocked(rt)
	}
	h.mu.Unlock()
	h.deliver([]*interestNote{note})
}

func (h *Hub) startLocked(rt *sourceRT) {
	ctx, cancel := context.WithCancel(h.ctx)
	rt.runGen++
	rt.running, rt.cancel, rt.done = true, cancel, make(chan struct{})
	h.setStatusLocked(rt, StatusStarting, "")
	sink := &hubSink{h: h, id: rt.id, src: rt.src, gen: rt.runGen}
	h.wg.Add(1)
	go h.runLoop(ctx, rt.id, rt.src, sink, rt.done)
}

// stopLocked cancels a running source; the returned channel closes when its goroutine ended.
func (h *Hub) stopLocked(rt *sourceRT) chan struct{} {
	if !rt.running {
		return nil
	}
	rt.cancel()
	done := rt.done
	rt.running, rt.cancel, rt.done = false, nil, nil
	return done
}

func (h *Hub) runLoop(ctx context.Context, id SourceID, src Source, sink *hubSink, done chan struct{}) {
	defer h.wg.Done()
	defer close(done)
	attempt := 0
	for {
		started := time.Now()
		err := runGuarded(ctx, src, sink)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > time.Minute {
			attempt = 0
		}
		delay := h.opts.Backoff[min(attempt, len(h.opts.Backoff)-1)]
		attempt++
		reason := ReasonStopped
		if err != nil {
			reason = ReasonCrashed
		}
		h.log.Warn("homedash: source ended, restarting", "source", id, "error", err, "retry_in", delay)
		h.mu.Lock()
		if rt, ok := sink.live(); ok {
			h.setStatusLocked(rt, StatusDegraded, reason)
		}
		h.mu.Unlock()
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

func runGuarded(ctx context.Context, src Source, sink Sink) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("source panic: %v", r)
		}
	}()
	return src.Run(ctx, sink)
}

func catalogGuarded(ctx context.Context, src Source) (list []Entity, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("catalog panic: %v", r)
		}
	}()
	return src.Catalog(ctx)
}

func (h *Hub) idleStop(id SourceID, gen int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rt := h.rts[id]
	if rt == nil || rt.idle == nil || rt.idleGen != gen || rt.interested(h.opts.Now()) {
		return
	}
	rt.idle = nil
	h.stopLocked(rt)
	h.setStatusLocked(rt, StatusIdle, "")
}

func (h *Hub) setStatusLocked(rt *sourceRT, state, reason string) {
	if rt.status.State == state && rt.status.Reason == reason {
		return
	}
	rt.status = SourceStatus{Source: rt.id, State: state, Reason: reason, At: h.opts.Now()}
	for _, s := range h.subs {
		if s.srcCount[rt.id] > 0 {
			s.pushSource(rt.status)
		}
	}
}

// fetchCatalogs runs Catalog for each job concurrently (CatalogTimeout each, outside the lock) and
// applies results whose source is still current. A failure marks a stopped source unavailable.
func (h *Hub) fetchCatalogs(ctx context.Context, jobs []catalogJob) {
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j catalogJob) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, h.opts.CatalogTimeout)
			defer cancel()
			list, err := catalogGuarded(cctx, j.src)
			h.mu.Lock()
			defer h.mu.Unlock()
			rt := h.rts[j.id]
			if h.closed || rt == nil || rt.src != j.src {
				return
			}
			if err != nil {
				h.log.Warn("homedash: catalog failed", "source", j.id, "error", err)
				if !rt.running {
					h.setStatusLocked(rt, StatusUnavailable, ReasonCatalogFailed)
				}
				return
			}
			h.applyCatalogLocked(rt, list)
		}(j)
	}
	wg.Wait()
}

// applyCatalogLocked replaces a source's catalog and pushes entity/removed frames for subscribed ids.
func (h *Hub) applyCatalogLocked(rt *sourceRT, list []Entity) {
	if len(list) > MaxCatalogEntities {
		list = list[:MaxCatalogEntities]
	}
	now := h.opts.Now()
	next := make(map[EntityID]Entity, len(list))
	for _, raw := range list {
		e, ok := sanitizeEntity(rt.id, raw, now)
		if !ok {
			continue
		}
		next[e.ID] = e
		if old, has := h.states[e.ID]; !has || e.State.UpdatedAt.After(old.UpdatedAt) {
			h.states[e.ID] = e.State
		}
	}
	for _, s := range h.subs {
		for id := range s.set {
			src, _, _ := ParseEntityID(string(id))
			if src != rt.id {
				continue
			}
			e, inNext := next[id]
			old, inOld := rt.catalog[id]
			switch {
			case inNext && (!inOld || !sameMeta(old, e)):
				e.State = h.states[id]
				s.pushEntity(h.viewLocked(e))
			case !inNext && inOld:
				s.pushRemoved(id)
			}
		}
	}
	rt.catalog, rt.catalogAt = next, now
}

func sameMeta(a, b Entity) bool {
	a.State, b.State = State{}, State{}
	return reflect.DeepEqual(a, b)
}

type hubSink struct {
	h   *Hub
	id  SourceID
	src Source
	gen int
}

// live returns the runtime when this sink still belongs to its current run (call with Hub.mu held).
func (s *hubSink) live() (*sourceRT, bool) {
	rt := s.h.rts[s.id]
	return rt, rt != nil && !s.h.closed && rt.src == s.src && rt.runGen == s.gen && rt.running
}

func (s *hubSink) Catalog(entities []Entity) {
	s.h.mu.Lock()
	defer s.h.mu.Unlock()
	if rt, ok := s.live(); ok {
		s.h.applyCatalogLocked(rt, entities)
	}
}

func (s *hubSink) Status(st SourceStatus) {
	s.h.mu.Lock()
	defer s.h.mu.Unlock()
	if rt, ok := s.live(); ok && slices.Contains([]string{StatusStarting, StatusLive, StatusPolling, StatusDegraded, StatusUnavailable}, st.State) {
		s.h.setStatusLocked(rt, st.State, CleanText(st.Reason, 64))
	}
}

func (s *hubSink) Update(updates ...StateUpdate) {
	h := s.h
	h.mu.Lock()
	defer h.mu.Unlock()
	rt, ok := s.live()
	if !ok {
		return
	}
	now := h.opts.Now()
	for _, u := range updates {
		id := MakeEntityID(s.id, u.Native)
		if _, _, err := ParseEntityID(string(id)); err != nil {
			continue
		}
		st := sanitizeState(u.State, now)
		h.states[id] = st
		if rt.natives[u.Native] == 0 {
			continue
		}
		if f, isNum := st.Value.(float64); isNum && h.rec != nil {
			h.rec.observe(id, f, st.UpdatedAt)
		}
		for _, sub := range h.subs {
			if _, held := sub.set[id]; held {
				sub.pushState(id, st)
			}
		}
	}
}
```
  `sanitizeState(st State, now time.Time) State` and `sanitizeEntity(src SourceID, e Entity, now time.Time) (Entity, bool)` (also in `hub_sources.go`), rules:
  1. State: `Value` kept when nil/bool/float64/string (string cleaned to 255 runes), int types become float64, anything else nil; `Text` cleaned 255; `Attributes` = `BoundAttributes`; zero `UpdatedAt` = now.
  2. Entity: `Source = src`; `ID` must parse with that source, otherwise dropped; `Name` cleaned 120 (empty → native id); `Area` 80, `AreaID` 64, `DeviceClass` 64, `Unit` 16 (all `CleanText`); `IconHint` kept only when `ValidIcon`; unknown kind → `KindGeneric`; capabilities with unknown names dropped, read capabilities `Allowed = true`, `Options` ≤ 64 entries cleaned to 64 runes, `BlockedBy` cleaned 64; `ReadOnly` recomputed as "no allowed command capability", `ReadOnlyReason` cleaned 64; `Media.Live` must be `go2rtc_viewer`, `mjpeg` or "" and `Media.ViewerPath` must start with `/api/`, otherwise cleared; `State` sanitized.
- [ ] `hub_reconfigure.go` — full code:
```go
package homedash

func (h *Hub) reconfigureLoop() {
	defer h.wg.Done()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.reconf:
			h.applyConfig()
		}
	}
}

func gatesChanged(a, b RuntimeConfig) bool {
	if a.ReadOnly != b.ReadOnly || a.DesktopReadOnly != b.DesktopReadOnly {
		return true
	}
	for id, s := range b.Sources {
		if a.Sources[id].Control != s.Control {
			return true
		}
	}
	return false
}

// applyConfig diffs the new snapshot by fingerprint, rebuilds only changed sources (waiting for the
// old run outside the lock), restarts them on interest and re-fetches subscribed catalogs.
func (h *Hub) applyConfig() {
	h.mu.Lock()
	if h.closed || h.pending == nil {
		h.mu.Unlock()
		return
	}
	cfg, old := *h.pending, h.cfg
	h.pending, h.cfg = nil, cfg
	if h.rec != nil {
		h.rec.resize(cfg.History.RingPoints, cfg.History.RingEntities, time.Duration(cfg.History.SampleSeconds)*time.Second)
	}
	var notes []*interestNote
	for _, s := range h.subs {
		switch {
		case !cfg.active():
			notes = append(notes, h.detachLocked(s, CloseDisabled)...)
		case s.p.Kind == PrincipalKiosk && !cfg.KioskEnabled:
			notes = append(notes, h.detachLocked(s, CloseKioskDisabled)...)
		}
	}
	type change struct {
		rt   *sourceRT
		done chan struct{}
	}
	var changed []change
	for _, id := range h.order {
		rt, set := h.rts[id], cfg.Sources[id]
		want, fp := cfg.active() && set.Enabled, set.Fingerprint()
		if rt.fp == fp && rt.active == want && (rt.src != nil || !want) {
			continue
		}
		if rt.idle != nil {
			rt.idle.Stop()
			rt.idle = nil
			rt.idleGen++
		}
		done := h.stopLocked(rt)
		rt.src, rt.active, rt.fp, rt.catalogAt = nil, false, fp, time.Time{}
		if want {
			if src, ok := h.factories[id](cfg); ok && src != nil {
				rt.src, rt.active = src, true
			}
		}
		state := StatusDisabled
		if rt.active {
			state = StatusIdle
		} else {
			rt.catalog = nil
		}
		h.setStatusLocked(rt, state, "")
		changed = append(changed, change{rt, done})
	}
	if gatesChanged(old, cfg) {
		for _, s := range h.subs {
			for id := range s.set {
				src, _, _ := ParseEntityID(string(id))
				if rt := h.rts[src]; rt != nil {
					if e, ok := rt.catalog[id]; ok {
						e.State = h.states[id]
						s.pushEntity(h.viewLocked(e))
					}
				}
			}
		}
	}
	h.mu.Unlock()
	h.deliver(notes)
	for _, c := range changed {
		if c.done != nil {
			select {
			case <-c.done:
			case <-time.After(10 * time.Second):
				h.log.Warn("homedash: source did not stop within 10 s", "source", c.rt.id)
			}
		}
	}
	h.mu.Lock()
	notes = nil
	var jobs []catalogJob
	for _, c := range changed {
		if !c.rt.active {
			continue
		}
		notes = append(notes, h.reconcileLocked(c.rt))
		if len(c.rt.natives) > 0 {
			jobs = append(jobs, catalogJob{c.rt.id, c.rt.src})
		}
	}
	h.mu.Unlock()
	h.deliver(notes)
	h.fetchCatalogs(h.ctx, jobs)
}
```
- [ ] `subscriber.go` — full code:
```go
package homedash

const (
	RejectInvalidID = "invalid_id"; RejectUnknownSource = "unknown_source"; RejectSourceDisabled = "source_disabled"
	RejectForbidden = "forbidden"; RejectLimit = "limit"
)

type Rejection struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// Batch is what a stream writer sends: Snapshot for ids newly added by Set, Entities for meta or
// capability changes, States coalesced latest-wins.
type Batch struct {
	Snapshot  []Entity
	Entities  []Entity
	States    map[EntityID]State
	Removed   []EntityID
	Sources   []SourceStatus
	Dashboard *DashboardEvent
	Closed    string
}

func (b Batch) Empty() bool {
	return len(b.Snapshot) == 0 && len(b.Entities) == 0 && len(b.States) == 0 && len(b.Removed) == 0 && len(b.Sources) == 0 && b.Dashboard == nil && b.Closed == ""
}

type Subscriber struct {
	h  *Hub
	id string
	p  Principal
	// guarded by Hub.mu
	set      map[EntityID]struct{}
	srcCount map[SourceID]int
	// guarded by mu
	mu      sync.Mutex
	snap    map[EntityID]Entity
	ents    map[EntityID]Entity
	states  map[EntityID]State
	removed map[EntityID]struct{}
	sources map[SourceID]SourceStatus
	dash    *DashboardEvent
	closed  string
	ready   chan struct{}
}

func (h *Hub) Attach(p Principal) (*Subscriber, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case h.closed:
		return nil, ErrHubClosed
	case !h.cfg.active():
		return nil, newError(CodeHomeDashDisabled, "HomeDash is disabled")
	case p.Kind == PrincipalAgent:
		return nil, &Error{Code: CodeForbiddenEntity, Message: "agents have no stream", BlockedBy: "agent"}
	case p.Kind == PrincipalKiosk && !h.cfg.KioskEnabled:
		return nil, newError(CodeKioskDisabled, "the kiosk is disabled")
	}
	limit, n := h.cfg.Limits.MaxStreams, 0
	if p.Kind == PrincipalKiosk {
		limit = h.cfg.Limits.MaxKioskStreams
	}
	for _, s := range h.subs {
		if (s.p.Kind == PrincipalKiosk) == (p.Kind == PrincipalKiosk) {
			n++
		}
	}
	if n >= limit {
		return nil, ErrTooManyStreams
	}
	s := &Subscriber{h: h, id: randomHex(16), p: p, set: map[EntityID]struct{}{}, srcCount: map[SourceID]int{},
		snap: map[EntityID]Entity{}, ents: map[EntityID]Entity{}, states: map[EntityID]State{},
		removed: map[EntityID]struct{}{}, sources: map[SourceID]SourceStatus{}, ready: make(chan struct{}, 1)}
	h.subs[s.id] = s
	return s, nil
}

// Subscriber finds an open stream; the principal must match the one that attached it.
func (h *Hub) Subscriber(id string, p Principal) (*Subscriber, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.subs[id]
	if s == nil || s.p.Kind != p.Kind || s.p.Key != p.Key || s.p.DeviceID != p.DeviceID {
		return nil, false
	}
	return s, true
}

func (s *Subscriber) ID() string              { return s.id }
func (s *Subscriber) Ready() <-chan struct{} { return s.ready }

func (s *Subscriber) signal() {
	select {
	case s.ready <- struct{}{}:
	default:
	}
}

func decNative(rt *sourceRT, native string) {
	if rt.natives[native]--; rt.natives[native] <= 0 {
		delete(rt.natives, native)
	}
}

// Set replaces the subscription. Valid ids of enabled sources not yet in the catalog are accepted
// and reported unavailable in the snapshot.
func (s *Subscriber) Set(ids []EntityID) ([]EntityID, []Rejection, error) {
	h := s.h
	h.mu.Lock()
	if h.closed || h.subs[s.id] != s {
		h.mu.Unlock()
		return nil, nil, newError(CodeStreamNotFound, "the stream is closed")
	}
	var accepted []EntityID
	var rejected []Rejection
	want := map[EntityID]struct{}{}
	for _, id := range ids {
		if _, dup := want[id]; dup {
			continue
		}
		src, _, err := ParseEntityID(string(id))
		rt := h.rts[src]
		reason := ""
		switch {
		case err != nil:
			reason = RejectInvalidID
		case rt == nil:
			reason = RejectUnknownSource
		case !rt.active:
			reason = RejectSourceDisabled
		case s.p.Kind == PrincipalKiosk && !s.p.Allowed.Has(id):
			reason = RejectForbidden
		case len(want) >= h.cfg.Limits.MaxSubscriptions:
			reason = RejectLimit
		}
		if reason != "" {
			rejected = append(rejected, Rejection{ID: string(id), Reason: reason})
			continue
		}
		want[id] = struct{}{}
		accepted = append(accepted, id)
	}
	touched := map[SourceID]*sourceRT{}
	for id := range s.set {
		if _, keep := want[id]; keep {
			continue
		}
		src, native, _ := ParseEntityID(string(id))
		delete(s.set, id)
		if s.srcCount[src]--; s.srcCount[src] <= 0 {
			delete(s.srcCount, src)
		}
		if rt := h.rts[src]; rt != nil {
			decNative(rt, native)
			touched[src] = rt
		}
		s.forget(id)
	}
	now := h.opts.Now()
	for _, id := range accepted {
		if _, had := s.set[id]; had {
			continue
		}
		src, native, _ := ParseEntityID(string(id))
		rt := h.rts[src]
		s.set[id] = struct{}{}
		s.srcCount[src]++
		rt.natives[native]++
		touched[src] = rt
		e, ok := rt.catalog[id]
		if !ok {
			e = Entity{ID: id, Source: src, Kind: KindGeneric, Name: CleanText(native, MaxNameRunes),
				Capabilities: []CapabilitySpec{}, ReadOnly: true, State: State{Available: false, UpdatedAt: now}}
		}
		if st, ok := h.states[id]; ok {
			e.State = st
		}
		s.pushSnapshot(h.viewLocked(e))
		s.pushSource(rt.status)
		if h.rec != nil {
			h.rec.touch(id, now)
		}
	}
	var notes []*interestNote
	for _, rt := range touched {
		notes = append(notes, h.reconcileLocked(rt))
	}
	h.mu.Unlock()
	h.deliver(notes)
	return accepted, rejected, nil
}

func (s *Subscriber) Close() {
	s.h.mu.Lock()
	notes := s.h.detachLocked(s, CloseClosed)
	s.h.mu.Unlock()
	s.h.deliver(notes)
}

func (h *Hub) detachLocked(s *Subscriber, reason string) []*interestNote {
	if h.subs[s.id] != s {
		return nil
	}
	delete(h.subs, s.id)
	touched := map[SourceID]*sourceRT{}
	for id := range s.set {
		src, native, _ := ParseEntityID(string(id))
		if rt := h.rts[src]; rt != nil {
			decNative(rt, native)
			touched[src] = rt
		}
	}
	s.set, s.srcCount = map[EntityID]struct{}{}, map[SourceID]int{}
	s.closeWith(reason)
	var notes []*interestNote
	for _, rt := range touched {
		notes = append(notes, h.reconcileLocked(rt))
	}
	return notes
}

func (s *Subscriber) closeWith(reason string) {
	s.mu.Lock()
	if s.closed == "" {
		s.closed = reason
	}
	s.mu.Unlock()
	s.signal()
}

// push helpers run with Hub.mu held; they never block and keep only the latest value per key.
func (s *Subscriber) pushState(id EntityID, st State) {
	s.mu.Lock()
	if s.closed == "" {
		s.states[id] = st
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) pushEntity(e Entity) {
	s.mu.Lock()
	if s.closed == "" {
		if _, inSnap := s.snap[e.ID]; inSnap {
			s.snap[e.ID] = e
		} else {
			s.ents[e.ID] = e
		}
		delete(s.removed, e.ID)
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) pushSnapshot(e Entity) {
	s.mu.Lock()
	if s.closed == "" {
		s.snap[e.ID] = e
		delete(s.ents, e.ID)
		delete(s.states, e.ID)
		delete(s.removed, e.ID)
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) pushRemoved(id EntityID) {
	s.mu.Lock()
	if s.closed == "" {
		s.forgetLocked(id)
		s.removed[id] = struct{}{}
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) pushSource(st SourceStatus) {
	s.mu.Lock()
	if s.closed == "" {
		s.sources[st.Source] = st
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) pushDashboard(ev DashboardEvent) {
	s.mu.Lock()
	if s.closed == "" {
		s.dash = &ev
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) forget(id EntityID) { s.mu.Lock(); s.forgetLocked(id); s.mu.Unlock() }

func (s *Subscriber) forgetLocked(id EntityID) {
	delete(s.snap, id)
	delete(s.ents, id)
	delete(s.states, id)
	delete(s.removed, id)
}

// Drain swaps out everything pending. Memory per subscriber is bounded by its subscription size.
func (s *Subscriber) Drain() Batch {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := Batch{Closed: s.closed, Dashboard: s.dash}
	s.dash = nil
	b.Snapshot, b.Entities = sortedEntities(s.snap), sortedEntities(s.ents)
	s.snap, s.ents = map[EntityID]Entity{}, map[EntityID]Entity{}
	if len(s.states) > 0 {
		b.States, s.states = s.states, map[EntityID]State{}
	}
	for id := range s.removed {
		b.Removed = append(b.Removed, id)
	}
	slices.Sort(b.Removed)
	s.removed = map[EntityID]struct{}{}
	for _, st := range s.sources {
		b.Sources = append(b.Sources, st)
	}
	sort.Slice(b.Sources, func(i, j int) bool { return b.Sources[i].Source < b.Sources[j].Source })
	s.sources = map[SourceID]SourceStatus{}
	return b
}

func sortedEntities(m map[EntityID]Entity) []Entity {
	if len(m) == 0 {
		return nil
	}
	out := make([]Entity, 0, len(m))
	for _, e := range m {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
```
- [ ] `sourcetest/fake.go` — full code:
```go
// Package sourcetest provides a scripted homedash.Source for hub tests.
package sourcetest

type FakeSource struct {
	Panics        int           // the first Panics runs panic
	CommandGate   chan struct{} // when set, Command waits until it is closed
	CommandErr    error
	CommandResult homedash.CommandResult

	id                       homedash.SourceID
	mu                       sync.Mutex
	catalog                  []homedash.Entity
	sink                     homedash.Sink
	runs, running, inCommand int
	interest                 []string
	commands                 []homedash.Command
}

func New(id homedash.SourceID, entities ...homedash.Entity) *FakeSource {
	return &FakeSource{id: id, catalog: entities}
}

func (f *FakeSource) ID() homedash.SourceID { return f.id }

func (f *FakeSource) Run(ctx context.Context, sink homedash.Sink) error {
	f.mu.Lock()
	f.runs++
	n := f.runs
	f.running++
	f.sink = sink
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.running--
		if f.sink == sink {
			f.sink = nil
		}
		f.mu.Unlock()
	}()
	if n <= f.Panics {
		panic("sourcetest: scripted panic")
	}
	sink.Status(homedash.SourceStatus{Source: f.id, State: homedash.StatusLive})
	<-ctx.Done()
	return nil
}

func (f *FakeSource) SetInterest(natives []string) {
	f.mu.Lock()
	f.interest = slices.Clone(natives)
	f.mu.Unlock()
}

func (f *FakeSource) Catalog(context.Context) ([]homedash.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.catalog), nil
}

func (f *FakeSource) Command(ctx context.Context, cmd homedash.Command) (homedash.CommandResult, error) {
	f.mu.Lock()
	f.commands = append(f.commands, cmd)
	f.inCommand++
	gate := f.CommandGate
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.inCommand--; f.mu.Unlock() }()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return homedash.CommandResult{}, ctx.Err()
		}
	}
	return f.CommandResult, f.CommandErr
}

// Push sends updates through the running sink; false when the source is not running.
func (f *FakeSource) Push(updates ...homedash.StateUpdate) bool {
	f.mu.Lock()
	sink := f.sink
	f.mu.Unlock()
	if sink == nil {
		return false
	}
	sink.Update(updates...)
	return true
}

func (f *FakeSource) Runs() int          { f.mu.Lock(); defer f.mu.Unlock(); return f.runs }
func (f *FakeSource) Running() int       { f.mu.Lock(); defer f.mu.Unlock(); return f.running }
func (f *FakeSource) InCommand() int     { f.mu.Lock(); defer f.mu.Unlock(); return f.inCommand }
func (f *FakeSource) Interest() []string { f.mu.Lock(); defer f.mu.Unlock(); return slices.Clone(f.interest) }
func (f *FakeSource) Commands() []homedash.Command {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.commands)
}

// Entity builds a catalog entity whose native id is also its name.
func Entity(id homedash.EntityID, kind homedash.Kind, caps ...homedash.CapabilitySpec) homedash.Entity {
	src, native, _ := homedash.ParseEntityID(string(id))
	if caps == nil {
		caps = []homedash.CapabilitySpec{}
	}
	return homedash.Entity{ID: id, Source: src, Kind: kind, Name: native, Capabilities: caps, State: homedash.State{Available: true}}
}

// RecordingSink records what a source sends (adapter tests).
type RecordingSink struct {
	mu       sync.Mutex
	Catalogs [][]homedash.Entity
	Updates  []homedash.StateUpdate
	Statuses []homedash.SourceStatus
}

func (r *RecordingSink) Catalog(e []homedash.Entity) { r.mu.Lock(); r.Catalogs = append(r.Catalogs, e); r.mu.Unlock() }
func (r *RecordingSink) Update(u ...homedash.StateUpdate) {
	r.mu.Lock()
	r.Updates = append(r.Updates, u...)
	r.mu.Unlock()
}
func (r *RecordingSink) Status(s homedash.SourceStatus) { r.mu.Lock(); r.Statuses = append(r.Statuses, s); r.mu.Unlock() }

// WaitFor polls cond every 5 ms until it holds or timeout passes.
func WaitFor(t testing.TB, timeout time.Duration, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
```
- [ ] `hub_test.go` (package `homedash_test`; imports `homedash` and `sourcetest`):
```go
func session(key string) homedash.Principal {
	return homedash.Principal{Kind: homedash.PrincipalSession, Key: key}
}

func newTestHub(t *testing.T, mutate func(*homedash.RuntimeConfig), sources ...*sourcetest.FakeSource) *homedash.Hub {
	t.Helper()
	h := homedash.NewHub(context.Background(), homedash.Options{IdleStop: 60 * time.Millisecond, CatalogTTL: time.Second, Backoff: []time.Duration{10 * time.Millisecond}})
	t.Cleanup(h.Close)
	cfg := homedash.RuntimeConfig{Enabled: true, KioskEnabled: true, KioskAllowControl: true, Sources: map[homedash.SourceID]homedash.SourceSettings{}}
	for _, f := range sources {
		f := f
		h.Register(f.ID(), func(homedash.RuntimeConfig) (homedash.Source, bool) { return f, true })
		cfg.Sources[f.ID()] = homedash.SourceSettings{Enabled: true, Control: true, ConfigKey: string(f.ID())}
	}
	if mutate != nil {
		mutate(&cfg)
	}
	h.Reconfigure(cfg)
	sourcetest.WaitFor(t, time.Second, func() bool {
		for _, st := range h.Sources() {
			if st.State == homedash.StatusDisabled {
				return false
			}
		}
		return true
	}, "sources configured")
	return h
}

func waitClosed(t *testing.T, s *homedash.Subscriber) string {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if b := s.Drain(); b.Closed != "" {
			return b.Closed
		}
		select {
		case <-s.Ready():
		case <-deadline:
			t.Fatal("stream not closed")
		}
	}
}

func ids(list ...string) []homedash.EntityID {
	out := make([]homedash.EntityID, len(list))
	for i, s := range list {
		out[i] = homedash.EntityID(s)
	}
	return out
}

func TestHubStartsSourceOnInterestAndStopsAfterIdle(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	h := newTestHub(t, nil, fa)
	if fa.Runs() != 0 {
		t.Fatal("source started without interest")
	}
	sub, err := h.Attach(session("s1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, rej, _ := sub.Set(ids("fa:x")); len(rej) != 0 {
		t.Fatalf("rejected %v", rej)
	}
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 1 && slices.Equal(fa.Interest(), []string{"x"}) }, "running with interest x")
	sub.Set(nil)
	time.Sleep(20 * time.Millisecond)
	sub.Set(ids("fa:x")) // interest returns before the idle stop
	time.Sleep(100 * time.Millisecond)
	if fa.Running() != 1 || fa.Runs() != 1 {
		t.Fatalf("idle stop not cancelled: running=%d runs=%d", fa.Running(), fa.Runs())
	}
	sub.Set(nil)
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 0 }, "idle stop")
	if st := h.Sources()[0]; st.State != homedash.StatusIdle {
		t.Fatalf("status %q", st.State)
	}
}

func TestHubCoalescesLatestStatePerEntity(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor), sourcetest.Entity("fa:y", homedash.KindSensor))
	h := newTestHub(t, nil, fa)
	sub, _ := h.Attach(session("s1"))
	sub.Set(ids("fa:x", "fa:y"))
	if b := sub.Drain(); len(b.Snapshot) != 2 {
		t.Fatalf("snapshot %d", len(b.Snapshot))
	}
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 1 }, "running")
	for v := 1.0; v <= 3; v++ {
		fa.Push(homedash.StateUpdate{Native: "x", State: homedash.State{Value: v, Available: true}})
	}
	fa.Push(homedash.StateUpdate{Native: "y", State: homedash.State{Value: "on", Available: true}})
	b := sub.Drain()
	if len(b.States) != 2 || b.States["fa:x"].Value != 3.0 || b.States["fa:y"].Value != "on" {
		t.Fatalf("states %+v", b.States)
	}
	if again := sub.Drain(); len(again.States) != 0 {
		t.Fatalf("states delivered twice: %+v", again.States)
	}
}

func TestHubSlowSubscriberDoesNotBlockOthers(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	h := newTestHub(t, nil, fa)
	slow, _ := h.Attach(session("slow"))
	fast, _ := h.Attach(session("fast"))
	slow.Set(ids("fa:x"))
	fast.Set(ids("fa:x"))
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 1 }, "running")
	start := time.Now()
	for i := 0; i < 10000; i++ {
		fa.Push(homedash.StateUpdate{Native: "x", State: homedash.State{Value: float64(i), Available: true}})
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("updates blocked behind a slow subscriber")
	}
	if b := fast.Drain(); b.States["fa:x"].Value != 9999.0 {
		t.Fatalf("fast: %+v", b.States)
	}
	if b := slow.Drain(); len(b.States) != 1 || b.States["fa:x"].Value != 9999.0 {
		t.Fatalf("slow kept more than the latest value: %+v", b.States)
	}
}

func TestHubReconfigureRebuildsOnlyChangedSource(t *testing.T) {
	var mu sync.Mutex
	built := map[homedash.SourceID]int{}
	count := func(id homedash.SourceID) int { mu.Lock(); defer mu.Unlock(); return built[id] }
	h := homedash.NewHub(context.Background(), homedash.Options{IdleStop: time.Second})
	t.Cleanup(h.Close)
	for _, id := range []homedash.SourceID{"fa", "fb"} {
		id := id
		h.Register(id, func(homedash.RuntimeConfig) (homedash.Source, bool) {
			mu.Lock()
			built[id]++
			mu.Unlock()
			return sourcetest.New(id), true
		})
	}
	cfg := func(poll int) homedash.RuntimeConfig {
		return homedash.RuntimeConfig{Enabled: true, Sources: map[homedash.SourceID]homedash.SourceSettings{
			"fa": {Enabled: true, PollSeconds: 5}, "fb": {Enabled: true, PollSeconds: poll}}}
	}
	h.Reconfigure(cfg(10))
	sourcetest.WaitFor(t, time.Second, func() bool { return count("fa") == 1 && count("fb") == 1 }, "first build")
	h.Reconfigure(cfg(10))
	time.Sleep(50 * time.Millisecond)
	h.Reconfigure(cfg(20))
	sourcetest.WaitFor(t, time.Second, func() bool { return count("fb") == 2 }, "fb rebuilt")
	if count("fa") != 1 {
		t.Fatalf("unchanged source rebuilt %d times", count("fa"))
	}
}

func TestHubSourcePanicIsIsolatedAndRestarted(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	fa.Panics = 1
	fb := sourcetest.New("fb", sourcetest.Entity("fb:y", homedash.KindSensor))
	h := newTestHub(t, nil, fa, fb)
	sub, _ := h.Attach(session("s1"))
	sub.Set(ids("fa:x", "fb:y"))
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return fa.Runs() >= 2 && fa.Running() == 1 && fb.Running() == 1 }, "restart after panic")
	if fb.Runs() != 1 {
		t.Fatalf("healthy source restarted %d times", fb.Runs())
	}
}

func TestHubCloseEndsSubscribersWithReason(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	h := newTestHub(t, nil, fa)
	sub, _ := h.Attach(session("s1"))
	sub.Set(ids("fa:x"))
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 1 }, "running")
	h.Close()
	if reason := waitClosed(t, sub); reason != homedash.CloseShutdown {
		t.Fatalf("reason %q", reason)
	}
	if fa.Running() != 0 {
		t.Fatal("source still running")
	}
	if _, err := h.Attach(session("late")); !errors.Is(err, homedash.ErrHubClosed) {
		t.Fatalf("attach after close: %v", err)
	}
}

func TestHubDisableClosesStreams(t *testing.T) {
	fa := sourcetest.New("fa")
	h := newTestHub(t, nil, fa)
	sub, _ := h.Attach(session("s1"))
	h.Reconfigure(homedash.RuntimeConfig{Enabled: false})
	if reason := waitClosed(t, sub); reason != homedash.CloseDisabled {
		t.Fatalf("reason %q", reason)
	}
	var he *homedash.Error
	if _, err := h.Attach(session("s2")); !errors.As(err, &he) || he.Code != homedash.CodeHomeDashDisabled {
		t.Fatalf("attach while disabled: %v", err)
	}
}

func TestHubSetRejections(t *testing.T) {
	fa := sourcetest.New("fa")
	h := newTestHub(t, func(c *homedash.RuntimeConfig) { c.Limits.MaxSubscriptions = 2 }, fa)
	kiosk := homedash.Principal{Kind: homedash.PrincipalKiosk, Key: "kd_1", DeviceID: "kd_1",
		Allowed: &homedash.AllowedSet{Commands: map[homedash.EntityID]map[homedash.Capability]bool{"fa:x": {}}}}
	for _, tc := range []struct {
		p        homedash.Principal
		in       []homedash.EntityID
		accepted []homedash.EntityID
		rejected []homedash.Rejection
	}{
		{session("s1"), ids("fa:x", "bad id", "zz:x", "fa:y", "fa:z"), ids("fa:x", "fa:y"),
			[]homedash.Rejection{{"bad id", homedash.RejectInvalidID}, {"zz:x", homedash.RejectUnknownSource}, {"fa:z", homedash.RejectLimit}}},
		{kiosk, ids("fa:x", "fa:y"), ids("fa:x"), []homedash.Rejection{{"fa:y", homedash.RejectForbidden}}},
	} {
		sub, err := h.Attach(tc.p)
		if err != nil {
			t.Fatal(err)
		}
		acc, rej, err := sub.Set(tc.in)
		if err != nil || !slices.Equal(acc, tc.accepted) || !slices.Equal(rej, tc.rejected) {
			t.Errorf("%s: accepted %v rejected %v err %v", tc.p.Kind, acc, rej, err)
		}
	}
}
```
- [ ] Run `go test ./internal/homedash/... -count=1 -v` → the eight `TestHub*` tests PASS plus earlier tests; `go vet ./internal/homedash/...`. Race runs once in Task 8.
- [ ] Commit `git add internal/homedash`; `feat(homedash): demand-driven entity hub with coalescing subscribers`.
- **Done when:** hub tests pass in under 10 s; no source call happens under `Hub.mu` except factories.

### Task 6: B9a Home Assistant websocket client

**Files:** Create `internal/homedash/sources/hass/{ws_messages,ws_client,ws_client_test}.go`.
**Read first:** backend §8.1 "Transport", "Protocol", "Reconnect and fallback"; main §9 (messages from HA's public API, verified once on aurago-test later). The client opens no `http.Client` (audit tripwire `audit_manifests_test.go:181`); the token travels in the auth message, never in a header.

- [ ] `ws_messages.go`:
```go
package hass

type wsMsg struct {
	ID      int64           `json:"id,omitempty"`
	Type    string          `json:"type"`
	Success *bool           `json:"success,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Event   json.RawMessage `json:"event,omitempty"`
	Message string          `json:"message,omitempty"`
}

type HAState struct {
	EntityID    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes"`
	LastChanged time.Time      `json:"last_changed"`
	LastUpdated time.Time      `json:"last_updated"`
}

type StateChanged struct {
	EntityID string   `json:"entity_id"`
	NewState *HAState `json:"new_state"`
	OldState *HAState `json:"old_state"`
}

type AreaEntry struct {
	AreaID string `json:"area_id"`
	Name   string `json:"name"`
}

type DeviceEntry struct {
	ID     string `json:"id"`
	AreaID string `json:"area_id"`
}

type EntityEntry struct {
	EntityID   string `json:"entity_id"`
	DeviceID   string `json:"device_id"`
	AreaID     string `json:"area_id"`
	HiddenBy   string `json:"hidden_by"`
	DisabledBy string `json:"disabled_by"`
}

type Registries struct {
	Areas    []AreaEntry
	Devices  []DeviceEntry
	Entities []EntityEntry
}
```
- [ ] `ws_client.go` — full code:
```go
package hass

const (
	WSConnecting = "connecting"; WSLive = "live"; WSDegraded = "degraded"; WSAuthFailed = "auth_failed"
	ReasonUnreachable = "unreachable"; ReasonTimeout = "timeout"; ReasonProtocol = "protocol"; ReasonAuthFailed = "auth_failed"
	kindStates = "states"; kindSubscribe = "subscribe"; kindAreas = "areas"; kindDevices = "devices"; kindEntities = "entities"; kindPing = "ping"
)

// WSStatus: Failures counts consecutive sessions that ended before auth_ok (not auth_invalid); the
// adapter switches to REST polling after 3.
type WSStatus struct {
	State, Reason string
	Failures      int
}

// WSHandler callbacks all run on the Run goroutine and must not block.
type WSHandler interface {
	OnStates([]HAState)
	OnStateChanged(StateChanged)
	OnRegistries(Registries)
	OnStatus(WSStatus)
}

type WSOptions struct {
	HandshakeTimeout time.Duration   // 10 s
	PingInterval     time.Duration   // 30 s
	PongTimeout      time.Duration   // 10 s
	AuthRetry        time.Duration   // 5 min
	RegistryDebounce time.Duration   // 2 s
	Backoff          []time.Duration // 1,2,4,8,16,30,60 s
	Jitter           float64         // 0 = 0.2; negative = none
	ReadLimit        int64           // 16 MiB
}

func (o WSOptions) withDefaults() WSOptions {
	def := func(d *time.Duration, v time.Duration) {
		if *d <= 0 {
			*d = v
		}
	}
	def(&o.HandshakeTimeout, 10*time.Second)
	def(&o.PingInterval, 30*time.Second)
	def(&o.PongTimeout, 10*time.Second)
	def(&o.AuthRetry, 5*time.Minute)
	def(&o.RegistryDebounce, 2*time.Second)
	if len(o.Backoff) == 0 {
		o.Backoff = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, time.Minute}
	}
	if o.Jitter == 0 {
		o.Jitter = 0.2
	}
	if o.ReadLimit <= 0 {
		o.ReadLimit = 16 << 20
	}
	return o
}

var (
	errAuthInvalid = errors.New("hass: access token rejected")
	errNoPong      = errors.New("hass: no pong")
	errProtocol    = errors.New("hass: unexpected message")
	errDial        = errors.New("hass: websocket handshake failed")
)

var registryCommand = map[string]string{
	kindAreas: "config/area_registry/list", kindDevices: "config/device_registry/list", kindEntities: "config/entity_registry/list",
}

type WSClient struct {
	url, token string
	h          WSHandler
	o          WSOptions
	dialer     *websocket.Dialer
	failures   int
}

// WebsocketURL derives ws(s)://<base>/api/websocket from the validated Home Assistant base URL.
func WebsocketURL(base string) (string, error) {
	if err := security.ValidateHTTPBaseURL(base); err != nil {
		return "", err
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/websocket"
	return u.String(), nil
}

func NewWSClient(baseURL, token string, h WSHandler, o WSOptions) (*WSClient, error) {
	u, err := WebsocketURL(baseURL)
	if err != nil {
		return nil, err
	}
	o = o.withDefaults()
	// No redirects: a 3xx fails the handshake, which keeps the same-origin rule of the REST client.
	return &WSClient{url: u, token: token, h: h, o: o,
		dialer: &websocket.Dialer{HandshakeTimeout: o.HandshakeTimeout, Proxy: http.ProxyFromEnvironment}}, nil
}

func (c *WSClient) jitter(d time.Duration) time.Duration {
	if c.o.Jitter <= 0 {
		return d
	}
	return time.Duration(float64(d) * (1 + c.o.Jitter*(2*rand.Float64()-1)))
}

func classify(err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, errNoPong), errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return ReasonTimeout
	case errors.Is(err, errDial):
		return ReasonUnreachable
	}
	return ReasonProtocol
}

// Run connects, resyncs and reconnects until ctx ends. auth_invalid retries only every AuthRetry.
func (c *WSClient) Run(ctx context.Context) error {
	if c.token != "" {
		security.RegisterSensitive(c.token)
	}
	attempt := 0
	for ctx.Err() == nil {
		c.h.OnStatus(WSStatus{State: WSConnecting, Failures: c.failures})
		authed, err := c.session(ctx)
		if ctx.Err() != nil {
			return nil
		}
		var wait time.Duration
		if errors.Is(err, errAuthInvalid) {
			c.h.OnStatus(WSStatus{State: WSAuthFailed, Reason: ReasonAuthFailed, Failures: c.failures})
			wait, attempt = c.o.AuthRetry, 0
		} else {
			if authed {
				attempt = 0
			} else {
				c.failures++
			}
			c.h.OnStatus(WSStatus{State: WSDegraded, Reason: classify(err), Failures: c.failures})
			wait = c.jitter(c.o.Backoff[min(attempt, len(c.o.Backoff)-1)])
			attempt++
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}
	}
	return nil
}

func (c *WSClient) session(ctx context.Context) (bool, error) {
	dctx, cancel := context.WithTimeout(ctx, c.o.HandshakeTimeout)
	conn, resp, err := c.dialer.DialContext(dctx, c.url, nil)
	cancel()
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return false, errors.Join(errDial, err)
	}
	defer conn.Close()
	conn.SetReadLimit(c.o.ReadLimit)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	s := &wsSession{c: c, conn: conn, pending: map[int64]string{}}
	if err := s.authenticate(); err != nil {
		return false, err
	}
	c.failures = 0
	return true, s.serve(ctx)
}

type wsSession struct {
	c        *WSClient
	conn     *websocket.Conn
	wmu      sync.Mutex // single writer
	mu       sync.Mutex // pending, nextID, reg, regLeft, relist, closed
	nextID   int64
	pending  map[int64]string
	reg      Registries
	regLeft  int
	relist   *time.Timer
	closed   bool
	lastPong atomic.Int64
}

func (s *wsSession) write(v any) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return s.conn.WriteJSON(v)
}

func (s *wsSession) request(kind string, body map[string]any) error {
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	s.pending[id] = kind
	s.mu.Unlock()
	body["id"] = id
	return s.write(body)
}

func (s *wsSession) take(id int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.pending[id]
	delete(s.pending, id)
	return k
}

func (s *wsSession) authenticate() error {
	_ = s.conn.SetReadDeadline(time.Now().Add(s.c.o.HandshakeTimeout))
	var m wsMsg
	if err := s.conn.ReadJSON(&m); err != nil {
		return err
	}
	if m.Type != "auth_required" {
		return errProtocol
	}
	if err := s.write(map[string]any{"type": "auth", "access_token": s.c.token}); err != nil {
		return err
	}
	m = wsMsg{}
	if err := s.conn.ReadJSON(&m); err != nil {
		return err
	}
	switch m.Type {
	case "auth_ok":
		return s.conn.SetReadDeadline(time.Time{})
	case "auth_invalid":
		return errAuthInvalid
	}
	return errProtocol
}

func (s *wsSession) listRegistries() error {
	s.mu.Lock()
	s.reg, s.regLeft = Registries{}, 3
	s.mu.Unlock()
	for _, k := range []string{kindAreas, kindDevices, kindEntities} {
		if err := s.request(k, map[string]any{"type": registryCommand[k]}); err != nil {
			return err
		}
	}
	return nil
}

func (s *wsSession) serve(ctx context.Context) error {
	defer s.stopRelist()
	if err := s.request(kindStates, map[string]any{"type": "get_states"}); err != nil {
		return err
	}
	if err := s.request(kindSubscribe, map[string]any{"type": "subscribe_events", "event_type": "state_changed"}); err != nil {
		return err
	}
	if err := s.listRegistries(); err != nil {
		return err
	}
	for _, ev := range []string{"area_registry_updated", "device_registry_updated", "entity_registry_updated"} {
		if err := s.request(kindSubscribe, map[string]any{"type": "subscribe_events", "event_type": ev}); err != nil {
			return err
		}
	}
	pctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pingErr := make(chan error, 1)
	go s.pinger(pctx, pingErr)
	for {
		var m wsMsg
		if err := s.conn.ReadJSON(&m); err != nil {
			select {
			case perr := <-pingErr:
				return perr
			default:
				return err
			}
		}
		if err := s.handle(m); err != nil {
			return err
		}
	}
}

func (s *wsSession) pinger(ctx context.Context, errc chan<- error) {
	t := time.NewTicker(s.c.o.PingInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		sent := time.Now()
		if err := s.request(kindPing, map[string]any{"type": "ping"}); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.c.o.PongTimeout):
		}
		if s.lastPong.Load() < sent.UnixNano() {
			errc <- errNoPong
			s.conn.Close()
			return
		}
	}
}

func (s *wsSession) handle(m wsMsg) error {
	switch m.Type {
	case "pong":
		s.lastPong.Store(time.Now().UnixNano())
		s.take(m.ID)
	case "event":
		var ev struct {
			EventType string          `json:"event_type"`
			Data      json.RawMessage `json:"data"`
		}
		if json.Unmarshal(m.Event, &ev) != nil {
			return nil
		}
		switch ev.EventType {
		case "state_changed":
			var sc StateChanged
			if json.Unmarshal(ev.Data, &sc) == nil && sc.EntityID != "" {
				s.c.h.OnStateChanged(sc)
			}
		case "area_registry_updated", "device_registry_updated", "entity_registry_updated":
			s.scheduleRelist()
		}
	case "result":
		kind, ok := s.take(m.ID), m.Success != nil && *m.Success
		switch kind {
		case kindStates:
			if !ok {
				return errProtocol
			}
			var st []HAState
			if err := json.Unmarshal(m.Result, &st); err != nil {
				return fmt.Errorf("hass: decode states: %w", err)
			}
			s.c.h.OnStates(st)
			s.c.h.OnStatus(WSStatus{State: WSLive})
		case kindAreas, kindDevices, kindEntities:
			s.registryResult(kind, ok, m.Result)
		}
	}
	return nil
}

func (s *wsSession) registryResult(kind string, ok bool, raw json.RawMessage) {
	s.mu.Lock()
	if ok {
		switch kind {
		case kindAreas:
			_ = json.Unmarshal(raw, &s.reg.Areas)
		case kindDevices:
			_ = json.Unmarshal(raw, &s.reg.Devices)
		case kindEntities:
			_ = json.Unmarshal(raw, &s.reg.Entities)
		}
	}
	s.regLeft--
	done, reg := s.regLeft == 0, s.reg
	s.mu.Unlock()
	if done {
		s.c.h.OnRegistries(reg)
	}
}

func (s *wsSession) scheduleRelist() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.relist != nil {
		s.relist.Reset(s.c.o.RegistryDebounce)
		return
	}
	s.relist = time.AfterFunc(s.c.o.RegistryDebounce, func() {
		s.mu.Lock()
		s.relist = nil
		closed := s.closed
		s.mu.Unlock()
		if !closed {
			_ = s.listRegistries()
		}
	})
}

func (s *wsSession) stopRelist() {
	s.mu.Lock()
	s.closed = true
	if s.relist != nil {
		s.relist.Stop()
	}
	s.mu.Unlock()
}
```
- [ ] `ws_client_test.go` (package `hass`):
```go
type fakeConn struct {
	mu sync.Mutex
	c  *websocket.Conn
}

func (f *fakeConn) send(v any) { f.mu.Lock(); defer f.mu.Unlock(); _ = f.c.WriteJSON(v) }

type hassFake struct {
	token      string
	states     []HAState
	ignorePing atomic.Bool
	mu         sync.Mutex
	conns      []*fakeConn
	srv        *httptest.Server
}

func newHassFake(t *testing.T, token string, states []HAState) *hassFake {
	f := &hassFake{token: token, states: states}
	up := websocket.Upgrader{}
	f.srv = testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/websocket" {
			http.NotFound(w, r)
			return
		}
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		conn := &fakeConn{c: c}
		f.mu.Lock()
		f.conns = append(f.conns, conn)
		f.mu.Unlock()
		conn.send(map[string]any{"type": "auth_required", "ha_version": "2026.11.0"})
		var auth struct {
			Type        string `json:"type"`
			AccessToken string `json:"access_token"`
		}
		if c.ReadJSON(&auth) != nil {
			return
		}
		if auth.AccessToken != f.token {
			conn.send(map[string]any{"type": "auth_invalid", "message": "Invalid access token"})
			return
		}
		conn.send(map[string]any{"type": "auth_ok"})
		for {
			var m struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			}
			if c.ReadJSON(&m) != nil {
				return
			}
			var result any
			switch m.Type {
			case "ping":
				if !f.ignorePing.Load() {
					conn.send(map[string]any{"id": m.ID, "type": "pong"})
				}
				continue
			case "get_states":
				result = f.states
			case "config/area_registry/list":
				result = []map[string]any{{"area_id": "kitchen", "name": "Kitchen"}}
			case "config/device_registry/list":
				result = []map[string]any{{"id": "dev1", "area_id": "kitchen"}}
			case "config/entity_registry/list":
				result = []map[string]any{{"entity_id": "light.kitchen", "device_id": "dev1", "area_id": nil}}
			}
			conn.send(map[string]any{"id": m.ID, "type": "result", "success": true, "result": result})
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *hassFake) push(eventType string, data any) {
	f.mu.Lock()
	c := f.conns[len(f.conns)-1]
	f.mu.Unlock()
	c.send(map[string]any{"id": 2, "type": "event", "event": map[string]any{"event_type": eventType, "data": data}})
}

func (f *hassFake) drop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.conns {
		c.c.Close()
	}
}

func (f *hassFake) connections() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.conns) }

type recHandler struct {
	mu       sync.Mutex
	states   [][]HAState
	changes  []StateChanged
	regs     []Registries
	statuses []WSStatus
}

func (r *recHandler) OnStates(s []HAState)          { r.mu.Lock(); r.states = append(r.states, s); r.mu.Unlock() }
func (r *recHandler) OnStateChanged(c StateChanged) { r.mu.Lock(); r.changes = append(r.changes, c); r.mu.Unlock() }
func (r *recHandler) OnRegistries(g Registries)     { r.mu.Lock(); r.regs = append(r.regs, g); r.mu.Unlock() }
func (r *recHandler) OnStatus(s WSStatus)           { r.mu.Lock(); r.statuses = append(r.statuses, s); r.mu.Unlock() }
func (r *recHandler) count() (states, changes, regs int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.states), len(r.changes), len(r.regs)
}
func (r *recHandler) sawStatus(state, reason string, failures int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.ContainsFunc(r.statuses, func(s WSStatus) bool {
		return s.State == state && (reason == "" || s.Reason == reason) && s.Failures >= failures
	})
}

var testWSOptions = WSOptions{HandshakeTimeout: time.Second, PingInterval: time.Hour, PongTimeout: time.Minute,
	AuthRetry: time.Hour, RegistryDebounce: 20 * time.Millisecond, Backoff: []time.Duration{30 * time.Millisecond}, Jitter: -1}

func startClient(t *testing.T, base, token string, o WSOptions) *recHandler {
	t.Helper()
	h := &recHandler{}
	c, err := NewWSClient(base, token, h, o)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = c.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return h
}

func waitUntil(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHAWebsocketAuthGetStatesAndStateChanged(t *testing.T) {
	f := newHassFake(t, "tok", []HAState{{EntityID: "light.kitchen", State: "on"}})
	h := startClient(t, f.srv.URL, "tok", testWSOptions)
	waitUntil(t, func() bool { s, _, g := h.count(); return s == 1 && g == 1 && h.sawStatus(WSLive, "", 0) }, "states, registries, live")
	h.mu.Lock()
	reg := h.regs[0]
	h.mu.Unlock()
	if len(reg.Areas) != 1 || reg.Devices[0].AreaID != "kitchen" || reg.Entities[0].DeviceID != "dev1" {
		t.Fatalf("registries %+v", reg)
	}
	f.push("state_changed", map[string]any{"entity_id": "light.kitchen", "new_state": map[string]any{"entity_id": "light.kitchen", "state": "off"}})
	waitUntil(t, func() bool { _, c, _ := h.count(); return c == 1 }, "state_changed")
	if h.changes[0].NewState == nil || h.changes[0].NewState.State != "off" {
		t.Fatalf("change %+v", h.changes[0])
	}
	f.push("area_registry_updated", map[string]any{})
	f.push("entity_registry_updated", map[string]any{})
	waitUntil(t, func() bool { _, _, g := h.count(); return g == 2 }, "debounced re-list")
	time.Sleep(60 * time.Millisecond)
	if _, _, g := h.count(); g != 2 {
		t.Fatalf("re-listed %d times, want one debounced re-list", g-1)
	}
}

func TestHAWebsocketReconnectBackoffAndResync(t *testing.T) {
	f := newHassFake(t, "tok", []HAState{{EntityID: "switch.a", State: "on"}})
	h := startClient(t, f.srv.URL, "tok", testWSOptions)
	waitUntil(t, func() bool { s, _, _ := h.count(); return s == 1 }, "first states")
	dropped := time.Now()
	f.drop()
	waitUntil(t, func() bool { s, _, _ := h.count(); return s == 2 }, "resync after reconnect")
	if time.Since(dropped) < 30*time.Millisecond || f.connections() != 2 || !h.sawStatus(WSDegraded, "", 0) {
		t.Fatalf("elapsed %v connections %d", time.Since(dropped), f.connections())
	}
}

func TestHAAuthInvalidStopsTightRetry(t *testing.T) {
	f := newHassFake(t, "tok", nil)
	h := startClient(t, f.srv.URL, "wrong", testWSOptions)
	waitUntil(t, func() bool { return h.sawStatus(WSAuthFailed, ReasonAuthFailed, 0) }, "auth_failed")
	time.Sleep(150 * time.Millisecond)
	if n := f.connections(); n != 1 {
		t.Fatalf("%d connections after auth_invalid, want 1", n)
	}
}

func TestHAWebsocketMissingPongReconnects(t *testing.T) {
	f := newHassFake(t, "tok", nil)
	f.ignorePing.Store(true)
	o := testWSOptions
	o.PingInterval, o.PongTimeout = 40*time.Millisecond, 20*time.Millisecond
	h := startClient(t, f.srv.URL, "tok", o)
	waitUntil(t, func() bool { return f.connections() >= 2 && h.sawStatus(WSDegraded, ReasonTimeout, 0) }, "reconnect after missing pong")
}

func TestHAWebsocketCountsFailedHandshakes(t *testing.T) {
	srv := testutil.NewHTTPServer(t, http.NotFoundHandler())
	defer srv.Close()
	h := startClient(t, srv.URL, "tok", testWSOptions)
	waitUntil(t, func() bool { return h.sawStatus(WSDegraded, ReasonUnreachable, 3) }, "three failed handshakes")
}

func TestWebsocketURL(t *testing.T) {
	for in, want := range map[string]string{
		"http://ha.lan:8123":        "ws://ha.lan:8123/api/websocket",
		"https://ha.example/":       "wss://ha.example/api/websocket",
		"https://proxy.lan/hass":    "wss://proxy.lan/hass/api/websocket",
		"ftp://ha.lan":              "",
		"http://user:pw@ha.lan":     "",
		"http://ha.lan/?token=abc": "",
	} {
		got, err := WebsocketURL(in)
		if (want == "") != (err != nil) || got != want {
			t.Errorf("%s: %q %v", in, got, err)
		}
	}
}
```
- [ ] Run `go test ./internal/homedash/sources/hass/ -count=1 -v` → PASS the six websocket tests and the Task 4 tests; `go test ./internal/audit/ -run '(?i)(NetworkClient|CredentialedHTTP)' -count=1` stays green.
- [ ] Commit `git add internal/homedash/sources/hass`; `feat(homedash): Home Assistant websocket client with reconnect, keepalive and registry re-list`.
- **Done when:** tests pass; the client never builds an `http.Client`.

### Task 7: B3 store (`homedash.db`)

**Files:** Create `internal/homedash/{store,store_dashboards,store_kiosk,store_test}.go`. Modify `internal/config/sqlite_paths.go:28-29` and `:97-98`, `internal/config/sqlite_paths_test.go:49`.
**Read first:** backend §7 "Store schema" and "Store rules", §9 kiosk rows; main §4.5; `internal/chibisensei/store.go:23-42`; `internal/dbutil/open.go:16-55` and `options.go:38-46` (`dbutil.Open` gives WAL, `foreign_keys(1)`, busy timeout; `WithMaxOpenConns(1)`); `internal/dbutil/AGENTS.md`; `internal/audit/sqlite_backup_coverage_test.go:37-97` (keep the literal `"homedash.db"` out of `internal/homedash` production code; the store takes a path).

- [ ] Write `store_test.go` (package `homedash`):
```go
func storeDoc(slug string) Dashboard {
	return Dashboard{SchemaVersion: 1, Slug: slug, Name: "Board", Pages: []Page{{ID: "main", Name: "Main", Widgets: []Widget{}, Layouts: Layouts{LG: []LayoutItem{}}}}}
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
```
  In `internal/config/sqlite_paths_test.go`: add `filepath.Join(dataDir, "homedash.db"): true,` after line 49, and append:
```go
func TestSQLiteArchivePathsIncludeHomeDash(t *testing.T) {
	cfg := &Config{}
	cfg.Directories.DataDir = filepath.Join(t.TempDir(), "data")
	want := filepath.Join(cfg.Directories.DataDir, HomeDashDBFilename)
	if got := SQLiteArchivePaths(cfg)[SQLiteArchivePrefix+"homedash.db"]; got != want {
		t.Fatalf("archive path %q, want %q", got, want)
	}
	if !containsPath(SQLiteProtectedPaths(cfg), want+"-wal") {
		t.Fatal("homedash.db WAL is not protected from agent file tools")
	}
}
```
- [ ] Run `go test ./internal/homedash/ -run '(?i)(Store)' -count=1` and `go test ./internal/config/ -run '(?i)(SQLite)' -count=1` → FAIL (undefined).
- [ ] GitNexus impact on `SQLiteArchivePaths` (callers: backup/restore, `SQLiteDatabasePaths`, `SQLiteProtectedPaths`).
- [ ] `sqlite_paths.go`: in the const block after line 29 `// HomeDashDBFilename is the HomeDash store (<data_dir>/homedash.db).` / `HomeDashDBFilename = "homedash.db"`; after line 98 `paths["homedash"] = filepath.Join(dataDir, HomeDashDBFilename)`.
- [ ] `store.go` — full code:
```go
package homedash

const storeSchemaVersion = 1

const storeSchemaSQL = `
CREATE TABLE IF NOT EXISTS hd_meta (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL);
INSERT OR IGNORE INTO hd_meta (id, version) VALUES (1, 1);
CREATE TABLE IF NOT EXISTS hd_dashboards (
	id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, schema_version INTEGER NOT NULL,
	revision INTEGER NOT NULL, approved_revision INTEGER NOT NULL DEFAULT 0, document_json TEXT NOT NULL,
	saved_by TEXT NOT NULL CHECK (saved_by IN ('user','agent')), created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS hd_dashboard_revisions (
	dashboard_id TEXT NOT NULL REFERENCES hd_dashboards(id) ON DELETE CASCADE, revision INTEGER NOT NULL,
	document_json TEXT NOT NULL, saved_by TEXT NOT NULL, saved_at TEXT NOT NULL, PRIMARY KEY (dashboard_id, revision));
CREATE TABLE IF NOT EXISTS hd_kiosk_pairings (
	code_hash TEXT PRIMARY KEY, dashboard_id TEXT NOT NULL REFERENCES hd_dashboards(id) ON DELETE CASCADE,
	label TEXT NOT NULL, view_only INTEGER NOT NULL, expires_at TEXT NOT NULL, used_at TEXT);
CREATE TABLE IF NOT EXISTS hd_kiosk_devices (
	id TEXT PRIMARY KEY, dashboard_id TEXT NOT NULL REFERENCES hd_dashboards(id) ON DELETE CASCADE,
	label TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE, view_only INTEGER NOT NULL, created_at TEXT NOT NULL,
	last_seen_at TEXT, last_ip TEXT, expires_at TEXT);
CREATE TABLE IF NOT EXISTS hd_mqtt_entities (
	id TEXT PRIMARY KEY, name TEXT NOT NULL, kind TEXT NOT NULL, state_topic TEXT NOT NULL, json_path TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL DEFAULT '', device_class TEXT NOT NULL DEFAULT '', command_topic TEXT NOT NULL DEFAULT '',
	payload_on TEXT NOT NULL DEFAULT '', payload_off TEXT NOT NULL DEFAULT '', options_json TEXT NOT NULL DEFAULT '[]',
	min REAL, max REAL, step REAL, qos INTEGER NOT NULL DEFAULT 0, retain INTEGER NOT NULL DEFAULT 0, updated_at TEXT NOT NULL);
`

// Store is homedash.db: one connection, WAL, foreign keys on (dbutil.Open). It does not validate
// documents; callers run Normalize, Autopack and Validate first. hd_mqtt_entities CRUD comes with B12.
type Store struct {
	db            *sql.DB
	now           func() time.Time
	maxDashboards int
}

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create HomeDash data directory: %w", err)
	}
	db, err := dbutil.Open(path, dbutil.WithMaxOpenConns(1))
	if err != nil {
		return nil, fmt.Errorf("open HomeDash database: %w", err)
	}
	if _, err := db.Exec(storeSchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize HomeDash database: %w", err)
	}
	var v int
	if err := db.QueryRow(`SELECT version FROM hd_meta WHERE id = 1`).Scan(&v); err != nil || v != storeSchemaVersion {
		db.Close()
		return nil, errors.New("unsupported HomeDash database schema")
	}
	return &Store{db: db, now: func() time.Time { return time.Now().UTC() }, maxDashboards: 50}, nil
}

func (s *Store) Close() error            { return s.db.Close() }
func (s *Store) SetMaxDashboards(n int) { if n > 0 { s.maxDashboards = n } }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTS(v string) time.Time { t, _ := time.Parse(time.RFC3339Nano, v); return t }

func hashSecret(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
```
  Later schema versions back up with `dbutil.BackupSQLiteForMigration` before any change (dbutil AGENTS.md); version 1 needs none.
- [ ] `store_dashboards.go`: types and rules, with `UpdateDashboard`, `Approve` and the prune in full:
```go
const (
	SavedByUser  = "user"
	SavedByAgent = "agent"
	MaxRevisions = 20
)

type DashboardSummary struct {
	ID               string    `json:"id"`
	Slug             string    `json:"slug"`
	Name             string    `json:"name"`
	Icon             string    `json:"icon,omitempty"`
	Revision         int64     `json:"revision"`
	ApprovedRevision int64     `json:"approved_revision"`
	UpdatedAt        time.Time `json:"updated_at"`
	SavedBy          string    `json:"saved_by"`
}

type RevisionInfo struct {
	Revision int64     `json:"revision"`
	SavedAt  time.Time `json:"saved_at"`
	SavedBy  string    `json:"saved_by"`
}

func encodeDocument(d Dashboard) (string, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	if len(raw) > Catalog().Limits.DocumentBytes {
		return "", newError(CodeLimitExceeded, "the document is larger than 256 KiB")
	}
	return string(raw), nil
}

func slugError(err error) error {
	if err != nil && strings.Contains(err.Error(), "hd_dashboards.slug") {
		return newError(CodeSlugTaken, "another dashboard uses this slug")
	}
	return err
}

func (s *Store) UpdateDashboard(ctx context.Context, doc Dashboard, revision int64, by string) (Dashboard, error) {
	if by != SavedByUser && by != SavedByAgent {
		return Dashboard{}, newError(CodeBadRequest, "saved_by must be user or agent")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Dashboard{}, err
	}
	defer tx.Rollback()
	var cur, approved int64
	err = tx.QueryRowContext(ctx, `SELECT revision, approved_revision FROM hd_dashboards WHERE id = ?`, doc.ID).Scan(&cur, &approved)
	if errors.Is(err, sql.ErrNoRows) {
		return Dashboard{}, ErrDashboardNotFound
	} else if err != nil {
		return Dashboard{}, err
	}
	if cur != revision {
		return Dashboard{}, &Error{Code: CodeRevisionConflict, Message: "the dashboard was changed elsewhere", CurrentRevision: cur}
	}
	next, now := cur+1, s.now()
	if by == SavedByUser {
		approved = next // a user save approves; an agent save never moves what displays show
	}
	doc.SchemaVersion, doc.Revision, doc.ApprovedRevision, doc.UpdatedAt, doc.SavedBy = SchemaVersion, next, approved, now, by
	raw, err := encodeDocument(doc)
	if err != nil {
		return Dashboard{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE hd_dashboards SET slug = ?, name = ?, schema_version = ?, revision = ?, approved_revision = ?,
		document_json = ?, saved_by = ?, updated_at = ? WHERE id = ?`, doc.Slug, doc.Name, doc.SchemaVersion, next, approved, raw, by, ts(now), doc.ID); err != nil {
		return Dashboard{}, slugError(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO hd_dashboard_revisions (dashboard_id, revision, document_json, saved_by, saved_at)
		VALUES (?, ?, ?, ?, ?)`, doc.ID, next, raw, by, ts(now)); err != nil {
		return Dashboard{}, err
	}
	// Keep the last MaxRevisions and always the approved one (kiosk displays read it).
	if _, err := tx.ExecContext(ctx, `DELETE FROM hd_dashboard_revisions WHERE dashboard_id = ? AND revision <= ? AND revision <> ?`,
		doc.ID, next-MaxRevisions, approved); err != nil {
		return Dashboard{}, err
	}
	return doc, tx.Commit()
}

func (s *Store) Approve(ctx context.Context, id string, revision int64) (Dashboard, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Dashboard{}, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hd_dashboards WHERE id = ?`, id).Scan(&n); err != nil {
		return Dashboard{}, err
	} else if n == 0 {
		return Dashboard{}, ErrDashboardNotFound
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hd_dashboard_revisions WHERE dashboard_id = ? AND revision = ?`, id, revision).Scan(&n); err != nil {
		return Dashboard{}, err
	} else if n == 0 {
		return Dashboard{}, newError(CodeRevisionNotFound, "no such revision")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE hd_dashboards SET approved_revision = ? WHERE id = ?`, revision, id); err != nil {
		return Dashboard{}, err
	}
	if err := tx.Commit(); err != nil {
		return Dashboard{}, err
	}
	return s.GetDashboard(ctx, id)
}
```
  Remaining methods, rules:
  1. `CreateDashboard(ctx, doc, by) (Dashboard, error)`: check `by`; count rows, `>= maxDashboards` → `limit_exceeded`; `ID = NewDashboardID()`, revision 1, approved 1 for user and 0 for agent, `SchemaVersion`, `UpdatedAt = now`, `SavedBy`; insert the row and revision 1 in one transaction; unique slug violation → `slug_taken` (`slugError`).
  2. `GetDashboard(ctx, id) (Dashboard, error)`: read `document_json`, `revision`, `approved_revision`, `saved_by`, `updated_at`; unmarshal; overwrite `ID`, `Revision`, `ApprovedRevision`, `SavedBy`, `UpdatedAt` from the columns; then `Migrate` — on `schema_too_new` return the document and the error together. Missing row → `ErrDashboardNotFound`.
  3. `ListDashboards(ctx) ([]DashboardSummary, error)`: from columns (icon from the decoded document), ordered by name, then id.
  4. `Revisions(ctx, id) ([]RevisionInfo, error)`: newest first; unknown dashboard → `ErrDashboardNotFound`.
  5. `GetRevision(ctx, id, rev) (Dashboard, error)`: the stored document with `ID`, `Revision = rev`, and the current `ApprovedRevision` column; missing → `revision_not_found`.
  6. `GetApproved(ctx, id) (Dashboard, error)`: `approved_revision` 0 → `*Error{Code: CodeDashboardNotFound, Message: "no approved revision"}`; else `GetRevision`.
  7. `DeleteDashboard(ctx, id, revision) ([]string, error)`: in one transaction: missing → not found; revision mismatch → `revision_conflict` with `CurrentRevision`; collect device ids (ordered by id); delete the row (cascades revisions, pairings, devices); return the ids so the server calls `Hub.RevokeDevice`.
- [ ] `store_kiosk.go` — full code:
```go
const (
	DeviceTokenPrefix = "hdk_"
	pairingCodeLen    = 26 // base32 of 16 bytes, no padding
)

type KioskDevice struct {
	ID          string     `json:"id"`
	DashboardID string     `json:"dashboard_id"`
	Label       string     `json:"label"`
	ViewOnly    bool       `json:"view_only"`
	CreatedAt   time.Time  `json:"created_at"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	LastIP      string     `json:"last_ip,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

var errPairingInvalid = newError(CodePairingInvalid, "the pairing code is invalid or expired")

// CreatePairing stores only the SHA-256 of a single-use 128-bit code and returns the code once.
func (s *Store) CreatePairing(ctx context.Context, dashboardID, label string, viewOnly bool, ttl time.Duration) (string, time.Time, error) {
	if label = CleanText(label, 60); label == "" {
		label = "Display"
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM hd_dashboards WHERE id = ?`, dashboardID).Scan(&n); err != nil {
		return "", time.Time{}, err
	} else if n == 0 {
		return "", time.Time{}, ErrDashboardNotFound
	}
	now := s.now()
	code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes(16))
	expires := now.Add(ttl)
	if _, err := s.db.ExecContext(ctx, `DELETE FROM hd_kiosk_pairings WHERE expires_at < ? OR used_at IS NOT NULL`, ts(now)); err != nil {
		return "", time.Time{}, err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO hd_kiosk_pairings (code_hash, dashboard_id, label, view_only, expires_at) VALUES (?, ?, ?, ?, ?)`,
		hashSecret(code), dashboardID, label, dbutil.BoolToInt(viewOnly), ts(expires))
	return code, expires, err
}

// RedeemPairing consumes a code once and creates a device. Unknown, used, expired and foreign codes
// all return the same pairing_invalid. The device token is returned once and stored as SHA-256.
func (s *Store) RedeemPairing(ctx context.Context, dashboardID, code, ip string, deviceTTL time.Duration, maxDevices int) (KioskDevice, string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != pairingCodeLen {
		return KioskDevice{}, "", errPairingInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return KioskDevice{}, "", err
	}
	defer tx.Rollback()
	now := s.now()
	var label, expires string
	var viewOnly int
	var used sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT label, view_only, expires_at, used_at FROM hd_kiosk_pairings WHERE code_hash = ? AND dashboard_id = ?`,
		hashSecret(code), dashboardID).Scan(&label, &viewOnly, &expires, &used)
	if err != nil || used.Valid || !parseTS(expires).After(now) {
		return KioskDevice{}, "", errPairingInvalid
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hd_kiosk_devices`).Scan(&count); err != nil {
		return KioskDevice{}, "", err
	}
	if maxDevices > 0 && count >= maxDevices {
		return KioskDevice{}, "", newError(CodeLimitExceeded, "too many paired displays")
	}
	res, err := tx.ExecContext(ctx, `UPDATE hd_kiosk_pairings SET used_at = ? WHERE code_hash = ? AND used_at IS NULL`, ts(now), hashSecret(code))
	if err != nil {
		return KioskDevice{}, "", err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return KioskDevice{}, "", errPairingInvalid
	}
	token := DeviceTokenPrefix + randomHex(32)
	dev := KioskDevice{ID: "kd_" + randomBase32(12), DashboardID: dashboardID, Label: label, ViewOnly: viewOnly == 1, CreatedAt: now, LastIP: CleanText(ip, 64)}
	var exp any
	if deviceTTL > 0 {
		e := now.Add(deviceTTL)
		dev.ExpiresAt, exp = &e, ts(e)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO hd_kiosk_devices (id, dashboard_id, label, token_hash, view_only, created_at, last_seen_at, last_ip, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, dev.ID, dashboardID, label, hashSecret(token), viewOnly, ts(now), ts(now), dev.LastIP, exp); err != nil {
		return KioskDevice{}, "", err
	}
	dev.LastSeenAt = &now
	return dev, token, tx.Commit()
}

// DeviceByToken resolves a cookie token; an expired device is deleted and reported revoked.
func (s *Store) DeviceByToken(ctx context.Context, token string) (KioskDevice, error) {
	unpaired := newError(CodeKioskUnpaired, "this display is not paired")
	if !strings.HasPrefix(token, DeviceTokenPrefix) || len(token) != len(DeviceTokenPrefix)+64 {
		return KioskDevice{}, unpaired
	}
	var d KioskDevice
	var viewOnly int
	var created string
	var seen, ip, expires sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, dashboard_id, label, view_only, created_at, last_seen_at, last_ip, expires_at
		FROM hd_kiosk_devices WHERE token_hash = ?`, hashSecret(token)).Scan(&d.ID, &d.DashboardID, &d.Label, &viewOnly, &created, &seen, &ip, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return KioskDevice{}, unpaired
	} else if err != nil {
		return KioskDevice{}, err
	}
	d.ViewOnly, d.CreatedAt, d.LastIP = viewOnly == 1, parseTS(created), ip.String
	if seen.Valid {
		t := parseTS(seen.String)
		d.LastSeenAt = &t
	}
	if expires.Valid {
		t := parseTS(expires.String)
		d.ExpiresAt = &t
		if !t.After(s.now()) {
			_, _ = s.db.ExecContext(ctx, `DELETE FROM hd_kiosk_devices WHERE id = ?`, d.ID)
			return KioskDevice{}, newError(CodeKioskRevoked, "this display's pairing expired")
		}
	}
	return d, nil
}
```
  Remaining, rules: `TouchDevice(ctx, id, ip string) error` sets `last_seen_at = now`, `last_ip = CleanText(ip, 64)`; `ListDevices(ctx, dashboardID string) ([]KioskDevice, error)` (empty `dashboardID` = all) ordered by `created_at`, never selecting `token_hash`; `RevokeDevice(ctx, id string) error` deletes the row (missing → `*Error{Code: CodeKioskUnpaired}`).
- [ ] Run `go test ./internal/homedash/ -count=1 -v` (five `TestStore*` PASS plus earlier), `go test ./internal/config/... -count=1`, `go test ./internal/audit/ -run TestSQLiteOpensAreBackedUpOrExcluded -count=1` → PASS.
- [ ] Commit `git add internal/homedash internal/config/sqlite_paths.go internal/config/sqlite_paths_test.go`; `feat(homedash): homedash.db store with revisions, approval and kiosk pairing` + GitNexus lines for `SQLiteArchivePaths`.
- **Done when:** store tests pass; `homedash.db` is in backups and protected from agent file tools; no clear-text secret on disk.

### Task 8: B4b commands, rate limiter, kiosk allowed set, history

**Files:** Create `internal/homedash/{command,ratelimit,history,command_test,ratelimit_test,history_test}.go`. Modify `internal/homedash/principal.go` (append `BuildAllowedSet`), `internal/homedash/hub.go` (`Hub` struct and `NewHub`).
**Read first:** backend §5 "Commands" (order 1–12) and "History"; §9 confirmation list; main §4.5; `internal/server/flows_handlers.go:72-107` (limiter semantics copied, not imported).

- [ ] `ratelimit_test.go` (package `homedash`):
```go
func TestRateLimiterWindowsAreAtomic(t *testing.T) {
	var l rateLimiter
	base := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	ws := []window{{"p", 3, time.Minute}, {"e", 2, 5 * time.Second}}
	for i := 0; i < 2; i++ {
		if ok, _ := l.allow(base, ws); !ok {
			t.Fatalf("event %d refused", i)
		}
	}
	ok, wait := l.allow(base.Add(time.Second), ws)
	if ok || wait != 4*time.Second {
		t.Fatalf("third event: ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.allow(base.Add(6*time.Second), ws); !ok {
		t.Fatal("refused after the entity window passed")
	}
	if ok, _ := l.allow(base.Add(7*time.Second), []window{{"p", 3, time.Minute}}); ok {
		t.Fatal("the refused event must not have counted, but the third accepted one must")
	}
}
```
- [ ] `history_test.go` (package `homedash`):
```go
func TestHistoryRingDownsamplesAndEvictsLRU(t *testing.T) {
	base := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	hs := newHistoryStore(4, 2, time.Minute)
	hs.observe("fa:a", 9, base) // no ring yet: ignored
	hs.touch("fa:a", base)
	for i, v := range []float64{1, 2, 3} {
		hs.observe("fa:a", v, base.Add(time.Duration(i)*20*time.Second))
	}
	if got := hs.series("fa:a", base, base.Add(time.Hour), 100); len(got) != 1 || got[0].V != 3 {
		t.Fatalf("one bucket, last value wins: %+v", got)
	}
	for i := 1; i <= 6; i++ {
		hs.observe("fa:a", float64(i*10), base.Add(time.Duration(i)*time.Minute))
	}
	got := hs.series("fa:a", base, base.Add(time.Hour), 100)
	if len(got) != 4 || got[0].V != 30 || got[3].V != 60 {
		t.Fatalf("ring keeps the last 4 points: %+v", got)
	}
	if got := hs.series("fa:a", base, base.Add(time.Hour), 2); len(got) != 2 || got[0].V != 35 || got[1].V != 55 {
		t.Fatalf("downsampled: %+v", got)
	}
	hs.touch("fa:b", base.Add(10*time.Minute))
	hs.touch("fa:c", base.Add(11*time.Minute))
	if got := hs.series("fa:a", base, base.Add(time.Hour), 10); got != nil {
		t.Fatal("least recently used ring not evicted")
	}
}
```
- [ ] `command_test.go` (package `homedash_test`; reuses `newTestHub`, `session` from `hub_test.go`):
```go
func f64(v float64) *float64 { return &v }

func cmd(id string, c homedash.Capability, args map[string]any, p homedash.Principal) homedash.Command {
	if args == nil {
		args = map[string]any{}
	}
	return homedash.Command{Entity: homedash.EntityID(id), Name: c, Args: args, Principal: p}
}

func gateSource() *sourcetest.FakeSource {
	return sourcetest.New("fa",
		sourcetest.Entity("fa:light", homedash.KindLight, homedash.CapabilitySpec{Name: homedash.CapToggle, Allowed: true},
			homedash.CapabilitySpec{Name: homedash.CapBrightness, Allowed: true, Min: f64(0), Max: f64(100), Step: f64(1)}),
		sourcetest.Entity("fa:lock", homedash.KindLock, homedash.CapabilitySpec{Name: homedash.CapLock, Allowed: true},
			homedash.CapabilitySpec{Name: homedash.CapUnlock, Allowed: true}),
		sourcetest.Entity("fa:cover", homedash.KindCover, homedash.CapabilitySpec{Name: homedash.CapOpen, BlockedBy: "home_assistant.allowed_services"}))
}

func TestCommandPipelineGates(t *testing.T) {
	sess := session("s")
	kiosk := func(viewOnly bool) homedash.Principal {
		return homedash.Principal{Kind: homedash.PrincipalKiosk, Key: "kd", DeviceID: "kd", ViewOnly: viewOnly,
			Allowed: &homedash.AllowedSet{Commands: map[homedash.EntityID]map[homedash.Capability]bool{
				"fa:light": {homedash.CapToggle: true}, "fa:lock": {homedash.CapUnlock: true}}}}
	}
	agent := homedash.Principal{Kind: homedash.PrincipalAgent, Key: "a"}
	unlockConfirmed := cmd("fa:lock", homedash.CapUnlock, nil, kiosk(false))
	unlockConfirmed.Confirmed = true
	for _, tc := range []struct {
		name            string
		mut             func(*homedash.RuntimeConfig)
		c               homedash.Command
		code, blockedBy string
	}{
		{"unknown entity", nil, cmd("fa:nope", homedash.CapToggle, nil, sess), homedash.CodeEntityNotFound, ""},
		{"unknown source", nil, cmd("zz:x", homedash.CapToggle, nil, sess), homedash.CodeEntityNotFound, ""},
		{"missing capability", nil, cmd("fa:light", homedash.CapOpen, nil, sess), homedash.CodeUnsupportedCommand, ""},
		{"bad args", nil, cmd("fa:light", homedash.CapBrightness, map[string]any{"value": 150.0}, sess), homedash.CodeInvalidArgs, ""},
		{"integration blocked", nil, cmd("fa:cover", homedash.CapOpen, nil, sess), homedash.CodeReadOnly, "home_assistant.allowed_services"},
		{"homedash readonly", func(c *homedash.RuntimeConfig) { c.ReadOnly = true }, cmd("fa:light", homedash.CapToggle, nil, sess), homedash.CodeReadOnly, "homedash.readonly"},
		{"desktop readonly", func(c *homedash.RuntimeConfig) { c.DesktopReadOnly = true }, cmd("fa:light", homedash.CapToggle, nil, sess), homedash.CodeReadOnly, "virtual_desktop.readonly"},
		{"control off", func(c *homedash.RuntimeConfig) { s := c.Sources["fa"]; s.Control = false; c.Sources["fa"] = s },
			cmd("fa:light", homedash.CapToggle, nil, sess), homedash.CodeReadOnly, "homedash.sources.fa.control"},
		{"kiosk outside set", nil, cmd("fa:lock", homedash.CapLock, nil, kiosk(false)), homedash.CodeForbiddenEntity, ""},
		{"kiosk view only", nil, cmd("fa:light", homedash.CapToggle, nil, kiosk(true)), homedash.CodeViewOnly, ""},
		{"kiosk control off", func(c *homedash.RuntimeConfig) { c.KioskAllowControl = false }, cmd("fa:light", homedash.CapToggle, nil, kiosk(false)), homedash.CodeViewOnly, ""},
		{"kiosk denied unlock", nil, unlockConfirmed, homedash.CodeForbiddenEntity, "kiosk.denied"},
		{"sensitive unconfirmed", nil, cmd("fa:lock", homedash.CapUnlock, nil, sess), homedash.CodeConfirmationRequired, ""},
		{"agent", nil, cmd("fa:light", homedash.CapToggle, nil, agent), homedash.CodeForbiddenEntity, "agent"},
		{"ok", nil, cmd("fa:light", homedash.CapToggle, nil, sess), "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fa := gateSource()
			h := newTestHub(t, tc.mut, fa)
			var outcomes []string
			homedash.SetAuditForTest(h, func(_ context.Context, _ homedash.Command, outcome string) { outcomes = append(outcomes, outcome) })
			_, err := h.Command(context.Background(), tc.c)
			var he *homedash.Error
			got, blocked := "", ""
			if errors.As(err, &he) {
				got, blocked = he.Code, he.BlockedBy
			} else if err != nil {
				got = "untyped"
			}
			if got != tc.code || blocked != tc.blockedBy {
				t.Fatalf("code %q blocked_by %q, want %q %q (%v)", got, blocked, tc.code, tc.blockedBy, err)
			}
			if tc.code == "" && (len(fa.Commands()) != 1 || !slices.Equal(outcomes, []string{"ok"})) {
				t.Fatalf("commands %d outcomes %v", len(fa.Commands()), outcomes)
			}
			if tc.code != "" && len(fa.Commands()) != 0 {
				t.Fatal("a refused command reached the source")
			}
		})
	}
}

func TestCommandRateLimitAndRequestIDDedupe(t *testing.T) {
	fa := gateSource()
	h := newTestHub(t, nil, fa)
	ctx := context.Background()
	first := cmd("fa:light", homedash.CapToggle, nil, session("s"))
	first.RequestID = "req-1"
	for i := 0; i < 2; i++ {
		if _, err := h.Command(ctx, first); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(fa.Commands()); n != 1 {
		t.Fatalf("duplicate request id reached the source %d times", n)
	}
	for i := 0; i < 2; i++ { // per entity 5 per 5 s; 3 events counted so far
		if _, err := h.Command(ctx, cmd("fa:light", homedash.CapToggle, nil, session("s"))); err != nil {
			t.Fatal(err)
		}
	}
	_, err := h.Command(ctx, cmd("fa:light", homedash.CapToggle, nil, session("s")))
	var he *homedash.Error
	if !errors.As(err, &he) || he.Code != homedash.CodeRateLimited || he.RetryAfterMS <= 0 {
		t.Fatalf("sixth command: %v", err)
	}
}

func TestCommandLatestWinsSlot(t *testing.T) {
	fa := gateSource()
	fa.CommandGate = make(chan struct{})
	h := newTestHub(t, nil, fa)
	ctx := context.Background()
	send := func(v float64) chan homedash.CommandResult {
		out := make(chan homedash.CommandResult, 1)
		go func() {
			res, err := h.Command(ctx, cmd("fa:light", homedash.CapBrightness, map[string]any{"value": v}, session("s")))
			if err != nil {
				t.Errorf("value %v: %v", v, err)
			}
			out <- res
		}()
		return out
	}
	a := send(10)
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.InCommand() == 1 }, "first command in flight")
	b := send(20)
	time.Sleep(30 * time.Millisecond)
	c := send(30)
	if res := <-b; !res.Superseded {
		t.Fatal("queued value 20 was not superseded by 30")
	}
	close(fa.CommandGate)
	<-a
	<-c
	sourcetest.WaitFor(t, time.Second, func() bool { return len(fa.Commands()) == 2 }, "two commands sent")
	got := fa.Commands()
	if got[0].Args["value"] != 10.0 || got[1].Args["value"] != 30.0 {
		t.Fatalf("sent %v then %v, want 10 then 30", got[0].Args["value"], got[1].Args["value"])
	}
}
```
  `SetAuditForTest` lives in a test-only file `internal/homedash/export_test.go` (package `homedash`): `func SetAuditForTest(h *Hub, fn func(context.Context, Command, string)) { h.mu.Lock(); h.opts.Audit = fn; h.mu.Unlock() }`. Because `export_test.go` is compiled into the package test binary, the external `homedash_test` package can call it. The audit call reads `h.opts.Audit` under `h.mu`.
- [ ] Run `go test ./internal/homedash/ -run '(?i)(Command|RateLimiter|HistoryRing)' -count=1` → FAIL (undefined).
- [ ] `ratelimit.go` — full code:
```go
package homedash

type window struct {
	key   string
	limit int
	per   time.Duration
}

type hitList struct {
	per time.Duration
	ts  []time.Time
}

// rateLimiter is a sliding-window limiter (flowRateLimiter semantics, flows_handlers.go:72-107)
// that checks several windows atomically. The zero value is ready.
type rateLimiter struct {
	mu   sync.Mutex
	hits map[string]*hitList
}

const rateKeysSweep = 256

// allow records one event in every window when all have room; otherwise it records nothing and
// returns how long until the fullest window frees a slot.
func (l *rateLimiter) allow(now time.Time, ws []window) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hits == nil {
		l.hits = map[string]*hitList{}
	}
	if len(l.hits) > rateKeysSweep {
		for k, h := range l.hits {
			if len(h.ts) == 0 || !h.ts[len(h.ts)-1].After(now.Add(-h.per)) {
				delete(l.hits, k)
			}
		}
	}
	ok, wait := true, time.Duration(0)
	for _, w := range ws {
		if w.limit <= 0 {
			continue
		}
		h := l.hits[w.key]
		if h == nil {
			h = &hitList{per: w.per}
			l.hits[w.key] = h
		}
		cut := now.Add(-w.per)
		i := 0
		for i < len(h.ts) && !h.ts[i].After(cut) {
			i++
		}
		h.ts = h.ts[i:]
		if len(h.ts) >= w.limit {
			ok = false
			if d := h.ts[len(h.ts)-w.limit].Add(w.per).Sub(now); d > wait {
				wait = d
			}
		}
	}
	if !ok {
		return false, wait
	}
	for _, w := range ws {
		if w.limit > 0 {
			l.hits[w.key].ts = append(l.hits[w.key].ts, now)
		}
	}
	return true, 0
}
```
- [ ] `command.go` — full code:
```go
package homedash

const (
	dedupeTTL         = 30 * time.Second
	perSourceInFlight = 4
	burstLimit        = 10
	burstWindow       = 10 * time.Second
	entityLimit       = 5
	entityWindow      = 5 * time.Second
	maxRequestIDLen   = 64
)

type commandState struct {
	limiter rateLimiter
	mu      sync.Mutex
	dedupe  map[string]*dedupeEntry
	slots   map[string]*cmdSlot
	sems    map[SourceID]chan struct{}
}

type dedupeEntry struct {
	at   time.Time
	done chan struct{}
	res  CommandResult
	err  error
}

type cmdSlot struct{ next *queuedCmd }

type queuedCmd struct {
	cmd  Command
	done chan struct{}
	res  CommandResult
	err  error
}

func newCommandState() *commandState {
	return &commandState{dedupe: map[string]*dedupeEntry{}, slots: map[string]*cmdSlot{}, sems: map[SourceID]chan struct{}{}}
}

func (c *commandState) sem(src SourceID) chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sems[src]
	if s == nil {
		s = make(chan struct{}, perSourceInFlight)
		c.sems[src] = s
	}
	return s
}

func (c *commandState) claim(key string, now time.Time) (*dedupeEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.dedupe {
		if now.Sub(e.at) > dedupeTTL {
			delete(c.dedupe, k)
		}
	}
	if e, ok := c.dedupe[key]; ok {
		return e, false
	}
	e := &dedupeEntry{at: now, done: make(chan struct{})}
	c.dedupe[key] = e
	return e, true
}

func (e *dedupeEntry) wait(ctx context.Context) (CommandResult, error) {
	select {
	case <-e.done:
		return e.res, e.err
	case <-ctx.Done():
		return CommandResult{}, ctx.Err()
	}
}

// Command runs the pipeline of backend §5 (validate, gates, kiosk rules, confirmation, rate limits,
// request-id dedupe, in-flight control, Source.Command, audit). Agents have no command path.
func (h *Hub) Command(ctx context.Context, cmd Command) (CommandResult, error) {
	spec, err := h.admitCommand(ctx, &cmd)
	if err != nil {
		h.audit(ctx, cmd, err)
		return CommandResult{}, err
	}
	if cmd.RequestID != "" {
		key := string(cmd.Principal.Kind) + "|" + cmd.Principal.Key + "|" + cmd.RequestID
		entry, first := h.cmds.claim(key, h.opts.Now())
		if !first {
			return entry.wait(ctx)
		}
		entry.res, entry.err = h.dispatch(ctx, cmd, spec)
		close(entry.done)
		h.audit(ctx, cmd, entry.err)
		return entry.res, entry.err
	}
	res, err := h.dispatch(ctx, cmd, spec)
	h.audit(ctx, cmd, err)
	return res, err
}

func (h *Hub) audit(ctx context.Context, cmd Command, err error) {
	h.mu.Lock()
	fn := h.opts.Audit
	h.mu.Unlock()
	if fn == nil {
		return
	}
	outcome := "ok"
	var he *Error
	switch {
	case err == nil:
	case errors.As(err, &he) && he.Code != CodeCommandFailed && he.Code != CodeSourceError:
		outcome = "refused:" + he.Code
	default:
		outcome = "failed"
	}
	fn(ctx, cmd, outcome)
}

func findCap(caps []CapabilitySpec, c Capability) (CapabilitySpec, bool) {
	for _, s := range caps {
		if s.Name == c {
			return s, true
		}
	}
	return CapabilitySpec{}, false
}

func readOnlyErr(blockedBy string) *Error {
	return &Error{Code: CodeReadOnly, Message: "commands are switched off", BlockedBy: blockedBy}
}

func (h *Hub) admitCommand(ctx context.Context, cmd *Command) (CapabilitySpec, error) {
	src, native, err := ParseEntityID(string(cmd.Entity))
	if err != nil {
		return CapabilitySpec{}, ErrEntityNotFound
	}
	cmd.Native = native
	if cmd.Principal.Kind == PrincipalAgent {
		return CapabilitySpec{}, &Error{Code: CodeForbiddenEntity, Message: "agents send no device commands", BlockedBy: "agent"}
	}
	if len(cmd.RequestID) > maxRequestIDLen || ContainsControl(cmd.RequestID) {
		return CapabilitySpec{}, newError(CodeBadRequest, "invalid request_id")
	}
	h.mu.Lock()
	switch rt := h.rts[src]; {
	case h.closed:
		h.mu.Unlock()
		return CapabilitySpec{}, ErrHubClosed
	case !h.cfg.active():
		h.mu.Unlock()
		return CapabilitySpec{}, newError(CodeHomeDashDisabled, "HomeDash is disabled")
	case rt == nil || !rt.active:
		h.mu.Unlock()
		return CapabilitySpec{}, ErrEntityNotFound
	default:
		_, have := rt.catalog[cmd.Entity]
		job := catalogJob{rt.id, rt.src}
		h.mu.Unlock()
		if !have {
			h.fetchCatalogs(ctx, []catalogJob{job})
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	rt := h.rts[src]
	if rt == nil || !rt.active {
		return CapabilitySpec{}, ErrEntityNotFound
	}
	e, ok := rt.catalog[cmd.Entity]
	if !ok {
		return CapabilitySpec{}, ErrEntityNotFound
	}
	spec, ok := findCap(e.Capabilities, cmd.Name)
	info, known := LookupCapability(cmd.Name)
	if !ok || !known || !info.Command {
		return CapabilitySpec{}, newError(CodeUnsupportedCommand, "the entity does not offer this command")
	}
	if err := ValidateArgs(spec, cmd.Args); err != nil {
		return CapabilitySpec{}, err
	}
	cfg, set := h.cfg, h.cfg.Sources[src]
	switch {
	case !spec.Allowed:
		blocked := spec.BlockedBy
		if blocked == "" {
			blocked = "integration"
		}
		return CapabilitySpec{}, readOnlyErr(blocked)
	case cfg.ReadOnly:
		return CapabilitySpec{}, readOnlyErr("homedash.readonly")
	case cfg.DesktopReadOnly:
		return CapabilitySpec{}, readOnlyErr("virtual_desktop.readonly")
	case !set.Control && set.ConfigKey != "":
		return CapabilitySpec{}, readOnlyErr("homedash.sources." + set.ConfigKey + ".control")
	}
	if p := cmd.Principal; p.Kind == PrincipalKiosk {
		switch {
		case !p.Allowed.Allows(cmd.Entity, cmd.Name):
			return CapabilitySpec{}, newError(CodeForbiddenEntity, "this display may not send this command")
		case p.ViewOnly || !cfg.KioskAllowControl:
			return CapabilitySpec{}, newError(CodeViewOnly, "this display is view-only")
		case info.KioskDenied:
			return CapabilitySpec{}, &Error{Code: CodeForbiddenEntity, Message: "not allowed from a wall display", BlockedBy: "kiosk.denied"}
		}
	}
	if (spec.Sensitive || info.Sensitive) && !cmd.Confirmed {
		return CapabilitySpec{}, newError(CodeConfirmationRequired, "this command needs a confirmation")
	}
	pk := string(cmd.Principal.Kind) + ":" + cmd.Principal.Key
	if ok, wait := h.cmds.limiter.allow(h.opts.Now(), []window{
		{"pm:" + pk, cfg.Limits.CommandsPerMinute, time.Minute},
		{"pb:" + pk, burstLimit, burstWindow},
		{"e:" + string(cmd.Entity), entityLimit, entityWindow},
	}); !ok {
		return CapabilitySpec{}, &Error{Code: CodeRateLimited, Message: "too many commands", RetryAfterMS: max(1, wait.Milliseconds())}
	}
	return spec, nil
}

// dispatch keeps one queued command per (entity, capability) for continuous controls: a newer
// value replaces a waiting one (the replaced caller gets Superseded) and the queue drains after the
// running command, on a hub goroutine.
func (h *Hub) dispatch(ctx context.Context, cmd Command, spec CapabilitySpec) (CommandResult, error) {
	info, _ := LookupCapability(cmd.Name)
	if !info.Continuous {
		return h.execute(ctx, cmd)
	}
	key := string(cmd.Entity) + "|" + string(cmd.Name)
	c := h.cmds
	c.mu.Lock()
	if slot, busy := c.slots[key]; busy {
		q := &queuedCmd{cmd: cmd, done: make(chan struct{})}
		if prev := slot.next; prev != nil {
			prev.res = CommandResult{Superseded: true}
			close(prev.done)
		}
		slot.next = q
		c.mu.Unlock()
		select {
		case <-q.done:
			return q.res, q.err
		case <-ctx.Done():
			return CommandResult{}, ctx.Err()
		}
	}
	slot := &cmdSlot{}
	c.slots[key] = slot
	c.mu.Unlock()
	res, err := h.execute(ctx, cmd)
	h.releaseSlot(key, slot)
	return res, err
}

func (h *Hub) releaseSlot(key string, slot *cmdSlot) {
	c := h.cmds
	c.mu.Lock()
	q := slot.next
	slot.next = nil
	if q == nil {
		delete(c.slots, key)
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		q.res, q.err = h.execute(h.ctx, q.cmd)
		close(q.done)
		h.releaseSlot(key, slot)
	}()
}

func commandGuarded(ctx context.Context, src Source, cmd Command) (res CommandResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("command panic: %v", r)
		}
	}()
	return src.Command(ctx, cmd)
}

// execute counts as interest while it runs, holds one of 4 per-source slots and never reports raw
// upstream text.
func (h *Hub) execute(ctx context.Context, cmd Command) (CommandResult, error) {
	srcID, _, _ := ParseEntityID(string(cmd.Entity))
	h.mu.Lock()
	rt := h.rts[srcID]
	if rt == nil || !rt.active || rt.src == nil {
		h.mu.Unlock()
		return CommandResult{}, newError(CodeSourceUnavailable, "the source is not available")
	}
	source := rt.src
	rt.inflight++
	note := h.reconcileLocked(rt)
	h.mu.Unlock()
	h.deliver([]*interestNote{note})
	defer func() {
		h.mu.Lock()
		if rt.inflight > 0 {
			rt.inflight--
		}
		note := h.reconcileLocked(rt)
		h.mu.Unlock()
		h.deliver([]*interestNote{note})
	}()
	sem := h.cmds.sem(srcID)
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return CommandResult{}, ctx.Err()
	}
	defer func() { <-sem }()
	cctx, cancel := context.WithTimeout(ctx, h.opts.CommandTimeout)
	defer cancel()
	res, err := commandGuarded(cctx, source, cmd)
	if err != nil {
		var he *Error
		if errors.As(err, &he) {
			return CommandResult{}, he
		}
		h.log.Warn("homedash: command failed", "source", srcID, "command", cmd.Name, "error", err)
		return CommandResult{}, newError(CodeCommandFailed, "the device did not accept the command")
	}
	if res.Expected != nil {
		st := sanitizeState(*res.Expected, h.opts.Now())
		res.Expected = &st
	}
	return res, nil
}
```
- [ ] `history.go` — full code for the ring; rules for `History`/`Snapshot`:
```go
package homedash

type ring struct {
	buf     []Sample
	head, n int
	lastUse time.Time
}

func (r *ring) last() *Sample {
	if r.n == 0 {
		return nil
	}
	return &r.buf[(r.head+r.n-1)%len(r.buf)]
}

func (r *ring) push(s Sample) {
	if r.n < len(r.buf) {
		r.buf[(r.head+r.n)%len(r.buf)] = s
		r.n++
		return
	}
	r.buf[r.head] = s
	r.head = (r.head + 1) % len(r.buf)
}

// historyStore keeps in-memory numeric rings for subscribed entities: one point per bucket (last
// value wins), points per ring, at most maxEntities rings (least recently used evicted). Nothing persists.
type historyStore struct {
	mu          sync.Mutex
	points      int
	maxEntities int
	bucket      time.Duration
	rings       map[EntityID]*ring
}

func newHistoryStore(points, entities int, bucket time.Duration) *historyStore {
	return &historyStore{points: points, maxEntities: entities, bucket: bucket, rings: map[EntityID]*ring{}}
}

func (s *historyStore) touch(id EntityID, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.rings[id]; r != nil {
		r.lastUse = now
		return
	}
	for len(s.rings) >= s.maxEntities {
		var oldest EntityID
		var at time.Time
		for k, r := range s.rings {
			if oldest == "" || r.lastUse.Before(at) {
				oldest, at = k, r.lastUse
			}
		}
		delete(s.rings, oldest)
	}
	s.rings[id] = &ring{buf: make([]Sample, s.points), lastUse: now}
}

func (s *historyStore) observe(id EntityID, v float64, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rings[id]
	if r == nil {
		return
	}
	r.lastUse = at
	if l := r.last(); l != nil && l.T.Truncate(s.bucket).Equal(at.Truncate(s.bucket)) {
		*l = Sample{T: at, V: v}
		return
	}
	r.push(Sample{T: at, V: v})
}

func (s *historyStore) resize(points, entities int, bucket time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if points == s.points && entities == s.maxEntities && bucket == s.bucket {
		return
	}
	s.points, s.maxEntities, s.bucket, s.rings = points, entities, bucket, map[EntityID]*ring{}
}

func (s *historyStore) series(id EntityID, from, to time.Time, points int) []Sample {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rings[id]
	if r == nil {
		return nil
	}
	out := []Sample{}
	for i := 0; i < r.n; i++ {
		smp := r.buf[(r.head+i)%len(r.buf)]
		if !smp.T.Before(from) && !smp.T.After(to) {
			out = append(out, smp)
		}
	}
	return downsample(out, points)
}

// downsample averages consecutive groups so at most points samples remain (T = last of the group).
func downsample(in []Sample, points int) []Sample {
	if points <= 0 || len(in) <= points {
		return in
	}
	stride := (len(in) + points - 1) / points
	out := make([]Sample, 0, points)
	for i := 0; i < len(in); i += stride {
		end := min(i+stride, len(in))
		sum := 0.0
		for _, s := range in[i:end] {
			sum += s.V
		}
		out = append(out, Sample{T: in[end-1].T, V: sum / float64(end-i)})
	}
	return out
}
```
  `Series` and the hub methods (also in `history.go`):
```go
type Series struct {
	Origin string    `json:"origin"` // "source" | "memory"
	Since  time.Time `json:"since,omitempty"`
	Points []Sample  `json:"points"`
}

const (
	MaxHistoryRange  = 7 * 24 * time.Hour
	MaxHistoryPoints = 500
	MaxSnapshotBytes = 8 << 20
)

func (h *Hub) History(ctx context.Context, id EntityID, from, to time.Time, points int) (Series, error)
func (h *Hub) Snapshot(ctx context.Context, id EntityID, w, hgt int) ([]byte, string, error)
```
  Rules: (1) `History`: `to` not after `from`, range > 7 d, or points outside 1..500 → `bad_request`; resolve the entity with `h.Entity` (errors pass through); when the active source implements `HistorySource`, call it with `CatalogTimeout` outside the lock, map errors to `source_error` (log the raw error), return `Series{Origin: "source", Points: downsample(pts, points)}`; otherwise `Series{Origin: "memory", Since: h.startedAt, Points: h.rec.series(...)}` with `[]Sample{}` instead of nil. (2) `Snapshot`: `h.Entity` must list `CapSnapshot` (else `unsupported_command`) and the source must implement `SnapshotSource` (else `unsupported_command`); width outside 1..1920 → 640, height outside 1..1080 → 360; call with `CommandTimeout`; errors or more than `MaxSnapshotBytes` → `source_error`; never stored.
- [ ] `principal.go`: append
```go
// BuildAllowedSet gives a kiosk display the entities bound on doc (the approved revision) and the
// commands its widgets can issue (catalog Commands); bindings without commands are read only.
func BuildAllowedSet(doc Dashboard, cat WidgetCatalog) *AllowedSet {
	set := &AllowedSet{Commands: map[EntityID]map[Capability]bool{}}
	for _, p := range doc.Pages {
		for _, w := range p.Widgets {
			wt, typed := cat.Type(w.Type)
			for name, raw := range w.Bindings {
				if _, _, err := ParseEntityID(raw); err != nil {
					continue
				}
				id := EntityID(raw)
				m := set.Commands[id]
				if m == nil {
					m = map[Capability]bool{}
					set.Commands[id] = m
				}
				if typed {
					for _, c := range wt.Commands[name] {
						m[c] = true
					}
				}
			}
		}
	}
	return set
}
```
  Add to `dashboard_test.go` a short `TestBuildAllowedSet`: `validDoc()` gives `ha:switch.lamp` → `{toggle, turn_on, turn_off}` and `system:cpu` → empty map; `Allows("system:cpu", CapToggle)` is false; a nil `*AllowedSet` allows nothing.
- [ ] `hub.go` edits (B4a file; describe them in the report): add field `cmds *commandState` to `Hub`; in `NewHub` set `cmds: newCommandState()` and `rec: newHistoryStore(360, 500, time.Minute)` in the struct literal.
- [ ] Run `go test ./internal/homedash/... -count=1 -v` → PASS `TestCommandPipelineGates` (15 subtests), `TestCommandRateLimitAndRequestIDDedupe`, `TestCommandLatestWinsSlot`, `TestRateLimiterWindowsAreAtomic`, `TestHistoryRingDownsamplesAndEvictsLRU`, `TestBuildAllowedSet` and all earlier tests. `go vet ./internal/homedash/...`.
- [ ] Race, once (test-agent): Windows has no gcc. On aurago-test (memory reference_aurago_test_server, dockerwatch `AGENTS.md:37`): from the worktree `git archive --format=tar HEAD -- go.mod go.sum internal/homedash internal/security internal/dbutil internal/fileutil internal/httporigin internal/testutil | gzip | ssh aurago-test 'rm -rf ~/race-hd && mkdir -p ~/race-hd && tar -xz -C ~/race-hd'` (if the build reports a missing package, add its directory from `go list -deps -test ./internal/homedash/... | Select-String '^aurago/'` and resend), then `ssh aurago-test 'cd ~/race-hd && GOTOOLCHAIN=go1.27.4 CGO_ENABLED=1 PATH=/usr/local/go/bin:$PATH go test -race -count=1 ./internal/homedash/ -run "(?i)(Hub|Command)"'`. If the server is unreachable, report "race not run" to the lead; do not block.
- [ ] Commit `git add internal/homedash`; `feat(homedash): command pipeline, rate limits, kiosk allowed set and history ring`.
- **Done when:** all homedash tests pass; race run clean or explicitly reported as not run.

### Task 9: Documentation (docs-agent)

**Files:** Create `internal/homedash/AGENTS.md`. Modify root `AGENTS.md` (Child DOX Index, after the Chibi Sensei row, line 247) and `internal/tools/AGENTS.md:25-30`.
**Read first:** root `AGENTS.md:5-12` (DOX section order), this plan, backend §3–§5, §7, §9.

- [ ] `internal/homedash/AGENTS.md` in the order Purpose → Ownership → Local Contracts → Work Guidance → Verification → Child DOX Index:
  - Purpose: core of the HomeDash app (entity hub, widget catalog, document schema, `homedash.db`), plus `sources/hass`.
  - Ownership: imports only stdlib, `internal/security`, `internal/dbutil`, `modernc.org/sqlite`; never `server`, `agent`, `tools` or integrations (adapters under `sources/` may import their own integration). Server wiring, HTTP and the config UI are later milestones; the design is `docs/superpowers/specs/2026-11-23-homedash-design.md`.
  - Local Contracts: entity id grammar and query-string-only ids; hub lock order and "no source call under `Hub.mu` except factories"; sinks never block, latest-wins coalescing, memory bounded by subscription size; idle stop 60 s; command order (validate → integration/hub gates → kiosk allowed set → view-only → kiosk-denied → confirmation → rate limits → request-id dedupe → 4 in flight per source → latest-wins slot → audit); agents have no command or stream path; a kiosk principal with nil `Allowed` gets nothing; the kiosk `AllowedSet` comes from the approved revision × catalog `Commands`; agent saves never move `approved_revision`; revisions keep the last 20 plus the approved one; kiosk codes and tokens are stored only as SHA-256; documents carry no URLs; the catalog is Go plus the golden `testdata/widget_catalog.json` read by the frontend contract test; `hass` holds the only HA websocket client and uses the HA REST client only through `tools.HARequestJSON`/`HAGetBytes`.
  - Work Guidance: hub tests use `sourcetest.FakeSource` and short real timers; change the catalog only together with the golden file (`go test ./internal/homedash -run TestWidgetCatalogGolden -update`) and tell the frontend owners; new SQLite schema versions back up through `dbutil.BackupSQLiteForMigration` first.
  - Verification: `go test ./internal/homedash/... -count=1`; race on Linux as in Task 8; `go test ./internal/config/ -run '(?i)(HomeDash|SQLite)'`; `go test ./internal/audit/ -run TestSQLiteOpensAreBackedUpOrExcluded`.
  - Child DOX Index: none.
- [ ] Root `AGENTS.md` row: `| HomeDash | \`internal/homedash/AGENTS.md\`, \`internal/server/AGENTS.md\` | Dashboards for devices and sensors from every source: entity hub, widget catalog with golden JSON, document schema, \`homedash.db\`, Home Assistant mapping and websocket client; config \`homedash\`; design \`docs/superpowers/specs/2026-11-23-homedash-design.md\`. |`
- [ ] `internal/tools/AGENTS.md` HA bullet: append "`HARequestJSON`/`HAGetBytes` (`homeassistant_raw.go`) are thin wrappers over `haDo`, the single HA HTTP path; they report non-2xx answers as `HAHTTPError` without the body."
- [ ] Commit: `git add -f internal/homedash/AGENTS.md`, `git add AGENTS.md internal/tools/AGENTS.md`, confirm `git ls-files internal/homedash/AGENTS.md`; message `docs(homedash): child guide and index rows for the M0 backend`.
- **Done when:** the guide is tracked and the root index links it.

### Task 10: Final verification (test-agent, then lead)

- [ ] `go build ./...`; `go vet ./internal/homedash/... ./internal/config/ ./internal/tools/`.
- [ ] `go test ./internal/homedash/... ./internal/config/... ./internal/audit/... ./cmd/config-merger/... -count=1`.
- [ ] `go test ./internal/tools/ -run '(?i)(TestHA)' -count=1`; `go test ./internal/server/ -run '(?i)(HomeDashConfig|ChibiSenseiConfig|ConfigSave)' -count=1`; `go test ./ui/ -run TestConfigDerivedValidationRulesCoverTemplate -count=1`.
- [ ] Compare against the Task 0 baseline: no new failures. `git status --short --ignored` in the worktree: every `!!` line besides caches is explained (memory reference_agents_md_gitignored).
- **Done when:** all green; the branch holds nine commits (Tasks 1–9) and no user-visible change.

## Batches and reviews

Same-package work runs in sequence (a parallel maker's half-written file breaks `go test` of the whole package). Three lanes: config (B0), core (`internal/homedash`: B1 → B2 → B4a → B3 → B4b), HA (`internal/tools` + `sources/hass`: B8 → B9a).

| Batch | Parallel tasks | Review (reader-agent, once per batch: Critical/Important only) |
|---|---|---|
| 1 | Task 1 (B0) ∥ Task 2 (B1) | Config: template = loader defaults, three Normalize call sites, error format `homedash: <key>`. Core: id grammar, `ValidateArgs` against backend §6, sanitize drops bidi/control, stdlib-only imports. |
| 2 | Task 3 (B2) ∥ Task 4 (B8) | Catalog against main §4.3 row by row (sizes, 24/47), commands vs backend §8.1, options minimal; validation rules vs backend §7; patch does not bypass validation. HA: one HTTP path, no body text in errors, policy-derived `blocked_by`, attribute denylist. **security-agent**: B8 (`haDo` refactor, attribute allowlist). |
| 3 | Task 5 (B4a) ∥ Task 6 (B9a) | Hub locking (no source call under `Hub.mu` except factories, notes delivered after unlock), generation guards, bounded subscriber memory, stale sinks ignored. WS: single writer, auth_invalid retry interval, token never in headers or logs, read limit 16 MiB. |
| 4 | Task 7 (B3) then Task 8 (B4b) | Store transactions, approved revision never pruned, uniform pairing errors, hashes only. Pipeline order vs backend §5, kiosk nil-`Allowed`, agent refusal, latest-wins correctness, limiter atomicity; race result. **security-agent**: B3 kiosk store and B4b gates (one combined review). |
| 5 | Task 9, Task 10 | Lead checks the batch-5 diff only. |

Shared worktree discipline for parallel lanes: each maker stages only its own paths and commits with `git commit -m ... -- <paths>`; if `.git/worktrees/homedash-m0-backend/index.lock` exists, wait and retry (never delete it). The test-agent runs the named commands after each task; the lead runs Task 10 and decides on landing (rebase on `main`, then `merge --ff-only`, per memory project_concurrent_sessions). Fixes from a review are made by the maker; the lead checks the diff and tests without a second review round.

## Self-review

**Spec coverage**

| Scope item | Task |
|---|---|
| B0 config block (backend §11, main §4.5 defaults) | 1 |
| B1 ids, entity, capabilities, sanitize, source interfaces, RuntimeConfig (§4) | 2 |
| B8 `HARequestJSON`/`HAGetBytes`, domain table, attribute allowlist (§8.1) | 4 |
| B2 catalog (main §4.3), document (main §4.2), validate, autopack, migrate, patch (§7) | 3 |
| B3 store, `homedash.db`, archive list (§7 store) | 7 |
| B4a hub core, subscribers, coalescing, reconfigure, idle stop, `sourcetest` (§5) | 5 |
| B4b commands, limiter, principal `BuildAllowedSet`, history ring (§5) | 8 |
| B9a HA websocket client against a fake server (§8.1) | 6 |
| Golden catalog JSON | 3 |
| Test plan §12 (core, config, hass items in scope) | 1–8 (`TestHAAreasFromEntityThenDeviceRegistry`, REST fallback, history, camera are B9b) |

**Decisions and deviations (with evidence)**
1. Batching differs from backend §13 ("B0 ∥ B1 ∥ B8", "B2 ∥ B4a ∥ B9a", "B3, B4b"): B8's mapping uses `homedash.Kind`/`Capability`, so it needs B1; B2/B4a/B3/B4b all live in package `homedash` and B8/B9a share `hass`, so they cannot be edited in parallel without breaking each other's compile.
2. Main §4.3 first gave switch/tile as 1×1 / 2×1 while visual §4 says 2×2. The lead corrected the main document on 2026-11-23: switch/tile is 1×1 / 2×2, and the catalog table above uses that.
3. Backend §8.1 lists humidifier `set_humidity`/`set_mode` and vacuum commands; main §10 defers them. Main wins: humidifiers toggle only, vacuums are read-only. The registry keeps those capability names for later.
4. Catalog struct changed from backend §7: variants carry min and default sizes, the type carries the max; added `schema_version`, `grid`, `limits`, `palette_tokens`, `common_options`, `family_alias`, `OptionSpec.Required` (main §4.3 asks the catalog to publish grid constants and aliases).
5. `palette.custom` is a nested object (backend §7 says "custom holds the inputs"); `page.compact`, `kiosk.show_clock` are pointers and `kiosk` is optional so absent values default correctly.
6. Snapshot token list (39 names) derived from visual §1.1; the design names only "a fixed list".
7. Backend §4 says a nil `Allowed` means unrestricted. For kiosk principals this plan treats nil as "nothing" (fail closed). Agents get no stream and no command (backend §2.8), enforced in the hub too.
8. "Keep the last 20 revisions" would delete the approved revision after 20 agent saves, leaving displays without a document; the prune keeps the approved revision.
9. Added `CommandResult.Superseded`, `Batch.Snapshot`, `SourceSettings.ConfigKey/Params/Lists`, `Options.CatalogTimeout/Backoff`, codes `kiosk_disabled`, `slug_taken`, `revision_not_found`, `invalid_patch`, `hub_closed`.
10. B0 touches two `internal/server` files: these are the config-save Normalize call sites named in backend §11, not server wiring.
11. Dedupe runs after the rate limit, as backend §5 orders it; a retried request id consumes rate budget.

**Open risks**
- HA websocket message shapes come from HA's public API, not this repo; the fake server encodes the plan's assumptions (backend §14). Check once against aurago-test in B9b.
- GitNexus CLI had no graph on 2026-11-23 (index version mismatch); commits will mostly carry the grep fallback note.
- `-race` needs aurago-test (no gcc locally); if unreachable the race run is deferred and reported.
- `TestCommandLatestWinsSlot` orders two goroutines with a 30 ms sleep; it could flake on a loaded machine. If it does, add a test-only hook that reports the queued slot rather than lengthening the sleep.
- The relist timer can overlap a registry list still in flight; worst case one extra `OnRegistries` with partial lists, corrected by the next list.
- B9a passes the audit network-client tripwire only because it builds no `http.Client`; B9b still adds the `NetworkClientInventory` entry the design asks for.
