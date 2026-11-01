# WhatsApp Foundation (Stage 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stage 1 ships the WhatsApp foundation:
- the whatsmeow dependency and its license docs
- the `whatsapp:` config section
- a private device store
- the client adapter
- event and status mapping
- the single per-process `Bot`
- QR and pairing-code link sessions
- admin endpoints
- the Config UI link panel
- an owner-only test message

Inbound messages are only counted.

**Architecture:** `internal/whatsapp` keeps whatsmeow behind the unexported interfaces `waClient`, `deviceStore` and `backend`, so unit tests use fakes. `Start` returns one `*Bot` generation; it opens the store and connects with backoff. `StartLink` runs one link session against an unlinked store. The server holds both behind `whatsAppLifecycleMu` and drives them from admin handlers through a small `whatsAppController` interface that handler tests replace with a fake.
**Tech Stack:** Go 1.27.3, CGO-free. whatsmeow `v0.0.0-20261030195514-56e5ad0581ce`. modernc SQLite v1.60.1. `internal/qrcode`. Vanilla JS. rod.

Master plan with the standing rules: `plans/2026-10-31-whatsapp-integration-plan.md`. They apply to every task below (GitNexus impact and detect-changes, private-index commits, agent roster, lean tests).

---

## Interface summary (defined in this plan; keep names exact)

- `internal/config` (Task 1):
  - Type `WhatsAppConfig{Enabled, Mode, OwnerNumbers, ReadOnly, SendReadReceipts, SelfChatPrefix}`.
  - Constants `WhatsAppModeDedicated`, `WhatsAppModeSelfChat`, `WhatsAppDirName`, `WhatsAppDeviceDBFilename`, `DefaultWhatsAppSelfChatPrefix`.
  - Functions:
    - `ApplyWhatsAppDefaults(*WhatsAppConfig)`
    - `NormalizeWhatsAppConfig(*WhatsAppConfig)`
    - `ValidateWhatsAppConfig(WhatsAppConfig) error`
    - `NormalizeE164(string) (string, error)`
    - `(WhatsAppConfig) Equal(WhatsAppConfig) bool`
    - `WhatsAppStoreDir(dataDir string) string`
    - `WhatsAppDeviceDBPath(dataDir string) string`
    - `WhatsAppProtectedPaths(*Config) []string`
- `internal/integrationstatus` (Task 2):
  - Constants `WhatsAppState{Disabled,NotConfigured,NotLinked,Linking,Connecting,Healthy,Degraded,LoggedOut,Conflict,TempBanned,Outdated}`.
  - Type `WhatsAppRuntimeStatus`.
  - Functions:
    - `SetWhatsAppConfigState(state, mode string)`
    - `SetWhatsAppState(state string)`
    - `SetWhatsAppLinked(linked bool, masked string)`
    - `MarkWhatsAppConnected(time.Time)`
    - `MarkWhatsAppError(state, code string)`
    - `MarkWhatsAppTempBan(code string, expires time.Time)`
    - `CountWhatsAppMessage(time.Time)`
    - `MarkWhatsAppOutbound(time.Time)`
    - `MarkWhatsAppDeliveryFailure()`
    - `MarkWhatsAppTest(time.Time)`
    - `WhatsAppStatus() WhatsAppRuntimeStatus`
- `internal/whatsapp`:
  - Task 3:
    - `openDeviceStore(ctx, dataDir string, log waLog.Logger) (*sqliteDeviceStore, error)`
    - `(*sqliteDeviceStore)` methods `Linked() bool`, `OwnJID() types.JID`, `DeleteLocal(ctx) error`, `Close() error`
    - `sqliteURI(path, query string) string`
    - `newWALogger(*slog.Logger, module string) waLog.Logger`
    - `maskDigits(string) string`
    - `maskJID(types.JID) string`
  - Task 5:
    - Errors `ErrNotConfigured`, `ErrAlreadyLinked`, `ErrLinkInProgress`, `ErrInvalidLinkMethod`, `ErrInvalidPhone`, `ErrNotConnected`, `ErrNoRecipient`.
    - Interfaces `waClient`, `deviceStore`, `backend`.
    - `sqliteBackend{log waLog.Logger}`
    - `(*sqliteDeviceStore) NewClient() waClient`
    - `whatsmeowClient`
    - `configureDeviceProps()`
    - Test fakes `fakeClient`, `fakeStore`, `fakeBackend`, `waitFor`.
  - Task 6:
    - Type `eventEffect`.
    - `classifyEvent(evt any, synced bool) eventEffect`
  - Task 7:
    - `type Bot`
    - `Start(parent context.Context, cfg *config.Config, logger *slog.Logger) *Bot`
    - `ConfigState(*config.Config) string`
    - `(*Bot)` methods `MatchesConfig`, `CancelIfConfigChanged`, `Stop`, `Logout(ctx) (LogoutResult, error)`, `SendTestMessage(ctx, now time.Time) error`
    - `LogoutResult{RemoteRemoved bool}`
    - `runtimeDeps{backend, sleep, now}`
    - `start(...)`
    - `ownerRecipients(config.WhatsAppConfig, types.JID) ([]types.JID, error)`
    - `testMessageText(config.WhatsAppConfig, time.Time) string`
    - `minBackoff`, `maxBackoff`, `nextBackoff`
    - `removeLocalDevice(ctx, backend, dataDir) error`
  - Task 8:
    - `LinkRequest{Method, Phone}`
    - `LinkSnapshot{State, QRPNG, PairingCode, ExpiresAt, Error}`
    - Constants `LinkMethodQR`, `LinkMethodPhone`, `LinkState{Idle,Waiting,QR,Code,Success,Timeout,Error}`, `linkTTL`
    - `type LinkSession` with `Snapshot()`, `Active()`, `Cancel()`, `Done()`
    - `StartLink(parent, cfg, req, logger, onFinish func(linked bool)) (*LinkSession, error)`
    - `DeviceLinked(ctx, dataDir) (bool, error)`
    - `RemoveLocalDevice(ctx, dataDir) error`
    - `renderQRDataURI(ctx, payload) (string, error)`
    - `linkDeps{backend, now, render}`
- `internal/server`:
  - Task 9:
    - Fields `whatsAppLifecycleMu`, `whatsAppBot atomic.Pointer[whatsapp.Bot]`, `whatsAppLink *whatsapp.LinkSession`, `whatsAppClosed`, `whatsAppCtl whatsAppController`.
    - Methods `configureWhatsAppBot()`, `configureWhatsAppBotLocked(force bool)`, `stopWhatsApp()`, `afterWhatsAppLink(linked bool)`.
    - `whatsAppController{Link(ctx, whatsapp.LinkRequest, force bool) (whatsapp.LinkSnapshot, error); LinkSnapshot() whatsapp.LinkSnapshot; Logout(ctx) (whatsapp.LogoutResult, error); SendTest(ctx, time.Time) error}`
    - `serverWhatsApp{s}`
    - `(s) whatsApp() whatsAppController`
  - Task 10:
    - `registerWhatsAppRoutes(mux, s)`
    - `handleWhatsAppStatus`, `handleWhatsAppLink`, `handleWhatsAppLogout`, `handleWhatsAppTest`
    - `whatsAppTestMu`, `whatsAppTestLast`, `whatsAppTestNow`
    - `writeWhatsAppJSON(w, code, v)`
    - `whatsAppErrorStatus(err) (int, string)`

## Task overview

| # | Task | Agents |
| --- | --- | --- |
| 1 | Config section, defaults, E.164, template, backup exclusion | maker → test |
| 2 | Integration status | maker → test |
| 3 | Dependency, device store, log adapter, masking | maker → test |
| 4 | License documentation and guard test | docs → test |
| 5 | Client interface, whatsmeow adapter, fakes | maker → test |
| 6 | Event → state mapping | maker → test |
| 7 | Bot runtime, recipients, test message, logout | maker → test |
| 8 | Link sessions and QR rendering | maker → test |
| 9 | Server lifecycle wiring and config save/reload | maker → test |
| 10 | Admin endpoints, route manifest, network inventory | maker → test |
| 11 | Security: file-tool denial, backup exclusion | maker → test → security review |
| 12 | Translations (16 locales) and config help | docs → test |
| 13 | Config UI section, link panel, sidebar icon, browser test | maker → test |
| 14 | DOX guides | docs |
| 15 | Real-device link acceptance (user scans) | test with the user |

---

### Task 1: Config section, defaults, E.164 normalization

**Files:**
- Create: `internal/config/whatsapp.go`
- Create (test): `internal/config/whatsapp_test.go`
- Modify: `internal/config/config_types.go` — insert after the line `	} \`yaml:"telegram"\`` (≈1479)
- Modify: `internal/config/config.go` — in `Load`, after `	cfg.ModelCatalog.CatalogOnlyVisible = true` (≈418), and after the `// Telegram defaults` if-block (≈2367-2370)
- Modify: `config_template.yaml` — before the top-level `whisper:` line (≈1436)
- Modify (test): `internal/config/template_integrations_test.go` — `TestIntegrationTemplateSectionsUseSupportedFieldsAndSafeDefaults`
- Modify (test): `internal/audit/sqlite_backup_coverage_test.go` — `sqliteBackupExclusions`

- [ ] **Step 1: Impact analysis**

```bash
node .gitnexus/run.cjs impact "Load" --direction upstream --repo .
node .gitnexus/run.cjs impact "TestIntegrationTemplateSectionsUseSupportedFieldsAndSafeDefaults" --direction upstream --repo .
```
Expected: `Load` is HIGH or CRITICAL (every component loads config). Report it. The change is additive: one defaults call and one normalize call.

- [ ] **Step 2: Write the failing test** `internal/config/whatsapp_test.go`

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func loadWhatsAppYAML(t *testing.T, content string) *Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestWhatsAppLoadDefaultsAndNormalization(t *testing.T) {
	w := loadWhatsAppYAML(t, "{}\n").WhatsApp
	if w.Enabled || w.ReadOnly || w.Mode != WhatsAppModeDedicated || !w.SendReadReceipts ||
		w.SelfChatPrefix != DefaultWhatsAppSelfChatPrefix || w.OwnerNumbers != nil {
		t.Fatalf("defaults = %+v", w)
	}
	w = loadWhatsAppYAML(t, "whatsapp:\n  enabled: true\n  mode: SELF_CHAT\n  send_read_receipts: false\n  self_chat_prefix: \"\"\n"+
		"  owner_numbers: [\"+49 170 123-4567\", \"0049 (170) 1234567\", \"12345\", \"\", \"+43 660 1234567\"]\n").WhatsApp
	if !w.Enabled || w.Mode != WhatsAppModeSelfChat || w.SendReadReceipts || w.SelfChatPrefix != "" {
		t.Fatalf("explicit values lost: %+v", w)
	}
	if got := strings.Join(w.OwnerNumbers, ","); got != "+491701234567,+436601234567" {
		t.Fatalf("owner numbers = %q", got)
	}
	if mode := loadWhatsAppYAML(t, "whatsapp:\n  mode: bogus\n").WhatsApp.Mode; mode != WhatsAppModeDedicated {
		t.Fatalf("unknown mode loaded as %q, want dedicated", mode)
	}
}

func TestNormalizeE164(t *testing.T) {
	for raw, want := range map[string]string{
		"+491701234567":            "+491701234567",
		" +49 (170) 123-45.67 ":    "+491701234567",
		"0043 660 1234567":         "+436601234567",
	} {
		if got, err := NormalizeE164(raw); err != nil || got != want {
			t.Errorf("NormalizeE164(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "01701234567", "+0491701234567", "+49170x1234567", "+123456", "+1234567890123456"} {
		if got, err := NormalizeE164(raw); err == nil {
			t.Errorf("NormalizeE164(%q) = %q, want an error", raw, got)
		}
	}
}

func TestValidateWhatsAppConfig(t *testing.T) {
	valid := WhatsAppConfig{Mode: "self_chat", OwnerNumbers: []string{"+49 170 1234567", ""}, SelfChatPrefix: DefaultWhatsAppSelfChatPrefix}
	if err := ValidateWhatsAppConfig(valid); err != nil {
		t.Fatalf("valid section rejected: %v", err)
	}
	var tooMany []string
	for i := 0; i < 11; i++ {
		tooMany = append(tooMany, fmt.Sprintf("+4917000000%02d", i))
	}
	for name, c := range map[string]WhatsAppConfig{
		"mode":     {Mode: "group"},
		"number":   {OwnerNumbers: []string{"+491701234567", "01701234567"}},
		"too many": {OwnerNumbers: tooMany},
		"prefix":   {SelfChatPrefix: strings.Repeat("x", 17)},
	} {
		if ValidateWhatsAppConfig(c) == nil {
			t.Errorf("%s: invalid section accepted", name)
		}
	}
}

func TestWhatsAppStorePathsStayOutOfBackups(t *testing.T) {
	if WhatsAppDeviceDBPath("") != "" || WhatsAppProtectedPaths(&Config{}) != nil {
		t.Fatal("an empty data dir produced a store path")
	}
	cfg := &Config{}
	cfg.Directories.DataDir = filepath.Join("srv", "data")
	db := filepath.Join("srv", "data", "whatsapp", "device.db")
	if got := WhatsAppDeviceDBPath(cfg.Directories.DataDir); got != db {
		t.Fatalf("device db = %q, want %q", got, db)
	}
	if got, want := WhatsAppProtectedPaths(cfg), []string{db, db + "-wal", db + "-shm", db + "-journal"}; !slices.Equal(got, want) {
		t.Fatalf("protected paths = %v, want %v", got, want)
	}
	if slices.Contains(SQLiteDatabasePaths(cfg), db) {
		t.Fatal("the WhatsApp device store must never be backed up")
	}
}
```

- [ ] **Step 3: Run it and confirm it fails**

```bash
CGO_ENABLED=0 go test ./internal/config/ -run '(TestWhatsApp|TestNormalizeE164|TestValidateWhatsAppConfig)' -count=1
```
Expected: FAIL, build error `undefined: WhatsAppModeDedicated` (and the other new names).

- [ ] **Step 4: Implement** `internal/config/whatsapp.go`

```go
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"unicode/utf8"
)

// WhatsApp channel modes.
const (
	WhatsAppModeDedicated = "dedicated"
	WhatsAppModeSelfChat  = "self_chat"
)

// The linked-device keys live in <data_dir>/whatsapp/device.db. The store is
// never backed up (internal/audit sqliteBackupExclusions) and agent file tools
// never read it (internal/tools protectedSystemFilesFromConfig).
const (
	WhatsAppDirName          = "whatsapp"
	WhatsAppDeviceDBFilename = "device.db"
)

// DefaultWhatsAppSelfChatPrefix marks AuraGo's messages in the self chat.
const DefaultWhatsAppSelfChatPrefix = "🤖 "

const (
	maxWhatsAppOwnerNumbers = 10
	maxWhatsAppPrefixRunes  = 16
	minE164Digits           = 7
	maxE164Digits           = 15
)

// WhatsAppConfig controls the WhatsApp channel. No field is secret: the linked
// device in the store is the credential.
type WhatsAppConfig struct {
	Enabled          bool     `yaml:"enabled" json:"enabled"`
	Mode             string   `yaml:"mode" json:"mode"`
	OwnerNumbers     []string `yaml:"owner_numbers" json:"owner_numbers"`
	ReadOnly         bool     `yaml:"readonly" json:"readonly"`
	SendReadReceipts bool     `yaml:"send_read_receipts" json:"send_read_receipts"`
	SelfChatPrefix   string   `yaml:"self_chat_prefix" json:"self_chat_prefix"`
}

var errInvalidE164 = errors.New("not an international phone number")

// Equal reports whether two loaded sections are identical. Normalization
// turns empty owner lists into nil, so loaded sections compare reliably.
func (c WhatsAppConfig) Equal(other WhatsAppConfig) bool {
	return reflect.DeepEqual(c, other)
}

// ApplyWhatsAppDefaults runs before YAML is decoded: absent keys keep the
// defaults, explicit values (including false and "") win.
func ApplyWhatsAppDefaults(c *WhatsAppConfig) {
	if c == nil {
		return
	}
	c.Mode = WhatsAppModeDedicated
	c.SendReadReceipts = true
	c.SelfChatPrefix = DefaultWhatsAppSelfChatPrefix
}

// NormalizeWhatsAppConfig canonicalizes a loaded section without failing
// startup. An unknown mode becomes dedicated. Owner numbers become unique
// E.164 values, and invalid ones are dropped with a warning that carries no
// number. The config save path rejects them first.
func NormalizeWhatsAppConfig(c *WhatsAppConfig) {
	if c == nil {
		return
	}
	c.Mode = strings.ToLower(strings.TrimSpace(c.Mode))
	if c.Mode != WhatsAppModeSelfChat {
		c.Mode = WhatsAppModeDedicated
	}
	var owners []string
	seen := make(map[string]bool, len(c.OwnerNumbers))
	dropped := 0
	for _, raw := range c.OwnerNumbers {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		number, err := NormalizeE164(raw)
		if err != nil {
			dropped++
			continue
		}
		if seen[number] || len(owners) == maxWhatsAppOwnerNumbers {
			continue
		}
		seen[number] = true
		owners = append(owners, number)
	}
	if dropped > 0 {
		slog.Warn("[Config] Ignored invalid whatsapp.owner_numbers entries", "count", dropped)
	}
	c.OwnerNumbers = owners
	if utf8.RuneCountInString(c.SelfChatPrefix) > maxWhatsAppPrefixRunes {
		c.SelfChatPrefix = string([]rune(c.SelfChatPrefix)[:maxWhatsAppPrefixRunes])
	}
}

// ValidateWhatsAppConfig rejects values the save path must not persist.
func ValidateWhatsAppConfig(c WhatsAppConfig) error {
	switch strings.ToLower(strings.TrimSpace(c.Mode)) {
	case "", WhatsAppModeDedicated, WhatsAppModeSelfChat:
	default:
		return errors.New("whatsapp.mode must be dedicated or self_chat")
	}
	if len(c.OwnerNumbers) > maxWhatsAppOwnerNumbers {
		return fmt.Errorf("whatsapp.owner_numbers allows at most %d numbers", maxWhatsAppOwnerNumbers)
	}
	for i, raw := range c.OwnerNumbers {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if _, err := NormalizeE164(raw); err != nil {
			return fmt.Errorf("whatsapp.owner_numbers[%d] must be an international number such as +491701234567", i)
		}
	}
	if utf8.RuneCountInString(c.SelfChatPrefix) > maxWhatsAppPrefixRunes {
		return fmt.Errorf("whatsapp.self_chat_prefix allows at most %d characters", maxWhatsAppPrefixRunes)
	}
	return nil
}

// NormalizeE164 turns a typed phone number into +<digits>. Spaces, dashes,
// dots and parentheses are ignored, and a leading 00 counts as +. The result
// has 7 to 15 digits, and the first digit is not 0.
func NormalizeE164(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "00") {
		s = "+" + s[2:]
	}
	if !strings.HasPrefix(s, "+") {
		return "", errInvalidE164
	}
	var b strings.Builder
	b.WriteByte('+')
	digits := 0
	for _, r := range s[1:] {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			digits++
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')':
		default:
			return "", errInvalidE164
		}
	}
	out := b.String()
	if digits < minE164Digits || digits > maxE164Digits || out[1] == '0' {
		return "", errInvalidE164
	}
	return out, nil
}

// WhatsAppStoreDir is <dataDir>/whatsapp, or "" without a data directory.
func WhatsAppStoreDir(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		return ""
	}
	return filepath.Join(dataDir, WhatsAppDirName)
}

// WhatsAppDeviceDBPath is the linked-device database, or "" without a data
// directory.
func WhatsAppDeviceDBPath(dataDir string) string {
	dir := WhatsAppStoreDir(dataDir)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, WhatsAppDeviceDBFilename)
}

// WhatsAppProtectedPaths lists the device database and its SQLite sidecars,
// which agent file tools must refuse like the vault.
func WhatsAppProtectedPaths(cfg *Config) []string {
	if cfg == nil {
		return nil
	}
	db := WhatsAppDeviceDBPath(cfg.Directories.DataDir)
	if db == "" {
		return nil
	}
	return []string{db, db + "-wal", db + "-shm", db + "-journal"}
}
```

- [ ] **Step 5: Wire it into the config and template**

`internal/config/config_types.go` — directly after the line `	} \`yaml:"telegram"\`` insert:
```go
	WhatsApp WhatsAppConfig `yaml:"whatsapp" json:"whatsapp"`
```

`internal/config/config.go`, in `Load` — directly after `	cfg.ModelCatalog.CatalogOnlyVisible = true` insert:
```go
	ApplyWhatsAppDefaults(&cfg.WhatsApp)
```

In the same function, directly after the closing `}` of the block that starts with `	// Telegram defaults` and sets `cfg.Telegram.MaxConcurrentWorkers = 5`, insert:
```go

	// WhatsApp: canonical mode and E.164 owner numbers.
	NormalizeWhatsAppConfig(&cfg.WhatsApp)
```

`config_template.yaml` — directly before the top-level line `whisper:` insert:
```yaml
whatsapp:
    enabled: false
    # dedicated: AuraGo has its own number and the owner writes to it.
    # self_chat: your personal number; only the "Message yourself" chat reaches AuraGo.
    mode: dedicated
    owner_numbers: []                  # international format, e.g. "+491701234567"; required in dedicated mode
    readonly: false                    # true blocks send_whatsapp and notifications; replies to the owner still work
    send_read_receipts: true           # dedicated mode only
    self_chat_prefix: "🤖 "            # marks AuraGo's messages in self_chat mode
    # Link the account in the Config UI (QR code or pairing code). The linked-device
    # keys live in <data_dir>/whatsapp/device.db and are never backed up.
```

`internal/config/template_integrations_test.go` — in the map literal of the `for name, target := range map[string]any{` loop, add the line `		"whatsapp": &cfg.WhatsApp,` before `	} {`. Append `|| cfg.WhatsApp.Enabled` to the condition of the `"reference silently enables an optional integration"` check. Add as the last statement of the function:
```go
	if cfg.WhatsApp.Mode != WhatsAppModeDedicated || !cfg.WhatsApp.SendReadReceipts || cfg.WhatsApp.ReadOnly ||
		cfg.WhatsApp.SelfChatPrefix != DefaultWhatsAppSelfChatPrefix || len(cfg.WhatsApp.OwnerNumbers) != 0 {
		t.Fatalf("reference whatsapp block differs from loader defaults: %+v", cfg.WhatsApp)
	}
```

`internal/audit/sqlite_backup_coverage_test.go` — in `sqliteBackupExclusions`, after the entry `"media_registry.db": ...`, add:
```go

	// Credential store, deliberately outside backups: a restored copy would clone
	// the linked device. The owner relinks after a restore.
	"device.db": "WhatsApp linked-device keys (<data_dir>/whatsapp/device.db)",
```
Then run `gofmt -w internal/config/whatsapp.go internal/config/whatsapp_test.go internal/config/config_types.go internal/config/config.go internal/config/template_integrations_test.go internal/audit/sqlite_backup_coverage_test.go`.

- [ ] **Step 6: Run the tests and confirm they pass**

```bash
CGO_ENABLED=0 go test ./internal/config/ -run '(TestWhatsApp|TestNormalizeE164|TestValidateWhatsAppConfig|TestIntegrationTemplateSections|TestRetroGames)' -count=1
CGO_ENABLED=0 go test ./internal/audit/ -run 'TestSQLiteOpensAreBackedUpOrExcluded' -count=1
CGO_ENABLED=0 go vet ./internal/config/
```
Expected: PASS (`ok aurago/internal/config`, `ok aurago/internal/audit`); vet prints nothing.

- [ ] **Step 7: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="internal/config/whatsapp.go internal/config/whatsapp_test.go internal/config/config_types.go internal/config/config.go config_template.yaml internal/config/template_integrations_test.go internal/audit/sqlite_backup_coverage_test.go"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "feat(config): whatsapp section with E.164 owner numbers" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```

---

### Task 2: Integration status

**Files:**
- Create: `internal/integrationstatus/whatsapp.go`
- Create (test): `internal/integrationstatus/whatsapp_test.go`

- [ ] **Step 1: Write the failing test** `internal/integrationstatus/whatsapp_test.go`

```go
package integrationstatus

import (
	"testing"
	"time"
)

func TestWhatsAppStatusTransitions(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	SetWhatsAppConfigState(WhatsAppStateDisabled, "")
	if s := WhatsAppStatus(); s.Enabled || s.State != WhatsAppStateDisabled {
		t.Fatalf("disabled status = %+v", s)
	}
	SetWhatsAppConfigState(WhatsAppStateConnecting, "dedicated")
	SetWhatsAppLinked(true, "***5678")
	MarkWhatsAppConnected(now)
	MarkWhatsAppTempBan("temp_ban_101", now.Add(time.Hour))
	s := WhatsAppStatus()
	if !s.Enabled || s.State != WhatsAppStateTempBanned || s.LastError != "temp_ban_101" ||
		s.TempBanExpiresAt != "2026-10-31T13:00:00Z" || s.ConnectedSince != "2026-10-31T12:00:00Z" ||
		!s.Linked || s.LinkedJIDMasked != "***5678" || s.Mode != "dedicated" {
		t.Fatalf("temp ban status = %+v", s)
	}
	received, failures := s.MessagesReceived, s.DeliveryFailures
	CountWhatsAppMessage(now)
	MarkWhatsAppDeliveryFailure()
	MarkWhatsAppOutbound(now)
	SetWhatsAppState(WhatsAppStateHealthy)
	s = WhatsAppStatus()
	if s.State != WhatsAppStateHealthy || s.LastError != "" || s.TempBanExpiresAt != "" ||
		s.MessagesReceived != received+1 || s.DeliveryFailures != failures+1 ||
		s.LastInboundAt == "" || s.LastOutboundAt == "" {
		t.Fatalf("healthy status = %+v", s)
	}
	SetWhatsAppConfigState(WhatsAppStateNotConfigured, "dedicated")
	if s := WhatsAppStatus(); !s.Enabled || s.Linked || s.LinkedJIDMasked != "" || s.ConnectedSince != "" {
		t.Fatalf("not_configured status kept connection details: %+v", s)
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

```bash
CGO_ENABLED=0 go test ./internal/integrationstatus/ -run 'TestWhatsAppStatusTransitions' -count=1
```
Expected: FAIL, `undefined: SetWhatsAppConfigState`.

- [ ] **Step 3: Implement** `internal/integrationstatus/whatsapp.go`

```go
package integrationstatus

import (
	"sync"
	"time"
)

// WhatsApp runtime states. disabled and not_configured come from the
// configuration. The others come from the device store, link sessions and
// whatsmeow events.
const (
	WhatsAppStateDisabled      = "disabled"
	WhatsAppStateNotConfigured = "not_configured"
	WhatsAppStateNotLinked     = "not_linked"
	WhatsAppStateLinking       = "linking"
	WhatsAppStateConnecting    = "connecting"
	WhatsAppStateHealthy       = "healthy"
	WhatsAppStateDegraded      = "degraded"
	WhatsAppStateLoggedOut     = "logged_out"
	WhatsAppStateConflict      = "conflict"
	WhatsAppStateTempBanned    = "temp_banned"
	WhatsAppStateOutdated      = "outdated"
)

// WhatsAppRuntimeStatus is a sanitized passive snapshot. Numbers keep only
// their last four digits. It never holds message content, keys or QR codes.
type WhatsAppRuntimeStatus struct {
	Enabled              bool   `json:"enabled"`
	State                string `json:"state"`
	Mode                 string `json:"mode,omitempty"`
	Linked               bool   `json:"linked"`
	LinkedJIDMasked      string `json:"linked_jid_masked,omitempty"`
	ConnectedSince       string `json:"connected_since,omitempty"`
	LastInboundAt        string `json:"last_inbound_at,omitempty"`
	LastOutboundAt       string `json:"last_outbound_at,omitempty"`
	MessagesReceived     int    `json:"messages_received"`
	DroppedNonOwner      int    `json:"dropped_non_owner"`
	DroppedUnresolvedLID int    `json:"dropped_unresolved_lid"`
	DeliveryFailures     int    `json:"delivery_failures"`
	LastError            string `json:"last_error,omitempty"`
	TempBanExpiresAt     string `json:"temp_ban_expires_at,omitempty"`
	LastTestAt           string `json:"last_test_at,omitempty"`
}

var whatsAppRuntime = struct {
	sync.RWMutex
	status WhatsAppRuntimeStatus
}{status: WhatsAppRuntimeStatus{State: WhatsAppStateDisabled}}

func whatsAppStamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// SetWhatsAppConfigState publishes a new generation's first state and clears
// the previous connection's details. Counters survive generations.
func SetWhatsAppConfigState(state, mode string) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	s := &whatsAppRuntime.status
	s.Enabled = state != WhatsAppStateDisabled
	s.State = state
	s.Mode = mode
	s.ConnectedSince = ""
	s.LastError = ""
	s.TempBanExpiresAt = ""
	if state == WhatsAppStateDisabled || state == WhatsAppStateNotConfigured {
		s.Linked = false
		s.LinkedJIDMasked = ""
	}
}

// SetWhatsAppState moves the runtime to state. healthy clears the last error
// and an expired temporary ban.
func SetWhatsAppState(state string) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	s := &whatsAppRuntime.status
	s.State = state
	switch state {
	case WhatsAppStateHealthy:
		s.LastError = ""
		s.TempBanExpiresAt = ""
	case WhatsAppStateNotLinked, WhatsAppStateLinking:
		s.ConnectedSince = ""
	}
}

// SetWhatsAppLinked records whether the store holds a linked device and its
// masked number.
func SetWhatsAppLinked(linked bool, masked string) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.Linked = linked
	if !linked {
		masked = ""
	}
	whatsAppRuntime.status.LinkedJIDMasked = masked
}

// MarkWhatsAppConnected records the latest completed login.
func MarkWhatsAppConnected(now time.Time) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.ConnectedSince = whatsAppStamp(now)
}

// MarkWhatsAppError sets state together with a fixed error code.
func MarkWhatsAppError(state, code string) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.State = state
	whatsAppRuntime.status.LastError = code
}

// MarkWhatsAppTempBan records a temporary ban. A zero expiry means WhatsApp
// sent none.
func MarkWhatsAppTempBan(code string, expires time.Time) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	s := &whatsAppRuntime.status
	s.State = WhatsAppStateTempBanned
	s.LastError = code
	s.TempBanExpiresAt = ""
	if !expires.IsZero() {
		s.TempBanExpiresAt = whatsAppStamp(expires)
	}
}

// CountWhatsAppMessage counts an inbound message event before any filtering.
func CountWhatsAppMessage(now time.Time) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.MessagesReceived++
	whatsAppRuntime.status.LastInboundAt = whatsAppStamp(now)
}

// MarkWhatsAppOutbound records a delivered message.
func MarkWhatsAppOutbound(now time.Time) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.LastOutboundAt = whatsAppStamp(now)
}

// MarkWhatsAppDeliveryFailure counts a message that could not be delivered.
func MarkWhatsAppDeliveryFailure() {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.DeliveryFailures++
}

// MarkWhatsAppTest records a delivered manual test message.
func MarkWhatsAppTest(now time.Time) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.LastTestAt = whatsAppStamp(now)
}

// WhatsAppStatus returns a copy of the current snapshot.
func WhatsAppStatus() WhatsAppRuntimeStatus {
	whatsAppRuntime.RLock()
	defer whatsAppRuntime.RUnlock()
	return whatsAppRuntime.status
}
```

- [ ] **Step 4: Run the test and confirm it passes**

```bash
CGO_ENABLED=0 go test ./internal/integrationstatus/ -count=1
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="internal/integrationstatus/whatsapp.go internal/integrationstatus/whatsapp_test.go"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "feat(integrationstatus): whatsapp runtime status" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```

---

### Task 3: Dependency, device store, log adapter, masking

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/whatsapp/store.go`, `internal/whatsapp/log.go`, `internal/whatsapp/mask.go`
- Create (test): `internal/whatsapp/store_test.go`, `internal/whatsapp/log_test.go`

- [ ] **Step 1: Add the dependency**

```bash
go get go.mau.fi/whatsmeow@v0.0.0-20261030195514-56e5ad0581ce
git diff -- go.mod
```
Expected:
- `go.mau.fi/whatsmeow v0.0.0-20261030195514-56e5ad0581ce` appears.
- `go.mau.fi/libsignal`, `go.mau.fi/util` and their dependencies appear as indirect.
- `golang.org/x/exp` may rise to whatsmeow's minimum.

If any other existing require changes, or go.mod/go.sum show hunks from another session, stop and report.

- [ ] **Step 2: Confirm the SQLite dialect name**

```bash
go doc go.mau.fi/util/dbutil ParseDialect
```
Expected: SQLite is selected for `sqlite` (prefix match on `sqlite`/`sqlite3`). The store test below proves it too, because `sqlstore.NewWithDB` panics on an unknown dialect.

- [ ] **Step 3: Write the failing tests**

`internal/whatsapp/store_test.go`:
```go
package whatsapp

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenDeviceStoreCreatesPrivateDatabaseWithForeignKeys(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	s, err := openDeviceStore(ctx, dataDir, nil)
	if err != nil {
		t.Fatalf("openDeviceStore: %v", err)
	}
	var foreignKeys int
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, %v; want 1", foreignKeys, err)
	}
	var tables int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'whatsmeow_device'").Scan(&tables); err != nil || tables != 1 {
		t.Fatalf("whatsmeow_device tables = %d, %v; want 1", tables, err)
	}
	if s.Linked() || !s.OwnJID().IsEmpty() {
		t.Fatal("a fresh store reports a linked device")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	reopened, err := openDeviceStore(ctx, dataDir, nil)
	if err != nil {
		t.Fatalf("reopen migrated store: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close reopened: %v", err)
	}
	dir := filepath.Join(dataDir, "whatsapp")
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("store dir: %v", err)
	}
	fileInfo, err := os.Stat(filepath.Join(dir, "device.db"))
	if err != nil {
		t.Fatalf("store file: %v", err)
	}
	if runtime.GOOS != "windows" && (dirInfo.Mode().Perm() != 0o700 || fileInfo.Mode().Perm() != 0o600) {
		t.Fatalf("modes dir=%v file=%v, want 0700/0600", dirInfo.Mode().Perm(), fileInfo.Mode().Perm())
	}
}

func TestOpenDeviceStoreRequiresDataDir(t *testing.T) {
	if _, err := openDeviceStore(context.Background(), " ", nil); err == nil {
		t.Fatal("an empty data dir opened a device store")
	}
}
```

`internal/whatsapp/log_test.go`:
```go
package whatsapp

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestMaskDigitsKeepsLastFour(t *testing.T) {
	for in, want := range map[string]string{
		"4917012345678@s.whatsapp.net": "***5678@s.whatsapp.net",
		"code 405 after 2026":          "code 405 after 2026",
		"123456789012345:12@lid":       "***2345:12@lid",
	} {
		if got := maskDigits(in); got != want {
			t.Errorf("maskDigits(%q) = %q, want %q", in, got, want)
		}
	}
	if got := maskJID(types.NewJID("4917012345678", types.DefaultUserServer)); got != "***5678" {
		t.Errorf("maskJID = %q, want ***5678", got)
	}
	if maskJID(types.EmptyJID) != "" {
		t.Error("an empty JID produced a masked value")
	}
}

func TestWALoggerMasksAndDropsDebug(t *testing.T) {
	var buf bytes.Buffer
	log := newWALogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), "Client")
	log.Debugf("Emitting QR code %s", "2@secret-qr-payload")
	log.Sub("Socket").Infof("Successfully paired %s", "4917012345678:3@s.whatsapp.net")
	out := buf.String()
	if strings.Contains(out, "secret-qr-payload") {
		t.Fatal("whatsmeow debug output reached the log")
	}
	if strings.Contains(out, "4917012345678") || !strings.Contains(out, "***5678") || !strings.Contains(out, "Client/Socket") {
		t.Fatalf("log = %q", out)
	}
}
```

- [ ] **Step 4: Run them and confirm they fail**

```bash
CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestOpenDeviceStore|TestMaskDigits|TestWALogger)' -count=1
```
Expected: FAIL, `undefined: openDeviceStore` / `maskDigits` / `newWALogger`.

- [ ] **Step 5: Implement**

`internal/whatsapp/mask.go`:
```go
package whatsapp

import (
	"regexp"

	"go.mau.fi/whatsmeow/types"
)

var longDigitRun = regexp.MustCompile(`\d{5,}`)

// maskDigits keeps the last four digits of every run of five or more digits,
// so phone numbers and JID users never reach logs or status in full.
func maskDigits(s string) string {
	return longDigitRun.ReplaceAllStringFunc(s, func(run string) string {
		return "***" + run[len(run)-4:]
	})
}

// maskJID returns the masked user part of a phone or LID JID, or "".
func maskJID(jid types.JID) string {
	if jid.IsEmpty() {
		return ""
	}
	return maskDigits(jid.User)
}
```

`internal/whatsapp/log.go`:
```go
package whatsapp

import (
	"context"
	"fmt"
	"log/slog"

	"aurago/internal/security"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// slogLogger forwards whatsmeow logging to slog. Debug output is dropped
// because it carries QR payloads and raw protocol nodes. Every line is
// scrubbed, and phone numbers keep only their last four digits.
type slogLogger struct {
	logger *slog.Logger
	module string
}

func newWALogger(logger *slog.Logger, module string) waLog.Logger {
	if logger == nil {
		return waLog.Noop
	}
	return slogLogger{logger: logger, module: module}
}

func (l slogLogger) Errorf(msg string, args ...any) { l.log(slog.LevelError, msg, args...) }
func (l slogLogger) Warnf(msg string, args ...any)  { l.log(slog.LevelWarn, msg, args...) }
func (l slogLogger) Infof(msg string, args ...any)  { l.log(slog.LevelInfo, msg, args...) }
func (l slogLogger) Debugf(string, ...any)          {}

func (l slogLogger) Sub(module string) waLog.Logger {
	return slogLogger{logger: l.logger, module: l.module + "/" + module}
}

func (l slogLogger) log(level slog.Level, msg string, args ...any) {
	text := maskDigits(security.Scrub(fmt.Sprintf(msg, args...)))
	l.logger.Log(context.Background(), level, "whatsmeow: "+text, "module", l.module)
}
```

`internal/whatsapp/store.go`:
```go
package whatsapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/fileutil"

	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "modernc.org/sqlite"
)

// storeDSNQuery enables the foreign keys whatsmeow's Upgrade insists on and
// waits for a concurrent writer instead of failing with SQLITE_BUSY.
const storeDSNQuery = "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"

var errNoDataDir = errors.New("whatsapp device store: directories.data_dir is not set")

// sqliteDeviceStore is one opened <data_dir>/whatsapp/device.db with its first
// and only device. A fresh store holds an unsaved, unlinked device.
type sqliteDeviceStore struct {
	db     *sql.DB
	device *store.Device
	log    waLog.Logger
}

// openDeviceStore creates the private store on first use, migrates whatsmeow's
// schema and loads the device.
func openDeviceStore(ctx context.Context, dataDir string, log waLog.Logger) (*sqliteDeviceStore, error) {
	path := config.WhatsAppDeviceDBPath(dataDir)
	if path == "" {
		return nil, errNoDataDir
	}
	if log == nil {
		log = waLog.Noop
	}
	if err := preparePrivateStoreFile(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", sqliteURI(path, storeDSNQuery))
	if err != nil {
		return nil, fmt.Errorf("open whatsapp device store: %w", err)
	}
	container := sqlstore.NewWithDB(db, "sqlite", log.Sub("Store"))
	if err := container.Upgrade(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate whatsapp device store: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("read whatsapp device: %w", err)
	}
	return &sqliteDeviceStore{db: db, device: device, log: log}, nil
}

// preparePrivateStoreFile creates the directory (0700) and the file (0600) and
// restricts both to the owner, so the keys never sit in a readable file.
func preparePrivateStoreFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create whatsapp store directory: %w", err)
	}
	if err := fileutil.RestrictToOwner(dir); err != nil {
		return fmt.Errorf("protect whatsapp store directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("create whatsapp device store: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("create whatsapp device store: %w", err)
	}
	if err := fileutil.RestrictToOwner(path); err != nil {
		return fmt.Errorf("protect whatsapp device store: %w", err)
	}
	return nil
}

// sqliteURI builds a file: URI that also works for Windows drive paths and
// names with spaces, like internal/memory's snapshot URIs.
func sqliteURI(path, query string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	value := filepath.ToSlash(abs)
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return (&url.URL{Scheme: "file", Path: value, RawQuery: query}).String()
}

// Linked reports whether the store holds a paired device.
func (s *sqliteDeviceStore) Linked() bool { return s.device != nil && s.device.ID != nil }

// OwnJID is the linked account's phone JID without a device part.
func (s *sqliteDeviceStore) OwnJID() types.JID {
	if !s.Linked() {
		return types.EmptyJID
	}
	return s.device.ID.ToNonAD()
}

// DeleteLocal removes the linked device's keys from the store.
func (s *sqliteDeviceStore) DeleteLocal(ctx context.Context) error {
	if !s.Linked() {
		return nil
	}
	if err := s.device.Delete(ctx); err != nil {
		return fmt.Errorf("delete whatsapp device: %w", err)
	}
	return nil
}

// Close releases the database handle.
func (s *sqliteDeviceStore) Close() error { return s.db.Close() }
```

- [ ] **Step 6: Tidy, run the tests, confirm they pass, and cross-compile**

```bash
go mod tidy
git diff --stat -- go.mod go.sum
CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestOpenDeviceStore|TestMaskDigits|TestWALogger)' -count=1
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./internal/whatsapp/
```
Expected:
- `go.mod` lists whatsmeow as a direct require.
- The tests PASS.
- The arm64 build succeeds without CGO.

- [ ] **Step 7: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="go.mod go.sum internal/whatsapp/store.go internal/whatsapp/log.go internal/whatsapp/mask.go internal/whatsapp/store_test.go internal/whatsapp/log_test.go"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "feat(whatsapp): whatsmeow dependency and private device store" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```

---

### Task 4: License documentation and guard test

**Files:**
- Modify: `THIRD_PARTY_LIBRARIES.md` — header bullets, §1 table, §2 table, §9 table
- Modify: `THIRD_PARTY_NOTICES.md` — new section after the `# Third-party notices` title
- Modify: `licences/UEBERSICHT.md` — Sonderfälle table, `### MPL-2.0` list, new transitive paragraph under `## Go-Direktabhängigkeiten`
- Modify: `README.md` — line 276
- Modify: `licences/AGENTS.md` — Local Contracts and Verification
- Create (test): `internal/audit/whatsapp_license_docs_test.go`

- [ ] **Step 1: Write the failing guard test** `internal/audit/whatsapp_license_docs_test.go`

```go
package audit

import (
	"strings"
	"testing"
)

// The WhatsApp stack links GPL-3.0 code. AuraGo's source stays MIT, and
// distributed binaries are GPL-3.0 combined works. Every inventory says so.
func TestWhatsAppLicenseNoticesStayAligned(t *testing.T) {
	t.Parallel()
	license := readRepoFile(t, "LICENSE")
	if !strings.Contains(license, "MIT License") || strings.Contains(license, "GNU GENERAL PUBLIC LICENSE") {
		t.Fatal("LICENSE must stay the MIT text")
	}
	requireAll(t, "THIRD_PARTY_NOTICES.md", readRepoFile(t, "THIRD_PARTY_NOTICES.md"), []string{
		"## WhatsApp channel (whatsmeow and libsignal)",
		"`go.mau.fi/libsignal`",
		"GNU General Public License version 3",
		"`licences/texts/GPL-3.0-only.txt`",
	})
	requireAll(t, "THIRD_PARTY_LIBRARIES.md", readRepoFile(t, "THIRD_PARTY_LIBRARIES.md"), []string{
		"| `go.mau.fi/whatsmeow` | v0.0.0-20261030195514-56e5ad0581ce | MPL-2.0 |",
		"| go.mau.fi/libsignal | GPL-3.0 |",
	})
	requireAll(t, "licences/UEBERSICHT.md", readRepoFile(t, "licences/UEBERSICHT.md"), []string{
		"`go.mau.fi/whatsmeow` v0.0.0-20261030195514-56e5ad0581ce",
		"GPL-3.0-Gesamtwerke",
	})
	requireAll(t, "README.md", readRepoFile(t, "README.md"), []string{
		"**Source code: [MIT licensed](LICENSE).**",
		"GPL-3.0 combined works",
	})
}
```

- [ ] **Step 2: Run it and confirm it fails**

```bash
CGO_ENABLED=0 go test ./internal/audit/ -run 'TestWhatsAppLicenseNoticesStayAligned' -count=1
```
Expected: FAIL, `THIRD_PARTY_NOTICES.md is missing "## WhatsApp channel (whatsmeow and libsignal)"` (the `requireAll` message).

- [ ] **Step 3: Read the licenses of every new module**

```bash
C=$(git log -1 --format=%H -- go.mod)
git diff "$C~1" "$C" -- go.mod
MOD=$(go env GOMODCACHE)
for m in go.mau.fi/whatsmeow@v0.0.0-20261030195514-56e5ad0581ce go.mau.fi/libsignal@$(go list -m -f '{{.Version}}' go.mau.fi/libsignal) go.mau.fi/util@$(go list -m -f '{{.Version}}' go.mau.fi/util); do head -n 5 "$MOD/$m/LICENSE"; done
```
Expected: the Mozilla Public License 2.0 header (whatsmeow, util) and the GNU General Public License version 3 header (libsignal).

Then run `head -n 5 "$MOD/<path>@<version>/LICENSE"` for every other module the diff shows as newly added (for example `github.com/beeper/argo-go`, `github.com/rs/zerolog`, `github.com/elliotchance/orderedmap/v3`, `github.com/petermattis/goid`, `github.com/vektah/gqlparser/v2`). Record each SPDX identifier exactly as read. If a license family has no file in `licences/texts/`, copy that module's full license text verbatim into `licences/texts/<SPDX>.txt`. `GPL-3.0-only.txt`, `MPL-2.0.txt`, `MIT.txt`, `ISC.txt`, `BSD-2-Clause.txt`, `BSD-3-Clause.txt` and `Apache-2.0.txt` already exist.

- [ ] **Step 4: Edit the documents**

`THIRD_PARTY_NOTICES.md` — directly after the first line `# Third-party notices` insert:
```markdown

## WhatsApp channel (whatsmeow and libsignal)

The WhatsApp channel compiles `go.mau.fi/whatsmeow` (MPL-2.0, Copyright Tulir
Asokan and contributors), `go.mau.fi/util` (MPL-2.0) and `go.mau.fi/libsignal`
(GPL-3.0, Go port of the Signal protocol library) into the AuraGo binary.
AuraGo's own source code stays under the MIT license in `LICENSE`. Because the
binary links GPL-3.0 code, distributed AuraGo binaries are combined works under
the GNU General Public License version 3. Their corresponding source is the tagged
AuraGo commit of the release plus the module versions pinned in `go.mod` and
`go.sum`. The modules are used unmodified. License texts:
`licences/texts/GPL-3.0-only.txt` and `licences/texts/MPL-2.0.txt`.
```

`THIRD_PARTY_LIBRARIES.md`:
- Replace the bullet `- **AuraGo project license:** [MIT](LICENSE)` with `- **AuraGo project license:** [MIT](LICENSE) for the source code; distributed binaries are GPL-3.0 combined works because the WhatsApp channel links GPL-3.0 code (see §9)`.
- Set `- **Reviewed:** 2026-10-31`.
- In the §1 table, between the `github.com/xuri/excelize/v2` and `golang.org/x/crypto` rows, add:
```markdown
| `go.mau.fi/whatsmeow` | v0.0.0-20261030195514-56e5ad0581ce | MPL-2.0 | https://github.com/tulir/whatsmeow |
```
- In the §2 table, after the `github.com/ysmood/*` row, add:
```markdown
| `go.mau.fi/libsignal` | GPL-3.0 | https://github.com/tulir/libsignal-protocol-go |
| `go.mau.fi/util` | MPL-2.0 | https://github.com/mautrix/go-util |
```
  Add one more §2 row per other new module from Step 3, in the form ``| `<module path>` | <SPDX read in Step 3> | <repository URL from the module's go.mod or LICENSE> |``.
- In the §9 table, after the `chromem-go` row, add:
```markdown
| whatsmeow | MPL-2.0 | WhatsApp channel |
| go.mau.fi/util | MPL-2.0 | WhatsApp channel (whatsmeow dependency) |
| go.mau.fi/libsignal | GPL-3.0 | WhatsApp channel (whatsmeow dependency); makes distributed AuraGo binaries GPL-3.0 combined works |
```

`licences/UEBERSICHT.md`:
- In the Sonderfälle table, after the row starting `| noVNC, Diago, go-sql-driver/mysql, chromem-go |`, add:
```markdown
| whatsmeow, go.mau.fi/util, go.mau.fi/libsignal | MPL-2.0, MPL-2.0, GPL-3.0 | WhatsApp-Kanal, in die AuraGo-Binärdatei kompiliert. Der Quellcode von AuraGo bleibt MIT. Weitergegebene Binärdateien sind dadurch GPL-3.0-Gesamtwerke und kommen mit Lizenztext und dem Quellcode des getaggten Commits. Texte: `texts/GPL-3.0-only.txt`, `texts/MPL-2.0.txt`. |
```
- Under `### MPL-2.0`, add the line `` `go.mau.fi/whatsmeow` v0.0.0-20261030195514-56e5ad0581ce, `` before `` `github.com/emiago/diago` ``.
- After the paragraph under `## Go-Direktabhängigkeiten` (the one ending `… enthält SQLite als Public Domain.`), add:
```markdown

Neu über den WhatsApp-Kanal (transitiv): `go.mau.fi/libsignal` (GPL-3.0) und `go.mau.fi/util` (MPL-2.0).
```
  Then append each other new module from Step 3 to this sentence as `` `<module path>` (<SPDX>) ``, separated by commas and ending with a period.

`README.md` line 276 — replace `**[MIT licensed](LICENSE).** Have fun. Mind the permissions.` with:
```markdown
**Source code: [MIT licensed](LICENSE).** The release binaries link the GPL-3.0 WhatsApp stack, so the binaries we ship are GPL-3.0 combined works ([notices](THIRD_PARTY_NOTICES.md)). Have fun. Mind the permissions.
```

`licences/AGENTS.md`:
- Add to `## Local Contracts`:
```markdown
- WhatsApp: `go.mau.fi/libsignal` (GPL-3.0) is linked into the binary through whatsmeow (MPL-2.0). `LICENSE` stays MIT; `THIRD_PARTY_NOTICES.md`, `THIRD_PARTY_LIBRARIES.md` §9, the Sonderfälle row here and the README license line state that distributed binaries are GPL-3.0 combined works. A GPL-3.0-incompatible module linked into the binary blocks a release. Guard: `internal/audit` `TestWhatsAppLicenseNoticesStayAligned`.
```
- Add to `## Verification`:
```markdown
Modules linked into the binary: `go list -deps -f '{{with .Module}}{{.Path}} {{.Version}}{{end}}' ./cmd/aurago | sort -u`.
```

- [ ] **Step 5: Run the tests and confirm they pass**

```bash
CGO_ENABLED=0 go test ./internal/audit/ -run '(TestWhatsAppLicenseNoticesStayAligned|TestLocalWikipedia|TestDashboardManual|TestPersonDetection)' -count=1
```
Expected: PASS. The README strings pinned by the Local Wikipedia and Dashboard audit tests are unchanged.

- [ ] **Step 6: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="THIRD_PARTY_LIBRARIES.md THIRD_PARTY_NOTICES.md licences/UEBERSICHT.md README.md licences/AGENTS.md internal/audit/whatsapp_license_docs_test.go"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "docs(licences): whatsmeow and libsignal; MIT source, GPL-3.0 binaries" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```
Add any new `licences/texts/<SPDX>.txt` from Step 3 to `F`.

---

### Task 5: Client interface, whatsmeow adapter, fakes

**Files:**
- Create: `internal/whatsapp/errors.go`, `internal/whatsapp/client.go`
- Create (test): `internal/whatsapp/fake_test.go`, `internal/whatsapp/client_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/whatsapp/fake_test.go` (shared by the tests of Tasks 5–8):
```go
package whatsapp

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

type sentMessage struct {
	to   types.JID
	text string
}

// fakeClient records every call and lets a test emit whatsmeow events.
type fakeClient struct {
	mu          sync.Mutex
	handlers    []func(evt any)
	connectErrs []error
	connects    int
	disconnects int
	loggedIn    bool
	own         types.JID
	qr          chan whatsmeow.QRChannelItem
	pairCode    string
	pairPhones  []string
	logoutErr   error
	logouts     int
	sendErr     error
	sent        []sentMessage
	presences   []types.Presence
	presenceErr error
}

func (f *fakeClient) with(fn func(f *fakeClient)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fakeClient) AddEventHandler(handler func(evt any)) uint32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers = append(f.handlers, handler)
	return uint32(len(f.handlers))
}

func (f *fakeClient) ConnectContext(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connects++
	if len(f.connectErrs) > 0 {
		err := f.connectErrs[0]
		f.connectErrs = f.connectErrs[1:]
		return err
	}
	return nil
}

func (f *fakeClient) Disconnect() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disconnects++
}

func (f *fakeClient) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connects > f.disconnects
}

func (f *fakeClient) IsLoggedIn() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loggedIn
}

func (f *fakeClient) GetQRChannel(context.Context) (<-chan whatsmeow.QRChannelItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.qr == nil {
		return nil, errors.New("fake: no qr channel")
	}
	return f.qr, nil
}

func (f *fakeClient) PairPhone(_ context.Context, phone string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pairPhones = append(f.pairPhones, phone)
	return f.pairCode, nil
}

func (f *fakeClient) Logout(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logouts++
	return f.logoutErr
}

func (f *fakeClient) SendMessage(_ context.Context, to types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		return whatsmeow.SendResponse{}, f.sendErr
	}
	f.sent = append(f.sent, sentMessage{to: to, text: msg.GetConversation()})
	return whatsmeow.SendResponse{ID: types.MessageID("fake-" + strconv.Itoa(len(f.sent)))}, nil
}

func (f *fakeClient) SendPresence(_ context.Context, state types.Presence) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.presences = append(f.presences, state)
	return f.presenceErr
}

func (f *fakeClient) OwnJID() types.JID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.own
}

// emit delivers evt to every handler, like whatsmeow's dispatcher.
func (f *fakeClient) emit(evt any) {
	f.mu.Lock()
	handlers := slices.Clone(f.handlers)
	f.mu.Unlock()
	for _, handler := range handlers {
		handler(evt)
	}
}

// fakeStore is one opened device store backed by a fakeClient.
type fakeStore struct {
	mu      sync.Mutex
	linked  bool
	own     types.JID
	client  *fakeClient
	deleted int
	closed  int
}

func (s *fakeStore) Linked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.linked
}

func (s *fakeStore) OwnJID() types.JID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.own
}

func (s *fakeStore) NewClient() waClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

func (s *fakeStore) DeleteLocal(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted++
	s.linked = false
	return nil
}

func (s *fakeStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed++
	return nil
}

type fakeBackend struct {
	store *fakeStore
	err   error
}

func (b *fakeBackend) open(context.Context, string) (deviceStore, error) {
	if b.err != nil {
		return nil, b.err
	}
	return b.store, nil
}

// waitFor polls cond until it holds or five seconds pass.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}
```

`internal/whatsapp/client_test.go`:
```go
package whatsapp

import (
	"context"
	"testing"

	"go.mau.fi/whatsmeow/store"
)

func TestSQLiteBackendOpensUnlinkedStoreAndNamesDevice(t *testing.T) {
	var _ waClient = (*fakeClient)(nil)
	var _ deviceStore = (*fakeStore)(nil)
	var _ backend = (*fakeBackend)(nil)

	ctx := context.Background()
	s, err := (sqliteBackend{}).open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	client := s.NewClient()
	if client.IsConnected() || client.IsLoggedIn() || !client.OwnJID().IsEmpty() {
		t.Fatal("an unlinked client reports a session")
	}
	if err := s.DeleteLocal(ctx); err != nil {
		t.Fatalf("DeleteLocal on an unlinked store: %v", err)
	}
	if got := store.DeviceProps.GetOs(); got != "AuraGo" {
		t.Fatalf("linked device name = %q, want AuraGo", got)
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

```bash
CGO_ENABLED=0 go test ./internal/whatsapp/ -run 'TestSQLiteBackendOpensUnlinkedStoreAndNamesDevice' -count=1
```
Expected: FAIL, `undefined: waClient`, `undefined: sqliteBackend`.

- [ ] **Step 3: Implement**

`internal/whatsapp/errors.go`:
```go
package whatsapp

import "errors"

// Sentinel errors the server maps to fixed HTTP error codes.
var (
	ErrNotConfigured     = errors.New("whatsapp is disabled or not configured")
	ErrAlreadyLinked     = errors.New("a whatsapp device is already linked")
	ErrLinkInProgress    = errors.New("a whatsapp link session is already active")
	ErrInvalidLinkMethod = errors.New("whatsapp link method must be qr or phone")
	ErrInvalidPhone      = errors.New("whatsapp pairing needs an international phone number")
	ErrNotConnected      = errors.New("whatsapp is not connected")
	ErrNoRecipient       = errors.New("whatsapp has no owner recipient")
)
```

`internal/whatsapp/client.go`:
```go
package whatsapp

import (
	"context"
	"sync"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// linkedDeviceName is what the phone lists under Linked devices.
const linkedDeviceName = "AuraGo"

// pairClientDisplay is the companion name WhatsApp validates for pairing
// codes. It must have the form "Browser (OS)".
const pairClientDisplay = "Chrome (Linux)"

// waClient is the narrow transport the channel uses. whatsmeowClient wraps the
// library; tests use fakeClient. Later stages add receipts, chat presence, LID
// lookups and media here.
type waClient interface {
	AddEventHandler(handler func(evt any)) uint32
	ConnectContext(ctx context.Context) error
	Disconnect()
	IsConnected() bool
	IsLoggedIn() bool
	GetQRChannel(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error)
	PairPhone(ctx context.Context, phone string) (string, error)
	Logout(ctx context.Context) error
	SendMessage(ctx context.Context, to types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error)
	SendPresence(ctx context.Context, state types.Presence) error
	OwnJID() types.JID
}

// deviceStore is one opened linked-device database.
type deviceStore interface {
	Linked() bool
	OwnJID() types.JID
	NewClient() waClient
	DeleteLocal(ctx context.Context) error
	Close() error
}

// backend opens device stores. sqliteBackend is the production backend.
type backend interface {
	open(ctx context.Context, dataDir string) (deviceStore, error)
}

type sqliteBackend struct{ log waLog.Logger }

func (b sqliteBackend) open(ctx context.Context, dataDir string) (deviceStore, error) {
	s, err := openDeviceStore(ctx, dataDir, b.log)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// NewClient binds a whatsmeow client to the store's device.
func (s *sqliteDeviceStore) NewClient() waClient { return newWhatsmeowClient(s.device, s.log) }

var devicePropsOnce sync.Once

// configureDeviceProps names the linked device "AuraGo". It changes
// whatsmeow's process-wide defaults, so it runs once, before the first client.
// RequireFullSync stays false: no full history sync is requested.
func configureDeviceProps() {
	devicePropsOnce.Do(func() {
		store.SetOSInfo(linkedDeviceName, [3]uint32{1, 0, 0})
		store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_CHROME.Enum()
	})
}

type whatsmeowClient struct{ cli *whatsmeow.Client }

func newWhatsmeowClient(device *store.Device, log waLog.Logger) *whatsmeowClient {
	configureDeviceProps()
	cli := whatsmeow.NewClient(device, log)
	cli.EnableAutoReconnect = true
	return &whatsmeowClient{cli: cli}
}

func (c *whatsmeowClient) AddEventHandler(handler func(evt any)) uint32 {
	return c.cli.AddEventHandler(handler)
}

func (c *whatsmeowClient) ConnectContext(ctx context.Context) error { return c.cli.ConnectContext(ctx) }
func (c *whatsmeowClient) Disconnect()                              { c.cli.Disconnect() }
func (c *whatsmeowClient) IsConnected() bool                        { return c.cli.IsConnected() }
func (c *whatsmeowClient) IsLoggedIn() bool                         { return c.cli.IsLoggedIn() }

func (c *whatsmeowClient) GetQRChannel(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error) {
	return c.cli.GetQRChannel(ctx)
}

func (c *whatsmeowClient) PairPhone(ctx context.Context, phone string) (string, error) {
	return c.cli.PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, pairClientDisplay)
}

func (c *whatsmeowClient) Logout(ctx context.Context) error { return c.cli.Logout(ctx) }

func (c *whatsmeowClient) SendMessage(ctx context.Context, to types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
	return c.cli.SendMessage(ctx, to, msg)
}

func (c *whatsmeowClient) SendPresence(ctx context.Context, state types.Presence) error {
	return c.cli.SendPresence(ctx, state)
}

func (c *whatsmeowClient) OwnJID() types.JID { return c.cli.Store.GetJID().ToNonAD() }
```

- [ ] **Step 4: Run the tests and confirm they pass**

```bash
CGO_ENABLED=0 go test ./internal/whatsapp/ -count=1
CGO_ENABLED=0 go vet ./internal/whatsapp/
```
Expected: PASS; vet prints nothing.

- [ ] **Step 5: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="internal/whatsapp/errors.go internal/whatsapp/client.go internal/whatsapp/fake_test.go internal/whatsapp/client_test.go"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "feat(whatsapp): waClient interface and whatsmeow adapter" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```

---

### Task 6: Event → state mapping

**Files:**
- Create: `internal/whatsapp/events.go`
- Create (test): `internal/whatsapp/events_test.go`

- [ ] **Step 1: Write the failing test** `internal/whatsapp/events_test.go`

```go
package whatsapp

import (
	"testing"
	"time"

	"aurago/internal/integrationstatus"

	"go.mau.fi/whatsmeow/types/events"
)

func TestClassifyEventMapsStatesAndPolicies(t *testing.T) {
	const (
		connecting = integrationstatus.WhatsAppStateConnecting
		healthy    = integrationstatus.WhatsAppStateHealthy
		degraded   = integrationstatus.WhatsAppStateDegraded
	)
	for name, tc := range map[string]struct {
		evt    any
		synced bool
		want   eventEffect
	}{
		"connected":                     {&events.Connected{}, true, eventEffect{State: connecting, Connected: true}},
		"offline sync":                  {&events.OfflineSyncCompleted{Count: 3}, false, eventEffect{State: healthy, Synced: true, Healthy: true, SendPresence: true}},
		"keepalive restored before sync": {&events.KeepAliveRestored{}, false, eventEffect{}},
		"keepalive restored after sync": {&events.KeepAliveRestored{}, true, eventEffect{State: healthy, Healthy: true}},
		"disconnected":                  {&events.Disconnected{}, true, eventEffect{State: degraded, ErrorCode: "disconnected"}},
		"keepalive timeout":             {&events.KeepAliveTimeout{ErrorCount: 2}, true, eventEffect{State: degraded, ErrorCode: "keepalive_timeout"}},
		"stream error":                  {&events.StreamError{Code: "999"}, true, eventEffect{State: degraded, ErrorCode: "stream_error"}},
		"logged out": {&events.LoggedOut{Reason: events.ConnectFailureLoggedOut}, true,
			eventEffect{State: integrationstatus.WhatsAppStateLoggedOut, ErrorCode: "logged_out", Terminal: true}},
		"stream replaced": {&events.StreamReplaced{}, true,
			eventEffect{State: integrationstatus.WhatsAppStateConflict, ErrorCode: "stream_replaced", Terminal: true}},
		"client outdated": {&events.ClientOutdated{}, false,
			eventEffect{State: integrationstatus.WhatsAppStateOutdated, ErrorCode: "client_outdated", Terminal: true}},
		"temporary ban": {&events.TemporaryBan{Code: events.TempBanSentToTooManyPeople, Expire: 2 * time.Hour}, true,
			eventEffect{State: integrationstatus.WhatsAppStateTempBanned, ErrorCode: "temp_ban_101", Terminal: true, RetryAfter: 2 * time.Hour}},
		"unknown connect failure": {&events.ConnectFailure{Reason: events.ConnectFailureReason(418)}, false,
			eventEffect{State: degraded, ErrorCode: "connect_failure_418", Reconnect: true}},
		"push name before sync": {&events.PushNameSetting{}, false, eventEffect{}},
		"push name after sync":  {&events.PushNameSetting{}, true, eventEffect{SendPresence: true}},
		"message":               {&events.Message{}, true, eventEffect{Message: true}},
		"history sync ignored":  {&events.HistorySync{}, true, eventEffect{}},
	} {
		if got := classifyEvent(tc.evt, tc.synced); got != tc.want {
			t.Errorf("%s: got %+v, want %+v", name, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

```bash
CGO_ENABLED=0 go test ./internal/whatsapp/ -run 'TestClassifyEventMapsStatesAndPolicies' -count=1
```
Expected: FAIL, `undefined: eventEffect`, `undefined: classifyEvent`.

- [ ] **Step 3: Implement** `internal/whatsapp/events.go`

```go
package whatsapp

import (
	"fmt"
	"time"

	"aurago/internal/integrationstatus"

	"go.mau.fi/whatsmeow/types/events"
)

// eventEffect is what one whatsmeow event means for the runtime.
type eventEffect struct {
	State        string        // status state to publish; "" keeps the current one
	ErrorCode    string        // fixed code for status.last_error
	Connected    bool          // a login completed (events.Connected)
	Synced       bool          // the offline sync of this connection finished
	Healthy      bool          // messages flow; resets the connect backoff
	Terminal     bool          // stop retrying until the user acts or RetryAfter passes
	Reconnect    bool          // whatsmeow will not reconnect by itself
	RetryAfter   time.Duration // temporary ban expiry; 0 waits for user action
	SendPresence bool          // publish presence "unavailable"
	Message      bool          // an inbound message (stage 1 only counts it)
}

// classifyEvent maps a whatsmeow event to status and reconnect policy. synced
// reports whether the current connection already finished its offline sync.
// StreamReplaced, ClientOutdated and TemporaryBan are hard errors (the
// mautrix-whatsapp pattern). The connection is healthy only after
// OfflineSyncCompleted. Transient drops are left to whatsmeow's own
// auto-reconnect. ConnectFailure disables that auto-reconnect, so the Bot
// reconnects itself with backoff.
func classifyEvent(evt any, synced bool) eventEffect {
	switch e := evt.(type) {
	case *events.Connected:
		return eventEffect{State: integrationstatus.WhatsAppStateConnecting, Connected: true}
	case *events.OfflineSyncCompleted:
		return eventEffect{State: integrationstatus.WhatsAppStateHealthy, Synced: true, Healthy: true, SendPresence: true}
	case *events.KeepAliveRestored:
		if synced {
			return eventEffect{State: integrationstatus.WhatsAppStateHealthy, Healthy: true}
		}
	case *events.Disconnected:
		return eventEffect{State: integrationstatus.WhatsAppStateDegraded, ErrorCode: "disconnected"}
	case *events.KeepAliveTimeout:
		return eventEffect{State: integrationstatus.WhatsAppStateDegraded, ErrorCode: "keepalive_timeout"}
	case *events.StreamError:
		return eventEffect{State: integrationstatus.WhatsAppStateDegraded, ErrorCode: "stream_error"}
	case *events.LoggedOut:
		return eventEffect{State: integrationstatus.WhatsAppStateLoggedOut, ErrorCode: "logged_out", Terminal: true}
	case *events.StreamReplaced:
		return eventEffect{State: integrationstatus.WhatsAppStateConflict, ErrorCode: "stream_replaced", Terminal: true}
	case *events.ClientOutdated:
		return eventEffect{State: integrationstatus.WhatsAppStateOutdated, ErrorCode: "client_outdated", Terminal: true}
	case *events.TemporaryBan:
		return eventEffect{
			State:      integrationstatus.WhatsAppStateTempBanned,
			ErrorCode:  fmt.Sprintf("temp_ban_%d", int(e.Code)),
			Terminal:   true,
			RetryAfter: e.Expire,
		}
	case *events.ConnectFailure:
		return eventEffect{
			State:     integrationstatus.WhatsAppStateDegraded,
			ErrorCode: "connect_failure_" + e.Reason.NumberString(),
			Reconnect: true,
		}
	case *events.PushNameSetting:
		if synced {
			return eventEffect{SendPresence: true}
		}
	case *events.Message:
		return eventEffect{Message: true}
	}
	return eventEffect{}
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

```bash
CGO_ENABLED=0 go test ./internal/whatsapp/ -count=1
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="internal/whatsapp/events.go internal/whatsapp/events_test.go"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "feat(whatsapp): event to state and reconnect policy" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```

---


### Task 7: Bot runtime, owner recipients, test message, logout

**Files:**
- Create: `internal/whatsapp/runtime.go`, `internal/whatsapp/recipients.go`
- Create (test): `internal/whatsapp/runtime_test.go`

- [ ] **Step 1: Write the failing test** `internal/whatsapp/runtime_test.go`

```go
package whatsapp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var testOwn = types.NewJID("4917099990000", types.DefaultUserServer)

func fixedNow() time.Time { return time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC) }

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func testBotConfig(mode string, owners ...string) *config.Config {
	cfg := &config.Config{}
	cfg.Directories.DataDir = "data"
	cfg.WhatsApp.Enabled = true
	cfg.WhatsApp.Mode = mode
	cfg.WhatsApp.OwnerNumbers = owners
	cfg.WhatsApp.SendReadReceipts = true
	cfg.WhatsApp.SelfChatPrefix = config.DefaultWhatsAppSelfChatPrefix
	return cfg
}

type recordedSleeps struct {
	mu     sync.Mutex
	values []time.Duration
}

func (r *recordedSleeps) sleep(ctx context.Context, d time.Duration) bool {
	r.mu.Lock()
	r.values = append(r.values, d)
	r.mu.Unlock()
	return ctx.Err() == nil
}

func (r *recordedSleeps) list() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.values)
}

func startTestBot(t *testing.T, cfg *config.Config, st *fakeStore, sleeps *recordedSleeps) *Bot {
	t.Helper()
	deps := runtimeDeps{backend: &fakeBackend{store: st}, sleep: sleeps.sleep, now: fixedNow}
	b := start(context.Background(), cfg, quietLogger(), deps)
	if b == nil {
		t.Fatal("start returned no Bot")
	}
	t.Cleanup(b.Stop)
	return b
}

func clientCount(c *fakeClient, field func(*fakeClient) int) int {
	n := 0
	c.with(func(f *fakeClient) { n = field(f) })
	return n
}

func connectsOf(c *fakeClient) int { return clientCount(c, func(f *fakeClient) int { return f.connects }) }

func TestStartSkipsDisabledAndUnconfiguredSections(t *testing.T) {
	deps := runtimeDeps{backend: &fakeBackend{err: errors.New("must not open")}, sleep: sleepContext, now: fixedNow}
	disabled := testBotConfig(config.WhatsAppModeDedicated, "+491701234567")
	disabled.WhatsApp.Enabled = false
	egg := testBotConfig(config.WhatsAppModeSelfChat)
	egg.EggMode.Enabled = true
	noDataDir := testBotConfig(config.WhatsAppModeSelfChat)
	noDataDir.Directories.DataDir = ""
	for _, c := range []struct {
		cfg  *config.Config
		want string
	}{
		{disabled, integrationstatus.WhatsAppStateDisabled},
		{egg, integrationstatus.WhatsAppStateDisabled},
		{testBotConfig(config.WhatsAppModeDedicated), integrationstatus.WhatsAppStateNotConfigured},
		{noDataDir, integrationstatus.WhatsAppStateNotConfigured},
	} {
		if b := start(context.Background(), c.cfg, quietLogger(), deps); b != nil {
			b.Stop()
			t.Fatalf("a %s section started a Bot", c.want)
		}
		if got := integrationstatus.WhatsAppStatus().State; got != c.want {
			t.Fatalf("state = %q, want %q", got, c.want)
		}
	}
}

func TestBotWithoutLinkedDeviceReportsNotLinked(t *testing.T) {
	client := &fakeClient{}
	st := &fakeStore{client: client}
	b := startTestBot(t, testBotConfig(config.WhatsAppModeSelfChat), st, &recordedSleeps{})
	select {
	case <-b.done:
	case <-time.After(5 * time.Second):
		t.Fatal("an unlinked Bot did not finish")
	}
	st.mu.Lock()
	closed := st.closed
	st.mu.Unlock()
	if s := integrationstatus.WhatsAppStatus(); s.State != integrationstatus.WhatsAppStateNotLinked || s.Linked {
		t.Fatalf("status = %+v", s)
	}
	if connectsOf(client) != 0 || closed != 1 {
		t.Fatalf("connects=%d closed=%d, want 0/1", connectsOf(client), closed)
	}
}

func TestBotHealthyOnlyAfterOfflineSyncAndSendsUnavailable(t *testing.T) {
	client := &fakeClient{own: testOwn, loggedIn: true}
	startTestBot(t, testBotConfig(config.WhatsAppModeDedicated, "+491701234567"), &fakeStore{linked: true, own: testOwn, client: client}, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(client) == 1 })
	client.emit(&events.Connected{})
	if s := integrationstatus.WhatsAppStatus(); s.State != integrationstatus.WhatsAppStateConnecting ||
		s.ConnectedSince != "2026-10-31T12:00:00Z" || !s.Linked || s.LinkedJIDMasked != "***0000" {
		t.Fatalf("after Connected: %+v", s)
	}
	received := integrationstatus.WhatsAppStatus().MessagesReceived
	client.emit(&events.OfflineSyncCompleted{})
	client.emit(&events.Message{})
	if s := integrationstatus.WhatsAppStatus(); s.State != integrationstatus.WhatsAppStateHealthy || s.MessagesReceived != received+1 {
		t.Fatalf("after OfflineSyncCompleted: %+v", s)
	}
	waitFor(t, "presence unavailable", func() bool {
		var presences []types.Presence
		client.with(func(f *fakeClient) { presences = slices.Clone(f.presences) })
		return slices.Equal(presences, []types.Presence{types.PresenceUnavailable})
	})
}

func TestBotBackoffReconnectAndTerminalStates(t *testing.T) {
	dial := errors.New("dial tcp: connection refused")
	client := &fakeClient{connectErrs: []error{dial, dial}, own: testOwn, loggedIn: true}
	sleeps := &recordedSleeps{}
	startTestBot(t, testBotConfig(config.WhatsAppModeDedicated, "+491701234567"), &fakeStore{linked: true, own: testOwn, client: client}, sleeps)
	waitFor(t, "third connect", func() bool { return connectsOf(client) == 3 })
	if got := sleeps.list(); !slices.Equal(got, []time.Duration{5 * time.Second, 10 * time.Second}) {
		t.Fatalf("backoff = %v, want [5s 10s]", got)
	}
	client.emit(&events.ConnectFailure{Reason: events.ConnectFailureReason(418)})
	waitFor(t, "reconnect after a connect failure", func() bool { return connectsOf(client) == 4 })
	client.emit(&events.OfflineSyncCompleted{})
	client.emit(&events.ConnectFailure{Reason: events.ConnectFailureReason(418)})
	waitFor(t, "reconnect after a healthy connection", func() bool { return connectsOf(client) == 5 })
	if got := sleeps.list(); !slices.Equal(got, []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 5 * time.Second}) {
		t.Fatalf("backoff after reconnects = %v", got)
	}
	client.emit(&events.TemporaryBan{Code: events.TempBanSentToTooManyPeople, Expire: time.Hour})
	waitFor(t, "retry after the ban expired", func() bool { return connectsOf(client) == 6 })
	if s := integrationstatus.WhatsAppStatus(); s.TempBanExpiresAt != "2026-10-31T13:00:00Z" || s.LastError != "temp_ban_101" {
		t.Fatalf("temp ban status = %+v", s)
	}
	client.emit(&events.StreamReplaced{})
	waitFor(t, "conflict state", func() bool {
		return integrationstatus.WhatsAppStatus().State == integrationstatus.WhatsAppStateConflict
	})
	time.Sleep(50 * time.Millisecond)
	if n := connectsOf(client); n != 6 {
		t.Fatalf("a terminal state kept reconnecting: %d connects", n)
	}
}

func TestBotMatchesConfigAndCancelsOnChange(t *testing.T) {
	cfg := testBotConfig(config.WhatsAppModeDedicated, "+491701234567")
	b := startTestBot(t, cfg, &fakeStore{client: &fakeClient{}}, &recordedSleeps{})
	if !b.MatchesConfig(cfg.Clone()) {
		t.Fatal("the starting config does not match")
	}
	b.CancelIfConfigChanged(cfg.Clone())
	if b.ctx.Err() != nil {
		t.Fatal("an unchanged config cancelled the generation")
	}
	changed := cfg.Clone()
	changed.WhatsApp.SelfChatPrefix = "AI: "
	b.CancelIfConfigChanged(changed)
	if b.ctx.Err() == nil || b.MatchesConfig(cfg) {
		t.Fatal("a changed config did not revoke the generation")
	}
}

func TestBotSendTestMessageReachesOnlyOwners(t *testing.T) {
	ctx := context.Background()
	if _, err := ownerRecipients(config.WhatsAppConfig{Mode: config.WhatsAppModeSelfChat}, types.EmptyJID); !errors.Is(err, ErrNoRecipient) {
		t.Fatalf("self chat without own JID: %v", err)
	}
	if _, err := ownerRecipients(config.WhatsAppConfig{Mode: config.WhatsAppModeDedicated}, testOwn); !errors.Is(err, ErrNoRecipient) {
		t.Fatalf("dedicated without owners: %v", err)
	}
	offline := &fakeClient{own: testOwn}
	b := startTestBot(t, testBotConfig(config.WhatsAppModeDedicated, "+491701234567"), &fakeStore{linked: true, own: testOwn, client: offline}, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(offline) == 1 })
	if err := b.SendTestMessage(ctx, fixedNow()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("logged-out send = %v, want ErrNotConnected", err)
	}
	want := "AuraGo WhatsApp test (manual) — 2026-10-31T12:00:00Z"
	dedicated := &fakeClient{own: testOwn, loggedIn: true}
	b = startTestBot(t, testBotConfig(config.WhatsAppModeDedicated, "+491701234567", "+436601234567"), &fakeStore{linked: true, own: testOwn, client: dedicated}, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(dedicated) == 1 })
	if err := b.SendTestMessage(ctx, fixedNow()); err != nil {
		t.Fatalf("dedicated send: %v", err)
	}
	var sent []sentMessage
	dedicated.with(func(f *fakeClient) { sent = slices.Clone(f.sent) })
	if len(sent) != 2 || sent[0].to.String() != "491701234567@s.whatsapp.net" || sent[1].to.String() != "436601234567@s.whatsapp.net" || sent[0].text != want {
		t.Fatalf("dedicated sends = %+v", sent)
	}
	self := &fakeClient{own: testOwn, loggedIn: true}
	b = startTestBot(t, testBotConfig(config.WhatsAppModeSelfChat), &fakeStore{linked: true, own: testOwn, client: self}, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(self) == 1 })
	if err := b.SendTestMessage(ctx, fixedNow()); err != nil {
		t.Fatalf("self-chat send: %v", err)
	}
	self.with(func(f *fakeClient) { sent = slices.Clone(f.sent) })
	if len(sent) != 1 || sent[0].to != testOwn || sent[0].text != config.DefaultWhatsAppSelfChatPrefix+want {
		t.Fatalf("self-chat sends = %+v", sent)
	}
	if integrationstatus.WhatsAppStatus().LastTestAt != "2026-10-31T12:00:00Z" {
		t.Fatal("the test send was not recorded")
	}
}

func TestBotLogoutPrefersRemoteAndFallsBackToLocal(t *testing.T) {
	ctx := context.Background()
	remote := &fakeClient{own: testOwn, loggedIn: true}
	remoteStore := &fakeStore{linked: true, own: testOwn, client: remote}
	b := startTestBot(t, testBotConfig(config.WhatsAppModeSelfChat), remoteStore, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(remote) == 1 })
	result, err := b.Logout(ctx)
	remoteStore.mu.Lock()
	deleted := remoteStore.deleted
	remoteStore.mu.Unlock()
	if err != nil || !result.RemoteRemoved || clientCount(remote, func(f *fakeClient) int { return f.logouts }) != 1 || deleted != 0 || b.ctx.Err() == nil {
		t.Fatalf("remote logout: result=%+v err=%v deleted=%d", result, err, deleted)
	}
	local := &fakeClient{own: testOwn, loggedIn: true, logoutErr: errors.New("websocket not connected")}
	localStore := &fakeStore{linked: true, own: testOwn, client: local}
	b = startTestBot(t, testBotConfig(config.WhatsAppModeSelfChat), localStore, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(local) == 1 })
	result, err = b.Logout(ctx)
	localStore.mu.Lock()
	deleted = localStore.deleted
	localStore.mu.Unlock()
	if err != nil || result.RemoteRemoved || deleted != 1 {
		t.Fatalf("local fallback: result=%+v err=%v deleted=%d", result, err, deleted)
	}
	if s := integrationstatus.WhatsAppStatus(); s.Linked || s.State != integrationstatus.WhatsAppStateNotLinked {
		t.Fatalf("status after logout = %+v", s)
	}
}
```

- [ ] **Step 2: Run the tests and confirm they fail**

```bash
CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestStartSkips|TestBot)' -count=1
```
Expected: FAIL with a build error, `undefined: start` (also `runtimeDeps`, `ownerRecipients`, `Bot`).

- [ ] **Step 3: Implement**

`internal/whatsapp/recipients.go`:
```go
package whatsapp

import (
	"strings"
	"time"

	"aurago/internal/config"

	"go.mau.fi/whatsmeow/types"
)

// ownerRecipients is the single owner-recipient resolver. Every send goes to
// the configured owners (dedicated mode) or to the linked account itself
// (self_chat mode). No API accepts arbitrary numbers.
func ownerRecipients(cfg config.WhatsAppConfig, own types.JID) ([]types.JID, error) {
	if cfg.Mode == config.WhatsAppModeSelfChat {
		if own.IsEmpty() {
			return nil, ErrNoRecipient
		}
		return []types.JID{own.ToNonAD()}, nil
	}
	out := make([]types.JID, 0, len(cfg.OwnerNumbers))
	for _, raw := range cfg.OwnerNumbers {
		number, err := config.NormalizeE164(raw)
		if err != nil {
			continue
		}
		out = append(out, types.NewJID(strings.TrimPrefix(number, "+"), types.DefaultUserServer))
	}
	if len(out) == 0 {
		return nil, ErrNoRecipient
	}
	return out, nil
}

// testMessageText is the manual test message. In self_chat mode it carries
// the prefix so it stands out from the owner's own notes.
func testMessageText(cfg config.WhatsAppConfig, now time.Time) string {
	text := "AuraGo WhatsApp test (manual) — " + now.UTC().Format(time.RFC3339)
	if cfg.Mode == config.WhatsAppModeSelfChat {
		return cfg.SelfChatPrefix + text
	}
	return text
}
```

`internal/whatsapp/runtime.go`:
```go
package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/security"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// Connect backoff for startup failures and for disconnects whatsmeow does not
// recover by itself. It never gives up.
const (
	minBackoff = 5 * time.Second
	maxBackoff = 5 * time.Minute
)

func nextBackoff(d time.Duration) time.Duration {
	if d*2 > maxBackoff {
		return maxBackoff
	}
	return d * 2
}

// runtimeDeps are the Bot's replaceable collaborators.
type runtimeDeps struct {
	backend backend
	sleep   func(ctx context.Context, d time.Duration) bool
	now     func() time.Time
}

func defaultRuntimeDeps(logger *slog.Logger) runtimeDeps {
	return runtimeDeps{backend: sqliteBackend{log: newWALogger(logger, "whatsmeow")}, sleep: sleepContext, now: time.Now}
}

// sleepContext waits for d. It returns false if ctx ends first.
func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// LogoutResult says whether WhatsApp also removed the linked device from the
// phone. false means only the local keys were deleted.
type LogoutResult struct {
	RemoteRemoved bool `json:"remote_removed"`
}

// ConfigState returns "disabled" or "not_configured" when the configuration
// does not allow a Bot, and "" when it does. It does not check the device
// store.
func ConfigState(cfg *config.Config) string {
	switch {
	case cfg == nil || cfg.EggMode.Enabled || !cfg.WhatsApp.Enabled:
		return integrationstatus.WhatsAppStateDisabled
	case strings.TrimSpace(cfg.Directories.DataDir) == "":
		return integrationstatus.WhatsAppStateNotConfigured
	case cfg.WhatsApp.Mode != config.WhatsAppModeSelfChat && len(cfg.WhatsApp.OwnerNumbers) == 0:
		return integrationstatus.WhatsAppStateNotConfigured
	}
	return ""
}

func configMode(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.WhatsApp.Mode
}

// Bot is one WhatsApp connection generation. The server owns at most one per
// process. A config change cancels it, and the next generation starts without
// relinking.
type Bot struct {
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	initial config.WhatsAppConfig
	dataDir string
	logger  *slog.Logger
	deps    runtimeDeps

	mu     sync.Mutex
	client waClient
	synced atomic.Bool
}

// Start returns a running generation, or nil when the configuration does not
// allow one; the status then says why. The device store opens asynchronously.
func Start(parent context.Context, cfg *config.Config, logger *slog.Logger) *Bot {
	if logger == nil {
		logger = slog.Default()
	}
	return start(parent, cfg, logger, defaultRuntimeDeps(logger))
}

func start(parent context.Context, cfg *config.Config, logger *slog.Logger, deps runtimeDeps) *Bot {
	if state := ConfigState(cfg); state != "" {
		integrationstatus.SetWhatsAppConfigState(state, configMode(cfg))
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}
	if parent.Err() != nil {
		return nil
	}
	cfg = cfg.Clone()
	ctx, cancel := context.WithCancel(parent)
	b := &Bot{ctx: ctx, cancel: cancel, done: make(chan struct{}), initial: cfg.WhatsApp,
		dataDir: cfg.Directories.DataDir, logger: logger, deps: deps}
	integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateConnecting, cfg.WhatsApp.Mode)
	go b.run()
	return b
}

// MatchesConfig reports whether this live generation still serves cfg.
func (b *Bot) MatchesConfig(cfg *config.Config) bool {
	return b != nil && b.ctx.Err() == nil && cfg != nil && !cfg.EggMode.Enabled &&
		b.dataDir == cfg.Directories.DataDir && b.initial.Equal(cfg.WhatsApp)
}

// CancelIfConfigChanged revokes the generation when a config is published.
// Stop drains it after the server releases its config lock.
func (b *Bot) CancelIfConfigChanged(cfg *config.Config) {
	if b != nil && !b.MatchesConfig(cfg) {
		b.cancel()
	}
}

// Stop cancels the generation and waits until it has disconnected and closed
// the store. It is idempotent.
func (b *Bot) Stop() {
	if b != nil {
		b.cancel()
		<-b.done
	}
}

func (b *Bot) setClient(c waClient) {
	b.mu.Lock()
	b.client = c
	b.mu.Unlock()
}

func (b *Bot) currentClient() waClient {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.client
}

func (b *Bot) run() {
	defer close(b.done)
	store, err := b.deps.backend.open(b.ctx, b.dataDir)
	if err != nil {
		if b.ctx.Err() == nil {
			integrationstatus.MarkWhatsAppError(integrationstatus.WhatsAppStateDegraded, "store_unavailable")
			b.logger.Error("WhatsApp device store unavailable", "error", maskDigits(security.Scrub(err.Error())))
		}
		return
	}
	defer store.Close()
	if b.ctx.Err() != nil {
		return
	}
	integrationstatus.SetWhatsAppLinked(store.Linked(), maskJID(store.OwnJID()))
	if !store.Linked() {
		integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateNotLinked)
		return
	}
	client := store.NewClient()
	signals := make(chan eventEffect, 16)
	client.AddEventHandler(func(evt any) { b.handleEvent(client, evt, signals) })
	b.setClient(client)
	defer func() {
		b.setClient(nil)
		client.Disconnect()
	}()
	b.connectLoop(client, signals)
}

// connectLoop keeps one connection open until the Bot stops. Connect errors,
// and disconnects that whatsmeow does not recover, retry with backoff.
// Terminal states wait for the user; a temporary ban waits until it expires.
func (b *Bot) connectLoop(client waClient, signals <-chan eventEffect) {
	backoff := minBackoff
	for b.ctx.Err() == nil {
		integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateConnecting)
		if err := client.ConnectContext(b.ctx); err != nil {
			if b.ctx.Err() != nil {
				return
			}
			integrationstatus.MarkWhatsAppError(integrationstatus.WhatsAppStateDegraded, "connect_failed")
			b.logger.Warn("WhatsApp connect failed", "retry_in", backoff.String(), "error", maskDigits(security.Scrub(err.Error())))
			if !b.deps.sleep(b.ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}
		eff, ok := b.awaitInterruption(signals, &backoff)
		if !ok {
			return
		}
		client.Disconnect()
		switch {
		case eff.Terminal && eff.RetryAfter > 0:
			if !b.deps.sleep(b.ctx, eff.RetryAfter) {
				return
			}
			backoff = minBackoff
		case eff.Terminal:
			<-b.ctx.Done()
			return
		default:
			if !b.deps.sleep(b.ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
		}
	}
}

// awaitInterruption waits on an open connection until the Bot stops (false)
// or an event needs the loop. A healthy connection resets the backoff.
func (b *Bot) awaitInterruption(signals <-chan eventEffect, backoff *time.Duration) (eventEffect, bool) {
	for {
		select {
		case <-b.ctx.Done():
			return eventEffect{}, false
		case eff := <-signals:
			if eff.Healthy {
				*backoff = minBackoff
			}
			if eff.Terminal || eff.Reconnect {
				return eff, true
			}
		}
	}
}

// handleEvent runs on whatsmeow's dispatcher and must not block. A stopped
// generation never overwrites its successor's status.
func (b *Bot) handleEvent(client waClient, evt any, signals chan<- eventEffect) {
	if b.ctx.Err() != nil {
		return
	}
	eff := classifyEvent(evt, b.synced.Load())
	now := b.deps.now()
	if eff.Connected {
		b.synced.Store(false)
		integrationstatus.MarkWhatsAppConnected(now)
	}
	if eff.Synced {
		b.synced.Store(true)
	}
	switch {
	case eff.State == integrationstatus.WhatsAppStateTempBanned:
		var expires time.Time
		if eff.RetryAfter > 0 {
			expires = now.Add(eff.RetryAfter)
		}
		integrationstatus.MarkWhatsAppTempBan(eff.ErrorCode, expires)
	case eff.ErrorCode != "":
		integrationstatus.MarkWhatsAppError(eff.State, eff.ErrorCode)
	case eff.State != "":
		integrationstatus.SetWhatsAppState(eff.State)
	}
	if eff.State == integrationstatus.WhatsAppStateLoggedOut {
		integrationstatus.SetWhatsAppLinked(false, "")
	}
	if eff.Message {
		integrationstatus.CountWhatsAppMessage(now)
	}
	if eff.SendPresence {
		go b.sendUnavailable(client)
	}
	if eff.Healthy || eff.Terminal || eff.Reconnect {
		select {
		case signals <- eff:
		default:
			b.logger.Warn("WhatsApp event signal dropped", "state", eff.State)
		}
	}
}

// sendUnavailable keeps AuraGo "offline" so the phone still shows
// notifications. If no push name is known yet, PushNameSetting retries it.
func (b *Bot) sendUnavailable(client waClient) {
	ctx, cancel := context.WithTimeout(b.ctx, 15*time.Second)
	defer cancel()
	err := client.SendPresence(ctx, types.PresenceUnavailable)
	if err != nil && !errors.Is(err, whatsmeow.ErrNoPushName) && ctx.Err() == nil {
		b.logger.Warn("WhatsApp presence update failed", "error", maskDigits(security.Scrub(err.Error())))
	}
}

// SendTestMessage sends the manual test message to the owners only. Stage 2
// moves it onto the serialized send queue.
func (b *Bot) SendTestMessage(ctx context.Context, now time.Time) error {
	client := b.currentClient()
	if client == nil || !client.IsLoggedIn() {
		return ErrNotConnected
	}
	recipients, err := ownerRecipients(b.initial, client.OwnJID())
	if err != nil {
		return err
	}
	text := testMessageText(b.initial, now)
	var failed error
	for _, to := range recipients {
		if _, err := client.SendMessage(ctx, to, &waE2E.Message{Conversation: proto.String(text)}); err != nil {
			integrationstatus.MarkWhatsAppDeliveryFailure()
			failed = fmt.Errorf("send whatsapp test message: %w", err)
			continue
		}
		integrationstatus.MarkWhatsAppOutbound(now)
	}
	if failed != nil {
		return failed
	}
	integrationstatus.MarkWhatsAppTest(now)
	return nil
}

// Logout unlinks the device. With a live connection WhatsApp also removes it
// from the phone, and whatsmeow deletes the local keys. Otherwise only the
// local keys are deleted, and the result says so. The Bot is stopped either
// way.
func (b *Bot) Logout(ctx context.Context) (LogoutResult, error) {
	remote := false
	if client := b.currentClient(); client != nil && client.IsLoggedIn() {
		if err := client.Logout(ctx); err != nil {
			b.logger.Warn("WhatsApp remote unlink failed; removing the local device only", "error", maskDigits(security.Scrub(err.Error())))
		} else {
			remote = true
		}
	}
	b.Stop()
	if !remote {
		if err := removeLocalDevice(ctx, b.deps.backend, b.dataDir); err != nil {
			return LogoutResult{}, err
		}
	}
	integrationstatus.SetWhatsAppLinked(false, "")
	integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateNotLinked)
	return LogoutResult{RemoteRemoved: remote}, nil
}

// removeLocalDevice deletes the linked device's keys without contacting
// WhatsApp.
func removeLocalDevice(ctx context.Context, b backend, dataDir string) error {
	store, err := b.open(ctx, dataDir)
	if err != nil {
		return fmt.Errorf("open whatsapp device store: %w", err)
	}
	defer store.Close()
	return store.DeleteLocal(ctx)
}
```

- [ ] **Step 4: Run the tests and confirm they pass; add the race run where available**

```bash
CGO_ENABLED=0 go test ./internal/whatsapp/ -count=1
go test -race ./internal/whatsapp/ -count=1
```
Expected: PASS. `-race` needs CGO. If it fails here with "requires cgo", note it; the Linux Security Gates run it.

- [ ] **Step 5: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="internal/whatsapp/runtime.go internal/whatsapp/recipients.go internal/whatsapp/runtime_test.go"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "feat(whatsapp): Bot runtime with backoff, health, test message and logout" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```

---

### Task 8: Link sessions and QR rendering

**Files:**
- Create: `internal/whatsapp/link.go`, `internal/whatsapp/qr.go`
- Create (test): `internal/whatsapp/link_test.go`

This task adds one method beyond the summary names: `(LinkRequest) Validate() error`. The server calls it before any destructive step (Task 9).

- [ ] **Step 1: Write the failing test** `internal/whatsapp/link_test.go`

```go
package whatsapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image/png"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"

	"go.mau.fi/whatsmeow"
)

func fakeLinkDeps(st *fakeStore) linkDeps {
	return linkDeps{backend: &fakeBackend{store: st}, now: fixedNow, render: func(_ context.Context, payload string) (string, error) {
		return "data:image/png;base64," + payload, nil
	}}
}

func startTestLink(t *testing.T, req LinkRequest, st *fakeStore) (*LinkSession, chan bool) {
	t.Helper()
	finished := make(chan bool, 1)
	l, err := startLink(context.Background(), testBotConfig(config.WhatsAppModeSelfChat), req, quietLogger(), func(linked bool) { finished <- linked }, fakeLinkDeps(st))
	if err != nil {
		t.Fatalf("startLink: %v", err)
	}
	t.Cleanup(func() { l.Cancel(); <-l.Done() })
	return l, finished
}

func waitLinkState(t *testing.T, l *LinkSession, state string) {
	t.Helper()
	waitFor(t, "link state "+state, func() bool { return l.Snapshot().State == state })
}

func waitFinished(t *testing.T, finished <-chan bool) bool {
	t.Helper()
	select {
	case linked := <-finished:
		return linked
	case <-time.After(5 * time.Second):
		t.Fatal("link session did not finish")
		return false
	}
}

func TestStartLinkRejectsInvalidRequests(t *testing.T) {
	cfg := testBotConfig(config.WhatsAppModeSelfChat)
	disabled := cfg.Clone()
	disabled.WhatsApp.Enabled = false
	unlinked := &fakeStore{client: &fakeClient{}}
	for _, c := range []struct {
		cfg  *config.Config
		req  LinkRequest
		want error
	}{
		{disabled, LinkRequest{Method: LinkMethodQR}, ErrNotConfigured},
		{cfg, LinkRequest{Method: "sms"}, ErrInvalidLinkMethod},
		{cfg, LinkRequest{Method: LinkMethodPhone, Phone: "01701234567"}, ErrInvalidPhone},
	} {
		if _, err := startLink(context.Background(), c.cfg, c.req, quietLogger(), nil, fakeLinkDeps(unlinked)); !errors.Is(err, c.want) {
			t.Errorf("startLink(%+v) = %v, want %v", c.req, err, c.want)
		}
	}
	linked := &fakeStore{linked: true, own: testOwn, client: &fakeClient{}}
	if _, err := startLink(context.Background(), cfg, LinkRequest{Method: LinkMethodQR}, quietLogger(), nil, fakeLinkDeps(linked)); !errors.Is(err, ErrAlreadyLinked) {
		t.Fatalf("linked store: %v, want ErrAlreadyLinked", err)
	}
	linked.mu.Lock()
	closed := linked.closed
	linked.mu.Unlock()
	if closed != 1 {
		t.Fatalf("store closed %d times after ErrAlreadyLinked, want 1", closed)
	}
}

func TestLinkSessionQRSuccessHandsOverTheDevice(t *testing.T) {
	client := &fakeClient{qr: make(chan whatsmeow.QRChannelItem, 4)}
	st := &fakeStore{client: client}
	l, finished := startTestLink(t, LinkRequest{Method: LinkMethodQR}, st)
	if snap := l.Snapshot(); snap.State != LinkStateWaiting || snap.ExpiresAt != "2026-10-31T12:03:00Z" {
		t.Fatalf("initial snapshot = %+v", snap)
	}
	if got := integrationstatus.WhatsAppStatus().State; got != integrationstatus.WhatsAppStateLinking {
		t.Fatalf("status = %q, want linking", got)
	}
	client.qr <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "2@fake-ref,fake-keys", Timeout: time.Minute}
	waitLinkState(t, l, LinkStateQR)
	if snap := l.Snapshot(); snap.QRPNG != "data:image/png;base64,2@fake-ref,fake-keys" || snap.ExpiresAt != "2026-10-31T12:03:00Z" {
		t.Fatalf("qr snapshot = %+v", snap)
	}
	client.qr <- whatsmeow.QRChannelSuccess
	if !waitFinished(t, finished) {
		t.Fatal("success reported as not linked")
	}
	<-l.Done()
	st.mu.Lock()
	closed := st.closed
	st.mu.Unlock()
	if l.Active() || l.Snapshot().State != LinkStateSuccess || clientCount(client, func(f *fakeClient) int { return f.disconnects }) == 0 || closed != 1 {
		t.Fatalf("the link session still holds the device: snapshot=%+v closed=%d", l.Snapshot(), closed)
	}
}

func TestLinkSessionPhoneCodeTimeoutPasskeyAndCancel(t *testing.T) {
	client := &fakeClient{qr: make(chan whatsmeow.QRChannelItem, 4), pairCode: "ABCD-EFGH"}
	l, finished := startTestLink(t, LinkRequest{Method: LinkMethodPhone, Phone: "+49 170 1234567"}, &fakeStore{client: client})
	client.qr <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "2@first"}
	waitLinkState(t, l, LinkStateCode)
	if snap := l.Snapshot(); snap.PairingCode != "ABCD-EFGH" || snap.QRPNG != "" {
		t.Fatalf("code snapshot = %+v", snap)
	}
	client.qr <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "2@second"}
	client.qr <- whatsmeow.QRChannelTimeout
	if waitFinished(t, finished) {
		t.Fatal("a timeout reported a linked device")
	}
	var phones []string
	client.with(func(f *fakeClient) { phones = slices.Clone(f.pairPhones) })
	if l.Snapshot().State != LinkStateTimeout || !slices.Equal(phones, []string{"491701234567"}) {
		t.Fatalf("snapshot=%+v pairPhones=%v", l.Snapshot(), phones)
	}
	if got := integrationstatus.WhatsAppStatus().State; got != integrationstatus.WhatsAppStateNotLinked {
		t.Fatalf("status after timeout = %q", got)
	}
	passkey := &fakeClient{qr: make(chan whatsmeow.QRChannelItem, 1)}
	l2, finished2 := startTestLink(t, LinkRequest{Method: LinkMethodQR}, &fakeStore{client: passkey})
	passkey.qr <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventPasskeyRequest}
	waitFinished(t, finished2)
	if snap := l2.Snapshot(); snap.State != LinkStateError || snap.Error != "passkey_unsupported" {
		t.Fatalf("passkey snapshot = %+v", snap)
	}
	idle := &fakeClient{qr: make(chan whatsmeow.QRChannelItem)}
	l3, finished3 := startTestLink(t, LinkRequest{Method: LinkMethodQR}, &fakeStore{client: idle})
	l3.Cancel()
	waitFinished(t, finished3)
	if snap := l3.Snapshot(); snap.State != LinkStateError || snap.Error != "cancelled" {
		t.Fatalf("cancelled snapshot = %+v", snap)
	}
}

func TestRenderQRDataURIProducesVerifiedPNG(t *testing.T) {
	payload := "2@" + strings.Repeat("Ab3+/", 30) + "," + strings.Repeat("Zx9=", 20)
	uri, err := renderQRDataURI(context.Background(), payload)
	if err != nil {
		t.Fatalf("renderQRDataURI: %v", err)
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(uri, prefix) {
		t.Fatalf("uri prefix = %q", uri[:min(len(uri), 32)])
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, prefix))
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width != linkQRSize || cfg.Height != linkQRSize {
		t.Fatalf("png = %+v, %v", cfg, err)
	}
}
```

- [ ] **Step 2: Run the tests and confirm they fail**

```bash
CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestStartLink|TestLinkSession|TestRenderQR)' -count=1
```
Expected: FAIL, `undefined: linkDeps`, `undefined: startLink`, `undefined: renderQRDataURI`.

- [ ] **Step 3: Implement**

`internal/whatsapp/qr.go`:
```go
package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"

	"aurago/internal/qrcode"
)

const linkQRSize = 384

// renderQRDataURI renders a WhatsApp link code as a PNG data URI that
// internal/qrcode has verified. The UI shows it like the TOTP enrollment QR.
// The payload is never logged.
func renderQRDataURI(ctx context.Context, payload string) (string, error) {
	result, err := qrcode.Render(ctx, qrcode.QRSpec{
		SchemaVersion: qrcode.SchemaVersion,
		Content:       qrcode.Content{Type: "text", Text: payload},
		Style:         qrcode.Style{Size: linkQRSize},
		Output:        qrcode.Output{Format: "png"},
	})
	if err != nil {
		return "", fmt.Errorf("render whatsapp link qr: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(result.Data), nil
}
```

`internal/whatsapp/link.go`:
```go
package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/security"

	"go.mau.fi/whatsmeow"
)

// Link methods.
const (
	LinkMethodQR    = "qr"
	LinkMethodPhone = "phone"
)

// Link session states. idle means this process has no session.
const (
	LinkStateIdle    = "idle"
	LinkStateWaiting = "waiting"
	LinkStateQR      = "qr"
	LinkStateCode    = "code"
	LinkStateSuccess = "success"
	LinkStateTimeout = "timeout"
	LinkStateError   = "error"
)

const linkTTL = 3 * time.Minute

// LinkRequest starts a link session, either by QR code or by phone pairing
// code.
type LinkRequest struct {
	Method string `json:"method"`
	Phone  string `json:"phone,omitempty"`
}

// Validate checks the method and, for phone linking, the number.
func (r LinkRequest) Validate() error {
	switch strings.ToLower(strings.TrimSpace(r.Method)) {
	case LinkMethodQR:
		return nil
	case LinkMethodPhone:
		if _, err := config.NormalizeE164(r.Phone); err != nil {
			return ErrInvalidPhone
		}
		return nil
	}
	return ErrInvalidLinkMethod
}

// LinkSnapshot is what GET /api/whatsapp/link returns. Error is a fixed code.
type LinkSnapshot struct {
	State       string `json:"state"`
	QRPNG       string `json:"qr_png,omitempty"`
	PairingCode string `json:"pairing_code,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	Error       string `json:"error,omitempty"`
}

type linkDeps struct {
	backend backend
	now     func() time.Time
	render  func(ctx context.Context, payload string) (string, error)
}

func defaultLinkDeps(logger *slog.Logger) linkDeps {
	return linkDeps{backend: sqliteBackend{log: newWALogger(logger, "whatsmeow")}, now: time.Now, render: renderQRDataURI}
}

// LinkSession owns a temporary client on an unlinked store for at most
// linkTTL. On success it disconnects and closes the store before onFinish
// runs, so the next Bot is the only user of the new device.
type LinkSession struct {
	method string
	phone  string
	cancel context.CancelFunc
	done   chan struct{}

	mu       sync.Mutex
	snap     LinkSnapshot
	finished bool
}

// StartLink opens the store, refuses an already linked device, and links in
// the background. onFinish(linked) runs once, on the session goroutine, after
// cleanup. It must not block.
func StartLink(parent context.Context, cfg *config.Config, req LinkRequest, logger *slog.Logger, onFinish func(linked bool)) (*LinkSession, error) {
	if logger == nil {
		logger = slog.Default()
	}
	return startLink(parent, cfg, req, logger, onFinish, defaultLinkDeps(logger))
}

func startLink(parent context.Context, cfg *config.Config, req LinkRequest, logger *slog.Logger, onFinish func(bool), deps linkDeps) (*LinkSession, error) {
	if ConfigState(cfg) != "" {
		return nil, ErrNotConfigured
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	method := strings.ToLower(strings.TrimSpace(req.Method))
	phone := ""
	if method == LinkMethodPhone {
		number, _ := config.NormalizeE164(req.Phone)
		phone = strings.TrimPrefix(number, "+")
	}
	if parent == nil {
		parent = context.Background()
	}
	store, err := deps.backend.open(parent, cfg.Directories.DataDir)
	if err != nil {
		return nil, fmt.Errorf("open whatsapp device store: %w", err)
	}
	if store.Linked() {
		_ = store.Close()
		return nil, ErrAlreadyLinked
	}
	ctx, cancel := context.WithTimeout(parent, linkTTL)
	l := &LinkSession{method: method, phone: phone, cancel: cancel, done: make(chan struct{}),
		snap: LinkSnapshot{State: LinkStateWaiting, ExpiresAt: deps.now().Add(linkTTL).UTC().Format(time.RFC3339)}}
	integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateLinking)
	go l.run(ctx, store, deps, logger, onFinish)
	return l, nil
}

func (l *LinkSession) run(ctx context.Context, store deviceStore, deps linkDeps, logger *slog.Logger, onFinish func(bool)) {
	client := store.NewClient()
	linked := false
	defer func() {
		// Cancel first: whatsmeow's 515 reconnect after pairing uses ctx.
		l.cancel()
		client.Disconnect()
		_ = store.Close()
		l.mu.Lock()
		l.finished = true
		l.mu.Unlock()
		if !linked {
			integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateNotLinked)
		}
		if onFinish != nil {
			onFinish(linked)
		}
		close(l.done)
	}()
	items, err := client.GetQRChannel(ctx)
	if err != nil {
		l.fail(logger, "link_failed", err)
		return
	}
	if err := client.ConnectContext(ctx); err != nil {
		l.fail(logger, "connect_failed", err)
		return
	}
	codeRequested := false
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				l.set(LinkSnapshot{State: LinkStateTimeout})
			} else {
				l.set(LinkSnapshot{State: LinkStateError, Error: "cancelled"})
			}
			return
		case item, ok := <-items:
			if !ok {
				l.set(LinkSnapshot{State: LinkStateTimeout})
				return
			}
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				if l.method == LinkMethodPhone {
					// PairPhone needs the connection whose first QR event just arrived.
					if codeRequested {
						continue
					}
					codeRequested = true
					code, err := client.PairPhone(ctx, l.phone)
					if err != nil {
						l.fail(logger, "pair_failed", err)
						return
					}
					l.set(LinkSnapshot{State: LinkStateCode, PairingCode: code})
					continue
				}
				uri, err := deps.render(ctx, item.Code)
				if err != nil {
					l.fail(logger, "qr_failed", err)
					return
				}
				l.set(LinkSnapshot{State: LinkStateQR, QRPNG: uri})
			case whatsmeow.QRChannelSuccess.Event:
				linked = true
				l.set(LinkSnapshot{State: LinkStateSuccess})
				return
			case whatsmeow.QRChannelTimeout.Event:
				l.set(LinkSnapshot{State: LinkStateTimeout})
				return
			case whatsmeow.QRChannelScannedWithoutMultidevice.Event:
				// The phone can scan the same code again after enabling linked devices.
				continue
			case whatsmeow.QRChannelEventPasskeyRequest, whatsmeow.QRChannelEventPasskeyResponse:
				l.set(LinkSnapshot{State: LinkStateError, Error: "passkey_unsupported"})
				return
			case whatsmeow.QRChannelClientOutdated.Event:
				l.set(LinkSnapshot{State: LinkStateError, Error: "outdated"})
				return
			default:
				l.fail(logger, "pair_failed", item.Error)
				return
			}
		}
	}
}

// set replaces the snapshot but keeps the session expiry.
func (l *LinkSession) set(next LinkSnapshot) {
	l.mu.Lock()
	defer l.mu.Unlock()
	next.ExpiresAt = l.snap.ExpiresAt
	l.snap = next
}

func (l *LinkSession) fail(logger *slog.Logger, code string, err error) {
	if err != nil {
		logger.Warn("WhatsApp link failed", "code", code, "error", maskDigits(security.Scrub(err.Error())))
	}
	l.set(LinkSnapshot{State: LinkStateError, Error: code})
}

// Snapshot returns a copy of the current state.
func (l *LinkSession) Snapshot() LinkSnapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.snap
}

// Active reports whether the session still owns the device store.
func (l *LinkSession) Active() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return !l.finished
}

// Cancel ends the session. Wait on Done for the cleanup.
func (l *LinkSession) Cancel() { l.cancel() }

// Done is closed after cleanup and onFinish.
func (l *LinkSession) Done() <-chan struct{} { return l.done }

// DeviceLinked reports whether <dataDir>/whatsapp/device.db holds a linked
// device.
func DeviceLinked(ctx context.Context, dataDir string) (bool, error) {
	store, err := (sqliteBackend{}).open(ctx, dataDir)
	if err != nil {
		return false, fmt.Errorf("open whatsapp device store: %w", err)
	}
	defer store.Close()
	return store.Linked(), nil
}

// RemoveLocalDevice deletes the linked device's keys without contacting
// WhatsApp. The owner then removes "AuraGo" from the phone's linked devices.
func RemoveLocalDevice(ctx context.Context, dataDir string) error {
	return removeLocalDevice(ctx, sqliteBackend{}, dataDir)
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

```bash
CGO_ENABLED=0 go test ./internal/whatsapp/ -count=1
CGO_ENABLED=0 go vet ./internal/whatsapp/
```
Expected: the tests PASS and vet prints nothing.

- [ ] **Step 5: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="internal/whatsapp/link.go internal/whatsapp/qr.go internal/whatsapp/link_test.go"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "feat(whatsapp): QR and pairing-code link sessions" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```

---

### Task 9: Server lifecycle wiring, config save validation and hot reload

**Files:**
- Create: `internal/server/whatsapp_runtime.go`
- Create (test): `internal/server/whatsapp_runtime_test.go`
- Modify: `internal/server/server.go`:
  - add the fields after `	telegramOffsets       telegram.UpdateOffsets` (≈406)
  - in `replaceConfigSnapshot`, add the revocation after the `s.telegramBot.Load()` block (≈637-639)
  - add the import
- Modify: `internal/server/server_routes.go` — in `(*Server).run`, after `	s.configureTelegramBot()` (≈690)
- Modify: `internal/server/server_shutdown.go` — in `closeRuntimeResources`, after `	s.stopTelegramBot()` (≈48)
- Modify: `internal/server/config_handlers_main.go`, in `handleUpdateConfig`:
  - save validation before `if err := config.ValidateDecisionsConfig(validateCfg.Decisions); err != nil {` (≈717)
  - the change flag after `telegramChanged := false` (≈979) and after `telegramChanged = oldCfg.Telegram != …` (≈1007)
  - the reload after the `if loadErr == nil && telegramChanged {` block (≈1714-1716)

- [ ] **Step 1: Impact analysis**

```bash
node .gitnexus/run.cjs impact "replaceConfigSnapshot" --direction upstream --repo .
node .gitnexus/run.cjs impact "closeRuntimeResources" --direction upstream --repo .
node .gitnexus/run.cjs impact "handleUpdateConfig" --direction upstream --repo .
node .gitnexus/run.cjs impact "run" --direction upstream --repo .
```
For `run`, use the method in `internal/server/server_routes.go`. Expected: `replaceConfigSnapshot`, `handleUpdateConfig` and `run` are HIGH or CRITICAL; report them. Every edit only adds lines, mirroring the Telegram lines next to it.

- [ ] **Step 2: Write the failing test** `internal/server/whatsapp_runtime_test.go`

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/security"
	"aurago/internal/whatsapp"
)

func waitWhatsAppState(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for integrationstatus.WhatsAppStatus().State != want {
		if time.Now().After(deadline) {
			t.Fatalf("WhatsApp state = %q, want %q", integrationstatus.WhatsAppStatus().State, want)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// Uses the real backend on an empty data dir. The store opens, no device is
// linked, and nothing touches the network.
func TestWhatsAppRuntimeGenerationsFollowConfigAndShutdown(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), integrationCtx: context.Background()}
	s.initConfigSnapshot()
	t.Cleanup(s.stopWhatsApp)
	publish := func(next *config.Config) { s.CfgMu.Lock(); s.replaceConfigSnapshot(next); s.CfgMu.Unlock() }

	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() != nil || integrationstatus.WhatsAppStatus().State != integrationstatus.WhatsAppStateDisabled {
		t.Fatal("a disabled section started a Bot")
	}
	next := cfg.Clone()
	next.WhatsApp.Enabled = true
	next.WhatsApp.Mode = config.WhatsAppModeDedicated
	publish(next)
	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() != nil || integrationstatus.WhatsAppStatus().State != integrationstatus.WhatsAppStateNotConfigured {
		t.Fatal("dedicated mode without owners started a Bot")
	}
	next = next.Clone()
	next.WhatsApp.OwnerNumbers = []string{"+491701234567"}
	publish(next)
	s.configureWhatsAppBot()
	first := s.whatsAppBot.Load()
	if first == nil {
		t.Fatal("a configured section started no Bot")
	}
	waitWhatsAppState(t, integrationstatus.WhatsAppStateNotLinked)
	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() != first {
		t.Fatal("unchanged settings started another generation")
	}
	changed := next.Clone()
	changed.WhatsApp.SelfChatPrefix = "AI: "
	publish(changed)
	if first.MatchesConfig(changed) {
		t.Fatal("publication did not revoke the old generation")
	}
	s.configureWhatsAppBot()
	if second := s.whatsAppBot.Load(); second == nil || second == first {
		t.Fatal("a changed section did not start a new generation")
	}
	egg := changed.Clone()
	egg.EggMode.Enabled = true
	publish(egg)
	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() != nil {
		t.Fatal("egg mode retained a Bot")
	}
	publish(changed)
	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() == nil {
		t.Fatal("leaving egg mode started no Bot")
	}
	s.stopWhatsApp()
	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() != nil {
		t.Fatal("shutdown allowed a new generation")
	}
}

func TestServerWhatsAppControllerGuards(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.WhatsApp.Enabled = true
	cfg.WhatsApp.Mode = config.WhatsAppModeSelfChat
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), integrationCtx: context.Background()}
	s.initConfigSnapshot()
	t.Cleanup(s.stopWhatsApp)
	ctl := serverWhatsApp{s: s}
	if _, err := ctl.Link(ctx, whatsapp.LinkRequest{Method: whatsapp.LinkMethodPhone, Phone: "0170"}, true); !errors.Is(err, whatsapp.ErrInvalidPhone) {
		t.Fatalf("invalid phone = %v, want ErrInvalidPhone before any unlink", err)
	}
	if snap := ctl.LinkSnapshot(); snap.State != whatsapp.LinkStateIdle {
		t.Fatalf("snapshot without session = %+v", snap)
	}
	result, err := ctl.Logout(ctx)
	if err != nil || result.RemoteRemoved {
		t.Fatalf("logout without device = %+v, %v", result, err)
	}
	if s.whatsAppBot.Load() == nil {
		t.Fatal("logout did not start a fresh generation")
	}
	if err := ctl.SendTest(ctx, time.Now()); !errors.Is(err, whatsapp.ErrNotConnected) {
		t.Fatalf("test send without connection = %v", err)
	}
}

func TestHandleUpdateConfigWhatsAppValidatesAndHotReloads(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	body := fmt.Sprintf("directories:\n  data_dir: %q\nwhatsapp:\n  enabled: false\n", filepath.ToSlash(filepath.Join(root, "data")))
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("01", 32), filepath.Join(root, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Vault: vault}
	defer s.stopWhatsApp()
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"whatsapp":{"owner_numbers":["01701234567"]}}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "whatsapp.owner_numbers[0]") {
		t.Fatalf("invalid number: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"whatsapp":{"enabled":true,"mode":"self_chat"}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["needs_restart"] == true {
		t.Fatalf("WhatsApp required a restart: %s", rec.Body.String())
	}
	if s.ConfigSnapshot().WhatsApp.Mode != config.WhatsAppModeSelfChat || s.whatsAppBot.Load() == nil {
		t.Fatal("the WhatsApp change was not hot-reloaded")
	}
}
```

- [ ] **Step 3: Run the tests and confirm they fail**

```bash
CGO_ENABLED=0 go test ./internal/server/ -run '(TestWhatsAppRuntime|TestServerWhatsAppController|TestHandleUpdateConfigWhatsApp)' -count=1
```
Expected: FAIL, `s.configureWhatsAppBot undefined`, `undefined: serverWhatsApp`.

- [ ] **Step 4: Implement** `internal/server/whatsapp_runtime.go`

```go
package server

import (
	"context"
	"strings"
	"time"

	"aurago/internal/integrationstatus"
	"aurago/internal/whatsapp"
)

// whatsAppController is what the WhatsApp HTTP handlers need from the
// runtime. serverWhatsApp is the production implementation. Handler tests
// set Server.whatsAppCtl to a fake.
type whatsAppController interface {
	Link(ctx context.Context, req whatsapp.LinkRequest, force bool) (whatsapp.LinkSnapshot, error)
	LinkSnapshot() whatsapp.LinkSnapshot
	Logout(ctx context.Context) (whatsapp.LogoutResult, error)
	SendTest(ctx context.Context, now time.Time) error
}

func (s *Server) whatsApp() whatsAppController {
	if s.whatsAppCtl != nil {
		return s.whatsAppCtl
	}
	return serverWhatsApp{s: s}
}

// configureWhatsAppBot publishes the Bot generation for the current config.
func (s *Server) configureWhatsAppBot() {
	s.whatsAppLifecycleMu.Lock()
	defer s.whatsAppLifecycleMu.Unlock()
	s.configureWhatsAppBotLocked(false)
}

// configureWhatsAppBotLocked replaces the generation unless it still matches.
// force replaces it anyway, because a newly linked device needs a fresh
// connection. While a link session owns the device, no Bot starts. The
// caller holds whatsAppLifecycleMu.
func (s *Server) configureWhatsAppBotLocked(force bool) {
	if s.whatsAppClosed || (s.whatsAppLink != nil && s.whatsAppLink.Active()) {
		return
	}
	s.CfgMu.RLock()
	current := s.whatsAppBot.Load()
	unchanged := !force && current != nil && current.MatchesConfig(s.ConfigSnapshot())
	s.CfgMu.RUnlock()
	if unchanged {
		return
	}
	if previous := s.whatsAppBot.Swap(nil); previous != nil {
		previous.Stop()
	}
	// Publish under CfgMu so a concurrent config change cannot miss revoking it.
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	s.whatsAppBot.Store(whatsapp.Start(parent, s.ConfigSnapshot(), s.Logger))
}

// stopWhatsApp ends the link session and the Bot at shutdown. Nothing
// restarts afterwards.
func (s *Server) stopWhatsApp() {
	s.whatsAppLifecycleMu.Lock()
	defer s.whatsAppLifecycleMu.Unlock()
	s.whatsAppClosed = true
	s.cancelWhatsAppLinkLocked()
	if previous := s.whatsAppBot.Swap(nil); previous != nil {
		previous.Stop()
	}
}

func (s *Server) cancelWhatsAppLinkLocked() {
	if link := s.whatsAppLink; link != nil && link.Active() {
		link.Cancel()
		<-link.Done()
	}
}

// afterWhatsAppLink runs on its own goroutine when a link session ends.
func (s *Server) afterWhatsAppLink(linked bool) {
	s.whatsAppLifecycleMu.Lock()
	defer s.whatsAppLifecycleMu.Unlock()
	s.configureWhatsAppBotLocked(linked)
}

type serverWhatsApp struct{ s *Server }

// Link starts one link session. An existing device is refused unless force is
// set; then it is unlinked first, remotely when connected. The request is
// validated before anything is unlinked or stopped.
func (w serverWhatsApp) Link(ctx context.Context, req whatsapp.LinkRequest, force bool) (whatsapp.LinkSnapshot, error) {
	if err := req.Validate(); err != nil {
		return whatsapp.LinkSnapshot{}, err
	}
	s := w.s
	s.whatsAppLifecycleMu.Lock()
	defer s.whatsAppLifecycleMu.Unlock()
	cfg := s.ConfigSnapshot()
	if s.whatsAppClosed || whatsapp.ConfigState(cfg) != "" {
		return whatsapp.LinkSnapshot{}, whatsapp.ErrNotConfigured
	}
	if s.whatsAppLink != nil && s.whatsAppLink.Active() {
		return whatsapp.LinkSnapshot{}, whatsapp.ErrLinkInProgress
	}
	linked, err := whatsapp.DeviceLinked(ctx, cfg.Directories.DataDir)
	if err != nil {
		return whatsapp.LinkSnapshot{}, err
	}
	if linked && !force {
		return whatsapp.LinkSnapshot{}, whatsapp.ErrAlreadyLinked
	}
	if linked {
		if _, err := w.logoutLocked(ctx); err != nil {
			return whatsapp.LinkSnapshot{}, err
		}
	} else if previous := s.whatsAppBot.Swap(nil); previous != nil {
		previous.Stop()
	}
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	session, err := whatsapp.StartLink(parent, cfg, req, s.Logger, func(linked bool) { go s.afterWhatsAppLink(linked) })
	if err != nil {
		s.configureWhatsAppBotLocked(false)
		return whatsapp.LinkSnapshot{}, err
	}
	s.whatsAppLink = session
	return session.Snapshot(), nil
}

func (w serverWhatsApp) LinkSnapshot() whatsapp.LinkSnapshot {
	w.s.whatsAppLifecycleMu.Lock()
	defer w.s.whatsAppLifecycleMu.Unlock()
	if w.s.whatsAppLink == nil {
		return whatsapp.LinkSnapshot{State: whatsapp.LinkStateIdle}
	}
	return w.s.whatsAppLink.Snapshot()
}

// Logout unlinks the device and starts a fresh generation, which reports
// not_linked or disabled.
func (w serverWhatsApp) Logout(ctx context.Context) (whatsapp.LogoutResult, error) {
	s := w.s
	s.whatsAppLifecycleMu.Lock()
	defer s.whatsAppLifecycleMu.Unlock()
	if s.whatsAppClosed || strings.TrimSpace(s.ConfigSnapshot().Directories.DataDir) == "" {
		return whatsapp.LogoutResult{}, whatsapp.ErrNotConfigured
	}
	result, err := w.logoutLocked(ctx)
	s.configureWhatsAppBotLocked(true)
	return result, err
}

// logoutLocked uses the running Bot's connection when there is one.
// Otherwise it deletes the local keys only. The caller holds
// whatsAppLifecycleMu.
func (w serverWhatsApp) logoutLocked(ctx context.Context) (whatsapp.LogoutResult, error) {
	s := w.s
	s.cancelWhatsAppLinkLocked()
	if bot := s.whatsAppBot.Swap(nil); bot != nil {
		return bot.Logout(ctx)
	}
	if err := whatsapp.RemoveLocalDevice(ctx, s.ConfigSnapshot().Directories.DataDir); err != nil {
		return whatsapp.LogoutResult{}, err
	}
	integrationstatus.SetWhatsAppLinked(false, "")
	return whatsapp.LogoutResult{}, nil
}

func (w serverWhatsApp) SendTest(ctx context.Context, now time.Time) error {
	bot := w.s.whatsAppBot.Load()
	if bot == nil {
		return whatsapp.ErrNotConnected
	}
	return bot.SendTestMessage(ctx, now)
}
```

- [ ] **Step 5: Wire it into the server**

`internal/server/server.go`:
- Add `"aurago/internal/whatsapp"` to the imports.
- Directly after `	telegramOffsets       telegram.UpdateOffsets`, insert:
```go
	whatsAppLifecycleMu   sync.Mutex
	whatsAppBot           atomic.Pointer[whatsapp.Bot]
	whatsAppLink          *whatsapp.LinkSession // guarded by whatsAppLifecycleMu
	whatsAppClosed        bool
	whatsAppCtl           whatsAppController // tests only; nil uses serverWhatsApp
```
- In `replaceConfigSnapshot`, directly after
```go
	if bot := s.telegramBot.Load(); bot != nil {
		bot.CancelIfConfigChanged(cfg)
	}
```
  insert:
```go
	if bot := s.whatsAppBot.Load(); bot != nil {
		bot.CancelIfConfigChanged(cfg)
	}
```

`internal/server/server_routes.go` — directly after `	s.configureTelegramBot()`, insert:
```go
	s.configureWhatsAppBot()
```

`internal/server/server_shutdown.go` — in `closeRuntimeResources`, directly after `	s.stopTelegramBot()`, insert:
```go
	s.stopWhatsApp()
```

`internal/server/config_handlers_main.go`, in `handleUpdateConfig`:
- Directly before `		if err := config.ValidateDecisionsConfig(validateCfg.Decisions); err != nil {`, insert:
```go
		if err := config.ValidateWhatsAppConfig(validateCfg.WhatsApp); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
```
- Directly after `		telegramChanged := false`, insert:
```go
		whatsAppChanged := false
```
- Directly after `			telegramChanged = oldCfg.Telegram != newCfg.Telegram || oldCfg.EggMode.Enabled != newCfg.EggMode.Enabled`, insert:
```go
			whatsAppChanged = !oldCfg.WhatsApp.Equal(newCfg.WhatsApp) || oldCfg.EggMode.Enabled != newCfg.EggMode.Enabled ||
				oldCfg.Directories.DataDir != newCfg.Directories.DataDir
```
- Directly after
```go
		if loadErr == nil && telegramChanged {
			s.configureTelegramBot()
		}
```
  insert:
```go
		if loadErr == nil && whatsAppChanged {
			s.configureWhatsAppBot()
		}
```

Then run `gofmt -w internal/server/server.go internal/server/server_routes.go internal/server/server_shutdown.go internal/server/config_handlers_main.go internal/server/whatsapp_runtime.go internal/server/whatsapp_runtime_test.go`.

- [ ] **Step 6: Run the tests and confirm they pass**

```bash
CGO_ENABLED=0 go test ./internal/server/ -run '(TestWhatsAppRuntime|TestServerWhatsAppController|TestHandleUpdateConfigWhatsApp|TestTelegramConfigPublication|TestHandleUpdateConfigTelegram)' -count=1
CGO_ENABLED=0 go build ./cmd/aurago
```
Expected: the tests PASS, and the binary builds with whatsmeow linked.

- [ ] **Step 7: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="internal/server/whatsapp_runtime.go internal/server/whatsapp_runtime_test.go internal/server/server.go internal/server/server_routes.go internal/server/server_shutdown.go internal/server/config_handlers_main.go"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "feat(server): whatsapp lifecycle, save validation and hot reload" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```

---

### Task 10: Admin endpoints, route manifest, network inventory

**Files:**
- Create: `internal/server/whatsapp_handlers.go`
- Create (test): `internal/server/whatsapp_handlers_test.go`
- Modify: `internal/server/server_routes.go` — after `	mux.Handle("/api/telegram/test", requireAdmin(s, handleTelegramTest(s)))` (≈393)
- Modify: `internal/audit/audit_manifests.go`:
  - `RouteContractManifest`: after the `/api/telegram/test-connection` row (≈197)
  - `NetworkClientInventory`: after the `internal/discord/` row (≈240)

- [ ] **Step 1: Impact analysis**

```bash
node .gitnexus/run.cjs impact "RouteContractManifest" --direction upstream --repo .
node .gitnexus/run.cjs impact "NetworkClientInventory" --direction upstream --repo .
```
Expected: only audit tests call these. LOW.

- [ ] **Step 2: Write the failing test** `internal/server/whatsapp_handlers_test.go`

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/whatsapp"
)

type fakeWhatsAppController struct {
	linkReq   whatsapp.LinkRequest
	linkForce bool
	linkErr   error
	snapshot  whatsapp.LinkSnapshot
	logout    whatsapp.LogoutResult
	tests     int
	testErr   error
}

func (f *fakeWhatsAppController) Link(_ context.Context, req whatsapp.LinkRequest, force bool) (whatsapp.LinkSnapshot, error) {
	f.linkReq, f.linkForce = req, force
	if f.linkErr != nil {
		return whatsapp.LinkSnapshot{}, f.linkErr
	}
	return whatsapp.LinkSnapshot{State: whatsapp.LinkStateWaiting}, nil
}

func (f *fakeWhatsAppController) LinkSnapshot() whatsapp.LinkSnapshot { return f.snapshot }

func (f *fakeWhatsAppController) Logout(context.Context) (whatsapp.LogoutResult, error) {
	return f.logout, nil
}

func (f *fakeWhatsAppController) SendTest(context.Context, time.Time) error {
	f.tests++
	return f.testErr
}

func whatsAppTestServer(ctl whatsAppController) *Server {
	return &Server{Cfg: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), whatsAppCtl: ctl}
}

func TestWhatsAppHandlersUseNoStoreAndMapErrors(t *testing.T) {
	fake := &fakeWhatsAppController{snapshot: whatsapp.LinkSnapshot{State: whatsapp.LinkStateQR, QRPNG: "data:image/png;base64,AAAA"}}
	s := whatsAppTestServer(fake)
	rec := httptest.NewRecorder()
	handleWhatsAppLink(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/whatsapp/link", strings.NewReader(`{"method":"phone","phone":"+49 170 1234567","force":true}`)))
	if rec.Code != http.StatusAccepted || rec.Header().Get("Cache-Control") != "no-store" || fake.linkReq.Phone != "+49 170 1234567" || !fake.linkForce {
		t.Fatalf("link: status=%d headers=%v req=%+v force=%v", rec.Code, rec.Header(), fake.linkReq, fake.linkForce)
	}
	rec = httptest.NewRecorder()
	handleWhatsAppLink(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/whatsapp/link", nil))
	var snap whatsapp.LinkSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil || snap.State != whatsapp.LinkStateQR || snap.QRPNG == "" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("snapshot: %s %v", rec.Body.String(), err)
	}
	for err, want := range map[error]struct {
		code   int
		reason string
	}{
		whatsapp.ErrAlreadyLinked:      {http.StatusConflict, "already_linked"},
		whatsapp.ErrLinkInProgress:     {http.StatusConflict, "link_in_progress"},
		whatsapp.ErrNotConfigured:      {http.StatusConflict, "not_configured"},
		whatsapp.ErrInvalidPhone:       {http.StatusBadRequest, "invalid_phone"},
		whatsapp.ErrInvalidLinkMethod:  {http.StatusBadRequest, "invalid_method"},
		errors.New("websocket closed"): {http.StatusInternalServerError, "whatsapp_failed"},
	} {
		fake.linkErr = err
		rec = httptest.NewRecorder()
		handleWhatsAppLink(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/whatsapp/link", strings.NewReader(`{"method":"qr"}`)))
		if rec.Code != want.code || !strings.Contains(rec.Body.String(), `"`+want.reason+`"`) || strings.Contains(rec.Body.String(), "websocket") {
			t.Errorf("%v: status=%d body=%s", err, rec.Code, rec.Body.String())
		}
	}
	rec = httptest.NewRecorder()
	handleWhatsAppLink(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/whatsapp/link", strings.NewReader(`{"method":"qr","number":"+491701234567"}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("unknown field: status=%d body=%s", rec.Code, rec.Body.String())
	}
	integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateNotLinked, config.WhatsAppModeDedicated)
	rec = httptest.NewRecorder()
	handleWhatsAppStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/whatsapp/status", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"status":"not_linked"`) {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	fake.logout = whatsapp.LogoutResult{RemoteRemoved: true}
	rec = httptest.NewRecorder()
	handleWhatsAppLogout(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/whatsapp/logout", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"remote_removed":true`) {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleWhatsAppLogout(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/whatsapp/logout", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET logout = %d, want 405", rec.Code)
	}
}

func TestWhatsAppTestRateLimitAllowsOneSendPerMinute(t *testing.T) {
	originalNow, originalLast := whatsAppTestNow, whatsAppTestLast
	t.Cleanup(func() { whatsAppTestNow, whatsAppTestLast = originalNow, originalLast })
	fixed := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	whatsAppTestNow = func() time.Time { return fixed }
	whatsAppTestLast = time.Time{}
	fake := &fakeWhatsAppController{}
	s := whatsAppTestServer(fake)
	first, second := httptest.NewRecorder(), httptest.NewRecorder()
	handleWhatsAppTest(s).ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/whatsapp/test", nil))
	handleWhatsAppTest(s).ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/whatsapp/test", nil))
	if first.Code != http.StatusOK || second.Code != http.StatusTooManyRequests || fake.tests != 1 || !strings.Contains(second.Body.String(), "rate_limited") {
		t.Fatalf("responses = %d/%d, sends = %d", first.Code, second.Code, fake.tests)
	}
}

func TestWhatsAppRoutesRequireAdmin(t *testing.T) {
	cfg := &config.Config{}
	cfg.WebConfig.Enabled = true
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "whatsapp-route-test-session-secret"
	s := &Server{Cfg: cfg, Logger: slog.Default(), whatsAppCtl: &fakeWhatsAppController{}}
	mux := http.NewServeMux()
	registerWhatsAppRoutes(mux, s)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/whatsapp/status"},
		{http.MethodGet, "/api/whatsapp/link"},
		{http.MethodPost, "/api/whatsapp/link"},
		{http.MethodPost, "/api/whatsapp/logout"},
		{http.MethodPost, "/api/whatsapp/test"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`)))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", route.method, route.path, rec.Code)
		}
	}
}
```

- [ ] **Step 3: Run the tests and confirm they fail**

```bash
CGO_ENABLED=0 go test ./internal/server/ -run '(TestWhatsAppHandlers|TestWhatsAppTestRateLimit|TestWhatsAppRoutesRequireAdmin)' -count=1
```
Expected: FAIL, `undefined: handleWhatsAppLink`, `undefined: registerWhatsAppRoutes`.

- [ ] **Step 4: Implement** `internal/server/whatsapp_handlers.go`

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"aurago/internal/integrationstatus"
	"aurago/internal/whatsapp"
)

var (
	whatsAppTestMu   sync.Mutex
	whatsAppTestLast time.Time
	whatsAppTestNow  = time.Now
)

// whatsAppActionTimeout bounds store access and a remote unlink per request.
// The link session itself outlives the request.
const whatsAppActionTimeout = 30 * time.Second

// registerWhatsAppRoutes mounts the admin-only, no-store WhatsApp endpoints.
func registerWhatsAppRoutes(mux *http.ServeMux, s *Server) {
	mux.Handle("/api/whatsapp/status", requireAdmin(s, handleWhatsAppStatus(s)))
	mux.Handle("/api/whatsapp/link", requireAdmin(s, handleWhatsAppLink(s)))
	mux.Handle("/api/whatsapp/logout", requireAdmin(s, handleWhatsAppLogout(s)))
	mux.Handle("/api/whatsapp/test", requireAdmin(s, handleWhatsAppTest(s)))
}

func writeWhatsAppJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// whatsAppErrorStatus maps runtime errors to an HTTP status and a fixed code.
// Raw errors may carry JIDs, so they never reach the response or the log.
func whatsAppErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, whatsapp.ErrInvalidLinkMethod):
		return http.StatusBadRequest, "invalid_method"
	case errors.Is(err, whatsapp.ErrInvalidPhone):
		return http.StatusBadRequest, "invalid_phone"
	case errors.Is(err, whatsapp.ErrNotConfigured):
		return http.StatusConflict, "not_configured"
	case errors.Is(err, whatsapp.ErrAlreadyLinked):
		return http.StatusConflict, "already_linked"
	case errors.Is(err, whatsapp.ErrLinkInProgress):
		return http.StatusConflict, "link_in_progress"
	case errors.Is(err, whatsapp.ErrNotConnected), errors.Is(err, whatsapp.ErrNoRecipient):
		return http.StatusConflict, "not_connected"
	}
	return http.StatusInternalServerError, "whatsapp_failed"
}

func writeWhatsAppFailure(s *Server, w http.ResponseWriter, action string, err error) {
	code, reason := whatsAppErrorStatus(err)
	if code >= http.StatusInternalServerError && s.Logger != nil {
		s.Logger.Warn("WhatsApp request failed", "action", action, "error_code", reason)
	}
	writeWhatsAppJSON(w, code, map[string]string{"error": reason})
}

func whatsAppMethodNotAllowed(w http.ResponseWriter) {
	writeWhatsAppJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
}

func handleWhatsAppStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			whatsAppMethodNotAllowed(w)
			return
		}
		status := integrationstatus.WhatsAppStatus()
		label := "ok"
		if status.State != integrationstatus.WhatsAppStateHealthy {
			label = status.State
		}
		// Always 200: the Config UI shows every state itself.
		writeWhatsAppJSON(w, http.StatusOK, map[string]any{"status": label, "whatsapp": status})
	}
}

type whatsAppLinkBody struct {
	Method string `json:"method"`
	Phone  string `json:"phone"`
	Force  bool   `json:"force"`
}

func handleWhatsAppLink(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeWhatsAppJSON(w, http.StatusOK, s.whatsApp().LinkSnapshot())
		case http.MethodPost:
			var body whatsAppLinkBody
			decoder := json.NewDecoder(io.LimitReader(r.Body, 4<<10))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&body); err != nil {
				writeWhatsAppJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), whatsAppActionTimeout)
			defer cancel()
			snapshot, err := s.whatsApp().Link(ctx, whatsapp.LinkRequest{Method: body.Method, Phone: body.Phone}, body.Force)
			if err != nil {
				writeWhatsAppFailure(s, w, "link", err)
				return
			}
			writeWhatsAppJSON(w, http.StatusAccepted, snapshot)
		default:
			whatsAppMethodNotAllowed(w)
		}
	}
}

func handleWhatsAppLogout(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			whatsAppMethodNotAllowed(w)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), whatsAppActionTimeout)
		defer cancel()
		result, err := s.whatsApp().Logout(ctx)
		if err != nil {
			writeWhatsAppFailure(s, w, "logout", err)
			return
		}
		writeWhatsAppJSON(w, http.StatusOK, map[string]any{"status": "unlinked", "remote_removed": result.RemoteRemoved})
	}
}

// handleWhatsAppTest sends the owner-only test message, at most once per
// minute (like /api/telegram/test).
func handleWhatsAppTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			whatsAppMethodNotAllowed(w)
			return
		}
		now := whatsAppTestNow()
		whatsAppTestMu.Lock()
		if !whatsAppTestLast.IsZero() && now.Sub(whatsAppTestLast) < time.Minute {
			whatsAppTestMu.Unlock()
			writeWhatsAppJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
			return
		}
		whatsAppTestLast = now
		whatsAppTestMu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), whatsAppActionTimeout)
		defer cancel()
		if err := s.whatsApp().SendTest(ctx, now); err != nil {
			writeWhatsAppFailure(s, w, "test", err)
			return
		}
		writeWhatsAppJSON(w, http.StatusOK, map[string]any{"status": "sent", "sent_at": now.UTC().Format(time.RFC3339)})
	}
}
```

- [ ] **Step 5: Register the routes and the manifests**

`internal/server/server_routes.go` — directly after `	mux.Handle("/api/telegram/test", requireAdmin(s, handleTelegramTest(s)))`, insert:
```go
	registerWhatsAppRoutes(mux, s)
```

`internal/audit/audit_manifests.go`:
- In `RouteContractManifest`, directly after `		{Pattern: "/api/telegram/test-connection", Methods: []string{"POST"}, Auth: "admin", Category: "notifications"},`, insert:
```go
		{Pattern: "/api/whatsapp/status", Methods: []string{"GET"}, Auth: "admin", Category: "integration", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/whatsapp/link", Methods: []string{"GET", "POST"}, Auth: "admin", Category: "integration", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/whatsapp/logout", Methods: []string{"POST"}, Auth: "admin", Category: "integration", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/whatsapp/test", Methods: []string{"POST"}, Auth: "admin", Category: "integration", ContentTypes: []string{"application/json"}},
```
- In `NetworkClientInventory`, directly after `		{Path: "internal/discord/", Classification: "configured-messaging", Credentialed: true},`, insert:
```go
		{Path: "internal/whatsapp/", Classification: "fixed-whatsapp-web-multidevice-websocket-and-media-hosts-owner-only", Credentialed: true},
```

- [ ] **Step 6: Run the tests and confirm they pass**

```bash
CGO_ENABLED=0 go test ./internal/server/ -run '(TestWhatsApp|TestServerWhatsAppController|TestIntegrationTestRoutesRequireAdminSession)' -count=1
CGO_ENABLED=0 go test ./internal/audit/ -run '(TestRouteContractManifestCoversRegisteredServerRoutes|TestNetworkClientInventoryCoversProductionHTTPClients)' -count=1
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="internal/server/whatsapp_handlers.go internal/server/whatsapp_handlers_test.go internal/server/server_routes.go internal/audit/audit_manifests.go"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "feat(server): admin whatsapp link, status, logout and test endpoints" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```

---

### Task 11: Security — file-tool denial, documented backup exclusion, security review

**Files:**
- Modify: `internal/tools/protected_paths.go` — `protectedSystemFilesFromConfig`
- Create (test): `internal/tools/whatsapp_protected_paths_test.go`
- Modify: `internal/server/backup_handlers.go` — in `handleBackupCreate`, the comment block that starts `// Deliberately NOT backed up` (≈680-686)

Task 1 already proves the backup exclusion. `TestWhatsAppStorePathsStayOutOfBackups` checks `SQLiteDatabasePaths`, and the audit guard lists `device.db`. Apart from SQLite stores, backup copies only an explicit list of data files plus `data/documents/`.

This task adds the store to the explicit protected-file list, as the vault already is. The file-tool denial then no longer relies only on the `data_dir` prefix rule. The explicit list also enables hard-link detection for outgoing attachments.

- [ ] **Step 1: Impact analysis**

```bash
node .gitnexus/run.cjs impact "protectedSystemFilesFromConfig" --direction upstream --repo .
node .gitnexus/run.cjs impact "handleBackupCreate" --direction upstream --repo .
```
Expected:
- `protectedSystemFilesFromConfig` feeds `IsProtectedSystemPath`, `ConfigureRuntimePermissions` and outgoing attachments. This is likely HIGH; report it. The change only appends paths.
- The `handleBackupCreate` edit is a comment only.

- [ ] **Step 2: Write the failing test** `internal/tools/whatsapp_protected_paths_test.go`

```go
package tools

import (
	"path/filepath"
	"slices"
	"testing"

	"aurago/internal/config"
)

func TestWhatsAppDeviceStoreIsProtectedSystemState(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.DataDir = filepath.Join(root, "data")
	db := config.WhatsAppDeviceDBPath(cfg.Directories.DataDir)
	for _, path := range []string{db, db + "-wal", db + "-shm", db + "-journal", filepath.Dir(db)} {
		if !IsProtectedSystemPath(path, root, cfg) {
			t.Errorf("%s is reachable by agent file tools", path)
		}
	}
	protected := protectedSystemFilesFromConfig(cfg)
	for _, path := range config.WhatsAppProtectedPaths(cfg) {
		if !slices.Contains(protected, path) {
			t.Errorf("the explicit protected list misses %s", path)
		}
	}
	if !outgoingPathIsProtected(db, outgoingProtectedFiles(cfg)) {
		t.Error("an outgoing attachment could send the WhatsApp device store")
	}
}
```

- [ ] **Step 3: Run it and confirm it fails**

```bash
CGO_ENABLED=0 go test ./internal/tools/ -run 'TestWhatsAppDeviceStoreIsProtectedSystemState' -count=1
```
Expected: FAIL with `the explicit protected list misses …/whatsapp/device.db` and `an outgoing attachment could send the WhatsApp device store`. The `IsProtectedSystemPath` loop already passes because of the `data_dir` prefix rule.

- [ ] **Step 4: Implement**

`internal/tools/protected_paths.go`, in `protectedSystemFilesFromConfig` — replace
```go
	files := append([]string{cfg.ConfigPath, vaultBase, vaultBase + ".lock"}, config.SQLiteProtectedPaths(cfg)...)
	files = append(files, registeredProtectedFiles()...)
```
with
```go
	files := append([]string{cfg.ConfigPath, vaultBase, vaultBase + ".lock"}, config.SQLiteProtectedPaths(cfg)...)
	// The WhatsApp linked-device store is a credential like the vault and not a
	// backed-up SQLite store, so SQLiteProtectedPaths does not list it.
	files = append(files, config.WhatsAppProtectedPaths(cfg)...)
	files = append(files, registeredProtectedFiles()...)
```
Also update the function's doc comment to: `// … the active config, the vault and its lock, the WhatsApp device store, and every configured SQLite database with its WAL/SHM sidecars.`

`internal/server/backup_handlers.go`, in `handleBackupCreate` — directly after the comment line `		//     to another machine; the restored install re-registers instead.`, insert:
```go
		//   - data/whatsapp (WhatsApp linked-device keys): a restored copy would clone
		//     the linked device; the owner relinks after a restore.
```

- [ ] **Step 5: Run the tests and confirm they pass**

```bash
CGO_ENABLED=0 go test ./internal/tools/ -run '(TestWhatsAppDeviceStoreIsProtectedSystemState|TestIsProtectedSystemPath|TestOutgoing|TestRuntimePermission)' -count=1
CGO_ENABLED=0 go test ./internal/config/ -run 'TestWhatsAppStorePathsStayOutOfBackups' -count=1
CGO_ENABLED=0 go test ./internal/audit/ -run 'TestSQLiteOpensAreBackedUpOrExcluded' -count=1
CGO_ENABLED=0 go build ./internal/server/
```
Expected: the tests PASS and the build succeeds.

- [ ] **Step 6: Security review (security-agent)**

Review the commits from Task 1 through this task against the checklist below. Report only Critical and Important findings; fix them in this task.

- **Store protection:**
  - The directory is 0700 and the file 0600, set via `fileutil.RestrictToOwner` (a DACL on Windows).
  - The store is never in `SQLiteDatabasePaths`.
  - Agent file tools and outgoing attachments cannot reach it.
- **Logging:**
  - No log contains a QR payload, pairing code, message content or full phone number.
  - `maskDigits` is applied in `log.go`, `runtime.go` and `link.go`.
  - Server handlers log fixed error codes only.
- **Recipients:** every send goes through `ownerRecipients`, and no endpoint accepts a recipient.
- **Endpoints:**
  - All four use `requireAdmin` and `Cache-Control: no-store`.
  - `POST /api/whatsapp/link` caps the body at 4 KiB and rejects unknown fields.
  - The test endpoint is rate-limited to one call per 60 s.
- **Link sessions:**
  - Only one session runs at a time.
  - Force-relink validates the request before it unlinks anything.
  - The link client is cancelled and disconnected before the next Bot connects.
- **Revocation and shutdown:**
  - Config publication cancels the generation (`CancelIfConfigChanged`).
  - Shutdown drains the session and the Bot (`stopWhatsApp`).
  - A stopped generation never writes status.
- **Licensing:** binaries are documented as GPL-3.0 combined works, and `LICENSE` stays MIT.

- [ ] **Step 7: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="internal/tools/protected_paths.go internal/tools/whatsapp_protected_paths_test.go internal/server/backup_handlers.go"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "fix(security): deny the whatsapp device store to agent file tools" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```
If the security review leads to changes in other files, add them to `F`.

---


### Task 12: Translations (16 locales) and config help

**Files:**
- Create: `ui/lang/config/whatsapp/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`
- Modify: `ui/lang/config/sections/<16 locales>.json` — add two keys between `config.section.webhooks.label` and `config.section.whisper.desc`
- Modify: `ui/config_help.json` — insert a `whatsapp` block before `  "discord": {`

Keys are merged by prefix, not by folder (`internal/i18n`, `uiI18NSections`). So the `help.whatsapp.*` keys can live next to `config.whatsapp.*`, as `ui/lang/config/manus/` already does. Task 13 references every `config.whatsapp.*` key literally, which `TestTranslations_ConfigCfgJSKeysExist` checks. German uses "Du" and real umlauts. All files are flat UTF-8 JSON, LF, no BOM, 2-space indent, and keep the key order shown below.

- [ ] **Step 1: Write the English reference** `ui/lang/config/whatsapp/en.json`

```json
{
  "config.whatsapp.intro": "Chat with AuraGo on WhatsApp. AuraGo joins an account as a linked device and answers only owner numbers.",
  "config.whatsapp.enabled": "Enable WhatsApp",
  "config.whatsapp.mode": "Number",
  "config.whatsapp.mode_dedicated": "Dedicated AuraGo number",
  "config.whatsapp.mode_self_chat": "My own number (Message yourself)",
  "config.whatsapp.owner_numbers": "Owner numbers",
  "config.whatsapp.self_chat_notice": "Self-chat mode reads only your Message yourself chat. All other chats stay private.",
  "config.whatsapp.readonly": "Read-only",
  "config.whatsapp.send_read_receipts": "Send read receipts",
  "config.whatsapp.self_chat_prefix": "Self-chat prefix",
  "config.whatsapp.link_title": "Linked device",
  "config.whatsapp.phone_rule": "Open WhatsApp on the phone at least every 14 days, or linked devices are removed. Up to 4 linked devices per account.",
  "config.whatsapp.linked_as": "Linked as {number}",
  "config.whatsapp.not_linked": "No device linked.",
  "config.whatsapp.link_qr": "Link via QR code",
  "config.whatsapp.link_phone": "Link via phone number",
  "config.whatsapp.unlink": "Unlink",
  "config.whatsapp.send_test": "Send test message",
  "config.whatsapp.qr_hint": "On the phone: Settings → Linked devices → Link a device, then scan this code.",
  "config.whatsapp.code_hint": "On the phone: Linked devices → Link a device → Link with phone number instead, then enter this code.",
  "config.whatsapp.relink_confirm": "A device is already linked. Unlink it and link again?",
  "config.whatsapp.unlink_confirm": "Unlink AuraGo from this WhatsApp account?",
  "config.whatsapp.unlinked_local": "Removed here. Also remove AuraGo under Linked devices on the phone.",
  "config.whatsapp.link_success": "Linked. AuraGo is connecting…",
  "config.whatsapp.link_timeout": "The code expired. Start again.",
  "config.whatsapp.save_first": "Save your changes first.",
  "config.whatsapp.err_link_in_progress": "A link is already in progress.",
  "config.whatsapp.err_not_configured": "Turn WhatsApp on, add an owner number for dedicated mode and save.",
  "config.whatsapp.err_invalid_phone": "Enter the number in international format, e.g. +491701234567.",
  "config.whatsapp.err_not_connected": "WhatsApp is not connected.",
  "config.whatsapp.err_rate_limited": "Wait a minute before the next test message.",
  "config.whatsapp.err_failed": "WhatsApp did not respond. Try again.",
  "config.whatsapp.err_passkey_unsupported": "The phone asked for a passkey link, which AuraGo does not support yet. Use the QR code.",
  "config.whatsapp.state_logged_out": "Logged out on the phone. Link again.",
  "config.whatsapp.state_conflict": "Another client uses this link. Unlink and link again.",
  "config.whatsapp.state_temp_banned": "Temporarily blocked by WhatsApp.",
  "config.whatsapp.state_outdated": "This AuraGo version is outdated. Update AuraGo.",
  "help.whatsapp.mode": "Dedicated: AuraGo has its own number (recommended). Own number: only your Message yourself chat reaches AuraGo.",
  "help.whatsapp.owner_numbers": "International format, separated by commas. Only these numbers can reach AuraGo.",
  "help.whatsapp.readonly": "Replies to you still work; the agent cannot send on its own.",
  "help.whatsapp.send_read_receipts": "Dedicated mode only."
}
```

- [ ] **Step 2: Run the completeness test and confirm it fails**

```bash
CGO_ENABLED=0 go test -count=1 ./ui/ -run 'TestTranslations_AllKeysPresentInAllLanguages'
```
Expected: FAIL with `Missing file lang/config/whatsapp/cs.json` (and the other 14 locales).

- [ ] **Step 3: Add the other 15 locales** (same key order as `en.json`)

`de.json`:
```json
{
  "config.whatsapp.intro": "Chatte mit AuraGo über WhatsApp. AuraGo meldet sich als verknüpftes Gerät an und antwortet nur Besitzernummern.",
  "config.whatsapp.enabled": "WhatsApp aktivieren",
  "config.whatsapp.mode": "Nummer",
  "config.whatsapp.mode_dedicated": "Eigene AuraGo-Nummer",
  "config.whatsapp.mode_self_chat": "Meine eigene Nummer (Chat mit dir selbst)",
  "config.whatsapp.owner_numbers": "Besitzernummern",
  "config.whatsapp.self_chat_notice": "Im Selbst-Chat-Modus liest AuraGo nur deinen Chat mit dir selbst. Alle anderen Chats bleiben privat.",
  "config.whatsapp.readonly": "Nur lesen",
  "config.whatsapp.send_read_receipts": "Lesebestätigungen senden",
  "config.whatsapp.self_chat_prefix": "Präfix im Selbst-Chat",
  "config.whatsapp.link_title": "Verknüpftes Gerät",
  "config.whatsapp.phone_rule": "Öffne WhatsApp auf dem Handy mindestens alle 14 Tage, sonst werden verknüpfte Geräte entfernt. Pro Konto sind bis zu 4 verknüpfte Geräte möglich.",
  "config.whatsapp.linked_as": "Verknüpft als {number}",
  "config.whatsapp.not_linked": "Kein Gerät verknüpft.",
  "config.whatsapp.link_qr": "Per QR-Code verknüpfen",
  "config.whatsapp.link_phone": "Per Telefonnummer verknüpfen",
  "config.whatsapp.unlink": "Verknüpfung lösen",
  "config.whatsapp.send_test": "Testnachricht senden",
  "config.whatsapp.qr_hint": "Auf dem Handy: Einstellungen → Verknüpfte Geräte → Gerät hinzufügen, dann diesen Code scannen.",
  "config.whatsapp.code_hint": "Auf dem Handy: Verknüpfte Geräte → Gerät hinzufügen → Stattdessen mit Telefonnummer verknüpfen, dann diesen Code eingeben.",
  "config.whatsapp.relink_confirm": "Es ist bereits ein Gerät verknüpft. Verknüpfung lösen und neu verknüpfen?",
  "config.whatsapp.unlink_confirm": "AuraGo von diesem WhatsApp-Konto trennen?",
  "config.whatsapp.unlinked_local": "Hier entfernt. Entferne AuraGo auch auf dem Handy unter „Verknüpfte Geräte“.",
  "config.whatsapp.link_success": "Verknüpft. AuraGo verbindet sich …",
  "config.whatsapp.link_timeout": "Der Code ist abgelaufen. Starte neu.",
  "config.whatsapp.save_first": "Speichere zuerst deine Änderungen.",
  "config.whatsapp.err_link_in_progress": "Eine Verknüpfung läuft bereits.",
  "config.whatsapp.err_not_configured": "Schalte WhatsApp ein, trage für die eigene AuraGo-Nummer eine Besitzernummer ein und speichere.",
  "config.whatsapp.err_invalid_phone": "Gib die Nummer im internationalen Format ein, z. B. +491701234567.",
  "config.whatsapp.err_not_connected": "WhatsApp ist nicht verbunden.",
  "config.whatsapp.err_rate_limited": "Warte eine Minute bis zur nächsten Testnachricht.",
  "config.whatsapp.err_failed": "WhatsApp hat nicht geantwortet. Versuche es erneut.",
  "config.whatsapp.err_passkey_unsupported": "Das Handy verlangt eine Verknüpfung per Passkey, die AuraGo noch nicht unterstützt. Nutze den QR-Code.",
  "config.whatsapp.state_logged_out": "Auf dem Handy abgemeldet. Verknüpfe neu.",
  "config.whatsapp.state_conflict": "Ein anderer Client nutzt diese Verknüpfung. Löse sie und verknüpfe neu.",
  "config.whatsapp.state_temp_banned": "Vorübergehend von WhatsApp gesperrt.",
  "config.whatsapp.state_outdated": "Diese AuraGo-Version ist veraltet. Aktualisiere AuraGo.",
  "help.whatsapp.mode": "Eigene AuraGo-Nummer: AuraGo hat eine eigene Nummer (empfohlen). Meine Nummer: Nur dein Chat mit dir selbst erreicht AuraGo.",
  "help.whatsapp.owner_numbers": "Im internationalen Format, durch Kommas getrennt. Nur diese Nummern erreichen AuraGo.",
  "help.whatsapp.readonly": "Antworten an dich funktionieren weiter; der Agent kann nicht von sich aus senden.",
  "help.whatsapp.send_read_receipts": "Nur mit eigener AuraGo-Nummer."
}
```

`cs.json`:
```json
{
  "config.whatsapp.intro": "Pište si s AuraGo přes WhatsApp. AuraGo se připojí jako propojené zařízení a odpovídá jen číslům vlastníka.",
  "config.whatsapp.enabled": "Zapnout WhatsApp",
  "config.whatsapp.mode": "Číslo",
  "config.whatsapp.mode_dedicated": "Vlastní číslo pro AuraGo",
  "config.whatsapp.mode_self_chat": "Moje vlastní číslo (chat sám se sebou)",
  "config.whatsapp.owner_numbers": "Čísla vlastníka",
  "config.whatsapp.self_chat_notice": "V tomto režimu čte AuraGo jen váš chat sám se sebou. Ostatní chaty zůstávají soukromé.",
  "config.whatsapp.readonly": "Jen pro čtení",
  "config.whatsapp.send_read_receipts": "Odesílat potvrzení o přečtení",
  "config.whatsapp.self_chat_prefix": "Předpona v chatu se sebou",
  "config.whatsapp.link_title": "Propojené zařízení",
  "config.whatsapp.phone_rule": "Otevřete WhatsApp v telefonu alespoň každých 14 dní, jinak se propojená zařízení odstraní. Účet může mít až 4 propojená zařízení.",
  "config.whatsapp.linked_as": "Propojeno jako {number}",
  "config.whatsapp.not_linked": "Žádné propojené zařízení.",
  "config.whatsapp.link_qr": "Propojit QR kódem",
  "config.whatsapp.link_phone": "Propojit telefonním číslem",
  "config.whatsapp.unlink": "Zrušit propojení",
  "config.whatsapp.send_test": "Odeslat testovací zprávu",
  "config.whatsapp.qr_hint": "V telefonu: Nastavení → Propojená zařízení → Propojit zařízení, pak naskenujte tento kód.",
  "config.whatsapp.code_hint": "V telefonu: Propojená zařízení → Propojit zařízení → Propojit pomocí telefonního čísla, pak zadejte tento kód.",
  "config.whatsapp.relink_confirm": "Zařízení už je propojené. Zrušit propojení a propojit znovu?",
  "config.whatsapp.unlink_confirm": "Odpojit AuraGo od tohoto účtu WhatsApp?",
  "config.whatsapp.unlinked_local": "Odstraněno zde. Odstraňte AuraGo také v telefonu v části Propojená zařízení.",
  "config.whatsapp.link_success": "Propojeno. AuraGo se připojuje…",
  "config.whatsapp.link_timeout": "Platnost kódu vypršela. Začněte znovu.",
  "config.whatsapp.save_first": "Nejprve uložte změny.",
  "config.whatsapp.err_link_in_progress": "Propojování už probíhá.",
  "config.whatsapp.err_not_configured": "Zapněte WhatsApp, pro vlastní číslo AuraGo zadejte číslo vlastníka a uložte.",
  "config.whatsapp.err_invalid_phone": "Zadejte číslo v mezinárodním formátu, např. +491701234567.",
  "config.whatsapp.err_not_connected": "WhatsApp není připojen.",
  "config.whatsapp.err_rate_limited": "Před další testovací zprávou počkejte minutu.",
  "config.whatsapp.err_failed": "WhatsApp neodpověděl. Zkuste to znovu.",
  "config.whatsapp.err_passkey_unsupported": "Telefon požaduje propojení přes passkey, které AuraGo zatím nepodporuje. Použijte QR kód.",
  "config.whatsapp.state_logged_out": "Odhlášeno v telefonu. Propojte znovu.",
  "config.whatsapp.state_conflict": "Toto propojení používá jiný klient. Zrušte je a propojte znovu.",
  "config.whatsapp.state_temp_banned": "Dočasně zablokováno službou WhatsApp.",
  "config.whatsapp.state_outdated": "Tato verze AuraGo je zastaralá. Aktualizujte AuraGo.",
  "help.whatsapp.mode": "Vlastní číslo pro AuraGo: AuraGo má vlastní číslo (doporučeno). Moje číslo: AuraGo dostává jen váš chat sám se sebou.",
  "help.whatsapp.owner_numbers": "V mezinárodním formátu, oddělená čárkami. AuraGo odpovídá jen těmto číslům.",
  "help.whatsapp.readonly": "Odpovědi vám fungují dál; agent nemůže posílat zprávy sám od sebe.",
  "help.whatsapp.send_read_receipts": "Jen s vlastním číslem pro AuraGo."
}
```

`da.json`:
```json
{
  "config.whatsapp.intro": "Chat med AuraGo på WhatsApp. AuraGo tilføjes som en tilknyttet enhed og svarer kun ejernumre.",
  "config.whatsapp.enabled": "Aktivér WhatsApp",
  "config.whatsapp.mode": "Nummer",
  "config.whatsapp.mode_dedicated": "Eget AuraGo-nummer",
  "config.whatsapp.mode_self_chat": "Mit eget nummer (chat med dig selv)",
  "config.whatsapp.owner_numbers": "Ejernumre",
  "config.whatsapp.self_chat_notice": "I selvchat-tilstand læser AuraGo kun din chat med dig selv. Alle andre chats forbliver private.",
  "config.whatsapp.readonly": "Skrivebeskyttet",
  "config.whatsapp.send_read_receipts": "Send læsekvitteringer",
  "config.whatsapp.self_chat_prefix": "Præfiks i selvchat",
  "config.whatsapp.link_title": "Tilknyttet enhed",
  "config.whatsapp.phone_rule": "Åbn WhatsApp på telefonen mindst hver 14. dag, ellers fjernes tilknyttede enheder. Op til 4 tilknyttede enheder pr. konto.",
  "config.whatsapp.linked_as": "Tilknyttet som {number}",
  "config.whatsapp.not_linked": "Ingen enhed tilknyttet.",
  "config.whatsapp.link_qr": "Tilknyt med QR-kode",
  "config.whatsapp.link_phone": "Tilknyt med telefonnummer",
  "config.whatsapp.unlink": "Fjern tilknytning",
  "config.whatsapp.send_test": "Send testbesked",
  "config.whatsapp.qr_hint": "På telefonen: Indstillinger → Tilknyttede enheder → Tilknyt en enhed, og scan derefter koden.",
  "config.whatsapp.code_hint": "På telefonen: Tilknyttede enheder → Tilknyt en enhed → Tilknyt med telefonnummer i stedet, og indtast derefter koden.",
  "config.whatsapp.relink_confirm": "En enhed er allerede tilknyttet. Fjern den og tilknyt igen?",
  "config.whatsapp.unlink_confirm": "Fjern AuraGo fra denne WhatsApp-konto?",
  "config.whatsapp.unlinked_local": "Fjernet her. Fjern også AuraGo under Tilknyttede enheder på telefonen.",
  "config.whatsapp.link_success": "Tilknyttet. AuraGo forbinder…",
  "config.whatsapp.link_timeout": "Koden er udløbet. Start forfra.",
  "config.whatsapp.save_first": "Gem dine ændringer først.",
  "config.whatsapp.err_link_in_progress": "En tilknytning er allerede i gang.",
  "config.whatsapp.err_not_configured": "Slå WhatsApp til, tilføj et ejernummer til eget AuraGo-nummer, og gem.",
  "config.whatsapp.err_invalid_phone": "Indtast nummeret i internationalt format, f.eks. +491701234567.",
  "config.whatsapp.err_not_connected": "WhatsApp er ikke forbundet.",
  "config.whatsapp.err_rate_limited": "Vent et minut før næste testbesked.",
  "config.whatsapp.err_failed": "WhatsApp svarede ikke. Prøv igen.",
  "config.whatsapp.err_passkey_unsupported": "Telefonen bad om tilknytning med adgangsnøgle, som AuraGo endnu ikke understøtter. Brug QR-koden.",
  "config.whatsapp.state_logged_out": "Logget ud på telefonen. Tilknyt igen.",
  "config.whatsapp.state_conflict": "En anden klient bruger denne tilknytning. Fjern den og tilknyt igen.",
  "config.whatsapp.state_temp_banned": "Midlertidigt blokeret af WhatsApp.",
  "config.whatsapp.state_outdated": "Denne AuraGo-version er forældet. Opdater AuraGo.",
  "help.whatsapp.mode": "Eget AuraGo-nummer: AuraGo har sit eget nummer (anbefalet). Mit nummer: Kun din chat med dig selv når AuraGo.",
  "help.whatsapp.owner_numbers": "Internationalt format, adskilt med kommaer. Kun disse numre kan nå AuraGo.",
  "help.whatsapp.readonly": "Svar til dig virker stadig; agenten kan ikke sende af sig selv.",
  "help.whatsapp.send_read_receipts": "Kun med eget AuraGo-nummer."
}
```

`el.json`:
```json
{
  "config.whatsapp.intro": "Συνομίλησε με το AuraGo στο WhatsApp. Το AuraGo συνδέεται ως συνδεδεμένη συσκευή και απαντά μόνο στους αριθμούς κατόχου.",
  "config.whatsapp.enabled": "Ενεργοποίηση WhatsApp",
  "config.whatsapp.mode": "Αριθμός",
  "config.whatsapp.mode_dedicated": "Ξεχωριστός αριθμός AuraGo",
  "config.whatsapp.mode_self_chat": "Ο δικός μου αριθμός (συνομιλία με τον εαυτό σου)",
  "config.whatsapp.owner_numbers": "Αριθμοί κατόχου",
  "config.whatsapp.self_chat_notice": "Σε αυτή τη λειτουργία το AuraGo διαβάζει μόνο τη συνομιλία με τον εαυτό σου. Όλες οι άλλες συνομιλίες μένουν ιδιωτικές.",
  "config.whatsapp.readonly": "Μόνο ανάγνωση",
  "config.whatsapp.send_read_receipts": "Αποστολή επιβεβαιώσεων ανάγνωσης",
  "config.whatsapp.self_chat_prefix": "Πρόθεμα στη συνομιλία με τον εαυτό σου",
  "config.whatsapp.link_title": "Συνδεδεμένη συσκευή",
  "config.whatsapp.phone_rule": "Άνοιγε το WhatsApp στο κινητό τουλάχιστον κάθε 14 ημέρες, αλλιώς οι συνδεδεμένες συσκευές αφαιρούνται. Έως 4 συνδεδεμένες συσκευές ανά λογαριασμό.",
  "config.whatsapp.linked_as": "Συνδεδεμένο ως {number}",
  "config.whatsapp.not_linked": "Δεν υπάρχει συνδεδεμένη συσκευή.",
  "config.whatsapp.link_qr": "Σύνδεση με κωδικό QR",
  "config.whatsapp.link_phone": "Σύνδεση με αριθμό τηλεφώνου",
  "config.whatsapp.unlink": "Αποσύνδεση",
  "config.whatsapp.send_test": "Αποστολή δοκιμαστικού μηνύματος",
  "config.whatsapp.qr_hint": "Στο κινητό: Ρυθμίσεις → Συνδεδεμένες συσκευές → Σύνδεση συσκευής και σάρωσε αυτόν τον κωδικό.",
  "config.whatsapp.code_hint": "Στο κινητό: Συνδεδεμένες συσκευές → Σύνδεση συσκευής → Σύνδεση με αριθμό τηλεφώνου και πληκτρολόγησε αυτόν τον κωδικό.",
  "config.whatsapp.relink_confirm": "Υπάρχει ήδη συνδεδεμένη συσκευή. Αποσύνδεση και νέα σύνδεση;",
  "config.whatsapp.unlink_confirm": "Αποσύνδεση του AuraGo από αυτόν τον λογαριασμό WhatsApp;",
  "config.whatsapp.unlinked_local": "Αφαιρέθηκε εδώ. Αφαίρεσε το AuraGo και από τις Συνδεδεμένες συσκευές στο κινητό.",
  "config.whatsapp.link_success": "Συνδέθηκε. Το AuraGo συνδέεται…",
  "config.whatsapp.link_timeout": "Ο κωδικός έληξε. Ξεκίνα ξανά.",
  "config.whatsapp.save_first": "Αποθήκευσε πρώτα τις αλλαγές σου.",
  "config.whatsapp.err_link_in_progress": "Μια σύνδεση βρίσκεται ήδη σε εξέλιξη.",
  "config.whatsapp.err_not_configured": "Ενεργοποίησε το WhatsApp, πρόσθεσε αριθμό κατόχου για τον ξεχωριστό αριθμό και αποθήκευσε.",
  "config.whatsapp.err_invalid_phone": "Γράψε τον αριθμό σε διεθνή μορφή, π.χ. +491701234567.",
  "config.whatsapp.err_not_connected": "Το WhatsApp δεν είναι συνδεδεμένο.",
  "config.whatsapp.err_rate_limited": "Περίμενε ένα λεπτό πριν από το επόμενο δοκιμαστικό μήνυμα.",
  "config.whatsapp.err_failed": "Το WhatsApp δεν απάντησε. Δοκίμασε ξανά.",
  "config.whatsapp.err_passkey_unsupported": "Το κινητό ζήτησε σύνδεση με κλειδί πρόσβασης, που το AuraGo δεν υποστηρίζει ακόμη. Χρησιμοποίησε τον κωδικό QR.",
  "config.whatsapp.state_logged_out": "Έγινε αποσύνδεση στο κινητό. Σύνδεσε ξανά.",
  "config.whatsapp.state_conflict": "Άλλος πελάτης χρησιμοποιεί αυτή τη σύνδεση. Αποσύνδεσε και σύνδεσε ξανά.",
  "config.whatsapp.state_temp_banned": "Προσωρινά αποκλεισμένο από το WhatsApp.",
  "config.whatsapp.state_outdated": "Αυτή η έκδοση του AuraGo είναι παλιά. Ενημέρωσε το AuraGo.",
  "help.whatsapp.mode": "Ξεχωριστός αριθμός: το AuraGo έχει δικό του αριθμό (προτείνεται). Δικός μου αριθμός: μόνο η συνομιλία με τον εαυτό σου φτάνει στο AuraGo.",
  "help.whatsapp.owner_numbers": "Διεθνής μορφή, χωρισμένοι με κόμμα. Μόνο αυτοί οι αριθμοί φτάνουν στο AuraGo.",
  "help.whatsapp.readonly": "Οι απαντήσεις σε σένα λειτουργούν· ο πράκτορας δεν στέλνει από μόνος του.",
  "help.whatsapp.send_read_receipts": "Μόνο με ξεχωριστό αριθμό AuraGo."
}
```

`es.json`:
```json
{
  "config.whatsapp.intro": "Chatea con AuraGo en WhatsApp. AuraGo se une como dispositivo vinculado y solo responde a los números del propietario.",
  "config.whatsapp.enabled": "Activar WhatsApp",
  "config.whatsapp.mode": "Número",
  "config.whatsapp.mode_dedicated": "Número propio de AuraGo",
  "config.whatsapp.mode_self_chat": "Mi propio número (chat contigo mismo)",
  "config.whatsapp.owner_numbers": "Números del propietario",
  "config.whatsapp.self_chat_notice": "En este modo AuraGo solo lee tu chat contigo mismo. Los demás chats siguen siendo privados.",
  "config.whatsapp.readonly": "Solo lectura",
  "config.whatsapp.send_read_receipts": "Enviar confirmaciones de lectura",
  "config.whatsapp.self_chat_prefix": "Prefijo en el chat contigo mismo",
  "config.whatsapp.link_title": "Dispositivo vinculado",
  "config.whatsapp.phone_rule": "Abre WhatsApp en el teléfono al menos cada 14 días o se eliminarán los dispositivos vinculados. Hasta 4 dispositivos vinculados por cuenta.",
  "config.whatsapp.linked_as": "Vinculado como {number}",
  "config.whatsapp.not_linked": "Ningún dispositivo vinculado.",
  "config.whatsapp.link_qr": "Vincular con código QR",
  "config.whatsapp.link_phone": "Vincular con número de teléfono",
  "config.whatsapp.unlink": "Desvincular",
  "config.whatsapp.send_test": "Enviar mensaje de prueba",
  "config.whatsapp.qr_hint": "En el teléfono: Ajustes → Dispositivos vinculados → Vincular un dispositivo y escanea este código.",
  "config.whatsapp.code_hint": "En el teléfono: Dispositivos vinculados → Vincular un dispositivo → Vincular con número de teléfono y escribe este código.",
  "config.whatsapp.relink_confirm": "Ya hay un dispositivo vinculado. ¿Desvincularlo y vincular de nuevo?",
  "config.whatsapp.unlink_confirm": "¿Desvincular AuraGo de esta cuenta de WhatsApp?",
  "config.whatsapp.unlinked_local": "Eliminado aquí. Elimina también AuraGo en Dispositivos vinculados del teléfono.",
  "config.whatsapp.link_success": "Vinculado. AuraGo se está conectando…",
  "config.whatsapp.link_timeout": "El código caducó. Empieza de nuevo.",
  "config.whatsapp.save_first": "Guarda primero tus cambios.",
  "config.whatsapp.err_link_in_progress": "Ya hay una vinculación en curso.",
  "config.whatsapp.err_not_configured": "Activa WhatsApp, añade un número del propietario para el número propio y guarda.",
  "config.whatsapp.err_invalid_phone": "Escribe el número en formato internacional, p. ej. +491701234567.",
  "config.whatsapp.err_not_connected": "WhatsApp no está conectado.",
  "config.whatsapp.err_rate_limited": "Espera un minuto antes del siguiente mensaje de prueba.",
  "config.whatsapp.err_failed": "WhatsApp no respondió. Inténtalo de nuevo.",
  "config.whatsapp.err_passkey_unsupported": "El teléfono pidió vincular con una llave de acceso, que AuraGo aún no admite. Usa el código QR.",
  "config.whatsapp.state_logged_out": "Sesión cerrada en el teléfono. Vincula de nuevo.",
  "config.whatsapp.state_conflict": "Otro cliente usa esta vinculación. Desvincula y vincula de nuevo.",
  "config.whatsapp.state_temp_banned": "Bloqueado temporalmente por WhatsApp.",
  "config.whatsapp.state_outdated": "Esta versión de AuraGo está desactualizada. Actualiza AuraGo.",
  "help.whatsapp.mode": "Número propio: AuraGo tiene su propio número (recomendado). Mi número: solo tu chat contigo mismo llega a AuraGo.",
  "help.whatsapp.owner_numbers": "Formato internacional, separados por comas. Solo estos números llegan a AuraGo.",
  "help.whatsapp.readonly": "Las respuestas a ti siguen funcionando; el agente no puede enviar por iniciativa propia.",
  "help.whatsapp.send_read_receipts": "Solo con número propio de AuraGo."
}
```

`fr.json`:
```json
{
  "config.whatsapp.intro": "Discutez avec AuraGo sur WhatsApp. AuraGo rejoint un compte en tant qu'appareil connecté et ne répond qu'aux numéros du propriétaire.",
  "config.whatsapp.enabled": "Activer WhatsApp",
  "config.whatsapp.mode": "Numéro",
  "config.whatsapp.mode_dedicated": "Numéro dédié à AuraGo",
  "config.whatsapp.mode_self_chat": "Mon propre numéro (discussion avec vous-même)",
  "config.whatsapp.owner_numbers": "Numéros du propriétaire",
  "config.whatsapp.self_chat_notice": "Dans ce mode, AuraGo lit uniquement votre discussion avec vous-même. Toutes les autres discussions restent privées.",
  "config.whatsapp.readonly": "Lecture seule",
  "config.whatsapp.send_read_receipts": "Envoyer les confirmations de lecture",
  "config.whatsapp.self_chat_prefix": "Préfixe dans la discussion avec vous-même",
  "config.whatsapp.link_title": "Appareil connecté",
  "config.whatsapp.phone_rule": "Ouvrez WhatsApp sur le téléphone au moins tous les 14 jours, sinon les appareils connectés sont supprimés. Jusqu'à 4 appareils connectés par compte.",
  "config.whatsapp.linked_as": "Connecté en tant que {number}",
  "config.whatsapp.not_linked": "Aucun appareil connecté.",
  "config.whatsapp.link_qr": "Connecter par code QR",
  "config.whatsapp.link_phone": "Connecter par numéro de téléphone",
  "config.whatsapp.unlink": "Déconnecter",
  "config.whatsapp.send_test": "Envoyer un message de test",
  "config.whatsapp.qr_hint": "Sur le téléphone : Paramètres → Appareils connectés → Connecter un appareil, puis scannez ce code.",
  "config.whatsapp.code_hint": "Sur le téléphone : Appareils connectés → Connecter un appareil → Connecter avec le numéro de téléphone, puis saisissez ce code.",
  "config.whatsapp.relink_confirm": "Un appareil est déjà connecté. Le déconnecter et connecter à nouveau ?",
  "config.whatsapp.unlink_confirm": "Déconnecter AuraGo de ce compte WhatsApp ?",
  "config.whatsapp.unlinked_local": "Supprimé ici. Supprimez aussi AuraGo dans Appareils connectés sur le téléphone.",
  "config.whatsapp.link_success": "Connecté. AuraGo établit la connexion…",
  "config.whatsapp.link_timeout": "Le code a expiré. Recommencez.",
  "config.whatsapp.save_first": "Enregistrez d'abord vos modifications.",
  "config.whatsapp.err_link_in_progress": "Une connexion est déjà en cours.",
  "config.whatsapp.err_not_configured": "Activez WhatsApp, ajoutez un numéro du propriétaire pour le numéro dédié et enregistrez.",
  "config.whatsapp.err_invalid_phone": "Saisissez le numéro au format international, par ex. +491701234567.",
  "config.whatsapp.err_not_connected": "WhatsApp n'est pas connecté.",
  "config.whatsapp.err_rate_limited": "Attendez une minute avant le prochain message de test.",
  "config.whatsapp.err_failed": "WhatsApp n'a pas répondu. Réessayez.",
  "config.whatsapp.err_passkey_unsupported": "Le téléphone demande une connexion par clé d'accès, qu'AuraGo ne prend pas encore en charge. Utilisez le code QR.",
  "config.whatsapp.state_logged_out": "Déconnecté sur le téléphone. Connectez à nouveau.",
  "config.whatsapp.state_conflict": "Un autre client utilise cette connexion. Déconnectez puis connectez à nouveau.",
  "config.whatsapp.state_temp_banned": "Temporairement bloqué par WhatsApp.",
  "config.whatsapp.state_outdated": "Cette version d'AuraGo est obsolète. Mettez AuraGo à jour.",
  "help.whatsapp.mode": "Numéro dédié : AuraGo a son propre numéro (recommandé). Mon numéro : seule votre discussion avec vous-même atteint AuraGo.",
  "help.whatsapp.owner_numbers": "Format international, séparés par des virgules. Seuls ces numéros atteignent AuraGo.",
  "help.whatsapp.readonly": "Les réponses qui vous sont destinées fonctionnent ; l'agent ne peut pas envoyer de lui-même.",
  "help.whatsapp.send_read_receipts": "Uniquement avec un numéro dédié à AuraGo."
}
```

`hi.json`:
```json
{
  "config.whatsapp.intro": "WhatsApp पर AuraGo से चैट करें। AuraGo लिंक किए गए डिवाइस के रूप में जुड़ता है और केवल मालिक के नंबरों को जवाब देता है।",
  "config.whatsapp.enabled": "WhatsApp चालू करें",
  "config.whatsapp.mode": "नंबर",
  "config.whatsapp.mode_dedicated": "AuraGo का अलग नंबर",
  "config.whatsapp.mode_self_chat": "मेरा अपना नंबर (खुद से चैट)",
  "config.whatsapp.owner_numbers": "मालिक के नंबर",
  "config.whatsapp.self_chat_notice": "इस मोड में AuraGo केवल आपकी खुद से की गई चैट पढ़ता है। बाकी सभी चैट निजी रहती हैं।",
  "config.whatsapp.readonly": "केवल पढ़ने के लिए",
  "config.whatsapp.send_read_receipts": "रीड रसीदें भेजें",
  "config.whatsapp.self_chat_prefix": "खुद से चैट में उपसर्ग",
  "config.whatsapp.link_title": "लिंक किया गया डिवाइस",
  "config.whatsapp.phone_rule": "फ़ोन पर कम से कम हर 14 दिन में WhatsApp खोलें, नहीं तो लिंक किए गए डिवाइस हटा दिए जाते हैं। एक खाते में अधिकतम 4 लिंक किए गए डिवाइस।",
  "config.whatsapp.linked_as": "{number} के रूप में लिंक",
  "config.whatsapp.not_linked": "कोई डिवाइस लिंक नहीं है।",
  "config.whatsapp.link_qr": "QR कोड से लिंक करें",
  "config.whatsapp.link_phone": "फ़ोन नंबर से लिंक करें",
  "config.whatsapp.unlink": "लिंक हटाएँ",
  "config.whatsapp.send_test": "परीक्षण संदेश भेजें",
  "config.whatsapp.qr_hint": "फ़ोन पर: सेटिंग्स → लिंक किए गए डिवाइस → डिवाइस लिंक करें, फिर यह कोड स्कैन करें।",
  "config.whatsapp.code_hint": "फ़ोन पर: लिंक किए गए डिवाइस → डिवाइस लिंक करें → फ़ोन नंबर से लिंक करें, फिर यह कोड डालें।",
  "config.whatsapp.relink_confirm": "एक डिवाइस पहले से लिंक है। लिंक हटाकर फिर से लिंक करें?",
  "config.whatsapp.unlink_confirm": "क्या AuraGo को इस WhatsApp खाते से हटाएँ?",
  "config.whatsapp.unlinked_local": "यहाँ से हटा दिया गया। फ़ोन पर लिंक किए गए डिवाइस में से भी AuraGo हटाएँ।",
  "config.whatsapp.link_success": "लिंक हो गया। AuraGo कनेक्ट हो रहा है…",
  "config.whatsapp.link_timeout": "कोड की अवधि समाप्त हो गई। फिर से शुरू करें।",
  "config.whatsapp.save_first": "पहले अपने बदलाव सहेजें।",
  "config.whatsapp.err_link_in_progress": "एक लिंक पहले से चल रहा है।",
  "config.whatsapp.err_not_configured": "WhatsApp चालू करें, अलग नंबर के लिए मालिक का नंबर जोड़ें और सहेजें।",
  "config.whatsapp.err_invalid_phone": "नंबर अंतरराष्ट्रीय प्रारूप में डालें, जैसे +491701234567।",
  "config.whatsapp.err_not_connected": "WhatsApp कनेक्ट नहीं है।",
  "config.whatsapp.err_rate_limited": "अगले परीक्षण संदेश से पहले एक मिनट रुकें।",
  "config.whatsapp.err_failed": "WhatsApp ने जवाब नहीं दिया। फिर से कोशिश करें।",
  "config.whatsapp.err_passkey_unsupported": "फ़ोन ने पासकी से लिंक माँगा, जिसे AuraGo अभी समर्थित नहीं करता। QR कोड का उपयोग करें।",
  "config.whatsapp.state_logged_out": "फ़ोन पर लॉग आउट हो गया। फिर से लिंक करें।",
  "config.whatsapp.state_conflict": "कोई दूसरा क्लाइंट यह लिंक इस्तेमाल कर रहा है। लिंक हटाकर फिर से लिंक करें।",
  "config.whatsapp.state_temp_banned": "WhatsApp ने अस्थायी रूप से रोक लगाई है।",
  "config.whatsapp.state_outdated": "AuraGo का यह संस्करण पुराना है। AuraGo अपडेट करें।",
  "help.whatsapp.mode": "अलग नंबर: AuraGo का अपना नंबर होता है (अनुशंसित)। मेरा नंबर: केवल आपकी खुद से चैट AuraGo तक पहुँचती है।",
  "help.whatsapp.owner_numbers": "अंतरराष्ट्रीय प्रारूप, कॉमा से अलग। केवल ये नंबर AuraGo तक पहुँच सकते हैं।",
  "help.whatsapp.readonly": "आपको जवाब मिलते रहते हैं; एजेंट अपने आप संदेश नहीं भेज सकता।",
  "help.whatsapp.send_read_receipts": "केवल AuraGo के अलग नंबर के साथ।"
}
```

`it.json`:
```json
{
  "config.whatsapp.intro": "Chatta con AuraGo su WhatsApp. AuraGo si collega come dispositivo collegato e risponde solo ai numeri del proprietario.",
  "config.whatsapp.enabled": "Attiva WhatsApp",
  "config.whatsapp.mode": "Numero",
  "config.whatsapp.mode_dedicated": "Numero dedicato ad AuraGo",
  "config.whatsapp.mode_self_chat": "Il mio numero (chat con te stesso)",
  "config.whatsapp.owner_numbers": "Numeri del proprietario",
  "config.whatsapp.self_chat_notice": "In questa modalità AuraGo legge solo la tua chat con te stesso. Tutte le altre chat restano private.",
  "config.whatsapp.readonly": "Sola lettura",
  "config.whatsapp.send_read_receipts": "Invia conferme di lettura",
  "config.whatsapp.self_chat_prefix": "Prefisso nella chat con te stesso",
  "config.whatsapp.link_title": "Dispositivo collegato",
  "config.whatsapp.phone_rule": "Apri WhatsApp sul telefono almeno ogni 14 giorni, altrimenti i dispositivi collegati vengono rimossi. Fino a 4 dispositivi collegati per account.",
  "config.whatsapp.linked_as": "Collegato come {number}",
  "config.whatsapp.not_linked": "Nessun dispositivo collegato.",
  "config.whatsapp.link_qr": "Collega con codice QR",
  "config.whatsapp.link_phone": "Collega con numero di telefono",
  "config.whatsapp.unlink": "Scollega",
  "config.whatsapp.send_test": "Invia messaggio di prova",
  "config.whatsapp.qr_hint": "Sul telefono: Impostazioni → Dispositivi collegati → Collega un dispositivo, poi scansiona questo codice.",
  "config.whatsapp.code_hint": "Sul telefono: Dispositivi collegati → Collega un dispositivo → Collega con il numero di telefono, poi inserisci questo codice.",
  "config.whatsapp.relink_confirm": "Un dispositivo è già collegato. Scollegarlo e collegare di nuovo?",
  "config.whatsapp.unlink_confirm": "Scollegare AuraGo da questo account WhatsApp?",
  "config.whatsapp.unlinked_local": "Rimosso qui. Rimuovi AuraGo anche da Dispositivi collegati sul telefono.",
  "config.whatsapp.link_success": "Collegato. AuraGo si sta connettendo…",
  "config.whatsapp.link_timeout": "Il codice è scaduto. Ricomincia.",
  "config.whatsapp.save_first": "Salva prima le modifiche.",
  "config.whatsapp.err_link_in_progress": "Un collegamento è già in corso.",
  "config.whatsapp.err_not_configured": "Attiva WhatsApp, aggiungi un numero del proprietario per il numero dedicato e salva.",
  "config.whatsapp.err_invalid_phone": "Inserisci il numero in formato internazionale, ad es. +491701234567.",
  "config.whatsapp.err_not_connected": "WhatsApp non è connesso.",
  "config.whatsapp.err_rate_limited": "Attendi un minuto prima del prossimo messaggio di prova.",
  "config.whatsapp.err_failed": "WhatsApp non ha risposto. Riprova.",
  "config.whatsapp.err_passkey_unsupported": "Il telefono ha chiesto un collegamento con passkey, che AuraGo non supporta ancora. Usa il codice QR.",
  "config.whatsapp.state_logged_out": "Disconnesso sul telefono. Collega di nuovo.",
  "config.whatsapp.state_conflict": "Un altro client usa questo collegamento. Scollega e collega di nuovo.",
  "config.whatsapp.state_temp_banned": "Bloccato temporaneamente da WhatsApp.",
  "config.whatsapp.state_outdated": "Questa versione di AuraGo è obsoleta. Aggiorna AuraGo.",
  "help.whatsapp.mode": "Numero dedicato: AuraGo ha un proprio numero (consigliato). Il mio numero: solo la tua chat con te stesso raggiunge AuraGo.",
  "help.whatsapp.owner_numbers": "Formato internazionale, separati da virgole. Solo questi numeri raggiungono AuraGo.",
  "help.whatsapp.readonly": "Le risposte a te funzionano ancora; l'agente non può inviare di sua iniziativa.",
  "help.whatsapp.send_read_receipts": "Solo con numero dedicato ad AuraGo."
}
```

`ja.json`:
```json
{
  "config.whatsapp.intro": "WhatsApp で AuraGo とチャットします。AuraGo はリンク済みデバイスとして参加し、所有者の番号にだけ返信します。",
  "config.whatsapp.enabled": "WhatsApp を有効にする",
  "config.whatsapp.mode": "番号",
  "config.whatsapp.mode_dedicated": "AuraGo 専用の番号",
  "config.whatsapp.mode_self_chat": "自分の番号（自分とのチャット）",
  "config.whatsapp.owner_numbers": "所有者の番号",
  "config.whatsapp.self_chat_notice": "このモードでは AuraGo は自分とのチャットだけを読みます。ほかのチャットは非公開のままです。",
  "config.whatsapp.readonly": "読み取り専用",
  "config.whatsapp.send_read_receipts": "既読通知を送信",
  "config.whatsapp.self_chat_prefix": "自分とのチャットの接頭辞",
  "config.whatsapp.link_title": "リンク済みデバイス",
  "config.whatsapp.phone_rule": "少なくとも 14 日ごとにスマートフォンで WhatsApp を開いてください。開かないとリンク済みデバイスは削除されます。1 アカウントにつき最大 4 台までリンクできます。",
  "config.whatsapp.linked_as": "{number} としてリンク済み",
  "config.whatsapp.not_linked": "リンク済みのデバイスはありません。",
  "config.whatsapp.link_qr": "QR コードでリンク",
  "config.whatsapp.link_phone": "電話番号でリンク",
  "config.whatsapp.unlink": "リンクを解除",
  "config.whatsapp.send_test": "テストメッセージを送信",
  "config.whatsapp.qr_hint": "スマートフォンで：設定 → リンク済みデバイス → デバイスをリンク を開き、このコードをスキャンします。",
  "config.whatsapp.code_hint": "スマートフォンで：リンク済みデバイス → デバイスをリンク → 電話番号でリンク を開き、このコードを入力します。",
  "config.whatsapp.relink_confirm": "すでにデバイスがリンクされています。解除して再リンクしますか？",
  "config.whatsapp.unlink_confirm": "この WhatsApp アカウントから AuraGo のリンクを解除しますか？",
  "config.whatsapp.unlinked_local": "ここでは削除しました。スマートフォンのリンク済みデバイスからも AuraGo を削除してください。",
  "config.whatsapp.link_success": "リンクしました。AuraGo が接続しています…",
  "config.whatsapp.link_timeout": "コードの有効期限が切れました。もう一度始めてください。",
  "config.whatsapp.save_first": "先に変更を保存してください。",
  "config.whatsapp.err_link_in_progress": "すでにリンク処理中です。",
  "config.whatsapp.err_not_configured": "WhatsApp を有効にし、専用番号の場合は所有者の番号を追加して保存してください。",
  "config.whatsapp.err_invalid_phone": "番号は国際形式で入力してください（例：+491701234567）。",
  "config.whatsapp.err_not_connected": "WhatsApp に接続されていません。",
  "config.whatsapp.err_rate_limited": "次のテストメッセージまで 1 分お待ちください。",
  "config.whatsapp.err_failed": "WhatsApp から応答がありません。もう一度お試しください。",
  "config.whatsapp.err_passkey_unsupported": "スマートフォンがパスキーによるリンクを求めましたが、AuraGo はまだ対応していません。QR コードを使ってください。",
  "config.whatsapp.state_logged_out": "スマートフォンでログアウトされました。再リンクしてください。",
  "config.whatsapp.state_conflict": "別のクライアントがこのリンクを使っています。解除して再リンクしてください。",
  "config.whatsapp.state_temp_banned": "WhatsApp により一時的にブロックされています。",
  "config.whatsapp.state_outdated": "この AuraGo のバージョンは古くなっています。AuraGo を更新してください。",
  "help.whatsapp.mode": "専用の番号：AuraGo が自分の番号を持ちます（推奨）。自分の番号：自分とのチャットだけが AuraGo に届きます。",
  "help.whatsapp.owner_numbers": "国際形式でカンマ区切り。これらの番号だけが AuraGo に届きます。",
  "help.whatsapp.readonly": "あなたへの返信は引き続き動作します。エージェントが自発的に送信することはできません。",
  "help.whatsapp.send_read_receipts": "AuraGo 専用の番号の場合のみ。"
}
```

`nl.json`:
```json
{
  "config.whatsapp.intro": "Chat met AuraGo via WhatsApp. AuraGo meldt zich aan als gekoppeld apparaat en antwoordt alleen eigenaarsnummers.",
  "config.whatsapp.enabled": "WhatsApp inschakelen",
  "config.whatsapp.mode": "Nummer",
  "config.whatsapp.mode_dedicated": "Eigen AuraGo-nummer",
  "config.whatsapp.mode_self_chat": "Mijn eigen nummer (chat met jezelf)",
  "config.whatsapp.owner_numbers": "Eigenaarsnummers",
  "config.whatsapp.self_chat_notice": "In deze modus leest AuraGo alleen je chat met jezelf. Alle andere chats blijven privé.",
  "config.whatsapp.readonly": "Alleen-lezen",
  "config.whatsapp.send_read_receipts": "Leesbevestigingen sturen",
  "config.whatsapp.self_chat_prefix": "Voorvoegsel in chat met jezelf",
  "config.whatsapp.link_title": "Gekoppeld apparaat",
  "config.whatsapp.phone_rule": "Open WhatsApp op de telefoon minstens elke 14 dagen, anders worden gekoppelde apparaten verwijderd. Maximaal 4 gekoppelde apparaten per account.",
  "config.whatsapp.linked_as": "Gekoppeld als {number}",
  "config.whatsapp.not_linked": "Geen apparaat gekoppeld.",
  "config.whatsapp.link_qr": "Koppelen met QR-code",
  "config.whatsapp.link_phone": "Koppelen met telefoonnummer",
  "config.whatsapp.unlink": "Ontkoppelen",
  "config.whatsapp.send_test": "Testbericht sturen",
  "config.whatsapp.qr_hint": "Op de telefoon: Instellingen → Gekoppelde apparaten → Apparaat koppelen en scan deze code.",
  "config.whatsapp.code_hint": "Op de telefoon: Gekoppelde apparaten → Apparaat koppelen → Koppelen met telefoonnummer en voer deze code in.",
  "config.whatsapp.relink_confirm": "Er is al een apparaat gekoppeld. Ontkoppelen en opnieuw koppelen?",
  "config.whatsapp.unlink_confirm": "AuraGo ontkoppelen van dit WhatsApp-account?",
  "config.whatsapp.unlinked_local": "Hier verwijderd. Verwijder AuraGo ook onder Gekoppelde apparaten op de telefoon.",
  "config.whatsapp.link_success": "Gekoppeld. AuraGo maakt verbinding…",
  "config.whatsapp.link_timeout": "De code is verlopen. Begin opnieuw.",
  "config.whatsapp.save_first": "Sla eerst je wijzigingen op.",
  "config.whatsapp.err_link_in_progress": "Er loopt al een koppeling.",
  "config.whatsapp.err_not_configured": "Schakel WhatsApp in, voeg voor het eigen AuraGo-nummer een eigenaarsnummer toe en sla op.",
  "config.whatsapp.err_invalid_phone": "Voer het nummer in internationaal formaat in, bijv. +491701234567.",
  "config.whatsapp.err_not_connected": "WhatsApp is niet verbonden.",
  "config.whatsapp.err_rate_limited": "Wacht een minuut tot het volgende testbericht.",
  "config.whatsapp.err_failed": "WhatsApp reageerde niet. Probeer het opnieuw.",
  "config.whatsapp.err_passkey_unsupported": "De telefoon vroeg om koppelen met een toegangssleutel, wat AuraGo nog niet ondersteunt. Gebruik de QR-code.",
  "config.whatsapp.state_logged_out": "Afgemeld op de telefoon. Koppel opnieuw.",
  "config.whatsapp.state_conflict": "Een andere client gebruikt deze koppeling. Ontkoppel en koppel opnieuw.",
  "config.whatsapp.state_temp_banned": "Tijdelijk geblokkeerd door WhatsApp.",
  "config.whatsapp.state_outdated": "Deze AuraGo-versie is verouderd. Werk AuraGo bij.",
  "help.whatsapp.mode": "Eigen AuraGo-nummer: AuraGo heeft een eigen nummer (aanbevolen). Mijn nummer: alleen je chat met jezelf bereikt AuraGo.",
  "help.whatsapp.owner_numbers": "Internationaal formaat, gescheiden door komma's. Alleen deze nummers bereiken AuraGo.",
  "help.whatsapp.readonly": "Antwoorden aan jou blijven werken; de agent kan niet uit zichzelf versturen.",
  "help.whatsapp.send_read_receipts": "Alleen met een eigen AuraGo-nummer."
}
```

`no.json`:
```json
{
  "config.whatsapp.intro": "Chat med AuraGo på WhatsApp. AuraGo kobles til som en tilknyttet enhet og svarer bare eiernumre.",
  "config.whatsapp.enabled": "Aktiver WhatsApp",
  "config.whatsapp.mode": "Nummer",
  "config.whatsapp.mode_dedicated": "Eget AuraGo-nummer",
  "config.whatsapp.mode_self_chat": "Mitt eget nummer (chat med deg selv)",
  "config.whatsapp.owner_numbers": "Eiernumre",
  "config.whatsapp.self_chat_notice": "I denne modusen leser AuraGo bare chatten med deg selv. Alle andre chatter forblir private.",
  "config.whatsapp.readonly": "Skrivebeskyttet",
  "config.whatsapp.send_read_receipts": "Send lesebekreftelser",
  "config.whatsapp.self_chat_prefix": "Prefiks i chat med deg selv",
  "config.whatsapp.link_title": "Tilknyttet enhet",
  "config.whatsapp.phone_rule": "Åpne WhatsApp på telefonen minst hver 14. dag, ellers fjernes tilknyttede enheter. Opptil 4 tilknyttede enheter per konto.",
  "config.whatsapp.linked_as": "Tilknyttet som {number}",
  "config.whatsapp.not_linked": "Ingen enhet tilknyttet.",
  "config.whatsapp.link_qr": "Koble til med QR-kode",
  "config.whatsapp.link_phone": "Koble til med telefonnummer",
  "config.whatsapp.unlink": "Fjern tilknytning",
  "config.whatsapp.send_test": "Send testmelding",
  "config.whatsapp.qr_hint": "På telefonen: Innstillinger → Tilknyttede enheter → Koble til en enhet, og skann deretter koden.",
  "config.whatsapp.code_hint": "På telefonen: Tilknyttede enheter → Koble til en enhet → Koble til med telefonnummer i stedet, og skriv inn koden.",
  "config.whatsapp.relink_confirm": "En enhet er allerede tilknyttet. Fjerne den og koble til på nytt?",
  "config.whatsapp.unlink_confirm": "Fjerne AuraGo fra denne WhatsApp-kontoen?",
  "config.whatsapp.unlinked_local": "Fjernet her. Fjern også AuraGo under Tilknyttede enheter på telefonen.",
  "config.whatsapp.link_success": "Tilknyttet. AuraGo kobler til…",
  "config.whatsapp.link_timeout": "Koden er utløpt. Start på nytt.",
  "config.whatsapp.save_first": "Lagre endringene dine først.",
  "config.whatsapp.err_link_in_progress": "En tilknytning pågår allerede.",
  "config.whatsapp.err_not_configured": "Slå på WhatsApp, legg til et eiernummer for eget AuraGo-nummer, og lagre.",
  "config.whatsapp.err_invalid_phone": "Skriv nummeret i internasjonalt format, f.eks. +491701234567.",
  "config.whatsapp.err_not_connected": "WhatsApp er ikke tilkoblet.",
  "config.whatsapp.err_rate_limited": "Vent ett minutt før neste testmelding.",
  "config.whatsapp.err_failed": "WhatsApp svarte ikke. Prøv igjen.",
  "config.whatsapp.err_passkey_unsupported": "Telefonen ba om tilknytning med tilgangsnøkkel, som AuraGo ikke støtter ennå. Bruk QR-koden.",
  "config.whatsapp.state_logged_out": "Logget ut på telefonen. Koble til på nytt.",
  "config.whatsapp.state_conflict": "En annen klient bruker denne tilknytningen. Fjern den og koble til på nytt.",
  "config.whatsapp.state_temp_banned": "Midlertidig blokkert av WhatsApp.",
  "config.whatsapp.state_outdated": "Denne AuraGo-versjonen er utdatert. Oppdater AuraGo.",
  "help.whatsapp.mode": "Eget AuraGo-nummer: AuraGo har sitt eget nummer (anbefalt). Mitt nummer: bare chatten med deg selv når AuraGo.",
  "help.whatsapp.owner_numbers": "Internasjonalt format, adskilt med komma. Bare disse numrene når AuraGo.",
  "help.whatsapp.readonly": "Svar til deg virker fortsatt; agenten kan ikke sende på egen hånd.",
  "help.whatsapp.send_read_receipts": "Bare med eget AuraGo-nummer."
}
```

`pl.json`:
```json
{
  "config.whatsapp.intro": "Rozmawiaj z AuraGo przez WhatsApp. AuraGo dołącza jako połączone urządzenie i odpowiada tylko numerom właściciela.",
  "config.whatsapp.enabled": "Włącz WhatsApp",
  "config.whatsapp.mode": "Numer",
  "config.whatsapp.mode_dedicated": "Osobny numer AuraGo",
  "config.whatsapp.mode_self_chat": "Mój własny numer (czat z samym sobą)",
  "config.whatsapp.owner_numbers": "Numery właściciela",
  "config.whatsapp.self_chat_notice": "W tym trybie AuraGo czyta tylko Twój czat z samym sobą. Wszystkie inne czaty pozostają prywatne.",
  "config.whatsapp.readonly": "Tylko odczyt",
  "config.whatsapp.send_read_receipts": "Wysyłaj potwierdzenia odczytu",
  "config.whatsapp.self_chat_prefix": "Prefiks w czacie z samym sobą",
  "config.whatsapp.link_title": "Połączone urządzenie",
  "config.whatsapp.phone_rule": "Otwieraj WhatsApp w telefonie co najmniej co 14 dni, inaczej połączone urządzenia zostaną usunięte. Do 4 połączonych urządzeń na konto.",
  "config.whatsapp.linked_as": "Połączono jako {number}",
  "config.whatsapp.not_linked": "Brak połączonego urządzenia.",
  "config.whatsapp.link_qr": "Połącz kodem QR",
  "config.whatsapp.link_phone": "Połącz numerem telefonu",
  "config.whatsapp.unlink": "Rozłącz",
  "config.whatsapp.send_test": "Wyślij wiadomość testową",
  "config.whatsapp.qr_hint": "W telefonie: Ustawienia → Połączone urządzenia → Połącz urządzenie, a potem zeskanuj ten kod.",
  "config.whatsapp.code_hint": "W telefonie: Połączone urządzenia → Połącz urządzenie → Połącz za pomocą numeru telefonu, a potem wpisz ten kod.",
  "config.whatsapp.relink_confirm": "Urządzenie jest już połączone. Rozłączyć je i połączyć ponownie?",
  "config.whatsapp.unlink_confirm": "Odłączyć AuraGo od tego konta WhatsApp?",
  "config.whatsapp.unlinked_local": "Usunięto tutaj. Usuń też AuraGo w sekcji Połączone urządzenia w telefonie.",
  "config.whatsapp.link_success": "Połączono. AuraGo nawiązuje połączenie…",
  "config.whatsapp.link_timeout": "Kod wygasł. Zacznij od nowa.",
  "config.whatsapp.save_first": "Najpierw zapisz zmiany.",
  "config.whatsapp.err_link_in_progress": "Łączenie już trwa.",
  "config.whatsapp.err_not_configured": "Włącz WhatsApp, dodaj numer właściciela dla osobnego numeru i zapisz.",
  "config.whatsapp.err_invalid_phone": "Wpisz numer w formacie międzynarodowym, np. +491701234567.",
  "config.whatsapp.err_not_connected": "WhatsApp nie jest połączony.",
  "config.whatsapp.err_rate_limited": "Odczekaj minutę przed kolejną wiadomością testową.",
  "config.whatsapp.err_failed": "WhatsApp nie odpowiedział. Spróbuj ponownie.",
  "config.whatsapp.err_passkey_unsupported": "Telefon poprosił o połączenie kluczem dostępu, którego AuraGo jeszcze nie obsługuje. Użyj kodu QR.",
  "config.whatsapp.state_logged_out": "Wylogowano w telefonie. Połącz ponownie.",
  "config.whatsapp.state_conflict": "Inny klient używa tego połączenia. Rozłącz i połącz ponownie.",
  "config.whatsapp.state_temp_banned": "Tymczasowo zablokowane przez WhatsApp.",
  "config.whatsapp.state_outdated": "Ta wersja AuraGo jest nieaktualna. Zaktualizuj AuraGo.",
  "help.whatsapp.mode": "Osobny numer: AuraGo ma własny numer (zalecane). Mój numer: do AuraGo trafia tylko Twój czat z samym sobą.",
  "help.whatsapp.owner_numbers": "Format międzynarodowy, oddzielone przecinkami. Tylko te numery docierają do AuraGo.",
  "help.whatsapp.readonly": "Odpowiedzi do Ciebie nadal działają; agent nie może wysyłać z własnej inicjatywy.",
  "help.whatsapp.send_read_receipts": "Tylko z osobnym numerem AuraGo."
}
```

`pt.json`:
```json
{
  "config.whatsapp.intro": "Converse com o AuraGo no WhatsApp. O AuraGo entra como dispositivo conectado e só responde aos números do proprietário.",
  "config.whatsapp.enabled": "Ativar WhatsApp",
  "config.whatsapp.mode": "Número",
  "config.whatsapp.mode_dedicated": "Número próprio do AuraGo",
  "config.whatsapp.mode_self_chat": "Meu próprio número (conversa consigo mesmo)",
  "config.whatsapp.owner_numbers": "Números do proprietário",
  "config.whatsapp.self_chat_notice": "Neste modo, o AuraGo lê apenas a sua conversa consigo mesmo. Todas as outras conversas continuam privadas.",
  "config.whatsapp.readonly": "Somente leitura",
  "config.whatsapp.send_read_receipts": "Enviar confirmações de leitura",
  "config.whatsapp.self_chat_prefix": "Prefixo na conversa consigo mesmo",
  "config.whatsapp.link_title": "Dispositivo conectado",
  "config.whatsapp.phone_rule": "Abra o WhatsApp no telefone pelo menos a cada 14 dias, ou os dispositivos conectados serão removidos. Até 4 dispositivos conectados por conta.",
  "config.whatsapp.linked_as": "Conectado como {number}",
  "config.whatsapp.not_linked": "Nenhum dispositivo conectado.",
  "config.whatsapp.link_qr": "Conectar com código QR",
  "config.whatsapp.link_phone": "Conectar com número de telefone",
  "config.whatsapp.unlink": "Desconectar",
  "config.whatsapp.send_test": "Enviar mensagem de teste",
  "config.whatsapp.qr_hint": "No telefone: Configurações → Dispositivos conectados → Conectar um dispositivo e escaneie este código.",
  "config.whatsapp.code_hint": "No telefone: Dispositivos conectados → Conectar um dispositivo → Conectar com número de telefone e digite este código.",
  "config.whatsapp.relink_confirm": "Já existe um dispositivo conectado. Desconectar e conectar de novo?",
  "config.whatsapp.unlink_confirm": "Desconectar o AuraGo desta conta do WhatsApp?",
  "config.whatsapp.unlinked_local": "Removido aqui. Remova também o AuraGo em Dispositivos conectados no telefone.",
  "config.whatsapp.link_success": "Conectado. O AuraGo está se conectando…",
  "config.whatsapp.link_timeout": "O código expirou. Comece de novo.",
  "config.whatsapp.save_first": "Salve suas alterações primeiro.",
  "config.whatsapp.err_link_in_progress": "Já há uma conexão em andamento.",
  "config.whatsapp.err_not_configured": "Ative o WhatsApp, adicione um número do proprietário para o número próprio e salve.",
  "config.whatsapp.err_invalid_phone": "Digite o número no formato internacional, por ex. +491701234567.",
  "config.whatsapp.err_not_connected": "O WhatsApp não está conectado.",
  "config.whatsapp.err_rate_limited": "Aguarde um minuto antes da próxima mensagem de teste.",
  "config.whatsapp.err_failed": "O WhatsApp não respondeu. Tente novamente.",
  "config.whatsapp.err_passkey_unsupported": "O telefone pediu conexão com chave de acesso, que o AuraGo ainda não suporta. Use o código QR.",
  "config.whatsapp.state_logged_out": "Sessão encerrada no telefone. Conecte de novo.",
  "config.whatsapp.state_conflict": "Outro cliente usa esta conexão. Desconecte e conecte de novo.",
  "config.whatsapp.state_temp_banned": "Bloqueado temporariamente pelo WhatsApp.",
  "config.whatsapp.state_outdated": "Esta versão do AuraGo está desatualizada. Atualize o AuraGo.",
  "help.whatsapp.mode": "Número próprio: o AuraGo tem seu próprio número (recomendado). Meu número: só a sua conversa consigo mesmo chega ao AuraGo.",
  "help.whatsapp.owner_numbers": "Formato internacional, separados por vírgulas. Só esses números chegam ao AuraGo.",
  "help.whatsapp.readonly": "As respostas para você continuam funcionando; o agente não pode enviar por conta própria.",
  "help.whatsapp.send_read_receipts": "Apenas com número próprio do AuraGo."
}
```

`sv.json`:
```json
{
  "config.whatsapp.intro": "Chatta med AuraGo på WhatsApp. AuraGo ansluts som en länkad enhet och svarar bara ägarnummer.",
  "config.whatsapp.enabled": "Aktivera WhatsApp",
  "config.whatsapp.mode": "Nummer",
  "config.whatsapp.mode_dedicated": "Eget AuraGo-nummer",
  "config.whatsapp.mode_self_chat": "Mitt eget nummer (chatt med dig själv)",
  "config.whatsapp.owner_numbers": "Ägarnummer",
  "config.whatsapp.self_chat_notice": "I det här läget läser AuraGo bara din chatt med dig själv. Alla andra chattar förblir privata.",
  "config.whatsapp.readonly": "Skrivskyddad",
  "config.whatsapp.send_read_receipts": "Skicka läskvitton",
  "config.whatsapp.self_chat_prefix": "Prefix i chatt med dig själv",
  "config.whatsapp.link_title": "Länkad enhet",
  "config.whatsapp.phone_rule": "Öppna WhatsApp på telefonen minst var 14:e dag, annars tas länkade enheter bort. Upp till 4 länkade enheter per konto.",
  "config.whatsapp.linked_as": "Länkad som {number}",
  "config.whatsapp.not_linked": "Ingen enhet länkad.",
  "config.whatsapp.link_qr": "Länka med QR-kod",
  "config.whatsapp.link_phone": "Länka med telefonnummer",
  "config.whatsapp.unlink": "Ta bort länk",
  "config.whatsapp.send_test": "Skicka testmeddelande",
  "config.whatsapp.qr_hint": "På telefonen: Inställningar → Länkade enheter → Länka en enhet, och skanna sedan koden.",
  "config.whatsapp.code_hint": "På telefonen: Länkade enheter → Länka en enhet → Länka med telefonnummer i stället, och ange sedan koden.",
  "config.whatsapp.relink_confirm": "En enhet är redan länkad. Ta bort den och länka igen?",
  "config.whatsapp.unlink_confirm": "Ta bort AuraGo från det här WhatsApp-kontot?",
  "config.whatsapp.unlinked_local": "Borttagen här. Ta också bort AuraGo under Länkade enheter på telefonen.",
  "config.whatsapp.link_success": "Länkad. AuraGo ansluter…",
  "config.whatsapp.link_timeout": "Koden har gått ut. Börja om.",
  "config.whatsapp.save_first": "Spara dina ändringar först.",
  "config.whatsapp.err_link_in_progress": "En länkning pågår redan.",
  "config.whatsapp.err_not_configured": "Slå på WhatsApp, lägg till ett ägarnummer för eget AuraGo-nummer och spara.",
  "config.whatsapp.err_invalid_phone": "Ange numret i internationellt format, t.ex. +491701234567.",
  "config.whatsapp.err_not_connected": "WhatsApp är inte anslutet.",
  "config.whatsapp.err_rate_limited": "Vänta en minut innan nästa testmeddelande.",
  "config.whatsapp.err_failed": "WhatsApp svarade inte. Försök igen.",
  "config.whatsapp.err_passkey_unsupported": "Telefonen bad om länkning med nyckel, vilket AuraGo inte stöder än. Använd QR-koden.",
  "config.whatsapp.state_logged_out": "Utloggad på telefonen. Länka igen.",
  "config.whatsapp.state_conflict": "En annan klient använder den här länken. Ta bort den och länka igen.",
  "config.whatsapp.state_temp_banned": "Tillfälligt blockerad av WhatsApp.",
  "config.whatsapp.state_outdated": "Den här AuraGo-versionen är föråldrad. Uppdatera AuraGo.",
  "help.whatsapp.mode": "Eget AuraGo-nummer: AuraGo har ett eget nummer (rekommenderas). Mitt nummer: bara din chatt med dig själv når AuraGo.",
  "help.whatsapp.owner_numbers": "Internationellt format, separerade med kommatecken. Bara dessa nummer når AuraGo.",
  "help.whatsapp.readonly": "Svar till dig fungerar fortfarande; agenten kan inte skicka på eget initiativ.",
  "help.whatsapp.send_read_receipts": "Bara med eget AuraGo-nummer."
}
```

`zh.json`:
```json
{
  "config.whatsapp.intro": "通过 WhatsApp 与 AuraGo 聊天。AuraGo 以关联设备身份加入，只回复所有者号码。",
  "config.whatsapp.enabled": "启用 WhatsApp",
  "config.whatsapp.mode": "号码",
  "config.whatsapp.mode_dedicated": "AuraGo 专用号码",
  "config.whatsapp.mode_self_chat": "我自己的号码（与自己聊天）",
  "config.whatsapp.owner_numbers": "所有者号码",
  "config.whatsapp.self_chat_notice": "此模式下 AuraGo 只读取你与自己的聊天，其他聊天保持私密。",
  "config.whatsapp.readonly": "只读",
  "config.whatsapp.send_read_receipts": "发送已读回执",
  "config.whatsapp.self_chat_prefix": "自聊前缀",
  "config.whatsapp.link_title": "关联设备",
  "config.whatsapp.phone_rule": "请至少每 14 天在手机上打开一次 WhatsApp，否则关联设备会被移除。每个账号最多 4 台关联设备。",
  "config.whatsapp.linked_as": "已关联为 {number}",
  "config.whatsapp.not_linked": "未关联设备。",
  "config.whatsapp.link_qr": "通过二维码关联",
  "config.whatsapp.link_phone": "通过手机号关联",
  "config.whatsapp.unlink": "取消关联",
  "config.whatsapp.send_test": "发送测试消息",
  "config.whatsapp.qr_hint": "在手机上：设置 → 关联设备 → 关联新设备，然后扫描此二维码。",
  "config.whatsapp.code_hint": "在手机上：关联设备 → 关联新设备 → 改用手机号关联，然后输入此代码。",
  "config.whatsapp.relink_confirm": "已有设备关联。要取消关联并重新关联吗？",
  "config.whatsapp.unlink_confirm": "要将 AuraGo 从此 WhatsApp 账号取消关联吗？",
  "config.whatsapp.unlinked_local": "已在此处移除。请同时在手机的关联设备中移除 AuraGo。",
  "config.whatsapp.link_success": "已关联。AuraGo 正在连接…",
  "config.whatsapp.link_timeout": "代码已过期，请重新开始。",
  "config.whatsapp.save_first": "请先保存更改。",
  "config.whatsapp.err_link_in_progress": "已有关联正在进行。",
  "config.whatsapp.err_not_configured": "请启用 WhatsApp，在专用号码模式下添加所有者号码并保存。",
  "config.whatsapp.err_invalid_phone": "请使用国际格式输入号码，例如 +491701234567。",
  "config.whatsapp.err_not_connected": "WhatsApp 未连接。",
  "config.whatsapp.err_rate_limited": "请等待一分钟再发送下一条测试消息。",
  "config.whatsapp.err_failed": "WhatsApp 没有响应，请重试。",
  "config.whatsapp.err_passkey_unsupported": "手机要求使用通行密钥关联，AuraGo 暂不支持。请使用二维码。",
  "config.whatsapp.state_logged_out": "已在手机上退出登录，请重新关联。",
  "config.whatsapp.state_conflict": "另一个客户端正在使用此关联。请取消关联后重新关联。",
  "config.whatsapp.state_temp_banned": "已被 WhatsApp 暂时封禁。",
  "config.whatsapp.state_outdated": "此 AuraGo 版本已过时，请更新 AuraGo。",
  "help.whatsapp.mode": "专用号码：AuraGo 拥有自己的号码（推荐）。我的号码：只有你与自己的聊天会发送到 AuraGo。",
  "help.whatsapp.owner_numbers": "国际格式，以逗号分隔。只有这些号码能联系 AuraGo。",
  "help.whatsapp.readonly": "回复你的消息仍可正常发送；代理不能主动发送。",
  "help.whatsapp.send_read_receipts": "仅适用于 AuraGo 专用号码。"
}
```

- [ ] **Step 4: Add the section name and description** to each `ui/lang/config/sections/<locale>.json`, between `config.section.webhooks.label` and `config.section.whisper.desc`. The label is `"config.section.whatsapp.label": "WhatsApp"` in every locale (it is a brand name). The descriptions are `"config.section.whatsapp.desc"`:

| Locale | `config.section.whatsapp.desc` |
| --- | --- |
| cs | Propojí AuraGo s WhatsAppem jako propojené zařízení, abyste tam mohli chatovat s agentem. |
| da | Forbinder AuraGo med WhatsApp som tilknyttet enhed, så du kan chatte med agenten der. |
| de | Verbindet AuraGo als verknüpftes Gerät mit WhatsApp, damit du dort mit dem Agenten chatten kannst. |
| el | Συνδέει το AuraGo με το WhatsApp ως συνδεδεμένη συσκευή, ώστε να συνομιλείς εκεί με τον πράκτορα. |
| en | Connects AuraGo to WhatsApp as a linked device so you can chat with the agent there. |
| es | Conecta AuraGo a WhatsApp como dispositivo vinculado para que puedas chatear allí con el agente. |
| fr | Connecte AuraGo à WhatsApp en tant qu'appareil connecté pour discuter avec l'agent. |
| hi | AuraGo को लिंक किए गए डिवाइस के रूप में WhatsApp से जोड़ता है, ताकि आप वहाँ एजेंट से चैट कर सकें। |
| it | Collega AuraGo a WhatsApp come dispositivo collegato, così puoi chattare lì con l'agente. |
| ja | AuraGo をリンク済みデバイスとして WhatsApp に接続し、そこでエージェントとチャットできるようにします。 |
| nl | Verbindt AuraGo als gekoppeld apparaat met WhatsApp, zodat je daar met de agent kunt chatten. |
| no | Kobler AuraGo til WhatsApp som tilknyttet enhet, slik at du kan chatte med agenten der. |
| pl | Łączy AuraGo z WhatsApp jako połączone urządzenie, aby można było tam rozmawiać z agentem. |
| pt | Conecta o AuraGo ao WhatsApp como dispositivo conectado para você conversar com o agente lá. |
| sv | Ansluter AuraGo till WhatsApp som länkad enhet så att du kan chatta med agenten där. |
| zh | 将 AuraGo 作为关联设备连接到 WhatsApp，让你可以在那里与代理聊天。 |

`ui/config_help.json` — directly before the line `  "discord": {`, insert:
```json
  "whatsapp": {
    "mode": {
      "title": "Number",
      "description": "Dedicated: AuraGo has its own number (recommended because of ban risk). Own number: only your Message yourself chat reaches AuraGo."
    },
    "owner_numbers": {
      "title": "Owner numbers",
      "description": "International format such as +491701234567. Everyone else is ignored and never gets a reply."
    },
    "readonly": {
      "title": "Read-only",
      "description": "Replies to the owner still work, but the agent may not send on its own."
    },
    "send_read_receipts": {
      "title": "Send read receipts",
      "description": "Marks accepted owner messages as read. Dedicated mode only."
    }
  },
```

- [ ] **Step 5: Validate and run the tests; confirm they pass**

```bash
for f in ui/lang/config/whatsapp/*.json ui/lang/config/sections/*.json ui/config_help.json; do python -m json.tool "$f" > /dev/null || echo "BROKEN $f"; done
git diff --check -- ui/lang/config/sections ui/config_help.json
CGO_ENABLED=0 go test -count=1 ./ui/ -run 'TestTranslations'
CGO_ENABLED=0 go test -count=1 ./internal/server/ -run 'TestUII18NSectionsManifestCoversEveryPage'
```
Expected: no `BROKEN` output, no whitespace errors, and the tests PASS. Check `git diff --stat -- ui/lang/config/sections`: it must show exactly 2 added lines per locale and no hunks from another session.

- [ ] **Step 6: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="ui/lang/config/whatsapp ui/lang/config/sections ui/config_help.json"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "i18n(config): whatsapp section in all 16 locales" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```

---

### Task 13: Config UI section, link panel, sidebar icon, browser test

**Files:**
- Create: `ui/cfg/whatsapp.js`
- Create (test): `ui/whatsapp_config_browser_test.go`
- Modify: `ui/js/config/main.js`, in four places:
  - `SECTIONS` communication group, after the `telegram` item (≈155)
  - `CONFIG_SIDEBAR_ICON_SLOTS`, after `telegram: 59,` (≈682)
  - `CONFIG_SIDEBAR_ICON_SYMBOLS`, after the `kaggle:` entry (≈746)
  - `SECTION_MODULES`, after the `kaggle:` entry (≈3716)
- Modify: `ui/img/config-sidebar-icons.svg` (before `</svg>`) and `ui/img/config-sidebar-icons.json` (after the `kaggle` entry)
- Modify: `ui/css/config-workspace.css`: add a slot rule after `.config-icon-slot-127` (≈561) and the panel rules at the end of the file
- Modify (test): `ui/config_refresh_browser_test.go` — add fixtures in the `fixtures := map[string]any{` literal after the `"/api/local-llm/status"` entry (≈232)

- [ ] **Step 1: Write the failing browser test** `ui/whatsapp_config_browser_test.go`

```go
package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"testing"
)

func TestWhatsAppConfigLinkPanelBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	base, _ := url.Parse(configRefreshFixtureOriginWith(t, "de", false, func(cfg map[string]any) {
		section, _ := cfg["whatsapp"].(map[string]any)
		if section == nil {
			section = map[string]any{}
			cfg["whatsapp"] = section
		}
		section["enabled"] = true
		section["mode"] = "dedicated"
		section["owner_numbers"] = []any{"+491701234567"}
	}))
	proxy := httputil.NewSingleHostReverseProxy(base)
	var linkGets atomic.Int32
	var linked atomic.Bool
	writeJSON := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/whatsapp/status":
			state := "not_linked"
			if linked.Load() {
				state = "connecting"
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": state, "whatsapp": map[string]any{"enabled": true, "state": state, "linked": linked.Load(), "linked_jid_masked": "***4567"}})
		case "/api/whatsapp/link":
			if r.Method == http.MethodPost {
				writeJSON(w, http.StatusAccepted, map[string]any{"state": "waiting"})
				return
			}
			switch n := linkGets.Add(1); {
			case n == 1: // resume probe when the section renders
				writeJSON(w, http.StatusOK, map[string]any{"state": "idle"})
			case n == 2:
				writeJSON(w, http.StatusOK, map[string]any{"state": "qr", "qr_png": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="})
			default:
				linked.Store(true)
				writeJSON(w, http.StatusOK, map[string]any{"state": "success"})
			}
		default:
			proxy.ServeHTTP(w, r)
		}
	}))
	defer origin.Close()
	browser := newSmokeBrowser(t)
	page := browser.MustPage(origin.URL + "/config#whatsapp")
	defer func() {
		page.MustEval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `()=>{const b=document.getElementById('whatsapp-link-qr');return !!b&&!b.disabled}`)
	if !page.MustEval(`()=>document.getElementById('whatsapp-unlink').disabled&&document.getElementById('whatsapp-test').disabled`).Bool() {
		t.Error("unlink and test are enabled without a linked, healthy device")
	}
	page.MustElement("#whatsapp-link-qr").MustClick()
	waitForJSBool(t, page, `()=>{const img=document.getElementById('whatsapp-qr');return !img.hidden&&img.src.startsWith('data:image/png;base64,')}`)
	waitForJSBool(t, page, `()=>document.getElementById('whatsapp-link-view').hidden&&document.getElementById('whatsapp-linked').textContent.includes('***4567')`)
	gets := linkGets.Load()
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,2500))`)
	if linkGets.Load() != gets {
		t.Error("the panel kept polling after the link finished")
	}
	for _, width := range []int{390, 1440} {
		page.MustSetViewport(width, 900, 1, width == 390)
		if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
			t.Errorf("config overflow at %d px", width)
		}
	}
}
```

Also add to the base fixture map in `ui/config_refresh_browser_test.go`, directly after `		"/api/local-llm/status": map[string]any{"state": "disabled", "release_manifest_ready": false},`, so that the section matrix renders WhatsApp offline:
```go
		"/api/whatsapp/status": map[string]any{"status": "disabled", "whatsapp": map[string]any{"enabled": false, "state": "disabled"}},
		"/api/whatsapp/link":   map[string]any{"state": "idle"},
```

- [ ] **Step 2: Run the tests and confirm they fail**

```bash
AURAGO_RUN_BROWSER_SMOKE=1 CGO_ENABLED=0 go test -count=1 ./ui/ -run 'TestWhatsAppConfigLinkPanelBrowser'
```
Expected: FAIL. `waitForJSBool` times out because there is no `#whatsapp-link-qr`: the section does not exist yet.

- [ ] **Step 3: Implement** `ui/cfg/whatsapp.js`

```js
// WhatsApp: config fields plus the link panel. Link, unlink and test act on
// saved settings only. The QR code is the server-rendered PNG data URI.
function renderWhatsAppSection(section) {
    const c = { enabled: false, mode: 'dedicated', owner_numbers: [], readonly: false, send_read_receipts: true, self_chat_prefix: '', ...configData.whatsapp };
    const form = window.AuraConfigForm;
    const field = (key, label, options = {}) => form.field({ id: 'whatsapp-' + key, path: 'whatsapp.' + key, label, value: c[key], ...options });
    const toggle = (key, label, help) => field(key, label, { type: 'toggle', help: help || '', value: c[key] === true });
    const button = (id, label, primary = false) => `<button id="whatsapp-${id}" type="button" class="${primary ? 'btn-save' : 'btn-secondary'}">${escapeHtml(label)}</button>`;
    document.getElementById('content').innerHTML = form.renderSpec({
        label: section.label,
        desc: t('config.whatsapp.intro'),
        beforeHTML: cfgStatusBanner.html('whatsapp-status', 'checking'),
        groups: [
            { titleKey: 'config.refresh.operation', content:
                toggle('enabled', t('config.whatsapp.enabled')) +
                field('mode', t('config.whatsapp.mode'), { type: 'select', help: t('help.whatsapp.mode'), options: [
                    { value: 'dedicated', label: t('config.whatsapp.mode_dedicated') },
                    { value: 'self_chat', label: t('config.whatsapp.mode_self_chat') }
                ] }) +
                '<div id="whatsapp-self-chat-notice" class="cfg-note is-warning" role="status" hidden>' + escapeHtml(t('config.whatsapp.self_chat_notice')) + '</div>' +
                '<div id="whatsapp-owner-fields">' + field('owner_numbers', t('config.whatsapp.owner_numbers'), {
                    value: (c.owner_numbers || []).join(', '), help: t('help.whatsapp.owner_numbers'), placeholder: '+491701234567'
                }) + '</div>'
            },
            { title: t('config.whatsapp.link_title'), desc: t('config.whatsapp.phone_rule'), className: 'whatsapp-link-panel', content:
                '<p id="whatsapp-linked" class="field-help" aria-live="polite"></p>' +
                // The pairing phone is not a config field: data-cfg-dirty-bound keeps it out of the save dock.
                '<label class="field-group pw-field"><span class="field-label">' + escapeHtml(t('config.whatsapp.link_phone')) + '</span>' +
                '<input id="whatsapp-phone" class="field-input" type="tel" autocomplete="off" inputmode="tel" placeholder="+491701234567" data-cfg-dirty-bound="true"></label>' +
                form.actions([
                    { html: button('link-qr', t('config.whatsapp.link_qr'), true) },
                    { html: button('link-phone', t('config.whatsapp.link_phone')) },
                    { html: button('unlink', t('config.whatsapp.unlink')) },
                    { html: button('test', t('config.whatsapp.send_test')) }
                ]) +
                '<p id="whatsapp-action-reason" class="field-help" aria-live="polite"></p>' +
                cfgActionResult.html('whatsapp-result') +
                '<div id="whatsapp-link-view" class="whatsapp-link-view" hidden>' +
                    '<img id="whatsapp-qr" class="whatsapp-qr" alt="' + escapeAttr(t('config.whatsapp.link_qr')) + '" hidden>' +
                    '<p id="whatsapp-code" class="whatsapp-pairing-code" hidden></p>' +
                    '<p id="whatsapp-link-hint" class="field-help"></p>' +
                '</div>'
            },
            { titleKey: 'config.refresh.settings', content:
                toggle('readonly', t('config.whatsapp.readonly'), t('help.whatsapp.readonly')) +
                toggle('send_read_receipts', t('config.whatsapp.send_read_receipts'), t('help.whatsapp.send_read_receipts')) +
                field('self_chat_prefix', t('config.whatsapp.self_chat_prefix'))
            }
        ]
    });
    const root = document.querySelector('#content .cfg-section');
    root.id = 'whatsapp-config';
    root.querySelector('#whatsapp-owner_numbers').dataset.type = 'array';
    const lifecycle = new AbortController();
    let status = {};
    let pollTimer = 0;
    // Literal maps keep every key visible to the static i18n lint.
    const ERRORS = {
        link_in_progress: () => t('config.whatsapp.err_link_in_progress'),
        already_linked: () => t('config.whatsapp.relink_confirm'),
        not_configured: () => t('config.whatsapp.err_not_configured'),
        invalid_phone: () => t('config.whatsapp.err_invalid_phone'),
        not_connected: () => t('config.whatsapp.err_not_connected'),
        rate_limited: () => t('config.whatsapp.err_rate_limited'),
        passkey_unsupported: () => t('config.whatsapp.err_passkey_unsupported'),
        outdated: () => t('config.whatsapp.state_outdated')
    };
    const DETAILS = {
        not_linked: () => t('config.whatsapp.not_linked'),
        logged_out: () => t('config.whatsapp.state_logged_out'),
        conflict: () => t('config.whatsapp.state_conflict'),
        temp_banned: () => t('config.whatsapp.state_temp_banned'),
        outdated: () => t('config.whatsapp.state_outdated')
    };
    const BANNER = { disabled: 'off', not_configured: 'unconfigured', not_linked: 'unconfigured', linking: 'checking', connecting: 'checking',
        healthy: 'connected', degraded: 'degraded', logged_out: 'error', conflict: 'error', temp_banned: 'error', outdated: 'error' };
    const ACTIVE = ['waiting', 'qr', 'code'];
    const errorText = code => (typeof code === 'string' && Object.hasOwn(ERRORS, code) ? ERRORS[code]() : t('config.whatsapp.err_failed'));
    const reasonText = body => (body && body.error ? errorText(body.error) : '');

    function reason(action) {
        const saved = window.AuraConfigState.get('whatsapp') || {};
        if (hasUnsavedConfigChanges()) return t('config.whatsapp.save_first');
        if (action === 'unlink') return status.linked ? '' : t('config.whatsapp.not_linked');
        if (!saved.enabled || (saved.mode !== 'self_chat' && !(saved.owner_numbers || []).length)) return t('config.whatsapp.err_not_configured');
        if (action === 'link-phone' && !/^(\+|00)[0-9 ().-]{6,}$/.test(root.querySelector('#whatsapp-phone').value.trim())) return t('config.whatsapp.err_invalid_phone');
        if (action === 'test' && status.state !== 'healthy') return t('config.whatsapp.err_not_connected');
        return '';
    }

    function sync() {
        if (!root.isConnected) return;
        const mode = root.querySelector('#whatsapp-mode').value;
        root.querySelector('#whatsapp-self-chat-notice').hidden = mode !== 'self_chat';
        root.querySelector('#whatsapp-owner-fields').hidden = mode === 'self_chat';
        root.querySelector('#whatsapp-linked').textContent = status.linked
            ? t('config.whatsapp.linked_as').replace('{number}', status.linked_jid_masked || '')
            : t('config.whatsapp.not_linked');
        for (const action of ['link-qr', 'link-phone', 'unlink', 'test']) {
            const control = root.querySelector('#whatsapp-' + action);
            const why = reason(action);
            control.disabled = !!why || control.getAttribute('aria-busy') === 'true';
            control.title = why;
        }
        root.querySelector('#whatsapp-action-reason').textContent = reason('link-qr');
    }

    async function refreshStatus() {
        if (!root.isConnected) return;
        await cfgStatusBanner.check(root.querySelector('#whatsapp-status'), signal => fetch('/api/whatsapp/status', { signal, credentials: 'same-origin' }), body => {
            status = (body && body.whatsapp) || {};
            const detail = Object.hasOwn(DETAILS, status.state) ? DETAILS[status.state]() : '';
            return { state: Object.hasOwn(BANNER, status.state) ? BANNER[status.state] : 'unknown', detail };
        });
        sync();
    }

    function showLink(snap) {
        const state = snap && snap.state;
        const qr = root.querySelector('#whatsapp-qr');
        const code = root.querySelector('#whatsapp-code');
        root.querySelector('#whatsapp-link-view').hidden = state !== 'qr' && state !== 'code';
        qr.hidden = state !== 'qr';
        if (state === 'qr' && typeof snap.qr_png === 'string' && snap.qr_png.startsWith('data:image/png;base64,')) qr.src = snap.qr_png;
        code.hidden = state !== 'code';
        code.textContent = state === 'code' ? String(snap.pairing_code || '') : '';
        root.querySelector('#whatsapp-link-hint').textContent = state === 'qr' ? t('config.whatsapp.qr_hint') : (state === 'code' ? t('config.whatsapp.code_hint') : '');
        if (state === 'success') cfgActionResult.show('whatsapp-result', 'success', t('config.whatsapp.link_success'));
        else if (state === 'timeout') cfgActionResult.show('whatsapp-result', 'warning', t('config.whatsapp.link_timeout'));
        else if (state === 'error') cfgActionResult.show('whatsapp-result', 'error', errorText(snap.error));
    }

    // Polls every 2 s only while a session is waiting, showing a QR code or a pairing code.
    function poll() {
        clearTimeout(pollTimer);
        pollTimer = setTimeout(async () => {
            if (!root.isConnected) return;
            try {
                const response = await fetch('/api/whatsapp/link', { credentials: 'same-origin', signal: lifecycle.signal });
                const snap = await response.json();
                if (!root.isConnected) return;
                showLink(snap);
                if (ACTIVE.includes(snap.state)) poll(); else refreshStatus();
            } catch (_) {
                if (root.isConnected && !lifecycle.signal.aborted) poll();
            }
        }, 2000);
    }

    function startLink(control, method) {
        const linked = !!status.linked;
        const body = { method, force: linked };
        if (method === 'phone') body.phone = root.querySelector('#whatsapp-phone').value.trim();
        return cfgActionResult(control, () => fetch('/api/whatsapp/link', {
            method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
        }), {
            target: 'whatsapp-result',
            confirm: linked ? { message: t('config.whatsapp.relink_confirm') } : undefined,
            reasonText,
            onSuccess: outcome => {
                cfgActionResult.show('whatsapp-result', '', '');
                showLink(outcome.body || {});
                refreshStatus();
                poll();
            }
        });
    }

    root.querySelector('#whatsapp-link-qr').addEventListener('click', event => startLink(event.currentTarget, 'qr'));
    root.querySelector('#whatsapp-link-phone').addEventListener('click', event => startLink(event.currentTarget, 'phone'));
    root.querySelector('#whatsapp-unlink').addEventListener('click', event => cfgActionResult(event.currentTarget,
        () => fetch('/api/whatsapp/logout', { method: 'POST', credentials: 'same-origin' }), {
            target: 'whatsapp-result',
            confirm: { message: t('config.whatsapp.unlink_confirm') },
            reasonText,
            success: outcome => (outcome.body && outcome.body.remote_removed === false ? t('config.whatsapp.unlinked_local') : t('config.feedback.done')),
            onSuccess: () => { showLink({ state: 'idle' }); refreshStatus(); }
        }));
    root.querySelector('#whatsapp-test').addEventListener('click', event => cfgActionResult(event.currentTarget,
        () => fetch('/api/whatsapp/test', { method: 'POST', credentials: 'same-origin' }), { target: 'whatsapp-result', reasonText, onSuccess: () => refreshStatus() }));
    root.querySelector('#whatsapp-phone').addEventListener('input', sync);
    root.querySelector('#whatsapp-mode').addEventListener('change', sync);
    document.addEventListener('cfg:statechange', sync, { signal: lifecycle.signal });
    window.addEventListener('cfg:section-leave', () => { lifecycle.abort(); clearTimeout(pollTimer); }, { once: true, signal: lifecycle.signal });
    sync();
    // Resume a session that started before this section was opened.
    fetch('/api/whatsapp/link', { credentials: 'same-origin', signal: lifecycle.signal })
        .then(response => response.json())
        .then(snap => { if (root.isConnected && ACTIVE.includes(snap.state)) { showLink(snap); poll(); } })
        .catch(() => { /* the panel still offers linking */ });
    return refreshStatus();
}
```

- [ ] **Step 4: Register the section, icon and styles**

`ui/js/config/main.js`:
- In the `communication` group, directly after the `telegram` item, add:
```js
            { key: 'whatsapp', label: t('config.section.whatsapp.label'), desc: t('config.section.whatsapp.desc') },
```
- In `CONFIG_SIDEBAR_ICON_SLOTS`, directly after `    telegram: 59,`, add `    whatsapp: 128,`.
- In `CONFIG_SIDEBAR_ICON_SYMBOLS`, directly after the `kaggle:` line, add:
```js
    whatsapp: '<path d="M64 24a38 38 0 0 0-33 57l-6 21 22-6a38 38 0 1 0 17-72z" fill="#25d366" opacity=".22"/><path d="M64 24a38 38 0 0 0-33 57l-6 21 22-6a38 38 0 1 0 17-72z" fill="none" stroke="#25d366" stroke-width="6" stroke-linejoin="round"/><path d="M50 46c-3 11 9 28 24 33l6-7-9-6-4 4c-5-2-10-7-12-12l4-4-6-9z" fill="#25d366"/>',
```
- In `SECTION_MODULES`, directly after `    kaggle: { m: 'kaggle', fn: 'renderKaggleSection' },`, add:
```js
    whatsapp: { m: 'whatsapp', fn: 'renderWhatsAppSection' },
```

`ui/img/config-sidebar-icons.svg` — directly before `</svg>`, add (slot 128 = column 7, row 11):
```xml
  <g class="cfg-icon-cell" data-key="whatsapp" data-slot="128" transform="translate(896 1408)"><title>whatsapp</title><path d="M64 24a38 38 0 0 0-33 57l-6 21 22-6a38 38 0 1 0 17-72z" fill="#25d366" opacity=".22"/><path d="M64 24a38 38 0 0 0-33 57l-6 21 22-6a38 38 0 1 0 17-72z" fill="none" stroke="#25d366" stroke-width="6" stroke-linejoin="round"/><path d="M50 46c-3 11 9 28 24 33l6-7-9-6-4 4c-5-2-10-7-12-12l4-4-6-9z" fill="#25d366"/></g>
```

`ui/img/config-sidebar-icons.json` — after the `kaggle` object, add `,` and then:
```json
    {
      "key": "whatsapp",
      "slot": 128,
      "column": 7,
      "row": 11,
      "source_type": "custom",
      "source": "AuraGo custom Precision sidebar icon set"
    }
```

`ui/css/config-workspace.css`:
- Directly after the `.config-icon-slot-127` rule, add:
```css
.pw-page[data-workspace-page="config"] .config-icon-slot-128 { background-position: 70% 100%; }
```
- At the end of the file, add:
```css
/* WhatsApp link panel: server-rendered QR code and pairing code. */
.pw-page[data-workspace-page="config"] .whatsapp-link-view { display: grid; justify-items: start; gap: 12px; margin-top: 16px; }
.pw-page[data-workspace-page="config"] .whatsapp-link-view[hidden],
.pw-page[data-workspace-page="config"] .whatsapp-qr[hidden],
.pw-page[data-workspace-page="config"] .whatsapp-pairing-code[hidden] { display: none; }
.pw-page[data-workspace-page="config"] .whatsapp-qr { width: min(256px, 100%); height: auto; aspect-ratio: 1; border-radius: 12px; image-rendering: pixelated; }
.pw-page[data-workspace-page="config"] .whatsapp-pairing-code { margin: 0; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 28px; font-weight: 700; letter-spacing: .12em; }
```

- [ ] **Step 5: Run the checks and tests; confirm they pass**

```bash
node --check ui/cfg/whatsapp.js
node --check ui/js/config/main.js
python -m json.tool ui/img/config-sidebar-icons.json > /dev/null
git diff --check -- ui/cfg/whatsapp.js ui/js/config/main.js ui/css/config-workspace.css ui/img
npm run build:ui -- --check
CGO_ENABLED=0 go test -count=1 ./ui/ -run 'Precision|Config|I18N|TestTranslations'
AURAGO_RUN_BROWSER_SMOKE=1 CGO_ENABLED=0 go test -count=1 ./ui/ -run 'TestWhatsAppConfigLinkPanelBrowser'
AURAGO_RUN_BROWSER_SMOKE=1 CGO_ENABLED=0 go test -count=1 ./ui/ -run 'TestConfigRefreshRealSectionsBrowser/matrix/whatsapp$'
```
Expected:
- Both `node --check` runs and the JSON check succeed.
- `build:ui --check` reports no changes.
- The tests PASS, including `TestConfigSidebarIconSpriteContract` and `TestTranslations_ConfigCfgJSKeysExist`.

On this Windows machine the browser tests can depend on the animation setting. If a timing assertion flakes, rerun with reduced motion both ways before treating it as a failure.

- [ ] **Step 6: Commit**

```bash
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="ui/cfg/whatsapp.js ui/whatsapp_config_browser_test.go ui/config_refresh_browser_test.go ui/js/config/main.js ui/img/config-sidebar-icons.svg ui/img/config-sidebar-icons.json ui/css/config-workspace.css"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "feat(ui): whatsapp config section with QR and pairing-code link panel" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```

---

### Task 14: DOX guides

**Files:**
- Create: `internal/whatsapp/AGENTS.md`
- Modify: `AGENTS.md` (repository map, dependencies, Child DOX Index)
- Modify: `internal/server/AGENTS.md` (new section before `### Rocket.Chat Runtime Contract`)
- Modify: `internal/tools/AGENTS.md` (Agent Filesystem Jail Contract)
- Modify: `ui/AGENTS.md` (sidebar slot list; Config bullets)

`licences/AGENTS.md` was already updated in Task 4.

- [ ] **Step 1: Create** `internal/whatsapp/AGENTS.md`

```markdown
# WhatsApp channel

## Purpose

Owner-only WhatsApp channel over `go.mau.fi/whatsmeow` (WhatsApp Web multi-device). AuraGo is a linked companion device named "AuraGo". Design: `plans/2026-10-31-whatsapp-integration-design.md`. Plan: `plans/2026-10-31-whatsapp-integration-plan.md`.

## Ownership

- `client.go`: `waClient`, `deviceStore`, `backend`, the whatsmeow adapter and device props. This is the only place that constructs whatsmeow clients.
- `store.go`: `<data_dir>/whatsapp/device.db`. Uses modernc with `_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)` and `sqlstore` dialect `sqlite`.
- `events.go`: event → state, terminal and reconnect policy.
- `runtime.go`: the `Bot` generation. `recipients.go`: the owner-recipient resolver.
- `link.go`, `qr.go`: link sessions; PNG data URIs via `internal/qrcode`.
- `log.go`, `mask.go`: whatsmeow→slog adapter and digit masking.
- Owned elsewhere:
  - `internal/server/whatsapp_*.go`: lifecycle and HTTP.
  - `internal/integrationstatus/whatsapp.go`: status.
  - `internal/config/whatsapp.go`: the `whatsapp:` section.

## Local Contracts

- **One connection per process.** At most one Bot or one LinkSession owns the device. The link client is cancelled and disconnected before a Bot connects.
- **The device store is a credential.**
  - Directory 0700, file 0600 via `fileutil.RestrictToOwner`.
  - Never in `config.SQLiteDatabasePaths` or backups.
  - Listed in `config.WhatsAppProtectedPaths` for agent file tools and outgoing attachments.
- **Owner-only.** Every send resolves recipients through `ownerRecipients`; no API accepts arbitrary numbers. In self-chat mode messages go to the own JID with `self_chat_prefix`.
- **Health and reconnects.**
  - `healthy` only after `OfflineSyncCompleted`, followed by presence `unavailable`.
  - Terminal: LoggedOut, StreamReplaced, ClientOutdated and TemporaryBan. A temporary ban retries after it expires.
  - Connect errors and `ConnectFailure` retry with a backoff of 5 s doubling to 5 min, forever.
  - A stopped generation never writes status.
- **Privacy.**
  - whatsmeow debug output is dropped.
  - Every forwarded line is scrubbed and digit-masked.
  - QR payloads, pairing codes and message content are never logged.
  - Status holds masked numbers only.
- **Stage scope.** Stage 1 only counts inbound messages. Filters and the pipeline arrive in stage 2 (`plans/2026-10-31-whatsapp-2-text.md`).
- **Release pin.** whatsmeow is pinned by pseudo-version; a "client outdated" outage needs a release with a newer pin. Distributed binaries are GPL-3.0 combined works (`licences/AGENTS.md`).

## Work Guidance

- Add a whatsmeow call to `waClient` and `fakeClient` (`fake_test.go`) first. Never call whatsmeow from other packages.
- Map a new event in `classifyEvent` with a table-test row before using it.
- A new SQLite file needs an entry in `config.SQLiteDatabasePaths` or in `internal/audit` `sqliteBackupExclusions`.

## Verification

- `CGO_ENABLED=0 go test ./internal/whatsapp/ -count=1` uses the fake client; the store test runs on a real temp file.
- Run these with `CGO_ENABLED=0 go test`:
  - `./internal/server/ -run 'WhatsApp'`
  - `./internal/config/ -run '(WhatsApp|E164)'`
  - `./internal/tools/ -run 'WhatsApp'`
  - `./internal/audit/ -run '(WhatsApp|RouteContract|SQLiteOpens|NetworkClient)'`
- Browser: `AURAGO_RUN_BROWSER_SMOKE=1 go test -count=1 ./ui -run 'TestWhatsAppConfigLinkPanelBrowser'`.
- Real-device linking is manual (stage 1 Task 15). Never claim it without a phone.

## Child DOX Index

None.
```

- [ ] **Step 2: Update root** `AGENTS.md`
- In the Integrations line, replace `` `webhooks` (incoming/outgoing); `sandbox` owns Linux Landlock. `` with `` `webhooks` (incoming/outgoing), `whatsapp` (owner-only WhatsApp linked device); `sandbox` owns Linux Landlock. ``
- In the Dependencies sentence, after `` `github.com/bwmarrin/discordgo`, `` insert `` `go.mau.fi/whatsmeow` (links GPL-3.0 `go.mau.fi/libsignal`, so distributed binaries are GPL-3.0 combined works), ``.
- In the Child DOX Index table, directly after the `| Video Studio |` row, add:
```markdown
| WhatsApp channel | `internal/whatsapp/AGENTS.md`, `internal/server/AGENTS.md`, `ui/AGENTS.md`, `licences/AGENTS.md` | Linked-device store/runtime/link sessions, admin endpoints, Config link panel, GPL-3.0 binary licensing. |
```

- [ ] **Step 3: Update** `internal/server/AGENTS.md` — directly before `### Rocket.Chat Runtime Contract`, insert:
```markdown
### WhatsApp Runtime Contract
- `whatsapp_runtime.go` owns one `whatsapp.Bot` and at most one `whatsapp.LinkSession` behind `whatsAppLifecycleMu`.
  - `replaceConfigSnapshot` revokes a changed generation (`CancelIfConfigChanged`).
  - `configureWhatsAppBot` drains outside the config lock and restarts. Startup, the Config save hot reload (`whatsAppChanged`) and `afterWhatsAppLink` call it.
  - `stopWhatsApp` in `closeRuntimeResources` drains the session and the Bot and forbids restarts.
  - No Bot starts while a link session is active; egg mode never runs one.
- `/api/whatsapp/status|link|logout|test`:
  - All use `requireAdmin` and `Cache-Control: no-store` and return fixed error codes only, because raw errors can carry JIDs.
  - `POST /api/whatsapp/link` caps the body at 4 KiB, rejects unknown fields and validates before a forced unlink.
  - `GET /api/whatsapp/link` returns the session snapshot or `idle`.
  - The test message is owner-only, at most one per minute.
  - Handlers reach the runtime through `whatsAppController`; tests set `Server.whatsAppCtl`.
- `handleUpdateConfig` rejects invalid sections with `config.ValidateWhatsAppConfig`. Backups never include `data/whatsapp`.
- Verify `TestWhatsAppRuntime*`, `TestServerWhatsAppController*`, `TestHandleUpdateConfigWhatsApp*`, `TestWhatsAppHandlers*` and `TestWhatsAppRoutesRequireAdmin`.
```

- [ ] **Step 4: Update** `internal/tools/AGENTS.md` — directly after the bullet starting `` - `data/tresor.db` and its WAL/SHM sidecars ``, add:
```markdown
- The WhatsApp device store `<data_dir>/whatsapp/device.db` and its `-wal`/`-shm`/`-journal` sidecars come from `config.WhatsAppProtectedPaths` and are listed in `protectedSystemFilesFromConfig`. That list is also used for the outgoing-attachment hard-link checks. The store is a credential, not a backed-up SQLite store. Verify `TestWhatsAppDeviceStoreIsProtectedSystemState`.
```

- [ ] **Step 5: Update** `ui/AGENTS.md`
- Replace `(slots 0–131; \`gog\` 121, \`ruview\` 122, \`cyd_creator\` 123, \`decisions\` 124)` with `(slots 0–131; \`gog\` 121, \`ruview\` 122, \`cyd_creator\` 123, \`decisions\` 124, \`whatsapp\` 128)`.
- Directly after the bullet `- Configuration connection tests operate only on saved configuration. …`, add:
```markdown
- Config WhatsApp (`cfg/whatsapp.js`) acts only on saved settings. Link, unlink and test show a visible reason when they are unavailable.
  - The QR code is the server-rendered PNG data URI from `GET /api/whatsapp/link`; it is never rendered client-side.
  - The panel polls every 2 s only while the session is `waiting`, `qr` or `code`, and stops on section leave.
  - Relinking and unlinking are confirmed in a dialog.
  - The pairing-phone input is not a config field.
  - Verify `TestWhatsAppConfigLinkPanelBrowser`.
```

- [ ] **Step 6: Check and commit**

```bash
git diff --check -- AGENTS.md internal/server/AGENTS.md internal/tools/AGENTS.md ui/AGENTS.md internal/whatsapp/AGENTS.md
CGO_ENABLED=0 go test ./internal/audit/ -count=1 -run '(Contract|Routed|Docs)'
node .gitnexus/run.cjs detect-changes --scope all --repo .
F="internal/whatsapp/AGENTS.md AGENTS.md internal/server/AGENTS.md internal/tools/AGENTS.md ui/AGENTS.md"
IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "docs(dox): whatsapp channel guides and routing" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
```
Expected: no whitespace errors, and the audit doc tests PASS.

The root `AGENTS.md` has a history of being marked assume-unchanged locally. If `git status` does not show the edit, run `git update-index --no-assume-unchanged AGENTS.md` before staging. Report any guide that was left unchanged on purpose (for example `internal/qrcode/AGENTS.md`: only a consumer was added).

---

### Task 15: Real-device link acceptance (user scans)

**Files:** none in the repository. Evidence goes to the ignored `reports/whatsapp-stage1-acceptance.md`.

The user must do this with a real second number; never claim it without a phone. Use a local scratch instance:
- Build with the assetpack ldflags and run `-install-assets`.
- Set its own `AURAGO_MASTER_KEY` and a dead `DOCKER_HOST`.
- Stop only the instance's own PID.

- [ ] **Step 1: Prepare.** Build the binary from the stage-1 HEAD. Start a scratch instance with a fresh data dir and set:
  - `whatsapp.enabled: true`
  - `mode: dedicated`
  - `owner_numbers` set to the user's personal number

  Open Config → WhatsApp. Expected: the banner shows "not set up" with detail "No device linked", and "Link via QR code" is enabled.
- [ ] **Step 2: Link by QR.** Click "Link via QR code"; the user scans with the second phone. Expected:
  - The QR code rotates without errors.
  - The phone lists "AuraGo" under Linked devices.
  - The banner goes to Connected within 60 s.
  - `GET /api/whatsapp/status` shows `healthy`, `linked: true` and a masked number.
- [ ] **Step 3: Test message.** Click "Send test message". The owner phone receives "AuraGo WhatsApp test (manual) — <timestamp>". A second click within 60 s shows the rate-limit text.
- [ ] **Step 4: Restart without relinking.** Stop and start the instance. Expected: it is `healthy` again without scanning. `device.db` exists under `<data>/whatsapp/`; on Linux, check the modes with `stat -c '%a' <data>/whatsapp <data>/whatsapp/device.db` (expected `700` and `600`).
- [ ] **Step 5: Unlink.** Click "Unlink" and confirm. Expected:
  - The phone no longer lists AuraGo, and the result says Done (remote removed).
  - The banner returns to "not set up".
- [ ] **Step 6: Link by pairing code.** Enter the second phone's number in international format and click "Link via phone number". The user enters the `XXXX-XXXX` code on the phone. Expected: healthy, as in Step 2. If the phone asks for a passkey, record that the `passkey_unsupported` message appeared, and finish with QR.
- [ ] **Step 7: Self-chat.** Switch to `self_chat`, save and relink on the personal account if the user agrees; otherwise skip and record why. Send the test message. Expected: it appears in "Message yourself" with the 🤖 prefix.
- [ ] **Step 8: Log privacy.** Search the instance log:
  - `grep -nE '[0-9]{8,}' <log>` must not show full phone numbers.
  - `grep -n '2@' <log>` must not show QR payloads.
- [ ] **Step 9: Record.** Write the date, AuraGo commit, whatsmeow pseudo-version, phone OS and the result of each step to `reports/whatsapp-stage1-acceptance.md` (ignored; never committed).
- [ ] **Step 10: Follow up on failures.** If linking hangs or ends in `conflict` right after `PairSuccess`, stop. This is the hand-off risk in the master plan. Implement the fallback: the link session hands its client to the Bot instead of disconnecting it. Do this as a new task with a test in `link_test.go`, then repeat Steps 2–5.

---

## Self-review

### Spec coverage (stage 1)

| Stage-1 requirement | Task(s) |
| --- | --- |
| whatsmeow dependency at the pinned pseudo-version, CGO-free | 3 |
| License docs (THIRD_PARTY_LIBRARIES/NOTICES, UEBERSICHT, texts, README; LICENSE stays MIT; pinned README strings kept) | 4 |
| `config.WhatsApp` struct, defaults, E.164 normalization/validation, template | 1; save-time validation in 9 |
| Config UI labels and help in 16 locales | 12 |
| Device store at `<DataDir>/whatsapp/device.db`, 0700/0600, modernc DSN with foreign keys, real temp-file test | 3 |
| `waClient` interface, whatsmeow adapter, slog adapter, test fake | 3 (slog), 5 |
| `integrationstatus` WhatsApp status | 2 |
| Event→state mapping including terminal states | 6 |
| Runtime `Bot`: one connection, backoff 5 s→5 min, terminal stop, healthy only after OfflineSyncCompleted, presence `unavailable` | 7, 9 |
| Link sessions: server-side PNG QR via `internal/qrcode`, phone code, 3-min TTL, one at a time, force relink | 8, 9 |
| Owner-recipient resolver and `SendTestMessage` (self_chat → own JID with prefix) | 7 |
| Server wiring: fields, configure/stop, startup, hot reload, revocation, shutdown | 9 |
| Admin endpoints with no-store; route manifest | 10 |
| Config UI link panel (state badge, masked number, QR/phone buttons, unlink, test, 14-day hint) | 13 |
| Deny `<DataDir>/whatsapp` like the vault: the store joins `protectedSystemFilesFromConfig` (agent file tools, Docker Compose host access, outgoing attachments); shell/Python keep the vault's existing boundary; test backup exclusion | 1 (backup), 11 |
| DOX/AGENTS.md updates | 4 (licences), 14 |
| Inbound messages only counted | 6, 7 (`CountWhatsAppMessage`) |
| Real-device acceptance of linking | 15 |

### Deliberate deviations and additions

1. `GET /api/whatsapp/link` returns `{"state":"idle"}` when the process has no session. That state is not in the spec's list.
2. The status adds `linked`, `messages_received` and `last_test_at` to the spec's field list. The stage-2 drop counters exist already but stay 0.
3. New limits: at most 10 owner numbers and a 16-rune `self_chat_prefix`. At load time invalid numbers are dropped with a warning (no number in it); at save time they are rejected.
4. Egg mode maps to the status `disabled`.
5. Passkey pairing, which this whatsmeow version can request, is not supported. The session ends with `passkey_unsupported` and the UI advises QR.
6. On `PairSuccess` the link client is cancelled and disconnected, and a new Bot logs in with the stored device. Task 15 verifies this, and Step 10 holds the fallback.
7. The test message goes to every configured owner number. It is sent directly through the client until stage 2 introduces the send queue.
8. The network-client inventory entry is added in stage 1 rather than stage 4, because the client exists from stage 1.
9. The `help.whatsapp.*` strings live in `ui/lang/config/whatsapp/` rather than `ui/lang/help/`. The loader merges by key prefix, as with Manus. `ui/config_help.json` carries the English entry.
10. Linking requires an enabled, configured and saved section, per the spec's state table. Unlinking only requires a data dir and works while WhatsApp is disabled.
11. When there is no live connection, logout removes the local keys only and tells the UI to remove AuraGo on the phone as well.
12. A temporary ban with an expiry retries automatically once it expires ("terminal until expiry").
13. The maintenance-briefing hook moves to stage 4, because it touches `internal/agent` files other sessions are editing right now.
14. The `MessagingIngressManifest` update moves to stage 2, where the ingress lands.
15. The device platform type is set to CHROME to match the pairing-code client type. The phone lists the device as "AuraGo" via `store.SetOSInfo`.
16. The design says file tools and the Python sandbox deny `<DataDir>/whatsapp` like the vault. File tools, outgoing attachments and Compose protection do (via `protectedSystemFilesFromConfig` plus the `whatsapp/device.db*` name rule). Shell and Python get no WhatsApp-specific deny, because the vault has none there either: both rely on the sandbox scope, and both are readable under the Danger Zone `allow_unsafe_host_execution`. That shared risk is accepted; stage 4 names the WhatsApp keys in the unsafe-execution warning.

### Executed (2026-11-01)

Branch `claude/whatsapp-stage1`. Review fixes beyond the plan: whatsmeow auto-reconnect capped at 300 s; healthy after a late `Connected`; node-redacting logs; store-open retry; silent stopped generations; forced link replaces a running session (link generation counter); 10 s remote unlink with local fallback; strict 4 KiB POST body; same-origin `Origin` on POSTs; rebinding-safe host guard; `secure_delete` and file removal on local unlink; backup walks skip the store; `whatsapp/device.db*` name rule. Found in passing and split off: the pre-existing `send_*` agent tools copy protected files by absolute path (separate session).
